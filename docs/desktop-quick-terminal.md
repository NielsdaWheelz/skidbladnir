# desktop quick terminal

2026-10-16: implemented and [source-qualified](#qualification). `n` chooses only
a machine; enter creates and immediately attaches a home terminal there. this plan owns that narrow replacement of the desktop
creation clauses in [browser simplification](desktop-browser-simplification.md),
[desktop browser](desktop-browser.md) and [terminal continuity](terminal-continuity.md).
the [architecture](architecture.md), [rules](rules.md) and
[testing policy](rules.md#testing) govern all other behavior. no product question
remains open. v0.17.3 is deployed; [shared installed qualification](desktop-single-line-rows.md#installed-qualification)
records the three-host/phone update and read-only chooser/resize/cancel smokes.
creation/attachment claims retain the isolated source qualification below.

## behavior and content

one feature, one content designer. good content makes the destination, fixed
context, available action and failure legible at 80×24 without color. reuse the
existing modal chrome, focused field, wrapping and notice styles.

```text
quick terminal

machine  ‹ macbook ›
home (~) · group: research

←→ machine  enter open terminal  ctrl-r refresh  escape cancel
```

| event or state | required result |
| --- | --- |
| table `n` | open `quick-create`, select the configured default and capture the current named group; all/needs-input/unassigned means unassigned. opening never depends on availability or creates anything |
| left/right | wrap through `client.Machines()` in configuration order, including checking/unavailable machines; change only the draft machine |
| refresh | update availability, never destination or captured group; no automatic host substitution |
| available | selected peer has `OK`; no extra ready prose; enter submits |
| checking | selected peer has `!OK` and no error; show `checking inventory; opening disabled` |
| unavailable | show `<host> unavailable; opening disabled`; a known error takes precedence over checking; enter sends nothing and keeps the chooser open |
| accepted enter | copy the draft, close the chooser, show `opening terminal on <host>…`, then call existing `execute` once |
| escape or `q` | clear the draft and return to the table without creation or a navigation change |
| other input | retain existing `ctrl-r` refresh and pre-dispatch `ctrl-c` quit; ignore tab, text, paste and other creation shortcuts |
| dispatched | retain the existing busy lane: all keys are refused with `operation in flight; delivery will be reported` |

the only editable control is `machine`. context is `home (~) · group: <label>`
or `home (~) · unassigned`; retain authored label case and sanitize only for
display. no name/profile/directory/group editor or separate create control.
availability copy is derived from the current choice, so cycling away from a
failed host cannot leave its refusal on a ready host. one configured machine
still requires enter. below 80×24, retain the draft and existing size notice;
before dispatch allow escape/`q` cancellation and `ctrl-c`, never submission.
the busy lane continues to refuse every key after dispatch, including after resize.

desktop `start` failure notices prefix the chosen host to the existing
`fleetclient.ErrorMessage` text. this applies to `n` and `N` through their shared
completion path; preserve error classification, uncertainty and known-created
handle messages. the designer also owns cli help and the remote `T` refusal:
`terminal here unavailable for a remote connection; use n to choose a machine`.

## state and capability contract

`sessionui` owns selection and submission; `fleetclient` owns configured routing
and transport; the gateway owns host creation; tmux owns terminal lifetime.

```text
table -> quick-create draft -> table + existing busy operation
  -> start receipt -> exact returned session -> existing fullscreen attachment
```

`m.page == "quick-create"` gives that page ownership of `m.pending`. initialize
the entire existing `fleetclient.Request` once on opening:

| field | value |
| --- | --- |
| `Operation`, `Kind` | `"start"`, `LaunchTerminal` |
| `Machine` | `client.DefaultMachine().Label`; cycling replaces this configured label |
| `CWD`, `Group` | `"~"`, captured named group or zero/unassigned label |
| other fields | zero; name/profile/model/effort/text/ref are absent |

configuration uniquely maps the label to its pinned machine handle and stays
fixed for the client lifetime. no second selection index, readiness cache,
draft struct or persisted preference. `peerLabeled(m.pending.Machine)` resolves
the draft's selected peer for admission and rendering;
missing/unavailable peers cannot authorize submission. reuse `targetChecking`.

handle quick-page keys before generic modal navigation. on accepted enter,
copy `m.pending`, set `m.page = ""`, then call `execute(request)`. this ordering
preserves the current creation-result adoption guard, which accepts the table
or the full creation form. metadata keeps its independent `editor.request`.
reuse busy suppression, usage suspension, access invalidation,
`refreshAfterAction`, result decoding, reveal/selection and attachment.

the existing api remains `POST /v1/sessions` on the selected gateway with
`{kind:"terminal", cwd:"~", group?:<captured label>}`. omit unassigned group and
optional name; the host expands home, validates cwd and owns automatic naming
and shell startup. existing `201 {observedAt, session}` establishes creation,
not shell readiness. consume the existing `StartResult`; add no route, dto,
configuration, persistence, dependency or polling change.

| outcome | required behavior |
| --- | --- |
| dispatched failure or unknown creation | chooser remains closed; preserve collection view/selection and existing failure/access handling; never replay, infer success or identify a guessed session from inventory |
| known creation with failed follow-up | retain the existing exact-target/handle diagnostic; inspect through existing controls, never recreate automatically |
| successful creation | reveal/select the returned lifetime, change view only as needed to admit its actual group, then attach that exact reference |
| attachment failure or detach | retain the created session selection; retry attachment through ordinary enter |

duplicate suppression lasts while the action is in flight. a later deliberate
`n` submission may create another terminal; no exactly-once or remote-cancellation
guarantee is added.

## scope and ownership

freeze the page name, draft contract, content table and acceptance before
parallel edits.

| role | exclusive write paths and responsibility |
| --- | --- |
| state builder | `internal/sessionui/{session,create}.go`: chooser transitions/admission; host-prefixed start failures; remote `T` copy; small-terminal cancellation |
| presentation builder | `internal/sessionui/view.go`, `internal/agentcli/run.go`: one-field body, context, footer/hints, help |
| root integrator | this spec; `docs/{architecture,roadmap,desktop-browser-simplification,desktop-browser,terminal-continuity,shells}.md`; `docs/issues/desktop-browser-runtime-acceptance.md`; later temporary `internal/sessionui/quick_terminal_test.go` and `internal/sessionui/quick_terminal_live_test.go` |
| content designer | no writes; defines fixtures/copy within the content contract and reviews rendered output before and after implementation |
| adversarial verifier | no writes; challenges requirements, test sensitivity, implementation, refactor and final evidence at each step |

root owns all scope decisions, integration, temporary test authoring/execution
and evidence. builders never edit another slice; integrate before running tests.
preserve unrelated working-tree changes.

hard cutover deletes the immediate-dispatch `n` branch and replaces its active
help/contracts. retain `creationPeer`, the `N` form and its search/group helpers:
they still have callers. reuse `field`, `window`, `hints`/`keyLines` and current
error mapping. do not reuse the full form's availability-filtered machine cycle.
extract only demonstrated shared logic; no generic picker or form-mode framework.
keep historical qualification source-attributed; align active summaries at
implementation cutover and record new qualification separately.

excluded: backend/provider/android changes, machine filters or discovery,
remembered choices, failover, preflight requests, new readiness/cancellation
protocols, durable receipts/retries, new settings, unrelated cleanup and release
or deployment. `N` and `T` keep their behavior; only the named copy changes apply.

## implementation and acceptance

follow [testing policy](rules.md#testing). use disposable tests against the
actual model/events/rendering and browser/pty → production client → loopback
gateways → owned isolated tmux. use two dummy machines, default different from
the selected destination; terminal-only fixtures suffice. reuse the
[documented isolated composition](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/issues/desktop-browser-runtime-acceptance.md#session-row-qualification).
no production test seams, retained harness or new gate.

1. **red:** root authors probes before production edits. designer fixes content
   fixtures; verifier reviews sensitivity. first admit healthy default-machine
   inventory; baseline must fail because `n` dispatches before enter.
   unchanged invariants may already pass.
2. **green:** integrate the disjoint builders; run the same probes. designer
   inspects actual output; verifier challenges outcomes and ownership.
3. **refactor:** remove the replaced branch and demonstrated duplication/dead
   code after checking callers. rerun affected probes and review the final code.
4. **retire:** close/reap owned processes and clean recorded sessions/sockets
   before deleting their ownership records or artifacts. delete disposable
   tests, align docs/help, then run `scripts/check verify host`. verifier checks
   the final diff, removed probes, links and content-free evidence.

| acceptance boundary | required evidence |
| --- | --- |
| chooser and content | `n` creates nothing; only machine is editable; fixed/wrapping cycle includes unavailable hosts; default resets each opening; long/unicode context wraps; 80×24 and 160×24 with color and `NO_COLOR`; resize never dispatches |
| admission and cancellation | open before inventory and with offline default; selected unavailable/checking host refuses enter without closing; ready alternate remains selectable; escape/`q` send nothing; refresh changes no draft destination/group; metadata reconciliation retains its separate target |
| creation and return | actual browser journey on darwin and linux: selected nondefault host gets exactly one terminal at its home, correct named/unassigned group and automatic naming; exact auto-attachment; detach leaves its row selected and first navigation works; exercise both default and nondefault choices |
| delayed and uncertain outcomes | delay reply while repeating enter/`n`/escape/`ctrl-r` and independently completing inventory: all keys refused, one dispatch, captured target retained. withhold a successful creation reply: one actual terminal, no replay or automatic attachment across refresh. separately fail attachment after confirmed creation: selected terminal remains and attachment retry creates nothing |
| failure content | `n` and `N` show the chosen host for definite rejection, unknown creation and known-created/follow-up failure; preserve existing mapper text and exact-target handle |
| preserved paths | focused `N` explicit creation and local `T` source creation still work; remote `T` refuses with updated guidance; no provider launch needed |

tmux execution needs explicit approval in that execution turn under
[AGENTS.md](../AGENTS.md); approval of this plan does not supply it. use only
recorded owned sessions on unique `-L` sockets; no default socket, `kill-server`,
phone or installed-fleet mutation. close gateways/terminals and reap children.
keep terminal assertions in memory; evidence contains no terminal bytes,
credentials, prompts or account data. unexecuted boundaries are `NOT_RUN`.

record source/platform, red/green outcomes, designer/verifier conclusions and
cleanup here and in the existing desktop acceptance issue. unrelated findings
get their own `docs/issues/<short-name>.md` with impact, evidence and resolution
criterion. source qualification does not establish installed acceptance.

## deliberate tradeoffs

- explicit destination adds enter even for the default or sole machine.
- resetting to the configured default sacrifices last-used convenience.
- keeping unavailable cycle stops preserves order at the cost of extra keys.
- existing inventory admission may delay initial creation; no optimistic bypass.
- closing on dispatch preserves the existing completion contract; failure
  requires reopening `n` from the default.
- in-flight blocking and no replay preserve truthful outcomes; cancellation and
  automatic recovery are unavailable, and uncertainty may need manual inspection.
- deleted probes qualify the changed source without retained regression coverage.

## qualification

2026-10-16; `codex/desktop-quick-terminal`, based on `28660ab3`. only the four
assigned client files change runtime behavior; no api, configuration or backend
change. source qualification passes on darwin/arm64 and linux/arm64 (disposable
debian trixie container), with real tls gateways, production client/terminal
startup, browser/event loop/pty and separate owned tmux sockets.

- red: healthy real inventory admitted before `n`; both platforms fail because
  baseline `n` dispatches before enter. the darwin baseline browser also fails
  the chooser.
- green: chooser/admission/cancel/resize/default reset, sole-machine enter,
  refresh/draft and metadata ownership, exact hints/focus, long unicode context,
  80×24/160×24 color and `NO_COLOR` pass. default/unassigned and nondefault/named
  creation reach the correct home, automatic name and exact auto-attachment.
- live delayed reply plus independent inventory and repeated keys creates once;
  a successful creation with its reply lost remains unknown across refresh,
  without replay or auto-attachment. failed attachment retries only attachment.
  detach/first navigation, explicit `N`, exact-source `T`, and source survival
  pass on both platforms. model completion probes preserve host-prefixed
  definite/unknown/known-created mapper text and remote `T` refusal.
- designer review and independent adversarial review pass. the refactor review
  retains the small chooser handler and shared peer lookup; the immediate `n`
  path is removed. no additional abstraction or extraction is warranted.
- all owned browsers are reaped, gateway attachments/listeners closed, exact
  sessions removed and inert fixture sockets unlinked before artifact deletion.
  both temporary probes, binaries and disposable container image are removed.
  terminal assertions stayed in memory; no installed fleet or phone was used.

`scripts/check verify host`, diff checks and local documentation links pass
after probe deletion. release/deployment/installed acceptance: `NOT_RUN`.
the broader [desktop acceptance issue](issues/unrun-acceptance.md)
retains unrelated boundaries; deleted probes provide no retained regression gate.
