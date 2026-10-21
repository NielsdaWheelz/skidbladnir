package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentcontrol"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/fleet"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/logging"
	"github.com/NielsdaWheelz/skidbladnir/internal/machine"
	"github.com/NielsdaWheelz/skidbladnir/internal/platform"
	"github.com/NielsdaWheelz/skidbladnir/internal/pressure"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/strictjson"
	"github.com/NielsdaWheelz/skidbladnir/internal/workdir"
)

const (
	MaximumBodyBytes int64 = 64 * 1024
	machineHeader          = "Skidbladnir-Machine"
)

type Config struct {
	Agents        *agentcontrol.Service
	Sessions      *sessions.Manager
	Workdir       *workdir.Service
	Pressure      *pressure.Monitor
	Fleet         *fleet.Service
	Notifications http.Handler
	Logger        logging.Logger
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
	logger        logging.Logger
	machine       machine.Handle
	platform      platform.Descriptor
	logMutex      sync.Mutex

	// The declared pressure capability is constant for one running gateway, so
	// it is projected onto the wire union once, here, instead of per request.
	unsupportedMetrics   []pressure.Metric
	unsupportedMetricSet map[pressure.Metric]struct{}

	terminalLifecycle  chan struct{}
	liveMutex          sync.Mutex
	liveConnections    map[uint64]*liveConnection
	nextLiveConnection uint64
	closing            bool
	closedDevices      map[string]bool
}

func New(config Config) *Gateway {
	if config.Workdir == nil {
		panic("gateway working directory service is not configured") // justify-defect: main must share one concrete validated workdir service with sessions and the gateway.
	}
	if config.Machine.String() == "" {
		panic("gateway machine identity is not configured") // justify-defect: gateway composition must load one canonical machine handle before construction.
	}
	switch config.Platform.Kind {
	case platform.KindLinux, platform.KindDarwin:
	default:
		panic("gateway platform is not configured") // justify-defect: platform.Current returns only the closed supported platform union.
	}
	if config.Fleet == nil {
		panic("gateway fleet service is not configured") // justify-defect: composition owns one fleet authority for authentication.
	}
	unsupportedMetrics, unsupportedMetricSet := mapUnsupportedMetrics(config.Pressure.Unsupported())
	return &Gateway{
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
		liveConnections:      make(map[uint64]*liveConnection),
		closedDevices:        make(map[string]bool),
		terminalLifecycle:    make(chan struct{}, 1),
	}
}

func (gateway *Gateway) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	tracked := &trackedResponseWriter{ResponseWriter: writer}
	if request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, "/v1/sessions/") && strings.HasSuffix(request.URL.Path, "/group") ||
		request.Method == http.MethodPost && (request.URL.Path == "/v1/sessions" || strings.HasPrefix(request.URL.Path, "/v1/sessions/") && strings.HasSuffix(request.URL.Path, "/shell")) {
		tracked.errorDispatch = "not_sent"
	}
	if strings.HasPrefix(request.URL.Path, "/v1/sessions/") && (strings.Contains(request.URL.Path, "/terminal/") || request.Method == http.MethodDelete) {
		tracked.errorDispatch = "not_sent"
	}
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
		case http.MethodPut:
			method = logging.MethodPut
		case http.MethodPatch:
			method = logging.MethodPatch
		case http.MethodDelete:
			method = logging.MethodDelete
		}
		if tracked.status >= 400 && tracked.errorCode == logging.ErrorNone {
			switch route {
			case logging.RouteFleet, logging.RouteLocal:
				tracked.errorCode = logging.ErrorFleet
			case logging.RouteNotifications:
				tracked.errorCode = logging.ErrorNotifications
			}
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
	gateway.serveHTTP(tracked, request)
}

func (gateway *Gateway) serveHTTP(writer *trackedResponseWriter, request *http.Request) {
	if request.URL.Path == "/healthz" {
		gateway.serveHealth(writer, request)
		return
	}
	if !strings.HasPrefix(request.URL.Path, "/v1") {
		writeError(writer, errorInvalidRequest)
		return
	}
	if request.URL.Path == "/v1/fleet" || strings.HasPrefix(request.URL.Path, "/v1/fleet/") || strings.HasPrefix(request.URL.Path, "/v1/local/") {
		gateway.fleet.Handler().ServeHTTP(writer, request)
		return
	}
	member, authenticated := gateway.authenticate(writer, request)
	if !authenticated {
		return
	}
	request = request.WithContext(context.WithValue(request.Context(), deviceContextKey{}, member.DeviceID))
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

	if strings.HasPrefix(request.URL.Path, "/v1/notifications/") && gateway.notifications != nil {
		ctx, cancel := context.WithCancel(request.Context())
		defer cancel()
		deviceID := request.Context().Value(deviceContextKey{}).(string)
		registration, admitted := gateway.registerLiveConnection(deviceID, "", "", cancel)
		if !admitted {
			if _, authenticated := gateway.authenticate(writer, request); authenticated {
				writeError(writer, errorUnauthenticated)
			}
			return
		}
		defer gateway.unregisterLiveConnection(registration, nil)
		gateway.notifications.ServeHTTP(writer, request.WithContext(ctx))
		return
	}
	switch {
	case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/v1/sessions/") && strings.HasSuffix(request.URL.Path, "/shell"):
		gateway.createShell(writer, request)
	case request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, "/v1/sessions/") && strings.HasSuffix(request.URL.Path, "/group"):
		gateway.setSessionGroup(writer, request)
	case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/v1/sessions/") && strings.Contains(request.URL.Path, "/terminal/"):
		gateway.terminalOperation(writer, request)
	case request.Method == http.MethodGet && request.URL.Path == "/v1/sessions":
		gateway.listSessions(writer, request)
	case request.Method == http.MethodGet && request.URL.Path == "/v1/profile-usage":
		gateway.readProfileUsage(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/v1/sessions":
		gateway.createSession(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/v1/directory-listings":
		gateway.listDirectory(writer, request)
	case request.Method == http.MethodPost && request.URL.Path == "/v1/directory-searches":
		gateway.searchDirectories(writer, request)
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/terminal-contexts/"):
		gateway.readTerminalContext(writer, request)
	case request.Method == http.MethodGet && request.URL.Path == "/v1/pressure":
		gateway.readPressure(writer)
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v1/sessions/") && strings.HasSuffix(request.URL.Path, "/terminal"):
		gateway.openTerminal(writer, request)
	case request.Method == http.MethodPatch && strings.HasPrefix(request.URL.Path, "/v1/sessions/"):
		gateway.renameSession(writer, request)
	case request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/v1/sessions/"):
		gateway.killSession(writer, request)
	default:
		writeError(writer, errorInvalidRequest)
	}
}

func (gateway *Gateway) listDirectory(writer http.ResponseWriter, request *http.Request) {
	input, failure := decodeJSON[directoryListingRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if !input.Directory.present {
		writeError(writer, errorInvalidRequest)
		return
	}
	directory, err := gateway.workdir.ParseBrowseDirectory(input.Directory.value)
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
	mapped, err := mapDirectoryListing(gateway.machineDTO(), listing)
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	writeDirectoryListing(writer, mapped)
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

var errDirectoryListingEncodingTooLarge = errors.New("directory listing encoding is too large")

type boundedJSONBuffer struct {
	buffer bytes.Buffer
}

func (buffer *boundedJSONBuffer) Write(contents []byte) (int, error) {
	if len(contents) > int(MaximumBodyBytes)-buffer.buffer.Len() {
		return 0, errDirectoryListingEncodingTooLarge
	}
	return buffer.buffer.Write(contents)
}

func writeDirectoryListing(writer http.ResponseWriter, listing directoryListingResponseDTO) {
	var encoded boundedJSONBuffer
	if err := json.NewEncoder(&encoded).Encode(listing); err != nil {
		if errors.Is(err, errDirectoryListingEncodingTooLarge) {
			writeError(writer, errorDirectoryListingTooLarge)
			return
		}
		writeError(writer, errorInternal)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(encoded.buffer.Bytes()) // justify-ignore-error: a client disconnect after headers cannot be repaired.
}

func (gateway *Gateway) bindMachine(writer http.ResponseWriter, request *http.Request) bool {
	switch auth.BindMachine(request, gateway.machine.String()) {
	case "":
	case auth.AdmissionInvalid:
		writeError(writer, errorInvalidRequest)
		return false
	case auth.AdmissionMachineMismatch:
		writeError(writer, errorMachineIdentityMismatch)
		return false
	default:
		panic("unhandled machine admission") // justify-defect: auth closes this result set.
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

type deviceContextKey struct{}

func (gateway *Gateway) authenticate(writer http.ResponseWriter, request *http.Request) (fleet.Member, bool) {
	member, _, err := gateway.fleet.Authenticate(request)
	if err == nil {
		return member, true
	}
	var failure *fleet.Error
	if errors.As(err, &failure) {
		if tracked, ok := writer.(interface{ setErrorCode(logging.ErrorCode) }); ok {
			code := logging.ErrorFleet
			if failure.Code == "Unauthenticated" {
				code = logging.ErrorUnauthenticated
			}
			tracked.setErrorCode(code)
		}
		if failure.Status == http.StatusUnauthorized {
			writer.Header().Set("WWW-Authenticate", "Bearer")
		}
		writeJSON(writer, failure.Status, struct {
			Code    string          `json:"code"`
			Message string          `json:"message"`
			Removal *fleet.Envelope `json:"removal,omitempty"`
		}{failure.Code, failure.Message, failure.Removal})
	} else {
		writeError(writer, errorInternal)
	}
	return fleet.Member{}, false
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
	response, err := gateway.observeSessions(request.Context())
	if err != nil {
		writeError(writer, errorInternal)
		return
	}
	writeJSON(writer, http.StatusOK, response)
}

func (gateway *Gateway) observeSessions(parent context.Context) (sessionsResponseDTO, error) {
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	observation := logging.SessionsObservation{Outcome: logging.ObservationFailed}
	defer func() {
		observation.Total = time.Since(startedAt)
		if errors.Is(parent.Err(), context.Canceled) {
			observation.Outcome = logging.ObservationCanceled
			observation.Count = 0
			if observation.Phase == logging.ObservationComplete {
				observation.Phase = logging.ObservationProjection
			}
		}
		event, err := logging.NewSessionsObserved(observation)
		if err != nil {
			panic("invalid sessions-observed log event") // justify-defect: phases and durations come from the closed local observation pipeline.
		}
		gateway.log(event)
	}()
	inventory, diagnostics, err := gateway.sessions.List(ctx)
	observation.Admission, observation.Capture = diagnostics.Admission, diagnostics.Capture
	observation.Normalize, observation.Foreground = diagnostics.Normalize, diagnostics.Foreground
	observation.Phase = logging.ObservationPhase(diagnostics.Phase)
	if err != nil {
		return sessionsResponseDTO{}, err
	}
	observation.Phase = logging.ObservationTerminal
	terminalStarted := time.Now()
	for _, failure := range gateway.agents.Enrich(ctx, inventory.Sessions) {
		gateway.logObservationFailure(failure.TmuxID, failure.Reason, failure.Elapsed)
	}
	terminalElapsed := time.Since(terminalStarted)
	observation.Terminal = &terminalElapsed
	if err := ctx.Err(); err != nil {
		return sessionsResponseDTO{}, err
	}
	observation.Phase = logging.ObservationProjection
	projectionStarted := time.Now()
	response, err := mapSessionsResponse(gateway.machineDTO(), inventory, gateway.sessions.Profiles())
	projectionElapsed := time.Since(projectionStarted)
	observation.Projection = &projectionElapsed
	if err != nil {
		return sessionsResponseDTO{}, err
	}
	if err := ctx.Err(); err != nil {
		return sessionsResponseDTO{}, err
	}
	observation.Outcome, observation.Phase = logging.ObservationSuccess, logging.ObservationComplete
	observation.Count = uint64(len(response.Sessions))
	return response, nil
}

func (gateway *Gateway) createSession(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	input, failure := decodeJSON[createSessionRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if !input.CWD.present {
		writeError(writer, errorInvalidRequest)
		return
	}
	switch input.Kind {
	case sessions.LaunchAgent:
		if !input.Profile.present {
			writeError(writer, errorInvalidRequest)
			return
		}
	case sessions.LaunchTerminal:
		if input.Profile.present {
			writeError(writer, errorInvalidRequest)
			return
		}
	default:
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
	label, err := group.Parse(input.Group.value)
	if err != nil || input.Group.present && label.IsUnassigned() {
		writeError(writer, errorGroupInvalid)
		return
	}
	createInput := sessions.CreateInput{
		Kind:             input.Kind,
		CWD:              input.CWD.value,
		Profile:          input.Profile.value,
		OptionalTmuxName: optionalTmuxName,
		Objective:        objective,
		Group:            label,
	}
	createInput, err = gateway.sessions.PreflightCreate(request.Context(), createInput)
	if err != nil {
		gateway.completeCreation(request.Context(), writer, sessions.ObservedSession{}, err, startedAt)
		return
	}
	created, err := gateway.sessions.Create(request.Context(), createInput)
	gateway.completeCreation(request.Context(), writer, created, err, startedAt)
}

func (gateway *Gateway) createShell(writer http.ResponseWriter, request *http.Request) {
	startedAt := time.Now()
	id, valid := parseSessionPath(strings.TrimSuffix(request.URL.Path, "/shell"))
	if !valid {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[shellSessionRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if input.IdentityToken.value == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	created, err := gateway.sessions.CreateShell(request.Context(), sessions.ShellInput{TmuxID: id, IdentityToken: input.IdentityToken.value})
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
		writeSessionError(writer, err)
		return
	}
	// Session.Created times creation alone; the status observation below is response latency.
	event, eventErr := logging.NewSessionCreated(created.Session.TmuxID, created.Session.LaunchProfile, time.Since(startedAt))
	if eventErr != nil {
		panic("invalid session-created log event") // justify-defect: creation minted the session identity and optional profile.
	}
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
	gateway.log(event)
	writeJSON(writer, http.StatusCreated, response)
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
	tmuxID, valid := parseSessionPath(request.URL.Path)
	if !valid {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[killSessionRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if input.IdentityToken.value == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	if err := gateway.sessions.ResolveSession(ctx, tmuxID, input.IdentityToken.value); err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	if err := gateway.closeSessionTerminal(ctx, tmuxID, input.IdentityToken.value); err != nil {
		writeError(writer, terminalFailure(err))
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (gateway *Gateway) renameSession(writer http.ResponseWriter, request *http.Request) {
	tmuxID, validPath := parseSessionPath(request.URL.Path)
	if !validPath {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[renameSessionRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if input.ExpectedNaming == nil || input.Naming == nil || !input.IdentityToken.present || input.IdentityToken.value == "" {
		writeError(writer, errorInvalidRequest)
		return
	}
	err := gateway.sessions.Rename(request.Context(), sessions.RenameInput{TmuxID: tmuxID, IdentityToken: input.IdentityToken.value, ExpectedNaming: input.ExpectedNaming.naming(), Naming: input.Naming.naming()})
	if err != nil {
		writeSessionError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func parseSessionPath(path string) (string, bool) {
	tmuxID := strings.TrimPrefix(path, "/v1/sessions/")
	return tmuxID, tmuxID != "" && tmuxID != path && !strings.ContainsRune(tmuxID, '/')
}

func (gateway *Gateway) setSessionGroup(writer http.ResponseWriter, request *http.Request) {
	id, valid := parseSessionPath(strings.TrimSuffix(request.URL.Path, "/group"))
	if !valid || len(id) < 2 || id[0] != '$' || strings.IndexFunc(id[1:], func(value rune) bool { return value < '0' || value > '9' }) >= 0 {
		writeError(writer, errorInvalidRequest)
		return
	}
	input, failure := decodeJSON[setSessionGroupRequest](writer, request)
	if failure != nil {
		writeError(writer, *failure)
		return
	}
	if input.IdentityToken.value == "" || !input.Group.present {
		writeError(writer, errorInvalidRequest)
		return
	}
	label, err := group.Parse(input.Group.value)
	if err != nil {
		writeError(writer, errorGroupInvalid)
		return
	}
	err = gateway.sessions.SetGroup(request.Context(), sessions.SetGroupInput{
		TmuxID: id, IdentityToken: input.IdentityToken.value, Group: label,
	})
	if errors.Is(err, sessions.ErrGroupDispatchUnknown) {
		failure := errorInternal
		failure.Dispatch = "unknown"
		writeError(writer, failure)
		return
	}
	if err != nil {
		writeSessionError(writer, err)
		return
	}
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
	// internal/strictjson owns this repository's one strict protocol decode:
	// it rejects duplicate object members, invalid UTF-8, unknown fields, and
	// trailing values. The HTTP ingress reads the capped body once and hands it
	// the bytes rather than restating a weaker subset of that contract.
	encoded, err := io.ReadAll(request.Body)
	if err != nil {
		var maximum *http.MaxBytesError
		if errors.As(err, &maximum) {
			failure := errorRequestTooLarge
			return zero, &failure
		}
		failure := errorInvalidRequest
		return zero, &failure
	}
	var decoded *T
	if err := strictjson.Decode(encoded, &decoded); err != nil || decoded == nil {
		failure := errorInvalidRequest
		return zero, &failure
	}
	return *decoded, nil
}

func writeSessionError(writer http.ResponseWriter, err error) {
	writeError(writer, sessionFailure(err))
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
	if tracked, ok := writer.(*trackedResponseWriter); ok && failure.Dispatch == "" {
		failure.Dispatch = tracked.errorDispatch
	}
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
	status        int
	errorCode     logging.ErrorCode
	errorDispatch string
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
	case path == "/v1/fleet" || strings.HasPrefix(path, "/v1/fleet/"):
		return logging.RouteFleet
	case strings.HasPrefix(path, "/v1/local/"):
		return logging.RouteLocal
	case strings.HasPrefix(path, "/v1/notifications/"):
		return logging.RouteNotifications
	case path == "/healthz":
		return logging.RouteHealth
	case path == "/v1/sessions":
		return logging.RouteSessions
	case path == "/v1/profile-usage":
		return logging.RouteProfileUsage
	case path == "/v1/pressure":
		return logging.RoutePressure
	case path == "/v1/directory-listings":
		return logging.RouteDirectoryListings
	case path == "/v1/directory-searches":
		return logging.RouteDirectorySearches
	case strings.HasPrefix(path, "/v1/terminal-contexts/"):
		return logging.RouteTerminalContexts
	case strings.HasPrefix(path, "/v1/sessions/") && strings.Contains(path, "/terminal/"):
		return logging.RouteTerminalControl
	case strings.HasPrefix(path, "/v1/sessions/") && strings.HasSuffix(path, "/shell"):
		return logging.RouteSessionShell
	case strings.HasPrefix(path, "/v1/sessions/") && strings.HasSuffix(path, "/group"):
		return logging.RouteSessionGroup
	case strings.HasPrefix(path, "/v1/sessions/") && strings.HasSuffix(path, "/terminal"):
		return logging.RouteTerminal
	case strings.HasPrefix(path, "/v1/sessions/") && !strings.ContainsRune(strings.TrimPrefix(path, "/v1/sessions/"), '/'):
		return logging.RouteSession
	default:
		return logging.RouteUnmatched
	}
}

func (gateway *Gateway) logObservationFailure(tmuxID string, reason sessions.StatusReason, elapsed time.Duration) {
	event, err := logging.NewTerminalObservationFailed(tmuxID, reason, elapsed)
	if err != nil {
		panic("invalid terminal-observation-failed log event") // justify-defect: only a valid unavailable status of a canonical session reaches this event.
	}
	gateway.log(event)
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

func (writer *trackedResponseWriter) Flush() {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(writer.ResponseWriter).Flush() // justify-ignore-error: the streaming write owns disconnect handling.
}
