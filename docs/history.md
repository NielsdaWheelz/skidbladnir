# History

Skíðblaðnir was specified, built and run by one person for their own computers
and Android phone, from a first commit on 22 August 2026 to v0.23.0, its 72nd
release, on 26 October. Below are its phases, then eight decisions, each with
the constraint that forced it and the cost accepted. Dates are UTC. Until 25
September the project lived in a repository since renamed herdr-mobile:
"original #N" is a pull request there, a plain #N one here.

## Timeline

| dates | phase | marks |
| --- | --- | --- |
| 22–25 Aug | A rulebook, then a specification drafted over four days, first named Ariadne, renamed Skíðblaðnir on 24 Aug; no Go or Kotlin until P0 froze the contracts and proved the platform with probes; then a scope reset to a tmux remote | `b460281`, original #1, `6012a5e` |
| 26 Aug – 18 Sep | v0 for one host and the phone; a multi-host fleet with signed releases; agent controls across hosts; session spaces; the desktop browser; behavioral test suites retired | original #2, #9, #18, #79, #94, #98, #102; v0.2.0 (27 Aug) to v0.6.0 (17 Sep) |
| 22–25 Sep | herdr: feasibility study, cutover, a phone-only gateway, the split | original #133 to #137; v0.7.0 (24 Sep), v0.8.0 (25 Sep) |
| 26–30 Sep | The tmux product restored from `927c554` into this repository; herdr retired | `4eeba15`, original #138; v0.9.0 (26 Sep) to v0.10.6 |
| 29 Sep – 20 Oct | Native conversation controls, then status read from the screen; workspace recovery; notifications; account usage | #14, #40, #65, #66; v0.11.0 (1 Oct) to v0.18.0 (20 Oct) |
| 20–21 Oct | Standalone installer and a fleet with a hub; terminal-only agent control | `e7c659e`, #118; v0.19.0, v0.20.0 (21 Oct) |
| 21–23 Oct | Reduction campaign: 48% of tracked lines removed | #120 to #168; v0.21.0 (22 Oct), v0.22.0 (24 Oct) |
| 25–26 Oct | Publication: plan, license, host fixes, a new Android signing key | #175 to #182; v0.23.0 (26 Oct) |

## Decisions

### 1. Specification first

**Context.** The first commit held no code, only 44 files of rules, some written
for a TypeScript runtime, which bound Go and Kotlin only by intent. The
specification grew from 496 lines to 1,592, with a roadmap of phases P0 to P7.

**Choice.** No product code before the contract. P0 (original #1, 25 August)
froze the contracts and proved, against real tmux and the real phone, the
mechanisms they rested on: tmux grouped sessions, xterm.js with Gboard in an
Android WebView, provider resume. It shipped no runtime.

**Consequence.** The specification paid for itself by failing early. Five hours
after P0 merged, the 1,245-line design gave way to a 306-line one (`6012a5e`):
it had "built an audited Codex runtime system where the product is a specialized
Android remote for tmux." Its platform findings carried over, and v0 shipped the
next day (original #2). The habit stayed: one contract, the
[architecture](architecture.md), and no new capability without an explicit scope
decision. Its costs came later: the rulebook "taught same-system distrust and so
produced most of the defensive code" (#121), and the docs outgrew the Kotlin.

### 2. tmux as the store

**Context.** The pre-reset design kept its own state (SQLite as authority, hook
and control sockets, a registry of runtimes), so a restarted gateway
"reconstructs owned runtime facts from SQLite, tmux, `/proc`, and hook gap
files." The reset wanted a restart to mean "list tmux again".

**Choice.** "tmux is the database and the process supervisor" (`6012a5e`).
skid's metadata are session-local `@skid_*` options, and identity is tmux's own:
a server lifetime is a random epoch plus the server's pid and start time, and a
session adds tmux's session id. Sessions started by hand appear like any other.

**Consequence.** In the positioning's words, "nothing migrates and sessions
outlive skid." Every read spawns a process: on 21 October one Mac with 18 to 20
agent sessions spent 13.6% of a core in the gateway, 17.5% in its children and
5.0% in the tmux server. The [exec budget](tmux-exec-budget.md) bounds a steady
listing to two tmux processes; the remeasurement is open. Sessions also die with
their tmux server: workspace recovery (#65, 7 October) checkpoints a private
recipe every 30 seconds and rebuilds the panes after a proven loss; recovery
from a real reboot has not yet been observed.

### 3. At most once, never replay

**Context.** When a reply is lost, a client cannot tell whether its request ran.
The first draft answered with durable operations (states ending in `Applied`,
`Rejected` or `OutcomeUnknown`, idempotency keys kept 90 days, endpoints to
resolve the unknown) under one rule: "Ambiguity is visible recovery, never a
guessed result or automatic redispatch."

**Choice.** The reset kept the rule and dropped the machinery: "durable command
receipts and replay" became a non-goal. Every effect is one tmux command queue
in which `if-shell -F` checks the server lifetime and session id, then runs the
effect and an applied marker; tmux drains that queue before serving another
client ([`internal/tmux/guard.go`](../internal/tmux/guard.go), since v0). A
missing marker is `unknown`, reported and never retried.

**Consequence.** No layer of skid repeats an effect: a lost create is not
resubmitted, a reconnect never resends keystrokes. The person resolves the
uncertainty instead; a lost join reply, for one, leaves the invitation spent.
Receipts and replay return "only if notifications grow buttons that act
unattended ... or unattended orchestration arrives."

### 4. A terminal on the phone, not a chat view

**Context.** The first draft was an Android "command deck" over Codex's App
Server, in which "an agent is a durable conversation, not a repo, directory,
worktree, terminal, or process."

**Choice.** Two days later the answer to "TUI or structured chat?" was the stock
TUI: "It already owns conversation, tools, approvals, and rendering." The phone
runs xterm.js in a WebView and types through Gboard, with "no parallel
speech/composer lifecycle"; v0 made phone and laptop "two tmux clients. One
process, one screen, one draft." A chat UI is still a non-goal.

**Consequence.** A new provider is a configured command, not client work. The
cost is a phone keyboard driving a terminal: a row of terminal keys, a one-shot
Shift because "gboard's shift cannot modify the custom key bar" (#117), and a
patch to xterm.js for touch selection, taps and scrolling, kept from 6 September
until stock xterm.js replaced it on 22 October (#143). Raising Gboard can still
resize the shared tmux window in steps, redrawing the laptop's view too.

### 5. herdr adopted, then abandoned

**Context.** herdr is, in this project's own positioning, "the most capable
agent-aware multiplexer": it owns terminals, panes and layouts, and observes
agents. The plan was to "delegate terminal compatibility and agent interaction
mechanics to a maintained upstream", keeping skid's Android app, gateways and
CLI. skid's later research credited herdr with separating detection from
attention, an explicit unknown and startup readiness.

**Choice.** A feasibility study on 22 September returned "reconsider"
([original #133](https://github.com/NielsdaWheelz/herdr-mobile/pull/133)):
closing a workspace's last pane also removed a linked worktree terminal, and the
phone had no mode-aware tap route. The owner amended the scope to accept both,
and skid cut over
([original #134](https://github.com/NielsdaWheelz/herdr-mobile/pull/134)):
v0.7.0 ran on every host on 24 September, and v0.8.0 reduced skid to the Android
app and a phone-only gateway over herdr (original #136).

**Consequence.** The cutover had accepted in writing that Claude lost native
status, history and halt confirmation, that a linked close could cascade, and
that a cold restart invalidated terminal lifetimes; herdr had not recognized
Claude as an agent in either test fleet, and live terminals were not migrated.
On 25 September the owner requested a split, restoring the tmux product from
`927c554`, the last commit before the migration, into a new repository that took
the skidbladnir name (original #137); by 30 September herdr was retired across
the fleet (original #138). The record states no reason for leaving; it records
the costs above. It all took eight days and two releases.

### 6. Screen inference over provider hooks

**Context.** A dashboard needs one fact per agent: working, idle or waiting. The
early drafts took it from provider hooks, with "no pixel inference". But "a
SIGKILLed tui emits nothing, so only tmux and kernel facts prove liveness." A
native path (#14, 29 September) read provider history and offered native
controls; Claude's native messages "failed exact-recipient qualification."

**Choice.** On 1 October skid began reading status from the screen
([#40](https://github.com/NielsdaWheelz/skidbladnir/pull/40)): one bounded
capture per terminal, classified by a frozen grammar per provider into activity,
interaction and notice, with `unknown` where evidence falls short. The
requirement was one contract for every launch, managed or typed by hand: "no
conversation id, native helper, provider hook or transcript is required." On 21
October #118 removed the native conversation APIs and registration hooks.

**Consequence.** Status is evidence for a person, never authority for an effect.
The spec named the costs: conservative unknowns, a few accepted false claims for
Codex, missed idles for Claude, unknown behind obscuring views "even where herdr
would guess from titles." A provider upgrade counts only after requalification;
an authored corpus of provider frames is the one test the repository keeps.

### 7. The fixed hub

**Context.** Execution is host-local: each gateway controls only its own tmux,
and clients call every gateway directly. Computers still share membership,
settings and notification state, which need one owner. The standalone fleet
(`e7c659e`, 20 October) made that owner a movable hub, with Ed25519-signed state
and a five-phase handoff.

**Choice.** Two days later the hub became fixed
([#137](https://github.com/NielsdaWheelz/skidbladnir/pull/137)): the computer
that created the fleet or last ran `skid recover`. "The old fleet machinery
assumed the hub could move and that authority needed signatures"; both went.
Members fetch plain JSON over tailnet TLS with their own bearer.
`internal/fleet` shrank from 2,950 lines to 1,367.

**Consequence.** A lost hub stops edits, enrolment, removal and notifications,
never terminals: cached membership never expires. Accepted by name: moving the
hub means `skid recover` and enrolling every device again; nothing signs fleet
state, so the tailnet's TLS and a bearer per device are the only protection. At
"one owner, three computers and one phone", the architecture says, "availability
and polish are" negotiable; exact targets and at-most-once effects are not.

### 8. The 48% reduction

**Context.** At v0.20.0 (`12b1516`, 21 October) the repository held 101,194
lines of tracked text: 34,792 of Go, 15,766 of Kotlin, 32,759 of Markdown and a
5,444-line patch to xterm.js. Static analysis found 11 unreachable functions;
"the bloat is live complexity, not dead code", caused by rules that "mandated
same-system distrust."

**Choice.** The campaign's pull requests, #120 to #168 (21 to 23 October), took
one cluster of findings each, proved by temporary tests run against the old and
new code. The cause went first: #121 replaced the 24-file rulebook with one
[rules](rules.md) file, which validates data where it enters and trusts
producers of the same release. Product cuts came with stated costs: the fixed
hub; attention held by the hub, so acknowledging on one device clears it on all;
no screen-reader support; stock xterm.js. The docs became one contract plus one
file per open issue (#162).

**Consequence.** Counting every line of every file git tracks as text, 101,194
lines at `12b1516` became 52,311 at `8dbb295` (23 October): 48.3% fewer. Go
fell 29%, Kotlin 39%, Markdown 75%, and the xterm.js patch went. The owner
stopped there (#168): a one-fifth target was "out of reach on the decided
scope", the rest being kept on purpose, such as the grammars' precision and
workspace recovery. Stock xterm.js has yet to be accepted on a physical phone.
