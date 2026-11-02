# History

Skíðblaðnir was specified, built and run by one person for their own computers
and Android phone, from a first commit on 22 August 2026 (`b460281`) to v0.23.0
on 26 October, its 72nd release: 39 Skíðblaðnir releases from its first
repository (v0.2.0 to v0.8.0) and 33 from this one. Below are its phases, then
nine decisions, each with the constraint that forced it and the cost accepted.

Dates are UTC: a pull request's is its merge, a release's its publication. Until
25 September the project lived in its first repository, since renamed
herdr-mobile: "original #N" is a pull request there, "herdr-mobile `abc1234`" a
commit only there, and a plain #N a pull request here. A bare hash is a commit
in this repository; cited for the specification, it means `docs/architecture.md`
at that commit unless another file is named. The cleanup tracker is
`docs/cleanup.md` as it stood at `58413a4`, before #168 deleted it.

## Timeline

| Dates | Phase | Marks |
| --- | --- | --- |
| 22–25 Aug | A rulebook, then a specification drafted over four days, first named Ariadne and renamed Skíðblaðnir on 24 Aug; no Go or Kotlin until P0 froze the contracts and proved the platform; then a scope reset to a tmux remote | `b460281`, `85b4e02`, `ec49cb2`, original #1, `6012a5e` |
| 26 Aug – 18 Sep | v0 for one host and the phone; a multi-host fleet with signed releases; agent controls across hosts; session spaces; the desktop browser; behavioral tests retired, then a run of cleanups | original #2, #9, #18, #79, #94, #98, #102, #105 to #132 (all original); v0.2.0 (27 Aug) to v0.6.0 (17 Sep) |
| 22–25 Sep | After four days without a commit, herdr: a feasibility study, the cutover, a phone-only gateway, the split | original #133 to #137; v0.7.0 (24 Sep) and v0.8.0 (25 Sep) |
| 26–30 Sep | The tmux product restored from `927c554` into this repository; herdr retired | `4eeba15`, original #138; v0.9.0 (26 Sep) to v0.10.7 (30 Sep) |
| 29 Sep – 20 Oct | Native conversation controls, then status read from the screen; account usage; workspace recovery; notifications | #14, #40, #57, #65, #66; v0.11.0 (1 Oct) to v0.18.0 (20 Oct) |
| 21 Oct | A standalone installer and a fleet with a hub; terminal-only agent control | #100, #118; v0.19.0 to v0.20.0 (21 Oct) |
| 21–24 Oct | Reduction campaign, closed on 23 Oct: 48% of tracked lines removed | #120 to #168; v0.20.1 (22 Oct) to v0.22.0 (24 Oct) |
| From 25 Oct | Publication: plan, license, host fixes, a new Android signing key | #175 onward; v0.23.0 (26 Oct) |

## Decisions

### 1. Specification first

**Context.** The first commit held no code, only 44 files of rules (`b460281`).
They "bind by intent, translated to Go and Kotlin", though some mechanisms were
"specific to a TypeScript/Effect runtime" (`f064bd2`). By 25 August the
specification had grown from 496 lines (`85b4e02`) to 1,592 (`c69f1f6`), with a
roadmap of phases P0 to P7 (`docs/roadmap.md` in `c69f1f6`).

**Choice.** No product code before the contract: the tree held no Go or Kotlin
until P0 (original #1, 25 August). P0 froze the contracts and proved, against
real tmux and a real phone, the mechanisms they rested on: tmux grouped sessions
(`evidence/live/p0-tmux-3.4.md` in `c4b418e`), xterm.js with Gboard in an
Android WebView, and provider resume. It "ships no agent-management runtime"
(original #1).

**Consequence.** Five hours after P0 merged, the 1,245-line design (`6f2d697`)
gave way to a 306-line one (`6012a5e`). The reset called the old design
"internally consistent", but it had "built an audited Codex runtime system where
the product is a specialized Android remote for tmux." Its platform evidence was
"carried forward", and v0 shipped the next day (original #2). The habit stayed:
one contract, the [architecture](architecture.md), where a new capability "needs
an explicit scope and acceptance decision by the owner." The costs came later:
the rulebook "taught same-system distrust and so produced most of the defensive
code" (#121), and by 21 October the repository held 32,759 lines of Markdown
against 15,766 of Kotlin (cleanup tracker).

### 2. tmux as the store

**Context.** The pre-reset design kept its own state: SQLite as authority, hook
and control sockets, and a registry of agents with their identity and lifecycle
facts (`6f2d697`). A restarted gateway "reconstructs owned runtime facts from
SQLite, tmux, `/proc`, and hook gap files" (`6f2d697`). The reset wanted a
restart to mean "list tmux again" (`6012a5e`).

**Choice.** "tmux is the database and the process supervisor" (`6012a5e`). skid
keeps its metadata in tmux user options. A server lifetime is a random epoch
that skid mints and stores in tmux, plus tmux's own pid and start time, so an
option restored into a new server never forges one; a session adds tmux's
session id ([architecture §2](architecture.md#2-concepts),
[`internal/sessions/identity.go`](../internal/sessions/identity.go)). Sessions
started by hand "simply appear in the inventory" (`6012a5e`).

**Consequence.** In the positioning's words, "nothing migrates and sessions
outlive skid" (#175). Every read spawns a process: on 21 October, on one Mac
with 18 to 20 agent sessions, the gateway used 13.6% of a core, its children
17.5% and the tmux server 5.0% ([exec overhead](issues/tmux-exec-overhead.md)).
The [exec budget](tmux-exec-budget.md) (#128) bounds a steady listing to two
tmux processes, shipped in v0.22.0 (#156); the remeasurement is still open.
Sessions also die with their tmux server: workspace recovery (#65, 7 October)
checkpoints every 30 seconds and, after a proven loss, rebuilds the saved tmux
graph once with fresh shells. Recovery from a real reboot has not yet been
observed (#65; [roadmap](roadmap.md)).

### 3. At most once, never replay

**Context.** When a reply is lost, a client cannot tell whether its request ran.
The first draft answered with durable operations (`85b4e02`): internal states
ending in `Applied`, `Rejected` or `OutcomeUnknown`, idempotency keys kept for
90 days, and a recoveries endpoint to resolve the unknown, under one rule:
"Ambiguity is visible recovery, never a guessed result or automatic redispatch."

**Choice.** The reset dropped the machinery and kept the refusal to guess: a
restart "never replays input or guesses", and "durable command receipts and
replay" became a non-goal (`6012a5e`). The mechanism is tmux's own command
queue. v0 already guarded its kill with `if-shell -F` on the server lifetime,
session id and name (`internal/tmux/client.go` in `ddc82b3`). Today every effect
on an existing session is one such queue: the guard checks the server lifetime
and session id (terminal effects also the active pane), the same queue runs the
effect and prints an applied marker, and tmux drains that queue before serving
another client ([architecture §6](architecture.md#host-invariants)). Since 22
October kill, group, dwarf and name changes share one guard primitive (#139,
[`internal/tmux/guard.go`](../internal/tmux/guard.go)). Without the marker the
outcome is `unknown`: reported, never retried.

**Consequence.** No layer of skid repeats an effect: a lost create is not
resubmitted, a reconnect never resends keystrokes ([rules](rules.md)). The
person resolves the uncertainty instead; a lost join reply, for one, leaves the
invitation spent ([architecture §4](architecture.md#4-fleet)). The reset
deferred receipts and replay until "notifications grow buttons that act
unattended ... or unattended orchestration arrives" (`6012a5e`). Since 31 August
they return only through "a new architecture decision for a concrete user
action" (`724cd85`, original #48).

### 4. A terminal on the phone, not a chat view

**Context.** The first draft was an Android "command deck" over Codex's App
Server (`85b4e02`): "An agent is a durable conversation, not a repo, directory,
worktree, terminal, or process."

**Choice.** Two days later (`f064bd2`), the answer to "TUI or structured chat?"
was the stock TUI: "It already owns conversation, tools, approvals, and
rendering." The phone would run xterm.js in a WebView and type through Gboard,
with "no parallel speech/composer lifecycle". The reset made the phone and the
laptop "two tmux clients": "One process, one screen, one draft" (`6012a5e`). A
chat UI is still a non-goal ([architecture §1](architecture.md#non-goals)).

**Consequence.** A new provider is a configuration row, "not a design event"
(`6012a5e`). The cost is a phone keyboard driving a terminal: a row of terminal
keys (`6012a5e`), a one-shot Shift because "gboard's shift cannot modify the
custom key bar" (#117), and a patch to xterm.js, begun for touch scrolling on 4
September (original #54) and extended to selection on 6 September (original
#60). By 22 October it also routed taps, and stock xterm.js replaced it (#143).
Raising Gboard can still resize the shared tmux window in steps, redrawing the
laptop's view too ([resize churn](issues/terminal-resize-churn.md)).

### 5. Temporary tests over a retained suite

**Context.** P0 arrived with test tiers, proof commands and a proof ledger
(original #1). The reset retired the ledger (`6012a5e`); the gates and suites
stayed and grew with the product, to eight gates, from unit to full, by 17
September (listed as removed in original #102).

**Choice.** On 17 September the owner retired the behavioral test system "so its
maintenance cost and proof machinery do not constrain the replacement" (original
#102, which removed 43,419 lines). The replacement suite never arrived. The same
day the rules made temporary tests the method (original #105): run at the
affected boundary against the old and new code, then deleted before commit,
"with no retained automatic regression protection" (`docs/rules/testing.md` in
`d8e91a6`).

**Consequence.** CI runs engineering checks and builds. They catch invalid
source and broken builds, "not session targeting, input delivery, pairing,
terminal interaction or client behavior" ([test
system](issues/test-system-reset.md)). The proofs behind later pull requests,
#65 and #137 among them, were temporary tests, deleted after use. One test
stays, at the owner's request: a replay of authored provider frames through the
screen classifier ([rules](rules.md#testing)). Pure-core unit tests were
proposed on 25 October and deferred by the owner ([test
system](issues/test-system-reset.md)).

### 6. herdr adopted, then abandoned

**Context.** herdr is, in this project's positioning, "the most capable
agent-aware multiplexer" (#175). On 22 September the plan was to "delegate
terminal compatibility and agent interaction mechanics to a maintained upstream"
(herdr-mobile `c6661a5`, `docs/herdr-migration.md`): herdr would own terminals,
pane processes, layouts and agent observations, and skid would keep its Android
app, gateways and CLI.

**Choice.** A feasibility study the same day returned "reconsider" ([original
#133](https://github.com/NielsdaWheelz/herdr-mobile/pull/133)). At herdr v0.9.1,
with close confirmation disabled, closing a workspace's sole pane also removed a
linked worktree terminal; with confirmation on, herdr guarded it. Its public
terminal command also lacked a mode-aware tap route for the phone. The owner
amended the scope to accept herdr's native close semantics and drop remote
clicks (herdr-mobile `ff114c8`, `docs/herdr-pr1.md`), and skid cut over
([original #134](https://github.com/NielsdaWheelz/herdr-mobile/pull/134)).
v0.7.0 was active on all three hosts on 24 September (herdr-mobile `f2a07b1`,
`docs/herdr-migration.md`), and v0.8.0 reduced skid to the Android app and a
phone-only gateway over herdr, deleting the CLI (original #136).

**Consequence.** The cutover accepted in writing that Claude lost native status,
history and halt confirmation, that a linked close could cascade, and that a
cold restart invalidated terminal lifetimes (original #134). At v0.9.1, herdr
did not recognize Claude as an agent in either isolated test fleet (original
#134), and live terminals were never migrated (herdr-mobile `f2a07b1`,
`docs/herdr-migration.md`). Two separate steps followed. On 25 September the
owner requested a product split for coexistence: the tmux product was restored
from `927c554`, the last commit before the herdr plan, into a new repository
that took the skidbladnir name (original #137, `4eeba15`), and both products
were to run side by side on every host and the same phone (herdr-mobile
`68a652d`, `docs/herdr-mobile-separation.md`). In a record dated 29 September
(original #138, merged 30 September), the owner retired herdr and herdr-mobile
across the fleet and moved worker control to skid, as qualified on v0.10.6,
which carried skid's new native controls (#14, #18, #21). The records state no
reason for either step; the owner gave it afterwards (26 October). herdr's
interface did not suit them: it leans on the mouse and on shortcuts they found
arcane, and pairing the phone was laborious. Having used skid, minimal and
keyboard-first, they would not go back. The episode took eight days and two
skid releases, v0.7.0 and v0.8.0. Something carried over: skid's own status
research named herdr's separation of detection from attention, its explicit
unknown and its startup readiness as "the useful borrowing"
(`docs/agent-status-research.md` in `67af37f`, #14).

### 7. Screen inference over provider hooks

**Context.** A dashboard needs one fact per agent: working, idle or waiting. The
early drafts took it from hooks, with "no pixel inference" (`f064bd2`) and
"never screen or transcript parsing" (`6f2d697`). P0's platform evidence held
that "SIGKILL of a TUI emits nothing; only tmux/process facts are reliable
liveness evidence" (`6012a5e`). A native path (#14, 29 September) read provider
history and offered native controls, but Claude's native messages "failed
exact-recipient qualification" (#14).

**Choice.** On 1 October skid began reading status from the screen
([#40](https://github.com/NielsdaWheelz/skidbladnir/pull/40)): one bounded
capture per terminal, classified into activity, interaction and notice by one
grammar per provider, qualified against named provider versions, with `unknown`
where evidence falls short. The requirement was one contract for every launch,
managed or typed by hand: "no conversation id, native helper, provider hook or
transcript is required" (`docs/terminal-observation.md` in `78b3aa4`). On 21
October #118 removed the native conversation APIs and registration hooks.

**Consequence.** Status is evidence for a person, never authority for an effect
([architecture](architecture.md)). The spec named the costs (#40;
`docs/terminal-observation.md` in `78b3aa4`): conservative unknowns, Claude's
lost idles, four accepted false idles (Claude's remote usage-limit copy and
three of Codex's) with a few other false claims for Codex, and unknown behind
obscuring views "even where herdr would guess from titles." A provider upgrade
counts only after requalification ([observation
§7](terminal-observation.md#7-requalification)).

### 8. The fixed hub

**Context.** Execution is host-local: each gateway controls only its own tmux,
and clients call every gateway directly ([architecture](architecture.md)).
Computers still share membership, settings and attention, which need one owner.
The standalone fleet (#100, `e7c659e`, 21 October) made that owner a movable
hub, with Ed25519-signed state and a planned transfer in five steps
(`docs/generalization-plan.md` §3–4 in `e7c659e`).

**Choice.** On 21 October the owner approved two cuts with their costs, a fixed
hub and no signatures (cleanup tracker), and the next day the hub became fixed
([#137](https://github.com/NielsdaWheelz/skidbladnir/pull/137)): the computer
that created the fleet or last ran `skid recover`. In #137's words, "the old
fleet machinery assumed the hub could move and that authority needed
signatures"; both assumptions went, with the five-phase handoff and the Ed25519
envelopes. Members fetch plain JSON over tailnet TLS with their own bearer.
`internal/fleet` shrank from 2,950 lines to 1,367 (#137).

**Consequence.** A lost hub stops edits, enrolment, removal and notifications,
never terminals: cached membership never expires ([architecture
§4](architecture.md#4-fleet)). Accepted by name: moving the hub means `skid
recover` and enrolling every device again, and nothing signs fleet state, so
tailnet TLS and a bearer per device are the whole defence (cleanup tracker;
[architecture §3](architecture.md#3-topology-and-trust)). The architecture's
calibration is "one owner, three computers and one phone", and under it "Exact
targets and at-most-once effects are not negotiable; availability and polish
are."

### 9. The 48% reduction

**Context.** On main after the v0.20.0 pin (`12b1516`, #119, 21 October), the
repository's tracked non-binary files held 101,192 lines: 34,792 of Go, 15,766
of Kotlin, 32,759 of Markdown and a 5,444-line patch to xterm.js. Static
analysis found 11 unreachable functions: "the bloat is live complexity, not dead
code", rooted in rules that "mandated same-system distrust" (cleanup tracker).

**Choice.** Between #120 and #168 (21 to 23 October), the campaign's pull
requests each took one cluster of findings, proved by temporary tests run
against the old and new code (cleanup tracker). The cause went first: #121
replaced the 24-file rulebook with one [rules](rules.md) file, which validates
data where it enters and trusts producers of the same release. Product cuts came
with stated costs: the fixed hub; attention held by the hub, so acknowledging on
one device clears it on all; no TalkBack or screen-reader support; stock
xterm.js (cleanup tracker). The docs became one contract plus one file per open
issue (#162).

**Consequence.** At `8dbb295` (#166, 23 October) the count was 52,309 lines,
48.3% fewer (#168). Go fell 29%, Kotlin 39% and Markdown 75%, and the xterm.js
patch went (cleanup tracker). The owner stopped there (#168): a one-fifth target
was "out of reach on the decided scope", the rest kept on purpose, such as the
grammars' precision and workspace recovery (cleanup tracker). Stock xterm.js has
yet to be accepted on a physical phone ([phone
acceptance](issues/stock-xterm-phone-acceptance.md)).
