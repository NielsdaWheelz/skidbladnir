package fleetclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/NielsdaWheelz/skidbladnir/internal/space"
)

func namedSpace(t *testing.T, text string) space.Label {
	t.Helper()
	label, err := space.Parse(text)
	if err != nil {
		t.Fatal("invalid synthetic label")
	}
	return label
}

func TestGroupsPreservePeerOrderAndStaleEvidence(t *testing.T) {
	alpha, upper, zulu := namedSpace(t, "alpha"), namedSpace(t, "Alpha"), namedSpace(t, "zulu")
	peers := []Peer{
		{Label: "first", OK: true, Sessions: []Session{{Name: "z", Space: alpha}, {Name: "b", Space: zulu}, {Name: "c"}}},
		{Label: "second", OK: false, Sessions: []Session{{Name: "a", Space: alpha}, {Name: "d", Space: upper}}},
	}
	groups := Groups(peers, space.Filter{})
	if len(groups) != 4 || groups[0].Space != upper || groups[1].Space != alpha || groups[2].Space != zulu || !groups[3].Space.IsUnassigned() {
		t.Fatal("group order or exact-case equality changed")
	}
	if groups[1].Rows[0].Session.Name != "z" || groups[1].Rows[1].Session.Name != "a" || groups[1].Rows[1].Available {
		t.Fatal("grouping reordered source rows or admitted stale evidence")
	}
	filter, _ := space.NamedFilter(alpha)
	if len(Groups(peers, filter)) != 1 || len(Groups(peers, space.UnassignedFilter())) != 1 || len(ObservedSpaces(peers)) != 3 {
		t.Fatal("filtering or observed suggestions changed source membership")
	}
}

func TestSpaceAssignmentRetainsSessionReferenceAndNeverReadsOrReplays(t *testing.T) {
	for _, clear := range []bool{false, true} {
		var calls atomic.Int32
		label := namedSpace(t, "alpha")
		if clear {
			label = space.Label{}
		}
		client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || r.Method != http.MethodPut || r.URL.Path != "/v1/sessions/$3/space" || !reflect.DeepEqual(body, map[string]string{"identityToken": "lifetime", "space": label.String()}) {
				t.Error("assignment changed retained session target or exact body")
			}
			w.WriteHeader(http.StatusNoContent)
		})
		ref, _ := DecodeReference(testRef())
		ref.Agent = nil
		result := client.Execute(context.Background(), Request{Operation: "space", Ref: ref.Encode(), Space: label})
		want, _ := json.Marshal(struct {
			Space string `json:"space"`
		}{label.String()})
		if !result.OK || calls.Load() != 1 || string(result.Value) != string(want) {
			t.Fatal("bodyless acknowledgement lost its exact requested result or required an agent")
		}
	}
	var calls atomic.Int32
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error("fixture could not drop acknowledgement")
			return
		}
		connection.Close()
	})
	result := client.Execute(context.Background(), Request{Operation: "space", Ref: testRef(), Space: namedSpace(t, "alpha")})
	if result.OK || result.Error.Dispatch != "unknown" || calls.Load() != 1 {
		t.Fatal("lost membership acknowledgement was replayed or given false certainty")
	}
}

func TestSpaceFilteredInventoryRetainsWirePeerAndHostOrder(t *testing.T) {
	first := strings.TrimSuffix(testSession, "}") + `,"space":"alpha"}`
	second := strings.Replace(first, `"reviewer"`, `"aaa"`, 1)
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { inventory(w, testMachine, first+","+second) })
	for _, filter := range []space.Filter{{}, space.UnassignedFilter()} {
		result := client.Execute(context.Background(), Request{Operation: "list", SpaceFilter: filter})
		var listed Inventory
		if !result.OK || json.Unmarshal(result.Value, &listed) != nil || len(listed.Peers) != 1 || !listed.Peers[0].OK || len(listed.Peers[0].Profiles) != 1 || listed.Peers[0].ObservedAt == "" {
			t.Fatal("filtered inventory lost source peer facts")
		}
		if filter.Kind() == space.FilterAll {
			if len(listed.Peers[0].Sessions) != 2 || listed.Peers[0].Sessions[0].Name != "reviewer" {
				t.Fatal("client reordered host rows")
			}
		} else if listed.Peers[0].Sessions == nil || len(listed.Peers[0].Sessions) != 0 {
			t.Fatal("empty filtered success lost its required array")
		}
	}
}

func TestSpaceCreateProjectsObservedMembershipAndStrictReplies(t *testing.T) {
	label := namedSpace(t, "alpha")
	client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["space"] != label.String() {
			t.Error("creation omitted initial membership")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"observedAt":"2026-09-15T00:00:00Z","session":`+testSession+`}`)
	})
	result := client.Execute(context.Background(), Request{Operation: "start", Kind: LaunchAgent, Machine: "arch", Name: "new", Profile: "work", Space: label})
	var observed ObservedSession
	if !result.OK || json.Unmarshal(result.Value, &observed) != nil || !observed.Session.Space.IsUnassigned() {
		t.Fatal("creation fabricated requested membership over observed result")
	}
}

func TestSpaceMalformedErrorNeverClaimsNonDispatch(t *testing.T) {
	for index, test := range []struct {
		status int
		body   string
	}{
		{422, `{"code":"SpaceInvalid","message":"use 1–64 nfc characters; only interior ordinary spaces, without display controls.","dispatch":"unknown"}`},
		{500, `{"code":"SpaceInvalid","message":"use 1–64 nfc characters; only interior ordinary spaces, without display controls.","dispatch":"not_sent"}`},
		{422, `{"code":"SpaceInvalid","message":"use 1–64 nfc characters; only interior ordinary spaces, without display controls."}`},
		{422, `{"code":"SpaceInvalid","message":"use 1–64 nfc characters; only interior ordinary spaces, without display controls.","dispatch":null}`},
		{422, `{"code":"SpaceInvalid","message":"use 1–64 nfc characters; only interior ordinary spaces, without display controls.","dispatch":"bogus"}`},
		{422, `{"code":"SpaceInvalid","message":"wrong message","dispatch":"not_sent"}`},
	} {
		client, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(test.status)
			io.WriteString(w, test.body)
		})
		result := client.Execute(context.Background(), Request{Operation: "space", Ref: testRef()})
		if result.OK || result.Error.Code != "protocol_error" || result.Error.Dispatch != "unknown" {
			t.Errorf("malformed membership error implied known dispatch: case=%d", index)
		}
	}
}

func TestSpaceFailureDispatchContract(t *testing.T) {
	for _, test := range []struct {
		code, message string
		status        int
	}{
		{"Unauthenticated", "Authentication required.", 401},
		{"MachineIdentityMismatch", "The machine identity changed. Fleet reset is required.", 409},
		{"InvalidRequest", "The request is not valid.", 400},
		{"RequestTooLarge", "The request is too large.", 413},
		{"SpaceInvalid", space.ErrInvalid.Error(), 422},
		{"SessionNotFound", "That session no longer exists.", 404},
		{"SessionIdentityMismatch", "The session changed. Refresh and try again.", 409},
		{"InternalError", "Skíðblaðnir could not complete the request.", 500},
	} {
		for _, dispatch := range []string{"not_sent", "unknown"} {
			encoded, _ := json.Marshal(struct {
				Code     string `json:"code"`
				Message  string `json:"message"`
				Dispatch string `json:"dispatch"`
			}{test.code, test.message, dispatch})
			failure := decodeMutationFailure("space", encoded, test.status)
			accepted := dispatch == "not_sent" || test.code == "InternalError"
			if accepted && (failure == nil || failure.Dispatch != dispatch || failure.Code != test.code) || !accepted && failure != nil {
				t.Fatal("closed membership error dispatch mapping differs from host contract")
			}
		}
	}
}
