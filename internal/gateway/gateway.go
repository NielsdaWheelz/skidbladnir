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
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
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
	Platform      platform.Descriptor
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
	platform      platform.Descriptor
	routes        *http.ServeMux

	// The declared pressure capability is constant for one running gateway, so
	// it is projected onto the wire union once, here, instead of per request.
	unsupportedMetrics   []pressure.Metric
	unsupportedMetricSet map[pressure.Metric]struct{}

	// liveMutex guards the live connections, closing and the closed devices (terminal.go).
	liveMutex     sync.Mutex
	live          map[*liveConnection]struct{}
	closing       bool
	closedDevices map[string]bool
}

func New(config Config) *Gateway {
	unsupportedMetrics, unsupportedMetricSet := mapUnsupportedMetrics(config.Pressure.Unsupported())
	gateway := &Gateway{
		agents:               config.Agents,
		sessions:             config.Sessions,
		workdir:              config.Workdir,
		pressure:             config.Pressure,
		fleet:                config.Fleet,
		notifications:        config.Notifications,
		logger:               config.Logger,
		machine:              config.Machine,
		platform:             config.Platform,
		unsupportedMetrics:   unsupportedMetrics,
		unsupportedMetricSet: unsupportedMetricSet,
		live:                 make(map[*liveConnection]struct{}),
		closedDevices:        make(map[string]bool),
	}
	// Each pattern without its method is the route name in the request log.
	gateway.routes = http.NewServeMux()
	gateway.routes.HandleFunc("GET /v1/sessions", gateway.listSessions)
	gateway.routes.HandleFunc("POST /v1/sessions", gateway.createSession)
	gateway.routes.HandleFunc("POST /v1/sessions/{tmuxId}/shell", gateway.createShell)
	gateway.routes.HandleFunc("PUT /v1/sessions/{tmuxId}/group", gateway.setSessionGroup)
	gateway.routes.HandleFunc("PATCH /v1/sessions/{tmuxId}", gateway.renameSession)
	gateway.routes.HandleFunc("DELETE /v1/sessions/{tmuxId}", gateway.killSession)
	gateway.routes.HandleFunc("GET /v1/sessions/{tmuxId}/terminal", gateway.openTerminal)
	gateway.routes.HandleFunc("POST /v1/sessions/{tmuxId}/terminal/{operation}", gateway.terminalOperation)
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
	// Every error envelope on a session-mutation path, routed or not, says it was not dispatched.
	if request.Method == http.MethodPut && strings.HasPrefix(path, "/v1/sessions/") && strings.HasSuffix(path, "/group") ||
		request.Method == http.MethodPost && (path == "/v1/sessions" || strings.HasPrefix(path, "/v1/sessions/") && strings.HasSuffix(path, "/shell")) ||
		strings.HasPrefix(path, "/v1/sessions/") && (strings.Contains(path, "/terminal/") || request.Method == http.MethodDelete) {
		tracked.dispatch = "not_sent"
	}
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

func (gateway *Gateway) listDirectory(writer http.ResponseWriter, request *http.Request) {
	input, ok := decodeJSON[directoryListingRequest](writer, request)
	if !ok {
		return
	}
	directory, err := gateway.workdir.ParseBrowseDirectory(input.Directory)
	if err != nil {
		writeDirectoryListingError(writer, err)
		return
	}
	listing, err := gateway.workdir.List(request.Context(), directory)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		writeDirectoryListingError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, mapDirectoryListing(gateway.machineDTO(), listing), &errorDirectoryListingTooLarge)
}

func writeDirectoryListingError(writer http.ResponseWriter, err error) {
	code, classified := workdir.ErrorCodeOf(err)
	if !classified {
		writeError(writer, errorInternal)
		return
	}
	switch code {
	case workdir.Invalid:
		writeError(writer, errorInvalidRequest)
	case workdir.Unavailable:
		writeError(writer, errorDirectoryListingUnavailable)
	case workdir.TooLarge:
		writeError(writer, errorDirectoryListingTooLarge)
	default:
		writeError(writer, errorInternal)
	}
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
		writeJSON(writer, failure.Status, struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{failure.Code, failure.Message}, nil)
	} else {
		writeError(writer, errorInternal)
	}
	return fleet.Member{}, false
}

func (gateway *Gateway) machineDTO() machineDTO {
	return machineDTO{Handle: gateway.machine.String(), Platform: gateway.platform.Kind}
}

func (gateway *Gateway) listSessions(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	inventory, err := gateway.sessions.List(ctx)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	for _, failure := range gateway.agents.Enrich(ctx, inventory.Sessions) {
		gateway.logObservationFailure(failure.TmuxID, failure.Reason, failure.Elapsed)
	}
	response, err := mapSessionsResponse(gateway.machineDTO(), inventory, gateway.sessions.Profiles())
	// The five-second budget covers enrichment and projection: a late inventory fails whole.
	if err != nil || ctx.Err() != nil {
		writeError(writer, errorInternal)
		return
	}
	writeJSON(writer, http.StatusOK, response, nil)
}

func (gateway *Gateway) createSession(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	input, ok := decodeJSON[createSessionRequest](writer, request)
	if !ok {
		return
	}
	if input.CWD == nil {
		writeError(writer, errorInvalidRequest)
		return
	}
	switch input.Kind {
	case sessions.LaunchAgent:
		if input.Profile == nil {
			writeError(writer, errorInvalidRequest)
			return
		}
	case sessions.LaunchTerminal:
		if input.Profile != nil {
			writeError(writer, errorInvalidRequest)
			return
		}
	default:
		writeError(writer, errorInvalidRequest)
		return
	}
	// Present but empty is invalid; absent means none.
	if input.OptionalTmuxName != nil && *input.OptionalTmuxName == "" {
		writeError(writer, errorSessionNameInvalid)
		return
	}
	if input.Objective != nil && *input.Objective == "" {
		writeError(writer, errorObjectiveInvalid)
		return
	}
	label, err := group.Parse(valueOf(input.Group))
	if err != nil || input.Group != nil && label.IsUnassigned() {
		writeError(writer, errorGroupInvalid)
		return
	}
	created, err := gateway.sessions.Create(request.Context(), sessions.CreateInput{
		Kind:             input.Kind,
		CWD:              *input.CWD,
		Profile:          valueOf(input.Profile),
		OptionalTmuxName: valueOf(input.OptionalTmuxName),
		Objective:        valueOf(input.Objective),
		Group:            label,
	})
	gateway.completeCreation(request.Context(), writer, created, err, startedAt)
}

func (gateway *Gateway) createShell(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	input, ok := decodeJSON[identityRequest](writer, request)
	if !ok {
		return
	}
	if input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	created, err := gateway.sessions.CreateShell(request.Context(), sessions.ShellInput{TmuxID: request.PathValue("tmuxId"), IdentityToken: input.IdentityToken})
	gateway.completeCreation(request.Context(), writer, created, err, startedAt)
}

func (gateway *Gateway) completeCreation(ctx context.Context, writer http.ResponseWriter, created sessions.ObservedSession, err error, startedAt time.Time) {
	if errors.Is(err, sessions.ErrCreateDispatchUnknown) {
		failure := errorInternal
		failure.Dispatch = "unknown"
		failure.Target = capturedCreationTarget(created)
		writeError(writer, failure)
		return
	}
	if err != nil {
		writeError(writer, sessionFailure(err))
		return
	}
	// Session.Created times creation alone; the status observation below is response latency.
	elapsed := time.Since(startedAt)
	observed := []sessions.Session{created.Session}
	for _, failure := range gateway.agents.Enrich(ctx, observed) {
		gateway.logObservationFailure(failure.TmuxID, failure.Reason, failure.Elapsed)
	}
	created.Session = observed[0]
	response, err := mapCreateSessionResponse(created, gateway.sessions.Profiles())
	if err != nil {
		failure := errorInternal
		failure.Dispatch = "unknown"
		failure.Target = capturedCreationTarget(created)
		writeError(writer, failure)
		return
	}
	attrs := []any{"skidbladnir.duration.ms", elapsed.Milliseconds(), "skidbladnir.session.tmux_id", created.Session.TmuxID}
	if created.Session.LaunchProfile != "" {
		attrs = append(attrs, "skidbladnir.session.launch_profile", string(created.Session.LaunchProfile))
	}
	gateway.logger.Info("Session.Created", attrs...)
	writeJSON(writer, http.StatusCreated, response, nil)
}

func capturedCreationTarget(created sessions.ObservedSession) *createdTargetDTO {
	// Manager only retains a partial observation once it has validated the
	// complete target. Projection errors retain the same original capture.
	if created.Session.TmuxID == "" || created.Session.IdentityToken == "" || created.Session.ActivePaneID == "" {
		return nil
	}
	return &createdTargetDTO{TmuxID: created.Session.TmuxID, IdentityToken: created.Session.IdentityToken, PaneID: created.Session.ActivePaneID}
}

func (gateway *Gateway) killSession(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 10*time.Second)
	defer cancel()
	tmuxID := request.PathValue("tmuxId")
	input, ok := decodeJSON[identityRequest](writer, request)
	if !ok {
		return
	}
	if input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	if err := gateway.sessions.ResolveSession(ctx, tmuxID, input.IdentityToken); err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	if err := gateway.closeSession(ctx, tmuxID, input.IdentityToken); err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) renameSession(writer http.ResponseWriter, request *http.Request) {
	input, ok := decodeJSON[renameSessionRequest](writer, request)
	if !ok {
		return
	}
	if input.ExpectedNaming == nil || input.Naming == nil || !input.ExpectedNaming.valid() || !input.Naming.valid() || input.IdentityToken == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	err := gateway.sessions.Rename(request.Context(), sessions.RenameInput{TmuxID: request.PathValue("tmuxId"), IdentityToken: input.IdentityToken, ExpectedNaming: input.ExpectedNaming.naming(), Naming: input.Naming.naming()})
	if err != nil {
		writeError(writer, sessionFailure(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) setSessionGroup(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("tmuxId")
	if !digitsAfter(id, '$') {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, ok := decodeJSON[setSessionGroupRequest](writer, request)
	if !ok {
		return
	}
	if input.IdentityToken == "" || input.Group == nil {
		writeError(writer, errorInvalidRequest)
		return
	}
	label, err := group.Parse(*input.Group)
	if err != nil {
		writeError(writer, errorGroupInvalid)
		return
	}
	err = gateway.sessions.SetGroup(request.Context(), sessions.SetGroupInput{
		TmuxID: id, IdentityToken: input.IdentityToken, Group: label,
	})
	if errors.Is(err, sessions.ErrGroupDispatchUnknown) {
		failure := errorInternal
		failure.Dispatch = "unknown"
		writeError(writer, failure)
		return
	}
	if err != nil {
		writeError(writer, sessionFailure(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) readPressure(writer http.ResponseWriter, _ *http.Request) {
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
	gateway.logger.Info("Pressure.Sampled", "skidbladnir.duration.ms", time.Since(startedAt).Milliseconds(), "skidbladnir.pressure.level", current.Level, "skidbladnir.pressure.reasons", current.Reasons)
	writeJSON(writer, http.StatusOK, pressureResponseDTO{Unsupported: gateway.unsupportedMetrics, Current: current, History: history}, nil)
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
// carry no cache header.
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
	writeJSON(writer, failure.Status, failure, nil)
}

// trackedResponseWriter records what the request log and the error envelope need.
type trackedResponseWriter struct {
	http.ResponseWriter
	status    int
	errorCode string // set with every status >= 400: the gateway's envelope code, else FleetError or NotificationsUnavailable
	dispatch  string // the envelope's default dispatch, decided from the method and path
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
