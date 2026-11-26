package agentcli

import (
	"context"
	"flag"
	"io"
	"slices"
	"strconv"

	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleetclient"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
)

func runJob(ctx context.Context, arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(arguments) == 1 && (arguments[0] == "-h" || arguments[0] == "--help") {
		text, _ := Help("job")
		if _, err := io.WriteString(stdout, text); err != nil {
			return 1
		}
		return 0
	}
	result, exit := executeJob(ctx, arguments, stdin)
	if exit == 2 && !slices.Contains(arguments, "--json") {
		_, _ = io.WriteString(stderr, "invalid job arguments; use skid help job\n")
		return 2
	}
	data, err := result.Encode("")
	if err != nil {
		return 1
	}
	if _, err := stdout.Write(data); err != nil {
		return 1
	}
	return exit
}

func executeJob(ctx context.Context, arguments []string, stdin io.Reader) (fleetclient.Result, int) {
	invalid := fleetclient.Failed("invalid_input", "not_sent")
	if len(arguments) < 2 {
		return invalid, 2
	}
	operation, id := arguments[0], arguments[1]
	if _, ok := api.JobIDBytes(id); !ok || !slices.Contains([]string{"submit", "result", "ack"}, operation) {
		return invalid, 2
	}
	options := flag.NewFlagSet("job", flag.ContinueOnError)
	options.SetOutput(io.Discard)
	host := options.String("machine", "", "")
	directory := options.String("config", "", "")
	options.Bool("json", false, "")
	var input api.JobRequest
	var offset int
	switch operation {
	case "submit":
		readStdin := options.Bool("stdin", false, "")
		if options.Parse(arguments[2:]) != nil || options.NArg() != 0 || !*readStdin {
			return invalid, 2
		}
		data, err := io.ReadAll(io.LimitReader(stdin, fleetclient.MaximumInputBytes+1))
		if err != nil {
			return fleetclient.Failed("input_unavailable", "not_sent"), 1
		}
		if len(data) > fleetclient.MaximumInputBytes {
			return fleetclient.Failed("input_limit", "not_sent"), 1
		}
		if strictjson.Decode(data, &input) != nil {
			return invalid, 2
		}
	case "result", "ack":
		var page *string
		if operation == "result" {
			page = options.String("offset", "0", "")
		}
		if options.Parse(arguments[2:]) != nil || options.NArg() != 0 {
			return invalid, 2
		}
		if page != nil {
			n, err := strconv.Atoi(*page)
			if err != nil || n < 0 || n > api.JobAnswerLimit || n%api.JobPageSize != 0 || strconv.Itoa(n) != *page {
				return invalid, 2
			}
			offset = n
		}
	}
	if *host == "" {
		return invalid, 2
	}
	var err error
	if *directory == "" {
		*directory, err = attention.StateDirectory()
		if err != nil {
			return fleetclient.Failed("configuration_invalid", "not_sent"), 1
		}
	}
	client, err := fleetclient.Open(*directory)
	if err != nil {
		return fleetclient.Failed("configuration_invalid", "not_sent"), 1
	}
	result := client.Job(ctx, operation, *host, id, input, offset)
	return result, result.ExitCode("")
}
