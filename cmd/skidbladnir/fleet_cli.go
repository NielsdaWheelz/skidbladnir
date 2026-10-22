package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/install"
	"github.com/mdp/qrterminal/v3"
	"golang.org/x/term"
)

func fleetCommand(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	directory, err := attention.StateDirectory()
	if err != nil {
		fmt.Fprintln(stderr, "skid state unavailable")
		return exitFailure
	}
	local, err := fleet.ReadLocal(directory)
	if err != nil {
		fmt.Fprintln(stderr, "skid is not installed; run skid install")
		return exitFailure
	}
	self, hub := local.Target(local.Machine), local.Target(local.Hub)
	var status fleet.Status
	switch arguments[0] {
	case "status", "apply", "recover":
		if len(arguments) != 1 {
			return exitUsage
		}
		method := "POST"
		if arguments[0] == "status" {
			method = "GET"
		}
		err = fleet.Call(ctx, self, method, "/v1/local/"+arguments[0], nil, &status)
	case "join":
		flags := flag.NewFlagSet("join", flag.ContinueOnError)
		flags.SetOutput(stderr)
		name := flags.String("name", "", "new computer name")
		if flags.Parse(arguments[1:]) != nil || flags.NArg() != 0 {
			return exitUsage
		}
		if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
			fmt.Fprintln(stderr, "paste the invitation from skid connect:")
		}
		line, readErr := bufio.NewReader(io.LimitReader(stdin, 64*1024+1)).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) || len(line) > 64*1024 {
			fmt.Fprintln(stderr, "invitation unavailable")
			return exitFailure
		}
		invitation, parseErr := fleet.ParseInvitation(strings.TrimSpace(line), time.Now())
		if parseErr != nil {
			fmt.Fprintln(stderr, "invalid invitation; run skid connect again")
			return exitFailure
		}
		err = fleet.Call(ctx, self, "POST", "/v1/local/join", struct {
			Invitation fleet.Invitation `json:"invitation"`
			Name       string           `json:"name"`
		}{invitation, *name}, &status)
	case "connect":
		if len(arguments) != 1 {
			return exitUsage
		}
		if err = fleet.Call(ctx, self, "GET", "/v1/local/status", nil, &status); err != nil {
			break
		}
		if status.Version != releaseVersion {
			fmt.Fprintln(stderr, "skid connect: installed service differs from this skid release; restart the installed service before connect")
			return exitFailure
		}
		origin, tailscale, networkErr := networkOrigin(ctx)
		// Point https:8443 at the gateway, replacing whatever 8443 served.
		if networkErr == nil {
			if serveErr := exec.CommandContext(ctx, tailscale, "serve", "--bg", "--https=8443", fleet.LocalOrigin).Run(); serveErr != nil {
				networkErr = fmt.Errorf("tailscale route configuration failed: %w", serveErr)
			}
		}
		if networkErr != nil {
			fmt.Fprintln(stderr, networkErr)
			return exitFailure
		}
		if err = fleet.Call(ctx, self, "PUT", "/v1/local/network", struct {
			Origin string `json:"origin"`
		}{origin}, &status); err != nil {
			break
		}
		if hub.Origin == "" {
			fmt.Fprintln(stderr, "skid connect: "+hubWithoutOrigin)
			return exitFailure
		}
		var invitation fleet.Invitation
		if err = fleet.Call(ctx, hub, "POST", "/v1/fleet/invites", nil, &invitation); err != nil {
			break
		}
		line := fleet.EncodeInvitation(invitation)
		if file, ok := stdout.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
			qrterminal.GenerateHalfBlock(line, qrterminal.L, stdout)
		}
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return exitFailure
		}
		fmt.Fprintln(stderr, "scan with skid on your phone, or paste into skid join on another computer; expires in five minutes")
		return 0
	case "remove":
		if len(arguments) != 2 || !fleet.ValidDeviceID(arguments[1]) {
			return exitUsage
		}
		if hub.Origin == "" {
			fmt.Fprintln(stderr, "skid remove: "+hubWithoutOrigin)
			return exitFailure
		}
		err = fleet.Call(ctx, hub, "DELETE", "/v1/fleet/members/"+arguments[1], nil, &status)
	default:
		return exitUsage
	}
	if err != nil {
		message := "skid service unavailable; check skid status and network access"
		var failure *fleet.Error
		if errors.As(err, &failure) {
			message = "skid rejected the request"
			if failure.Code != "" {
				message = failure.Code + ": " + failure.Message
			}
		}
		fmt.Fprintf(stderr, "skid %s: %s\n", arguments[0], message)
		return exitFailure
	}
	if err := json.NewEncoder(stdout).Encode(status); err != nil {
		return exitFailure
	}
	return 0
}

const hubWithoutOrigin = "hub has no remote address; connect tailscale on the hub and run skid connect there"

// networkOrigin returns this host's tailnet gateway origin and the tailscale
// command that resolved it.
func networkOrigin(ctx context.Context) (origin, tailscale string, err error) {
	tailscale, err = install.TailscalePath()
	if err != nil {
		return "", "", err
	}
	data, err := exec.CommandContext(ctx, tailscale, "status", "--json").Output()
	if err != nil {
		return "", "", errors.New("tailscale unavailable; open tailscale and sign in")
	}
	var status struct {
		BackendState string
		Self         *struct{ DNSName string }
	}
	if json.Unmarshal(data, &status) != nil || status.BackendState != "Running" || status.Self == nil || status.Self.DNSName == "" {
		return "", "", errors.New("tailscale is not connected; sign in to tailscale first")
	}
	return "https://" + strings.TrimSuffix(status.Self.DNSName, ".") + ":8443", tailscale, nil
}
