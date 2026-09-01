# Tmux terminal-activity hard cut

Status: **implemented, merged, released, pinned, and deployed**. The terminal
activity source merged into Skíðblaðnir `main` at `b53ee7f`, and its deployment
source merged into `dev-server` `main` at `e09f336`. Later Skíðblaðnir release
source `7317d1e` shipped as exact `v0.2.23`, pinned by `dev-server` at `410c469`;
publication, pinning, and three-host convergence/doctor are green. Owner
behavioral reds, focused greens, both routine suites, the approved
exact-feature-tree Darwin isolated tmux gate, and focused S22+ component proof
are recorded separately. Linux
isolated tmux, release-bound S22+ platform and hands-on, provider-live,
product, and second-phone acceptance remain `NOT_RUN`. This 2026-08-31 scope
correction supersedes the
rejected agent-interaction-state candidate. Its source and proofs do not prove
this contract. Git history retains that candidate; no historical runtime,
interaction, or attention model remains an active path or compatibility
obligation.

This document owns the terminal-activity capability, implementation boundary,
and red/green proof shape. [`architecture.md`](architecture.md) owns final
product behavior and acceptance; [`roadmap.md`](roadmap.md) owns delivery
order; [`design-language.md`](design-language.md) owns presentation.
[`rules/index.md`](rules/index.md), especially
[`rules/testing.md`](rules/testing.md), remains normative.

## Outcome

Every fresh session card answers one deliberately small question:

> Has this tmux session's current window had recent tmux activity?

The answer is exactly `Active | Quiet`:

| State | Exact evidence | User decision |
| --- | --- | --- |
| `Active` | tmux updated the current window's activity timestamp within the fixed recent-activity window | Wait, or open the terminal if inspection is intentional |
| `Quiet` | a complete current observation is older than the recent-activity window | Inspect or continue the work |

These states do not mean working, idle, ready, blocked, completed, successful,
unread, alive, or safe to interrupt. A provider may be working silently while
the card is `Quiet`; a shell, progress renderer, log tail, sibling pane, or
window selection may make the card `Active`. The product keeps those costs
visible rather than pretending terminal traffic is provider semantics.

This is one provider-neutral observation. Codex, Claude, another CLI/TUI, and a
plain shell are treated identically. Provider identity may remain optional
card metadata, but it never participates in activity derivation, rendering,
sorting, or accessibility.

## Authoritative observation

Tmux remains the sole source. For each ordinary visible session, the gateway
reads the current window's built-in `#{window_activity}` in the same read-only
`display-message` snapshot that already resolves the card's current-window and
active-pane anchor. It does not read `session_activity`, an activity flag, a
silence flag, a user option, process CPU, terminal contents, or provider state.

`window_activity` has the following accepted semantics:

- it is a positive Unix timestamp emitted at one-second resolution;
- any nonempty pane-PTY output batch in the window updates it, including
  control-only output and output from a non-active sibling pane;
- window creation and making that window current update it;
- keyboard or mouse input does not update it directly, although terminal echo
  or an application response is output and therefore does;
- selecting another pane does not update it by itself;
- reading inventory does not update or clear it and requires no tmux option;
- linked or grouped sessions that currently point at the same underlying
  window necessarily observe the same timestamp.

The card remains anchored to the session's current window. Activity from a
different window in the same session does not affect the card until that
window becomes current. Aggregating all windows would let an unrelated log
tail keep the whole session active and would no longer describe the terminal
the card opens.

Do not use `#{window_activity_flag}` or `#{session_activity_flag}`. They are
monitor configuration and alert-lifecycle facts, not a recent-activity clock;
they depend on per-link state and can be cleared by ordinary tmux navigation.
Do not enable `monitor-activity` or `monitor-silence`, install a tmux hook, or
change user configuration for this capability.

These semantics are source-verified against the installed tmux 3.7c line: tmux
owns and updates the [window timestamp](https://github.com/tmux/tmux/blob/e476c1230b958df0cb12977517d24b3dc931375b/window.c#L287-L292),
exports only its [whole Unix second](https://github.com/tmux/tmux/blob/e476c1230b958df0cb12977517d24b3dc931375b/format.c#L3887-L3900),
updates it from the [pane PTY output path](https://github.com/tmux/tmux/blob/e476c1230b958df0cb12977517d24b3dc931375b/input.c#L1012-L1051)
and [current-window selection](https://github.com/tmux/tmux/blob/e476c1230b958df0cb12977517d24b3dc931375b/session.c#L474-L497),
and exposes it through read-only formatting. This source review is platform
evidence, not a substitute for the isolated-tmux gate. Its approved
exact-feature-tree Darwin run is green; Linux remains `NOT_RUN`.

## Derivation and time

Each timing policy has one named owner at the layer that applies it:

```text
Android machine inventory scheduler: MACHINE_POLL_CADENCE = 5 seconds
Host session projection:             recentActivityWindow = 10 seconds
```

The existing Android-owned five-second inventory poll is retained.
`justify-polling`: the three gateways are independent request/response
services, Android already needs bounded reconciliation for tmux inventory,
and v0 has no always-on push channel. Activity adds no second poller, timer,
worker, or schedule. The host owns only the ten-second projection threshold;
Android owns only the existing inventory schedule and never derives activity.

After the required tmux snapshot and server identity are validated, the
session service captures the existing host `observedAt`. Let
`observedSecond = observedAt.Unix()` and let `activitySecond` be the strictly
parsed `window_activity`:

```text
Active iff 0 <= observedSecond - activitySecond <= 10
Quiet  iff      observedSecond - activitySecond > 10
```

The inclusive boundary accounts for tmux's one-second exported resolution.
Android receives the host projection and never runs a phone-side countdown or
rederives it from the phone clock. A state changes only when a fresh inventory
lands. Consequently the visible transition to `Quiet` may lag the ten-second
host boundary by one poll interval plus transport time.

The timestamp grammar is canonical positive decimal Unix seconds with no sign,
whitespace, leading zero, suffix, or overflow. A missing, zero, malformed,
overflowing, or future timestamp is a failed required observation. It never
maps to `Quiet`, `Active`, or a third activity state. If the session vanished
during the read, normal inventory reconciliation omits it. If the exact
session still exists but its required activity fact cannot be obtained, the
machine inventory fails and Android's existing machine-level stale behavior
owns the last snapshot.

Output may arrive immediately after the tmux snapshot. The result is still a
valid read that linearizes before that output; the next existing poll
converges. No retry loop, control-mode observer, or event log is added.

## Domain and API hard cut

Activity is a flat enum, not a tagged union:

```text
SessionActivity = Active | Quiet

Session {
  ...existing tmux and card facts,
  agent?: AgentRuntime,
  activity: SessionActivity
}
```

`agent` retains the already accepted optional, exact current provider/PID,
runtime-profile, and provider-session projection. Its absence is successful
optional metadata and makes no activity claim. There is no runtime-presence
label or state axis.

The strict session wire shape contains one required field:

```json
{
  "activity": "Active",
  "agent": {"provider": "Codex", "pid": 1234}
}
```

`agent` remains optional and is omitted, never `null`. `activity` is required
and accepts only the PascalCase strings `Active` and `Quiet`. Delete and reject
`status`, `signal`, `signalAt`, `runtime`, `interaction`, `attention`,
`NewResult`, and every old enum value. There is no schema version, alias,
fallback, dual reader, compatibility envelope, or unknown activity variant.
Create returns the same strict session DTO under the existing
`{observedAt,session}` envelope. Inventory keeps its existing top-level
`observedAt`; no activity timestamp crosses the API because the UI has no
consumer for it.

The session package owns timestamp validation and derivation once. Gateway DTO
mapping and Android ingress exhaustively map the domain enum; neither owns a
second threshold or inference table. Android owns the single global card order.

## Host and provider composition

No provider integration authors activity:

```text
current tmux window #{window_activity}
  -> session-owned Active | Quiet derivation
  -> strict gateway DTO
  -> strict Android model
  -> card presentation and order
```

The existing content-free `SessionStart` adapter may continue to register
optional process-lifetime runtime identity. It does not publish activity.
Delete Codex `UserPromptSubmit` and `Stop` handling, prompt acknowledgement,
semantic adapter admission, interaction leases, and every provider-state
writer.

The existing Codex completion notifier may remain only as a tiny desktop
presentation adapter that writes BEL to the pane terminal. It must not write a
tmux option, call the gateway, carry content, or affect correctness. Its BEL is
ordinary terminal output if tmux observes it and receives no privileged
meaning. Claude and every other program remain fully supported without it.

The hard-cut API still requires gateway and APK to ship together in the one
existing release. Do not add the rejected candidate's semantic `hostContract`
manifest epoch. The public release already binds the APK, both host binaries,
tag, source SHA, signer, and digests, while deployment-owned identity/BEL assets
no longer participate in activity. A second compatibility token would duplicate
that boundary without protecting a remaining invariant.

## Product and visual contract

Fresh cards render exactly:

| Wire | Visible label | Spoken state | Tone and motion |
| --- | --- | --- | --- |
| `Active` | `ACTIVE` | `Recent tmux activity at the last check` | Moss; the fixed activity facet may show one restrained angular spinner |
| `Quiet` | `QUIET` | `No recent tmux activity at the last check` | Muted; static facet |

The literal label and spoken state own meaning; color and motion are redundant.
With reduced motion enabled, `Active` uses the same static Moss facet and no
animation. The named activity bay has no age, evidence subline, unread badge,
completion mark, pulse, countdown, or provider qualifier. Optional agent and
profile facts remain quiet footer metadata.

Opening a card never acknowledges or clears anything and writes no
Skíðblaðnir-owned activity state. Its tmux operations receive no special case:
if creating or selecting the grouped window makes tmux refresh
`window_activity`, the next observation becomes `Active` exactly like any
other tmux selection. A newly created window begins `Active` because tmux
initializes its timestamp. Attach, detach, rename, resize, and key input are
otherwise reflected only if tmux itself updates the chosen timestamp. Rename
does not promise to preserve a concurrently observed activity value; the next
inventory simply derives it again.

Android orders cards by:

1. fresh `Quiet`;
2. fresh `Active`;
3. retained stale/non-actionable snapshots;
4. case-folded machine label, exact machine label, machine handle;
5. case-folded tmux name, exact tmux name, local tmux id.

This puts likely inspectable work first without allowing old activity to claim
current priority. A retained stale card keeps its last host-projected enum in
memory but does not animate and never presents it as current: existing visible
and spoken stale-machine qualification says that the value was last observed
and actions are disabled. Android never decays it using the phone clock.

## Hard-cut cleanup and scope

Implementation removes, rather than deprecates:

- `SessionRuntime`, `InteractionState`, `AttentionState`, prior `Status` types,
  their DTOs, decoders, presenters, priority/color tables, fixtures, and tests;
- `@skid_lifecycle`, `@skid_attention`, and any `@skid_interaction` reader,
  parser, writer, clear, compare-and-clear, or deployment asset;
- `internal/attention`, lifecycle/status derivation, semantic-adapter plans,
  attention revisions, interaction leases, and App Server assumptions;
- `WORKING`, `RUNNING`, `IDLE`, `SHELL`, `UNKNOWN`, `NEEDS YOU`, `READY`,
  `AGENT OPEN`, `NO AGENT DETECTED`, `CAN'T OBSERVE`, and `NEW RESULT` from
  active source, wire, UI, accessibility, configuration, and non-historical
  documentation;
- the rejected `agent-interaction-v1` host-contract manifest field and its
  producer, validator, deployment expectation, tests, and release prose.

Implementation retains and reuses:

- the existing card-anchor tmux read, host `observedAt`, five-second inventory
  schedule, machine freshness/admission, strict JSON ingress, DTO mapping, and
  global Android ordering primitives;
- optional `AgentRuntime` identity and the one foreground observation that
  validates it, without a runtime-state wrapper;
- the existing release tag/SHA/digest boundary and content-free BEL behavior;
- all unrelated strict-JSON, host-file, rename, pressure, terminal, and fleet
  correctness fixes on their independent merits.

Owned implementation paths are `internal/sessions/**`, the direct tmux query
owner, `internal/gateway/{dto,gateway}*`, the closed `agent-hook` CLI,
`android` product model/decoder/card/theme/sorting and focused tests,
deployment-owned hook/notifier configuration, and the canonical documents
named above. No `catalog/` or `scripts/test` composition change is required.
Only the root integrator changes canonical docs or gate composition.

## Red, green, and verification

Each builder observes a behavioral assertion failure before production edits.
A hard-rename compile failure is not a red. Tests assert public behavior and use
no internal mocks.

1. **Session red:** table-test strict parsing and exact derivation for now, one
   second inside, the exact inclusive boundary, one second outside, zero,
   malformed, overflow, future, and vanished-target reconciliation.
2. **Gateway red:** the authenticated inventory/create boundary accepts only
   required `activity: Active | Quiet`, optional orthogonal `agent`, and the
   existing envelope; old, missing, null, duplicate, extra, and unknown state
   shapes fail.
3. **Android red:** strict decode and total-order tests own both values,
   freshness-first/Quiet-first sorting, case-equivalent labels, stable keys,
   and rejection of all legacy fields.
4. **Compose red:** one real card matrix at the 170dp minimum and large text
   owns exact visible/spoken labels, redundant tone, Active motion, reduced
   motion, stale qualification, disabled stale actions, and absence of every
   retired badge or label.
5. **Hook/deployment red:** installed state has SessionStart identity only;
   the Codex notifier is BEL-only; no lifecycle, prompt acknowledgement,
   attention, interaction, or host-contract path remains.
6. **Refactor:** consolidate derivation and presentation with existing owners,
   move rather than duplicate surviving proofs, then require zero active
   legacy residue.

Routine `scripts/test verify` remains the first and final safe gate and invokes
no tmux, provider, or device. With explicit same-turn approval, one isolated
tmux integration uses a generic shell program—not Codex or Claude—to prove pane
output, silence, sibling-pane output, current-window selection, read-only
inventory, grouped/shared-window behavior, and no user-option/config mutation.
It owns only its private `-L` socket. Without approval it is `NOT_RUN`.

With explicit device approval, one focused S22+ component pass and one hands-on
glance cover compact/large text, TalkBack, motion/reduced motion, ordering, and
stale state. Provider-live remains the independent optional-agent-identity gate
and proves no activity behavior. Release, deployment, live host, platform,
product, and device evidence remain separate; every unrun boundary is
`NOT_RUN`, never a pass.

## Acceptance criteria

1. Every fresh card has exactly one required `Active | Quiet` value derived
   only from its current tmux window's `window_activity` and the host clock.
2. Codex, Claude, another terminal program, and a plain shell follow the same
   source, threshold, API, presentation, and order with no provider branch.
3. `Active` and `Quiet` are distinguishable without color, accurately spoken,
   and never described as working, ready, blocked, completed, unread, or safe.
4. The inclusive ten-second threshold and five-second inventory cadence each
   have one named owner; Android has no activity timer, threshold, or local
   decay.
5. Current-window selection, creation, sibling-pane output, linked windows,
   concurrent output, and delayed polling behave exactly as documented.
6. A required-observation failure never fabricates either state. Vanished
   sessions reconcile away; a persistent invalid fact fails that machine poll
   and existing stale-machine behavior takes over.
7. The strict wire rejects every old state field/value and every missing,
   null, duplicate, extra, or unknown activity shape. No compatibility path or
   schema negotiation exists.
8. Optional agent identity remains orthogonal. Agent presence, absence,
   provider, profile, PID, registration, and observation failure cannot change
   activity, priority, tone, motion, copy, or accessibility.
9. Opening, attaching, detaching, renaming, refreshing, and killing write no
   Skíðblaðnir-owned activity state and clear no result state. Any tmux-owned
   timestamp update retains its ordinary meaning. Inventory adds no tmux
   option, alert flag, monitor setting, hook, or background worker.
10. Active source/config/docs contain no retired state machinery or the
    rejected host-contract epoch; tmux user options contain no new activity
    projection.
11. Logs and evidence remain content-free and credential-free. No terminal
    bytes, prompts, objectives, provider payloads, tokens, or account data are
    captured to prove activity.
12. Routine, isolated tmux, Android, release, deployment, and live evidence are
    reported against exact trees and never substituted for one another.

## Non-goals

Agent work state, next-move ownership, approvals, elicitation, readiness,
completion, success/failure, unread results, attention history, task history,
provider notifications as authority, non-current windows, per-pane activity,
background-process detection, CPU heuristics, terminal parsing, App Server,
Agent SDK, ACP, push, SQLite, provenance, receipts, replay, telemetry, or a
general hook/plugin runtime. Adding any of them requires a separately accepted
scope and acceptance change; this cut adds no scaffold for them.

## Explicit trade-offs

| Decision | Cost accepted |
| --- | --- |
| Tmux `window_activity` instead of provider semantics | Silent work can be `Quiet`; noisy non-agent output can be `Active`. In exchange the signal is universal, content-free, read-only, and already owned by tmux. |
| Current window instead of all session windows | Background-window work can be missed. In exchange an unrelated log tail cannot keep the card active, and the state describes the terminal the card opens. |
| Window-wide instead of pane-specific | A sibling pane and current-window selection count. Tmux exposes no pane activity clock; per-pane truth would require new instrumentation. |
| Output rather than client input | Input with no terminal echo or application response can leave the card `Quiet`. Using `session_activity` would let attach, phone keys, and client navigation manufacture the signal. |
| Tmux-owned management effects | Creating or selecting a window, including selection incidental to attachment, may produce a brief `Active`. Suppressing that would require history and a second interpretation layer over tmux. |
| Shared linked windows | Two sessions pointing at the same window can show the same activity. Attributing shared output to one session would require state tmux does not expose. |
| Ten-second inclusive window on a five-second poll | Brief pauses may flicker and quiet can appear up to one poll late. A longer window would hide stops; a shorter one would amplify tmux's one-second resolution and ordinary redraw gaps. |
| Tmux wall-clock seconds | A host clock jump forward can project `Quiet`; a backward jump makes the timestamp future and fails the machine poll until time catches up. Tmux exposes an epoch timestamp, not a portable monotonic activity age. |
| Host projection with no Android countdown | The displayed state can outlive its exact cutoff until the next poll. One owner avoids clock skew, duplicate derivation, and continuous phone recomposition. |
| `ACTIVE` and one restrained spinner | The compact label can be misread as semantic work despite the exact spoken qualifier. The card gains the requested glance cue without inventing provider state; reduced motion is static. |
| Quiet-first fresh ordering | A recently noisy card moves below likely inspectable work. This serves the one user decision; it is not an urgency or success ranking. |
| Required observation fails the machine poll | One malformed required timestamp can stale otherwise usable cards. That is preferable to manufacturing `Quiet` or adding an `Unknown` state that violates the two-state contract. |
| Delete unread-result attention | The phone no longer remembers unseen completions. This removes provider-specific hooks, marker lifecycle, acknowledgement races, priority, and push scaffolding. |
| Keep optional agent identity | Some provider/process complexity remains, but it is an already accepted independent metadata capability and no longer infects the activity state model. |
| Retain BEL only as desktop presentation | Codex may still notify iTerm while other tools do not. Activity correctness remains universal because BEL has no privileged meaning and no state writer. |
| Reuse the existing release identity without `hostContract` | Mixed app/gateway versions remain unsupported and require the existing atomic release. The benefit is one release identity instead of a second semantic epoch for assets that no longer author state. |

No implementation question remains open.
