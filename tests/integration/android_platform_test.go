//go:build integration && androidplatform

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"flag"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
)

var platformADB = flag.String("skidbladnir-platform-adb", "", "approved device adb executable")
var platformSerial = flag.String("skidbladnir-platform-serial", "", "approved device serial")
var platformCapability = flag.String("skidbladnir-platform-capability", "", "explicit device capability")
var platformTimeout = flag.Duration("skidbladnir-platform-timeout", 0, "instrumentation deadline")
var platformFixtureState = flag.String("skidbladnir-platform-fixture-state", "", "private parent cleanup state")

// The existing platform gate owns installation, exact test-count validation,
// pairing preservation, device artifacts and release restoration. This fixture
// owns its real gateway and isolated socket, including interrupted runs.
func TestAndroidPlatformWithShellGateway(t *testing.T) {
	if *platformCapability != "device-cli-v1" || !filepath.IsAbs(*platformADB) || *platformSerial == "" ||
		!filepath.IsAbs(*platformFixtureState) || *platformTimeout <= 0 || *platformTimeout > 15*time.Minute {
		t.Fatal("platform fixture requires explicit device capability and bounded invocation")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fixture := newShellGateway(t, "", false)
	_, port, err := net.SplitHostPort(fixture.server.Listener.Addr().String())
	if err != nil {
		t.Fatal("read private TLS listener port")
	}
	deviceFile := fixture.socket + ".json"
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: fixture.server.Certificate().Raw})
	devboxBearer, err := auth.Mint(auth.MintOptions{Path: filepath.Join(fixture.root, "devbox-bearer")})
	if err != nil {
		t.Fatal("mint unavailable devbox fixture credential")
	}
	macbookBearer, err := auth.Mint(auth.MintOptions{Path: filepath.Join(fixture.root, "macbook-bearer")})
	if err != nil {
		t.Fatal("mint unavailable macbook fixture credential")
	}
	encoded, err := json.Marshal(map[string]any{
		"credentials": []map[string]string{
			{"handle": integrationMachineText, "label": "Arch", "origin": "https://127.0.0.1:8443/", "bearer": fixture.bearer},
			{"handle": "mh-11111111111111111111111111111111", "label": "Devbox", "origin": "https://127.0.0.2:8443/", "bearer": devboxBearer},
			{"handle": "mh-22222222222222222222222222222222", "label": "MacBook", "origin": "https://127.0.0.3:8443/", "bearer": macbookBearer},
		},
		"machineHandle": integrationMachineText, "cwd": fixture.project,
		"tlsCertificatePem": string(certificate),
	})
	if err != nil {
		t.Fatal("encode private phone fixture")
	}
	probeCtx, cancelProbe := context.WithTimeout(ctx, 10*time.Second)
	mappings, err := exec.CommandContext(probeCtx, *platformADB, "-s", *platformSerial, "reverse", "--list").Output()
	cancelProbe()
	if err != nil {
		t.Fatal("inspect existing phone reverse mappings")
	}
	for _, line := range strings.Split(string(mappings), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[1] == "tcp:8443" {
			t.Fatal("phone fixture port already has a reverse mapping")
		}
	}
	// Publish content-free cleanup coordinates before any device dispatch. The
	// parent removes a reverse mapping only when both endpoints match this run.
	if err := os.WriteFile(*platformFixtureState, []byte(deviceFile+"\n"+port+"\n"), 0o600); err != nil {
		t.Fatal("record private platform cleanup coordinates")
	}
	adb := func(input []byte, arguments ...string) error {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, *platformADB, append([]string{"-s", *platformSerial}, arguments...)...)
		command.Stdin = bytes.NewReader(input)
		return command.Run()
	}
	if err := adb(nil, "reverse", "--no-rebind", "tcp:8443", "tcp:"+port); err != nil {
		t.Fatal("reserve private phone reverse mapping")
	}
	if err := adb(encoded, "shell", "run-as", "dev.niels.skidbladnir", "sh", "-c",
		"'umask 077; cat > files/"+deviceFile+"'"); err != nil {
		t.Fatal("install private phone fixture")
	}
	ctx, cancel := context.WithTimeout(ctx, *platformTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, *platformADB, "-s", *platformSerial, "shell", "am", "instrument", "-r", "-w",
		"-e", "shellsFixture", deviceFile, "dev.niels.skidbladnir.test/androidx.test.runner.AndroidJUnitRunner")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		t.Fatal("Android instrumentation command failed or exceeded its deadline")
	}
}
