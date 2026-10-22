package gateway

import (
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/terminalcontext"
)

type remoteAgentDTO struct {
	Provider string `json:"provider"`
	Profile  string `json:"profile,omitempty"`
}

type terminalContextDTO struct {
	ObservedAt string          `json:"observedAt"`
	CWD        string          `json:"cwd,omitempty"`
	Agent      *remoteAgentDTO `json:"agent,omitempty"`
	Connection *connectionDTO  `json:"connection,omitempty"`
}

func (gateway *Gateway) readTerminalContext(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("connectionId")
	if !terminalcontext.ValidConnectionID(id) || request.ContentLength != 0 {
		writeError(writer, errorInvalidRequest)
		return
	}
	observed, err := gateway.sessions.TerminalContext(id)
	if err != nil {
		writeError(writer, errorTerminalContextUnavailable)
		return
	}
	instant, err := formatProjectionInstant(observed.ObservedAt)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	result := terminalContextDTO{ObservedAt: instant, CWD: observed.CWD}
	if result.CWD != "" {
		if _, err := gateway.workdir.ParseStart(result.CWD); err != nil {
			result.CWD = ""
		}
	}
	if observed.Agent != nil {
		result.Agent = &remoteAgentDTO{Provider: string(observed.Agent.Provider), Profile: string(observed.Agent.Profile)}
	}
	if observed.Connection != nil {
		result.Connection = &connectionDTO{Transport: observed.Connection.Transport, ID: observed.Connection.ID}
	}
	writeJSON(writer, http.StatusOK, result, &errorInternal)
}
