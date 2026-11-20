package gateway

import (
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/quota"
)

func (gateway *Gateway) readProfileUsage(writer http.ResponseWriter, request *http.Request) {
	observed, err := quota.Observe(request.Context(), gateway.sessions, gateway.stateDir, request.URL.Path == "/v1/claude-usage")
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	if observed.HistoryUnavailable {
		gateway.logger.Warn("Quota.ClaudeHistoryUnavailable")
	}
	// The wire needs an empty array when the host declares no provider profiles.
	document := api.ProfileUsageReport{
		Machine: gateway.apiMachine(), ObservedAt: observed.ObservedAt.UTC(),
		Profiles: make([]api.ProfileUsage, len(observed.Profiles)), OtherClaudeAccounts: observed.OtherClaudeAccounts,
	}
	for index, usage := range observed.Profiles {
		readState := "ok"
		if usage.Report == nil && usage.Claude == nil {
			readState = "unavailable"
		}
		document.Profiles[index] = api.ProfileUsage{Profile: apiProfile(usage.Profile), ReadState: readState, Report: usage.Report, Claude: usage.Claude}
	}
	writeJSON(writer, http.StatusOK, document, &errorInternal)
}
