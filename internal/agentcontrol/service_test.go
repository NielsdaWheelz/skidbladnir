package agentcontrol

import (
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
)

func TestInspectionAcceptsOnlyImplementedWriteCapabilities(t *testing.T) {
	for _, read := range []string{"native", "terminal"} {
		for _, send := range []string{"terminal", "native", "unavailable"} {
			for _, interrupt := range []string{"terminal", "native", "unavailable"} {
				t.Run(read+"/"+send+"/"+interrupt, func(t *testing.T) {
					inspection := nativeInspection{
						Status:  agentruntime.Status{State: "idle", Source: "native"},
						Methods: agentruntime.Methods{Read: read, Send: send, Interrupt: interrupt},
					}
					if got, want := validInspection(inspection), send == "terminal" && interrupt == "terminal"; got != want {
						t.Fatalf("helper capability admission = %t, want %t", got, want)
					}
				})
			}
		}
	}
}
