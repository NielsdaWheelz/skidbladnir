package gateway

import (
	"encoding/json"
	"net/http"
	"unicode/utf8"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
)

func writeAgentRead(writer http.ResponseWriter, result agentcontrol.ReadResult) {
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			writeError(writer, errorInternal)
			return
		}
		if len(encoded)+1 <= int(MaximumBodyBytes) {
			writeJSON(writer, http.StatusOK, result)
			return
		}
		if result.Text == "" {
			writeError(writer, errorInternal)
			return
		}
		result.Text = result.Text[len(result.Text)/2:]
		for len(result.Text) > 0 && !utf8.RuneStart(result.Text[0]) {
			result.Text = result.Text[1:]
		}
		result.Truncated = true
	}
}
