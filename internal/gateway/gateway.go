package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pairing"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

const (
	MaximumBodyBytes int64 = 64 * 1024
	machineHeader          = "Skidbladnir-Machine"
)

type sessionManager interface {
	Profiles() []agentruntime.Profile
	List(context.Context) ([]sessions.Session, error)
	Create(context.Context, sessions.CreateInput) (sessions.Session, error)
	ValidateKill(context.Context, sessions.KillInput) error
	Kill(context.Context, sessions.KillInput) error
	ValidateTerminal(context.Context, string, string) error
	OpenTerminal(context.Context, string, string) (*sessions.TerminalAttachment, error)
}

type Config struct {
	Sessions sessionManager
	Pressure *pressure.Monitor
	Bearer   auth.FileVerifier
	Pairing  *pairing.Slot
	Logger   logging.Logger
	Machine  machine.Handle
	Platform platform.Descriptor
}

type Gateway struct {
	sessions sessionManager
	pressure *pressure.Monitor
	bearer   auth.FileVerifier
	pairing  *pairing.Slot
	logger   logging.Logger
	machine  machine.Handle
	platform platform.Descriptor
	logMutex sync.Mutex

	// The declared pressure capability is constant for one running gateway, so
	// it is projected onto the wire union once, here, instead of per request.
	unsupportedMetrics   []pressure.Metric
	unsupportedMetricSet map[pressure.Metric]struct{}

	terminalLifecycle sync.Mutex
	liveMutex         sync.Mutex
	liveTerminals     map[uint64]*liveTerminal
	nextLiveTerminal  uint64
	closing           bool
}

func New(config Config) *Gateway {
	if config.Machine.String() == "" {
		panic("gateway machine identity is not configured") // justify-defect: gateway composition must load one canonical machine handle before construction.
	}
	switch config.Platform.Kind {
	case platform.KindLinux, platform.KindDarwin:
	default:
		panic("gateway platform is not configured") // justify-defect: platform.Current returns only the closed supported platform union.
	}
	if config.Pairing == nil {
		panic("gateway pairing slot is not configured") // justify-defect: every gateway process owns exactly one in-memory pairing slot.
	}
	unsupportedMetrics, unsupportedMetricSet := mapUnsupportedMetrics(config.Pressure.Unsupported())
	return &Gateway{
		sessions:             config.Sessions,
		pressure:             config.Pressure,
		bearer:               config.Bearer,
		pairing:              config.Pairing,
		logger:               config.Logger,
		machine:              config.Machine,
		platform:             config.Platform,
		unsupportedMetrics:   unsupportedMetrics,
		unsupportedMetricSet: unsupportedMetricSet,
		liveTerminals:        make(map[uint64]*liveTerminal),
	}
}

func (gateway *Gateway) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	tracked := &trackedResponseWriter{ResponseWriter: writer}
	startedAt := time.Now()
	route := logging.RouteUnmatched
	if request.URL.RawPath == "" {
		route = requestRoute(request.URL.Path)
	}
	defer func() {
		if tracked.status == 0 {
			return
		}
		method := logging.MethodOther
		switch request.Method {
		case http.MethodGet:
			method = logging.MethodGet
		case http.MethodPost:
			method = logging.MethodPost
		case http.MethodDelete:
			method = logging.MethodDelete
		}
		event, err := logging.NewRequestCompleted(method, route, tracked.status, time.Since(startedAt), tracked.errorCode)
		if err != nil {
			panic("invalid request-completion log event") // justify-defect: this handler closes every event field.
		}
		gateway.log(event)
	}()
	if request.URL.RawPath != "" {
		writeError(tracked, errorInvalidRequest)
		return
	}
	gateway.serveHTTP(tracked, request, route)
}

func (gateway *Gateway) serveHTTP(writer *trackedResponseWriter, request *http.Request, route logging.Route) {
	if request.URL.Path == "/healthz" {
		gateway.serveHealth(writer, request)
		return
	}
	if !strings.HasPrefix(request.URL.Path, "/v1") {
		writeError(writer, errorInvalidRequest)
		return
	}
	if request.URL.Path == "/v1/pairing-invites" {
		gateway.createPairingInvite(writer, request, route)
		return
	}
	if request.URL.Path == "/v1/pairings" {
		gateway.redeemPairingInvite(writer, request)
		return
	}
	if _, authenticated := gateway.authenticate(writer, request, route); !authenticated {
		return
	}
	if !gateway.bindMachine(writer, request) {
		return
	}
	if request.URL.RawQuery != "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	if request.ContentLength > MaximumBodyBytes {
		writeError(writer, errorRequestTooLarge)
		return
	}

	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/v1/sessions":
		gateway.listSessions(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/v1/sessions":
		gateway.createSession(writer, request)
	case request.Method == http.MethodGet && request.URL.Path == "/v1/pressure":
		gateway.readPressure(writer)
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/sessions/") && strings.HasSuffix(request.URL.Path, "/terminal"):
		gateway.openTerminal(writer, request)
	case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/v1/sessions/"):
		gateway.killSession(writer, request)
	default:
		writeError(writer, errorInvalidRequest)
	}
}

func (gateway *Gateway) bindMachine(writer http.ResponseWriter, request *http.Request) bool {
	values := request.Header.Values(machineHeader)
	if len(values) > 1 || (len(values) == 1 && strings.ContainsRune(values[0], ',')) {
		writeError(writer, errorInvalidRequest)
		return false
	}
	if len(values) == 0 {
		writeError(writer, errorMachineIdentityMismatch)
		return false
	}
	if values[0] != gateway.machine.String() {
		writeError(writer, errorMachineIdentityMismatch)
		return false
	}
	return true
}

func (gateway *Gateway) serveHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || request.URL.RawQuery != "" || !isLoopbackRemote(request.RemoteAddr) {
		writeError(writer, errorInvalidRequest)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(writer, "ok\n") // justify-ignore-error: the client disconnecting after the health status is not actionable.
}

func (gateway *Gateway) authenticate(writer http.ResponseWriter, request *http.Request, route logging.Route) (auth.Credential, bool) {
	var zero auth.Credential
	values := request.Header.Values("Authorization")
	if len(values) > 1 || len(values) == 1 && strings.ContainsRune(values[0], ',') {
		writeError(writer, errorInvalidRequest)
		return zero, false
	}
	credential, err := gateway.bearer.Read()
	if err != nil {
		writeError(writer, errorInternal)
		return zero, false
	}
	if len(values) != 1 || !credential.Verify(values[0]) {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		writeError(writer, errorUnauthenticated)
		event, eventErr := logging.NewAuthenticationRejected(route)
		if eventErr != nil {
			panic("invalid authentication-rejected log event") // justify-defect: requestRoute closes route names.
		}
		gateway.log(event)
		return zero, false
	}
	return credential, true
}

func (gateway *Gateway) createPairingInvite(writer http.ResponseWriter, request *http.Request, route logging.Route) {
	if request.Method != http.MethodPost {
		writeError(writer, errorInvalidRequest)
		return
	}
	credential, authenticated := gateway.authenticate(writer, request, route)
	if !authenticated {
		return
	}
	if !gateway.bindMachine(writer, request) {
		return
	}
	if !requireEmptyRequest(request) {
		writeError(writer, errorInvalidRequest)
		return
	}
	invite, err := gateway.pairing.Create(time.Now(), gateway.machine, credential)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	writeJSON(writer, http.StatusCreated, pairingInviteResponseDTO{
		PairingInviteToken: invite.Token.String(),
		ExpiresAt:          invite.ExpiresAt.Format(time.RFC3339Nano),
		Machine:            gateway.machineDTO(),
	})
}

func (gateway *Gateway) redeemPairingInvite(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, errorInvalidRequest)
		return
	}
	authorizationValues := request.Header.Values("Authorization")
	machineValues := request.Header.Values(machineHeader)
	if len(authorizationValues) > 1 || len(authorizationValues) == 1 && strings.ContainsRune(authorizationValues[0], ',') ||
		len(machineValues) > 1 || len(machineValues) == 1 && strings.ContainsRune(machineValues[0], ',') || !requireEmptyRequest(request) {
		writeError(writer, errorInvalidRequest)
		return
	}
	credential, err := gateway.bearer.Read()
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	encodedToken := ""
	if len(authorizationValues) == 1 {
		presentedToken, canonicalScheme := strings.CutPrefix(authorizationValues[0], "Skidbladnir-Invite ")
		if canonicalScheme {
			encodedToken = presentedToken
		}
	}
	var expectedMachine machine.Handle
	if len(machineValues) == 1 {
		parsedMachine, parseErr := machine.Parse(machineValues[0])
		if parseErr == nil {
			expectedMachine = parsedMachine
		}
	}
	if err := gateway.pairing.Redeem(time.Now(), encodedToken, expectedMachine, credential); err != nil {
		writeError(writer, errorPairingInviteRejected)
		return
	}
	writeJSON(writer, http.StatusOK, pairingResponseDTO{Machine: gateway.machineDTO(), Bearer: credential.CanonicalBearer()})
}

func (gateway *Gateway) machineDTO() machineDTO {
	return machineDTO{Handle: gateway.machine.String(), Platform: gateway.platform.Kind}
}

func requireEmptyRequest(request *http.Request) bool {
	if request.URL.RawQuery != "" || request.ContentLength != 0 || len(request.TransferEncoding) != 0 || request.Header.Get("Content-Type") != "" || request.Header.Get("Content-Encoding") != "" {
		return false
	}
	var one [1]byte
	read, err := request.Body.Read(one[:])
	return read == 0 && errors.Is(err, io.EOF)
}

func (gateway *Gateway) listSessions(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	listed, err := gateway.sessions.List(request.Context())
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	observedAt := time.Now().UTC()
	configuredProfiles := gateway.sessions.Profiles()
	profiles, err := mapProfiles(configuredProfiles)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	cards := make([]sessionDTO, len(listed))
	for index, session := range listed {
		card, mapErr := mapSession(session, configuredProfiles, observedAt)
		if mapErr != nil {
			writeError(writer, errorInternal)
			return
		}
		cards[index] = card
	}
	sort.Slice(cards, func(left, right int) bool {
		if cards[left].Attention != cards[right].Attention {
			return cards[left].Attention
		}
		leftPriority := sessionPriority(cards[left].Status.Kind)
		rightPriority := sessionPriority(cards[right].Status.Kind)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if cards[left].TmuxName != cards[right].TmuxName {
			return cards[left].TmuxName < cards[right].TmuxName
		}
		return cards[left].TmuxID < cards[right].TmuxID
	})
	event, eventErr := logging.NewSessionsListed(uint64(len(cards)), time.Since(startedAt))
	if eventErr != nil {
		panic("invalid sessions-listed log event") // justify-defect: count and duration are locally generated.
	}
	gateway.log(event)
	writeJSON(writer, http.StatusOK, sessionsResponseDTO{
		Machine:    gateway.machineDTO(),
		ObservedAt: observedAt.Format(time.RFC3339Nano),
		Profiles:   profiles,
		Sessions:   cards,
	})
}

func (gateway *Gateway) createSession(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	input, failure := decodeJSON[createSessionRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if !input.CWD.present || !input.Profile.present {
		writeError(writer, errorInvalidRequest)
		return
	}
	optionalTmuxName := ""
	if input.OptionalTmuxName.present {
		if input.OptionalTmuxName.value == "" {
			writeError(writer, errorSessionNameInvalid)
			return
		}
		optionalTmuxName = input.OptionalTmuxName.value
	}
	objective := ""
	if input.Objective.present {
		if input.Objective.value == "" {
			writeError(writer, errorObjectiveInvalid)
			return
		}
		objective = input.Objective.value
	}
	created, err := gateway.sessions.Create(request.Context(), sessions.CreateInput{
		CWD:              input.CWD.value,
		Profile:          input.Profile.value,
		OptionalTmuxName: optionalTmuxName,
		Objective:        objective,
	})
	if err != nil {
		writeSessionError(writer, err)
		return
	}
	card, err := mapSession(created, gateway.sessions.Profiles(), time.Now().UTC())
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	event, eventErr := logging.NewSessionCreated(created.TmuxID, created.TmuxName, agentruntime.ProfileKey(input.Profile.value), time.Since(startedAt))
	if eventErr != nil {
		panic("invalid session-created log event") // justify-defect: Create validated the profile key and tmux minted the id and name.
	}
	gateway.log(event)
	writeJSON(writer, http.StatusCreated, card)
}

func (gateway *Gateway) killSession(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	tmuxID := strings.TrimPrefix(request.URL.Path, "/v1/sessions/")
	if tmuxID == "" || strings.ContainsRune(tmuxID, '/') {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[killSessionRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if input.TmuxName == "" || input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	kill := sessions.KillInput{TmuxID: tmuxID, TmuxName: input.TmuxName, IdentityToken: input.IdentityToken}
	gateway.terminalLifecycle.Lock()
	defer gateway.terminalLifecycle.Unlock()
	if err := gateway.sessions.ValidateKill(request.Context(), kill); err != nil {
		writeSessionError(writer, err)
		return
	}
	if err := gateway.closeLiveTerminals(request.Context(), tmuxID); err != nil {
		writeError(writer, errorInternal)
		return
	}
	if err := gateway.sessions.Kill(request.Context(), kill); err != nil {
		writeSessionError(writer, err)
		return
	}
	event, eventErr := logging.NewSessionKilled(tmuxID, input.TmuxName, time.Since(startedAt))
	if eventErr != nil {
		panic("invalid session-killed log event") // justify-defect: sessions accepted the exact owned identity pair.
	}
	gateway.log(event)
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) readPressure(writer http.ResponseWriter) {
	startedAt := time.Now()
	snapshot := gateway.pressure.Snapshot()
	current, err := mapHostSample(snapshot.Current, gateway.unsupportedMetricSet)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	history := make([]pressureHistoryDTO, len(snapshot.Window))
	for index, sample := range snapshot.Window {
		mapped, mapErr := mapPressureHistory(sample)
		if mapErr != nil {
			writeError(writer, errorInternal)
			return
		}
		history[index] = mapped
	}
	level, reasons, err := pressureLogValues(snapshot.Current)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	event, eventErr := logging.NewPressureSampled(level, reasons, time.Since(startedAt))
	if eventErr != nil {
		panic("invalid pressure-sampled log event") // justify-defect: pressure values were exhaustively mapped.
	}
	gateway.log(event)
	writeJSON(writer, http.StatusOK, pressureResponseDTO{Unsupported: gateway.unsupportedMetrics, Current: current, History: history})
}

func decodeJSON[T any](writer http.ResponseWriter, request *http.Request) (T, *apiError) {
	var zero T
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || request.Header.Get("Content-Encoding") != "" {
		failure := errorInvalidRequest
		return zero, &failure
	}
	if request.ContentLength > MaximumBodyBytes {
		failure := errorRequestTooLarge
		return zero, &failure
	}
	// MaxBytesReader marks the connection for closure through an unexported
	// method of net/http's own writer, which the tracking wrapper would hide.
	limitWriter := writer
	if tracked, ok := writer.(*trackedResponseWriter); ok {
		limitWriter = tracked.ResponseWriter
	}
	request.Body = http.MaxBytesReader(limitWriter, request.Body, MaximumBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var decoded *T
	if err := decoder.Decode(&decoded); err != nil {
		var maximum *http.MaxBytesError
		if errors.As(err, &maximum) {
			failure := errorRequestTooLarge
			return zero, &failure
		}
		failure := errorInvalidRequest
		return zero, &failure
	}
	if decoded == nil {
		failure := errorInvalidRequest
		return zero, &failure
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		var maximum *http.MaxBytesError
		if errors.As(err, &maximum) {
			failure := errorRequestTooLarge
			return zero, &failure
		}
		failure := errorInvalidRequest
		return zero, &failure
	}
	return *decoded, nil
}

func writeSessionError(writer http.ResponseWriter, err error) {
	var sessionError *sessions.Error
	if !errors.As(err, &sessionError) {
		writeError(writer, errorInternal)
		return
	}
	switch sessionError.Code {
	case sessions.ErrorWorkingDirectoryInvalid:
		writeError(writer, errorWorkingDirectoryInvalid)
	case sessions.ErrorWorkingDirectoryUnavailable:
		writeError(writer, errorWorkingDirectoryUnavailable)
	case sessions.ErrorProfileUnknown:
		writeError(writer, errorProfileUnknown)
	case sessions.ErrorSessionNameInvalid:
		writeError(writer, errorSessionNameInvalid)
	case sessions.ErrorSessionNameConflict:
		writeError(writer, errorSessionNameConflict)
	case sessions.ErrorObjectiveInvalid:
		writeError(writer, errorObjectiveInvalid)
	case sessions.ErrorSessionNotFound:
		writeError(writer, errorSessionNotFound)
	case sessions.ErrorSessionIdentityMismatch:
		writeError(writer, errorSessionIdentityMismatch)
	case sessions.ErrorSessionGroupedConflict:
		writeError(writer, errorSessionGroupedConflict)
	default:
		writeError(writer, errorInternal)
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write(append(payload, '\n')) // justify-ignore-error: a client disconnect after headers cannot be repaired.
}

func writeError(writer http.ResponseWriter, failure apiError) {
	payload, err := json.Marshal(failure)
	if err != nil {
		panic("static API error failed JSON encoding") // justify-defect: every error is a fixed string pair.
	}
	writer.Header().Set("Content-Type", "application/json")
	if tracked, ok := writer.(interface{ setErrorCode(logging.ErrorCode) }); ok {
		tracked.setErrorCode(failure.logCode)
	}
	writer.WriteHeader(failure.Status)
	_, _ = writer.Write(append(payload, '\n')) // justify-ignore-error: a client disconnect after headers cannot be repaired.
}

type trackedResponseWriter struct {
	http.ResponseWriter
	status    int
	errorCode logging.ErrorCode
}

func (writer *trackedResponseWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *trackedResponseWriter) Write(contents []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	return writer.ResponseWriter.Write(contents)
}

func (writer *trackedResponseWriter) Unwrap() http.ResponseWriter {
	return writer.ResponseWriter
}

func (writer *trackedResponseWriter) setErrorCode(code logging.ErrorCode) {
	writer.errorCode = code
}

func requestRoute(path string) logging.Route {
	switch {
	case path == "/healthz":
		return logging.RouteHealth
	case path == "/v1/sessions":
		return logging.RouteSessions
	case path == "/v1/pairing-invites":
		return logging.RoutePairingInvites
	case path == "/v1/pairings":
		return logging.RoutePairings
	case path == "/v1/pressure":
		return logging.RoutePressure
	case strings.HasPrefix(path, "/v1/sessions/") && strings.HasSuffix(path, "/terminal"):
		return logging.RouteTerminal
	case strings.HasPrefix(path, "/v1/sessions/"):
		return logging.RouteSession
	default:
		return logging.RouteUnmatched
	}
}

func (gateway *Gateway) log(event logging.Event) {
	gateway.logMutex.Lock()
	defer gateway.logMutex.Unlock()
	err := gateway.logger.Write(event)
	if err == nil {
		return
	}
	var writeError *logging.WriteError
	if errors.As(err, &writeError) {
		_, _ = io.WriteString(os.Stderr, "{\"event.name\":\"Logging.WriteFailed\"}\n") // justify-ignore-error: observability cannot become product authority.
		return
	}
	panic("invalid structured log event") // justify-defect: only a programming/configuration error reaches this branch.
}

func isLoopbackRemote(remoteAddress string) bool {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
