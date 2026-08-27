package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
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
	errorProfileUnknown              = apiError{Code: "ProfileUnknown", Message: "Choose an available profile.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorProfileUnknown}
	errorSessionNameInvalid          = apiError{Code: "SessionNameInvalid", Message: "Use 1–64 letters, numbers, underscores, or hyphens, beginning with a letter or number.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorSessionNameInvalid}
	errorSessionNameConflict         = apiError{Code: "SessionNameConflict", Message: "A session with that name already exists.", Status: http.StatusConflict, logCode: logging.ErrorSessionNameConflict}
	errorObjectiveInvalid            = apiError{Code: "ObjectiveInvalid", Message: "Use 1–240 characters without terminal controls.", Status: http.StatusUnprocessableEntity, logCode: logging.ErrorObjectiveInvalid}
	errorSessionNotFound             = apiError{Code: "SessionNotFound", Message: "That session no longer exists.", Status: http.StatusNotFound, logCode: logging.ErrorSessionNotFound}
	errorSessionIdentityMismatch     = apiError{Code: "SessionIdentityMismatch", Message: "The session changed. Refresh before killing it.", Status: http.StatusConflict, logCode: logging.ErrorSessionIdentityMismatch}
	errorPairingInviteRejected       = apiError{Code: "PairingInviteRejected", Message: "This fleet invite is invalid, expired, or already used.", Status: http.StatusUnauthorized, logCode: logging.ErrorPairingInviteRejected}
	errorMachineIdentityMismatch     = apiError{Code: "MachineIdentityMismatch", Message: "The machine identity changed. Fleet reset is required.", Status: http.StatusConflict, logCode: logging.ErrorMachineIdentityMismatch}
	errorSessionGroupedConflict      = apiError{Code: "SessionGroupedConflict", Message: "This session shares its work with another non-phone tmux session. Resolve the group in tmux before killing it.", Status: http.StatusConflict, logCode: logging.ErrorSessionGroupedConflict}
	errorInternal                    = apiError{Code: "InternalError", Message: "Skíðblaðnir could not complete the request.", Status: http.StatusInternalServerError, logCode: logging.ErrorInternal}
)

type profileDTO struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type statusDTO struct {
	Kind     string `json:"kind"`
	Signal   string `json:"signal"`
	SignalAt string `json:"signalAt"`
}

type characterDTO struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

type sessionDTO struct {
	ID              string       `json:"id"`
	TmuxName        string       `json:"tmuxName"`
	IdentityToken   string       `json:"identityToken"`
	Character       characterDTO `json:"character"`
	Profile         string       `json:"profile,omitempty"`
	Objective       string       `json:"objective,omitempty"`
	CWD             string       `json:"cwd,omitempty"`
	ActiveCommand   string       `json:"activeCommand,omitempty"`
	AttachedClients int          `json:"attachedClients"`
	Attention       bool         `json:"attention"`
	Status          statusDTO    `json:"status"`
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

type pairingInviteResponseDTO struct {
	PairingInviteToken string     `json:"pairingInviteToken"`
	ExpiresAt          string     `json:"expiresAt"`
	Machine            machineDTO `json:"machine"`
}

type pairingResponseDTO struct {
	Machine machineDTO `json:"machine"`
	Bearer  string     `json:"bearer"`
}

type createSessionRequest struct {
	CWD              stringField `json:"cwd"`
	Profile          stringField `json:"profile"`
	OptionalTmuxName stringField `json:"optionalTmuxName"`
	Objective        stringField `json:"objective"`
}

type stringField struct {
	present bool
	value   string
}

func (field *stringField) UnmarshalJSON(encoded []byte) error {
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

func mapProfiles(profiles []sessions.Profile) ([]profileDTO, error) {
	mapped := make([]profileDTO, len(profiles))
	for index, profile := range profiles {
		if profile.Key == "" || profile.Label == "" {
			return nil, errors.New("invalid configured profile")
		}
		mapped[index] = profileDTO{Key: profile.Key, Label: profile.Label}
	}
	return mapped, nil
}

func mapSession(session sessions.Session, observedAt time.Time) (sessionDTO, error) {
	kind := ""
	switch session.Status.Kind {
	case sessions.StatusWorking:
		kind = "Working"
	case sessions.StatusRunning:
		kind = "Running"
	case sessions.StatusIdle:
		kind = "Idle"
	case sessions.StatusShell:
		kind = "Shell"
	case sessions.StatusUnknown:
		kind = "Unknown"
	default:
		return sessionDTO{}, errors.New("invalid session status")
	}
	signal := ""
	switch session.Status.Signal {
	case sessions.StatusSignalLifecycle:
		signal = "Lifecycle"
	case sessions.StatusSignalProcess:
		signal = "Process"
	case sessions.StatusSignalPollFailure:
		signal = "PollFailure"
	default:
		return sessionDTO{}, errors.New("invalid session status signal")
	}
	if session.Status.SignalAt.IsZero() || session.Status.SignalAt.After(observedAt) {
		return sessionDTO{}, errors.New("invalid session signal time")
	}
	card := sessionDTO{
		ID:              session.ID,
		TmuxName:        session.TmuxName,
		IdentityToken:   session.IdentityToken,
		Character:       characterDTO{Key: session.Character.Key, DisplayName: session.Character.DisplayName},
		Profile:         session.Profile,
		Objective:       session.Objective,
		CWD:             session.CWD,
		ActiveCommand:   session.ActiveCommand,
		AttachedClients: session.AttachedClients,
		Attention:       session.Attention,
		Status: statusDTO{
			Kind:     kind,
			Signal:   signal,
			SignalAt: session.Status.SignalAt.UTC().Format(time.RFC3339Nano),
		},
	}
	if card.IdentityToken == "" {
		return sessionDTO{}, errors.New("missing session identity token")
	}
	return card, nil
}

func sessionPriority(kind string) int {
	switch kind {
	case "Working":
		return 0
	case "Running":
		return 1
	case "Idle":
		return 2
	case "Shell":
		return 3
	case "Unknown":
		return 4
	default:
		panic("invalid session status kind") // justify-defect: mapSession closes the status universe.
	}
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
