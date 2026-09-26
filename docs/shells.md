# new terminal here

2026-09-25 restoration amendment: [the deployment handoff](dev-server-handoff.md)
selects existing personal accounts and adds `SKIDBLADNIR_SHELL=1` to new terminal
startup, with final bash/zsh startup integration for bare/account provider
commands. inherited herdr context is removed at the pane boundary; the existing
tmux server and unrelated sessions are unchanged. this supersedes
provider-environment inheritance below only for new skid terminals. ordinary
shells and herdr panes keep their existing provider commands, homes, histories
and native integrations. skid apply owns the guarded shell source installation;
shared provider maintenance has no skid startup-file prerequisite. inherited
`SKIDBLADNIR_AGENT` is cleared; only the final provider exec sets it. personal
claude uses its native unset `CLAUDE_CONFIG_DIR` default. shell readiness remains
unpromised.

implemented: standalone terminal and new terminal here, following [spaces](spaces.md).
[the roadmap](roadmap.md) indexes delivery. linux/darwin host/desktop and corrected
real-phone journeys passed on their recorded sources; [hands-on acceptance](issues/spaces-shells-hands-on.md)
remains `NOT_RUN`.

2026-09-17: [pr 3](desktop-browser.md) changes only desktop presentation and the
shortcut to `T` (shift+t); detach retains the newly created shell selection.
historical pr 2 evidence uses its original `t` binding and proves no pr 3 layout.
[architecture.md](architecture.md) owns scope; [roadmap.md](roadmap.md) owns
delivery; this document owns the contracts below. read [AGENTS.md](../AGENTS.md),
[agent-control.md](agent-control.md), [agent-control-ux.md](agent-control-ux.md),
[design-language.md](design-language.md), and [rules/index.md](rules/index.md).
the requested testing standards live at [rules/testing.md](rules/testing.md).

## 1. outcome and limits

one action opens an independent terminal on the source's host, in its current
directory and space. it creates a
separate ordinary tmux session, never a window, split, or linked session group.

| surface | target behavior |
| --- | --- |
| tui `n` | existing five-field form: machine, launch, required name, cwd, space. launch offers terminal alongside advertised agent profiles. create selects/reveals the result; enter attaches. |
| tui `T` (shift+t) | on a fresh selected session, create here and immediately attach the returned session. works on agent and terminal rows. |
| android forge | terminal in the launch picker; existing name/cwd/space/objective fields and automatic post-create attachment. |
| android attach header | one-tap terminal-plus action, spoken “new terminal here”, minimum 48dp target; same create-here behavior for any attached session. fit the existing header height and visual language. |

the shortcut has no form: allocate the smallest free `skidbladnir-terminal-N`
name and an ordinary dwarf identity. each deliberate invocation creates another
session. name and space remain editable afterward. closing either session
preserves the other; detach preserves both. source objective and profile
environment are not copied. a space is copied once, including unassigned.

pr 3 provides browser navigation, with no special source return: detach keeps
the created shell selected; reaching the source is ordinary navigation. no companion registry,
parent link, automatic reuse/cleanup, saved launch request, readiness protocol,
provider work, arbitrary command endpoint, shell profiles, or new runtime owner.

## 2. capability and wire contract

keep existing authentication, machine binding, exact opaque references, bounds,
directory/name/objective/space validation, inventory, and attachment transport.
launch is a closed choice; it is not a new provider or a profile named terminal.

| operation | exact request | success |
| --- | --- | --- |
| `POST /v1/sessions` | `{kind:"agent", profile, cwd, optionalTmuxName?, objective?, space?}` or `{kind:"terminal", cwd, optionalTmuxName?, objective?, space?}` | existing `201 {observedAt,session}` |
| `POST /v1/sessions/{tmuxId}/shell` | `{identityToken}` | same `201 {observedAt,session}` for the newly created session |

`kind` is required. terminal forbids `profile`; shell-here forbids overrides,
names, pane/process targets, and client-supplied cwd/space. reject unknown,
duplicate, missing required, null, and wrong-type fields through existing strict
decoders. optional fields retain their existing omission/empty-value rules.
the response is the existing strict session dto, with optional observed agent
facts; never invent `launchProfile:"terminal"` or a permanent session kind.
manual agent recognition retains its existing configured-profile requirements;
zero profiles does not introduce a generic agent detector.

cli:

```text
skid start name --machine host (--profile key | --terminal) [--cwd path] [--space label] [--json]
skid shell name [--machine host] [--json]
skid shell --ref reference [--json]
```

start still requires name and machine; omitted cwd means remote home. shell uses
the existing mutually exclusive name/machine or exact-ref selectors and returns
the created session without attaching. human output names its machine/name/ref;
json uses the existing observed-session envelope. `enter` owns cli attachment.
`--terminal` on start selects launch kind; existing read/send meanings remain.

both creation routes use `{code,message,dispatch}` errors. dispatch is
`not_sent` only when the host proves no create executed, otherwise `unknown`.
reuse existing validation, authentication, directory, name-conflict, not-found,
and identity-mismatch codes; missing/unreadable source cwd is
`WorkingDirectoryUnavailable`. internal, partial-queue, projection, or lost
response failures after possible creation remain unknown. a definite pre-create
rejection changes no sessions. no automatic retry, retarget, or compensating kill.
inventory reveals current sessions but cannot attribute an uncertain creation.

## 3. host composition and launch

```text
cli/tui -> fleetclient -----> authenticated gateway -> sessions -> tmux
android -> gateway client -/                         creation     session lifetime
                                                                  -> terminal-exec -> shell
clients attach the returned exact ref through the existing terminal transport.
```

reuse one private creation core beneath the existing manager mutation lock;
ordinary creation and source creation must not duplicate queues or recursively
lock. keep existing epoch, name, dwarf, optional metadata, response observation,
and partial-create behavior. terminal creates no `@skid_profile`; the agent
branch retains its exact command, arguments, environment, and identity behavior.

source creation, in order:

1. validate the source session lifetime, independent of its name or agent.
2. pin its current pane once; read its required cwd directly and local space
   through the existing pr 1 metadata reader. do not use best-effort inventory
   enrichment or a cached client row. absent/invalid metadata means unassigned;
   failed tmux observation fails creation. validate cwd through workdir.
3. copy those observations into creation input. they are samples, not a lock
   against later cwd, pane, or membership changes.
4. guard the create queue with the existing tmux lifetime predicate. a vanished
   source or restarted server before dispatch creates nothing. do not add name,
   agent, or pane-still-active conditions. after accepted dispatch, source and
   new session have independent lifetimes.

terminal startup uses one private `terminal-exec` entrypoint in the existing
binary, dispatched before normal gateway/cli initialization. resolve that binary
with `os.Executable`; preserve its literal path through tmux's argv grammar.
invoke it as direct tmux argv. carry absolute cwd and configured tmux executable
as raw base64url arguments, plus the existing socket selector; decode/validate
once with existing path/socket primitives. terminal creation omits tmux `-c` and sets only the tmux
creation subprocess's `exec.Cmd.Dir` to the validated cwd. this preserves tmux's
stored session cwd for later windows without format parsing; never change the
gateway process cwd. the helper re-enters the required directory before executing
user code. no shell interpolation.

the helper uses the exact configured tmux executable/socket to read global
`default-shell` after its new session exists. this handles first creation with
no existing server. require an absolute executable shell; failure exits with a
fixed content-free terminal-local error. never substitute another shell. change
directory, set `PWD` and `SHELL` accordingly, then `exec` that shell with login
argv[0] (`-` plus its basename), inherited terminal fds, and ordinary tmux
environment. no `-c` payload, shell-specific flag guessing, daemon, or handshake.

this small helper is necessary: [tmux](https://raw.githubusercontent.com/tmux/tmux/3.4/spawn.c)
expands cwd formats, falls back after failed directory entry, and substitutes an
invalid default shell; its [argv parser](https://raw.githubusercontent.com/tmux/tmux/3.4/cmd-parse.y)
also recognizes trailing semicolons. encoded launch data avoids those textual
boundaries. [tmux client cwd](https://raw.githubusercontent.com/tmux/tmux/3.4/server-client.c)
supplies the session default. actual directory entry must fail closed, including
deletion after host validation. shell startup files retain their normal authority.

`201` acknowledges an observed tmux session, not shell execution or readiness.
the helper can subsequently fail; normal exit/retention follows tmux settings.
do not relabel that as a pre-dispatch rejection or add launch-status metadata.

host config accepts `profiles:[]` or the existing complete ordered four-profile
table. retain every nonempty profile constraint and the existing config schema;
do not add arbitrary subsets or a second profile registry. successful host/fleet
inventories include `profiles:[]` when empty, never omit it or encode null.
no agent installation is needed for the empty
case; existing absolute native-control-path admission remains unchanged. creation
logging permits an absent launch profile and stays content-free.

## 4. client ownership and completion

reuse existing launch forms, result projection, mutation lane/read fence,
confirmed-create filter reconciliation, and attachment owners. terminal remains
available on an actionable host with zero profiles. machine changes retain the
terminal choice and existing name/objective/space drafts; reset host-specific
cwd and agent-profile choice. append terminal after the existing profile order.
tui keeps its first-profile default, choosing terminal only with zero profiles;
android keeps explicit launch selection. no persisted preferences or restoration
schema.

pin the source ref and initiating interaction. suppress duplicate submission
while pending. android keeps its current attachment until creation is confirmed.
only a still-current form/tui operation/terminal attempt may select, change
filters, or attach on completion; retain existing credential/runtime fences too.
a late result can trigger inventory refresh but cannot hijack navigation. add
the missing originating-form guard to existing forge completion instead of
creating a second completion path.

confirmed results reuse pr 1's reveal/select rule, then the invoking surface's
attachment behavior. failure/unknown retains filters and the current attachment,
subject to existing access-loss and session-liveness handling.
after confirmed creation, attachment failure retains the exact new target;
retry attachment only. pending creation and source refs stay in memory. activity
recreation restores only the existing content-free dashboard state, never a
creation request or terminal attachment. back/detach keeps the existing meaning.

## 5. work split, reuse, and hard cut

| owner | exclusive files and responsibility |
| --- | --- |
| host | `cmd/skidbladnir/` (including private terminal entrypoint), `internal/sessions/`, `internal/tmux/`, `internal/workdir/`, `internal/hostconfig/`, `internal/agentruntime/profile.go`, `internal/gateway/`, `internal/logging/`; matching tests; new `tests/integration/shells_test.go`. launch, source gate, schemas, empty profiles, errors. |
| desktop | `internal/fleetclient/`, `internal/agentcli/`, `internal/sessionui/`; matching tests; new `tests/integration/shell_clients_test.go`. one projection for both creation results; launch choice, shortcut, exact attach/return. |
| phone | `android/app/src/main/java/dev/niels/skidbladnir/{ProductModel,GatewayClient,SkidbladnirController,ForgeSheet,TerminalScreen,MainActivity}.kt`; corresponding JVM/instrumented tests. typed launch, header, existing mutation/admission composition. |
| root | docs, shared `tests/integration` fixtures, `scripts/test` composition, integration review. no catalog change. |

freeze these contracts before parallel implementation. host publishes request/
response types first; clients then proceed independently. owner changes only its
paths; shared-fixture edits go through root. reviewers write no production/tests.

delete superseded profile-only creation decoding/serialization, empty-profile
disabling, and affected copy/tests. reuse required/local metadata reads, lifetime
predicates, typed workdirs/spaces, allocation, strict json, and create-result
projection. extract only the shared locked create core and genuinely repeated
projection/completion logic; inline single-use work. no general launch framework.
hard-cut host/cli/android together; old create bodies and mixed versions fail.
no compatibility routes/readers, negotiation, alternate commands, or migration.
leave unrelated old code and pr 3 work outside this change.

## 6. acceptance and red/green/refactor

80/20 shape: one owner-level behavioral proof for each row below, with focused
pure validation/state tables where needed. cases extend that proof; do not repeat
the host matrix at every client layer. use real owned services and runtime ui;
fake gateways, intercepted internal api replies, callback counts, source scans,
and compilation failures do not prove create/attach behavior.

| proof / owner | acceptance |
| --- | --- |
| h / host: authenticated gateway + isolated tmux, linux and darwin | standalone creation with zero profiles and no server; source creation copies current cwd/local space to a new exact session; rename/agent replacement accepts the retained session ref; stale lifetime/auth/machine rejects without creation; later edits/closure stay independent. cover strict launch/error schemas, unassigned space, custom default-command bypass, invalid shell, directory loss, literal spaces/quotes/backslashes/format syntax/trailing semicolon, and unknown partial creation. assert responses and subsequent inventory/process outcomes. |
| d / desktop: real tui/pty -> fleetclient -> gateway -> isolated tmux | `n` terminal works without profiles and selects the result; `t` creates once and attaches; detach reveals its exact row with the correct filters; source survives. a real connection cut after creation proves visible uncertainty and no duplicate session. pure cli/schema/state cases cover exclusive launch flags, retained refs, and late completion. |
| p / phone: real compose/controller -> gateway -> isolated tmux | standalone forge and header action create/attach; duplicate tap creates once; source survives; back restores the collection. real platform cases cover failure/late completion, credential change, recreation without replay, and the header at narrow width/enlarged text without overlap or lost controls. |

workflow: review contracts/adversarial failure cases; establish the authorized
baseline; builder writes and observes its behavioral red against unchanged
production; implement the smallest green; independently review identity,
dispatch, literal paths, and lifecycle; refactor only while behavior stays green.
reviewers challenge the contract, red's sensitivity, green, and refactor at each
handoff; review fixes get a failing case at their owning boundary before repair.
finish routine `./scripts/test verify` and the approved owner gates. refactor-only
or mocked evidence cannot replace an unavailable owner proof. no provider matrix,
new test platform, benchmark, or full fleet cross-product.

tmux and integration/live require explicit current-turn approval; platform/adb
requires its own current-turn approval. only exact test-owned sessions on isolated
sockets; no live user content in evidence. unapproved or unavailable boundaries
stay `NOT_RUN`, including owner reds; this spec authorizes no runtime action.
completion means h/d/p and routine verification pass on the candidate, with no
retired path or unrelated runtime change. deployment/publication remain separate.

proof locations: `tests/integration/shells_test.go` owns h;
`shell_clients_test.go` owns d; android `ShellsInstrumentedTest.kt` owns p.
existing shared fixtures own the built production gateway and TLS transport.
the `integration && androidplatform` fixture composes p into the existing
release-bound platform gate; it runs the complete instrumentation suite and
retains its exact test-count/no-skips check. it requires both
`--allow-device-mutation --allow-isolated-tmux-mutation`, the existing isolated
opt-in, and a clean non-tmux invoking environment. routine verification only
compiles/vets this boundary; it never invokes adb.

the phone fixture uses an empty isolated gateway with `profiles:[]`, a test-only
fixed fleet, loopback HTTPS on device port 8443, and the fixture's public TLS
certificate with normal hostname verification. the private JSON contains only
`credentials`, `machineHandle`, `cwd`, and `tlsCertificatePem`; instrumentation
receives its basename in `shellsFixture`, resolves it under app files, and never
prints it. reject an occupied device reverse port. the existing platform trap
owns the file and exact reverse mapping, removing both before release restoration;
the host fixture owns gateway/socket cleanup. no production pairing is changed.

## 7. accepted costs

- one-tap creation uses a generated name; customization uses the existing form.
- directory/space are host samples; later changes do not synchronize sessions.
- one small exec helper enforces startup correctness; success promises no ready
  prompt, and startup files may change the shell's initial state.
- unknown creation may need manual inventory inspection; no durable receipts.
- a collision detected only after tmux starts can remain unknown; do not infer
  dispatch from tmux's human stderr or later inventory.
- compact detach/menu/text-size glyphs make room in the header; the title may
  truncate. retain its existing height behavior at each font scale; no extra row.
- returning to the agent requires browser navigation, including in pr 3.
- strict wire cutover requires coordinated host/cli/android updates and rollback.
