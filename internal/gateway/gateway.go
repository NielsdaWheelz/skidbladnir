package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	pathpkg "path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/api"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

const MaximumBodyBytes int64 = 64 * 1024

type Config struct {
	Agents        *agentcontrol.Service
	Sessions      *sessions.Manager
	Workdir       *workdir.Service
	Pressure      *pressure.Monitor
	Fleet         *fleet.Service
	Notifications http.Handler
	Logger        *slog.Logger // the process's one logging.New; its doc is the event-line contract
	Machine       machine.Handle
}

type Gateway struct {
	agents        *agentcontrol.Service
	sessions      *sessions.Manager
	workdir       *workdir.Service
	pressure      *pressure.Monitor
	fleet         *fleet.Service
	notifications http.Handler
	logger        *slog.Logger
	machine       machine.Handle
	routes        *http.ServeMux
	notSent       map[string]bool // route patterns whose error answers default to dispatch not_sent

	// liveMutex guards the live connections, closing and the closed devices (terminal.go).
	liveMutex     sync.Mutex
	live          map[*liveConnection]struct{}
	closing       bool
	closedDevices map[string]bool
}

func New(config Config) *Gateway {
	gateway := &Gateway{
		agents:        config.Agents,
		sessions:      config.Sessions,
		workdir:       config.Workdir,
		pressure:      config.Pressure,
		fleet:         config.Fleet,
		notifications: config.Notifications,
		logger:        config.Logger,
		machine:       config.Machine,
		live:          make(map[*liveConnection]struct{}),
		closedDevices: make(map[string]bool),
		notSent:       make(map[string]bool),
	}
	// Each pattern without its method is the route name in the request log.
	gateway.routes = http.NewServeMux()
	// A session mutation's error answers were not dispatched unless the handler says unknown. PATCH
	// rename is not one: its InternalError can follow a tmux naming effect.
	mutation := func(pattern string, handler http.HandlerFunc) {
		gateway.routes.HandleFunc(pattern, handler)
		gateway.notSent[pattern] = true
	}
	gateway.routes.HandleFunc("GET /v1/sessions", gateway.listSessions)
	mutation("POST /v1/sessions", gateway.createSession)
	mutation("POST /v1/sessions/{tmuxId}/shell", gateway.createShell)
	mutation("PUT /v1/sessions/{tmuxId}/group", gateway.setSessionGroup)
	gateway.routes.HandleFunc("PATCH /v1/sessions/{tmuxId}", gateway.renameSession)
	mutation("DELETE /v1/sessions/{tmuxId}", gateway.killSession)
	gateway.routes.HandleFunc("GET /v1/sessions/{tmuxId}/terminal", gateway.openTerminal)
	mutation("POST /v1/sessions/{tmuxId}/terminal/{operation}", gateway.terminalOperation)
	gateway.routes.HandleFunc("POST /v1/directory-listings", gateway.listDirectory)
	gateway.routes.HandleFunc("POST /v1/directory-searches", gateway.searchDirectories)
	gateway.routes.HandleFunc("GET /v1/terminal-contexts/{connectionId}", gateway.readTerminalContext)
	gateway.routes.HandleFunc("GET /v1/profile-usage", gateway.readProfileUsage)
	gateway.routes.HandleFunc("GET /v1/pressure", gateway.readPressure)
	gateway.routes.HandleFunc("/", func(writer http.ResponseWriter, _ *http.Request) { writeError(writer, errorInvalidRequest) })
	return gateway
}

// ServeHTTP admits a request and routes it; the first failure answers.
func (gateway *Gateway) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	tracked := &trackedResponseWriter{ResponseWriter: writer}
	route := "unmatched"
	startedAt := time.Now()
	defer func() {
		if tracked.status == 0 {
			return
		}
		method := "OTHER"
		switch request.Method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			method = request.Method
		}
		attrs := []any{"http.request.method", method, "http.response.status_code", tracked.status, "http.route", route, "skidbladnir.duration.ms", time.Since(startedAt).Milliseconds()}
		if tracked.errorCode != "" {
			attrs = append(attrs, "skidbladnir.error.code", tracked.errorCode)
		}
		gateway.logger.Info("Request.Completed", attrs...)
	}()
	if request.URL.RawPath != "" || !strings.HasPrefix(path, "/v1") {
		writeError(tracked, errorInvalidRequest)
		return
	}
	// Fleet authenticates its own requests.
	if path == "/v1/fleet" || strings.HasPrefix(path, "/v1/fleet/") || strings.HasPrefix(path, "/v1/local/") {
		route = "/v1/fleet/{operation}"
		if strings.HasPrefix(path, "/v1/local/") {
			route = "/v1/local/{operation}"
		}
		gateway.fleet.Handler().ServeHTTP(tracked, request)
		if tracked.status >= 400 && tracked.errorCode == "" {
			tracked.errorCode = "FleetError"
		}
		return
	}
	notifications := strings.HasPrefix(path, "/v1/notifications/")
	// Clean also rejects trailing slashes, which no route has.
	canonical := pathpkg.Clean(path) == path
	if notifications {
		route = "/v1/notifications/{operation}"
	} else if _, pattern := gateway.routes.Handler(request); canonical && pattern != "/" {
		route = pattern[strings.IndexByte(pattern, ' ')+1:]
		if gateway.notSent[pattern] {
			tracked.dispatch = "not_sent"
		}
	}
	member, authenticated := gateway.authenticate(tracked, request)
	if !authenticated {
		return
	}
	switch auth.BindMachine(request, gateway.machine.String()) {
	case auth.AdmissionInvalid:
		writeError(tracked, errorInvalidRequest)
		return
	case auth.AdmissionMachineMismatch:
		writeError(tracked, errorMachineIdentityMismatch)
		return
	}
	if request.URL.RawQuery != "" {
		writeError(tracked, errorInvalidRequest)
		return
	}
	if request.ContentLength > MaximumBodyBytes {
		writeError(tracked, errorRequestTooLarge)
		return
	}
	if notifications {
		ctx, cancel := context.WithCancel(request.Context())
		defer cancel()
		connection := gateway.register(member.DeviceID, "", "", cancel)
		if connection == nil { // the device was closed or the gateway is closing
			if _, authenticated := gateway.authenticate(tracked, request); authenticated {
				writeError(tracked, errorUnauthenticated)
			}
			return
		}
		defer gateway.unregister(connection, nil)
		gateway.notifications.ServeHTTP(tracked, request.WithContext(ctx))
		if tracked.status >= 400 && tracked.errorCode == "" {
			tracked.errorCode = "NotificationsUnavailable"
		}
		return
	}
	// ServeMux would serve HEAD from GET routes and redirect non-canonical paths; the wire answers both 400.
	if request.Method == http.MethodHead || !canonical {
		writeError(tracked, errorInvalidRequest)
		return
	}
	gateway.routes.ServeHTTP(tracked, request.WithContext(context.WithValue(request.Context(), deviceContextKey{}, member.DeviceID)))
}

// deviceContextKey carries the authenticated member's DeviceID to routed handlers.
type deviceContextKey struct{}

func (gateway *Gateway) authenticate(writer *trackedResponseWriter, request *http.Request) (fleet.Member, bool) {
	member, err := gateway.fleet.Authenticate(request)
	if err == nil {
		return member, true
	}
	var failure *fleet.Error
	if errors.As(err, &failure) {
		writer.errorCode = "FleetError"
		if failure.Code == "Unauthenticated" {
			writer.errorCode = "Unauthenticated"
		}
		if failure.Status == http.StatusUnauthorized {
			writer.Header().Set("WWW-Authenticate", "Bearer")
		}
		writeJSON(writer, failure.Status, api.Error{Code: failure.Code, Message: failure.Message}, nil)
	} else {
		writeError(writer, errorInternal)
	}
	return fleet.Member{}, false
}

func (gateway *Gateway) apiMachine() api.Machine {
	platform := "Linux" // only darwin and linux build: pressure has no other collector
	if runtime.GOOS == "darwin" {
		platform = "Darwin"
	}
	return api.Machine{Handle: gateway.machine.String(), Platform: platform}
}

func (gateway *Gateway) readPressure(writer http.ResponseWriter, _ *http.Request) {
	startedAt := time.Now()
	snapshot := gateway.pressure.Snapshot()
	gateway.logger.Info("Pressure.Sampled", "skidbladnir.duration.ms", time.Since(startedAt).Milliseconds(), "skidbladnir.pressure.level", snapshot.Current.Level, "skidbladnir.pressure.reasons", snapshot.Current.Reasons)
	writeJSON(writer, http.StatusOK, snapshot, nil)
}

// decodeJSON strictly decodes one application/json body of at most MaximumBodyBytes into T, answering
// 400 or 413 itself when it cannot.
func decodeJSON[T any](writer http.ResponseWriter, request *http.Request) (T, bool) {
	var zero T
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || request.Header.Get("Content-Encoding") != "" {
		writeError(writer, errorInvalidRequest)
		return zero, false
	}
	// MaxBytesReader marks the connection for closure through an unexported
	// method of net/http's own writer, which the tracking wrapper would hide.
	request.Body = http.MaxBytesReader(writer.(*trackedResponseWriter).ResponseWriter, request.Body, MaximumBodyBytes)
	encoded, err := io.ReadAll(request.Body)
	var maximum *http.MaxBytesError
	if errors.As(err, &maximum) {
		writeError(writer, errorRequestTooLarge)
		return zero, false
	}
	// internal/strictjson owns this repository's one strict protocol decode.
	var decoded *T
	if err != nil || strictjson.Decode(encoded, &decoded) != nil || decoded == nil {
		writeError(writer, errorInvalidRequest)
		return zero, false
	}
	return *decoded, true
}

// valueOf reads an optional member; absent is empty.
func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func sessionFailure(err error) apiError {
	var sessionError *sessions.Error
	if !errors.As(err, &sessionError) {
		return errorInternal
	}
	switch sessionError.Code {
	case sessions.ErrorWorkingDirectoryInvalid:
		return errorWorkingDirectoryInvalid
	case sessions.ErrorWorkingDirectoryUnavailable:
		return errorWorkingDirectoryUnavailable
	case sessions.ErrorProfileUnknown:
		return errorProfileUnknown
	case sessions.ErrorSessionNameInvalid:
		return errorSessionNameInvalid
	case sessions.ErrorSessionNameChanged:
		return errorSessionNameChanged
	case sessions.ErrorSessionNameConflict:
		return errorSessionNameConflict
	case sessions.ErrorObjectiveInvalid:
		return errorObjectiveInvalid
	case sessions.ErrorSessionNotFound:
		return errorSessionNotFound
	case sessions.ErrorSessionIdentityMismatch:
		return errorSessionIdentityMismatch
	default:
		return errorInternal
	}
}

// writeJSON writes value and a newline. A non-nil tooLarge bounds the whole response to
// MaximumBodyBytes, answering *tooLarge instead, and marks it no-store; unbounded responses
// carry no cache header. The bounded routes are exactly the no-store ones, so one parameter
// carries both facts.
func writeJSON(writer http.ResponseWriter, status int, value any, tooLarge *apiError) {
	payload, err := json.Marshal(value)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	payload = append(payload, '\n')
	if tooLarge != nil {
		if len(payload) > int(MaximumBodyBytes) {
			writeError(writer, *tooLarge)
			return
		}
		writer.Header().Set("Cache-Control", "no-store")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_, _ = writer.Write(payload) // justify-ignore-error: a client disconnect after headers cannot be repaired.
}

// writeError fills an absent dispatch from the request's default, records the code for the
// request log and writes the envelope.
func writeError(writer http.ResponseWriter, failure apiError) {
	tracked := writer.(*trackedResponseWriter)
	if failure.Dispatch == "" {
		failure.Dispatch = tracked.dispatch
	}
	tracked.errorCode = failure.Code
	writeJSON(writer, failure.Status, failure.Error, nil)
}

// trackedResponseWriter records what the request log and the error envelope need.
type trackedResponseWriter struct {
	http.ResponseWriter
	status    int
	errorCode string // set with every status >= 400: the gateway's envelope code, else FleetError or NotificationsUnavailable
	dispatch  string // the envelope's default dispatch, decided from the matched route
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

func (writer *trackedResponseWriter) Flush() {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(writer.ResponseWriter).Flush() // justify-ignore-error: the streaming write owns disconnect handling.
}

func (gateway *Gateway) logObservationFailure(tmuxID string, reason sessions.StatusReason, elapsed time.Duration) {
	gateway.logger.Info("Terminal.ObservationFailed", "skidbladnir.duration.ms", elapsed.Milliseconds(), "skidbladnir.observation.reason", string(reason), "skidbladnir.session.tmux_id", tmuxID)
}
