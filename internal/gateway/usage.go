package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

type profileUsageDTO struct {
	profileDTO
	Source    string                    `json:"source"`
	ReadState string                    `json:"readState"`
	Report    *agentcontrol.UsageReport `json:"report,omitempty"`
}

func (gateway *Gateway) readProfileUsage(writer http.ResponseWriter, request *http.Request) {
	if !requireEmptyRequest(request) {
		writeError(writer, errorInvalidRequest)
		return
	}
	observed, err := gateway.agents.ProfileUsage(request.Context())
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	configured := make([]agentruntime.Profile, len(observed.Profiles))
	for index, usage := range observed.Profiles {
		configured[index] = usage.Profile
	}
	profiles, err := mapProfiles(configured)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	mapped := make([]profileUsageDTO, len(profiles))
	for index, profile := range profiles {
		usage := observed.Profiles[index]
		mapped[index] = profileUsageDTO{profileDTO: profile, Source: usage.Source, ReadState: usage.ReadState, Report: usage.Report}
	}
	stamp, err := formatProjectionInstant(observed.ObservedAt)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	var encoded boundedJSONBuffer
	if json.NewEncoder(&encoded).Encode(struct {
		Machine    machineDTO        `json:"machine"`
		ObservedAt string            `json:"observedAt"`
		Profiles   []profileUsageDTO `json:"profiles"`
	}{gateway.machineDTO(), stamp, mapped}) != nil {
		writeError(writer, errorInternal)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(encoded.buffer.Bytes()) // justify-ignore-error: a client disconnect after headers cannot be repaired.
}
