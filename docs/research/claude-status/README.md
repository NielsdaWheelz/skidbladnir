# claude screen atlas

skid 0.32.1 called successful idle compaction an error. claude 2.1.332 uses a single foreground colour for both housekeeping and failures. on the same captured macbook pane the original detector returned `idle/none/error` and the revised detector returned `idle/none/none`. a real failed compaction exposed the opposite mistake. its error hangs under the submitting prompt, outside the bullet block skid used to inspect.

## what a status means

read the triples below as **activity / interaction / notice**. activity is `working`, `idle`, `starting` or `unknown`. claude’s ordinary startup has no qualified `starting` screen. interaction is `none`, `permission`, `question`, `confirmation`, `setup`, `input`, `menu` or `unknown`. notice is `none`, `interrupted` or `error`. composer admission is a separate observation with four values, `empty`, `draft`, `blocked` and `unknown`. an idle session can show an error or hold an unsent draft. a menu can hide work. ready is the hub's transition from work to idle and has no claude screenshot of its own.

this index covers the views sampled on macos and the other families in skid's grammar. account settings, plugins, server flags, key bindings and terminal size change which views exist. the full family inventory records qualification by renderer and platform. the command reference includes commands that print a result and immediately return to the composer. [family inventory](../../terminal-observation-claude.md#1-frozen-capability-table) · [wire contract](../../terminal-observation.md#2-capability-and-wire-contract) · [upstream commands](https://code.claude.com/docs/en/commands)

## compaction and turn notices

| observable screen | expected reading | samples |
| --- | --- | --- |
| idle housekeeping, or a summary prepared while the user was away | idle / none / none. these two screenshots use stock claude rendering a seeded informational record. the live pane comparison independently confirms the first notice. | [while idle](samples/seeded-idle-compact.png) · [away summary](samples/seeded-idle-summary.png) |
| automatic compaction scheduled for the next turn | idle / none / none in the sampled ready layout. the right-hand label alone supplies no work evidence. | [pending](samples/auto-compact-pending.png) |
| manual or automatic compaction in progress | working / none / none. the spinner can sit below a historical failure. | [manual](samples/compact-active.png) · [automatic](samples/auto-compact-active.png) |
| manual compact completed, or automatic compact followed by a completed turn | idle / none / none on the sampled settled screen | [manual](samples/compact-result.png) · [automatic](samples/auto-compact-result.png) |
| manual or automatic compaction failed | idle / none / error. the current result belongs to the submitted prompt. a tool printing the same words supplies no such evidence. | [manual](samples/compact-failure.png) · [automatic](samples/auto-compact-failure.png) |
| api error, then a successful subsequent turn | idle / none / error → idle / none / none | [error](samples/api-error.png) · [recovered](samples/error-recovered.png) |
| generation interrupted after output | idle / none / interrupted. interrupting before output restored a draft without a notice. | [interrupted](samples/interrupt-after-stream.png) |

## activity and input

| visible surface | expected reading on the pictured frame | samples |
| --- | --- | --- |
| ready, ordinary draft | idle / none / none. only the empty composer admits guarded send. | [empty](samples/ready.png) · [draft](samples/draft.png) |
| generation, api retry with and without a custom statusline | working / none / none. the lowercase countdown identifies a retry directly. | [generation](samples/working.png) · [retry](samples/retry.png) · [custom statusline](samples/retry-statusline.png) |
| main with an active subagent, then waiting for the subagent | working / none / none | [running](samples/background-agent.png) · [waiting](samples/background-wait.png) |
| approved dynamic workflow running, then waiting for completion | working / none / none | [running](samples/workflow.png) · [waiting](samples/workflow-wait.png) |
| viewing the active subagent | working / menu / none, composer blocked | [subagent](samples/viewed-agent.png) |
| shell command entry | unknown / none / none, composer draft even before typing a command | [bash mode](samples/bash-mode.png) |
| slash-command suggestions above a nonempty ordinary composer | currently idle / none / none, composer draft. the suggestion list itself goes unreported. | [autocomplete](samples/autocomplete.png) · [open issue](../../issues/claude-command-suggestions.md) |
| tool asks permission | unknown / permission / none | [bash permission](samples/permission.png) · [dynamic workflow](samples/workflow-permission.png) |
| question, option preview, multi-question submit review | unknown / question / none | [form](samples/question.png) · [preview](samples/question-preview.png) · [review](samples/question-review.png) |
| enter plan mode, leave it without a plan, approve a written plan | unknown / confirmation / none | [enter](samples/enter-plan.png) · [exit](samples/exit-plan.png) · [written plan](samples/plan-review.png) |
| mcp server requests form input | unknown / input / none | [elicitation](samples/elicitation.png) |
| setup before work can proceed | unknown / setup / none. the api key, path and server are fixture data. | [key choice](samples/setup-key.png) · [trust](samples/setup-trust.png) · [settings error](samples/setup-settings-error.png) · [bypass warning](samples/setup-bypass.png) · [one project server](samples/setup-project-mcp.png) · [several servers](samples/setup-project-mcp-multi.png) · [auto-mode offer](samples/setup-auto-default.png) |
| connectivity check fails at startup and the process exits | post-exit pixels cannot establish a current claude failure. inventory rechecks the foreground. | [residual screen](samples/startup-connectivity-failure.png) |

## navigation

each of these current menus yields `unknown/menu/none` with a blocked composer. the title and footer survive the removal of dummy session details from the images. opening a menu during a turn can obscure the activity that was visible a moment earlier. [claude interactive mode](https://code.claude.com/docs/en/interactive-mode)

| view | samples |
| --- | --- |
| model, effort, theme, help | [model](samples/menu-model.png) · [effort](samples/menu-effort.png) · [theme](samples/menu-theme.png) · [help](samples/menu-help.png) |
| settings tabs | [config](samples/menu-config.png) · [status](samples/menu-status.png) · [usage](samples/menu-usage.png) · [stats](samples/menu-stats.png) |
| permission rules, background tasks, memory settings | [permissions](samples/menu-permissions.png) · [tasks](samples/menu-tasks.png) · [memory](samples/menu-memory.png) |
| resume picker, rewind picker, side question, detailed transcript | [resume](samples/menu-resume.png) · [rewind](samples/menu-rewind.png) · [side question](samples/menu-btw.png) · [transcript footer](samples/transcript.png) |

in the empty test workspace, `/agents` printed its wizard-removal message, `/mcp` printed a no-servers result, `/diff` reported the absence of a git repository, and `/context` printed a report. each returned to idle. the foreground view decides the status.

## coverage and replay

the screenshots in `samples/` are crops of the stock darwin 2.1.332 tui in private tmux 3.8 panes at 100 × 36. isolated profiles used dummy input and a loopback messages server. setup cases used fresh configuration. the written-plan case used a dummy plan in the session's plan file. safe chrome was rendered from tmux cells using the repo's xterm 6.0.0. user text, local paths and identifying details were masked or cropped. the two idle-compaction samples exercised the stock renderer through seeded records. a timer firing in the isolated session was not observed.

the sheets below render **authored replay fixtures**. each panel names its input. these cover additional distinctions including wrapped forms, background panels, an obscured dialog, first-run onboarding and unsupported screen-reader output. use the inventory for each family's actual live qualification.

[background and limits](fixtures/background.png) · [requests and plan review](fixtures/requests.png) · [setup and onboarding](fixtures/setup.png) · [other views and unknowns](fixtures/other-views.png) · [compaction and error controls](fixtures/idle-compaction.png)

run `go test -count=1 ./internal/agentcontrol` from the repository root. the [authored frames](../../../internal/agentcontrol/testdata/frames/claude.txt) specify terminal dimensions, styled rows, all three status facts, the composer and the expected rules. the [existing capture tool](../../../internal/agentcontrol/testdata/frames/capture.py) paints dummy frames through private tmux and preserves the per-row styles that the detector actually reads. the pngs are for human inspection. add a matching frame and a nearby counterexample when a provider changes its ui.

current-build live coverage remains open for linux, narrow panes, account-gated rate-limit choices, terms and trial expiry, first-run login/theme/security behind external connectivity, teammate and cloud views, and server-gated variants such as url elicitation. cropped-off controls and unrecognized failure wording can produce unknown or no notice. [unruled requests](../../issues/claude-unruled-request-dialogs.md) · [panel ambiguity](../../issues/claude-panel-status-unknown.md) · [qualification gaps](../../issues/unrun-acceptance.md)

## what to borrow from herdr

herdr's claude manifest is a useful source of counterexamples, especially when a task panel sits beneath a request or transcript chrome persists. it answers a different operational question with three states, `idle`, `working` and `blocked`. the inspected manifest has 17 rules, no compaction-specific rule, no error outcome, and an idle fallback. in skid an idle claim can satisfy a wait and contribute to guarded-send admission. keep skid's independent facts and require visible evidence for each. [pinned herdr manifest](https://github.com/herdrdev/herdr/blob/e8d7d9705a7a2c89cef4dd42079379f05685af87/src/detect/manifests/claude.toml) · [herdr status authority](https://herdr.ink/docs/agents#status-authority)
