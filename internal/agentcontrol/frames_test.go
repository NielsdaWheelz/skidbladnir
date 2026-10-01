package agentcontrol

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	tmuxclient "github.com/NielsdaWheelz/skidbladnir/internal/tmux"
)

// TestFrames replays the authored provider frames in testdata/frames: real
// tmux captures of dummy screens, with each frame's expected status, composer
// and rules. It is the detector's fixture check, run on demand with
// `go test ./internal/agentcontrol`; testdata/frames/capture.py regenerates the
// captures after frames are added or edited.
func TestFrames(t *testing.T) {
	file, err := os.Open("testdata/frames/observations.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	composers := map[composer]string{composerUnknown: "unknown", composerEmpty: "empty", composerDraft: "draft", composerBlocked: "blocked"}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<24)
	for scanner.Scan() {
		var frame struct {
			ID        string                  `json:"id"`
			Provider  string                  `json:"provider"`
			Width     int                     `json:"width"`
			Height    int                     `json:"height"`
			Alternate bool                    `json:"alternate"`
			Regions   []tmuxclient.PaneRegion `json:"regions"`
			Expect    struct {
				Activity, Interaction, Notice, Composer, Reason string
				Rules                                           []string
			} `json:"expect"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &frame); err != nil {
			t.Fatal(err)
		}
		provider, err := agentruntime.ParseProvider(frame.Provider)
		if err != nil {
			t.Fatal(err)
		}
		got := detect(provider, tmuxclient.PaneObservation{Width: frame.Width, Height: frame.Height, Alternate: frame.Alternate, Regions: frame.Regions})
		status := []string{string(got.status.Activity), string(got.status.Interaction), string(got.status.Notice), composers[got.composer], string(got.status.Reason)}
		want := []string{frame.Expect.Activity, frame.Expect.Interaction, frame.Expect.Notice, frame.Expect.Composer, frame.Expect.Reason}
		if strings.Join(status, " ") != strings.Join(want, " ") {
			t.Errorf("%s: status %v, want %v", frame.ID, status, want)
		}
		var rules []string
		for _, rule := range got.rules {
			id := rule.ID
			if rule.Region != DiagnosticBottom {
				id += "@" + string(rule.Region)
			}
			rules = append(rules, id)
		}
		if strings.Join(rules, ",") != strings.Join(frame.Expect.Rules, ",") {
			t.Errorf("%s: rules %v, want %v", frame.ID, rules, frame.Expect.Rules)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
