# standalone skid and fleet generalization

status: implementation authorized, direction updated 2026-10-21. this is the
owner's decision record. [the implementation contract](generalization-plan.md)
owns schemas, boundaries and acceptance. unrelated completed implementation is
retained. installed fleet cutover remains deferred.

skid should be one installable product for one person and their machines. each
machine owns its local tmux sessions. desktop and phone clients manage enrolled
machines directly. an optional always-on machine can hold the hub role for shared
configuration and notifications. personal machine names, account wrappers, and
deployment conventions must become user configuration.

## scope: preserve existing behavior

2026-10-20 owner clarification: keep everything possible the same and adapt only
what generalization requires. existing accepted specifications and behavior are
the baseline. the explicit decisions below override that baseline where stated;
unrelated interaction, notification, api, or release redesign is outside scope.
product changes that are not required for generalization belong in separate prs,
even if discussed or approved earlier. this scope does not absorb them for
convenience.
2026-10-20 final clarification: the owner's upgraded skid must look, feel and act
the same as today, except for the necessary generalization changes explicitly
listed here. preserve existing names/order, defaults, effective commands/accounts,
shell and terminal selection; parity is release acceptance, not an aspiration.
licensing is explicitly outside this scope and does not block generalization.
the existing [license issue](issues/project-license.md) belongs to a separate pr;
no license choice is made here.

| retained behavior | existing owner |
| --- | --- |
| terminal creation/control, exact targets, input uncertainty, and unsupported-program behavior, except the explicitly agreed launch changes | [terminal control](terminal-agent-control.md) and [terminal continuity](terminal-continuity.md) |
| notification causes, display copy, device-local acknowledgement/dismissal, foreground suppression, exact-session clicks, and current-state reconciliation | [native notifications](native-notifications.md), with [session latency](session-latency.md) amendments |
| service startup and preservation of accounts, histories, identities, and running tmux sessions during installation | [deployment contract](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/dev-server-handoff.md); move installation ownership to skid |
| signed/checksummed immutable releases, coordinated matching-version updates, and verification boundaries | [distribution](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/public-fleet-distribution.md) as amended by [testing policy](rules.md#testing) |

engineering should choose concrete implementation details that preserve these
contracts. ask the owner only when a necessary adaptation forces a material
behavior change or conflicts with an earlier explicit decision. hypothetical
features and already-specified behavior are not additional interview branches.

## agreed topology and ownership

| decision | consequence |
| --- | --- |
| one skid installation per machine, including its required internal services | users do not assemble separate skid-main, gateway, observer, and dev-server installations. separate processes remain appropriate where their lifetimes differ. |
| skid manages tmux on machines where skid is installed and enrolled | a local installation alone does not manage a bare remote tmux server. |
| every enrolled client can manage sessions across the enrolled fleet | installing skid on several hosts retains today's remote session management; it does not restrict a client to its own host. |
| a machine without skid is reached through ordinary manual ssh/mosh inside a managed terminal | skid controls that source terminal. it does not acquire remote session inventory, provider identity, agent status/attention/usage, or remote management authority. |
| each host owns its tmux runtime; clients connect directly to the target host | the hub is not a terminal proxy or execution coordinator. a hub outage must not stop existing terminal work or direct access using valid cached membership. |
| one hub owns shared configuration and notification coordination | the first standalone installation automatically makes itself the hub; no hub choice is required for one-machine use. creating the fleet on an always-on host improves notification availability; a sleeping hub cannot observe or publish fresh alerts. |
| one trusted user's fleet, with arbitrary machine and profile names | fixed macbook/devbox/arch names and a fixed four-profile table disappear. multi-user roles and tenant administration are outside the present design. |

the original possibility of managing hosts without skid has been superseded by
the installed-and-enrolled requirement. manual ssh/mosh remains ordinary terminal
use, including when the destination also happens to be enrolled.
manual ssh/mosh retains attachment, rendered reads, explicit terminal input,
generic interruption, and closure of the source tmux terminal. current
[observation](../internal/agentcontrol/terminal.go) marks that connection as
remote context rather than parsing it as a local provider. use the destination's
own enrolled skid instance for its agent observations and attention notifications.

## agreed installation and launch platforms

skid owns its standalone installer, runtime dependencies, service registration,
configuration application, pairing, and product upgrades. dev-server becomes a
consumer: install skid and supply the owner's configuration. it can continue to
install and configure the owner's provider tools independently.

agreed provisioning boundary: skid accepts the same yaml whether a user edits it
or an external provisioning tool writes it. ordinary skid installation and upgrades
preserve existing user settings. there is no dev-server-specific configuration
mode or synchronization policy in skid. for this owner's fleet, dev-server remains
the ongoing source of desired configuration; its provisioning behavior belongs
in dev-server. changes made only to its generated hub file can be replaced by a
later dev-server run, so lasting personal changes belong in dev-server's source.

| decision | consequence |
| --- | --- |
| install skid, tmux, and required runtime dependencies; install or reuse tailscale for remote/mobile access | reuse compatible existing installations. package-manager and privilege requirements must be explicit. tailscale login and network permissions remain user actions; exact dependency versions and update ownership are engineering details. |
| provider installation and login remain external prerequisites for those profiles | plain terminals work without codex or claude. a missing provider must not prevent the whole product from running. |
| macos arm64 and linux amd64 initially | intel macs and linux arm64 are deferred. the implementation plan records the current mac artifact floor and requires a qualified ubuntu floor before publication. |
| ubuntu and arch with systemd are sufficient linux targets | other distributions and service managers are outside launch support. |
| android 13 and newer | the owner explicitly waived dedicated older-android verification: patch discovered problems. this does not waive ordinary build/static checks or establish device qualification. |

exact install commands, filesystem paths, dependency detection, and privilege
prompts are implementation work. retain the existing systemd/launch-agent startup
behavior and coordinated update policy; changing the installation owner does
not by itself require new startup or update semantics. removal must distinguish
skid-owned installation files from retained user/provider/session data.

## agreed networking and enrollment

2026-10-21 owner decision (cleanup w2.1): the hub is fixed: the machine that
created the fleet or last ran `skid recover`. devices fetch membership only from
that hub; there is no relay discovery through other machines and no signed fleet
state, removal proof or invitation proof. a removed device simply gets 401: no
pending-revocation report, no automatic detach or forgetting. a removed computer
that gets 401 from the hub stops admitting the fleet's other devices. join, recover and a
failed state write restart the skid service (crash-only; tmux keeps running).

tailscale remains required for remote/mobile access.
2026-10-20 recap clarification: skid installs or reuses tailscale as part of
preparing a host for remote/mobile access. this supersedes the earlier decision
to leave tailscale installation external. signing in, approving required network
permissions, and administering the tailnet remain user responsibilities.
the phone also needs the tailscale app, signed into the same reachable tailnet;
the skid apk/qr does not replace network setup. see tailscale's
[android setup](https://tailscale.com/docs/install/android) and
[mac setup](https://tailscale.com/docs/install/mac).
local-only use remains independent of tailscale connectivity and internet access.
custom https endpoints are deferred in [their issue](issues/custom-https-endpoints.md).

join/remove commands manage membership. enrollment grants access to the fleet,
including machines added later; a newly enrolled machine appears automatically
on existing clients. sharing a tailnet alone does not enroll a device in skid.

membership, machine identities, credentials, and device registrations are generated
private state. users do not duplicate the membership roster in yaml. multiple
phones are supported, with independent pairing/removal and notification settings.
membership changes require the authoritative hub; exact invitation encoding and
expiry are implementation details.

the intended enrollment experience is one pairing entry point plus a join
command. command spelling below is proposed, not an implemented cli promise:

1. on an already enrolled machine, `skid connect` requests a short-lived,
   single-use invitation from the current hub. present it as a qr for a phone
   and as a pasteable invitation for another computer.
2. on a new computer, install skid and complete tailscale setup, then run
   `skid join` and paste that invitation. do not require a credential in shell
   command arguments.
3. the hub enrolls that installation. it adopts the fleet configuration and
   appears automatically on existing computers and paired phones. no new phone
   qr scans or manual yaml roster edits are required.

joining preserves the installation's machine identity and running tmux sessions;
it replaces the former standalone fleet trust/configuration as described below.
the invitation authorizes membership; tailscale reachability alone does not.
`skid connect` can be invoked on any enrolled machine by contacting the hub.
enrollment needs the hub available.

agreed outage policy: machines retain cached membership access without expiry
when they cannot reach the hub. otherwise reachable machines keep accepting
previously enrolled devices; a hub outage alone does not impose a remote-access
deadline.

revocation takes effect on each machine when it next fetches membership from the
hub. until then, a removed device may still access that machine. upon receiving
revocation, reject new access and disconnect that device's existing skid
connections. tmux sessions and provider processes keep running; revocation does not undo commands already accepted. this does not
promise instantaneous revocation across disconnected machines.

## agreed configuration and hub changes

2026-10-21 owner decision (cleanup w2.1): there is no planned hub migration and no
`hub:` yaml key. moving the hub is a new fleet: `skid recover` on the new hub,
`skid join` on each other computer and a fresh phone scan; observer and receiver
state are not carried.

there is one authoritative editable yaml file on the hub. it defines profiles and
shared settings. other machines retain generated
identity/connection state and a last accepted configuration snapshot, not another
human-maintained copy of the shared settings. any necessary host overrides should
live in that same logical configuration. an external provisioner may maintain
that file through the same configuration contract; skid remains the sole
application authority.

users edit the file; no tui configuration editor is needed. agreed application
trigger: opening skid on any enrolled computer asks the current hub service to
validate and apply pending changes to its yaml. hub-service startup also applies
the current file. the hub is the sole application authority; clients do not submit
their cached settings as authoritative configuration.

invalid configuration produces a useful error and leaves the last valid applied
configuration in force. launch changes affect subsequent launches, not running
provider processes. live reload beyond startup is not added by this scope.
hub acceptance does not mean every machine has applied a change. disconnected
machines retain their last valid snapshot; startup reports which changes are
applied, pending, or blocked by an offline machine. an unreachable hub leaves
the client using its last accepted configuration.

the startup rule concerns accepting changes to yaml. generated membership
must propagate to running clients so connected clients need neither restarting
nor re-pairing for enrollment.

there is no automatic election or recovery of an inaccessible hub, and no hub
setting to synchronize across configuration files. terminal execution remains
host-local.

agreed recovery without a usable backup: if the hub is permanently lost, the user
explicitly creates a new fleet identity using available cached settings from a
surviving machine. re-enroll surviving machines and re-pair each phone. existing
tmux sessions on surviving machines keep running; recovery costs another round
of enrollment and pairing. cached settings do not restore private pairing or
notification state.

re-enrollment replaces a machine's old fleet trust. a phone paired to the old
fleet refuses another fleet's invite, so re-pairing it starts by clearing the
app's data. recovery completes per device: offline devices remain attached
to the old fleet until explicitly re-enrolled. a returning old hub must not regain
authority over recovered devices. this is a new fleet, not promotion of a cached
member under the old identity.

agreed joining policy: a separately configured standalone installation joining an
existing fleet adopts the destination fleet's shared configuration. its own
configuration file stays in place, unused while it is a member; do not
automatically merge profiles or settings. the user copies any profiles they want to
retain. running tmux sessions continue. phones paired only to the former
standalone installation must be paired to the destination fleet.

joining that machine does not merge entire fleets or transfer other devices
implicitly. a built-in backup/export and private-state restoration feature is
not required for launch; recovery without a usable backup is already agreed.

## agreed profiles and shell behavior

fresh installations start with an empty profile list and commented examples in
the configuration file. plain shells work immediately; users edit the file to
define their own agent launch commands. skid does not automatically create
profiles by discovering installed provider executables.

profile names and commands are user-defined, for example `codex --yolo`,
`codex-work2`, or `claude-work`. the command runs on the selected machine with
ordinary semantics in its initialized interactive shell: normal aliases,
functions, environment, quoting, expansions, and command composition apply.
each machine must provide the commands its profiles use.

agreed: a profile may also declare its provider and account configuration
directory as optional observation metadata. for example, the command
`codex-work2` can identify codex and `~/.codex-work2` for account attribution.
this metadata describes the account to observe; it does not rewrite the command
or change the launched provider's environment. an absent or ambiguous mapping
leaves profile usage unknown. custom accounts may require this small amount of
configuration; skid does not reverse-engineer aliases or shell functions.

skid must not replace provider aliases, select a different account, add permission
bypasses, or silently append remote-mode/model/effort arguments. the configured
command and the provider's own settings determine execution policy. the generic
startup responsibility remains: initialize the shell, enter the requested working
directory, and evaluate the command. ordinary returning commands leave the shell
available; explicit shell operations such as `exec` or `exit` retain their meaning.

agreed launch controls: run the configured command exactly. remove separate
model/effort overrides from creation in the cli, api, and user interfaces; those
choices belong in the configured command or provider settings. do not append
arguments to, parse, or rewrite arbitrary shell text to implement overrides.
callers currently supplying those options need a coordinated contract change;
do not silently accept and ignore them.

agreed shell support: bash and zsh are the supported initial shells for launch.
fish and other shell startup integrations are outside the first release. the
exact yaml schema is defined in the [implementation contract](generalization-plan.md#2-configuration-and-durable-state).

## creation flow and shell selection

retain the existing creation controls and both creation choices, with plain shell
as the default. the earlier agreement to remove directory selection is a
[separate product change](issues/session-creation-directory-picker.md), not part
of generalization. current quick creation already starts a shell at home.

| choice | behavior |
| --- | --- |
| plain shell | retain the creation path's working-directory behavior; quick creation starts at home, subject to normal shell startup. the user can change directory and run a command inside the terminal. |
| profile | immediately execute the selected profile's exact command in the initialized shell. the command may choose its own directory, for example `cd ~/project && codex-work`. |

profile launch does not wait for the user to choose a directory inside the shell.
a command without its own directory change runs in the shell's initial working
directory, unless the existing creation form, explicit cli/api cwd, or
new-terminal-here action sets it.

the recommended shell setting is optional: use the destination machine's login
shell by default, with an explicit override in the shared yaml. machine-specific
overrides belong in that same file when executable paths or preferences differ;
this does not introduce separately edited host configuration files. resolve and
validate the shell on the destination machine. if the selected or default shell
is unsupported, require an explicit bash/zsh selection for skid rather than
silently substituting another shell or changing the user's system login shell.
both plain-shell and profile creation use the supported startup integration;
commands subsequently run inside the terminal retain ordinary shell semantics.

retain optional cli/api working-directory input for automation and the existing
new-terminal-here action. those are separate capabilities from the creation form;
jarvis currently supplies an optional working directory. current-directory display
and workspace recovery also remain independent of removing the picker.

## agreed usage presentation

each logical profile denotes the same provider account on every machine. show
one usage entry per profile, selecting one eligible fresh observation from the
fleet. do not sum account quotas or show one entry per machine/profile pair.
different accounts behind the same profile would reopen this assumption.

missing or unsupported observations are unknown. a terminal's original launch
profile does not prove which account is running after manual commands, an account
switch, or process replacement. account attribution and the acquisition changes
below still need a precise contract.

## agreed desktop/mobile notifications

mac notifications remain on their existing path: the hub's authenticated
revision stream and current snapshot feed the native mac app. each mac retains
its own acknowledgement and presentation state. the standalone installer must
include the existing native app/service and its normal permission setup.
android's transport replacement does not replace the mac adapter.

agreed mac notification-click terminals: support apple terminal and ghostty
initially, selected in configuration. use the built-in apple terminal by default
for new installations and preserve existing installations' ghostty selection.
new installations do not require another terminal application. ordinary cli/tui
use remains in the user's chosen terminal independently of the notification-click
selection. the cost is maintaining and qualifying a second launch/focus
integration; mac automation permission remains.

the current [mac launch](../internal/macnotifications/app_darwin.go) and
[foreground detection](../internal/macnotifications/native_darwin.m) use ghostty.
the added apple terminal integration must preserve exact session targeting,
acknowledgement after first actual output, and proof that the exact session is
foreground before suppressing later alerts. exact attachment and first-output
acknowledgement already work independently of the terminal app through the
[entry visit](../internal/terminalclient/visit.go); launch and focus need each
app's interface.

[apple terminal is scriptable](https://support.apple.com/en-lb/guide/terminal/trml1003/mac).
read-only inspection of the installed `Terminal.sdef` found command execution
returning a tab, tab tty identity, selected tab, and application/window focus
properties. these support the design; they do not establish live qualification.
[ghostty](https://ghostty.org/docs/features/applescript) supplies the existing
integration. other terminals and generic custom launchers are outside the initial
supported set. a launch-only path would leave ongoing focus unknown and could
alert while the user is viewing the session. terminal focus escape sequences
would also require new handling; the current input path forwards them without
using them as notification evidence.

one-machine mac use must work with that mac as hub, without a phone, another
host, firebase configuration, or tailscale connectivity. this requires removing
the observer's current [three-machine and ntfy startup coupling](../internal/agentcli/notifier.go)
and supporting local access to the observer. this is a necessary generalization
adaptation; the notification behavior remains the existing contract.

an unavailable/sleeping hub pauses new qualified notifications while existing
terminal work and direct access continue. an always-on hub also cannot observe
a source machine that is asleep or unreachable. mac delivery reconciles current
state after wake under the existing rules.

after satisfying the networking prerequisite, install the standard skid apk,
run `skid connect` and scan the qr. pairing and desktop/mobile session management
require NO firebase setup. android background notifications are optional.

2026-10-20 direction change: there is NO project-operated notification relay.
users who opt into android push own their firebase project, sending credentials
and their maintenance. the existing hub sends content-free fcm messages directly
over outbound https. no additional server, public endpoint, custom domain, dns,
caddy or notification app is required. the proposed
`skid.nielseriknandal.com` deployment and its pending dns/security-classification
requests are cancelled. previously created cloud resources are inventoried for
separate owner-approved cleanup, never deleted as an incidental source edit.

one distributed apk initializes firebase at runtime using public configuration
received automatically through the hub's fleet state after pairing. no project
identity or private sending credential is compiled into it. four public firebase
fields live in optional hub yaml; the private service-account export is a local
`0600` hub file, outside shared yaml, fleet snapshots and phone payloads.
[the setup guide](android-push-setup.md) owns the opt-in procedure.

omitting android push configuration is healthy disabled operation: no firebase
initialization/registration, repeating setup error or blocked session management.
mac notifications and foreground hub reads remain independent of firebase.
configured push with missing credentials, denied permission or transport failure
reports its own actionable status; it cannot fail terminal APIs or mac delivery.

material sdk limitation, accepted by the owner: firebase options stay immutable
within an android process. a project/options change immediately fences old push,
persists the new desired configuration, and requires one normal app restart.
initial pairing can initialize an absent firebase app; same-options re-pairing
reuses it with a new fleet/device registration. no hot delete/reinitialize,
automatic process killing, separate push process or custom apk. ordinary process
death must restore runtime options before a cold messaging service handles work.
android force-stop requires reopening the app; pre-first-unlock delivery is not
promised. both differ from ordinary process death.

before broad production rewrites, prove runtime initialization and actual fcm
registration/background delivery using one unchanged apk, including two actual
projects, process death, re-pairing and stale-registration fencing. a build or
fcm validate-only response does not pass that gate. surface a failed boundary;
never restore a relay or require project-specific apks.

agreed content: names and reason, without terminal excerpts. under the owner's
preserve-existing-behavior clarification, this retains the current
[content contract](native-notifications.md#content-and-platform-contract):
session name plus the existing closed reason phrases. do not introduce machine,
profile, directory, question, or transcript text. the earlier machine/session
wording does not require adding labels absent from the existing product.

retain the existing notification behavior: device-local acknowledgement and
dismissal, exact foreground suppression and click targeting, one notice per
session lifetime, and only unseen, still-current attention after reconnect.
multiple phones require separate registrations, not a new shared acknowledgement
policy. keep the current android adapter and platform/channel settings; no new
lock-screen privacy setting is part of this work.

2026-10-20 cutover decision: one fresh `skid connect` qr scan per phone is accepted
to replace today's shared gateway credentials with independent device credentials.
preserve tmux/provider data, observer epoch/attention and valid device-local
acknowledgements; no app-data reset or automatic old-credential conversion.
the owner also approved a temporary conversion build using the same android
signing identity to preserve notification memory before the final apk replaces
it. temporary host tooling does the equivalent for observer state. this adds one
staged phone update during cutover; the final product retains no legacy reader.

adapt fcm at the existing boundary: a content-free revision hint prompts an
authenticated current-state fetch, then the existing device owner decides
presentation. session names and reasons arrive over the private connection.
the proposed encrypted summary displayed directly from a push is withdrawn:
it was not agreed and would require changing current snapshot-admission
semantics. do not add content-encryption key machinery merely for that proposal.

retain per-phone registration rotation, possession proof, revocation, hub
fleet routing, notification admission and device-local acknowledgement. bind
registration, challenge and hints to the accepted public firebase configuration
and receiver generation. changing/clearing configuration invalidates old
registrations without resetting attention or acknowledgement memory. google
observes delivery metadata; notification details come only from authenticated
hub reads over tailscale. no terminal or notification content goes through fcm.

moving the hub carries no observer/receiver state and NEVER firebase private
credentials. for configured push, place a matching private credential file on the
new hub before running `skid recover` there. credential rotation/reload uses existing startup/apply triggers,
not another configuration service.

android callbacks schedule bounded authenticated fetches under the existing
admission owner. retain locked-phone/outage qualification. the official
[delivery guidance](https://firebase.google.com/docs/cloud-messaging/android-message-priority)
does not guarantee delivery under force-stop or unreachable tailscale.

## native reads and quota with terminal control

agreed, clarified 2026-10-20: retain working codex native capabilities and the
existing separate terminal/native read apis. ordinary reads return rendered
terminal text; explicit native reads retain provider-history semantics and
report unavailable when that capability is absent. do not substitute either
result for the other.

automatic native-first selection with a terminal fallback applies to account
quota only. this narrows the earlier "reads/usage" wording; a generic read api or
automatic transcript selection is outside scope and would require a separate
product pr. ordinary prompt submission and interruption stay on tmux; there is
no automatic native write or transport fallback after dispatch. use
already-running daemons only. ordinary shell-command semantics remain agreed;
do not rewrite commands to attach them to a daemon.
2026-10-20 reconfirmation: retain these two deliberate quota sources under the
hard-cut instruction. "no fallbacks" removes legacy compatibility paths; it does
not remove this agreed acquisition policy.

skid already supports terminal list/create, attach, rename/group, rendered reads,
guarded send, deliberate text/keys, interruption, wait, and exact closure. retain
that public contract independently of native capabilities. a configured daemon
must not become a prerequisite for ordinary terminal operation.

users configure provider statuslines; skid supplies concise instructions and a
quiet hint when no supported display is observable. do not overwrite user provider
settings. absence is not proof of misconfiguration: a footer may be hidden,
clipped, stale, or unsupported. unsupported displays show unknown values without
preventing terminal use.

### agreed source and control boundaries

| capability | current source | generalization behavior |
| --- | --- | --- |
| codex quota | [native account read](profile-usage.md), independent of terminal/conversation identity | prefer an eligible fresh native report for the configured profile; otherwise select one attributable terminal report. add the missing codex quota parser. do not let stale native data suppress usable fresh terminal evidence or sum duplicate account observations. |
| displayed model and effort | [terminal footer](terminal-model-effort.md) | retain terminal extraction. current native inspection does not provide these fields; adding a different native meaning is new work, not a benefit obtained by merely retaining the daemon. |
| terminal status and notifications | recognized foreground process and captured screen | retain terminal evidence. a saved native conversation may differ from what the terminal currently displays. |
| ordinary rendered read | [terminal read](terminal-agent-control.md) | preserve the existing screen/scrollback contract and terminal source. no native-history substitution or new generic read facade. |
| native history and results | [captured native conversation](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/native-agent-observation.md) | retain exact-target semantics. a terminal screen cannot satisfy a request for saved conversation history or finalized native output. report unavailable when the requested native capability is absent. |
| ordinary send and stop | terminal input; separate explicit native operations also exist today | agreed: ordinary prompt submission and interruption use tmux. automatic native preference never applies to writes. retain exact-target guards, terminal delivery receipts, and no replay after uncertainty. |

quota is scoped to an account, so it does not need a terminal-to-conversation
association. native conversation control does. current codex creation leaves new
terminals unassociated; existing saved bindings can differ from a terminal's
current view. a reachable account daemon is not proof that it owns the conversation
displayed in a selected pane. do not infer that identity from cwd, timing, or the
newest transcript. preserve existing association and target contracts; no new
association writer is required for generalization. explicit native history
requests retain their declared semantics and cannot substitute terminal text.
existing explicit native mutation endpoints also remain; any unrelated public
api removal requires a separate product pr.

agreed daemon lifecycle: use a compatible already-running account daemon when
available. the provider/user owns starting, restarting, upgrading, and stopping
it. skid does not perform those actions as part of installation, observation, or
implicit launch setup, and does not inject remote-mode arguments into profile
commands. an explicit user-configured shell command retains its ordinary effects.
an absent or incompatible daemon leaves native capabilities unavailable; ordinary
terminal operation and supported terminal observations remain usable. this avoids
daemon lifecycle ownership but leaves native availability dependent on external
setup. daemon presence still does not supply a terminal's conversation identity.

retaining native integration preserves its helper/runtime and provider-protocol
maintenance costs. supporting both quota sources adds selection/freshness and
failure-path work. this can simplify setup for users without simplifying the
implementation. native unavailability should be bounded and visible in diagnostics,
not turn ordinary terminal access into a wait for a broken helper.

### statuslines and account attribution

[upstream codex statusline documentation](https://learn.chatgpt.com/docs/developer-commands#configure-footer-items-with-statusline)
offers rate-limit items. skid still needs a parser qualified against the actual
labels, windows, percentage semantics, and clipping behavior.
[claude's statusline contract](https://code.claude.com/docs/en/statusline#rate-limit-usage)
provides quota fields only for supported account arrangements and after an api
response. skid currently parses a particular configured suffix; document a
supported recipe instead of depending on dev-server to install it.

terminal quota requires an observable provider display. displayed values can be
rounded or lagging; a fresh capture does not establish a fresh provider report.
missing fields and ambiguous account attribution remain unknown. native quota can
retain account-level availability when no terminal is displaying that profile.
neither source should invent unsupported windows or reset times.

codex currently maps foreground account-home environment to configured profiles;
claude uses its identity hook. if launch-time hook injection is removed, replace
claude attribution through ordinary process observation using the agreed profile
metadata above. its schema, matching details, and default-home rules are
specified in the [implementation contract](generalization-plan.md#5-launch-observation-and-quota).
the original launch label cannot prove the account after manual
process replacement.

`start --stdin` checks observed profile and readiness before one terminal
submission. preserve or deliberately revise that guard when changing launch or
attribution; removing the hook without replacement can make the guard impossible
to satisfy. generic shell startup and lifetime checks remain necessary regardless
of the native integration choice.

### jarvis compatibility

jarvis uses skid's cli/json api, not its daemon implementation. its existing
terminal workflows need no changes if skid preserves the terminal launch/read/
send/wait/stop/close contracts. optional native metadata may remain absent.
removing unused native branches in jarvis would be cleanup, not a prerequisite.
arbitrary machine/profile support in skid also does not require changing jarvis's
personal fleet enums unless its configured targets change.

the existing public api does expose native and terminal operations separately:
`t-` handles select terminal operations; `c-` handles select native conversations.
jarvis validates the distinction. for example, its read adapter rejects a
terminal-source result for a native target, and native sends request peer input
with native admission receipts. silently returning terminal bytes for that native
request would change the public contract.

retaining both existing contracts needs no jarvis change merely because native
availability becomes optional. even complete native removal could leave ordinary
terminal workflows intact while returning the existing structured unavailable
error for native requests; that preserves error handling, not native capability.
removing native vocabulary, changing result meanings, or making existing reads
transparently choose either source would require coordinated consumer changes.
the now-agreed removal of model/effort launch overrides is a separate public
contract change: update jarvis's exposed launch options and adapter when that
cutover is implemented. daemon availability alone requires no such change.
never remap a saved conversation reference to whichever terminal now looks related.

the accepted [jarvis worker requirements](jarvis-orchestration.md#owner-decisions)
allow ordinary conversation and bounded reads. terminal idle does not certify
task completion, terminal input receipts prove delivery rather than provider
acceptance, and terminal close does not prove detached work stopped. jarvis already
accepts those limits for terminal targets. its independent cognition backend is
outside this design discussion.

## remaining generalization work

the previous interview table reopened established behavior and mixed owner
decisions with implementation details. the owner's scope clarification narrows
the remaining work:

| item | disposition and material consequence |
| --- | --- |
| account quota | implement the agreed native-first/terminal-fallback acquisition while preserving existing explicit terminal/native read apis. this needs attributable terminal quota evidence, not a new transcript api or association mechanism. |
| profile commands and attribution | ordinary shell semantics already determine missing-command behavior; existing terminal controls already work for unsupported programs. implementation must remove injected launch policy and replace account-attribution dependencies where necessary. do not add command parsing/preflight that rejects valid aliases, functions, or compositions. |
| standalone installation and configuration | implement one installer, the shared yaml and its generated snapshots, and dev-server consumption. retain startup, provider/session preservation, and matching-version updates. choose schema, paths, dependency handling, and compatible os minima as engineering details; surface an unavoidable new limitation. |
| enrollment and hub changes | implement the agreed membership, cached-access/revocation, joining and new-fleet recovery rules. invitation encoding and pending-status presentation are engineering details. no automatic election, automatic config merging, or built-in backup product is required. |
| notification transport | replace ntfy with optional user-configured direct hub→fcm delivery, one runtime-configured apk and independent phones. preserve content, currentness, acknowledgement and presentation. runtime firebase proof precedes production changes. |
| migration, removal, and documentation | translate the existing installed layout into the new ownership/configuration without deleting provider data or active tmux work; document one local install, fleet enrollment, phone pairing, and recovery path. preserve existing verification policy. these are delivery requirements, not new preference questions. |

### resolved clarifications from the 2026-10-20 audit

the renewed source audit identified four narrower ambiguities. configuration
provisioning is now resolved above: personal continuous management belongs in
dev-server, using skid's ordinary configuration contract. application is also
resolved: opening skid on any enrolled computer asks the hub to apply pending
edits. mac notification clicks will support apple terminal and ghostty as agreed
above. an offline phone whose known fleet addresses have all been retired requires
another `skid connect` qr scan. all four preference questions from this audit are
resolved; preserve the existing behavior elsewhere.

other findings are engineering obligations under existing decisions: update
generated membership/routing while
clients run, remove fixed-roster admission, remove provider installation as an
installer prerequisite, and complete optional push registration after pairing only when configured. successful
pairing or snapshot access does not prove background delivery. no further owner
preference question is currently identified. the [implementation contract](generalization-plan.md)
records the concrete design; implementation and qualification remain. surface a
question if they expose a necessary behavior
change or an unresolved conflict. current implementation authorization remains in force.

this document remains the decision record. actual defects and deferred
implementation follow-ups belong in individual `docs/issues/` records; the
custom-https issue is already linked above.

## verification boundary

follow the existing [testing policy](rules.md#testing); this discussion does not
reinstate retired harnesses. implementation needs focused evidence at meaningful
boundaries: ordinary shell semantics, exact terminal targeting and uncertain
input, supported provider frames and quota attribution, clean installation and
enrollment, new-fleet recovery, and phone delivery across sleep/outage.
size the checks to the failure's consequences and keep source/build evidence
separate from installed-host and physical-device behavior.

the [qualification record](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/generalization-qualification.md) separates completed
source/probes from this direction's new runtime proof and deferred installation.
the android13 compatibility waiver and two-phone deferral remain explicit.
