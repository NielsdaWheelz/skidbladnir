// Package agentcli adapts structured stdin/stdout to the shared fleet client.
package agentcli

import (
	"context"
	"io"

	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
)

// Run handles one operation. Diagnostics contain no request or response contents.
func Run(ctx context.Context, configPath, operation string, stdin io.Reader, stdout io.Writer) int {
	result := fleetclient.Failed("invalid_input", "not_sent")
	encoded, err := io.ReadAll(io.LimitReader(stdin, fleetclient.MaximumInputBytes+1))
	if err != nil {
		result = fleetclient.Failed("input_unavailable", "not_sent")
	} else if len(encoded) > fleetclient.MaximumInputBytes {
		result = fleetclient.Failed("input_limit", "not_sent")
	} else if client, err := fleetclient.Open(configPath); err != nil {
		result = fleetclient.Failed("configuration_invalid", "not_sent")
	} else {
		result = client.Execute(ctx, operation, encoded)
	}
	output, err := result.Encode(operation)
	if err != nil {
		dispatch := "unknown"
		if operation == "read" || operation == "list" || result.Error != nil && result.Error.Dispatch == "not_sent" {
			dispatch = "not_sent"
		}
		result = fleetclient.Failed("output_limit", dispatch)
		output, _ = result.Encode(operation) // The fixed failure envelope is below either output limit.
	}
	if _, err := stdout.Write(output); err != nil {
		return 1
	}
	if !result.OK {
		return 1
	}
	return 0
}
