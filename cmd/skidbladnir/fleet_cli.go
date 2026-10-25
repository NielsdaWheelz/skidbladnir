package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/install"
	"github.com/mdp/qrterminal/v3"
	"golang.org/x/term"
)

func fleetCommand(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	command := arguments[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	asJSON, name := false, ""
	if command != "connect" {
		flags.BoolVar(&asJSON, "json", false, "")
	}
	if command == "join" {
		flags.StringVar(&name, "name", "", "")
	}
	err := flags.Parse(arguments[1:])
	device := ""
	if err == nil && command == "remove" && flags.NArg() > 0 {
		device = flags.Arg(0)
		err = flags.Parse(flags.Args()[1:]) // options may follow DEVICE_ID
	}
	if errors.Is(err, flag.ErrHelp) {
		return commandHelp(stdout, command)
	}
	if err != nil || flags.NArg() != 0 || command == "remove" && !fleet.ValidDeviceID(device) {
		commandHelp(stderr, command)
		return exitUsage
	}
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
	switch command {
	case "status", "apply", "recover":
		method := "POST"
		if command == "status" {
			method = "GET"
		}
		err = fleet.Call(ctx, self, method, "/v1/local/"+command, nil, &status)
	case "join":
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
		}{invitation, name}, &status)
	case "connect":
		if err = fleet.Call(ctx, self, "GET", "/v1/local/status", nil, &status); err != nil {
			break
		}
		if status.Version != releaseVersion {
			fmt.Fprintln(stderr, "skid connect: installed service differs from this skid release; restart the installed service before connect")
			return exitFailure
		}
		origin, routeErr := routeGateway(ctx)
		if routeErr != nil {
			fmt.Fprintf(stderr, "skid connect: %v\n", routeErr)
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
		if hub.Origin == "" {
			fmt.Fprintln(stderr, "skid remove: "+hubWithoutOrigin)
			return exitFailure
		}
		err = fleet.Call(ctx, hub, "DELETE", "/v1/fleet/members/"+device, nil, &status)
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
		fmt.Fprintf(stderr, "skid %s: %s\n", command, message)
		return exitFailure
	}
	if asJSON {
		err = json.NewEncoder(stdout).Encode(status)
	} else {
		_, err = stdout.Write(statusText(status))
	}
	if err != nil {
		return exitFailure
	}
	return 0
}

// statusText renders a fleet status for people: the fleet, one row per device
// in join order, then any message.
func statusText(status fleet.Status) []byte {
	var text bytes.Buffer
	fmt.Fprintf(&text, "fleet %s; service %s\n", status.FleetID, status.Version)
	table := tabwriter.NewWriter(&text, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "device\tkind\tname\tversion\torigin\tconfiguration")
	for _, member := range status.Members {
		kind, name, version, origin, configuration := member.Kind, "", "", "", ""
		if member.Machine == status.Hub {
			kind = "hub"
		}
		for _, application := range status.Applications {
			if application.DeviceID == member.DeviceID {
				report := application.SelfReport
				name, version, origin, configuration = application.Name, report.Version, report.Origin, report.ApplyStatus.Status
				if report.ApplyStatus.Reason != "" {
					configuration += ": " + report.ApplyStatus.Reason
				}
			}
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n", member.DeviceID, kind, name, version, origin, configuration)
	}
	table.Flush()
	if status.Message != "" {
		fmt.Fprintln(&text, status.Message)
	}
	return text.Bytes()
}

const hubWithoutOrigin = "hub has no remote address; connect tailscale on the hub and run skid connect there"

// tailnetStatus is what skid reads of `tailscale status --json`.
type tailnetStatus struct {
	BackendState   string
	Self           *struct{ DNSName string }
	CurrentTailnet *struct{ MagicDNSEnabled bool }
	CertDomains    []string
}

// networkOrigin returns this host's tailnet gateway origin, the tailscale
// command that resolved it and the status it read.
func networkOrigin(ctx context.Context) (origin, tailscale string, status tailnetStatus, err error) {
	tailscale, err = install.TailscalePath()
	if err != nil {
		return "", "", status, err
	}
	data, err := exec.CommandContext(ctx, tailscale, "status", "--json").Output()
	if err != nil {
		return "", "", status, tailscaleError("tailscale unavailable; open tailscale and sign in", nil, err)
	}
	if json.Unmarshal(data, &status) != nil || status.BackendState != "Running" || status.Self == nil || status.Self.DNSName == "" {
		return "", "", status, errors.New("tailscale is not connected; sign in to tailscale first")
	}
	return "https://" + strings.TrimSuffix(status.Self.DNSName, ".") + ":8443", tailscale, status, nil
}

// routeGateway checks what serving the gateway over tailnet https needs, points
// https:8443 at the gateway, replacing whatever 8443 served, and returns this
// host's origin. A missing prerequisite is the owner's to fix: skid names the
// fix and changes no tailnet or host policy.
func routeGateway(ctx context.Context) (string, error) {
	origin, tailscale, status, err := networkOrigin(ctx)
	if err != nil {
		return "", err
	}
	var fixes []string
	if status.CurrentTailnet == nil || !status.CurrentTailnet.MagicDNSEnabled {
		fixes = append(fixes, "turn on magicdns and https certificates at https://login.tailscale.com/admin/dns")
	} else if !slices.Contains(status.CertDomains, strings.TrimSuffix(status.Self.DNSName, ".")) {
		fixes = append(fixes, "turn on https certificates at https://login.tailscale.com/admin/dns")
	}
	// linux tailscaled lets only root and its operator change what it serves.
	if runtime.GOOS == "linux" && os.Getuid() != 0 {
		account, err := user.Current()
		if err != nil {
			return "", err
		}
		data, err := exec.CommandContext(ctx, tailscale, "debug", "prefs").Output()
		if err != nil {
			return "", tailscaleError("tailscale preferences unavailable", nil, err)
		}
		var prefs struct{ OperatorUser string }
		if err = json.Unmarshal(data, &prefs); err != nil {
			return "", fmt.Errorf("tailscale preferences unreadable: %w", err)
		}
		if prefs.OperatorUser != account.Username {
			fixes = append(fixes, "make this user tailscale's operator: sudo tailscale set --operator="+account.Username)
		}
	}
	if len(fixes) > 0 {
		return "", errors.New("tailscale is not ready to serve skid:\n  " + strings.Join(fixes, "\n  ") + "\nthen run skid connect again")
	}
	if output, err := exec.CommandContext(ctx, tailscale, "serve", "--bg", "--https=8443", fleet.LocalOrigin).CombinedOutput(); err != nil {
		return "", tailscaleError("tailscale serve failed", output, err)
	}
	return origin, nil
}

// tailscaleError keeps tailscale's own explanation, which usually names the fix.
func tailscaleError(message string, output []byte, err error) error {
	var exit *exec.ExitError
	if len(output) == 0 && errors.As(err, &exit) {
		output = exit.Stderr
	}
	if text := strings.TrimSpace(string(output)); text != "" {
		return fmt.Errorf("%s:\n%s", message, text)
	}
	return fmt.Errorf("%s: %w", message, err)
}
