package agentcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetview"
	"github.com/muesli/cancelreader"
)

func writeClosePreview(writer io.Writer, targets []fleetclient.CloseTarget, terminalOnly bool) {
	fmt.Fprintln(writer, fleetview.CloseQuestion(targets, terminalOnly))
	for _, target := range targets {
		fmt.Fprintf(writer, "  %s on %s\n", attention.SingleLine(target.Name), attention.SingleLine(target.Label))
	}
}

func interactiveClose(ctx context.Context, client *fleetclient.Client, parsed command, stdin, stdout *os.File, stderr io.Writer) int {
	plan, failure := client.PreviewClose(ctx, parsed.request)
	if failure != nil {
		return render(parsed, fleetclient.Result{Error: failure}, stdout, stderr)
	}
	if len(plan.Targets) > 1 {
		writeClosePreview(stdout, plan.Targets, plan.TerminalOnly)
		fmt.Fprint(stdout, "type yes to close this branch: ")
		reader, err := cancelreader.NewReader(stdin)
		if err != nil {
			return render(parsed, fleetclient.Failed("input_unavailable", "not_sent"), stdout, stderr)
		}
		done := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { reader.Cancel(); close(done) })
		answer, err := bufio.NewReader(io.LimitReader(reader, 32)).ReadString('\n')
		if !stop() {
			<-done
		}
		reader.Close()
		if ctx.Err() != nil || err != nil || strings.TrimSpace(answer) != "yes" {
			fmt.Fprintln(stdout, "cancelled")
			return 1
		}
	}
	result := fleetclient.Result{OK: true, Value: client.ConfirmClose(ctx, plan)}
	return render(parsed, result, stdout, stderr)
}
