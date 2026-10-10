# native needs-input notifications

accepted design, 2026-10-02. source is implemented and merged into main
([skid #66](https://github.com/NielsdaWheelz/skidbladnir/pull/66),
[dev-server #163](https://github.com/NielsdaWheelz/dev-server/pull/163)).
[qualification](native-notifications-qualification.md)
records current evidence; release and fleet deployment need separate approval.

this owns background observation, native alerts and the coordinated attention
cutover. [terminal observation](terminal-observation.md) owns detection;
[terminal attention](reply-notifications.md) supplies the existing transition
predicates; [session views](session-views.md) owns the surrounding queue;
[architecture](architecture.md) owns capability scope. this plan supersedes
device-local readiness observation and affected visit clauses;
historical qualification does not qualify the new system.

## target behaviour

- every new qualifying attention episode alerts macbook and android independently:
  ready, a response request, or a current error/interruption. request and notice
  coexist; changing either alerts again. unchanged polls and renames stay silent.
- all new alerts use identical prominence and platform-default sound, including
  ready. respect focus/do-not-disturb and user notification settings.
- one native notice occupies each session lifetime. replace it for a new reason;
  remove it on fresh resolution or identity replacement. uncertainty preserves
  the last notice. swipe/click dismisses the notice, never acknowledges ready.
- opening a session acknowledges ready on that device after actual output.
  requests remain in the queue until fresh terminal evidence clears them.
- suppress a new episode only when that exact session is proven foreground on
  that device. record it as locally presented; departure causes no reminder.
- clicking opens the captured session lifetime: android's existing terminal or
  a new ghostty window. freshly capture its current selected pane. vanished,
  replaced or unreachable targets produce an explicit result, never a namesake.
- reconnect fetches current state and alerts only unseen, still-current attention.
  no event replay, stale popup replay or reminders.

devbox owns the always-on observer and private ntfy. android delivery must work
with skid backgrounded, removed from recents or process-reclaimed, while the phone
is locked and macbook sleeps. explicit force-stop/app pause, revoked permission,
or stopped ntfy/tailscale suspends delivery until restored; this is an
[android boundary](https://developer.android.com/about/versions/15/behavior-changes-all#stopped-state).
mac delivery resumes after wake; skid's browser need not be open.

## responsibilities and composition

```text
independent gateways: existing authenticated inventories
  -> devbox observer: retained transitions + current snapshot
  -> android: ntfy / unifiedpush revision hint -> fetch snapshot
  -> mac: authenticated revision stream -> fetch snapshot
  -> device owner: local ready acknowledgement + episode handling
  -> native adapter: os notice / dismissal / exact-session click
```

gateways remain independent; observation performs no terminal attachment,
mutation, native-history read, provider supervision or gateway proxying.
direct client inventories still own cards, controls and fresh request/notice
membership. only the observer advances readiness and alert episodes. direct
polls, creation and entry preflights must not maintain a second ready reducer.

each device combines observer readiness with local acknowledgement. queue and
alert use the same readiness record and predicates; their independent samples
can briefly differ. during a gap the queue hides unqualified ready, while an
already delivered notice survives. devbox failure pauses ready qualification
and new alerts; direct requests/errors and session access continue.

linux tuis also adopt observer readiness and their own local acknowledgement.
linux os popups are outside scope. devbox's tui never acknowledges producer
state or another device.

## identity and schemas

reuse existing validated identities, opaque `Reference`, status enums and strict
json codecs. identity is never a name, title, cwd or native conversation.

```text
slot = {machine, tmuxId, identityToken}           // session lifetime; os identifier
key = {machine, tmuxId, identityToken, paneId}    // existing TerminalKey
foreground = {provider, pid, startIdentity}      // existing exact process lifetime
cause = {kind:"ready"} |
        {kind:"action", request?: permission|question|setup|confirmation|input,
                        notice?: interrupted|error}

record = {key, foreground?, phase: quiet|armed|ready,
          readyGeneration, attentionEpisode, cause?}
observerFile = {schema:1, epoch, revision, receiverRevision, records:record[],
                androidSubscription?}

snapshot = {schema:1, epoch, revision, receiverTag,
            machines:[{machine, availability:fresh|gap, observedAt?,
                       sessions?:[{ref, name, terminalStatus,
                                   attentionQualified:boolean, record}]}]}
hint = {schema:1, epoch, revision}
subscription = {endpoint, keys:{p256dh, auth}}   // standard web-push subscription

deviceFile = {schema:3, observerEpoch?, admittedRevision, localRevision,
              config?, desiredRegistration?:{generation, subscription},
              records:[{key, foreground?,
                receivedReadyGeneration, acknowledgedReadyGeneration,
                handledAttentionEpisode, presentation:claimed|posted|closed}]}
config = {observerMachine, ntfyOrigin}
```

action requires at least one field; ready excludes action fields. `ref` and
`key` must agree; foreground agreement qualifies attention, while a gap may
retain its former foreground without qualifying it. fresh inventories are complete; gap omits
`observedAt` and `sessions` and retires nothing. per-session unavailable/unknown
status can still be a gap inside a successful inventory. `attentionQualified`
distinguishes a current positive cause/known clear from retained gap memory.
current names/status stay in memory; attention records contain no terminal or
question text. the private transport subscription contains capability/key
material and is never returned in snapshots.

all counters satisfy `0 <= value < 2^63 - 1`; the maximum signed-64-bit value
means exhaustion and is invalid. reject an increment into exhaustion before
persistence or publication. zero means no token.
one durable producer revision advances per admitted machine outcome; transition
tokens take that commit's revision, never a timestamp. the epoch is generated
once with the producer file and survives ordinary restarts: 16 random bytes as
32 lowercase hex characters. reject nulls,
unknown fields/enums, malformed identities, duplicate slots/keys, invalid
combinations, regressing tokens and exhausted counters. reuse existing inventory
limits; bound state responses at 1 mib, hints at 256 bytes and endpoint bodies
at 4 kib. oversized/incompatible state is unavailable, never partial.

retain only the selected record per live session lifetime. positive pane/process
replacement resets its phase; returning to an earlier pane cannot reuse an old
episode token. successful complete inventory retires absent lifetimes; failed
hosts and unproven foreground absence do not.

producer file: `<state>/skidbladnir/notification-observer-v1.json`; device file:
`<state>/skidbladnir/notifications-v3.json`, or android's equivalent datastore.
reuse the existing state-directory resolution, mode-0600 atomic replacement and
locking. producer state MUST NOT use devbox's ordinary device cache.

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

`justify-polling`: background observation is required while clients are closed.
run three independent, coalesced five-second inventory lanes, each with a shared
five-second request deadline and existing two-second detector enrichment. use
a narrow raw per-machine inventory read; ordinary fleet `list` waits its slowest
peer and resolves remote contexts unnecessarily. no overlapping same-host reads.
serialize commits; cancel lanes with the service. a sleeping mac cannot delay
devbox/arch. this is one additional fleet sampler, not provider-specific polling.

persist a commit before exposing its revision or publishing hints. publish on
meaningful attention/name/identity/qualification changes; coalesce to the latest
revision while sending. publish one fleet hint every minute even when empty,
piggybacking this scheduler. this repairs cache-expiring outages and missed clears.
no outbox or historical resend. startup durably advances revision and marks every
host gap before serving;
retained facts qualify only after fresh reads. corrupt/unwritable state makes
the service unavailable; do not manufacture an empty snapshot.

## device admission and native presentation

one device owner serializes snapshot admission, ready acknowledgement, visits,
permission changes, dismissals and native effects. fetch outside its mutation
queue; coalesce one current fetch and reject obsolete completion generations.
same epoch rejects older revisions; equal revisions may reconstruct the current
view/reconcile permissions without allocating tokens or replaying visits.
bind a first epoch once; epoch changes require explicit local notification reset.
fence fetches by owner/config/reset generation; merge new producer fields without
lowering local acknowledgement. captured visit tokens fence callbacks, never a
whole-file overwrite. no clock comparison across hosts.

retain newer hint evidence while a read is in flight. a same-epoch hint raises
the minimum revision for new presentation; older/duplicate hints never lower it.
withhold obsolete reads and immediately coalesce a fresh read. an alien-epoch
hint invalidates a read dispatched before it, but never binds or resets memory;
a read dispatched afterward is authoritative even if that hint was stale.
fetched epoch changes still require explicit reset. uncertainty preserves
delivered notices and forbids new submission/repair. follow-up reads belong to
the owner and remain bounded when a caller/receiver wait expires.

native ready requires producer availability, fresh machine, qualified producer
idle row, exact key/foreground, producer phase ready, and
`receivedReadyGeneration > acknowledgedReadyGeneration`. queue ready additionally
requires matching fresh direct local key/foreground/idle. closed-phone delivery
does not fetch three gateways. a first actual
nonempty output callback acknowledges only its captured generation. attachment
success is insufficient. later ready is consumed only in an already presented,
currently proven foreground visit. ending/reconstructing a visit cannot replay
entry acknowledgement or consume a newer generation. evaluate focus for each new
ready generation as well as each new alert episode: a ready transition hidden
under an unchanged notice must still be consumed when actually viewed.

android proof requires resumed activity, window focus, interactive/unlocked
device, exact active terminal and actual output presentation. mac proof requires
ghostty frontmost, its exact focused surface id, and a live registered skid visit.
the helper records the surface id returned when creating a window and passes an
ephemeral launch nonce to `skid enter`; the live attachment reconnects that
association after helper restart. the creator returns an initial nonce/surface
binding receipt over the socket; the attachment retains it for reconnect.
connection/process lifetime owns it. ordinary
manual windows remain unknown and alert. no title/cwd/attachment-count inference.
[ghostty's api](https://ghostty.org/docs/features/applescript) supports created
surface ids; installed 1.3.1 does not supply a general tty/pid mapping.

first handle exact foreground presentation, even if os permission is denied.
otherwise check known permissions/configuration; blocked current attention stays
unseen. for a new eligible episode, commit `closed` for foreground suppression,
or persist `claimed` before one audible os submission.
os submission success proves scheduling/initiation, not visibility or reading.
on both platforms, retain its unresolved exact
copy per slot in process memory, separately from actual os notices. missing
metadata for that submission is unknown; only observing its full matching copy
settles `posted`. existing refreshes/hints reconcile it; no settlement timer.
restart discards that volatile knowledge and permits the specified claimed
repair. unresolved rename preserves the prior posted state and suppresses a
duplicate rename. cancellation includes owned unresolved adds and refuses an
older tuple when a successor is pending in that slot.
validate eligibility and initiate the native add in the same owner turn; a pure
authorization result cannot protect a later queued add. await asynchronous os
completion outside that turn. uncertainty after initiation preserves the notice
until fresh positive obsolescence or local acknowledgement closes it.

| persisted presentation | reconciliation |
| --- | --- |
| claimed | full matching pending/delivered copy settles as posted; absence with no unresolved copy permits fresh same-episode silent repair |
| posted | retain existing notice; absence with no unresolved copy becomes closed, never recreated |
| closed | cancel leftovers; never recreate this episode |

failed os inspection is unknown. a fresh host with unknown terminal facts is still
a row-level gap: only current positive action or qualified ready authorizes
submission/repair. gap permits no new submission/repair; local
acknowledgement/dismissal can finish cancellation. inspect mac pending AND
delivered requests; remove both on cancellation. persist cancellation intent
before its effect. tags/userInfo carry exact slot, episode, captured ref and
`readyGeneration` (zero for action, the captured generation for ready). this
lets local acknowledgement cancel a ready notice during a gap without parsing
display copy or clearing an action notice. rename preserves that generation.
old callback episodes cannot close successors. one ordered native effect lane
finishes an old add, removes its matching episode if obsolete, then applies the
latest intent; coalesce pending intents while keeping the state owner responsive.
dropping a late callback alone cannot undo an os effect. rename updates only an existing notice,
silently; reconcile absence first.

swipe, click or inferred os absence closes presentation only. ready is
acknowledged by terminal output, not tapping the banner. use immutable explicit
android pending intents; route mac clicks through structured native events and
existing `skid enter --ref`. fresh session-lifetime preflight still owns attach
admission. shell-quote fixed executable/ref arguments; never interpolate names.
use a canonical collision-free slot encoding as the native identifier; android
uses that tag with id 0 and `autoCancel=false`. app-owned click/dismiss closes
only its captured episode, so an old pending intent cannot cancel a successor.

at most one deliberate audible attempt per device/episode is promised.
claim-before-post can lose sound on a crash. silent repair can restore a just
swiped notice if its dismissal was not persisted. no atomicity is claimed across
os and disk. repair requests no sound, vibration or popup; verify android
`setSilent(true)` and mac passive delivery at the actual boundary.

## api, auth and transport

mount the separate loopback observer at devbox's existing pinned `:8443`
`/v1/notifications` serve path. reuse `auth.FileVerifier`, the devbox bearer
file and `Skidbladnir-Machine` handle check. extract shared auth admission only
where the new consumer removes actual duplication; gateways gain no routes.

| endpoint | contract |
| --- | --- |
| `GET /v1/notifications/state` | no body/query; bounded immediate latest snapshot; no inline fleet reads |
| `GET /v1/notifications/events` | mac sse revision hints; initial current hint, changed hints and minute pulse; no event ids/history/replay |
| `PUT /v1/notifications/receivers/android` | strict `subscription` and `If-Match: receiverTag`; atomically replace the one subscription; 204 after persistence, then sync hint |

reuse typed auth/machine/malformed errors; unavailable returns 503. state is
`Cache-Control: no-store`. no ack, focus, unread or dismissal api. registration is
idempotent: an already installed identical subscription succeeds; a different
subscription with stale tag returns 412. `receiverTag` is
`epoch:receiverRevision`, sent as a quoted opaque precondition. subscription
change atomically advances the separate receiver counter and snapshot revision;
identical registration advances neither. missing precondition returns 428.
the tag in the body is not a whole-snapshot etag. no terminal/control mutation is retried.

android uses one unique network-constrained enrollment job. persist the job
before requesting an endpoint; only that durable worker's owner operation calls
public unifiedpush registration. setup/config/reset enqueue work; startup and
callback recovery use KEEP, explicit setup/config/reset may REPLACE. each attempt
arms a receipt, registers with the retained sdk token/keys, and waits for a valid
callback consumed durably by the sole owner. retained desired state alone cannot
satisfy this wait. synchronously order callback intake; remove arbitrary callback
persistence deadlines. identical callbacks acknowledge receipt without advancing
their generation or replacing their own worker.
persist the latest desired subscription and generation. bounded state-read/put attempts
use workmanager backoff; read latest desired state after fetching the tag, check
its generation before dispatch/after response, and refresh on conflict.
fence installation by config/reset, callback intake, desired generation and the
current public sdk keyset. only confirmed installation of the latest tuple
completes work. process death before callback persistence leaves unfinished work
that re-registers; missing receipt/conflict/uncertain response retries.
this one-shot setup retry repairs
retained-endpoint enrollment while skid remains closed; no periodic polling or registration
history. a lost successful response is safe to retry. publisher errors retain the
current subscription, including 404; minute hints retry after recovery.
the qualified ntfy 1.25.2 pin emits endpoints in response to REGISTER and fixes
the endpoint for an existing token; it does not autonomously rotate it. callback
entry order is not a promise about distributor broadcast order. reject obsolete
keysets; unsolicited future-distributor rotation, ntfy uninstall/data clearing
or explicit subscription deletion require renewed qualification.

go client config adds `notifications:config`; absence disables this capability
and shows setup required while preserving ordinary fleet access. the existing fleet qr
becomes `skidbladnir.fleet-invite.v2` with that same object; reject v1. explicit
`observerMachine` must match one paired handle, not a display-name guess.
it selects devbox; `ntfyOrigin` is canonical https on that peer's pinned hostname,
port 8444, without user-info/path/query/fragment.
android preserves existing encrypted pairings and stores notification config
bound to the complete installed handles/origins. after upgrade, one ordinary
reconnect qr supplies it. absent config means setup required, not legacy mode;
if config persistence fails, keep normal fleet access and report setup incomplete.

service entry is `skid notifications observer --config FILE`. its strict private
config names the existing client, bearer and machine-handle files, loopback listen
address and ntfy publisher credential file. derive the pinned ntfy origin from
client config; deployment must agree. no ambient credential lookup or new bearer.

serve ntfy privately on devbox's pinned tailnet https `:8444` origin. deny public
ingress, funnel, firebase/upstream forwarding and anonymous reads/writes. keep
publisher/reader credentials in deployment-owned private files or ntfy's own
account store; never push them, print endpoint capabilities or persist them as
attention facts. the observer's publisher may write the fixed `up*` namespace;
ntfy's phone account subscribes there. registration accepts only that exact
configured origin and ntfy topic shape; refuse redirects and arbitrary callback
urls. authenticate publication rather than adopting the documentation's anonymous
write example. qualify the actual acl/distributor combination before cutover.
[ntfy configuration](https://docs.ntfy.sh/config/#access-control) owns its acl.

android uses the official unifiedpush connector's current encrypted protocol
(`AND_3.1`), with ntfy only. pin exact connector/distributor versions during the
first interoperability probe. implement its `PushService` callbacks; reuse its
receiver/token checks, message acknowledgement and foreground-importance binding.
map `PushEndpoint.url/pubKeySet` to `endpoint/keys`, with `pubKey` as `p256dh`.
accept only decrypted `PushMessage` content; no deprecated receiver/plaintext,
embedded distributor or firebase path.

validate strict unpadded base64url: `p256dh` is a valid 65-byte uncompressed p-256
point, `auth` is 16 bytes. sender encrypts each hint as one bounded
rfc8291 `aes128gcm` record, with fresh sender key/salt; ciphertext stays below
512 bytes. post ciphertext with `Content-Encoding: aes128gcm`,
`Content-Type: application/octet-stream`, `X-UnifiedPush: 1` and the ntfy
publisher bearer. vapid is omitted for this fixed authenticated ntfy transport.
use one encode-only codec over go's standard ecdh, hkdf-sha256, aes-gcm and random
primitives; no unqualified web-push dependency. `rs=4096`; derive ikm from the
shared secret with auth salt and `WebPush: info\0 + receiverPub + senderPub`,
producing 32-byte ikm. derive 16-byte cek and 12-byte nonce from ikm with random
salt and `Content-Encoding: aes128gcm\0` / `Content-Encoding: nonce\0`.
wire is `salt16 | rs4 big-endian | key-length1 | senderPub65 | gcm(hint + 0x02)`,
with empty associated data. no decryptor, multi-record support or padding modes.
qualify against the normative known-answer vector,
an independent decoder and the actual connector before product integration;
no hand-written cryptographic primitives, dummy vapid or authorization replacement.
[web-push encryption](https://www.rfc-editor.org/rfc/rfc8291),
[connector keys](https://unifiedpush.org/kdoc/connector/org.unifiedpush.android.connector.data/-public-key-set/).

handle a hint with one bounded three-second state
fetch, completing receiver work within five seconds; timeout waits for the next
hint/resume. no permanent skid foreground service or periodic workmanager poll.
ntfy owns the persistent connection; tailscale and ntfy background/battery setup
are explicit prerequisites. ntfy transports bytes; skid renders the user alert.
[connector contract](https://unifiedpush.org/developers/spec/android/),
[ntfy publication](https://docs.ntfy.sh/publish/#unifiedpush).

mac uses the authenticated sse stream, avoiding another ntfy topic/read credential.
reconnect with one cancellable attempt and 1-to-30-second exponential backoff.
reset the delay only after an attempt lasts at least 30 seconds; an initial hint
followed by disconnection does not reset it. short useful streams can therefore
wait up to 30 seconds after disconnecting. hints fetch current state immediately;
wake/resume fetch immediately. foreground clients also fetch on their
existing five-second refresh and manual pull; mac reads coalesce through its owner,
linux uses its file-lock owner, android its application owner. no background phone
polling. startup/setup recovery fetches too; unchanged minute hints are silent.
transport is never canonical state:
[ntfy's cache](https://docs.ntfy.sh/config/#message-cache) is finite.

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
unicode/case; replace controls/format/line separators using the existing
`singleLine` policy, extracted at the second go consumer. native layout owns
wrapping/ellipsis. duplicate names remain visually ambiguous; hidden exact
identity still routes correctly. reuse existing reason phrases rather than a
second map keyed on strings. menu alone is excluded; menu + error says error.

android has one `needs-input` channel, label `needs input`, importance high,
default sound/vibration and no dnd bypass/full-screen intent. every fresh reason
uses the same settings; native policy may group/throttle alerts.
[channel behaviour](https://developer.android.com/develop/ui/views/notifications/channels).
request notification permission through native setup.

mac is one background `Skid.app`, stable bundle id `dev.niels.skidbladnir`,
existing icon, notification delegate and ghostty automation description.
go owns decisions/state; a narrow objective-c/cgo bridge owns appkit,
usernotifications and apple events. run the native event loop on its main os
thread, locked before appkit starts; go state work runs elsewhere. native callbacks
enqueue and return; appkit/automation dispatch to main is asynchronous to avoid
mutual waits. thread-safe notification add starts directly in the owner turn;
its buffered completion is awaited by the ordered effect lane outside the owner.
no swift child, terminal-notifier fallback or apns.
deployment copies the admitted signed bundle to the real directory
`~/Applications/Skid.app`; an application symlink is invalid. stop the helper
before replacing that directory. run its `register ABSOLUTE_APP_BUNDLE` mode on
the installed logical path before login activation: public `LSRegisterURL` with
update enabled, success or fixed failure, without config, sockets or consent.
login activates the installed app through public `/usr/bin/open -g -W -a` with
explicit state/config/executable arguments and app stdout/stderr paths. launchd
supervises the waiting `open`; the registered app owns native events and its
socket. unconditional keepalive restarts ordinary quits as well as crashes;
disabling delivery requires disabling the service. all application activation,
including cold launch, ordinary reopening and notification clicks, stays quiet.
`skid notifications setup` explicitly opens the existing native setup window
through the running helper; unavailable helper returns an error without launching
another owner. remove the former `--background` flag and activation-based setup.
replacement/removal first stop the waiter, then use the admitted helper's
`stop ABSOLUTE_APP_BUNDLE` mode to terminate exact registered app instances and
confirm absence. a surviving socket rejects replacement. `status` returns the
single finished registered app's pid; readiness requires that pid to own the
exact socket and answer a bounded typed `read`. setup may report producer
unavailable. these utility modes neither acquire the owner nor request consent.
unchanged apply is inert; generation, unit or private client bytes changing
restart the helper. initial absent fleet config permits the existing setup
state; provisioning it must trigger activation. no runtime registration repair.
the managed unit explicitly fixes `XDG_STATE_HOME` to the installer's
`~/.local/state`, keeping its socket/readiness checks on that same path.
the app is sole mac store/os writer; cli/tui use
`<state>/skidbladnir/notifications.sock`, mode 0600, with newline-delimited strict
json requests (4 kib) and responses (1 mib), a three-second operation deadline
shared with any child state fetch:
`{op,args}` -> `{ok:true,value}` or `{ok:false,error:invalid|unavailable}`.

| operation | request / result |
| --- | --- |
| setup | empty args -> `{}` after the main-thread native setup operation; no producer-read prerequisite |
| read | empty args -> `{deviceSnapshot,producerAvailable,currentProducerSnapshot?}`; join a fresh/coalesced state read |
| register | exact slot/key/foreground, captured ready token, already-presented flag, optional helper-issued surface association -> connection-owned visit id |
| presented | visit id + `{epoch,generation}` ready token -> acknowledge only that captured exact identity |
| end | visit id -> retire visit; no ready acknowledgement |

disconnect retires only that connection's visits; reconnect registers an already
presented visit without repeating first-output acknowledgement. helper-created
windows obtain their surface association from the creator before claiming focus;
lost/unproven associations stay unknown. query focus only for candidate episodes
or new ready generations, never with a periodic focus watcher.
focus binds the session slot; acknowledgement binds its captured key/foreground
and ready token. callers retain current producer view in memory; device records
alone cannot qualify ready. owner unavailable means the existing secondary notification failure, never a
local reducer fallback. terminal work continues.

entry performs optional socket reads/registration before websocket admission.
first-output acknowledgement is synchronous and bounded to three seconds; the
live visit closes before its secondary post-detachment refresh.

reuse a durable signing identity, or one persistent local signing certificate
for this internal install. ad-hoc signatures cannot establish upgrade permission
continuity. qualify notification/automation consent with two different builds.
local signing does not provide public gatekeeper distribution.
[apple signing contract](https://developer.apple.com/documentation/technotes/tn3127-inside-code-signing-requirements),
[local certificates](https://developer.apple.com/library/archive/technotes/tn2206/).

android reuses the dashboard notice panel with `set up notifications` and
`open notification settings` actions. mac opens one small native notification
setup window only through `skid notifications setup`; app activation stays quiet.
include current health, permission/settings
action and instructions to keep tailscale/ntfy configured. successful fleet
reconnect is separate from successful notification config/receiver registration.
click failures stay visible in the terminal; a failed ghostty launch uses a
user-initiated native error dialog.
android's notification owner is application/process-scoped, reusing datastore
serialization. controller teardown ends its visit/collector; it neither closes
push ownership nor creates a second os writer.
on epoch mismatch, setup offers `reset notification memory`. both native owners
persist old records as closed, invalidate captured visits and pending work, and
hold new snapshot admission. the existing native lane cancels observed and
unresolved copies. only after known owned absence and no unresolved copy, clear
local acknowledgement/handling records and bind a fresh snapshot. confirmation
is bounded; stale os observations wait for an existing wake rather than spin.
an interrupted reset retains cancellation intent; retry the available reset
action to finish clearing memory. android's public reset action remains limited
to reset-required setup; no general same-epoch retry control. preserve
pairings/config. android re-registers through its existing serialized owner.
linux exposes that same operation as `skid notifications reset`; no os surface.

one quiet setup/health disclosure uses: `allow notifications for needs input.`,
`notifications are blocked in system settings.`, `install ntfy to receive
notifications.`, `notification setup required.`, or `notification delivery
unavailable.`. name ntfy/tailscale as the fault only when proven. stale click
outcomes use existing terminal admission errors; ghostty launch failure says
`could not open ghostty. open the session from skid.`. no health sounds,
inline approve/reply, snooze, mute, reminders or settings dashboard.

## cutover and disjoint work

hard-cut all attention-owning clients to schema 3 and producer evidence; do not
read/migrate schema 2. old files are inert and the first upgraded baseline resets
old ready once. remove old `Observe/ObserveSession` arming paths, predecessor
plumbing and stale visit interfaces after switching callers. ordinary cli
`list/info/wait` still does not write device attention; strict needs-input wait
remains response-request only. provider `skid-notify` bel remains separate
terminal-local presentation, never an alert source.

| owner | exclusive files and responsibility |
| --- | --- |
| root integrator | this spec; architecture/roadmap; issues; shared final contracts; `catalog/**` if needed; `scripts/check` composition |
| attention engineer | new `internal/attention/**`; `internal/fleetclient/{notifications,status,content,config}.go`; new typed observer/socket client; extract pure predicates/copy and schema-3 owner without another reducer |
| observer engineer | new `internal/notifier/**`; notifier subcommand in a new `internal/agentcli/notifier.go`; raw per-machine read in `internal/fleetclient/client.go`; shared auth extraction in `internal/auth/**` |
| desktop engineer | `internal/sessionui/{notifications,session}.go`; `internal/terminalclient/terminal.go`; `internal/agentcli/run.go`; producer admission, captured-generation output ack and visit/socket lifecycle |
| mac engineer | new `cmd/skid-notifications/**`, `internal/macnotifications/**`, `macos/**`; sole owner, native bridge, focus, ghostty and app metadata |
| android engineer | `NotificationStore.kt`, `SkidbladnirController.kt`, `ProductModel.kt`, `GatewayClient.kt`, `MainActivity.kt`, `DashboardScreen.kt`, `FleetInvite.kt`, new push/native/enrollment files, manifest and dependency/build pins |
| release/deployment engineer | `scripts/{build,build-host-binary,build-host-archive,check-release,fleet,release}`, generated archive manifests outside catalog; separate assigned dev-server files for observer/ntfy units, acl, serve paths, mac login app and signing/install |
| content designer | copy/schema examples and native setup/alert review supplied to each writer; no overlapping production edits |
| adversarial verifier | read-only design/code/evidence review; writes no production or tests |

root delegates exact new file names before work; no cross-slice edits. freeze
shared contracts first, then implement independent adapters against them. writers
own their temporary boundary tests. install through dev-server only; extend the
exact darwin archive/checker for the app and native signer. `scripts/fleet`
remains verification/invitation/provisioning, not a second installer.

## implementation and acceptance

2026-10-09 completion amendment: reuse qualified behaviour through unchanged
source paths; do not repeat broad qa sweeps. retain one real mac-asleep/locked-phone
smoke check and clean release/rollback verification, including one current mac
bundle notice/click. forced-deep-idle delivery and muted-phone audible sound remain
explicit unverified follow-ups, accepted by the user; neither blocks completion.
runtime requirements remain unchanged. optional energy and historical sdk
diagnostics are separate follow-ups.

1. prove actual private ntfy/unifiedpush locked-phone delivery, mac bundle
   consent/upgrade and helper-created focus mapping. review failures before
   building product machinery; no mock counts as these boundaries.
2. write temporary end-to-end integration/live tests against unchanged source.
   demonstrate missing target behaviour, not compilation/missing-binary failure.
3. implement observer/contracts, then device owners/adapters and packaging.
   review each boundary and adversarial ordering before progressing.
4. make the tests pass; refactor ownership, repetition and dead paths under the
   rules; rerun only affected evidence and required engineering checks.
5. remove temporary tests/harnesses before commit. retain concise credential-free
   evidence and unresolved issues; release/deployment need separate authorization.

| acceptance boundary | required observation |
| --- | --- |
| full path, both providers | authored terminal frames -> isolated real tmux -> stock gateway -> observer -> real transport -> native os notice -> exact-session click; live-provider smoke |
| reasons/content | every request/notice, composites, menu+notice, ready including startup/menu; equal sound/prominence; unicode/controls; rename silent |
| continuity | working -> gap/restart -> idle; first idle quiet; replacement; action -> unknown activity/known absent request clears; ready -> working/unknown interaction clears; unchanged polling silent |
| device independence | both devices alert; mac ready visit leaves phone ready; two mac tuis share only local ack; no sound replay after swipe/departure |
| foreground | exact presented phone/helper window suppresses; background tab/split/window, other session and unknown manual window alert |
| races/recovery | reversed/duplicate hints, stale fetch/visit/dismiss callbacks, claim crash, OS query failure, producer/device/ntfy restart, long outage beyond cache, empty missed clear |
| local recovery | equal-revision restart/permission repair; denied permission during foreground presentation creates no departure alert; fleet reconnect with failed notification-config persistence preserves access and shows setup required |
| enrollment | retained-endpoint recovery while observer is down and skid remains closed; lost registration response; delayed receiver-registration submissions cannot replace a newer installed subscription or supersede newer locally accepted desired state |
| offline policy | resolved-before-reconnect stays silent; unseen still-current alerts once; notifier outage preserves memory/notice while controls and fresh action rows work |
| physical delivery | one real locked-phone delivery with skid closed and mac asleep; retain qualified tailscale/ntfy, heads-up, dnd and permission witnesses; forced-deep-idle and phone audible sound remain explicit unverified follow-ups |
| release | signed app/apk, managed login/service restart, two-build consent continuity, new qr/config, complete previous-release rollback without migrations |

tmux/phone runs require explicit current-turn approval; only exact temporary
sessions on isolated `-L` sockets may be controlled. sound/dnd/focus need actual
devices. absent boundaries are `NOT_RUN`, never passes. use
[testing policy](rules/testing.md); do not restore retired gates or modify the
retained detector corpus unless classification changes.

## accepted tradeoffs and exclusions

devbox is a readiness dependency; no observer failover. the additional sampler
adds bounded gateway/detector work. minute hints add background wakeups; transport
and os policy still prevent a hard delivery deadline. existing five-second
inference misses brief/same-kind resets and can misclassify; no completion proof.
manual ghostty focus is unknown. mac uses a second hint protocol to avoid extra
ntfy credentials. a bounded standard-library web-push codec adds protocol
responsibility, discharged by normative and independent interoperability evidence.
one durable enrollment job is necessary for background endpoint recovery.
schema cutover resets attention once and needs a reconnect qr.
the real mac application duplicates its signed generation and briefly stops
during replacement; launch services rejects a symlink to that generation.
at-most-once audible intent accepts the documented crash window. deleting tests
leaves no retained behavioral regression protection.
presentation settlement may wait for the next refresh/minute hint;
an unobserved initiation stays uncertain until resolution, acknowledgement,
supersession or restart.
completion accepts unverified forced-deep-idle delivery and phone audible sound;
keep those limits visible after release. preserve user alarms and audio settings.

no provider hooks/status api, agent-exit detection, shared read receipts,
presence routing, priority tiers, custom sounds/quiet hours, notification history,
public ingress, firebase, linux popups, arbitrary receivers, execution coordinator,
database/outbox, generalized notification framework or unrelated cleanup.
[implementation and qualification](issues/native-notifications.md) remain open.

## environment contract

- `SKIDBLADNIR_MAC_SIGNING_IDENTITY`: required for darwin release builds; no
  default. exact sha1 of the persistent certificate pinned by
  `macos/app-signing-cert.sha256`.
- `SKID_NOTIFICATION_LAUNCH`: optional, normally absent. the mac helper passes
  its ephemeral window nonce to the one `skid enter` child; it establishes no
  identity or foreground proof by itself.
- `XDG_STATE_HOME`: optional, retaining existing state resolution. absent uses
  `~/.local/state`; notification files live in its `skidbladnir` subdirectory.

the persistent signing keychain belongs to the ordinary user search list.
builds select its exact certificate digest; private identity material stays
outside git, in the operator's protected configuration directory.
