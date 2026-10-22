package gateway

import (
	"net/http"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalcontext"
)

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
	result := api.TerminalContext{ObservedAt: observed.ObservedAt.UTC(), CWD: observed.CWD}
	if result.CWD != "" {
		if _, err := gateway.workdir.ParseStart(result.CWD); err != nil {
			result.CWD = ""
		}
	}
	if observed.Agent != nil {
		result.Agent = &api.ContextAgent{Provider: string(observed.Agent.Provider), Profile: string(observed.Agent.Profile), Label: profileLabel(gateway.sessions.Profiles(), observed.Agent.Profile)}
	}
	if observed.Connection != nil {
		result.Connection = &api.Connection{Transport: observed.Connection.Transport, ID: observed.Connection.ID}
	}
	writeJSON(writer, http.StatusOK, result, &errorInternal)
}
