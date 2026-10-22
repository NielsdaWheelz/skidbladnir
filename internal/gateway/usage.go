package gateway

import (
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/quota"
)

func (gateway *Gateway) readProfileUsage(writer http.ResponseWriter, request *http.Request) {
	observed, err := quota.Observe(request.Context(), gateway.sessions)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	// make, not append: quota returns nil without a codex profile and the wire needs [].
	document := api.ProfileUsageReport{Machine: gateway.apiMachine(), ObservedAt: observed.ObservedAt.UTC(), Profiles: make([]api.ProfileUsage, len(observed.Profiles))}
	for index, usage := range observed.Profiles {
		readState := "ok"
		if usage.Report == nil {
			readState = "unavailable"
		}
		document.Profiles[index] = api.ProfileUsage{Profile: apiProfile(usage.Profile), ReadState: readState, Report: usage.Report}
	}
	writeJSON(writer, http.StatusOK, document, &errorInternal)
}
