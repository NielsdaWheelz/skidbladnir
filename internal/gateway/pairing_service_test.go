package gateway

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
)

func TestPairingHTTPServiceAllowsExactlyOneConcurrentRedemption(t *testing.T) {
	harness := newPairingHarness(t)
	invite := harness.createInvite(t)
	expiresAt, err := time.Parse(time.RFC3339Nano, invite.ExpiresAt)
	if err != nil {
		t.Fatalf("parse invitation expiry %q: %v", invite.ExpiresAt, err)
	}
	remaining := time.Until(expiresAt)
	if remaining < 4*time.Minute+55*time.Second || remaining > 5*time.Minute+time.Second {
		t.Fatalf("invitation lifetime = %s, want exactly five minutes from creation", remaining)
	}

	statuses := make([]int, 2)
	bodies := make([][]byte, 2)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range statuses {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			statuses[index], bodies[index] = harness.redeem(invite.PairingInviteToken, harness.machine.String())
		}(index)
	}
	close(start)
	wait.Wait()
	sortedStatuses := append([]int(nil), statuses...)
	sort.Ints(sortedStatuses)
	if sortedStatuses[0] != http.StatusOK || sortedStatuses[1] != http.StatusUnauthorized {
		t.Fatalf("concurrent redemption statuses = %v, want [200 401]; bodies=%q", statuses, bodies)
	}
	winner := bodies[0]
	if statuses[0] != http.StatusOK {
		winner = bodies[1]
	}
	var redeemed pairingResponse
	if err := json.Unmarshal(winner, &redeemed); err != nil {
		t.Fatalf("decode successful redemption: %v; body=%q", err, winner)
	}
	if redeemed.Bearer != harness.bearer || redeemed.Machine.Handle != harness.machine.String() || redeemed.Machine.Platform != platform.KindLinux {
		t.Fatalf("successful redemption = %+v, want current bearer and exact machine", redeemed)
	}
	status, body := harness.redeem(invite.PairingInviteToken, harness.machine.String())
	assertPairingRejected(t, status, body)

	logs := harness.logs.String()
	if strings.Contains(logs, invite.PairingInviteToken) || strings.Contains(logs, harness.bearer) {
		t.Fatalf("request log disclosed a pairing or durable credential: %s", logs)
	}
	for _, route := range []string{`"http.route":"/v1/pairing-invites"`, `"http.route":"/v1/pairings"`} {
		if !strings.Contains(logs, route) {
			t.Fatalf("request log omitted route %s: %s", route, logs)
		}
	}
}

func TestPairingHTTPServiceInvalidatesReplacementExpiryRestartAndBearerRotation(t *testing.T) {
	t.Run("replacement", func(t *testing.T) {
		harness := newPairingHarness(t)
		first := harness.createInvite(t)
		second := harness.createInvite(t)
		status, body := harness.redeem(first.PairingInviteToken, harness.machine.String())
		assertPairingRejected(t, status, body)
		status, _ = harness.redeem(second.PairingInviteToken, harness.machine.String())
		if status != http.StatusOK {
			t.Fatalf("replacement invitation status = %d, want 200", status)
		}
	})

	t.Run("expiry", func(t *testing.T) {
		harness := newPairingHarness(t)
		credential, err := harness.verifier.Read()
		if err != nil {
			t.Fatalf("read current bearer: %v", err)
		}
		invite, err := harness.slot.Create(time.Now().Add(-6*time.Minute), harness.machine, credential)
		if err != nil {
			t.Fatalf("create expired invitation fixture through pairing service: %v", err)
		}
		status, body := harness.redeem(invite.Token.String(), harness.machine.String())
		assertPairingRejected(t, status, body)
	})

	t.Run("restart", func(t *testing.T) {
		harness := newPairingHarness(t)
		invite := harness.createInvite(t)
		restarted := harness.restart(t)
		status, body := restarted.redeem(invite.PairingInviteToken, harness.machine.String())
		assertPairingRejected(t, status, body)
	})

	t.Run("bearer rotation", func(t *testing.T) {
		harness := newPairingHarness(t)
		invite := harness.createInvite(t)
		rotated, err := auth.Mint(auth.MintOptions{Path: harness.bearerPath})
		if err != nil {
			t.Fatalf("rotate bearer: %v", err)
		}
		if rotated == harness.bearer {
			t.Fatal("bearer rotation returned the old credential")
		}
		status, body := harness.redeem(invite.PairingInviteToken, harness.machine.String())
		assertPairingRejected(t, status, body)
	})
}

func TestPairingHTTPServiceRejectsNoncanonicalRequestsWithoutConsumingTheInvite(t *testing.T) {
	harness := newPairingHarness(t)
	invite := harness.createInvite(t)
	noncanonicalAlias := noncanonicalTokenAlias(t, invite.PairingInviteToken)
	otherMachine, err := machine.Parse("mh-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("parse other canonical machine: %v", err)
	}

	tests := []struct {
		name          string
		method        string
		path          string
		authorization string
		machine       string
		body          io.Reader
		duplicateAuth bool
		duplicateMach bool
		wantStatus    int
		wantCode      string
	}{
		{name: "encoded create path", method: http.MethodPost, path: "/v1/%70airing-invites", authorization: "Bearer " + harness.bearer, machine: harness.machine.String(), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "create query", method: http.MethodPost, path: "/v1/pairing-invites?retry=true", authorization: "Bearer " + harness.bearer, machine: harness.machine.String(), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "create body", method: http.MethodPost, path: "/v1/pairing-invites", authorization: "Bearer " + harness.bearer, machine: harness.machine.String(), body: strings.NewReader("{}"), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "create duplicate authorization", method: http.MethodPost, path: "/v1/pairing-invites", authorization: "Bearer " + harness.bearer, machine: harness.machine.String(), duplicateAuth: true, wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "create duplicate machine", method: http.MethodPost, path: "/v1/pairing-invites", authorization: "Bearer " + harness.bearer, machine: harness.machine.String(), duplicateMach: true, wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "create wrong method", method: http.MethodGet, path: "/v1/pairing-invites", authorization: "Bearer " + harness.bearer, machine: harness.machine.String(), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "encoded redeem path", method: http.MethodPost, path: "/v1/%70airings", authorization: "Skidbladnir-Invite " + invite.PairingInviteToken, machine: harness.machine.String(), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "query", method: http.MethodPost, path: "/v1/pairings?retry=true", authorization: "Skidbladnir-Invite " + invite.PairingInviteToken, machine: harness.machine.String(), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "body", method: http.MethodPost, path: "/v1/pairings", authorization: "Skidbladnir-Invite " + invite.PairingInviteToken, machine: harness.machine.String(), body: strings.NewReader("{}"), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "duplicate authorization", method: http.MethodPost, path: "/v1/pairings", authorization: "Skidbladnir-Invite " + invite.PairingInviteToken, machine: harness.machine.String(), duplicateAuth: true, wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "duplicate machine", method: http.MethodPost, path: "/v1/pairings", authorization: "Skidbladnir-Invite " + invite.PairingInviteToken, machine: harness.machine.String(), duplicateMach: true, wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "wrong method", method: http.MethodGet, path: "/v1/pairings", authorization: "Skidbladnir-Invite " + invite.PairingInviteToken, machine: harness.machine.String(), wantStatus: 400, wantCode: "InvalidRequest"},
		{name: "malformed token", method: http.MethodPost, path: "/v1/pairings", authorization: "Skidbladnir-Invite not-canonical", machine: harness.machine.String(), wantStatus: 401, wantCode: "PairingInviteRejected"},
		{name: "noncanonical token alias", method: http.MethodPost, path: "/v1/pairings", authorization: "Skidbladnir-Invite " + noncanonicalAlias, machine: harness.machine.String(), wantStatus: 401, wantCode: "PairingInviteRejected"},
		{name: "wrong canonical machine", method: http.MethodPost, path: "/v1/pairings", authorization: "Skidbladnir-Invite " + invite.PairingInviteToken, machine: otherMachine.String(), wantStatus: 401, wantCode: "PairingInviteRejected"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(test.method, harness.server.URL+test.path, test.body)
			if err != nil {
				t.Fatalf("create request: %v", err)
			}
			request.Header.Set("Authorization", test.authorization)
			request.Header.Set(machineHeader, test.machine)
			if test.duplicateAuth {
				request.Header.Add("Authorization", test.authorization)
			}
			if test.duplicateMach {
				request.Header.Add(machineHeader, test.machine)
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("perform request: %v", err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response: %v", err)
			}
			if response.StatusCode != test.wantStatus || !bytes.Contains(body, []byte(`"code":"`+test.wantCode+`"`)) {
				t.Fatalf("response = %d %s, want %d %s", response.StatusCode, body, test.wantStatus, test.wantCode)
			}
			if strings.HasPrefix(test.name, "encoded ") {
				lines := strings.Split(strings.TrimSpace(harness.logs.String()), "\n")
				last := lines[len(lines)-1]
				if !strings.Contains(last, `"http.route":"unmatched"`) {
					t.Fatalf("encoded path was classified as a canonical route: %s", last)
				}
			}
		})
	}

	status, _ := harness.redeem(invite.PairingInviteToken, harness.machine.String())
	if status != http.StatusOK {
		t.Fatalf("valid redemption after rejected requests = %d, want 200", status)
	}
}

func TestOrdinaryInventoryRequiresTheMachineHeader(t *testing.T) {
	harness := newPairingHarness(t)
	request, err := http.NewRequest(http.MethodGet, harness.server.URL+"/v1/sessions", nil)
	if err != nil {
		t.Fatalf("create inventory request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+harness.bearer)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("perform inventory request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read inventory rejection: %v", err)
	}
	if response.StatusCode != http.StatusConflict || !bytes.Contains(body, []byte(`"code":"MachineIdentityMismatch"`)) {
		t.Fatalf("headerless inventory response = %d %s, want 409 MachineIdentityMismatch", response.StatusCode, body)
	}
}

type pairingHarness struct {
	server     *httptest.Server
	slot       *pairing.Slot
	verifier   auth.FileVerifier
	bearerPath string
	bearer     string
	machine    machine.Handle
	logs       *bytes.Buffer
}

func newPairingHarness(t *testing.T) *pairingHarness {
	t.Helper()
	bearerPath := filepath.Join(t.TempDir(), "bearer")
	bearer, err := auth.Mint(auth.MintOptions{Path: bearerPath})
	if err != nil {
		t.Fatalf("mint bearer: %v", err)
	}
	handle, err := machine.Parse("mh-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("parse machine handle: %v", err)
	}
	harness := &pairingHarness{
		slot:       pairing.NewSlot(),
		verifier:   auth.FileVerifier{Path: bearerPath},
		bearerPath: bearerPath,
		bearer:     bearer,
		machine:    handle,
		logs:       &bytes.Buffer{},
	}
	harness.server = harness.newServer()
	t.Cleanup(harness.server.Close)
	return harness
}

func (harness *pairingHarness) restart(t *testing.T) *pairingHarness {
	t.Helper()
	restarted := &pairingHarness{
		slot:       pairing.NewSlot(),
		verifier:   harness.verifier,
		bearerPath: harness.bearerPath,
		bearer:     harness.bearer,
		machine:    harness.machine,
		logs:       &bytes.Buffer{},
	}
	restarted.server = restarted.newServer()
	t.Cleanup(restarted.server.Close)
	return restarted
}

func (harness *pairingHarness) newServer() *httptest.Server {
	return httptest.NewServer(New(Config{
		Pressure: pressure.NewMonitor(),
		Bearer:   harness.verifier,
		Pairing:  harness.slot,
		Logger:   logging.New(harness.logs),
		Machine:  harness.machine,
		Platform: platform.Descriptor{Kind: platform.KindLinux},
	}))
}

func (harness *pairingHarness) createInvite(t *testing.T) pairingInviteResponse {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, harness.server.URL+"/v1/pairing-invites", nil)
	if err != nil {
		t.Fatalf("create pairing-invite request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+harness.bearer)
	request.Header.Set(machineHeader, harness.machine.String())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("create pairing invite: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read pairing-invite response: %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("pairing-invite response = %d %s, want 201", response.StatusCode, body)
	}
	var invite pairingInviteResponse
	if err := json.Unmarshal(body, &invite); err != nil {
		t.Fatalf("decode pairing-invite response: %v", err)
	}
	if invite.Machine.Handle != harness.machine.String() || invite.Machine.Platform != platform.KindLinux {
		t.Fatalf("pairing-invite machine = %+v, want configured machine", invite.Machine)
	}
	parsedToken, err := pairing.ParseToken(invite.PairingInviteToken)
	if err != nil || parsedToken.String() != invite.PairingInviteToken {
		t.Fatalf("pairing-invite token is not canonical: %v", err)
	}
	return invite
}

func (harness *pairingHarness) redeem(token, handle string) (int, []byte) {
	request, err := http.NewRequest(http.MethodPost, harness.server.URL+"/v1/pairings", nil)
	if err != nil {
		return 0, []byte(err.Error())
	}
	request.Header.Set("Authorization", "Skidbladnir-Invite "+token)
	request.Header.Set(machineHeader, handle)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, []byte(err.Error())
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, []byte(err.Error())
	}
	return response.StatusCode, body
}

func assertPairingRejected(t *testing.T, status int, body []byte) {
	t.Helper()
	want := `{"code":"PairingInviteRejected","message":"This fleet invite is invalid, expired, or already used."}`
	if status != http.StatusUnauthorized || strings.TrimSpace(string(body)) != want {
		t.Fatalf("pairing rejection = %d %s, want 401 %s", status, body, want)
	}
}

func noncanonicalTokenAlias(t *testing.T, canonical string) string {
	t.Helper()
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, canonical[len(canonical)-1])
	if last < 0 || last%4 != 0 {
		t.Fatalf("canonical token has invalid final symbol: %q", canonical)
	}
	alias := canonical[:len(canonical)-1] + string(alphabet[last+1])
	canonicalBytes, err := base64.RawURLEncoding.DecodeString(canonical)
	if err != nil {
		t.Fatalf("decode canonical token: %v", err)
	}
	aliasBytes, err := base64.RawURLEncoding.DecodeString(alias)
	if err != nil || !bytes.Equal(aliasBytes, canonicalBytes) {
		t.Fatalf("test alias is not a noncanonical encoding of the invitation token")
	}
	return alias
}

type pairingInviteResponse struct {
	PairingInviteToken string     `json:"pairingInviteToken"`
	ExpiresAt          string     `json:"expiresAt"`
	Machine            machineDTO `json:"machine"`
}

type pairingResponse struct {
	Machine machineDTO `json:"machine"`
	Bearer  string     `json:"bearer"`
}
