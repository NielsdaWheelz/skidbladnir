package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
	logCode logging.ErrorCode
}

var (
	errorUnauthenticated             = apiError{Code: "Unauthenticated", Message: "Authentication required.", Status: http.StatusUnauthorized, logCode: logging.ErrorUnauthenticated}
	errorInvalidRequest              = apiError{Code: "InvalidRequest", Message: "The request is not valid.", Status: http.StatusBadRequest, logCode: logging.ErrorInvalidRequest}
	errorRequestTooLarge             = apiError{Code: "RequestTooLarge", Message: "The request is too large.", Status: http.StatusRequestEntityTooLarge, logCode: logging.ErrorRequestTooLarge}
	errorWorkingDirectoryInvalid     = apiError{Code: "WorkingDirectoryInvalid", Message: "Choose a valid working directory.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorWorkingDirectoryInvalid}
	errorWorkingDirectoryUnavailable = apiError{Code: "WorkingDirectoryUnavailable", Message: "That directory does not exist or cannot be opened.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorWorkingDirectoryUnavailable}
	errorDirectoryListingUnavailable = apiError{Code: "DirectoryListingUnavailable", Message: "This directory cannot be browsed. Enter the path instead.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorDirectoryListingUnavailable}
	errorDirectoryListingTooLarge    = apiError{Code: "DirectoryListingTooLarge", Message: "This directory has too many folders to show. Enter the path instead.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorDirectoryListingTooLarge}
	errorProfileUnknown              = apiError{Code: "ProfileUnknown", Message: "Choose an available profile.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorProfileUnknown}
	errorSessionNameInvalid          = apiError{Code: "SessionNameInvalid", Message: "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorSessionNameInvalid}
	errorSessionNameConflict         = apiError{Code: "SessionNameConflict", Message: "A session with that name already exists.", Status: http.StatusConflict, logCode: logging.ErrorSessionNameConflict}
	errorObjectiveInvalid            = apiError{Code: "ObjectiveInvalid", Message: "Use 1–240 characters without terminal controls.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorObjectiveInvalid}
	errorSessionNotFound             = apiError{Code: "SessionNotFound", Message: "That session no longer exists.", Status: http.StatusNotFound, logCode: logging.ErrorSessionNotFound}
	errorSessionIdentityMismatch     = apiError{Code: "SessionIdentityMismatch", Message: "The session changed. Refresh and try again.", Status: http.StatusConflict, logCode: logging.ErrorSessionIdentityMismatch}
	errorPairingInviteRejected       = apiError{Code: "PairingInviteRejected", Message: "This fleet invite is invalid, expired, or already used.", Status: http.StatusUnauthorized, logCode: logging.ErrorPairingInviteRejected}
	errorMachineIdentityMismatch     = apiError{Code: "MachineIdentityMismatch", Message: "The machine identity changed. Fleet reset is required.", Status: http.StatusConflict, logCode: logging.ErrorMachineIdentityMismatch}
	errorSessionGroupedConflict      = apiError{Code: "SessionGroupedConflict", Message: "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.", Status: http.StatusConflict, logCode: logging.ErrorSessionGroupedConflict}
	errorInternal                    = apiError{Code: "InternalError", Message: "Skíðblaðnir could not complete the request.", Status: http.StatusInternalServerError, logCode: logging.ErrorInternal}
)

type profileDTO struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Provider string `json:"provider"`
}

type characterDTO struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

type providerSessionDTO struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type agentDTO struct {
	Provider        string              `json:"provider"`
	PID             int                 `json:"pid"`
	Profile         string              `json:"profile,omitempty"`
	ProviderSession *providerSessionDTO `json:"providerSession,omitempty"`
}

type sessionDTO struct {
	TmuxID          string       `json:"tmuxId"`
	TmuxName        string       `json:"tmuxName"`
	IdentityToken   string       `json:"identityToken"`
	Character       characterDTO `json:"character"`
	LaunchProfile   string       `json:"launchProfile,omitempty"`
	Agent           *agentDTO    `json:"agent,omitempty"`
	Objective       string       `json:"objective,omitempty"`
	CWD             string       `json:"cwd,omitempty"`
	ActiveCommand   string       `json:"activeCommand,omitempty"`
	AttachedClients int          `json:"attachedClients"`
	Activity        string       `json:"activity"`
}

type machineDTO struct {
	Handle   string        `json:"handle"`
	Platform platform.Kind `json:"platform"`
}

type sessionsResponseDTO struct {
	Machine    machineDTO   `json:"machine"`
	ObservedAt string       `json:"observedAt"`
	Profiles   []profileDTO `json:"profiles"`
	Sessions   []sessionDTO `json:"sessions"`
}

type createSessionResponseDTO struct {
	ObservedAt string     `json:"observedAt"`
	Session    sessionDTO `json:"session"`
}

type pairingInviteResponseDTO struct {
	PairingInviteToken string     `json:"pairingInviteToken"`
	ExpiresAt          string     `json:"expiresAt"`
	Machine            machineDTO `json:"machine"`
}

type pairingResponseDTO struct {
	Machine machineDTO `json:"machine"`
	Bearer  string     `json:"bearer"`
}

type directoryListingRequest struct {
	Directory stringField `json:"directory"`
}

func (request *directoryListingRequest) UnmarshalJSON(encoded []byte) error {
	var members map[string]json.RawMessage
	if err := strictjson.Decode(encoded, &members); err != nil {
		return err
	}
	if len(members) != 1 || members["directory"] == nil {
		return errors.New("directory listing request does not have its exact field")
	}
	var decoded directoryListingRequest
	if err := json.Unmarshal(members["directory"], &decoded.Directory); err != nil {
		return err
	}
	*request = decoded
	return nil
}

type directoryListingResponseDTO struct {
	Machine         machineDTO                      `json:"machine"`
	Directory       string                          `json:"directory"`
	ParentDirectory *string                         `json:"parentDirectory,omitempty"`
	Children        []directoryListingEntryResponse `json:"children"`
	Omitted         bool                            `json:"omitted"`
}

type directoryListingEntryResponse struct {
	Directory string `json:"directory"`
	Kind      string `json:"kind"`
}

func mapDirectoryListing(machine machineDTO, listing workdir.Listing) (directoryListingResponseDTO, error) {
	children := listing.Children()
	mappedChildren := make([]directoryListingEntryResponse, len(children))
	for index, entry := range children {
		switch entry.Kind() {
		case workdir.Directory, workdir.SymbolicLink:
		default:
			return directoryListingResponseDTO{}, errors.New("invalid working directory entry kind")
		}
		mappedChildren[index] = directoryListingEntryResponse{
			Directory: entry.Directory().String(),
			Kind:      string(entry.Kind()),
		}
	}
	var parentDirectory *string
	if parent, present := listing.Parent().Value(); present {
		value := parent.String()
		parentDirectory = &value
	}
	switch listing.Omissions() {
	case workdir.None, workdir.Present:
	default:
		return directoryListingResponseDTO{}, errors.New("invalid working directory omission state")
	}
	return directoryListingResponseDTO{
		Machine:         machine,
		Directory:       listing.Directory().String(),
		ParentDirectory: parentDirectory,
		Children:        mappedChildren,
		Omitted:         listing.Omissions() == workdir.Present,
	}, nil
}

type createSessionRequest struct {
	CWD              stringField `json:"cwd"`
	Profile          stringField `json:"profile"`
	OptionalTmuxName stringField `json:"optionalTmuxName"`
	Objective        stringField `json:"objective"`
}

func (request *createSessionRequest) UnmarshalJSON(encoded []byte) error {
	var members map[string]json.RawMessage
	if err := strictjson.Decode(encoded, &members); err != nil {
		return err
	}
	if len(members) < 2 || len(members) > 4 || members["cwd"] == nil || members["profile"] == nil {
		return errors.New("create request does not have its exact required fields")
	}
	for member := range members {
		switch member {
		case "cwd", "profile", "optionalTmuxName", "objective":
		default:
			return errors.New("create request has an unknown field")
		}
	}
	var decoded createSessionRequest
	if err := json.Unmarshal(members["cwd"], &decoded.CWD); err != nil {
		return err
	}
	if err := json.Unmarshal(members["profile"], &decoded.Profile); err != nil {
		return err
	}
	if optionalTmuxName, present := members["optionalTmuxName"]; present {
		if err := json.Unmarshal(optionalTmuxName, &decoded.OptionalTmuxName); err != nil {
			return err
		}
	}
	if objective, present := members["objective"]; present {
		if err := json.Unmarshal(objective, &decoded.Objective); err != nil {
			return err
		}
	}
	*request = decoded
	return nil
}

type stringField struct {
	present bool
	value   string
}

func (field *stringField) UnmarshalJSON(encoded []byte) error {
	if field.present {
		return errors.New("duplicate string field")
	}
	field.present = true
	if bytes.Equal(encoded, []byte("null")) {
		return errors.New("null is not a string")
	}
	return json.Unmarshal(encoded, &field.value)
}

type killSessionRequest struct {
	TmuxName      string `json:"tmuxName"`
	IdentityToken string `json:"identityToken"`
}

type renameSessionRequest struct {
	TmuxName      stringField `json:"tmuxName"`
	NewTmuxName   stringField `json:"newTmuxName"`
	IdentityToken stringField `json:"identityToken"`
}

func (request *renameSessionRequest) UnmarshalJSON(encoded []byte) error {
	var members map[string]json.RawMessage
	if err := strictjson.Decode(encoded, &members); err != nil {
		return err
	}
	if len(members) != 3 || members["tmuxName"] == nil ||
		members["newTmuxName"] == nil || members["identityToken"] == nil {
		return errors.New("rename request does not have its exact fields")
	}
	var decoded renameSessionRequest
	if err := json.Unmarshal(members["tmuxName"], &decoded.TmuxName); err != nil {
		return err
	}
	if err := json.Unmarshal(members["newTmuxName"], &decoded.NewTmuxName); err != nil {
		return err
	}
	if err := json.Unmarshal(members["identityToken"], &decoded.IdentityToken); err != nil {
		return err
	}
	*request = decoded
	return nil
}

type pressureResponseDTO struct {
	Unsupported []pressure.Metric    `json:"unsupported"`
	Current     hostSampleDTO        `json:"current"`
	History     []pressureHistoryDTO `json:"history"`
}

type hostSampleDTO struct {
	SampledAt string             `json:"sampledAt"`
	Level     string             `json:"level"`
	Phase     pressure.Phase     `json:"phase"`
	Reasons   []string           `json:"reasons"`
	Signals   pressureSignalsDTO `json:"signals"`
	Missing   []pressure.Metric  `json:"missing"`
}

type pressureSignalsDTO struct {
	CPUPercent                          *pressureSignalDTO       `json:"cpuPercent,omitempty"`
	NormalizedLoad                      *pressureSignalDTO       `json:"normalizedLoad,omitempty"`
	MemoryAvailablePercent              *pressureSignalDTO       `json:"memoryAvailablePercent,omitempty"`
	SwapUsedPercent                     *pressureSignalDTO       `json:"swapUsedPercent,omitempty"`
	DiskAvailablePercent                *pressureSignalDTO       `json:"diskAvailablePercent,omitempty"`
	CPUPressureSomeAvg60Percent         *pressureSignalDTO       `json:"cpuPsiSomeAvg60Percent,omitempty"`
	MemoryPressureFullAvg60Percent      *pressureSignalDTO       `json:"memoryPsiFullAvg60Percent,omitempty"`
	InputOutputPressureFullAvg60Percent *pressureSignalDTO       `json:"ioPsiFullAvg60Percent,omitempty"`
	MemoryPressure                      *memoryPressureSignalDTO `json:"memoryPressure,omitempty"`
}

type pressureSignalDTO struct {
	Value float64               `json:"value"`
	State pressure.SignalStatus `json:"state"`
}

type memoryPressureSignalDTO struct {
	Value pressure.MemoryPressure `json:"value"`
	State pressure.SignalStatus   `json:"state"`
}

type pressureHistoryDTO struct {
	SampledAt string `json:"sampledAt"`
	Level     string `json:"level"`
}

func mapProfiles(profiles []agentruntime.Profile) ([]profileDTO, error) {
	validated, err := agentruntime.ValidateProfiles(profiles)
	if err != nil {
		return nil, err
	}
	mapped := make([]profileDTO, len(validated))
	for index, profile := range validated {
		mapped[index] = profileDTO{Key: string(profile.Key), Label: profile.Label, Provider: profile.Provider.String()}
	}
	return mapped, nil
}

func mapAgent(agent *agentruntime.AgentRuntime, profiles []agentruntime.Profile) (*agentDTO, error) {
	if agent == nil {
		return nil, nil
	}
	if err := agentruntime.ValidateAgentRuntime(profiles, *agent); err != nil {
		return nil, errors.New("invalid agent runtime")
	}
	mapped := &agentDTO{
		Provider: agent.Provider.String(),
		PID:      int(agent.PID),
		Profile:  string(agent.Profile),
	}
	if agent.ProviderSession != nil {
		id := agent.ProviderSession.ID()
		name := agent.ProviderSession.Name()
		mapped.ProviderSession = &providerSessionDTO{ID: id, Name: name}
	}
	return mapped, nil
}

func mapSession(session sessions.Session, profiles []agentruntime.Profile) (sessionDTO, error) {
	agent, err := mapAgent(session.Agent, profiles)
	if err != nil {
		return sessionDTO{}, err
	}
	activity := ""
	switch session.Activity {
	case sessions.SessionActivityActive, sessions.SessionActivityQuiet:
		activity = string(session.Activity)
	default:
		return sessionDTO{}, errors.New("invalid session activity")
	}
	if session.LaunchProfile != "" {
		matches := 0
		for _, profile := range profiles {
			if profile.Key == session.LaunchProfile {
				matches++
			}
		}
		if matches != 1 {
			return sessionDTO{}, errors.New("invalid session launch profile")
		}
	}
	card := sessionDTO{
		TmuxID:          session.TmuxID,
		TmuxName:        session.TmuxName,
		IdentityToken:   session.IdentityToken,
		Character:       characterDTO{Key: session.Character.Key, DisplayName: session.Character.DisplayName},
		LaunchProfile:   string(session.LaunchProfile),
		Agent:           agent,
		Objective:       session.Objective,
		CWD:             session.CWD,
		ActiveCommand:   session.ActiveCommand,
		AttachedClients: session.AttachedClients,
		Activity:        activity,
	}
	if card.TmuxID == "" || card.TmuxName == "" || card.IdentityToken == "" ||
		card.Character.Key == "" || card.Character.DisplayName == "" || card.AttachedClients < 0 {
		return sessionDTO{}, errors.New("invalid required session facts")
	}
	return card, nil
}

func mapSessionsResponse(
	machine machineDTO,
	inventory sessions.Inventory,
	configuredProfiles []agentruntime.Profile,
) (sessionsResponseDTO, error) {
	encodedObservedAt, err := formatProjectionInstant(inventory.ObservedAt)
	if err != nil {
		return sessionsResponseDTO{}, errors.New("invalid inventory observation time")
	}
	profiles, err := mapProfiles(configuredProfiles)
	if err != nil {
		return sessionsResponseDTO{}, err
	}
	cards := make([]sessionDTO, len(inventory.Sessions))
	for index, session := range inventory.Sessions {
		card, mapErr := mapSession(session, configuredProfiles)
		if mapErr != nil {
			return sessionsResponseDTO{}, mapErr
		}
		cards[index] = card
	}
	sort.Slice(cards, func(left, right int) bool {
		if cards[left].TmuxName != cards[right].TmuxName {
			return cards[left].TmuxName < cards[right].TmuxName
		}
		return cards[left].TmuxID < cards[right].TmuxID
	})
	return sessionsResponseDTO{
		Machine:    machine,
		ObservedAt: encodedObservedAt,
		Profiles:   profiles,
		Sessions:   cards,
	}, nil
}

func mapCreateSessionResponse(
	observed sessions.ObservedSession,
	profiles []agentruntime.Profile,
) (createSessionResponseDTO, error) {
	encodedObservedAt, err := formatProjectionInstant(observed.ObservedAt)
	if err != nil {
		return createSessionResponseDTO{}, errors.New("invalid session observation time")
	}
	card, err := mapSession(observed.Session, profiles)
	if err != nil {
		return createSessionResponseDTO{}, err
	}
	return createSessionResponseDTO{
		ObservedAt: encodedObservedAt,
		Session:    card,
	}, nil
}

func formatProjectionInstant(value time.Time) (string, error) {
	if !sessions.ValidProjectionInstant(value) {
		return "", errors.New("projection instant is outside canonical RFC 3339")
	}
	return value.UTC().Format(time.RFC3339Nano), nil
}

func mapHostSample(sample pressure.Sample, unsupported map[pressure.Metric]struct{}) (hostSampleDTO, error) {
	level, err := mapPressureLevel(sample.Status)
	if err != nil {
		return hostSampleDTO{}, err
	}
	reasons := make([]string, len(sample.Reasons))
	for index, reason := range sample.Reasons {
		switch reason {
		case pressure.ReasonMemory, pressure.ReasonDisk, pressure.ReasonLoad, pressure.ReasonCPUPSI, pressure.ReasonMemoryPSI, pressure.ReasonIOPSI:
			reasons[index] = string(reason)
		default:
			return hostSampleDTO{}, errors.New("invalid pressure reason")
		}
	}
	mapped := hostSampleDTO{
		SampledAt: sample.ObservedAt.Format(time.RFC3339Nano),
		Level:     level,
		Phase:     sample.Phase,
		Reasons:   reasons,
		Missing:   []pressure.Metric{},
	}
	switch sample.Phase {
	case pressure.PhaseSteady, pressure.PhaseRecovering:
	default:
		return hostSampleDTO{}, errors.New("invalid pressure phase")
	}
	metrics := [...]struct {
		metric        pressure.Metric
		signal        pressure.Signal
		destination   **pressureSignalDTO
		maximum       float64
		informational bool
	}{
		{pressure.MetricCPUPercent, sample.CPUPercent, &mapped.Signals.CPUPercent, 100, true},
		{pressure.MetricLoadNormalized, sample.LoadNormalized, &mapped.Signals.NormalizedLoad, math.MaxFloat64, false},
		{pressure.MetricMemoryAvailablePercent, sample.MemoryAvailablePercent, &mapped.Signals.MemoryAvailablePercent, 100, false},
		{pressure.MetricSwapUsedPercent, sample.SwapUsedPercent, &mapped.Signals.SwapUsedPercent, 100, true},
		{pressure.MetricDiskAvailablePercent, sample.DiskAvailablePercent, &mapped.Signals.DiskAvailablePercent, 100, false},
		{pressure.MetricCPUPressureSomeAvg60, sample.CPUPressureSomeAvg60, &mapped.Signals.CPUPressureSomeAvg60Percent, 100, false},
		{pressure.MetricMemoryPressureFullAvg60, sample.MemoryPressureFullAvg60, &mapped.Signals.MemoryPressureFullAvg60Percent, 100, false},
		{pressure.MetricInputOutputPressureFullAvg60, sample.InputOutputPressureFullAvg60, &mapped.Signals.InputOutputPressureFullAvg60Percent, 100, false},
	}
	for _, metric := range metrics {
		value, known := metric.signal.Value()
		if _, excluded := unsupported[metric.metric]; excluded {
			if err := partitionPressureMetric(metric.metric, known, true, &mapped.Missing); err != nil {
				return hostSampleDTO{}, err
			}
			continue
		}
		if err := partitionPressureMetric(metric.metric, known, false, &mapped.Missing); err != nil {
			return hostSampleDTO{}, err
		}
		if !known {
			continue
		}
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > metric.maximum {
			return hostSampleDTO{}, errors.New("invalid pressure metric")
		}
		if !validSignalStatus(metric.signal.Status, metric.informational) {
			return hostSampleDTO{}, errors.New("invalid pressure signal status")
		}
		*metric.destination = &pressureSignalDTO{Value: value, State: metric.signal.Status}
	}
	memoryPressure, known := sample.MemoryPressure.Value()
	_, memoryPressureUnsupported := unsupported[pressure.MetricMemoryPressure]
	if err := partitionPressureMetric(pressure.MetricMemoryPressure, known, memoryPressureUnsupported, &mapped.Missing); err != nil {
		return hostSampleDTO{}, err
	}
	if known && !memoryPressureUnsupported {
		switch {
		case memoryPressure == pressure.MemoryPressureNormal && sample.MemoryPressure.Status == pressure.SignalStatusNormal,
			memoryPressure == pressure.MemoryPressureWarning && sample.MemoryPressure.Status == pressure.SignalStatusWarm,
			memoryPressure == pressure.MemoryPressureCritical && sample.MemoryPressure.Status == pressure.SignalStatusHot:
		default:
			return hostSampleDTO{}, errors.New("invalid memory pressure signal")
		}
		mapped.Signals.MemoryPressure = &memoryPressureSignalDTO{Value: memoryPressure, State: sample.MemoryPressure.Status}
	}
	sort.Slice(mapped.Missing, func(left, right int) bool {
		return mapped.Missing[left] < mapped.Missing[right]
	})
	return mapped, nil
}

func mapPressureHistory(sample pressure.Sample) (pressureHistoryDTO, error) {
	level, err := mapPressureLevel(sample.Status)
	if err != nil {
		return pressureHistoryDTO{}, err
	}
	return pressureHistoryDTO{SampledAt: sample.ObservedAt.Format(time.RFC3339Nano), Level: level}, nil
}

func mapPressureLevel(status pressure.Status) (string, error) {
	switch status {
	case pressure.StatusNormal, pressure.StatusWarm, pressure.StatusHot, pressure.StatusUnknown:
		return string(status), nil
	default:
		return "", errors.New("invalid pressure status")
	}
}

func validSignalStatus(status pressure.SignalStatus, informational bool) bool {
	if informational {
		return status == pressure.SignalStatusInformational
	}
	return status == pressure.SignalStatusNormal || status == pressure.SignalStatusWarm || status == pressure.SignalStatusHot
}

func partitionPressureMetric(metric pressure.Metric, observed, unsupported bool, missing *[]pressure.Metric) error {
	if unsupported {
		if observed {
			return errors.New("unsupported pressure metric was observed")
		}
		return nil
	}
	if !observed {
		*missing = append(*missing, metric)
	}
	return nil
}

// mapUnsupportedMetrics projects one platform's declared pressure capability
// onto the closed wire union. internal/pressure owns the sorted, unique
// canonical set and establishes that invariant when it constructs the policy,
// so the gateway only has to close the enum once, when it is composed.
func mapUnsupportedMetrics(metrics []pressure.Metric) ([]pressure.Metric, map[pressure.Metric]struct{}) {
	mapped := make([]pressure.Metric, 0, len(metrics))
	set := make(map[pressure.Metric]struct{}, len(metrics))
	for _, metric := range metrics {
		switch metric {
		case pressure.MetricCPUPercent,
			pressure.MetricLoadNormalized,
			pressure.MetricMemoryAvailablePercent,
			pressure.MetricSwapUsedPercent,
			pressure.MetricDiskAvailablePercent,
			pressure.MetricCPUPressureSomeAvg60,
			pressure.MetricMemoryPressureFullAvg60,
			pressure.MetricInputOutputPressureFullAvg60,
			pressure.MetricMemoryPressure:
		default:
			panic("declared pressure capability names a metric outside the closed wire union") // justify-defect: the host policy and this same-system closed schema ship together; an unknown enum value must never reach a client.
		}
		mapped = append(mapped, metric)
		set[metric] = struct{}{}
	}
	return mapped, set
}

func pressureLogValues(sample pressure.Sample) (logging.PressureLevel, []logging.PressureReason, error) {
	level := logging.PressureLevel(sample.Status)
	switch level {
	case logging.PressureNormal, logging.PressureWarm, logging.PressureHot, logging.PressureUnknown:
	default:
		return "", nil, errors.New("invalid pressure log level")
	}
	reasons := make([]logging.PressureReason, len(sample.Reasons))
	for index, reason := range sample.Reasons {
		mapped := logging.PressureReason(reason)
		switch mapped {
		case logging.ReasonMemory, logging.ReasonDisk, logging.ReasonLoad, logging.ReasonCPUPSI, logging.ReasonMemoryPSI, logging.ReasonIOPSI:
			reasons[index] = mapped
		default:
			return "", nil, errors.New("invalid pressure log reason")
		}
	}
	return level, reasons, nil
}
