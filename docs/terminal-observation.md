# Terminal observation

This spec owns what terminal status means: the status facts, the observed screen
regions and their limits, provider classification, guarded-send admission and the
terminal wait states. The [Codex](terminal-observation-codex.md) and
[Claude](terminal-observation-claude.md) grammars own each provider's frozen
capability table, rules, rule IDs and costs. The
[architecture](architecture.md#7-observation-and-agent-control) owns recognition,
account attribution, the focused-observation mechanics, terminal effects and
wait's polling loop.

## 1. Requirements and limits

The product answers two questions independently: **Is work visible? Does this
terminal offer a response that needs attention?** A private third question asks
whether guarded send can safely use the ordinary composer. None is task success.

- Recognize supported Codex and Claude working, starting, idle, permission,
  question, review, setup and menu layouts, including concurrent work and
  questions.
- Distinguish successful ambiguity from observation failure; explain both without
  exposing terminal content. No quiet-time or process-presence idle.
- Apply one observation contract to every launch, skid-created or typed by hand;
  no conversation ID, native helper, provider hook or transcript is required.
- Preserve exact session, pane and foreground guards, drafts, the five-second
  polling, the two-second enrichment deadline, cancellation and machine
  freshness.
- Ordinary requests remain visible after a visit until fresh observation changes
  them.

Non-goals: authoritative completion, cancellation or crash detection; unseen or
between-poll requests; task or subagent history; request payloads or counts;
approval buttons; remote execution inference; provider forks or updaters; new
daemons; persisted status. A shell's return proves the provider absent, not why it
exited. Obscured execution may remain unknown, and no label may claim more.

## 2. Capability and wire contract

`session.terminalStatus` fields are required, non-null and closed:

```text
TerminalStatus = {
  activity: starting | working | idle | unknown,
  interaction: none | permission | question | confirmation | setup | input | menu | unknown,
  notice: none | interrupted | error,
  source: terminal | unavailable,
  reason: recognized | partial | layout_unknown | evidence_clipped | evidence_conflict |
          provider_unrecognized | remote_context | foreground_changed |
          observation_timeout | capture_failed | process_failed
}
Detection = {status: TerminalStatus, composer: empty | draft | blocked | unknown}
StatusPresentation = {label, tone: working | attention | muted,
  queue: none | ready | action, detail?, evidence?, queueNotice?}
```

Each inventory and creation row also carries `status: StatusPresentation`, the
host's rendering of `terminalStatus` as if fresh and not ready (§6). Queue `ready`
means eligible: a client keeps it only while the row is fresh and the hub holds a
pending ready. Optional members are omitted, never null or empty.

| field/value | exact meaning |
| --- | --- |
| `working` | Current provider activity: generation, tools, retry or active background work; a surviving process or task count alone is insufficient |
| `starting` | Recognized current startup activity; never a startup grace period |
| `idle` | Qualified provider ready-state evidence; no completion or success claim |
| `none` interaction | The complete recognized interaction region shows no response request or menu; hidden requests remain unknowable |
| `input` | Positive human-request evidence with no recognized subtype |
| `unknown` interaction | Insufficient evidence to classify the current interaction surface |
| `confirmation` | A provider-requested review or decision, such as plan implementation; not a generic selected item |
| `menu` | User navigation, such as model, settings or resume selection; not needs-input attention |
| `notice` | How the latest turn ended abnormally: the provider's own interruption or error marker as the transcript's last item (or a qualified current banner); `none` otherwise |

`working + question` is valid. Each dimension is classified independently;
conflicting evidence makes only the affected dimension unknown. `notice=none`
means no recognized current notice. Only the marker that ends the transcript
counts: historical error or interruption text above a later prompt or turn cannot
set it. A notice persists until the next turn replaces it, and never erases
independent work or request evidence.

`unavailable` requires unknown activity and interaction and notice none, with a
failure reason; successful unknown uses `source=terminal`. A changed foreground
within an otherwise valid target gives unknown/unknown/none with
`foreground_changed`; changed terminal authority rejects explicit operations.
Failure reasons are exactly observation_timeout, capture_failed and
process_failed; all other reasons require source terminal. Timeout is chosen before
the failed stage. For a successful sample, foreground_changed, remote_context or
provider_unrecognized come first when they apply; otherwise conflict, then
clipping, then recognized, partial or layout_unknown. Conflict and clipping must
affect a required inference, not unrelated transcript. Recognized means both
activity and interaction were classified, partial one, layout_unknown neither. No
failed stage is ever converted to successful ambiguity.

`internal/api` declares these types and their invariants; agentcontrol upholds
them, the gateway serializes them, the Go
client checks only that each value is known (`TerminalStatus.Known`), and Kotlin
decodes the closed enums it reads. Inventory freshness is the only status clock:
no confidence percentage, status expiry, transition timestamp or inference
history.

## 3. Composition and observation boundary

```text
inventory / terminal inspect / guarded send
  -> sessions: exact target + focused foreground validation
  -> tmux: bounded current-screen regions
  -> agentcontrol: pure provider classifier + composer interpretation
  -> gateway -> fleetclient + fleetview / android: projection, attention, views
```

The observation is private, beside the public terminal read, whose plain-text
history contract it does not share:

```text
PaneObservation = {
  width, height, alternate,
  regions: [{kind: top | bottom, firstRow, rows: string[], clipped}],
  // rows keep physical positions and bounded sgr styling
}
```

- The bottom region is the last 64 physical visible rows, capped at 64 KiB
  including SGR. For taller panes the top region is every row above the bottom
  region up to row 191 (`0..min(h−65, 191)`), capped at 16 KiB and read top-down,
  so a byte cap keeps row 0. The 192-row cap comes from tmux's 16 KiB limit on one
  client command, which bounds a guarded per-row capture to 256 rows; the stdin
  capture batch keeps it so batch and per-pane observations agree. Rows
  `192..h−65` of a taller pane are never captured, and scrollback is never read.
  Rows are selected before a region's byte limit applies.
- Regions are contiguous unless a byte cap or that row bound dropped rows. A
  dropped row that a required inference reads is clipped evidence
  (`evidence_clipped`); a dropped row no required read reaches costs nothing.
- Rows are complete UTF-8 with their style: `capture-pane -e` without `-J`, one
  capture per row, each parsed from the default style. Truncation means lost
  requested evidence, not an omitted middle transcript. A clipped upper region
  cannot invalidate an intact lower composer; a clipped required boundary
  invalidates that inference. Transcript and controls are never flattened into one
  text tail, and no emulator runs.
- One row parser serves both grammars. It reads SGR as tmux's writer emits it, OSC
  8 hyperlinks and SO/SI shifts as zero width, and tmux 3.7c's tab cells as spaces
  to the next 8-column stop (3.4 writes the blank cells instead). `capture-pane`
  runs without `-N`, so tmux trims the space run after a row's last escape and no
  grammar requires a trailing space. Any other escape makes only that row
  unparseable: present but unrecognized in both grammars, never clipped evidence.
- A size or alternate-screen change during capture invalidates the sample; the
  target and foreground are revalidated before and after. It is a bounded sample,
  not an atomic snapshot of provider execution.
- Inventory passes its captured identity into observation; no discovery pass runs
  per sample, and explicit controls resolve fresh. No capture runs under the
  manager gate; there is no extra poller, retry loop or deadline extension.
- Only a freshly recognized local provider is classified. Shells and SSH/mosh
  panes keep their controls; their agent status is unknown, never inferred remote
  state.

The row and byte limits are named constants, not configuration. Raise them only
for a demonstrated live layout, keep the envelope bounded and remeasure the
deadline.

Titles, progress sequences and bells are not classified and no emission watcher
exists. The only reuse is the [exec budget](tmux-exec-budget.md#a3-activity-gated-reuse)'s
activity-gated detection reuse. tmux keeps titles across process replacement, and
matching a provider composer does not establish their freshness. The accepted cost
is unknown behind obscuring views.

## 4. Provider adapters and managed displays

One pure entry point lives in `agentcontrol/detect.go`, with cohesive Codex and
Claude parsers where that keeps their grammars readable; they share SGR and
physical-row parsing and result construction. There is no rule registry, score
engine or generic regex manifest. Each rule names a complete current region,
required controls, optional variants, conflicting evidence and a content-free rule
ID.

Current modal, question and composer boundaries are located before their contents
are interpreted. A numbered list, a spinner glyph, the word `working`, a quoted
footer or an empty composer alone is insufficient. Layout knowledge tolerates
supported wrapping, optional animation and remapped keys without treating
arbitrary text as controls.

| provider | required families and decisive distinctions |
| --- | --- |
| Codex | Activity with or without glyph, remapped interrupt, suffix and wrapped details; background-terminal waiting remains working; a run-state word when the user's status line shows one; idle from a settled transcript, never from the footer alone |
| Codex | Permission overlay including the open-thread footer; legacy single, multi and free-text questions; collapsed and expanded async questions; the plan implementation decision; trust and update setup; ordinary pickers and transcript views |
| Claude | Generation, tools and retry; background agent, task, workflow and MCP activity; default and custom multiline statusline and a missing interruption hint; fullscreen and classic layouts |
| Claude | Permissions; questions with navigation between confirm and cancel hints; free-text and multiple questions; plan approval; MCP elicitation; trust, login and setup; model, settings, viewer and side-question overlays |
| Both | Current interruption and error structures when distinguishable; empty, draft, blocked and unknown composer; active work with historical request or error text as negatives |

Positive activity is never overridden by a persistent composer or footer. Codex
idle requires a settled transcript (its last stop is a turn-end separator, a `■`
notice cell or the session header), no status row, the main placeholder on a clean
band, no external editor and no goal indicator; Codex hides its status row while it
streams a final answer, so a prompt as the last stop is never idle. Claude idle
requires a qualified complete current ready layout, including every relevant
activity and request region; a missing spinner alone is not enough. Behind menus
and viewers only independently visible positive facts remain; the previous
activity is never carried forward.

skid sets no status line or title on Codex launches, so every launch reads the same
screen. When a user's own status line shows a run-state word, `Working`,
`Thinking` and `Waiting` mean working, `Starting` starting, and `Ready` beside a
status row is a conflict. Claude keeps its display configuration: its progress
setting adds no signal under this screen-only contract, and skid neither replaces
its statusline nor forces accessibility mode. Unsupported display settings read
unknown.

## 5. Controls and diagnostic API

Guarded send requires a fresh local provider, activity working or idle,
interaction none and an empty ordinary composer. A notice does not refuse: it is a
transcript marker, not a surface over the composer. Question editors, menus,
drafts, clipped or unknown composers and unknown interactions refuse before
writing. The composer is `empty` when the ordinary composer shows only its
placeholder, `draft` when it holds input (Claude bash mode included), `blocked`
when it visibly refuses ordinary input (a request or menu replaces it, it is
disabled, or a side view or external editor holds it), and `unknown` otherwise.
`blocked` refuses as `unknown` does and stays as diagnostic vocabulary. A refusal
names its reason: a request interaction is `dialog`; otherwise a draft composer is
`draft`; otherwise `unknown`. A failed sample refuses with `TerminalUnavailable`.
Better status coverage never relaxes composer recognition on its own.

`wait --state` accepts `working`, `idle` and `needs-input`, default idle. Idle
matches idle with interaction none, whatever notice the turn ended with, so a wait
ends on an interrupted or failed turn too. Working matches working even with a
question. needs-input matches permission, question, confirmation, setup and input.
An unknown tested dimension never matches: working with unknown interaction
matches working, and unknown activity with a question matches needs-input.

`POST /v1/sessions/{id}/terminal/inspect`:

```text
request  = {identityToken, paneId, explain?: boolean} // absent or null = false
response = {terminalStatus, diagnostics?}           // present iff explain=true
diagnostics = {rules: [id]}
```

`skid info HANDLE --explain` resolves once, inspects that target, and prints its
facts and the explanation from the same sample; ordinary `info` shows the
inventory status without a second sample. An observation failure after target
admission returns unavailable status with same-sample diagnostics; target, auth
and identity failures remain errors.

Rule IDs are string literals of `[a-z0-9_.-]` written in the grammars, never built
from screen text: no fragments, titles, paths, prompts, account data or provider
IDs. Rules follow the grammar's decision order; a successful sample may match none,
printed `none`. A classifier defect is never disguised as unavailable.

## 6. Content, attention and filtering

One Go projection, `api.PresentStatus`, renders label, tone, queue, detail and
evidence as if fresh and not ready. The gateway sends it as each row's `status`;
the desktop and CLI call it on the rows they hold; Android reads `status`. Clients
add only staleness, the hub's ready and notification health. Labels, tone and
predicates derive from the same facts, and behaviour never compares rendered
strings.

- Precedence: stale or unavailable treatment, then a request, a menu, a current
  notice, starting or working, ready, and idle or unknown. When a request, menu or
  notice takes precedence over working, the detail adds `work continues`.
- Tones: `working` for starting and working, `attention` for requests and an error
  notice, `muted` for everything else. Only `ready`, applied by a client from the
  hub's verdict, is green. Colour never carries meaning alone.
- Rows of a host whose read failed, or that is being re-read, are never ready and
  never in the queue. Their presentation is each client's
  ([desktop](architecture.md#desktop-invariants), [phone](architecture.md#phone-invariants)).
- The evidence qualifier `inferred from terminal` appears only for a recognized
  local agent with source terminal; unavailable and non-agent labels make no
  inference claim.
- A current interruption or error notice can hide a saved ready; a visit never
  dismisses a current request; available again does not assert that a question
  was answered. There are no request IDs or durable request records. When ready is
  created and what acknowledges it belong to the hub
  ([notifications](architecture.md#12-notifications)).
- The human `needs input` view holds ready, requests and current error and
  interruption notices ([views](architecture.md#views-and-the-needs-input-queue)).
  `wait --state needs-input` keeps the narrower explicit-request predicate of §5.
- The diagnostic heading is `status evidence`: activity, interaction and notice
  separately, then reason and matched rules. Unknown and unavailable details may
  append `open the terminal to inspect`; no raw error is interpolated, and partial
  never reads as `partially working`.

## 7. Requalification

Each run updates the grammar's §1 `qualified` column, and every required row it
leaves unqualified is tracked in [unrun acceptance](issues/unrun-acceptance.md). A
provider or tmux upgrade is claimed only after the grammar's requalification
([Codex §7](terminal-observation-codex.md#7-requalification),
[Claude §6](terminal-observation-claude.md#6-requalification)). Runs that start
real providers on the owner's accounts need current-turn approval under
[`AGENTS.md`](../AGENTS.md); isolated tmux, loopback gateways and emulators need
none. Use only the sessions on the isolated socket the probes created.

A requalification covers, beside the grammar's provider screens:

- Capture: normal and alternate screen through real isolated tmux; more than
  8 KiB of styled upper output with the lower controls intact; a top setup modal;
  a tall pane; SGR and UTF-8 clipping; a dimension change.
- Recognition: managed, bare, absolute, relative and symlink launches on Darwin
  and Linux; unrelated executables; exit, replacement and foreground suspend and
  resume.
- Negatives: the composer during active Codex work; quoted old chrome; a menu
  against a request; the title-generation spinner and the static Claude title;
  busy, action-required and progress state inherited across provider → shell →
  same provider (except §8's accepted false claims); historical errors under
  current work.
- Controls: an empty ordinary composer accepts send; a draft, dialog, menu,
  unknown, clipped or changed target refuses without writing bytes; text, keys,
  stop and close are unchanged.
- Full product: a real gateway to the CLI, the desktop and the phone: work, each
  request label, concurrent work, stale and unavailable rows, needs-input
  membership and order, visits, return restoration, large text.
- Diagnostics: the reason and the same-sample explanation agree, and logs and
  envelopes carry no content.

- Oracles are independent: controlled provider responses and tool gates, or a human
  watching the real provider, never skid's own status. A stock TUI against a local
  deterministic endpoint proves TUI and protocol behaviour, not service
  reliability; authored frames through real tmux prove capture and parser
  mechanics, not provider support; ordinary live provider smoke is separate.
- Each grammar freezes, before its parser changes, a provider, version and
  configuration capability table with required positive families, permitted
  unknown or none cases and upstream-unavailable variants. Permissions and
  questions the provider renders require positive recognition; notices without a
  distinguishable current structure and obscured state permit unknown or none.
  Unavailable upstream features are neither claims nor passes; unexercised
  required rows are `NOT_RUN` blockers. Installed versions are qualified first;
  there is no universal-version promise and no runtime version gate. Ambiguity
  controls read as their rows state and are never waived by an accuracy score.
- Each new rule is paired with a negative case, and a mutation of the rule must flip
  it. Cost is checked at 1 and 16 represented sessions per host platform within the
  two-second enrichment budget, with no induced timeouts; 16 is a workload, not a
  limit.
- Evidence records revisions, provider and tmux versions, platform, dimensions,
  scenario IDs, expected and observed categories and timing; never terminal bytes,
  prompts or account data. Frames hold authored dummy content only; real session
  screens are never saved.

## 8. Accepted costs

- Conservative unknown loses some idle waits and delays ready until idle is
  recognized; five-second sampling misses brief transitions.
- Each grammar lists its provider's costs and the conditions under which each
  reads unknown, none or a false claim
  ([Codex §6](terminal-observation-codex.md#6-accepted-costs),
  [Claude §5](terminal-observation-claude.md#5-accepted-costs-and-residual-ambiguity));
  the paragraph below names every cost that claims falsely or widens send.
- Tall panes: rows `192..h−65` of a pane taller than 256 rows are clipped evidence,
  and idle there needs the transcript tail within the bottom 64 rows.
- Wrappers and deleted images are not recognized.
- Provider UI upgrades require requalification; broader capture costs more per
  sample; no retained suite guards future regressions beyond the frame replay.

The accepted false idles are Claude's remote usage-limit copy and three of Codex's:
its cue-less turn starts (c4); its millisecond stale-`Ready` window in a pane at
most header + 4 columns wide (c14), which can raise ready; and a user status line
whose first item equals a run-state word (c12). Codex's other accepted false claims
are a stale request under a shell that prints nothing until a relaunched Codex
first draws (about 0.4 s; c13) and a wrong picker kind under a transcript row that
starts with a listed title (c11), each of which can enter the needs-input view; and
a request read as another request subtype (c15–c17), which misstates only the
subtype (its label, reported interaction and rule ID): every client treats all
request subtypes alike for wait, the needs-input view, attention, ready and guarded
send. The accepted send-admission exposures are Codex's displayed-thread scope and
Claude's `AbovePrompt` band and Vim NORMAL mode. No other cost justifies a false
idle, weaker input admission or silent scope expansion.
