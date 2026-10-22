package gateway

import (
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/quota"
)

type profileUsageDTO struct {
	profileDTO
	ReadState string        `json:"readState"`
	Report    *quota.Report `json:"report,omitempty"`
}

func (gateway *Gateway) readProfileUsage(writer http.ResponseWriter, request *http.Request) {
	observed, err := quota.Observe(request.Context(), gateway.sessions)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	configured := make([]agentruntime.Profile, len(observed.Profiles))
	for index, usage := range observed.Profiles {
		configured[index] = usage.Profile
	}
	profiles := mapProfiles(configured)
	mapped := make([]profileUsageDTO, len(profiles))
	for index, profile := range profiles {
		report := observed.Profiles[index].Report
		readState := "ok"
		if report == nil {
			readState = "unavailable"
		}
		mapped[index] = profileUsageDTO{profileDTO: profile, ReadState: readState, Report: report}
	}
	stamp, err := formatProjectionInstant(observed.ObservedAt)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	writeJSON(writer, http.StatusOK, struct {
		Machine    machineDTO        `json:"machine"`
		ObservedAt string            `json:"observedAt"`
		Profiles   []profileUsageDTO `json:"profiles"`
	}{gateway.machineDTO(), stamp, mapped}, &errorInternal)
}
