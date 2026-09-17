# Optional agent runtime identity

this document owns optional process-lifetime registration. [agent control](agent-control.md)
owns the current foreground target, sampled status and controls. only claude
registration contributes projected runtime profile/provider-session id;
codex registration is ignored by the current terminal-only projection.

Normative rules: [`rules/index.md`](rules/index.md), especially
[`rules/testing.md`](rules/testing.md).

## Outcome

For the active pane of each visible ordinary tmux session, inventory may expose
an exact current foreground-agent observation. these are only its
registration-related identity fields; [agent control](agent-control.md) owns the
complete required shape:

```text
AgentRuntime {
  provider: Codex | Claude
  pid: positive integer
  profile?: ProfileKey
  providerSession?: ProviderSessionFacts
}

ProviderSessionFacts {
  id?: 1..128 visible ASCII bytes
  name?: 1..128 NFC Unicode scalars without controls or bidi formatting
}
```

At least one provider-session member is required when that object exists.
Unknown facts are omitted, never `null`, guessed, cached, or recovered from a
second registry. The session row's launch profile remains a separate fact and
never substitutes for an unproven runtime profile.

The machine-bound tmux id, current tmux name, and server-lifetime identity
token remain the mutation address. Provider ids and names are descriptive
facts, not keys, routes, authority, or uniqueness claims.

## One process-lifetime registration

Tmux remains the database. The sole identity registration is the pane option:

```text
@skid_agent_runtime =
  v1:<pid>:<kernel-start-id>:<Codex|Claude>:<profile-key|->:<session-id-b64url>
```

The sole writer command is:

```text
skidbladnir agent-hook --host-config=PATH {Codex|Claude} SessionStart
```

Every other provider/event pair is rejected before provider input or host
configuration is read. The bounded decoder retains only the documented
provider session id. It never retains or logs a prompt, objective, cwd, model,
transcript path, token, credential, or arbitrary provider payload.

Publication is permitted only when all of these describe one exact lifetime:

1. `TMUX_PANE` is a canonical pane id.
2. The configured absolute tmux binary resolves that pane's terminal device.
3. The hook process ancestry reaches a configured provider runtime that is the
   pane's foreground process.
4. Provider and optional profile environment match the validated host profile
   catalogue.
5. PID, kernel start identity, provider, and pane terminal remain the facts
   encoded into the registration.

One argv-form tmux command then replaces the option. Last-writer-wins is
correct because inventory accepts registered fields only while they still
match the independently observed foreground lifetime. A stale, malformed,
nested, wrong-pane, wrong-provider, or ambiguous registration is ignored and
never repaired. Pane death removes it naturally.

The helper is a fail-open optional observer: input, configuration, process,
or publication failure omits metadata and must not block provider startup.
Its diagnostics are fixed and content-free. Invocation outside the closed
command union is a deployment error and exits with usage failure.

## Projection

Inventory performs one foreground observation for the current pane and reuses
it for all optional identity fields:

```text
current pane foreground observation
  + optional matching @skid_agent_runtime
  + configured provider/profile catalogue
  -> optional AgentRuntime
```

Provider and PID may be projected from one unique configured foreground
signature. claude registration contributes profile and provider session id only when
its provider, PID, start identity, and pane origin match that same observation.
Claude's explicit live `-n`, `--name value`, or `--name=value` argument may
contribute a provider session name. Codex has no accepted name source.

Process replacement immediately loses stale registered fields. Tmux rename
does not change a valid process identity. Missing hooks, raw launches, absent
provider names, and unproven profiles are successful omission. A concrete
process matching more than one provider predicate is omitted fail-closed.

identity registration acceptance reads no other process environment, provider
files/apis, transcripts or terminal bytes. it never rewrites a registration or
adds a worker, poller or database. subsequent status/read enrichment belongs to
[agent control](agent-control.md).

## Host configuration

Every profile has the strict shape:

```json
{
  "key": "work",
  "label": "Codex · Work",
  "provider": "Codex",
  "command": "/absolute/launcher",
  "environment": [{"name": "CODEX_HOME", "value": "/absolute/home"}],
  "foregroundSignatures": [{"executableBase": "codex"}],
  "arguments": []
}
```

Provider is exactly `Codex | Claude`. Each Codex profile declares exactly one
absolute, provider-unique `CODEX_HOME`; each Claude profile does the same for
`CLAUDE_CONFIG_DIR`. Identical foreground signatures cannot cross providers.
Profile-home values are compared only inside the hook process and are never
returned or logged.

Deployment atomically installs one `SessionStart` command for each Codex
profile and one deployment-owned local Claude plugin for Claude profiles.
Neither path edits a provider trust store. A direct Claude binary launch that
bypasses the plugin remains honestly unregistered.

## API and presentation

the strict session shape contains optional `agent` identity. its complete
current fields are owned by [agent control](agent-control.md#identity-state-and-dispatch),
including pane/start identity, status and methods; the old codex registration
projection and flat activity field are absent.

`agent` is omitted when no supported exact foreground runtime is proven. No
alias, nullable member, alternate registration, dual decoder, schema version,
or compatibility path exists. Gateway mapping and Android ingress validate
the provider/profile relationship exhaustively.

The compact card footer renders the runtime profile when proven, otherwise
`<provider> · profile unknown`; without an agent it renders the launch profile
when known, otherwise `profile unknown`. Provider ids, provider names, and PID
stay off android cards; they add no badge, state bay, raw-id label or action
there. desktop details may show the accepted provider-session facts.

## Acceptance

1. Managed and laptop Codex/Claude sessions expose provider and PID only from
   one exact configured foreground observation.
2. a matching installed claude `SessionStart` registration can add the exact runtime
   profile and provider session id; absence remains successful omission.
3. Managed Claude may expose its explicit provider name; Codex honestly omits
   a name.
4. Wrong pane, process lifetime, provider, registration grammar, nesting, or
   ambiguous provider match contributes no registered metadata.
5. Process replacement and server/pane restart cannot inherit identity; tmux
   rename preserves a still-valid process identity.
6. Runtime profile never falls back to launch profile, and provider facts are
   never treated as unique addresses.
7. Gateway, Android, deployment, and the CLI admit exactly this one shape and
   one event. Active source and configuration contain no alternate writer.
8. missing or failed optional identity observation preserves an otherwise valid
   session row; it never fabricates an agent or authorizes a control target.
9. Logs and evidence contain no provider input, identifiers, argv, environment
   values, prompts, objectives, terminal bytes, credentials, or account data.

[testing policy](rules/testing.md) owns current verification. required external
boundaries need their applicable approval and evidence; unexecuted checks are
`NOT_RUN`, never passes. identity evidence alone proves no status or control behavior.

## Explicit trade-offs

| Decision | Cost accepted |
| --- | --- |
| One current pane | Agents in other panes or windows are not independently inventoried. |
| No provider API or transcript/config-store read | Some provider names and resumed-session facts remain unavailable. |
| No cross-process environment read | Raw launches may expose provider/PID but omit runtime profile and provider session id. |
| Ignore stale registration | A harmless pane option can remain until pane death; inventory stays read-only. |
| Fail open on projection failure | Optional identity can disappear temporarily; provider startup is not held hostage by phone metadata. |
| Atomic strict schema | Gateway, app, binary, and installed hooks must converge together; mixed versions are unsupported. |

## registration boundary

this registration owner does not implement provider work state, input or message
transport, prompt sending, scheduling,
wake/resume, provider discovery outside tmux, non-current-pane enumeration,
provider configuration or transcript parsing, provider-name synchronization,
history, provenance, receipts, durable orchestration, new routes, card-detail
UI, or compatibility/migration machinery.
