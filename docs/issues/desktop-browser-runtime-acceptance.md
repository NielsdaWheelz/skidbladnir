# desktop browser runtime acceptance

## session-row qualification

2026-10-14; row/info source `2c918070874e` on darwin, isolated branch
`codex/desktop-session-rows` from `8551cd3`. the [session-row contract](../desktop-session-rows.md)
is implemented and qualified. the same frozen probes failed the intended old
row/info behavior at `8551cd3`, then passed after the production edit:

- `TestSessionRows`: real tls decoding/context resolution and model updates/view;
  columns at 80/100/120, color and `NO_COLOR`, full status/provider/profile,
  unicode and invisible groups, named/unassigned/empty views, offscreen stale33,
  remote known/unknown/failed/checking, configured label/key/unknown info,
  scrolling/resize, duplicate lifetimes and captured confirmation after refresh.
- `TestSessionRowsLive`: real browser/event loop, loopback gateway and two
  distinct owned tmux sockets. rendered grids at 80×24/120×30, one row per
  terminal, info, exact selected attachment, `ctrl-] d`, first navigation key,
  refresh/selection and both original cwd/session lifetimes pass (5.21s).
  terminal continuity also passed against the old renderer before the edit.
- content and independent adversarial review pass. no additional refactor was
  warranted; old row helpers/columns are removed. both disposable probes, their
  artifacts and the resolved profile-wording issue are removed.
- `scripts/check verify host` passes after probe deletion: formatting, syntax,
  shell lint, generated assets, dependency integrity, go vet and host build.

live traffic reached fixtures only; cleanup reaped the browser, closed gateway
terminals/tls and removed only recorded owned tmux sessions. no provider accounts,
production sessions, phone or deployment were involved. model/effort stays deferred.
this scoped pass does not qualify the broader historical matrix below.

## remaining browser acceptance

pr 3's [a4](../desktop-browser.md#7-acceptance-and-delivery) requires the real
browser/pty/gateway/isolated-tmux journey on linux and darwin. unit tests and
cross-compilation cannot establish tty ownership or attachment return.

2026-09-17: the approved linux journey failed against the preceding browser at
immediate space selection, then passed on the composed candidate (2.660s).
ssh authentication initially failed, and the user instructed skipping darwin
in that turn. access was later restored; a later turn approved isolated tmux
work, but the `v0.10.3` delivery still did not record a darwin browser journey.
that boundary remains `NOT_RUN`.

with current-turn approval, use a temporary real browser → pty → gateway →
isolated tmux journey under
[testing policy](../rules/testing.md). prove exact creation/attachment, retained
filters and selection, the first navigation key after detach, escape after
`ctrl-] d` staying in skid, `ctrl-c` quitting, source survival, and no creation
replay after a lost reply. record content-free results in the change; delete
this issue when the boundary passes.

the terminal-continuity extension adds final-release `n` quick creation,
`N` options, and `T` new-terminal-here to this live boundary, including target
and default selection, offline failure, duplicate suppression, and uncertain
creation without replay. [pr 4](https://github.com/NielsdaWheelz/skidbladnir/pull/4)
passed temporary component checks but explicitly left the rebased browser's
live fleet run unperformed. the `v0.10.3` delivery did not record this browser
journey. these cases remain `NOT_RUN` and belong in the same approved proof.

2026-09-30: browser `x` now interrupts then closes; `c` is removed and `s` retains
the terminal. temporary browser/client checks against a loopback tls fixture
pass confirmation/cancellation, exact-target dispatch, busy duplicate suppression,
partial-close feedback and removed-key behavior. provider/tmux closure for this
change remains `NOT_RUN`; include `s`/`x` in the approved live journey.

2026-09-30 info editing adds [a6](../desktop-browser.md#7-acceptance-and-delivery):
space → `r`/`g` → save/cancel must return to the captured info lifetime.
regrouping out of the table view must keep info usable. prove same-name manual
takeover, automatic reset, naming conflicts, lost acknowledgements without
replay, fresh-read fences and unavailable/replaced targets. temporary model and
loopback HTTP fixtures qualify client behavior, not the real gateway/tmux write
or terminal interaction. the live editing journey is `NOT_RUN`; current-turn
tmux approval is the remaining prerequisite.

2026-10-01: [revised attention qualification](../reply-notifications.md#qualification)
passes actual darwin browser → pty → gateway → isolated tmux attachment, ready
consumption and return, plus standalone entry and typed creation-response
admission. this attention slice does not qualify the full creation, naming,
control, duplicate/lost-reply and navigation matrix above; this record remains
open for those rows.
