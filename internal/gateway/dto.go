package gateway

import (
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalcontext"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

type apiError struct {
	Target   *createdTargetDTO `json:"target,omitempty"`
	Dispatch string            `json:"dispatch,omitempty"`
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Status   int               `json:"-"`
}

type createdTargetDTO struct {
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

var (
	errorUnauthenticated             = admissionError(auth.AdmissionUnauthenticated)
	errorInvalidRequest              = admissionError(auth.AdmissionInvalid)
	errorRequestTooLarge             = admissionError(auth.AdmissionTooLarge)
	errorWorkingDirectoryInvalid     = apiError{Code: "WorkingDirectoryInvalid", Message: "Choose a valid working directory.", Status: http.StatusUnprocessableEntity}
	errorWorkingDirectoryUnavailable = apiError{Code: "WorkingDirectoryUnavailable", Message: "That directory does not exist or cannot be opened.", Status: http.StatusUnprocessableEntity}
	errorDirectoryListingUnavailable = apiError{Code: "DirectoryListingUnavailable", Message: "This directory cannot be browsed. Enter the path instead.", Status: http.StatusUnprocessableEntity}
	errorDirectoryListingTooLarge    = apiError{Code: "DirectoryListingTooLarge", Message: "This directory has too many folders to show. Enter the path instead.", Status: http.StatusUnprocessableEntity}
	errorDirectorySearchUnavailable  = apiError{Code: "DirectorySearchUnavailable", Message: "Directory search is unavailable on this machine.", Status: http.StatusUnprocessableEntity}
	errorDirectorySearchTooLarge     = apiError{Code: "DirectorySearchTooLarge", Message: "Too many directory search results. Narrow the search.", Status: http.StatusUnprocessableEntity}
	errorTerminalContextUnavailable  = apiError{Code: "TerminalContextUnavailable", Message: "Remote context is unavailable.", Status: http.StatusNotFound}
	errorProfileUnknown              = apiError{Code: "ProfileUnknown", Message: "Choose an available profile.", Status: http.StatusUnprocessableEntity}
	errorSessionNameInvalid          = apiError{Code: "SessionNameInvalid", Message: "use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number.", Status: http.StatusUnprocessableEntity}
	errorSessionNameChanged          = apiError{Code: "SessionNameChanged", Message: "the session name changed. review and save again.", Status: http.StatusConflict}
	errorSessionNameConflict         = apiError{Code: "SessionNameConflict", Message: "another session on this machine uses that name.", Status: http.StatusConflict}
	errorObjectiveInvalid            = apiError{Code: "ObjectiveInvalid", Message: "Use 1–240 characters without terminal controls.", Status: http.StatusUnprocessableEntity}
	errorGroupInvalid                = apiError{Code: "GroupInvalid", Message: group.ErrInvalid.Error(), Status: http.StatusUnprocessableEntity}
	errorSessionNotFound             = apiError{Code: "SessionNotFound", Message: "That session no longer exists.", Status: http.StatusNotFound}
	errorSessionIdentityMismatch     = apiError{Code: "SessionIdentityMismatch", Message: "The session changed. Refresh and try again.", Status: http.StatusConflict}
	errorMachineIdentityMismatch     = admissionError(auth.AdmissionMachineMismatch)
	errorInternal                    = admissionError(auth.AdmissionUnavailable)
)

func admissionError(code auth.AdmissionCode) apiError {
	status, message := code.Response()
	return apiError{Code: string(code), Message: message, Status: status}
}

type profileDTO struct {
	AccountScope string `json:"accountScope,omitempty"`
	Key          string `json:"key"`
	Label        string `json:"label"`
	Provider     string `json:"provider,omitempty"`
}

type characterDTO struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

type agentDTO struct {
	PaneID        string `json:"paneId"`
	StartIdentity string `json:"startIdentity"`
	Provider      string `json:"provider"`
	PID           int    `json:"pid"`
	Profile       string `json:"profile,omitempty"`
}

type connectionDTO struct {
	Transport string `json:"transport"`
	ID        string `json:"id,omitempty"`
}

type sessionDTO struct {
	TerminalStatus  sessions.TerminalStatus  `json:"terminalStatus"`
	TerminalUsage   *sessions.TerminalUsage  `json:"terminalUsage,omitempty"`
	TerminalFooter  *sessions.TerminalFooter `json:"terminalFooter,omitempty"`
	TmuxID          string                   `json:"tmuxId"`
	ActivePaneID    string                   `json:"activePaneId"`
	TmuxName        string                   `json:"tmuxName"`
	NameMode        sessions.NameMode        `json:"nameMode"`
	IdentityToken   string                   `json:"identityToken"`
	Character       characterDTO             `json:"character"`
	LaunchProfile   string                   `json:"launchProfile,omitempty"`
	Agent           *agentDTO                `json:"agent,omitempty"`
	Connection      *connectionDTO           `json:"connection,omitempty"`
	Objective       string                   `json:"objective,omitempty"`
	Group           string                   `json:"group,omitempty"`
	CWD             string                   `json:"cwd,omitempty"`
	ActiveCommand   string                   `json:"activeCommand,omitempty"`
	AttachedClients int                      `json:"attachedClients"`
}

type machineDTO struct {
	Handle   string        `json:"handle"`
	Platform platform.Kind `json:"platform"`
}

type sessionsResponseDTO struct {
	Machine    machineDTO   `json:"machine"`
	ObservedAt string       `json:"observedAt"`
	Recovery   recoveryDTO  `json:"recovery"`
	Profiles   []profileDTO `json:"profiles"`
	Sessions   []sessionDTO `json:"sessions"`
}

type recoveryDTO struct {
	State   sessions.RecoveryState `json:"state"`
	SavedAt string                 `json:"savedAt,omitempty"`
	Failure *recoveryFailureDTO    `json:"failure,omitempty"`
}

type recoveryFailureDTO struct {
	At    string                 `json:"at"`
	Phase sessions.RecoveryPhase `json:"phase"`
	Cause sessions.RecoveryCause `json:"cause"`
}

type createSessionResponseDTO struct {
	ObservedAt string     `json:"observedAt"`
	Session    sessionDTO `json:"session"`
}

type directoryListingRequest struct {
	Directory string `json:"directory"`
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

func mapDirectoryListing(machine machineDTO, listing workdir.Listing) directoryListingResponseDTO {
	children := listing.Children()
	mappedChildren := make([]directoryListingEntryResponse, len(children))
	for index, entry := range children {
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
	return directoryListingResponseDTO{
		Machine:         machine,
		Directory:       listing.Directory().String(),
		ParentDirectory: parentDirectory,
		Children:        mappedChildren,
		Omitted:         listing.Omissions() == workdir.Present,
	}
}

// Request members decode null as absent. A pointer marks a member whose
// absence answers differently from its empty value.
type createSessionRequest struct {
	Kind             sessions.LaunchKind `json:"kind"`
	CWD              *string             `json:"cwd"`
	Profile          *string             `json:"profile"`
	OptionalTmuxName *string             `json:"optionalTmuxName"`
	Objective        *string             `json:"objective"`
	Group            *string             `json:"group"`
}

// identityRequest addresses a session lifetime: shell creation and deletion.
type identityRequest struct {
	IdentityToken string `json:"identityToken"`
}

type namingRequest struct {
	Mode string  `json:"mode"`
	Name *string `json:"name"`
}

// valid reports whether the naming has its mode's shape: automatic has no name, manual has one.
func (field namingRequest) valid() bool {
	return field.Mode == "automatic" && field.Name == nil || field.Mode == "manual" && field.Name != nil
}

func (field namingRequest) naming() sessions.Naming {
	return sessions.Naming{Mode: sessions.NameMode(field.Mode), Name: valueOf(field.Name)}
}

type renameSessionRequest struct {
	ExpectedNaming *namingRequest `json:"expectedNaming"`
	Naming         *namingRequest `json:"naming"`
	IdentityToken  string         `json:"identityToken"`
}

type setSessionGroupRequest struct {
	IdentityToken string  `json:"identityToken"`
	Group         *string `json:"group"`
}

// valueOf reads an optional member; absent is empty.
func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// digitsAfter reports whether value is prefix followed by one or more ASCII digits, the shape
// of tmux session ($N) and pane (%N) ids.
func digitsAfter(value string, prefix byte) bool {
	if len(value) < 2 || value[0] != prefix {
		return false
	}
	for _, character := range value[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
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
		// justify-ignore-error: absent account home omits this profile's quota scope without hiding its terminal launch.
		if scope, err := agentruntime.AccountScope(profile); err == nil {
			mapped[index].AccountScope = scope
		}
	}
	return mapped, nil
}

func mapAgent(agent *agentruntime.AgentRuntime, profiles []agentruntime.Profile) (*agentDTO, error) {
	if agent == nil {
		return nil, nil
	}
	if err := agentruntime.ValidateAgentRuntime(profiles, *agent); err != nil || agent.PaneID == "" || agent.StartIdentity == "" {
		return nil, errors.New("invalid agent runtime")
	}
	mapped := &agentDTO{
		PaneID: agent.PaneID, StartIdentity: string(agent.StartIdentity),
		Provider: agent.Provider.String(),
		PID:      int(agent.PID),
		Profile:  string(agent.Profile),
	}
	return mapped, nil
}

func mapSession(session sessions.Session, profiles []agentruntime.Profile) (sessionDTO, error) {
	agent, err := mapAgent(session.Agent, profiles)
	if err != nil {
		return sessionDTO{}, err
	}
	var connection *connectionDTO
	if session.Connection != nil {
		if agent != nil || session.CWD != "" || session.Connection.Transport != "ssh" && session.Connection.Transport != "mosh" || session.Connection.ID != "" && !terminalcontext.ValidConnectionID(session.Connection.ID) {
			return sessionDTO{}, errors.New("invalid session connection")
		}
		connection = &connectionDTO{Transport: session.Connection.Transport, ID: session.Connection.ID}
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
		ActivePaneID:    session.ActivePaneID,
		TmuxName:        session.TmuxName,
		NameMode:        session.NameMode,
		IdentityToken:   session.IdentityToken,
		Character:       characterDTO{Key: session.Character.Key, DisplayName: session.Character.DisplayName},
		LaunchProfile:   string(session.LaunchProfile),
		Agent:           agent,
		TerminalStatus:  session.TerminalStatus,
		TerminalUsage:   session.TerminalUsage,
		TerminalFooter:  session.TerminalFooter,
		Connection:      connection,
		Objective:       session.Objective,
		Group:           session.Group.String(),
		CWD:             session.CWD,
		ActiveCommand:   session.ActiveCommand,
		AttachedClients: session.AttachedClients,
	}
	if !card.TerminalStatus.Valid() || card.ActivePaneID == "" || agent != nil && agent.PaneID != card.ActivePaneID || card.TmuxID == "" || card.TmuxName == "" || card.NameMode != sessions.NameAutomatic && card.NameMode != sessions.NameManual || card.IdentityToken == "" ||
		card.Character.Key == "" || card.Character.DisplayName == "" || card.AttachedClients < 0 {
		return sessionDTO{}, errors.New("invalid required session facts")
	}
	if card.TerminalUsage != nil && (!card.TerminalUsage.ValidFor(card.TerminalStatus) || agent == nil || agent.Provider != agentruntime.ProviderClaude.String() && agent.Provider != agentruntime.ProviderCodex.String() || connection != nil) {
		return sessionDTO{}, errors.New("invalid terminal usage attachment")
	}
	if card.TerminalFooter != nil && (!card.TerminalFooter.ValidFor(card.TerminalStatus) || agent == nil || connection != nil) {
		return sessionDTO{}, errors.New("invalid terminal footer attachment")
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
	if !inventory.Recovery.Valid() {
		panic("inventory contains an invalid workspace recovery fact") // justify-defect: sessions.Manager owns the closed recovery union before returning inventory.
	}
	recovery := recoveryDTO{State: inventory.Recovery.State}
	if inventory.Recovery.SavedAt != nil {
		recovery.SavedAt = inventory.Recovery.SavedAt.UTC().Format(time.RFC3339Nano)
	}
	if failure := inventory.Recovery.Failure; failure != nil {
		recovery.Failure = &recoveryFailureDTO{
			At:    failure.At.UTC().Format(time.RFC3339Nano),
			Phase: failure.Phase,
			Cause: failure.Cause,
		}
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
		leftID, _ := strconv.ParseUint(strings.TrimPrefix(cards[left].TmuxID, "$"), 10, 64)
		rightID, _ := strconv.ParseUint(strings.TrimPrefix(cards[right].TmuxID, "$"), 10, 64)
		if leftID != rightID {
			return leftID < rightID
		}
		return cards[left].TmuxID < cards[right].TmuxID
	})
	return sessionsResponseDTO{
		Machine:    machine,
		ObservedAt: encodedObservedAt,
		Recovery:   recovery,
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
