//go:build live

package live

import "testing"

func TestLivePinnedAppServerRemoteTUIAcrossProfiles(t *testing.T) {
	for _, profile := range []string{"personal", "work", "work2"} {
		t.Run(profile, func(t *testing.T) {
			runLiveProfileProbe(t, profile)
		})
	}
}
