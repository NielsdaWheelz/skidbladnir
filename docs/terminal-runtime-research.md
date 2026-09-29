# terminal ownership and agent interaction research

2026-09-29 · research and recommendation; no implementation or acceptance change.

question: should skid replace tmux with its own pty/multiplexer to improve
process awareness, output access, and interaction with codex/claude tuis?

this review used three specialist subagents covering terminal internals, agent
control, and terminal ux, plus root synthesis. their positions below are an
engineering council exercise, not testimony from the products' maintainers.
evidence consists of repository inspection, upstream code and documentation,
original papers, and attributed user reports. no live terminal, provider, ssh,
or device experiments were performed. those boundaries remain `NOT_RUN`.

repository source was inspected at `71cea01`, alongside existing uncommitted
documentation. in particular, `docs/native-agent-observation.md` described an
unimplemented next target, with related changes in architecture, agent control
and roadmap. that proposal was present in the working tree but is not included
or adopted by this research commit. its relevant boundary is summarized below
so this document does not depend on an uncommitted file to be intelligible.

the [architecture](architecture.md) remains the contract owner. the unresolved
choice is tracked in [terminal runtime decision](issues/terminal-runtime-decision.md).

## recommendation

owning a small terminal runtime around an existing emulator is credible. pty
ownership does not, by itself, solve process interpretation, agent semantics,
or readable conversation history.

keep tmux's session lifetime for now. pursue native provider observation
independently, improve reading and selection at their responsible surfaces,
and evaluate a proper tmux control-mode client. replace tmux if that integration
proves harder to understand and maintain than an owned runtime, or cannot meet
a concrete essential requirement.

the strongest argument for replacement is coherent ownership of terminal state,
attachment, input and resize. the weakest is the expectation that owning bytes
will reveal what an agent is doing. implementation size is evidence in the
comparison; correctness and conceptual clarity decide.

## the responsibilities hidden inside the complaint

| desired fact or behavior | responsible owner | benefit and limit of owning the pty |
| --- | --- | --- |
| running process and foreground job | kernel plus process supervisor | direct child accounting and terminal access; arbitrary shells, wrappers, pipelines and remote processes still require observation |
| exact emitted output | application-facing pty stream | direct byte access; tmux control mode already exposes pane output |
| current terminal image | terminal emulator | explicit screen, cursor and mode state; overwritten content may no longer exist |
| currently selected conversation | provider tui | launch identity does not identify later conversation switches |
| working, blocked, completed, failed | provider runtime and turn | output activity is an imperfect proxy |
| complete agent reply/history | provider history | a tui may never emit the whole document into terminal history |
| selection, copy, viewport and unread acknowledgement | client | better local interaction policies; these do not require owning process lifetime |

four different things are routinely called output:

| representation | preserves | does not establish |
| --- | --- | --- |
| byte stream | emitted bytes and order from the observation point, including escape sequences | readable text, earlier output, per-writer provenance or application meaning |
| emulator state | current cells, cursor, modes, alternate screen and wrapping | every overwritten character or a chronological conversation |
| scrollback | retained lines that entered history | every fullscreen application's transcript |
| provider history | messages, turns, tools and native identities | exact terminal presentation or arbitrary shell output |

a tui may overwrite one line ten times. byte capture preserves ten updates;
screen capture preserves the current image; provider history may preserve one
answer. these representations answer different questions.

pty output also does not identify which process wrote each byte. foreground and
background output can mix even when the terminal has rich shell integration;
[warp documents that limitation](https://docs.warp.dev/terminal/blocks/background-blocks).

## current skid implementation

the attachment path is:

```text
provider tui -> pane pty -> tmux -> attach-client pty -> gateway -> xterm
```

- [attachment.go](../internal/tmux/attachment.go), `StartAttachment`, starts an
  ordinary `attach-session` inside another pty. `Read` consumes tmux's rendering
  for that client, rather than the original pane stream.
- [control.go](../internal/tmux/control.go), `CapturePane`, already reads tmux's
  retained text with soft wraps joined and a bounded tail. rendered text access
  exists; original byte streaming is absent from the current integration.
- [process.go](../internal/process/process.go), `ObserveForeground`, resolves
  the foreground process group. [linux](../internal/process/process_linux.go)
  and [darwin](../internal/process/process_darwin.go) obtain native process facts.
  [agent.go](../internal/sessions/agent.go) associates them with the pane.
  changing the launcher does not remove this work for arbitrary commands.
- [service.go](../internal/agentcontrol/service.go), `Enrich` and `Read`, combines
  terminal capture/detection with claude native inspection. alternate-screen
  reads identify their scope as visible content. native codex observation is
  not implemented in this source.
- [terminal.go](../internal/gateway/terminal.go) pumps attachment output into
  a [bounded queue](../internal/terminal/queue.go). exceeding one mib ends the
  slow attachment. gateway loss closes the owned client while tmux preserves
  the session. this is a valuable failure boundary.
- [terminal.js](../android/app/src/main/assets/terminal/terminal.js) configures
  1,000 scrollback rows. the [selection contract](terminal-selection-copy.md)
  already reserves phone-local long-press selection even under application
  mouse reporting, then freezes the copied text at release. selection covers
  only the active buffer retained since attachment and has no post-release
  adjustment handles.
- [touch scrolling](terminal-touch-scroll.md) deliberately routes a drag as a
  wheel through xterm. depending on terminal state, that means application
  mouse input, cursor-key fallback or local scrolling. the [key deck](terminal-key-deck.md)
  sends page-up/down keys to the application. those keys do not promise local
  history scrolling.
- [readable sizing](terminal-readable-sizing.md) uses shared latest-client
  geometry. changing the pty owner would require choosing another sizing rule;
  it would not eliminate that choice.

the uncommitted native-observation target already separates semantic operations
from terminal operations. providers retain history, execution and queues; tmux
retains lifetime. codex requires a proposed tui/app-server selected-view binding,
including process lifetime and conversation revision. that is a proposed provider
capability, not an existing universal interface for attaching to arbitrary tuis.
owning the pty would not remove the need for this association.

## products and mechanisms worth borrowing

| precedent | mechanism to borrow | purpose and cost |
| --- | --- | --- |
| [iterm2 tmux integration](https://iterm2.com/documentation-tmux-integration.html) | native windows, selection, search and scrollback over control mode | keep mature persistence while presenting interaction in the client; state restoration is real integration work |
| [cmux remote tmux](https://cmux.com/docs/remote-tmux) | map sessions, windows and panes to native surfaces | remove the terminal-inside-a-terminal presentation; the documented feature is beta |
| [herdr concepts](https://herdr.dev/docs/concepts/) | separate pane, process and agent concepts; deliberate attachment policy | location and occupant have different identities and lifetimes |
| [wezterm multiplexing](https://wezterm.org/multiplexing.html) | daemon-owned domains attached to native clients | coherent runtime/client separation, with compatible installations required |
| [vscode shell integration](https://code.visualstudio.com/docs/terminal/shell-integration) | explicit command boundaries, cwd and exit status | producer-supplied facts enable reliable navigation and copying |
| [ghostty shell integration](https://ghostty.org/docs/features/shell-integration) | explicit prompt/cwd information and terminal-native interaction | add useful semantics through a narrow interface |
| [warp blocks](https://www.warp.dev/blog/block-model-behind-warps-agentic-development-environment) | commands and results as manipulable objects | structure requires cooperation from the producer; arbitrary fullscreen apps still have terminal constraints |
| [wave durable sessions](https://docs.waveterm.dev/durable-sessions) | host job manager retains the pty and disconnected output | survival is owned beside the running process |
| [zellij resurrection](https://zellij.dev/documentation/session-resurrection) | explicitly distinguish layout/command restoration from live execution | restored commands wait for deliberate rerunning; wrapper-based command discovery has limits |
| [mosh](https://mosh.org/mosh-paper.pdf) | synchronize display state while preserving input ordering | obsolete frames can be skipped without treating user actions as disposable |

cmux is an instructive hybrid. its ordinary terminal uses ghostty, while its
[local tmux profile](https://github.com/manaflow-ai/cmux/blob/main/docs/local-tmux.md)
provides live process survival across gui quit, crash and update. ordinary
layout/history restoration is distinct from keeping the original process alive.
its remote control-mode integration delegates sizing/reflow to tmux and provides
native selection, but documents paste and reflow limitations. those limitations
should inform our acceptance cases; they are not behavior to copy blindly.

### tmux control mode reaches the underlying stream

the [official protocol](https://github.com/tmux/tmux/wiki/Control-Mode) exposes
application output through `%output`, with transport escaping, plus pane/session
events and flow control. bytes need not form complete utf-8 characters or escape
sequences in each message. tmux-generated copy-mode screens are not the pane's
application output.

the [manual](https://man.openbsd.org/tmux) also provides `pipe-pane` and
`capture-pane`. a pipe observes output but introduces its own command/lifetime
and per-pane pipe slot. capture retrieves retained content; neither primitive
alone supplies a full terminal-state model.

the difficult boundary is joining an already-running terminal. a text snapshot
does not include every cursor, mode, scroll-region, alternate-buffer or partial
escape-sequence state needed to continue correctly. iterm2 separately fetches
[history and pending bytes](https://github.com/gnachman/iTerm2/blob/master/sources/tmux/TmuxWindowOpener.m#L240)
and restores [terminal state](https://github.com/gnachman/iTerm2/blob/master/sources/tmux/TmuxStateParser.m#L13).

cmux source documents concrete failure mechanisms:

- [snapshot boundary](https://github.com/manaflow-ai/cmux/blob/main/Sources/RemoteTmuxPaneSeed.swift#L8):
  distinguish bytes already represented by a snapshot from later bytes applied
  once. a snapshot cannot continue a partially parsed live escape sequence.
- [resize ordering](https://github.com/manaflow-ai/cmux/blob/main/Sources/RemoteTmuxControlConnection%2BSizing.swift#L105):
  capture immediately after resize can precede the application's redraw and
  overwrite the correct later display with stale content.
- [pane recovery](https://github.com/manaflow-ai/cmux/blob/main/Sources/RemoteTmuxControlConnection%2BPaneSeed.swift#L299):
  one slow renderer previously caused reconnect and sibling history loss;
  synchronous repeated recovery exhausted the stack.
- [output routing](https://github.com/manaflow-ai/cmux/blob/main/Sources/RemoteTmuxSessionMirror%2BOutputRouting.swift#L36):
  replayed history must not emit old notifications again.

these are upstream implementation comments inspected during research, not local
reproductions. feeding `%output` to xterm demonstrates streaming, not correct
attachment and recovery.

### herdr demonstrates both the benefit and the limit of ownership

herdr owns the [pty master and spawned child](https://github.com/herdrdev/herdr/blob/7479f8e126da7579c9b2e3513086652aa4976bb4/src/pty/backend/unix.rs#L12)
and asks its [pty actor for the foreground process group](https://github.com/herdrdev/herdr/blob/7479f8e126da7579c9b2e3513086652aa4976bb4/src/pty/actor/unix.rs#L302).
it still needs process recognition, screen detection and provider integration.

its [fullscreen history reader](https://herdr.dev/docs/agent-automation/#alternate-screen-history-reads)
sometimes scrolls a recognized idle agent, gathers overlapping pages, then
restores the bottom. working/blocked agents and several passive/direct-attachment
cases are excluded. this can be useful automation, but it changes application
state to obtain history. it is not a canonical provider transcript.

version matters: inspected master `7479f8e126da7579c9b2e3513086652aa4976bb4`
adds codex working/idle lifecycle hooks alongside identity registration. those
additions were absent from the inspected `v0.9.2` installer. master still has
[authority classification](https://github.com/herdrdev/herdr/blob/7479f8e126da7579c9b2e3513086652aa4976bb4/src/detect/mod.rs#L323)
and [session arbitration](https://github.com/herdrdev/herdr/blob/7479f8e126da7579c9b2e3513086652aa4976bb4/src/terminal/state.rs#L736).
do not generalize yesterday's screen-only finding to every version, or treat
master source as proof of a released capability. these hooks are comparative
evidence, not a proposal to override skid's hook restrictions.

### user reports and provider interfaces

[codex issue 45044](https://github.com/openai/codex/issues/45044) describes
selection breaking across viewport boundaries without tmux, including with
`--no-alt-screen`. [herdr issue 4507](https://github.com/herdrdev/herdr/issues/4507)
describes false idle during active codex work; [the correction](https://github.com/herdrdev/herdr/pull/4563)
rejects static chrome as sufficient idle evidence. these establish reported
failure mechanisms, not prevalence or a reproduction of this user's setup.

current official codex documentation describes `tui.raw_output_mode`, `/raw`
and `alt-r` for easier selection, alongside `--no-alt-screen`:
[configuration](https://learn.chatgpt.com/docs/config-file/config-reference),
[cli reference](https://learn.chatgpt.com/docs/developer-commands?surface=cli).
qualify these against the pinned provider before assuming a runtime replacement
is necessary. a documented mode does not prove every selection defect fixed.

[codex app-server](https://learn.chatgpt.com/docs/app-server) exposes native
thread history, runtime state and turn outcomes. it still requires a correct
association with the tui's selected conversation. [noninteractive exec](https://learn.chatgpt.com/docs/non-interactive-mode)
provides structured automation events but operates a different run; it does
not observe an arbitrary existing interactive session.

the [claude sdk](https://code.claude.com/docs/en/agent-sdk/overview) operates an
agent loop. [session resume](https://code.claude.com/docs/en/agent-sdk/sessions)
continues conversation state; it does not establish attachment to the same live
tui process. even [stop hooks](https://code.claude.com/docs/en/hooks#stop) have
specific interrupt, background-work and persistence semantics. native events
need explicit interpretation; owning a pty does not strengthen their meaning.

## council agreement and dissent

the terminal specialist favors examining control mode first, preserving mature
session survival and ordinary terminal access. the objection to a superficial
bridge is incomplete restoration of state, output ordering and terminal replies.

the agent-control specialist favors native semantics regardless of the terminal
owner. process, pane, selected conversation and turn identities must remain
distinct. no new supervisor can infer all four from a launch pid.

the ux specialist wants ordinary reading, search, reflow and copying for long
answers, alongside faithful interaction with arbitrary tuis. reading should not
scroll somebody else's application or steal its terminal geometry.

the strongest dissent favors an owned runtime: reconstructing a foreign
emulator through commands and synchronizing a second emulator may be more
complicated than one authoritative host terminal engine. that argument wins if
skid intends to own the complete terminal experience and the narrower bridge
cannot satisfy its requirements cleanly.

the reliability counterargument is that tmux already protects live work while
skid is frequently restarted and repaired. replacement inherits a lifetime
boundary, crash semantics, upgrade policy and attachment protocol. restoring
conversation text does not restore the running computation.

agreement: reuse terminal competence, preserve factual ownership, distinguish
survival from restoration, and decide from concrete journeys. disagreement:
whether the total complexity of a tmux bridge is lower than an owned daemon.
source inspection alone does not settle that comparison.

## philosophy and proposed interaction

place authority where the information exists. this applies the
[end-to-end argument](https://web.mit.edu/6.1800/www/readings/papers/endtoend.pdf):
a lower layer cannot fully implement behavior requiring knowledge held by the
application. terminal activity cannot authoritatively declare a turn complete;
conversation history cannot reproduce arbitrary terminal interaction.

[acme](https://9p.io/sys/doc/acme/acme.pdf) supplies a complementary principle:
displayed text should be useful material people can select, manipulate and act
upon through consistent interactions. this is an interpretation for skid, not
a claim that acme's design makes arbitrary tui output into a document.

proposed surfaces, requiring an explicit product-scope decision:

- live terminal for direct interaction, faithful application input and dialogs;
- a bounded provider-backed reply view with ordinary selection, reflow, search
  and copying, associated with the exact conversation;
- an explicitly identified terminal snapshot when reading arbitrary programs
  is useful. it must not imply that undisplayed history was recovered.

provider history remains provider-owned. a reply view need not create a second
chat runtime, copied history database or conversation registry. its cost is
two presentations with different semantics, and maintenance of native bindings.

## architecture options and explicit costs

| option | benefit | cost and limit |
| --- | --- | --- |
| current attachment plus native observation/client improvements | smallest lifetime change; preserves existing terminal access | retains tmux-rendered presentation and shared terminal interaction |
| tmux lifetime plus a control-mode client | original pane output, native client presentation, ordinary tmux escape hatch | protocol parsing, state restoration, reflow/resize coordination and duplicated emulator state |
| owned pty daemon plus a reused emulator | direct lifecycle/stream control and authoritative host terminal state | new runtime and protocol, crash/update semantics, deployment coupling, loss of plain tmux attachment to those sessions |

evaluate reuse of an existing runtime before inventing a new one. this survey
identifies mechanisms; it does not establish that herdr or wezterm is a drop-in
library for skid.

if replacement wins, the stable host daemon owns ptys, process lifetime,
terminal state, bounded history, input ordering and geometry. clients own
rendering, viewport, selection and accessibility. provider adapters retain
semantic observation. the frequently changing gateway must not become the
process lifetime owner.

[xterm headless](https://github.com/xtermjs/xterm.js#nodejs-support) explicitly
supports host terminal state and serialization for reconnect. it adds a node
runtime to the go host. [libghostty-vt](https://github.com/ghostty-org/ghostty/blob/main/include/ghostty/vt.h)
provides a reusable native terminal core, but its public api is currently
declared unstable. it adds native integration and version maintenance. neither
dependency is cost-free, and neither authorizes writing our own vt emulator.

there is no present need for high availability, reboot resurrection, scheduling,
a transcript database or a general backend/plugin framework. daemon crash may
lose sessions if stated explicitly; tmux server death also does. gateway restart
and daemon restart must have separately stated consequences. replacing binaries
cannot resurrect a killed process or retract a misdirected keystroke.

### shared geometry and input

one terminal application has one grid size. clients can have independent
viewports; two independent application layouts require application cooperation.
herdr uses [last-interactor sizing](https://herdr.dev/docs/concepts/#client-and-server)
for shared tabs and [explicit writable-controller ownership](https://herdr.dev/docs/persistence-remote/#direct-terminal-attach)
for direct attachment.

| policy | benefit | cost |
| --- | --- | --- |
| latest interacting client sizes the application | convenient handoff | other views change shape |
| fixed grid with pan/zoom | stable application geometry | poor phone reading |
| explicit input/resize owner | predictable simultaneous attachment | takeover action and coordination policy |
| native reply view | independent reflow | cannot reproduce arbitrary tui interaction |

initial recommendation: preserve skid's current shared-input/latest-client
sizing for live interaction. a reader does not resize. introduce exclusive input
ownership only for an observed coordination problem and an explicit contract
change. control mode alone does not create exclusive authority over ordinary
tmux clients.

## invariants and discriminating experiments

the following are decision criteria, not new production contracts or gates:

1. one authoritative terminal size, with a declared handoff rule.
2. ordered input; paste remains a paste; uncertain delivery never triggers
   automatic resubmission.
3. one owner of terminal query responses, preventing duplicate device/cursor
   replies from mirrored emulators.
4. reconnect establishes consistent state plus subsequent updates without a
   gap or duplicate application at the boundary.
5. slow clients consume bounded memory and do not stall every observer.
   [xterm flow control](https://xtermjs.org/docs/guides/flowcontrol/) matters:
   websocket delivery alone does not prove the renderer caught up.
6. detach, gateway restart, daemon crash and host reboot have distinct outcomes.
7. selection and historical reading remain local unless the user deliberately
   sends input to the application.

compare candidate approaches against a small set of journeys:

- reconnect into an active alternate-screen application, including partial
  escape/utf-8 boundaries, modes and query replies;
- hand off phone/desktop geometry during redraw while preserving independent
  selection/reading where promised;
- saturate output while keeping interrupt responsive and memory bounded;
- paste multiline text and select/copy unicode through the actual client;
- restart the gateway and verify that the original process survives;
- exit the provider and return to the same shell without confusing occupant
  identity, conversation identity or session lifetime.

start with the control-mode boundary; compare an owned-runtime candidate only
where concrete failures or conceptual complexity justify it. no line-count or
calendar estimate was established by this review.

any experiments require the applicable current-turn approval, isolated resources
and content-free evidence. follow [testing policy](rules/testing.md): temporary
change-specific behavioral checks do not recreate retired harnesses, and
engineering checks cannot establish live acceptance.

## questions that determine the decision

- must plain `tmux attach` remain an escape hatch when skid is broken?
- must manually created tmux sessions remain first-class?
- does output access primarily mean original bytes, terminal history or complete
  provider replies? how much retention is required?
- may phone attachment resize the desktop application, or only deliberate
  interaction? must two clients type simultaneously?
- must the original process survive gateway updates, daemon updates, or both?
- which essential experience remains impossible after native reading and a
  richer terminal client?

no preference was inferred from the unanswered question about retaining ordinary
tmux attachment. the recommendation provisionally values that existing capability.
all upstream moving-branch links and product documentation describe the research
date; refresh relevant claims before implementation. user reports remain reports.
