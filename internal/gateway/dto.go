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
	errorMachineIdentityMismatch     = apiError{Code: "MachineIdentityMismatch", Message: "The machine identity changed. Provisioning repair is required.", Status: http.StatusConflict, logCode: logging.ErrorMachineIdentityMismatch}
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
	Unsupported []pressure.Metric `json:"unsupported"`
	Current     hostSampleDTO     `json:"current"`
	History     []hostSampleDTO   `json:"history"`
}

type hostSampleDTO struct {
	SampledAt string             `json:"sampledAt"`
	Level     string             `json:"level"`
	Reasons   []string           `json:"reasons"`
	Metrics   pressureMetricsDTO `json:"metrics"`
	Missing   []pressure.Metric  `json:"missing"`
}

type pressureMetricsDTO struct {
	CPUPercent                          *float64                 `json:"cpuPercent,omitempty"`
	NormalizedLoad                      *float64                 `json:"normalizedLoad,omitempty"`
	MemoryAvailablePercent              *float64                 `json:"memoryAvailablePercent,omitempty"`
	SwapUsedPercent                     *float64                 `json:"swapUsedPercent,omitempty"`
	DiskAvailablePercent                *float64                 `json:"diskAvailablePercent,omitempty"`
	CPUPressureSomeAvg60Percent         *float64                 `json:"cpuPsiSomeAvg60Percent,omitempty"`
	MemoryPressureFullAvg60Percent      *float64                 `json:"memoryPsiFullAvg60Percent,omitempty"`
	InputOutputPressureFullAvg60Percent *float64                 `json:"ioPsiFullAvg60Percent,omitempty"`
	MemoryPressure                      *pressure.MemoryPressure `json:"memoryPressure,omitempty"`
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
	level := string(sample.Status)
	switch sample.Status {
	case pressure.StatusNormal, pressure.StatusWarm, pressure.StatusHot, pressure.StatusUnknown:
	default:
		return hostSampleDTO{}, errors.New("invalid pressure status")
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
		Reasons:   reasons,
		Missing:   []pressure.Metric{},
	}
	metrics := [...]struct {
		metric      pressure.Metric
		signal      pressure.Signal
		destination **float64
		maximum     float64
	}{
		{pressure.MetricCPUPercent, sample.CPUPercent, &mapped.Metrics.CPUPercent, 100},
		{pressure.MetricLoadNormalized, sample.LoadNormalized, &mapped.Metrics.NormalizedLoad, math.MaxFloat64},
		{pressure.MetricMemoryAvailablePercent, sample.MemoryAvailablePercent, &mapped.Metrics.MemoryAvailablePercent, 100},
		{pressure.MetricSwapUsedPercent, sample.SwapUsedPercent, &mapped.Metrics.SwapUsedPercent, 100},
		{pressure.MetricDiskAvailablePercent, sample.DiskAvailablePercent, &mapped.Metrics.DiskAvailablePercent, 100},
		{pressure.MetricCPUPressureSomeAvg60, sample.CPUPressureSomeAvg60, &mapped.Metrics.CPUPressureSomeAvg60Percent, 100},
		{pressure.MetricMemoryPressureFullAvg60, sample.MemoryPressureFullAvg60, &mapped.Metrics.MemoryPressureFullAvg60Percent, 100},
		{pressure.MetricInputOutputPressureFullAvg60, sample.InputOutputPressureFullAvg60, &mapped.Metrics.InputOutputPressureFullAvg60Percent, 100},
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
		valueCopy := value
		*metric.destination = &valueCopy
	}
	memoryPressure, known := sample.MemoryPressure.Value()
	_, memoryPressureUnsupported := unsupported[pressure.MetricMemoryPressure]
	if err := partitionPressureMetric(pressure.MetricMemoryPressure, known, memoryPressureUnsupported, &mapped.Missing); err != nil {
		return hostSampleDTO{}, err
	}
	if known && !memoryPressureUnsupported {
		switch memoryPressure {
		case pressure.MemoryPressureNormal, pressure.MemoryPressureWarning, pressure.MemoryPressureCritical:
		default:
			return hostSampleDTO{}, errors.New("invalid memory pressure")
		}
		memoryPressureCopy := memoryPressure
		mapped.Metrics.MemoryPressure = &memoryPressureCopy
	}
	sort.Slice(mapped.Missing, func(left, right int) bool {
		return mapped.Missing[left] < mapped.Missing[right]
	})
	return mapped, nil
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
