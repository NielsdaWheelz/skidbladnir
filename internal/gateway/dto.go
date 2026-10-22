package gateway

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NielsdaWheelz/skidbladnir/internal/agentruntime"
	"github.com/NielsdaWheelz/skidbladnir/internal/auth"
	"github.com/NielsdaWheelz/skidbladnir/internal/group"
	"github.com/NielsdaWheelz/skidbladnir/internal/sessions"
	"github.com/NielsdaWheelz/skidbladnir/internal/terminalcontext"
)

type apiError struct {
	Target   *createdTargetDTO `json:"target,omitempty"`
	Dispatch string            `json:"dispatch,omitempty"`
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Status   int               `json:"-"`
}

type createdTargetDTO struct {
	TmuxID        string `json:"tmuxId"`
	IdentityToken string `json:"identityToken"`
	PaneID        string `json:"paneId"`
}

var (
	errorUnauthenticated             = admissionError(auth.AdmissionUnauthenticated)
	errorInvalidRequest              = admissionError(auth.AdmissionInvalid)
	errorRequestTooLarge             = admissionError(auth.AdmissionTooLarge)
	errorWorkingDirectoryInvalid     = apiError{Code: "WorkingDirectoryInvalid", Message: "Choose a valid working directory.", Status: http.StatusUnprocessableEntity}
	errorWorkingDirectoryUnavailable = apiError{Code: "WorkingDirectoryUnavailable", Message: "That directory does not exist or cannot be opened.", Status: http.StatusUnprocessableEntity}
	errorDirectoryListingUnavailable = apiError{Code: "DirectoryListingUnavailable", Message: "This directory cannot be browsed. Enter the path instead.", Status: http.StatusUnprocessableEntity}
	errorDirectoryListingTooLarge    = apiError{Code: "DirectoryListingTooLarge", Message: "This directory has too many folders to show. Enter the path instead.", Status: http.StatusUnprocessableEntity}
	errorDirectorySearchUnavailable  = apiError{Code: "DirectorySearchUnavailable", Message: "Directory search is unavailable on this machine.", Status: http.StatusUnprocessableEntity}
	errorDirectorySearchTooLarge     = apiError{Code: "DirectorySearchTooLarge", Message: "Too many directory search results. Narrow the search.", Status: http.StatusUnprocessableEntity}
	errorTerminalContextUnavailable  = apiError{Code: "TerminalContextUnavailable", Message: "Remote context is unavailable.", Status: http.StatusNotFound}
	errorProfileUnknown              = apiError{Code: "ProfileUnknown", Message: "Choose an available profile.", Status: http.StatusUnprocessableEntity}
	errorSessionNameInvalid          = apiError{Code: "SessionNameInvalid", Message: "use 1–64 letters, numbers, underscores, or hyphens; start with a letter or number.", Status: http.StatusUnprocessableEntity}
	errorSessionNameChanged          = apiError{Code: "SessionNameChanged", Message: "the session name changed. review and save again.", Status: http.StatusConflict}
	errorSessionNameConflict         = apiError{Code: "SessionNameConflict", Message: "another session on this machine uses that name.", Status: http.StatusConflict}
	errorObjectiveInvalid            = apiError{Code: "ObjectiveInvalid", Message: "Use 1–240 characters without terminal controls.", Status: http.StatusUnprocessableEntity}
	errorGroupInvalid                = apiError{Code: "GroupInvalid", Message: group.ErrInvalid.Error(), Status: http.StatusUnprocessableEntity}
	errorSessionNotFound             = apiError{Code: "SessionNotFound", Message: "That session no longer exists.", Status: http.StatusNotFound}
	errorSessionIdentityMismatch     = apiError{Code: "SessionIdentityMismatch", Message: "The session changed. Refresh and try again.", Status: http.StatusConflict}
	errorMachineIdentityMismatch     = admissionError(auth.AdmissionMachineMismatch)
	errorInternal                    = admissionError(auth.AdmissionUnavailable)
)

func admissionError(code auth.AdmissionCode) apiError {
	status, message := code.Response()
	return apiError{Code: string(code), Message: message, Status: status}
}

type profileDTO struct {
	AccountScope string `json:"accountScope,omitempty"`
	Key          string `json:"key"`
	Label        string `json:"label"`
	Provider     string `json:"provider,omitempty"`
}

type characterDTO struct {
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

type agentDTO struct {
	PaneID        string `json:"paneId"`
	StartIdentity string `json:"startIdentity"`
	Provider      string `json:"provider"`
	PID           int    `json:"pid"`
	Profile       string `json:"profile,omitempty"`
	Label         string `json:"label,omitempty"`
}

type connectionDTO struct {
	Transport string `json:"transport"`
	ID        string `json:"id,omitempty"`
}

type sessionDTO struct {
	TerminalStatus  sessions.TerminalStatus  `json:"terminalStatus"`
	TerminalUsage   *sessions.TerminalUsage  `json:"terminalUsage,omitempty"`
	TerminalFooter  *sessions.TerminalFooter `json:"terminalFooter,omitempty"`
	TmuxID          string                   `json:"tmuxId"`
	ActivePaneID    string                   `json:"activePaneId"`
	TmuxName        string                   `json:"tmuxName"`
	NameMode        sessions.NameMode        `json:"nameMode"`
	IdentityToken   string                   `json:"identityToken"`
	Character       characterDTO             `json:"character"`
	LaunchProfile   string                   `json:"launchProfile,omitempty"`
	Agent           *agentDTO                `json:"agent,omitempty"`
	Connection      *connectionDTO           `json:"connection,omitempty"`
	Objective       string                   `json:"objective,omitempty"`
	Group           string                   `json:"group,omitempty"`
	CWD             string                   `json:"cwd,omitempty"`
	ActiveCommand   string                   `json:"activeCommand,omitempty"`
	AttachedClients int                      `json:"attachedClients"`

	Status sessions.StatusPresentation `json:"status"` // the phone's copy of TerminalStatus
}

type machineDTO struct {
	Handle   string `json:"handle"`
	Platform string `json:"platform"`
}

type sessionsResponseDTO struct {
	Machine    machineDTO   `json:"machine"`
	ObservedAt string       `json:"observedAt"`
	Recovery   recoveryDTO  `json:"recovery"`
	Profiles   []profileDTO `json:"profiles"`
	Sessions   []sessionDTO `json:"sessions"`
}

type recoveryDTO struct {
	State   sessions.RecoveryState `json:"state"`
	SavedAt string                 `json:"savedAt,omitempty"`
	Failure *recoveryFailureDTO    `json:"failure,omitempty"`

	Notice *sessions.RecoveryNotice `json:"notice,omitempty"` // the phone's copy of the fact
}

type recoveryFailureDTO struct {
	At    string                 `json:"at"`
	Phase sessions.RecoveryPhase `json:"phase"`
	Cause sessions.RecoveryCause `json:"cause"`
}

type createSessionResponseDTO struct {
	ObservedAt string     `json:"observedAt"`
	Session    sessionDTO `json:"session"`
}

// Request members decode null as absent. A pointer marks a member whose
// absence answers differently from its empty value.
type createSessionRequest struct {
	Kind             sessions.LaunchKind `json:"kind"`
	CWD              *string             `json:"cwd"`
	Profile          *string             `json:"profile"`
	OptionalTmuxName *string             `json:"optionalTmuxName"`
	Objective        *string             `json:"objective"`
	Group            *string             `json:"group"`
}

// identityRequest addresses a session lifetime: shell creation and deletion.
type identityRequest struct {
	IdentityToken string `json:"identityToken"`
}

type namingRequest struct {
	Mode string  `json:"mode"`
	Name *string `json:"name"`
}

// valid reports whether the naming has its mode's shape: automatic has no name, manual has one.
func (field namingRequest) valid() bool {
	return field.Mode == "automatic" && field.Name == nil || field.Mode == "manual" && field.Name != nil
}

func (field namingRequest) naming() sessions.Naming {
	return sessions.Naming{Mode: sessions.NameMode(field.Mode), Name: valueOf(field.Name)}
}

type renameSessionRequest struct {
	ExpectedNaming *namingRequest `json:"expectedNaming"`
	Naming         *namingRequest `json:"naming"`
	IdentityToken  string         `json:"identityToken"`
}

type setSessionGroupRequest struct {
	IdentityToken string  `json:"identityToken"`
	Group         *string `json:"group"`
}

// valueOf reads an optional member; absent is empty.
func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// digitsAfter reports whether value is prefix followed by one or more ASCII digits, the shape
// of tmux session ($N) and pane (%N) ids.
func digitsAfter(value string, prefix byte) bool {
	if len(value) < 2 || value[0] != prefix {
		return false
	}
	for _, character := range value[1:] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func mapProfiles(profiles []agentruntime.Profile) ([]profileDTO, error) {
	validated, err := agentruntime.ValidateProfiles(profiles)
	if err != nil {
		return nil, err
	}
	mapped := make([]profileDTO, len(validated))
	for index, profile := range validated {
		mapped[index] = profileDTO{Key: string(profile.Key), Label: profile.Label, Provider: profile.Provider.String()}
		// justify-ignore-error: absent account home omits this profile's quota scope without hiding its terminal launch.
		if scope, err := agentruntime.AccountScope(profile); err == nil {
			mapped[index].AccountScope = scope
		}
	}
	return mapped, nil
}

func mapAgent(agent *agentruntime.AgentRuntime, profiles []agentruntime.Profile) (*agentDTO, error) {
	if agent == nil {
		return nil, nil
	}
	if err := agentruntime.ValidateAgentRuntime(profiles, *agent); err != nil || agent.PaneID == "" || agent.StartIdentity == "" {
		return nil, errors.New("invalid agent runtime")
	}
	mapped := &agentDTO{
		PaneID: agent.PaneID, StartIdentity: string(agent.StartIdentity),
		Provider: agent.Provider.String(),
		PID:      int(agent.PID),
		Profile:  string(agent.Profile),
		Label:    profileLabel(profiles, agent.Profile),
	}
	return mapped, nil
}

// profileLabel is key's configured label; empty when key is empty or not
// configured. Provider agreement is ValidateAgentRuntime's invariant.
func profileLabel(profiles []agentruntime.Profile, key agentruntime.ProfileKey) string {
	for _, profile := range profiles {
		if profile.Key == key {
			return profile.Label
		}
	}
	return ""
}

func mapSession(session sessions.Session, profiles []agentruntime.Profile) (sessionDTO, error) {
	agent, err := mapAgent(session.Agent, profiles)
	if err != nil {
		return sessionDTO{}, err
	}
	var connection *connectionDTO
	if session.Connection != nil {
		if agent != nil || session.CWD != "" || session.Connection.Transport != "ssh" && session.Connection.Transport != "mosh" || session.Connection.ID != "" && !terminalcontext.ValidConnectionID(session.Connection.ID) {
			return sessionDTO{}, errors.New("invalid session connection")
		}
		connection = &connectionDTO{Transport: session.Connection.Transport, ID: session.Connection.ID}
	}
	if session.LaunchProfile != "" {
		matches := 0
		for _, profile := range profiles {
			if profile.Key == session.LaunchProfile {
				matches++
			}
		}
		if matches != 1 {
			return sessionDTO{}, errors.New("invalid session launch profile")
		}
	}
	card := sessionDTO{
		TmuxID:          session.TmuxID,
		ActivePaneID:    session.ActivePaneID,
		TmuxName:        session.TmuxName,
		NameMode:        session.NameMode,
		IdentityToken:   session.IdentityToken,
		Character:       characterDTO{Key: session.Character.Key, DisplayName: session.Character.DisplayName},
		LaunchProfile:   string(session.LaunchProfile),
		Agent:           agent,
		TerminalStatus:  session.TerminalStatus,
		TerminalUsage:   session.TerminalUsage,
		TerminalFooter:  session.TerminalFooter,
		Connection:      connection,
		Objective:       session.Objective,
		Group:           session.Group.String(),
		CWD:             session.CWD,
		ActiveCommand:   session.ActiveCommand,
		AttachedClients: session.AttachedClients,
	}
	if !card.TerminalStatus.Valid() || card.ActivePaneID == "" || agent != nil && agent.PaneID != card.ActivePaneID || card.TmuxID == "" || card.TmuxName == "" || card.NameMode != sessions.NameAutomatic && card.NameMode != sessions.NameManual || card.IdentityToken == "" ||
		card.Character.Key == "" || card.Character.DisplayName == "" || card.AttachedClients < 0 {
		return sessionDTO{}, errors.New("invalid required session facts")
	}
	if card.TerminalUsage != nil && (!card.TerminalUsage.ValidFor(card.TerminalStatus) || agent == nil || agent.Provider != agentruntime.ProviderClaude.String() && agent.Provider != agentruntime.ProviderCodex.String() || connection != nil) {
		return sessionDTO{}, errors.New("invalid terminal usage attachment")
	}
	if card.TerminalFooter != nil && (!card.TerminalFooter.ValidFor(card.TerminalStatus) || agent == nil || connection != nil) {
		return sessionDTO{}, errors.New("invalid terminal footer attachment")
	}
	card.Status = sessions.PresentStatus(card.TerminalStatus, agent != nil && connection == nil)
	return card, nil
}

func mapSessionsResponse(
	machine machineDTO,
	inventory sessions.Inventory,
	configuredProfiles []agentruntime.Profile,
) (sessionsResponseDTO, error) {
	encodedObservedAt, err := formatProjectionInstant(inventory.ObservedAt)
	if err != nil {
		return sessionsResponseDTO{}, errors.New("invalid inventory observation time")
	}
	recovery := recoveryDTO{State: inventory.Recovery.State, Notice: inventory.Recovery.Notice()}
	if inventory.Recovery.SavedAt != nil {
		recovery.SavedAt = inventory.Recovery.SavedAt.UTC().Format(time.RFC3339Nano)
	}
	if failure := inventory.Recovery.Failure; failure != nil {
		recovery.Failure = &recoveryFailureDTO{
			At:    failure.At.UTC().Format(time.RFC3339Nano),
			Phase: failure.Phase,
			Cause: failure.Cause,
		}
	}
	profiles, err := mapProfiles(configuredProfiles)
	if err != nil {
		return sessionsResponseDTO{}, err
	}
	cards := make([]sessionDTO, len(inventory.Sessions))
	for index, session := range inventory.Sessions {
		card, mapErr := mapSession(session, configuredProfiles)
		if mapErr != nil {
			return sessionsResponseDTO{}, mapErr
		}
		cards[index] = card
	}
	sort.Slice(cards, func(left, right int) bool {
		leftID, _ := strconv.ParseUint(strings.TrimPrefix(cards[left].TmuxID, "$"), 10, 64)
		rightID, _ := strconv.ParseUint(strings.TrimPrefix(cards[right].TmuxID, "$"), 10, 64)
		if leftID != rightID {
			return leftID < rightID
		}
		return cards[left].TmuxID < cards[right].TmuxID
	})
	return sessionsResponseDTO{
		Machine:    machine,
		ObservedAt: encodedObservedAt,
		Recovery:   recovery,
		Profiles:   profiles,
		Sessions:   cards,
	}, nil
}

func mapCreateSessionResponse(
	observed sessions.ObservedSession,
	profiles []agentruntime.Profile,
) (createSessionResponseDTO, error) {
	encodedObservedAt, err := formatProjectionInstant(observed.ObservedAt)
	if err != nil {
		return createSessionResponseDTO{}, errors.New("invalid session observation time")
	}
	card, err := mapSession(observed.Session, profiles)
	if err != nil {
		return createSessionResponseDTO{}, err
	}
	return createSessionResponseDTO{
		ObservedAt: encodedObservedAt,
		Session:    card,
	}, nil
}

func formatProjectionInstant(value time.Time) (string, error) {
	if !sessions.ValidProjectionInstant(value) {
		return "", errors.New("projection instant is outside canonical RFC 3339")
	}
	return value.UTC().Format(time.RFC3339Nano), nil
}
