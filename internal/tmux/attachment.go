package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
)

const (
	phoneShadowMarker           = "phone-shadow"
	attachmentColumns           = 80
	attachmentRows              = 24
	attachmentControlLimit      = 5 * time.Second
	attachmentReadyLimit        = 5 * time.Second
	attachmentReadyPollInterval = 25 * time.Millisecond
	attachmentExitLimit         = 2 * time.Second
	attachmentRecoveryLimit     = 3 * time.Second
)

var (
	phoneShadowNamePattern              = regexp.MustCompile(`^skid-phone-[0-9a-f]{32}$`)
	ErrAttachmentIdentityMismatch       = errors.New("tmux attachment identity changed")
	ErrAttachmentCleanupFailed          = errors.New("tmux attachment startup cleanup failed")
	errAttachmentCleanupIncomplete      = errors.New("tmux attachment cleanup is incomplete")
	errAttachmentCleanupReadbackInvalid = errors.New("tmux attachment cleanup readback is invalid")
)

type AttachmentSpec struct {
	SourceID   string
	SourceName string
	ShadowName string
	Server     ServerIdentity
}

type Presence struct {
	AttachedClients int
	OwnsGeometry    bool
}

type phoneShadowRecord struct {
	id            string
	name          string
	attached      int
	groupSize     int
	groupSizeText string
	server        ServerIdentity
}

type Attachment struct {
	client      Client
	spec        AttachmentSpec
	shadowID    string
	pty         *os.File
	command     *exec.Cmd
	processDone chan struct{}
	processErr  error

	closePTYOnce    sync.Once
	closePTYErr     error
	closeClientOnce sync.Once
	closeClientErr  error
}

func (client Client) StartAttachment(ctx context.Context, spec AttachmentSpec) (*Attachment, error) {
	arguments, err := attachmentCommandArguments(spec)
	if err != nil {
		return nil, err
	}
	controlContext, cancelControl := context.WithTimeout(ctx, attachmentControlLimit)
	output, err := client.Output(controlContext, "create-phone-shadow", arguments[0], arguments[1:]...)
	cancelControl()
	if err != nil {
		return nil, attachmentStartFailure(err, reconcileAttachmentShadow(client, spec))
	}
	shadowID, err := parseAttachmentCreationOutput(output)
	if err != nil {
		if errors.Is(err, ErrAttachmentIdentityMismatch) {
			return nil, err
		}
		return nil, attachmentStartFailure(err, reconcileAttachmentShadow(client, spec))
	}
	clientArguments, err := attachmentClientArguments(shadowID)
	if err != nil {
		return nil, attachmentStartFailure(err, reconcileAttachmentShadow(client, spec))
	}
	command := client.commandWithStderr(ctx, nil, nil, clientArguments[0], clientArguments[1:]...)
	command.Env = attachmentEnvironment(command.Env)
	terminalPTY, err := pty.StartWithSize(command, &pty.Winsize{Cols: attachmentColumns, Rows: attachmentRows})
	if err != nil {
		return nil, attachmentStartFailure(fmt.Errorf("start tmux phone client: %w", err), reconcileAttachmentShadow(client, spec))
	}
	attachment := &Attachment{
		client:      client,
		spec:        spec,
		shadowID:    shadowID,
		pty:         terminalPTY,
		command:     command,
		processDone: make(chan struct{}),
	}
	go func() {
		attachment.processErr = command.Wait()
		close(attachment.processDone)
	}()
	if err := attachment.awaitAttachedAndArm(ctx); err != nil {
		return nil, attachmentStartFailure(err, attachment.abortStartedClient())
	}
	return attachment, nil
}

func attachmentStartFailure(cause, cleanupErr error) error {
	if cleanupErr == nil {
		return cause
	}
	return errors.Join(cause, ErrAttachmentCleanupFailed, cleanupErr)
}

func (attachment *Attachment) Read(contents []byte) (int, error) {
	return attachment.pty.Read(contents)
}

func (attachment *Attachment) Write(contents []byte) (int, error) {
	return attachment.pty.Write(contents)
}

func (attachment *Attachment) Resize(columns, rows int) error {
	if columns <= 0 || columns > 65535 || rows <= 0 || rows > 65535 {
		return errors.New("terminal geometry is invalid")
	}
	return pty.Setsize(attachment.pty, &pty.Winsize{Cols: uint16(columns), Rows: uint16(rows)})
}

func (attachment *Attachment) Presence(ctx context.Context) (Presence, error) {
	output, err := attachment.client.Output(ctx, "read-phone-presence", "display-message", "-p", "-t", attachment.shadowID,
		"#{"+ServerEpochOption+"}|#{pid}|#{start_time}|#{session_id}|#{session_name}|#{@skid_internal}|#{session_group_attached}|#{session_attached}")
	if err != nil {
		return Presence{}, err
	}
	fields := strings.Split(output, "|")
	if len(fields) != 8 || fields[0] != attachment.spec.Server.Epoch || fields[1] != attachment.spec.Server.PID ||
		fields[2] != attachment.spec.Server.StartTime || fields[3] != attachment.shadowID ||
		fields[4] != attachment.spec.ShadowName || fields[5] != phoneShadowMarker {
		return Presence{}, ErrAttachmentIdentityMismatch
	}
	attachedText := fields[6]
	if attachedText == "" {
		attachedText = fields[7]
	}
	attached, err := strconv.Atoi(attachedText)
	if err != nil || attached < 1 {
		return Presence{}, errors.New("tmux attachment returned an invalid client count")
	}
	return Presence{AttachedClients: attached, OwnsGeometry: attached == 1}, nil
}

func (attachment *Attachment) ClosePTY() error {
	attachment.closePTYOnce.Do(func() {
		attachment.closePTYErr = attachment.pty.Close()
	})
	return attachment.closePTYErr
}

func (attachment *Attachment) CloseClient() error {
	attachment.closeClientOnce.Do(func() {
		select {
		case <-attachment.processDone:
			return
		case <-time.After(attachmentExitLimit):
		}
		if err := attachment.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			attachment.closeClientErr = fmt.Errorf("stop owned tmux phone client: %w", err)
			return
		}
		select {
		case <-attachment.processDone:
		case <-time.After(attachmentExitLimit):
			attachment.closeClientErr = errors.New("owned tmux phone client did not exit")
		}
	})
	return attachment.closeClientErr
}

func (attachment *Attachment) ReleaseShadow(ctx context.Context) error {
	condition, err := shadowReleaseCondition(attachment.shadowID, attachment.spec.ShadowName, attachment.spec.Server)
	if err != nil {
		return err
	}
	release, err := shadowReleaseCommand(attachment.shadowID, attachment.spec.ShadowName, attachment.spec.Server)
	if err != nil {
		return err
	}
	output, err := attachment.client.Output(ctx, "release-phone-shadow", "if-shell", "-F", "-t", attachment.shadowID,
		condition, release, "display-message -p -l '"+identityMismatchMarker+"'")
	if err != nil {
		exists, existsErr := attachment.client.HasSession(ctx, attachment.shadowID)
		if existsErr != nil {
			return existsErr
		}
		if !exists {
			return nil
		}
		return err
	}
	if output == "" {
		return nil
	}
	if output == identityMismatchMarker {
		return ErrAttachmentIdentityMismatch
	}
	return errors.New("tmux shadow release returned unexpected output")
}

func attachmentCommandArguments(spec AttachmentSpec) ([]string, error) {
	if !sessionIDPattern.MatchString(spec.SourceID) || spec.SourceName == "" ||
		!phoneShadowNamePattern.MatchString(spec.ShadowName) || !spec.Server.valid() {
		return nil, errors.New("tmux attachment identity is invalid")
	}
	condition := killIdentityCondition(spec.SourceID, spec.SourceName, spec.Server)
	shadowSessionTarget := "=" + spec.ShadowName
	shadowPaneTarget := shadowSessionTarget + ":"
	success := strings.Join([]string{
		"new-session -d -E -t '" + spec.SourceID + "' -s '" + spec.ShadowName + "'",
		"set-option -t '" + shadowPaneTarget + "' -- @skid_internal " + phoneShadowMarker,
		"set-option -pqu -t '" + spec.SourceID + "' -- @skid_attention",
		"display-message -p -t '" + shadowPaneTarget + "' '#{session_id}'",
	}, " ; ")
	return []string{
		"if-shell", "-F", "-t", spec.SourceID, condition, success,
		"display-message -p -l '" + identityMismatchMarker + "'",
	}, nil
}

func parseAttachmentCreationOutput(output string) (string, error) {
	if output == identityMismatchMarker {
		return "", ErrAttachmentIdentityMismatch
	}
	if !sessionIDPattern.MatchString(output) {
		return "", errors.New("tmux attachment creation returned an invalid shadow id")
	}
	return output, nil
}

func attachmentClientArguments(shadowID string) ([]string, error) {
	if !sessionIDPattern.MatchString(shadowID) {
		return nil, errors.New("tmux attachment client identity is invalid")
	}
	return []string{"-T", "RGB", "attach-session", "-E", "-f", "active-pane,ignore-size", "-t", shadowID}, nil
}

func parseAttachmentStartupObservation(output string, spec AttachmentSpec, shadowID string) (int, error) {
	if !phoneShadowNamePattern.MatchString(spec.ShadowName) || !spec.Server.valid() ||
		!sessionIDPattern.MatchString(shadowID) {
		return 0, errors.New("tmux attachment observation identity is invalid")
	}
	fields := strings.Split(output, "|")
	if len(fields) != 7 {
		return 0, errors.New("tmux attachment returned an invalid readiness observation")
	}
	if fields[0] != spec.Server.Epoch || fields[1] != spec.Server.PID || fields[2] != spec.Server.StartTime ||
		fields[3] != shadowID || fields[4] != spec.ShadowName || fields[5] != phoneShadowMarker {
		return 0, ErrAttachmentIdentityMismatch
	}
	attached, err := strconv.Atoi(fields[6])
	if err != nil || attached < 0 {
		return 0, errors.New("tmux attachment returned an invalid readiness client count")
	}
	return attached, nil
}

func attachmentArmArguments(spec AttachmentSpec, shadowID string) ([]string, error) {
	if !phoneShadowNamePattern.MatchString(spec.ShadowName) || !spec.Server.valid() ||
		!sessionIDPattern.MatchString(shadowID) {
		return nil, errors.New("tmux attachment arm identity is invalid")
	}
	condition := "#{&&:" + killIdentityCondition(shadowID, spec.ShadowName, spec.Server) +
		",#{&&:#{==:#{@skid_internal}," + phoneShadowMarker + "},#{>:#{session_attached},0}}}"
	return []string{
		"if-shell", "-F", "-t", shadowID, condition,
		"set-option -t '" + shadowID + ":' destroy-unattached keep-last",
		"display-message -p -l '" + identityMismatchMarker + "'",
	}, nil
}

func (attachment *Attachment) awaitAttachedAndArm(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, attachmentReadyLimit)
	defer cancel()
	// justify-polling: tmux exposes no notification when a spawned client becomes
	// attached; bounded control reads keep PTY bytes exclusively in the data plane.
	ticker := time.NewTicker(attachmentReadyPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-attachment.processDone:
			return attachment.exitBeforeReadyError()
		default:
		}

		output, err := attachment.client.Output(ctx, "observe-phone-shadow-attachment", "display-message", "-p", "-t", attachment.shadowID,
			"#{"+ServerEpochOption+"}|#{pid}|#{start_time}|#{session_id}|#{session_name}|#{@skid_internal}|#{session_attached}")
		if err != nil {
			select {
			case <-attachment.processDone:
				return attachment.exitBeforeReadyError()
			default:
			}
			if ctx.Err() != nil {
				return attachmentReadinessContextError(ctx.Err())
			}
			return err
		}
		attached, err := parseAttachmentStartupObservation(output, attachment.spec, attachment.shadowID)
		if err != nil {
			return err
		}
		if attached > 0 {
			if err := attachment.arm(ctx); err != nil {
				return err
			}
			select {
			case <-attachment.processDone:
				return attachment.exitBeforeReadyError()
			default:
				return nil
			}
		}

		select {
		case <-attachment.processDone:
			return attachment.exitBeforeReadyError()
		case <-ctx.Done():
			return attachmentReadinessContextError(ctx.Err())
		case <-ticker.C:
		}
	}
}

func (attachment *Attachment) arm(ctx context.Context) error {
	arguments, err := attachmentArmArguments(attachment.spec, attachment.shadowID)
	if err != nil {
		return err
	}
	output, err := attachment.client.Output(ctx, "arm-phone-shadow", arguments[0], arguments[1:]...)
	if err != nil {
		return err
	}
	switch output {
	case "":
		return nil
	case identityMismatchMarker:
		return ErrAttachmentIdentityMismatch
	default:
		return errors.New("tmux attachment arm returned unexpected output")
	}
}

func (attachment *Attachment) exitBeforeReadyError() error {
	if attachment.processErr == nil {
		return errors.New("tmux phone client exited before attachment")
	}
	return fmt.Errorf("tmux phone client exited before attachment: %w", attachment.processErr)
}

func attachmentReadinessContextError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("tmux attachment readiness timed out")
	}
	return fmt.Errorf("tmux attachment readiness canceled: %w", err)
}

func shadowReleaseCondition(id, name string, server ServerIdentity) (string, error) {
	if !sessionIDPattern.MatchString(id) || !phoneShadowNamePattern.MatchString(name) || !server.valid() {
		return "", errors.New("tmux shadow identity is invalid")
	}
	return "#{&&:" + killIdentityCondition(id, name, server) +
		",#{&&:#{==:#{@skid_internal}," + phoneShadowMarker + "},#{==:#{session_attached},0}}}", nil
}

func shadowReleaseCommand(id, name string, server ServerIdentity) (string, error) {
	if _, err := shadowReleaseCondition(id, name, server); err != nil {
		return "", err
	}
	return "if-shell -F -t '" + id + "' '#{>:#{session_group_size},1}' " +
		"\"kill-session -t '" + id + "'\" " +
		"\"set-option -u -t '" + id + "' destroy-unattached ; set-option -qu -t '" + id + "' -- @skid_internal\"", nil
}

// IsPhoneShadow closes the reserved ownership marker over the minted name
// namespace. A user session carrying only one of those facts is ordinary and
// must never be hidden or reconciled.
func IsPhoneShadow(name, marker string) bool {
	return marker == phoneShadowMarker && phoneShadowNamePattern.MatchString(name)
}

// ReconcilePhoneShadows removes only stale duplicate links and promotes a
// stale last link into an ordinary visible session. Protected names belong to
// live attachments in this gateway process and are never inspected for
// mutation.
func (client Client) ReconcilePhoneShadows(ctx context.Context, server ServerIdentity, protected []string) (bool, error) {
	return client.reconcilePhoneShadows(ctx, server, protected, "")
}

func (client Client) reconcilePhoneShadows(
	ctx context.Context,
	server ServerIdentity,
	protected []string,
	onlyName string,
) (bool, error) {
	if !server.valid() || onlyName != "" && !phoneShadowNamePattern.MatchString(onlyName) {
		return false, errors.New("tmux phone-shadow reconciliation identity is invalid")
	}
	protectedSet := make(map[string]struct{}, len(protected))
	for _, name := range protected {
		if !phoneShadowNamePattern.MatchString(name) {
			return false, errors.New("protected tmux phone-shadow name is invalid")
		}
		protectedSet[name] = struct{}{}
	}

	changed := false
	for {
		ids, err := client.ListSessionIDs(ctx)
		if err != nil || len(ids) == 0 {
			return changed, err
		}
		output, err := client.Output(ctx, "list-phone-shadows", "list-sessions", "-F",
			"#{session_id}|#{session_name}|#{@skid_internal}|#{session_attached}|#{session_group_size}|#{"+ServerEpochOption+"}|#{pid}|#{start_time}")
		if err != nil {
			remaining, listErr := client.ListSessionIDs(ctx)
			if listErr == nil && len(remaining) == 0 {
				return changed, nil
			}
			return changed, err
		}
		records, err := parsePhoneShadowRecords(output, server)
		if err != nil {
			return changed, err
		}

		// A successful mutation changes the group size recorded for every sibling;
		// never authorize another mutation from that stale topology snapshot.
		topologyChanged := false
		for _, record := range records {
			if onlyName != "" && record.name != onlyName {
				continue
			}
			if !phoneShadowNeedsReconciliation(record, protectedSet) {
				continue
			}
			reconciled, reconcileErr := client.reconcilePhoneShadow(ctx, record)
			if reconcileErr != nil {
				return changed, reconcileErr
			}
			if reconciled {
				changed = true
				topologyChanged = true
				break
			}
		}
		if !topologyChanged {
			return changed, nil
		}
	}
}

func parsePhoneShadowRecords(output string, expected ServerIdentity) ([]phoneShadowRecord, error) {
	if !expected.valid() {
		return nil, errors.New("expected tmux server identity is invalid")
	}
	if output == "" {
		return []phoneShadowRecord{}, nil
	}
	records := make([]phoneShadowRecord, 0)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "|")
		if len(fields) != 8 || !IsPhoneShadow(fields[1], fields[2]) {
			continue
		}
		if !sessionIDPattern.MatchString(fields[0]) {
			return nil, errors.New("tmux phone shadow has an invalid session id")
		}
		attached, attachedErr := strconv.Atoi(fields[3])
		groupSize := 1
		var groupErr error
		if fields[4] != "" {
			groupSize, groupErr = strconv.Atoi(fields[4])
		}
		observed := ServerIdentity{Epoch: fields[5], PID: fields[6], StartTime: fields[7]}
		if attachedErr != nil || attached < 0 || groupErr != nil || groupSize < 1 {
			return nil, errors.New("tmux phone shadow has invalid topology")
		}
		if observed != expected {
			return nil, ErrAttachmentIdentityMismatch
		}
		records = append(records, phoneShadowRecord{
			id: fields[0], name: fields[1], attached: attached,
			groupSize: groupSize, groupSizeText: fields[4], server: observed,
		})
	}
	return records, nil
}

func phoneShadowNeedsReconciliation(record phoneShadowRecord, protected map[string]struct{}) bool {
	_, active := protected[record.name]
	return !active && record.attached == 0
}

func (client Client) reconcilePhoneShadow(ctx context.Context, record phoneShadowRecord) (bool, error) {
	arguments, err := phoneShadowReconciliationArguments(record)
	if err != nil {
		return false, err
	}
	output, err := client.Output(ctx, "reconcile-phone-shadow", arguments[0], arguments[1:]...)
	if err != nil {
		exists, existsErr := client.HasSession(ctx, record.id)
		if existsErr == nil && !exists {
			return true, nil
		}
		return false, err
	}
	switch output {
	case "":
		return true, nil
	case identityMismatchMarker:
		return false, nil
	default:
		return false, errors.New("tmux phone-shadow reconciliation returned unexpected output")
	}
}

func phoneShadowReconciliationArguments(record phoneShadowRecord) ([]string, error) {
	if !sessionIDPattern.MatchString(record.id) || !phoneShadowNamePattern.MatchString(record.name) ||
		record.attached != 0 || record.groupSize < 1 || !record.server.valid() {
		return nil, errors.New("tmux phone-shadow reconciliation identity is invalid")
	}
	groupSizeText := record.groupSizeText
	if groupSizeText == "" && record.groupSize > 1 {
		groupSizeText = strconv.Itoa(record.groupSize)
	}
	if groupSizeText != "" {
		observedGroupSize, err := strconv.Atoi(groupSizeText)
		if err != nil || observedGroupSize != record.groupSize {
			return nil, errors.New("tmux phone-shadow reconciliation topology is invalid")
		}
	}
	condition := "#{&&:" + killIdentityCondition(record.id, record.name, record.server) +
		",#{&&:#{==:#{@skid_internal}," + phoneShadowMarker + "},#{&&:#{==:#{session_attached},0},#{==:#{session_group_size}," + groupSizeText + "}}}}"
	action := "kill-session -t '" + record.id + "'"
	if record.groupSize == 1 {
		action = "set-option -u -t '" + record.id + "' destroy-unattached ; " +
			"set-option -qu -t '" + record.id + "' -- @skid_internal"
	}
	return []string{
		"if-shell", "-F", "-t", record.id, condition, action,
		"display-message -p -l '" + identityMismatchMarker + "'",
	}, nil
}

func (attachment *Attachment) abortStartedClient() error {
	ptyErr := attachment.ClosePTY()
	clientErr := attachment.CloseClient()
	shadowErr := reconcileAttachmentShadow(attachment.client, attachment.spec)
	return errors.Join(ptyErr, clientErr, shadowErr)
}

func reconcileAttachmentShadow(client Client, spec AttachmentSpec) error {
	ctx, cancel := context.WithTimeout(context.Background(), attachmentRecoveryLimit)
	defer cancel()
	if _, err := client.reconcilePhoneShadows(ctx, spec.Server, nil, spec.ShadowName); err != nil {
		return err
	}
	output, err := client.Output(ctx, "verify-phone-shadow-cleanup", "list-sessions", "-F",
		"#{"+ServerEpochOption+"}|#{pid}|#{start_time}|#{@skid_internal}|#{session_name}")
	if err != nil {
		return err
	}
	return parseAttachmentCleanupPostcondition(output, spec)
}

func parseAttachmentCleanupPostcondition(output string, spec AttachmentSpec) error {
	if !phoneShadowNamePattern.MatchString(spec.ShadowName) || !spec.Server.valid() || output == "" {
		return errAttachmentCleanupReadbackInvalid
	}
	found := false
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(line, "|", 5)
		if len(fields) != 5 {
			return errAttachmentCleanupReadbackInvalid
		}
		observed := ServerIdentity{Epoch: fields[0], PID: fields[1], StartTime: fields[2]}
		if !observed.valid() || observed != spec.Server {
			return ErrAttachmentIdentityMismatch
		}
		if fields[4] != spec.ShadowName {
			continue
		}
		if found {
			return errAttachmentCleanupReadbackInvalid
		}
		found = true
		if fields[3] == phoneShadowMarker {
			return errAttachmentCleanupIncomplete
		}
	}
	return nil
}

func attachmentEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if strings.HasPrefix(entry, "TERM=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, "TERM=xterm-256color")
}
