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

	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
)

const MaximumBodyBytes int64 = 64 * 1024

type sessionManager interface {
	Profiles() []sessions.Profile
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
	Logger   logging.Logger
}

type Gateway struct {
	sessions sessionManager
	pressure *pressure.Monitor
	bearer   auth.FileVerifier
	logger   logging.Logger
	logMutex sync.Mutex

	terminalLifecycle sync.Mutex
	liveMutex         sync.Mutex
	liveTerminals     map[uint64]*liveTerminal
	nextLiveTerminal  uint64
	closing           bool
}

func New(config Config) *Gateway {
	return &Gateway{
		sessions:      config.Sessions,
		pressure:      config.Pressure,
		bearer:        config.Bearer,
		logger:        config.Logger,
		liveTerminals: make(map[uint64]*liveTerminal),
	}
}

func (gateway *Gateway) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	tracked := &trackedResponseWriter{ResponseWriter: writer}
	startedAt := time.Now()
	route := requestRoute(request.URL.Path)
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
	gateway.serveHTTP(tracked, request, route)
}

func (gateway *Gateway) serveHTTP(writer *trackedResponseWriter, request *http.Request, route logging.Route) {
	if request.URL.Path == "/healthz" {
		gateway.serveHealth(writer, request)
		return
	}
	if !strings.HasPrefix(request.URL.Path, "/v1") {
		writer.setErrorCode(logging.ErrorInvalidRequest)
		http.NotFound(writer, request)
		return
	}
	if !gateway.authenticate(writer, request, route) {
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

func (gateway *Gateway) serveHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || request.URL.RawQuery != "" || !isLoopbackRemote(request.RemoteAddr) {
		if tracked, ok := writer.(interface{ setErrorCode(logging.ErrorCode) }); ok {
			tracked.setErrorCode(logging.ErrorInvalidRequest)
		}
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	writer.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(writer, "ok\n") // justify-ignore-error: the client disconnecting after the health status is not actionable.
}

func (gateway *Gateway) authenticate(writer http.ResponseWriter, request *http.Request, route logging.Route) bool {
	values := request.Header.Values("Authorization")
	authorization := ""
	if len(values) == 1 {
		authorization = values[0]
	}
	valid, err := gateway.bearer.Verify(authorization)
	if err != nil {
		writeError(writer, errorInternal)
		return false
	}
	if len(values) != 1 || !valid {
		writer.Header().Set("WWW-Authenticate", "Bearer")
		writeError(writer, errorUnauthenticated)
		event, eventErr := logging.NewAuthenticationRejected(route)
		if eventErr != nil {
			panic("invalid authentication-rejected log event") // justify-defect: requestRoute closes route names.
		}
		gateway.log(event)
		return false
	}
	return true
}

func (gateway *Gateway) listSessions(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	listed, err := gateway.sessions.List(request.Context())
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	observedAt := time.Now().UTC()
	profiles, err := mapProfiles(gateway.sessions.Profiles())
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	cards := make([]sessionDTO, len(listed))
	for index, session := range listed {
		card, mapErr := mapSession(session, observedAt)
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
		if cards[left].Name != cards[right].Name {
			return cards[left].Name < cards[right].Name
		}
		return cards[left].ID < cards[right].ID
	})
	event, eventErr := logging.NewSessionsListed(uint64(len(cards)), time.Since(startedAt))
	if eventErr != nil {
		panic("invalid sessions-listed log event") // justify-defect: count and duration are locally generated.
	}
	gateway.log(event)
	writeJSON(writer, http.StatusOK, sessionsResponseDTO{
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
	optionalName := ""
	if input.OptionalName.present {
		if input.OptionalName.value == "" {
			writeError(writer, errorSessionNameInvalid)
			return
		}
		optionalName = input.OptionalName.value
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
		CWD:          input.CWD.value,
		Profile:      input.Profile.value,
		OptionalName: optionalName,
		Objective:    objective,
	})
	if err != nil {
		writeSessionError(writer, err)
		return
	}
	card, err := mapSession(created, time.Now().UTC())
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	event, eventErr := logging.NewSessionCreated(created.ID, created.Name, created.Profile, time.Since(startedAt))
	if eventErr != nil {
		panic("invalid session-created log event") // justify-defect: sessions returned an invalid owned session.
	}
	gateway.log(event)
	writeJSON(writer, http.StatusCreated, card)
}

func (gateway *Gateway) killSession(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	id := strings.TrimPrefix(request.URL.Path, "/v1/sessions/")
	if id == "" || strings.ContainsRune(id, '/') {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[killSessionRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if input.Name == "" || input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	kill := sessions.KillInput{ID: id, DisplayedName: input.Name, IdentityToken: input.IdentityToken}
	gateway.terminalLifecycle.Lock()
	defer gateway.terminalLifecycle.Unlock()
	if err := gateway.sessions.ValidateKill(request.Context(), kill); err != nil {
		writeSessionError(writer, err)
		return
	}
	if err := gateway.closeLiveTerminals(request.Context(), id); err != nil {
		writeError(writer, errorInternal)
		return
	}
	if err := gateway.sessions.Kill(request.Context(), kill); err != nil {
		writeSessionError(writer, err)
		return
	}
	event, eventErr := logging.NewSessionKilled(id, input.Name, time.Since(startedAt))
	if eventErr != nil {
		panic("invalid session-killed log event") // justify-defect: sessions accepted the exact owned identity pair.
	}
	gateway.log(event)
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) readPressure(writer http.ResponseWriter) {
	startedAt := time.Now()
	snapshot := gateway.pressure.Snapshot()
	current, err := mapHostSample(snapshot.Current)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	history := make([]hostSampleDTO, len(snapshot.Window))
	for index, sample := range snapshot.Window {
		mapped, mapErr := mapHostSample(sample)
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
	writeJSON(writer, http.StatusOK, pressureResponseDTO{Current: current, History: history})
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
	request.Body = http.MaxBytesReader(writer, request.Body, MaximumBodyBytes)
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
