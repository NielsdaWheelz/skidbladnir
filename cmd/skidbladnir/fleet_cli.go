package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/attention"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/install"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"github.com/mdp/qrterminal/v3"
	"golang.org/x/term"
)

const invitationPrefix = "skid://connect/"

func installationCommand(arguments []string, stdout, stderr io.Writer) int {
	if err := install.Run(context.Background(), arguments, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "skid %s: %v\n", arguments[0], err)
		return exitFailure
	}
	return 0
}

func fleetCommand(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	directory, err := attention.StateDirectory()
	if err != nil {
		fmt.Fprintln(stderr, "skid state unavailable")
		return exitFailure
	}
	identity, err := fleet.LoadIdentity(directory)
	if err != nil {
		fmt.Fprintln(stderr, "skid is not installed; run skid install")
		return exitFailure
	}
	local := controlTarget{"http://127.0.0.1:7341", identity.Machine, identity.LocalBearer}
	var status fleet.Status
	switch arguments[0] {
	case "status":
		if len(arguments) != 1 {
			return exitUsage
		}
		err = controlRequest(ctx, local, "GET", "/v1/local/status", nil, &status)
	case "apply":
		if len(arguments) != 1 {
			return exitUsage
		}
		err = controlRequest(ctx, local, "POST", "/v1/local/apply", nil, &status)
	case "recover":
		if len(arguments) != 1 {
			return exitUsage
		}
		err = controlRequest(ctx, local, "POST", "/v1/local/recover", nil, &status)
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
		encoded, found := strings.CutPrefix(strings.TrimSpace(line), invitationPrefix)
		data, decodeErr := base64.RawURLEncoding.Strict().DecodeString(encoded)
		var invitation fleet.Invitation
		if !found || decodeErr != nil || strictjson.Decode(data, &invitation) != nil {
			fmt.Fprintln(stderr, "invalid invitation; run skid connect again")
			return exitFailure
		}
		err = controlRequest(ctx, local, "POST", "/v1/local/join", struct {
			Invitation fleet.Invitation `json:"invitation"`
			Name       string           `json:"name"`
		}{invitation, *name}, &status)
	case "connect", "remove":
		if arguments[0] == "connect" {
			if len(arguments) != 1 {
				return exitUsage
			}
			if err = controlRequest(ctx, local, "GET", "/v1/local/status", nil, &status); err != nil {
				break
			}
			if status.Version != releaseVersion {
				err = errors.New("installed service differs from this skid release; restart the installed service before connect")
				break
			}
			origin, networkErr := connectNetwork(ctx)
			if networkErr != nil {
				fmt.Fprintln(stderr, networkErr)
				return exitFailure
			}
			if err = controlRequest(ctx, local, "PUT", "/v1/local/network", struct {
				Origin string `json:"origin"`
			}{origin}, &status); err != nil {
				break
			}
		} else if len(arguments) != 2 || !fleet.ValidDeviceID(arguments[1]) {
			return exitUsage
		}
		hub, hubErr := activeHub(directory, identity)
		if hubErr != nil {
			err = hubErr
			break
		}
		if arguments[0] == "remove" {
			err = controlRequest(ctx, hub, "DELETE", "/v1/fleet/members/"+arguments[1], nil, &status)
			break
		}
		var invitation fleet.Invitation
		if err = controlRequest(ctx, hub, "POST", "/v1/fleet/invites", nil, &invitation); err != nil {
			break
		}
		if err = fleet.ValidateInvitation(invitation, time.Now()); err != nil {
			break
		}
		data, encodeErr := json.Marshal(invitation)
		if encodeErr != nil {
			err = encodeErr
			break
		}
		value := invitationPrefix + base64.RawURLEncoding.EncodeToString(data)
		if file, ok := stdout.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
			qrterminal.GenerateHalfBlock(value, qrterminal.L, stdout)
		}
		_, err = fmt.Fprintln(stdout, value)
		if err != nil {
			return exitFailure
		}
		fmt.Fprintln(stderr, "scan with skid on your phone, or paste into skid join on another computer; expires in five minutes")
		return 0
	default:
		return exitUsage
	}
	if err != nil {
		fmt.Fprintf(stderr, "skid %s: %v\n", arguments[0], err)
		return exitFailure
	}
	if err := json.NewEncoder(stdout).Encode(status); err != nil {
		return exitFailure
	}
	return 0
}

type controlTarget struct{ origin, machine, bearer string }

func activeHub(directory string, identity fleet.Identity) (controlTarget, error) {
	a := identity.ActiveFleet
	if a == nil {
		return controlTarget{}, errors.New("local mode; join a fleet or run skid recover")
	}
	key, err := fleet.ParsePublicKey(a.PublicKey)
	if err != nil {
		return controlTarget{}, err
	}
	state, err := fleet.LoadHostState(directory)
	if err != nil {
		return controlTarget{}, err
	}
	public, err := fleet.VerifyPublic(key, state.Public)
	if err != nil || public.FleetID != a.FleetID {
		return controlTarget{}, errors.New("fleet routing unavailable; check skid status")
	}
	for _, target := range public.Machines {
		if target.Machine != public.ActiveHub {
			continue
		}
		if target.Machine == identity.Machine {
			return controlTarget{"http://127.0.0.1:7341", target.Machine, identity.LocalBearer}, nil
		}
		if target.Origin == "" {
			return controlTarget{}, errors.New("hub has no remote address; connect tailscale on the hub and run skid connect there")
		}
		return controlTarget{target.Origin, target.Machine, a.Bearer}, nil
	}
	return controlTarget{}, errors.New("active hub unavailable")
}

func controlRequest(ctx context.Context, target controlTarget, method, path string, body, result any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	request, err := http.NewRequestWithContext(ctx, method, target.origin+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+target.bearer)
	request.Header.Set("Skidbladnir-Machine", target.machine)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("skid service unavailable; check skid status and network access")
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, fleet.MaximumStateBytes+1))
	err = errors.Join(err, response.Body.Close())
	if err != nil || len(data) > fleet.MaximumStateBytes {
		return errors.New("skid response unavailable")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Never surface arbitrary network response text, credentials or commands.
		var failure struct {
			Code    string          `json:"code"`
			Message string          `json:"message"`
			Routing *fleet.Envelope `json:"routing,omitempty"`
			Removal *fleet.Envelope `json:"removal,omitempty"`
		}
		if strictjson.Decode(data, &failure) != nil {
			return errors.New("skid rejected the request")
		}
		return errors.New(failure.Code + ": " + failure.Message)
	}
	return strictjson.Decode(data, result)
}

func networkOrigin(ctx context.Context) (string, error) {
	path, err := install.TailscalePath()
	if err != nil {
		return "", err
	}
	data, err := exec.CommandContext(ctx, path, "status", "--json").Output()
	if err != nil {
		return "", errors.New("tailscale unavailable; open tailscale and sign in")
	}
	var status struct {
		BackendState string
		Self         *struct{ DNSName string }
	}
	if json.Unmarshal(data, &status) != nil || status.BackendState != "Running" || status.Self == nil || status.Self.DNSName == "" {
		return "", errors.New("tailscale is not connected; sign in to tailscale first")
	}
	return "https://" + strings.TrimSuffix(status.Self.DNSName, ".") + ":8443", nil
}

func connectNetwork(ctx context.Context) (string, error) {
	origin, err := networkOrigin(ctx)
	if err != nil {
		return "", err
	}
	if err := install.ConfigureTailnetRoute(ctx, origin, "http://127.0.0.1:7341"); err != nil {
		return "", err
	}

	return origin, nil
}

// The service environment may omit SHELL. Resolve the account's login shell at
// this OS boundary rather than choosing a different shell on the user's behalf.
func loginShell(ctx context.Context, home string) (string, error) {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell, nil
	}
	var command *exec.Cmd
	if runtime.GOOS == "darwin" {
		command = exec.CommandContext(ctx, "/usr/bin/dscl", ".", "-read", "/Users/"+filepath.Base(home), "UserShell")
	} else {
		command = exec.CommandContext(ctx, "getent", "passwd", fmt.Sprint(os.Getuid()))
	}
	data, err := command.Output()
	if err != nil {
		return "", errors.New("login shell unavailable; set shell in config.yaml")
	}
	value := strings.TrimSpace(string(data))
	if runtime.GOOS == "darwin" {
		_, value, _ = strings.Cut(value, ": ")
	} else {
		fields := strings.Split(value, ":")
		if len(fields) != 7 {
			return "", errors.New("login shell unavailable")
		}
		value = fields[6]
	}
	if !filepath.IsAbs(value) {
		return "", errors.New("login shell unavailable; set shell in config.yaml")
	}
	return value, nil
}
