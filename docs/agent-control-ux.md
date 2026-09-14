# agent control: usable client, direct attachment

2026-09-13 spec · shipped in v0.4.1; a1–a9 verified on 2026-09-14.
baseline: skid `919e3d2`, dev-server `453d72c`, jarvis `1dbeee3`.
this document supersedes only the cli, attachment, and grouped-kill contracts in
[agent control](agent-control.md) and [architecture](architecture.md).
implementation must replace their affected normative sections; historical release
evidence remains historical. no compatibility path survives the cutover.

## outcome and limits

run `skid` on any configured linux/darwin machine: see the fleet, select a session,
enter its terminal, return to the list. ordinary commands work equally for humans
and agents. phone and desktop attach directly to the same tmux session.

- retain ordinary tmux discovery, one active agent per session, existing provider
  observations, direct peer routing, credentials, and three-host phone enrollment.
- bare names normally suffice. machine qualification resolves collisions; exact
  returned references support automation. no global name registry.
- delete skid's grouping and grouped-session refusal. shared window/pane navigation
  is intentional. unrelated user-created groups remain ordinary tmux objects.
- no search, attention system, layouts, scheduler, task schema, coordinator role,
  new agent launcher, native codex binding, transcript store, ssh transport,
  discovery service, mcp server, or new security/permission framework.

## public commands

install `skid` as a symlink to the existing binary. retain `skidbladnir` service and
administration commands; remove its `agent OPERATION` json-stdin client grammar.
default config: `~/.config/skidbladnir/client.json`; optional `--config PATH`.
reuse the existing peer schema, private-file checks, and direct authenticated client.

| command | behavior |
| --- | --- |
| `skid` | open tui; without a tty, print usage and exit 2 |
| `skid list [--machine arch]` | sessions and unavailable peers, including shell-only sessions |
| `skid info reviewer` | full metadata and fresh reference for this session |
| `skid enter reviewer` | attach; explicit detach returns to the caller |
| `skid read reviewer [--terminal] [--max-bytes N]` | existing bounded read; label source, scope, truncation |
| `skid send reviewer "review the patch" [--terminal]` | send once; alternative `--stdin` accepts literal text |
| `skid keys reviewer enter` | existing logical key vocabulary; 1–16 keys |
| `skid interrupt reviewer` | existing provider cancellation input; retain session |
| `skid stop reviewer` | best-effort agent halt, then exact session closure; report both outcomes |
| `skid kill reviewer` | close exactly this tmux session; works without an agent |
| `skid start reviewer --machine arch --profile work [--cwd '~']` | ordinary creation; cwd defaults to remote home; no initial prompt or readiness wait |

for commands targeting an existing session, replace the name with `--ref VALUE` or add
`--machine LABEL` to the name. these selector forms are mutually exclusive.
`--json` works on every noninteractive command. support the shown flag placement
and `--` for literal operands; publish complete usage in `skid --help`.
start requires machine, name, and advertised profile. commands never infer a host
from the caller's location. stdin text is exclusive with positional text, bounded
at the existing 32 kib limit; preserve newlines. normal human reads put text on
stdout and source/scope/truncation on stderr. no command logs prompt/output bytes.

## selection, identity, and results

1. resolve an unqualified name by exact, case-sensitive tmux-name equality across
   one fresh concurrent fleet inventory. one match succeeds; zero/duplicates fail
   with a useful error/candidate list. any unavailable peer makes this lookup
   incomplete: refuse before dispatch and suggest `--machine` or `--ref`.
2. qualified names query only that peer. references route by machine handle through
   configured peers and never fall back to a name. all mutations remain one attempt.
3. references are opaque to callers: unpadded base64url of strict json below,
   maximum 4096 characters. no credentials, signatures, registry, expiry, or cache.
   session operations use the session identity; agent operations also require the
   process identity. the host performs the existing final lifetime validation.

```text
ref payload = {machine, tmuxId, identityToken,
               agent?: {paneId, pid, startIdentity}}
row = {name, ref, cwd?, activeCommand?, launchProfile?, attachedClients,
       agent?: {provider, profile?, providerSession?, status, methods}}
peer = {label, machine, ok,
        observedAt?, profiles?, sessions?: [row], error?}
inventory = {partial: boolean, peers: [peer]}
success = {ok: true, result: ...}
failure = {ok: false, error: {code, dispatch: not_sent | unknown}}
```

reuse the existing field types/enums and strict decoders. successful peers have
observedAt/profiles/sessions; failed peers have error. `list` returns inventory;
`info` and `start` return `{label, machine, observedAt, session: row}`. other results
retain the current agent-control schema; `kill` returns `{terminal: closed}` only
after confirmed deletion. `--json` emits exactly one envelope on stdout, with no
human decoration. a partial list is a success envelope with `partial: true` and
nonzero exit status. local selector codes are `name_not_found`, `name_ambiguous`,
`inventory_incomplete`, existing `machine_unknown`, and `invalid_input` for malformed
refs. preserve host stale-target codes. selector failures have `dispatch: not_sent`;
human errors list ambiguous candidates, json retains the compact code envelope.
apply the existing 1 mib inventory limit to the final projected envelope; reject
overflow, never silently omit rows.

names are absent from the reference, so rename does not invalidate it. `info` and
`kill` retrieve current metadata from the referenced host and require the same
session lifetime; kill supplies the current name to the existing delete contract.
`info --ref` observes the exact session now, even if its previous agent exited;
it returns the newly observed agent reference. it never refreshes a mutation target.
start does not claim an agent is ready. an old agent reference cannot control a
replacement process or newly selected pane. an old session reference cannot bind
to a recreated session. callers never reconstruct the host's six-field target.

exit 0: complete result with the requested effect confirmed to the returned
contract (`written` means input delivered, never task completed). exit 1: operational
failure, partial inventory, unknown delivery, or unconfirmed stop/closure. exit 2:
invalid usage. successful bounded reads exit 0 even when `truncated: true`.
preserve partial stop results even with exit 1; nonzero never authorizes replay.

## one small tui

one model and table: machine, name, provider/profile or shell, state/source, cwd.
stable sort by configured peer order then name; selection follows session lifetime,
not row number. show offline peers explicitly. refresh every five seconds with
at most one inventory in flight; manual refresh is available. retain stale rows
only as visibly unavailable, with actions disabled.

- arrows/j/k select; enter attaches; space shows full metadata; r opens bounded
  output with source/scope/truncation; i interrupts; s stops; x kills; n creates;
  ctrl-r refreshes; q/escape returns or quits. key hints remain visible.
- stop/kill require one confirmation naming session, host, and effect. pin the
  reference when the action starts; refresh cannot change its target. cli commands
  are explicit actions and require no additional confirmation.
- creation is one form: machine, its advertised profile, name, cwd. reuse server
  validation; display errors inline. select the returned session. shell rows disable
  agent-only read/send/keys/interrupt/stop; info/enter/kill still work.
- use [bubble tea](https://github.com/charmbracelet/bubbletea) v2 for input/rendering
  and one local model. no component framework or handwritten escape parser.
  attachment exclusively owns tty input/output while active; restore tui and refresh
  on return. use [x/term](https://pkg.go.dev/golang.org/x/term) for raw-mode handling.

## direct terminal and session actions

desktop `enter` uses the existing authenticated websocket terminal route, machine
and session identity headers, frame types, and bounds. no local shortcut, ssh,
terminal emulator, replay, or automatic reconnect. require stdin/stdout ttys.
send measured initial resize before input, then forward terminal bytes and size
changes. ctrl-] then d detaches; doubled ctrl-] sends a literal ctrl-]; all other
prefix pairs pass through. escape and ctrl-c reach the provider. restore local
termios and terminal presentation after normal exit, failure, or handled signal.
cancel and join the input reader before returning tty ownership; a blocked stdin
goroutine must not consume later tui input. use library cancellation/handoff.

expand shared go/android geometry limits to 20–1024 columns and 5–512 rows, retaining
the 64 kib frame and 1 mib queue limits. desktop rejects unsupported initial
dimensions; a later unsupported resize detaches with an explanation and restores
the tty while work continues. no silent clamp or virtual viewport. update go/kotlin/js
bounds together; preserve the phone's existing undersized-viewport input gating
and continued output/automatic terminal replies.

host attachment uses one tmux queue: validate exact server/session lifetime and
supported options, then `attach-session -E -t <id>` in the owned pty. no new session,
group, active-pane isolation, session-option mutation, `-d`, or `-x`. require effective
`window-size latest`, `destroy-unattached off`, and `detach-on-destroy on`; install
these settings and fail before attaching when unsupported, with an actionable
websocket `Error` code `TerminalConfigurationUnsupported`, before `Hello`, with
message `tmux requires window-size latest, destroy-unattached off, and detach-on-destroy on.`
no new http error or duplicate preflight. checks are initial-only. retain bounded cleanup
of the owned pty/client only. detach, transport failure, and gateway exit preserve
source sessions, agents, and other clients.

presence counts `session_attached`. reuse its existing monitor to check the owned
client's pid/tty and current session against the attachment identity, ignoring
rename. a session switch ends the connection on detection by the existing two-second
monitor; bytes can pass before detection. this is not an atomic session lock.
no additional monitor or mutator-defense machinery.

kill uses the existing exact identity/name predicate and `kill-session` in one
queue; remove the group-size predicate. preserve validate → close owned terminal
connections → revalidate/delete ordering. deleting one ordinary grouped session
may leave shared windows/processes alive through another. no group-wide destruction.
stop retains separate agent/terminal outcomes; session closure never proves halt.
halting a shared-pane agent affects that work in every linked session; kill alone
does not request provider halt. explain this distinction in action descriptions.

provider policy is unchanged: codex uses terminal observations/controls; claude uses
verified native status/history when available and the existing terminal observation
policy otherwise. send/keys use terminal input. interrupt sends codex escape or
claude ctrl-c. native failure/unknown is never fabricated as idle/done. reads retain
honest native-history, terminal-history, or visible-only scope. this cut removes
legacy interfaces, not the accepted provider selection policy.

## composition and deletion

```text
cli / tui -> fleetclient -> selected gateway -> sessions + agentcontrol
             terminalclient -> same gateway websocket -> direct tmux client
jarvis agent.* -> installed skid --json -> same fleetclient
phone -> existing gateway api/websocket -> same direct attachment
```

expose existing fleetclient inventory types; own selection, reference encoding,
and the single client projection there. reuse its auth, bounds, validation, deadline,
and error machinery; extend explicit request handling for bodyless delete success.
cli parses/renders; tui presents; neither reimplements routing or provider logic.
reuse terminal protocol codecs in both directions. no new host endpoints.

jarvis exposes list/info/start/read/send/keys/interrupt/stop/kill. replace structured
targets with the returned opaque reference; spawn exact argv, never a shell, and
pass send text through `--stdin`. consume the common
json projection directly; delete `_session`, `_inventory`, and json-request stdin.
tool inputs: list `{machine?}`; start `{machine,name,profile,cwd?}`; info/interrupt/
stop/kill `{ref}`; read `{ref,mode?,maxBytes?}`; send `{ref,text,mode?}`; keys `{ref,keys}`.
retain existing mode/key/bound contracts; cwd defaults to `~`.
parse envelopes before interpreting exit status: exit 1 with `ok: true` retains
partial inventory or unconfirmed outcomes, rather than becoming malformed output.
before gating an addressed write, its existing dispatcher calls cli `info --ref`
once for metadata. pass the observed name/machine and original ref through existing
effect-target fields; metadata failure returns not-sent through existing handling.
absent owner input retains the existing denial before any metadata read.
persist/execute the ORIGINAL reference, never info's refreshed agent reference.
share the existing controller between tool composition and the dispatcher; no
second adapter or preparation subsystem. start needs no metadata read.
bump the agent tool implementation revision.
preserve existing write gating, uncertainty, budgets,
kernel, cognition, and immutable action-history rendering. no new authority model.

dev-server appends usage to its existing `assets/agent-instructions.md` and installs
it through `ai_install_instructions` for configured codex and claude roots. explain
help/list/info, name versus ref, ordinary verbs, startup dialogs, and uncertain
results. no new skill installer or second guide. new agents discover it normally;
busy agents learn it when asked to reread their instructions.

delete shadow creation/names/markers, hiding, reconciliation, promotion, last-link
settlement, readiness arming, reserved-name guards, `activeShadows`, `ReleaseShadow`,
`session_group_attached`, `SessionGroupedConflict`, and the old client parser/tests.
replace superseded proofs with direct-attachment behavior proofs. retain independent
tmux locking, exact identity checks, and bounded child cleanup. no migration reader,
legacy mode, or duplicated public command surface.

## implementation slices

each row owns its production files and matching tests exclusively. root assigns
the shared files first. builders observe a meaningful red before implementing;
readonly reviewers challenge each boundary before integration.

| owner | files and responsibility |
| --- | --- |
| host | `internal/tmux/**`, `internal/sessions/**`, `internal/agentcontrol/actions.go`, `internal/gateway/**`, `internal/logging/**`, `internal/terminal/cleanup{,_test}.go`; direct attach, group removal, closure outcomes; `tests/integration/**`, `tests/live/**` |
| client | `cmd/skidbladnir/main{,_test}.go` client dispatch; `internal/fleetclient/**`, `internal/agentcli/**`; new `internal/sessionui/**`, `internal/terminalclient/**`; normal commands, projection, tui, websocket client |
| phone | `android/**`; remove dead group errors/messages; new geometry bounds; retain current phone ui/confirmation/enrollment |
| integration | jarvis `src/jarvis/{agent_control,agent_tools,write_policy,write_gate,write_dispatch,definitions,cli}.py` (worker tools/wiring only), `scripts/qualify_{e2e,proactivity}.py` wiring, corresponding tests, spec/adr and architecture/operations docs; dev-server `assets/agent-instructions.md`, `lib/skidbladnir.sh`, `tests/skidbladnir.sh`, `assets/dotfiles/tmux.conf`, `tests/dotfiles.sh`, release pin and operator docs |
| root | this spec; architecture/roadmap and affected feature docs; shared `internal/terminal/protocol{,_test}.go`, `go.mod/go.sum`, `scripts/test` composition, release/pin coordination; no catalog change |

order: root freezes schemas/bounds → host and client build independently, phone
and integration adapt contracts → cross-boundary review → full acceptance → coordinated
release. never share ownership of a test file. inline one-use code unless it hides
substantial incidental complexity; no generalized command/lifecycle framework.

## acceptance and cutover

| id | required proof |
| --- | --- |
| a1 | default-installed `skid` opens the fleet; list/info expose host, name, directory, provider/profile, state/source; shell sessions are usable; unavailable peers remain visible |
| a2 | duplicate/incomplete names produce zero writes; qualified names and exact refs work despite unrelated peer outage; rename preserves refs; replacement session/process and changed pane reject stale controls |
| a3 | cli/tui use the same projection/dispatch; refresh and confirmation cannot retarget; creation, bounded read, interrupt, stop, kill, attach/return work without json assembly or mandatory config flags |
| a4 | multiline stdin and keys survive the real subprocess/http boundary; JSON is one valid envelope; partial/unknown results have nonzero exit; lost replies cause no automatic retry |
| a5 | two clients attach to one isolated session without creating sessions/options; window and pane navigation are shared; measured first resize and >240-column desktop geometry work; detach/error/shutdown restore tty, release input ownership, and preserve source/other clients |
| a6 | unsupported tmux settings/dimensions fail explicitly; rename preserves attachment; source destruction detaches, session switching closes within existing observation/cleanup bounds; server restart/id reuse cannot attach/delete a replacement |
| a7 | deleting one isolated ordinary grouped session leaves its sibling/shared process alive; stop reports provider uncertainty independently from closure; all grouped-conflict/shadow runtime paths are gone |
| a8 | ordinary codex and claude-work coordinators discover the guide and operate an exact peer; jarvis exercises nine tools through the installed cli, including startup-dialog keys and observed outcomes; an owner requests a session by name, without copying a reference, and the existing write gate permits the authorized action |
| a9 | macbook/arch/devbox control peers directly; phone still enters all three hosts and retains pairing, readable size, draft, scroll/copy and reconnect behavior; deployment preserves unrelated session/process identities |

use existing routine/integration/live/platform gates, adding proofs to their owners;
no separate proof framework. runtime acceptance needs the real boundary; absent
device/live access is `NOT_RUN`, never a pass. content/credentials stay out of logs
and evidence. obtain required live/device authorization before running those gates;
isolated tests own every tmux fixture they mutate. retain unrelated v1 acceptance.
jarvis's gate proof checks metadata failure before mutation, observed name/machine
with original ref, no replacement by a refreshed ref, and no agent lookup for
unrelated writes. a denied action may read metadata but dispatches zero mutations.

hard cutover: drain old jarvis agent actions; detach existing phone connections
normally. while the old gateway remains installed, let its existing reconciliation
settle owned shadows; require zero remaining owned shadow records before switching that
host. preserve last-link survivors as ordinary sessions using the old behavior.
promoted survivors may retain their old names; the new runtime treats them ordinarily.
if anything is attached or ambiguous, postpone that host; never kill user work.
stage matching host/client/phone/jarvis artifacts before switching callers. ship no
old grammar or shadow migrator. deploy gateway-only and instruction-only paths;
do not reboot machines, restart busy provider servers, or run a full workstation
apply. update authoritative docs and release pins together.
stage the managed tmux template through its existing owner; inspect effective options
and apply only needed settings during authorized rollout. do not blindly reload
the whole template: its plugin startup can affect busy sessions.
remove any owned persistent wrapper for the old cli; an existing shell function
named `skid` shadows the new executable until unset (`unfunction skid` in zsh).

explicit costs: shared navigation and latest-client sizing; no independent phone
view; an offline peer requires qualification for name lookup; a reserved detach
chord; supported tmux settings; eventual session-switch detection; a small tui dependency;
one metadata read before jarvis's existing write gate; bounded rather than complete
terminal history; shell-argument sends enter ordinary shell history/argv (automation
uses stdin); best-effort halt remains distinct from closure; coordinated hard cutover.

## delivery status

implementation is merged across skid, jarvis, and dev-server. immutable v0.4.1
at `bd4983f9ea23589283dac67c89d3b83cbe7cde8a` is published, pinned in both
repositories, and installed on all three hosts and the phone. jarvis runs
`f4e2ce6` through its matching installed cli. the original worktrees are preserved.

| boundary | current evidence |
| --- | --- |
| cli / tui / terminal bridge | local command, tls/http, authenticated websocket and real pty tests pass; includes multiline input, pinned confirmation, tea attach/detach/return, restored termios and joined input readers |
| host / phone | static/unit, android compile/lint and 112 unit tests pass; isolated integration and direct-client live pass on darwin, arch linux/tmux 3.7c and devbox linux/tmux 3.4; v0.4.1 physical platform gate passes 76/76, no skips |
| jarvis | exact-source canonical linux verification passes: 874 jarvis, 267 kernel, 1003 provider and 203 tools tests, static/package checks, one expected provider no-extras skip; hosted ci is `NOT_RUN` because of billing; genuine v3 owner events exercise all nine tools, both providers' actual replies, startup keys and exact closure; final stop retains unconfirmed halt separately from closed terminal |
| installer / guide | full local suite and hosted macos/linux checks pass; installer suite passes 20 groups including activation and command-link failure rollback; guide installed in existing provider roots; ordinary codex and claude read the installed guide through observed tool results |
| rollout | public-download verification and all three gateway/instruction-only upgrades pass; no owned shadows remain; all six directed routes and macbook↔arch control during devbox outage pass; immediate before/after session/process lifetimes and private files preserved |
| actual phone | all three hosts entered/detached at readable size; devbox additionally proves real gboard input/reply, draft retention, background/reconnect, touch scroll and native copy on v0.4.0, whose android tree is identical to v0.4.1; pairing survives both updates and platform gates |
| final phone actions | v0.4.1 stop confirmation/cancel passes; confirmed stop reports unconfirmed agent halt and closed terminal; terminal-only kill closes the other two exact fixtures; all three fixtures removed, remaining references preserved, public apk hash exact, test package absent and all three hosts authenticate |
| installed providers | all twelve profile launch/reply/closure cases pass across v0.4.0/v0.4.1; devbox claude's distinct successful coordination task supplies its reply proof; the earlier quota-failed turn remains failed |
| ordinary coordinators | both passed guide discovery, exact remote peer creation, trust keys, send/read, independently observed reply, kill and post-closure verification; all coordinator fixtures and owned empty directories removed; codex preserved all 18 baseline refs; claude preserved all 10 protected refs with exact concurrent test closures and one owner-confirmed unrelated closure recorded separately |

acceptance a1–a9 is complete. genuine owner events exercised jarvis's nine tools
through the installed cli without copying refs into owner requests. both jarvis
fixtures are closed; the final cleanup preserves all twelve other baseline refs.
historical denied/uncertain actions remain recorded: one conditional startup
request was denied before dispatch, and a new explicit owner grant then passed.
the owner probes used no gate/budget override, manual provider-session reset, or
context edit.
one completed owner turn lacks usage data; its token cost remains unverified.
rollout did not reboot machines, reload the tmux template,
or restart busy provider services. installed tui creation/read/interrupt/stop/kill
and terminal return pass; unconfirmed provider halt remains distinct from closure.
installed tui/outage and native tmux qualification are v0.4.0-era evidence over
host/client paths unchanged by v0.4.1. hosted v0.4.1 routine verification does not
substitute for those real boundaries.

v0.4.1 adds one observed claude idle-footer form, `← for agents`, to the existing
anchored detector. its unchanged-source red, focused green and independent review
pass. the original rejected arch fixture then accepted automatic send and produced
a native-history reply. unfamiliar terminal presentation still requires explicit
terminal intent; no broader provider policy or readiness inference was added.
terminal input delivery does not prove submission: the retained devbox claude
prompt remained a pasted draft until one separately observed enter. no second
paste was sent; native history then established one user turn and the limit notice.
ordinary coordinators retain their provider's existing trust and sandbox approvals.
the codex live check uses individual approvals for its exact fleet commands;
eleven approvals were required. this is not unattended-operation proof. no profile
policy or persistent command permission was changed. the coordinator's clipped
footer remained unknown; its initial and cleanup goals used deliberate terminal
mode after observing its composer. automatic-send readiness is not claimed there.
claude's distinct coordination and cleanup goals each used one terminal send and
one additional enter after the exact retained draft was observed; native history
confirmed each goal was submitted once. its peer received one explicit terminal
send after two automatic attempts returned `not_sent`. no uncertain input was
replayed and no persistent provider policy changed. its final stop returned
`agent: unconfirmed`, `terminal: closed`, exit 1; provider halt remains unclaimed.
