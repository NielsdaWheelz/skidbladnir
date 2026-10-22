# native needs-input notifications

this owns background observation, the owner's acknowledgement and native alerts.
[terminal observation](terminal-observation.md) owns detection;
[terminal attention](reply-notifications.md) supplies the transition predicates;
[session views](session-views.md) owns the surrounding queue;
[generalization §6](generalization-plan.md#6-optional-direct-android-push) owns
optional firebase configuration; [architecture](architecture.md) owns scope.
open device checks are in [unrun acceptance](issues/unrun-acceptance.md).

## behaviour

- every new qualifying attention episode alerts the mac app and the phone: ready,
  a response request, or a current error/interruption. request and notice
  coexist; changing either alerts again. unchanged polls and renames stay silent.
- all alerts use identical prominence and platform-default sound, and respect
  focus/do-not-disturb and user notification settings.
- one native notice occupies each session lifetime. replace it for a new reason;
  remove it on fresh resolution or identity replacement. uncertainty preserves it.
- acknowledgement is the owner's and the hub holds it: dismissing, clicking or
  visiting a session on any device clears its notice on every device. only a
  visit's first output acknowledges ready. a watched session alerts like any
  other; there is no foreground suppression.
- requests remain in the queue until fresh terminal evidence clears them.
- a click opens the captured session lifetime: android's terminal page or a new
  ghostty or apple terminal window. vanished, replaced or unreachable targets
  produce an explicit result, never a namesake.
- reconnect alerts only unhandled, still-current attention: no replay or reminders.

android delivers with skid backgrounded or reclaimed and the phone locked;
force-stop, revoked permission or stopped tailscale suspend it
([stopped state](https://developer.android.com/about/versions/15/behavior-changes-all#stopped-state)).
mac delivery resumes after wake. linux has no os popups; its tuis show the queue.

## composition

```text
gateways (unchanged inventories, 5 s per machine)
  -> hub notifier: transitions -> records{…, handledEpisode, ackedReadyGeneration}
     -> projection -> state{machines[{machine, availability, sessions[rows]}]}
        -> GET /v1/notifications/state     (every client, one read)
        -> sse "data: changed" on change    (mac app)
        -> fcm {kind:hint} on change        (phone, background)
  <- POST /v1/notifications/handled {slot, episode, readyGeneration}
     (dismiss, click, first output of a visit; any device)
clients: reconcile(state, os-delivered, alerted, permission) -> ordered os effects
```

observation performs no terminal attachment, mutation, native-history read or
gateway proxying; direct inventories still own cards, controls and fresh
request/notice membership. only the hub advances readiness, episodes and
acknowledgement. a client keeps one in-memory `alerted: slot -> episode` map per
process and no other notification memory. hub failure pauses ready and new
alerts; direct requests/errors and session access continue.

## identity and wire

identity is never a name, title, cwd or native conversation.

```text
slot = {machine, tmuxId, identityToken}           // session lifetime; native identifier
key = {machine, tmuxId, identityToken, paneId}    // fleetclient.Reference; ref encodes it
foreground = {provider, pid, startIdentity}       // exact agent process lifetime
cause = {kind:"ready"} |
        {kind:"action", request?: permission|question|setup|confirmation|input,
                        notice?: interrupted|error}
record = {key, foreground?, phase: quiet|armed|ready, readyGeneration,
          attentionEpisode, cause?, handledEpisode, ackedReadyGeneration}
state = {machines:[{machine, availability: fresh|gap,
                    sessions?:[{slot, ref, title?, body?, episode?,
                                delivery: post|keep|clear, readyGeneration?, foreground?}]}],
         receiver?: {status}}                     // phone callers only
handled = {slot, episode, readyGeneration}        // zero = not acknowledged
```

`slot` and `ref` are base64url json. action requires at least one field; ready
excludes action fields; records hold no terminal or question text. one hub
revision advances per fresh machine commit; episodes and ready generations take
that commit's revision, never a timestamp, and the revision is never served.
`title`, `body` and `episode` are absent only for `clear`; `readyGeneration` and
`foreground` are present only while ready is pending.

retain one record per live session lifetime. positive pane/process replacement
resets its phase and acknowledgements; an earlier pane cannot reuse an old
episode. complete inventory retires absent lifetimes with their
acknowledgements; failed hosts and unproven foreground absence do not; removed
machines' records stay, so a re-enrolled computer resumes them.
`<state>/skidbladnir/notification-observer-v2.json` (0600, atomic replace; the
name is the version) holds `{fleetId, configuration?, revision, records,
receivers}`. a missing or other-fleet file starts empty; a new fleet deletes it;
any other read or decode failure leaves the observer unavailable until restart.
its decoder checks only the closed sets the projection switches on. clients decode state
tolerantly and re-check nothing (an unknown `delivery` changes nothing on the
mac; android's enum decoding fails the read); `handled` and receiver
bodies are strict json at 4 kib.

## observation and transitions

reuse the existing predicates: requests, menus, starting and working are
non-idle; idle requires `activity=idle, interaction=none`; everything else is
a gap. notices do not arm readiness themselves.

| admitted evidence | producer result |
| --- | --- |
| first idle | quiet; no ready |
| non-idle | armed; supersede previous ready |
| armed then idle, same key/foreground | ready; allocate ready generation |
| repeated idle while ready | retain generation |
| unknown, unavailable, outage | retain continuity; no inferred resolution |
| proven replacement/exit | reset former identity; classify new identity |

a fresh local positive request/notice yields an action cause, even if another
dimension is unknown. first retire proven obsolete attention: known non-idle
supersedes old ready even with unknown interaction; interaction `none|menu`
and no notice clears old action even with unknown activity. then select positive
action, else qualified idle with phase ready, else known clear or gap retention.
clearing an error over latent ready selects ready in the SAME commit.
working with unknown interaction can arm readiness but cannot clear old action.
positive action can supersede uncertain previous
attention; never invent missing request content or completion.

allocate an episode when a qualifying cause first appears, changes structurally,
or belongs to a new exact key/foreground. a known clear permits a later same-kind
request to become new; an unobserved reset cannot. action outranks latent ready.
clearing a notice to reveal pending ready changes the visible reason and alerts;
already acknowledged ready does not. diagnostic reason/copy and name changes
never allocate an episode.

background observation is required while clients are closed. run one
independent, coalesced five-second inventory lane per machine, each with a
five-second request deadline and existing two-second detector enrichment. use
a narrow raw per-machine inventory read; ordinary fleet `list` waits its slowest
peer and resolves remote contexts unnecessarily. no overlapping same-host reads.
serialize commits; cancel lanes with the service. a sleeping mac cannot delay
other hosts. this is one additional fleet sampler, not provider-specific polling.

persist a commit before publishing it; the file is written only when a machine's
records change, and every value a client holds was saved under a revision at
least as large, so a restart allocates above it. publish iff the served
projection changes; no outbox or historical resend. startup marks every host gap
before serving; retained facts qualify only after fresh reads. corrupt/unwritable
state makes the service unavailable; never manufacture an empty state.

## projection and acknowledgement

for each session of a fresh machine the hub derives one row:

- ready is pending when the evidence is qualified, the foreground known, the
  source the terminal, the phase ready, the status idle and `readyGeneration >
  ackedReadyGeneration`; the row then carries `readyGeneration` and `foreground`.
- `clear` when there is no cause, the episode is handled (`attentionEpisode =
  handledEpisode`), or the cause is ready and its generation acknowledged. cleared
  rows carry no copy, so renaming them publishes nothing.
- otherwise the row carries the session name, cause copy and episode: `post` when
  the qualified terminal status still matches the cause and, for ready, ready is
  pending; else `keep`, which never posts and never removes.

`POST handled` acknowledges. each nonzero field applies only while it equals the
record's current value, else it is a no-op (stale, superseded or another file's
number); every well-formed body answers 204. a change persists, then publishes.
dismiss and click send `{slot, episode, 0}`; a visit sends the episode and ready
generation captured at entry. acknowledging a gap machine's session shows when
that machine is next fresh.

## client reconciliation

each client applies one pure reconcile to the state, the os's delivered notices,
`alerted` and whether posting is allowed, then applies effects in order:

1. per row of each fresh machine, with `copy` the delivered notice of its slot:
   - `clear`: cancel if a copy exists or `alerted` has the slot (a post the os has
     not surfaced yet).
   - `keep`: same-episode copy with different title/body → silent post; nothing
     else.
   - `post`: same-episode copy → silent post iff title/body differ; else
     `alerted[slot] = episode` → nothing; else allowed → audible post replacing
     any copy; else cancel the older copy. blocked attention posts once allowed.
2. cancel each delivered notice whose slot has no row unless its machine is a gap:
   removed machines and vanished lifetimes.

a failed state, os or permission read applies nothing. a successful cancel drops
the slot from `alerted`; an audible post, a dismiss and a click set it, so this
process never re-posts what the owner has seen, even when the ack fails. each
sync reads the os's notices before the hub's state, so a notice dismissed during
the read is still a same-episode copy and never posts again. `alerted` is never
persisted.

the mac app syncs from one sequential loop on stream hints, native ready,
permission, wake, setup, dismiss and click events; a hint during a fetch discards
that result. the stream reconnects with 1-to-30-second backoff, reset only after
a stream of at least 30 seconds. pending and delivered notices both count.

android's application owner runs one conflated sync at a time, after fcm hints,
resume, the foreground's five-second inventory cadence and fleet changes. tag =
slot, id 0, channel `needs-input`, `autoCancel=false`; `getActiveNotifications`
is the delivered set; silent posts set `setSilent` and `setOnlyAlertOnce`.
explicit immutable click and delete intents carry slot, episode and ref; the
dismiss receiver posts `handled` under `goAsync`.

tuis read state once per refresh. `ready` shows only with a good read, a pending
row for the session's ref, a direct idle row with no interaction, and the same
foreground: direct inventory qualifies readiness, never creates it. a failed read
shows the footer `notifications unavailable` and hides ready.

visits (`skid enter`, tui entry, the phone's terminal page) read state for the
entered ref concurrently with dialing, never delaying output, and after the first
nonempty output post that row's slot, episode and ready generation once. a detach
before output posts nothing; a replaced pane or agent makes the capture a no-op.
failure stays secondary: `skid enter` prints `notifications unavailable` with its
exit status unchanged; the tui sets its footer.

## api, auth and transport

the hub serves `/v1/notifications` behind gateway admission (bearer,
`Skidbladnir-Machine`, no raw path/query, 64 kib body), then re-authenticates the
bearer to learn the member's kind ([member auth](issues/notification-member-auth.md)). a member gateway
answers 503 `NotificationsUnavailable` "notification hub unavailable; check skid
status and service logs".

| endpoint | contract |
| --- | --- |
| `GET /v1/notifications/state` | no body; current `state`; `receiver` for phones only |
| `GET /v1/notifications/events` | `text/event-stream`: `data: changed` at connect and on each projection change; `: keepalive` every 60 s; no ids or replay |
| `POST /v1/notifications/handled` | strict `application/json` `handled`; 204 for any well-formed body |
| `PUT /v1/notifications/receivers/me` | phone only; strict `{configuration, generation, fid}`; 204 or 409 |

responses are `Cache-Control: no-store`; errors are `{code,message}`: 401
`Unauthenticated` (member removed mid-request), 400 `InvalidRequest`, 413
`RequestTooLarge`, 403 `Forbidden` "phone membership required", 409
`ReceiverConflict`, and 503 `NotificationsUnavailable` "notification state
unavailable; repair its state and restart the skid service" after a failed start
or save.

## android push

fcm messages are data-only `{kind:"hint"}`, restricted to `dev.niels.skidbladnir`,
four-week ttl, collapse key = urgency, sent on projection change, at hub start,
on a push reconfiguration and after each phone registration (which tries the new
destination at once); never periodically. urgency is
`attention` (high) iff a new episode was allocated since the last send, else
`sync` (normal). no notification payload, text, terminal data or fleet/device
tuple. the phone ignores other messages, checks the sender against its current
options, and schedules one bounded refresh (expedited when high, three retries);
`onDeletedMessages` does too. a hint is a wake; only the authenticated read presents.

each phone has one receiver `{deviceId, floor, fid?, failed}`. a `PUT` for a
non-current configuration or with push off is 409; `generation > floor` activates
`{floor = generation, fid}` and wakes the sender; the identical active tuple is
204; anything else is 409. a configuration change erases destinations and keeps
floors; removed or non-phone members' receivers are dropped. fcm 410/UNREGISTERED
clears the destination; another failure marks it failed until the next change
retries. status: `disabled` (push off), `unconfigured` (no credential),
`unavailable` (no destination or failed) or `active`.

one unique network-constrained `notification-enrollment` job (KEEP at startup,
REPLACE on fleet change) registers through the sdk when push is ready and
notifications are allowed, then puts `{configuration, generation, fid}`; a
conflict renews the local generation and retries. `push-v3.json` holds the floor,
configuration, target and registration.

phone health adds `background notifications are disabled on the hub.`, `stop and
reopen skid to activate changed notification settings.` and `configure android
push credentials on the hub.` to the shared copy below; a failed read, failed push
or inactive receiver says delivery unavailable. the
[compact dashboard](dashboard-compact-chrome.md) notice says only `set up
notifications`; the setup dialog keeps `open notification settings`.

## mac app

one background `Skid.app` ships in the release: bundle id `dev.niels.skidbladnir`,
existing icon, notification delegate and automation description. go owns state;
a narrow objective-c/cgo bridge owns appkit, usernotifications and apple events,
with the native loop on the main os thread. notices carry slot, machine, ref and
episode. the installer registers the app (`skid-notifications register APP`)
before login activation; the `dev.niels.skidbladnir.notifications` agent
supervises a quiet `/usr/bin/open -g -W -a` with explicit xdg environment and
`--config STATE --skid SKID`. replacement stops the waiter, then
`skid-notifications stop APP` ends the exact registered instances.

`<state>/skidbladnir/notifications.sock` (0600; one instance through
`notifications.sock.lock`) accepts one line, `setup`, answering `ok` or
`unavailable` within three seconds. `skid notifications setup` opens the setup
window through it and never launches another app; linux exits 2. the window shows
health and two buttons: `allow notifications`, `open notification settings`.

a click runs the shell-quoted `skid --config STATE enter --ref REF` per
`settings.terminal`: ghostty opens a new window (`wait after command`) and
activates it; apple terminal runs `do script`, then `activate`. failure shows
`could not open ghostty. open the session from skid.` or `could not open
terminal. open the session from skid.` in a native dialog.

health copy, shared with the phone: `allow notifications for needs input.`,
`notifications are blocked in system settings.`, `notification setup required.`
or `notification delivery unavailable.`. no health sounds or settings dashboard.

## content and platform contract

the content designer owns this closed mapping and setup/click copy before code.
good content identifies the session and observed reason without claiming success,
transcribing provider text or exposing transport in normal alerts.

| cause | body |
| --- | --- |
| ready | `ready` |
| permission | `needs permission` |
| question | `needs answer` |
| setup | `needs setup` |
| confirmation | `needs review` |
| input | `needs input` |
| interrupted | `interruption shown` |
| error | `error shown` |
| request + notice | `<request phrase> · <notice phrase>` |

app name/icon are skid's existing identity; title is the current session name.
no machine, directory, profile, objective, question or transcript. preserve
unicode/case; the hub replaces controls/format/line separators with
`attention.SingleLine`. native layout owns wrapping/ellipsis. duplicate names
remain visually ambiguous; hidden exact identity still routes correctly. menu
alone is excluded; menu + error says error.

android has one `needs-input` channel, label `needs input`, importance high,
default sound/vibration and no dnd bypass/full-screen intent; native policy may
group/throttle alerts ([channels](https://developer.android.com/develop/ui/views/notifications/channels)).
request notification permission through native setup.

## trade-offs and exclusions

the hub is a readiness dependency without failover; its sampler adds bounded
gateway work; five-second inference misses brief resets and proves no completion;
os policy prevents a delivery deadline; the mac app stops briefly when replaced.

- T1 a notice removed without a callback (android reboot) or dismissed with a failed ack re-alerts once at the next process start; the mac rarely hits it.
- T2 a watched session banners on every device; ready during an open visit is not auto-acknowledged and alerts until the next entry or the session goes non-idle.
- T3 a visit clears that session's action notices everywhere; the request stays in the queue.
- T4 offline phones converge through fcm collapse, the four-week ttl and `onDeletedMessages`; a failed send retries only at the next change. hub start, push reconfiguration and phone registration each send one sync hint: one extra phone read each.
- T5 a wrong fid reads `unavailable` after the first failed send; a phone that never enrolled reads `unavailable` at the hub but shows setup required.
- T6 acknowledgements are the hub's; a new fleet or the cutover starts with none.
- T7 acknowledging a gap machine's session clears other devices' copies only when the hub next reads that machine.
- T8 cross-device clears reach a dozing phone at normal fcm priority and may lag.
- T9 no revision reaches clients; the mac fetches on every hint and connect, and a hint during a fetch costs at most one more fetch.
- T10 the foreground phone sees notification changes on its five-second inventory cadence, not a stream.
- T11 each entry adds one state read beside dialing; exit may wait up to three seconds for it.
- T12 clients decode state tolerantly and trust the same-release hub: a skewed hub's unknown verdict is inert on the mac and reads as unavailable on android. the hub file checks only closed sets, so a hand-edited record yields odd verdicts, not a 503 (repair: delete the file).
- T13 a stale fcm hint wakes one harmless authenticated read; a local storage failure while handling a message drops that hint.
- T14 removed phones' receivers are deleted, not tombstoned; a send in flight during reconfiguration or removal completes within ten seconds and is discarded.
- T15 apple terminal launches with `do script` and `activate` and does not poll for the created tab.
- T16 the hub writes its file only when a machine's records change, not on every poll.
- T17 hosts and apk ship in lockstep for this contract.
- T18 fleet and notification hints stay two sse streams. one stream needs the notifier's wake inside `fleet.Service`, and member gateways (`fleet/member.go`) treat any data line without the current `stateRevision` as a fleet change, so they would refetch the fleet per attention change until they parse an event kind; android's fleet watcher likewise. a merge would also let the foreground phone drop its five-second notification lag (T10).

no provider hooks/status api, agent-exit detection, foreground or presence
routing, per-device acknowledgement, priority tiers, custom sounds/quiet hours,
snooze, mute, inline approve/reply, history or replay, public ingress, linux
popups, arbitrary receivers, execution coordinator, database/outbox or
generalized notification framework.

## environment contract

- `SKIDBLADNIR_MAC_SIGNING_IDENTITY`: required for darwin release builds; no
  default. exact sha1 of the certificate pinned by `macos/app-signing-cert.sha256`.
  its keychain is on the ordinary user search list; private identity material stays
  outside git in the operator's protected configuration directory.
- `XDG_STATE_HOME`: optional; absent uses `~/.local/state`. notification files
  live in its `skidbladnir` subdirectory.
