package contract

import (
	"strconv"
	"strings"
	"testing"
)

func TestHookArtifactsRejectMutationAtBoundary(t *testing.T) {
	hooks := reviewedHooksConfigFixture()
	lock := reviewedHooksLockFixture(hooks)
	if err := validateHookArtifacts(lock, codexLock{Version: "0.149.1", BinaryPath: reviewedCodexBinaryPath}, hooks); err != nil {
		t.Fatalf("reviewed hook artifacts rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*hooksLock, *[]byte)
	}{
		{name: "config bytes", mutate: func(_ *hooksLock, content *[]byte) { *content = append(*content, '\n') }},
		{name: "config digest", mutate: func(lock *hooksLock, _ *[]byte) { lock.HooksSHA256 = strings.Repeat("0", 64) }},
		{name: "Codex version", mutate: func(lock *hooksLock, _ *[]byte) { lock.CodexVersion = "0.149.0" }},
		{name: "helper path", mutate: func(lock *hooksLock, _ *[]byte) { lock.Helper.Path = "/tmp/skidbladnir-hook" }},
		{name: "helper digest", mutate: func(lock *hooksLock, _ *[]byte) { lock.Helper.SHA256 = "sha256:invalid" }},
		{name: "Go version", mutate: func(lock *hooksLock, _ *[]byte) { lock.Helper.Build.GoVersion = "go1.26.6" }},
		{name: "helper package", mutate: func(lock *hooksLock, _ *[]byte) { lock.Helper.Build.Package = "./cmd/other" }},
		{name: "helper flags", mutate: func(lock *hooksLock, _ *[]byte) { lock.Helper.Build.Flags = []string{"-trimpath"} }},
		{name: "helper environment", mutate: func(lock *hooksLock, _ *[]byte) { lock.Helper.Build.Environment[0] = "CGO_ENABLED=1" }},
		{name: "missing profile", mutate: func(lock *hooksLock, _ *[]byte) { lock.Profiles = lock.Profiles[:2] }},
		{name: "duplicate profile", mutate: func(lock *hooksLock, _ *[]byte) { lock.Profiles[2] = lock.Profiles[1] }},
		{name: "profile target", mutate: func(lock *hooksLock, _ *[]byte) { lock.Profiles[0].TargetPath = "/tmp/hooks.json" }},
		{name: "profile source", mutate: func(lock *hooksLock, _ *[]byte) { lock.Profiles[0].Source = "project" }},
		{name: "missing trust entry", mutate: func(lock *hooksLock, _ *[]byte) {
			delete(lock.Profiles[0].TrustState, lock.Profiles[0].TargetPath+":stop:0:0")
		}},
		{name: "foreign trust entry", mutate: func(lock *hooksLock, _ *[]byte) {
			lock.Profiles[0].TrustState["foreign"] = "sha256:" + strings.Repeat("1", 64)
		}},
		{name: "trust digest", mutate: func(lock *hooksLock, _ *[]byte) {
			lock.Profiles[0].TrustState[lock.Profiles[0].TargetPath+":stop:0:0"] = "invalid"
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changedLock := cloneHooksLock(lock)
			changedHooks := append([]byte(nil), hooks...)
			test.mutate(&changedLock, &changedHooks)
			if err := validateHookArtifacts(changedLock, codexLock{Version: "0.149.1", BinaryPath: reviewedCodexBinaryPath}, changedHooks); err == nil {
				t.Fatal("accepted mutated hook artifacts")
			}
		})
	}
}

func TestHookConfigRejectsAnythingOutsideReviewedClosure(t *testing.T) {
	valid := reviewedHooksConfigFixture()
	for name, changed := range map[string][]byte{
		"foreign event":   []byte(strings.Replace(string(valid), `"SessionStart":`, `"ForeignEvent":`, 1)),
		"async":           []byte(strings.Replace(string(valid), `"async":false`, `"async":true`, 1)),
		"matcher":         []byte(strings.Replace(string(valid), `"hooks":[`, `"matcher":"all","hooks":[`, 1)),
		"wrong timeout":   []byte(strings.Replace(string(valid), `"timeout":5`, `"timeout":6`, 1)),
		"unknown field":   []byte(strings.Replace(string(valid), `"async":false`, `"async":false,"unknown":true`, 1)),
		"duplicate field": []byte(strings.Replace(string(valid), `"async":false`, `"async":true,"async":false`, 1)),
		"changed command": []byte(strings.Replace(string(valid), reviewedHookCommand, "/usr/bin/foreign", 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateHooksConfig(changed, reviewedHookCommand); err == nil {
				t.Fatal("accepted hook config outside the reviewed closure")
			}
		})
	}
}

func reviewedHooksLockFixture(hooks []byte) hooksLock {
	profiles := make([]hookProfileLock, 0, len(reviewedProfiles))
	for _, profile := range reviewedProfiles {
		trust := make(map[string]string, len(reviewedHookEvents))
		for _, event := range reviewedHookEvents {
			trust[profile.TargetPath+":"+event.Key+":0:0"] = "sha256:" + strings.Repeat("1", 64)
		}
		profiles = append(profiles, hookProfileLock{Name: profile.Name, TargetPath: profile.TargetPath, Source: "user", TrustState: trust})
	}
	return hooksLock{
		CodexVersion:      "0.149.1",
		HooksPathFilename: "hooks.json",
		HooksSHA256:       sha256Hex(hooks),
		Helper: hookHelperLock{
			Path:   reviewedHelperPath,
			SHA256: strings.Repeat("2", 64),
			Build: hookBuildLock{
				GoVersion: "go1.26.7",
				Package:   "./cmd/skidbladnir-hook",
				Flags:     []string{"-trimpath", "-buildvcs=false"},
				Environment: []string{
					"CGO_ENABLED=0",
					"GOARCH=amd64",
					"GOENV=off",
					"GOEXPERIMENT=none",
					"GOFLAGS=",
					"GOOS=linux",
					"GOTOOLCHAIN=local",
					"GOAMD64=v1",
				},
			},
		},
		Profiles: profiles,
	}
}

func cloneHooksLock(lock hooksLock) hooksLock {
	clone := lock
	clone.Helper.Build.Flags = append([]string(nil), lock.Helper.Build.Flags...)
	clone.Helper.Build.Environment = append([]string(nil), lock.Helper.Build.Environment...)
	clone.Profiles = make([]hookProfileLock, len(lock.Profiles))
	for index, profile := range lock.Profiles {
		clone.Profiles[index] = profile
		clone.Profiles[index].TrustState = make(map[string]string, len(profile.TrustState))
		for key, value := range profile.TrustState {
			clone.Profiles[index].TrustState[key] = value
		}
	}
	return clone
}

func reviewedHooksConfigFixture() []byte {
	groups := make([]string, 0, len(reviewedHookEvents))
	for _, event := range reviewedHookEvents {
		groups = append(groups, `"`+event.ConfigName+`":[{"hooks":[{"type":"command","command":"`+reviewedHookCommand+`","timeout":`+strconv.Itoa(event.Timeout)+`,"async":false}]}]`)
	}
	return []byte(`{"hooks":{` + strings.Join(groups, ",") + `}}`)
}
