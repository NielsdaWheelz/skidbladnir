package appserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	maximumHooksListBytes = 1 << 18
	maximumHooks          = 128
	maximumHookWarnings   = 64
	maximumHookTextBytes  = 4096
)

// HookSnapshot is the closed, content-free hooks/list projection used by the
// reviewed-hook policy. It is valid only when returned by DecodeHooksList.
type HookSnapshot struct {
	CWD   string
	Hooks []Hook
}

// Hook holds only fields the reviewed-hook policy must compare.
type Hook struct {
	Event       HookEvent
	Handler     HookHandler
	Command     string
	Async       bool
	Key         string
	SourcePath  string
	Source      HookSource
	Enabled     bool
	Managed     bool
	CurrentHash string
	Trust       HookTrust
	Matcher     *string
	TimeoutSec  uint64
}

type HookEvent string

const (
	HookEventPreToolUse        HookEvent = "preToolUse"
	HookEventPermissionRequest HookEvent = "permissionRequest"
	HookEventPostToolUse       HookEvent = "postToolUse"
	HookEventPreCompact        HookEvent = "preCompact"
	HookEventPostCompact       HookEvent = "postCompact"
	HookEventSessionStart      HookEvent = "sessionStart"
	HookEventSessionEnd        HookEvent = "sessionEnd"
	HookEventUserPromptSubmit  HookEvent = "userPromptSubmit"
	HookEventSubagentStart     HookEvent = "subagentStart"
	HookEventSubagentStop      HookEvent = "subagentStop"
	HookEventStop              HookEvent = "stop"
)

type HookHandler string

const (
	HookHandlerCommand HookHandler = "command"
	HookHandlerMCPTool HookHandler = "mcpTool"
	HookHandlerPrompt  HookHandler = "prompt"
	HookHandlerAgent   HookHandler = "agent"
)

type HookSource string

const (
	HookSourceSystem                  HookSource = "system"
	HookSourceUser                    HookSource = "user"
	HookSourceProject                 HookSource = "project"
	HookSourceMDM                     HookSource = "mdm"
	HookSourceSessionFlags            HookSource = "sessionFlags"
	HookSourcePlugin                  HookSource = "plugin"
	HookSourceCloudRequirements       HookSource = "cloudRequirements"
	HookSourceCloudManagedConfig      HookSource = "cloudManagedConfig"
	HookSourceLegacyManagedConfigFile HookSource = "legacyManagedConfigFile"
	HookSourceLegacyManagedConfigMDM  HookSource = "legacyManagedConfigMdm"
	HookSourceUnknown                 HookSource = "unknown"
)

type HookTrust string

const (
	HookTrustManaged   HookTrust = "managed"
	HookTrustUntrusted HookTrust = "untrusted"
	HookTrustTrusted   HookTrust = "trusted"
	HookTrustModified  HookTrust = "modified"
)

type hooksListParams struct {
	CWDs []string `json:"cwds"`
}

// ListHooks obtains Codex's single effective hooks snapshot for cwd. It does
// not discover configuration files or merge precedence itself; its only
// request is the pinned App Server hooks/list method and its only retained
// result is DecodeHooksList's content-free projection.
func ListHooks(ctx context.Context, connection Connection, codexHome, cwd string) (HookSnapshot, error) {
	if !cleanAbsolutePath(codexHome) || !cleanAbsolutePath(cwd) {
		return HookSnapshot{}, errProtocol
	}
	if err := initializeConnection(ctx, connection, codexHome); err != nil {
		return HookSnapshot{}, fmt.Errorf("initialize App Server connection: %w", err)
	}
	result, err := exchange(ctx, connection, 2, "hooks/list", hooksListParams{CWDs: []string{cwd}})
	if err != nil {
		return HookSnapshot{}, fmt.Errorf("list App Server hooks: %w", err)
	}
	snapshot, err := DecodeHooksList(result, cwd)
	if err != nil {
		return HookSnapshot{}, fmt.Errorf("validate App Server hooks: %w", err)
	}
	return snapshot, nil
}

// DecodeHooksList accepts the one-entry stable 0.149.1 hooks/list result for
// expectedCWD. Codex owns discovery and precedence; this adapter only narrows
// its effective output for the reviewed-hook policy.
func DecodeHooksList(result []byte, expectedCWD string) (HookSnapshot, error) {
	if len(result) == 0 || len(result) > maximumHooksListBytes || !cleanAbsolutePath(expectedCWD) {
		return HookSnapshot{}, errProtocol
	}
	fields, err := object(result)
	if err != nil || !hasExactly(fields, "data") {
		return HookSnapshot{}, errProtocol
	}
	data, err := array(fields["data"])
	if err != nil || len(data) != 1 {
		return HookSnapshot{}, errProtocol
	}
	entry, err := object(data[0])
	if err != nil || !hasExactly(entry, "cwd", "errors", "hooks", "warnings") {
		return HookSnapshot{}, errProtocol
	}
	cwd, err := boundedAbsolutePath(entry["cwd"])
	if err != nil || cwd != expectedCWD {
		return HookSnapshot{}, errProtocol
	}
	if err := validateHookWarnings(entry["warnings"]); err != nil {
		return HookSnapshot{}, err
	}
	if err := rejectHookErrors(entry["errors"]); err != nil {
		return HookSnapshot{}, err
	}
	rawHooks, err := array(entry["hooks"])
	if err != nil || len(rawHooks) > maximumHooks {
		return HookSnapshot{}, errProtocol
	}
	hooks := make([]Hook, 0, len(rawHooks))
	for _, rawHook := range rawHooks {
		hook, err := decodeHook(rawHook)
		if err != nil {
			return HookSnapshot{}, err
		}
		hooks = append(hooks, hook)
	}
	return HookSnapshot{CWD: cwd, Hooks: hooks}, nil
}

func decodeHook(raw json.RawMessage) (Hook, error) {
	fields, err := object(raw)
	if err != nil {
		return Hook{}, errProtocol
	}
	handler, err := hookHandler(fields["handlerType"])
	if err != nil {
		return Hook{}, err
	}
	if !hasHookFields(fields, handler) {
		return Hook{}, errProtocol
	}
	event, err := hookEvent(fields["eventName"])
	if err != nil {
		return Hook{}, err
	}
	key, err := boundedHookText(fields["key"])
	if err != nil {
		return Hook{}, err
	}
	matcher, err := optionalHookString(fields, "matcher")
	if err != nil {
		return Hook{}, err
	}
	if _, err := optionalHookString(fields, "statusMessage"); err != nil {
		return Hook{}, err
	}
	if _, err := optionalHookString(fields, "pluginId"); err != nil {
		return Hook{}, err
	}
	if raw, present := fields["additionalContextLimit"]; present {
		if err := nullableHookUint64(raw); err != nil {
			return Hook{}, err
		}
	}
	timeout, err := unsignedInteger(fields["timeoutSec"])
	if err != nil {
		return Hook{}, errProtocol
	}
	if _, err := integer(fields["displayOrder"]); err != nil {
		return Hook{}, err
	}
	sourcePath, err := boundedAbsolutePath(fields["sourcePath"])
	if err != nil {
		return Hook{}, err
	}
	source, err := hookSource(fields["source"])
	if err != nil {
		return Hook{}, err
	}
	enabled, err := boolValue(fields["enabled"])
	if err != nil {
		return Hook{}, err
	}
	managed, err := boolValue(fields["isManaged"])
	if err != nil {
		return Hook{}, err
	}
	hash, err := normalizedHookHash(fields["currentHash"])
	if err != nil {
		return Hook{}, err
	}
	trust, err := hookTrust(fields["trustStatus"])
	if err != nil {
		return Hook{}, err
	}
	hook := Hook{Event: event, Handler: handler, Key: key, SourcePath: sourcePath, Source: source, Enabled: enabled, Managed: managed, CurrentHash: hash, Trust: trust, Matcher: matcher, TimeoutSec: timeout}
	switch handler {
	case HookHandlerCommand:
		command, err := boundedHookText(fields["command"])
		if err != nil {
			return Hook{}, err
		}
		asynchronous := false
		if raw, present := fields["async"]; present {
			asynchronous, err = boolValue(raw)
			if err != nil {
				return Hook{}, err
			}
		}
		hook.Command, hook.Async = command, asynchronous
	case HookHandlerMCPTool:
		if _, err := boundedHookText(fields["server"]); err != nil {
			return Hook{}, err
		}
		if _, err := boundedHookText(fields["tool"]); err != nil {
			return Hook{}, err
		}
	}
	return hook, nil
}

func hasHookFields(fields map[string]json.RawMessage, handler HookHandler) bool {
	if !hasOnly(fields, hookFields(handler)...) || !hasRequired(fields, "currentHash", "displayOrder", "enabled", "eventName", "isManaged", "key", "source", "sourcePath", "timeoutSec", "trustStatus") {
		return false
	}
	switch handler {
	case HookHandlerCommand:
		return hasRequired(fields, "handlerType", "command")
	case HookHandlerMCPTool:
		return hasRequired(fields, "handlerType", "server", "tool")
	default:
		return hasRequired(fields, "handlerType")
	}
}

func hookFields(handler HookHandler) []string {
	fields := []string{"additionalContextLimit", "currentHash", "displayOrder", "enabled", "eventName", "handlerType", "isManaged", "key", "matcher", "pluginId", "source", "sourcePath", "statusMessage", "timeoutSec", "trustStatus"}
	switch handler {
	case HookHandlerCommand:
		return append(fields, "async", "command")
	case HookHandlerMCPTool:
		return append(fields, "server", "tool")
	default:
		return fields
	}
}

func validateHookWarnings(raw json.RawMessage) error {
	warnings, err := array(raw)
	if err != nil || len(warnings) > maximumHookWarnings {
		return errProtocol
	}
	for _, warning := range warnings {
		if _, err := boundedHookString(warning); err != nil {
			return err
		}
	}
	return nil
}

func rejectHookErrors(raw json.RawMessage) error {
	errors, err := array(raw)
	if err != nil || len(errors) != 0 {
		return errProtocol
	}
	return nil
}

func hookEvent(raw json.RawMessage) (HookEvent, error) {
	value, err := stringValue(raw)
	if err != nil {
		return "", errProtocol
	}
	switch HookEvent(value) {
	case HookEventPreToolUse, HookEventPermissionRequest, HookEventPostToolUse, HookEventPreCompact, HookEventPostCompact, HookEventSessionStart, HookEventSessionEnd, HookEventUserPromptSubmit, HookEventSubagentStart, HookEventSubagentStop, HookEventStop:
		return HookEvent(value), nil
	default:
		return "", errProtocol
	}
}

func hookHandler(raw json.RawMessage) (HookHandler, error) {
	value, err := stringValue(raw)
	if err != nil {
		return "", errProtocol
	}
	switch HookHandler(value) {
	case HookHandlerCommand, HookHandlerMCPTool, HookHandlerPrompt, HookHandlerAgent:
		return HookHandler(value), nil
	default:
		return "", errProtocol
	}
}

func hookSource(raw json.RawMessage) (HookSource, error) {
	value, err := stringValue(raw)
	if err != nil {
		return "", errProtocol
	}
	switch HookSource(value) {
	case HookSourceSystem, HookSourceUser, HookSourceProject, HookSourceMDM, HookSourceSessionFlags, HookSourcePlugin, HookSourceCloudRequirements, HookSourceCloudManagedConfig, HookSourceLegacyManagedConfigFile, HookSourceLegacyManagedConfigMDM, HookSourceUnknown:
		return HookSource(value), nil
	default:
		return "", errProtocol
	}
}

func hookTrust(raw json.RawMessage) (HookTrust, error) {
	value, err := stringValue(raw)
	if err != nil {
		return "", errProtocol
	}
	switch HookTrust(value) {
	case HookTrustManaged, HookTrustUntrusted, HookTrustTrusted, HookTrustModified:
		return HookTrust(value), nil
	default:
		return "", errProtocol
	}
}

func boundedAbsolutePath(raw json.RawMessage) (string, error) {
	value, err := boundedHookText(raw)
	if err != nil || !cleanAbsolutePath(value) {
		return "", errProtocol
	}
	return value, nil
}

func cleanAbsolutePath(value string) bool {
	return filepath.IsAbs(value) && filepath.Clean(value) == value
}

func boundedHookText(raw json.RawMessage) (string, error) {
	value, err := stringValue(raw)
	if err != nil || value == "" || len(value) > maximumHookTextBytes || strings.IndexByte(value, 0) >= 0 {
		return "", errProtocol
	}
	return value, nil
}

func nullableHookText(raw json.RawMessage) (*string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, err := boundedHookText(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func nullableHookString(raw json.RawMessage) (*string, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, err := boundedHookString(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func optionalHookString(fields map[string]json.RawMessage, name string) (*string, error) {
	raw, present := fields[name]
	if !present {
		return nil, nil
	}
	return nullableHookString(raw)
}

func boundedHookString(raw json.RawMessage) (string, error) {
	value, err := stringValue(raw)
	if err != nil || len(value) > maximumHookTextBytes || strings.IndexByte(value, 0) >= 0 {
		return "", errProtocol
	}
	return value, nil
}

func nullableHookUint64(raw json.RawMessage) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	_, err := unsignedInteger(raw)
	return err
}

func unsignedInteger(raw json.RawMessage) (uint64, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == 'n' || raw[0] == '"' {
		return 0, errProtocol
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, errProtocol
	}
	return value, nil
}

func boolValue(raw json.RawMessage) (bool, error) {
	value := bytes.TrimSpace(raw)
	switch {
	case bytes.Equal(value, []byte("true")):
		return true, nil
	case bytes.Equal(value, []byte("false")):
		return false, nil
	default:
		return false, fmt.Errorf("%w: boolean", errProtocol)
	}
}

func normalizedHookHash(raw json.RawMessage) (string, error) {
	value, err := stringValue(raw)
	if err != nil || len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return "", errProtocol
	}
	for _, character := range value[len("sha256:"):] {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return "", errProtocol
		}
	}
	return value, nil
}
