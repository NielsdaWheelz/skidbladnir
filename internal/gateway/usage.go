package gateway

import (
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
)

func (gateway *Gateway) readProfileUsage(writer http.ResponseWriter, request *http.Request) {
	observed, err := gateway.quota.Observe(request.Context(), request.URL.Path == "/v1/claude-usage")
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	if observed.HistoryUnavailable {
		gateway.logger.Warn("Quota.ClaudeHistoryUnavailable")
	}
	// The wire needs an empty array when the host declares no provider profiles.
	document := api.ProfileUsageReport{
		Machine: gateway.apiMachine(), ObservedAt: observed.ObservedAt,
		Profiles: make([]api.ProfileUsage, len(observed.Profiles)), ClaudeAccounts: observed.ClaudeAccounts,
	}
	for index, usage := range observed.Profiles {
		readState := usage.Claude.ReadState
		if usage.Report != nil {
			readState = "ok"
		} else if readState == "" {
			readState = "unavailable"
		}
		document.Profiles[index] = api.ProfileUsage{
			Profile: apiProfile(usage.Profile), ReadState: readState, Report: usage.Report, ClaudeAccount: usage.Claude.Reading.Account,
		}
	}
	writeJSON(writer, http.StatusOK, document, &errorInternal)
}
