# Working-directory chooser

Status: **implementation candidate, 2026-08-31**. All four owner reds are
recorded and production source is hard-cut. On the pre-rebase feature tree,
routine verification, the approved isolated Darwin and Linux real-tmux Create
regressions, and the repository-signed same-version S22+ owner journey and full
55-test candidate suite were green, with pairing unchanged and the exact
production APK restored. Those results do not prove the rebased source.
Rebased routine verification and the approved Darwin/Linux isolated-tmux gates
are green; governed release-bound `platform` and hands-on
portrait/large-text/TalkBack gates remain `NOT_RUN`.

[`architecture.md`](architecture.md) owns final product behavior;
[`roadmap.md`](roadmap.md) owns delivery order. This document owns the closed
delta. Tests follow [`rules/testing.md`](rules/testing.md); this repository has
no separate `testing-standards.md` and must not add one.

## Scope and final state

Forge replaces its primary raw cwd field with **Choose a working directory** on
one explicit, fresh machine. The chooser has four routes:

1. Browse Home;
2. directly select an exact cwd from current tmux inventory (`Active`);
3. walk Home one level at a time and filter the current folder locally;
4. enter an exact path as the secondary expert route.

Selection only fills the Forge draft. Existing `POST /v1/sessions` remains the
sole mutation and revalidates cwd immediately before tmux. Tmux remains the
database. No directory, recency, favorite, query, listing, alias, or workspace
fact persists.

## Goals and rules

- Optimize for recognition and touch, not mobile path recall.
- Keep machine, current location, Parent, Back, Use, and Create unambiguous.
- Preserve arbitrary valid cwd support, including paths outside Home.
- Browse read-only, one level per bounded request; never crawl or infer.
- Use server-returned browse tokens; Android never joins filesystem paths.
- Retain truthful prior content through loading/failure; never render failure as
  an empty folder or choose a fallback.
- Keep paths, names, filters, payloads, listing sizes, and selections out of
  logs, analytics, crash breadcrumbs, screenshots, and evidence.
- Add no polling, retry loop, cache, watcher, index, persistence, or dependency.

No implementation question remains open. Changing browse root, persistence,
search depth, filesystem mutation, or AI behavior reopens scope and acceptance.

## Closed decisions and trade-offs

| Decision | Cost accepted | Rejected alternative |
| --- | --- | --- |
| Custom remote picker using Android interaction grammar | Owned Compose UI | SAF/`DocumentsProvider` models device/providers and creates a second filesystem product |
| Home is the only browse root; exact entry covers everything else | No `/` tree | Root enumeration, host configuration, or treating UI scope as authorization |
| Browse wire values are canonical `~` tokens | No dedupe with absolute Active paths | Exposing the account Home prefix or client canonicalization |
| `Active` is fresh tmux state, never persisted recency | Suggestions disappear with sessions | A second source of truth |
| Active tap selects; it never becomes a browse root | Nearby navigation starts at Home | Dual-action rows and inside/outside-Home branching |
| Immediate children plus local fuzzy filter | Repeated taps for depth | Recursive/global search, prefetch, pagination, or index |
| 4,096 scanned entries; 256 folders; 32 KiB path text; 64 KiB encoded response | Huge folders use exact entry | Silent truncation or unbounded work |
| Filter: 256 Unicode scalars; Back history: 32 decoded views | Longer query/deeper history is bounded | Per-keystroke or retained-memory growth |
| Server order folds ASCII `A–Z` only, then uses exact unsigned UTF-8 | Non-ASCII case variants are not grouped | Moving, personalized, locale-dependent, or Unicode-version-dependent rows |
| Folder tap enters; sticky Use selects | One extra tap | Accidental wrong-cwd launch |
| Parent plus Back, no tappable breadcrumb strip | Ancestor jumps take repeated taps | Crowded mobile chrome or client-derived ancestor paths |
| Picker Back owns the modal window dispatcher while Material Back dismissal is disabled | One small Android-window lifecycle adapter; no custom predictive-progress animation | Material's shared dismiss callback cannot distinguish Back from the scrim, so either Back would close Forge or the scrim would stop dismissing it |
| Root-relative symlinks resolving within Home are marked/traversable | Absolute/escaping links use exact entry | Reimplementing `os.Root` or silent escape |
| Root-resolution failures are `Unavailable` | No raced-escape distinction | Matching private OS errors or duplicating the safe resolver |
| Hidden folders return once and are hidden locally by default | One toggle | Extra server query or inaccessible dot-directories |
| One documented cwd grammar is enforced at display and server boundaries | Unsafe/over-limit paths stop launching | Deceptive rendering or divergent rules |
| Machine labels hard-cut to the same display-safe control grammar | A stored label containing bidi controls becomes invalid | Isolation alone cannot make an internally overridden identity trustworthy |
| Exact path is secondary but first-class | One additional page | Removing expert access or retaining two primary editors |
| Read query uses `POST` | Not cacheable REST retrieval | Sensitive paths in URLs/logs/caches |
| Reject unpaired JSON surrogate escapes in shared strict ingress | Every route hardens together | Route-local scanning would permit divergent normalization to U+FFFD |
| Compose proof injects concrete store/client defaults and intercepts HTTP only | Two concrete constructor seams remain visible internally | A callback harness bypasses the production owner; generic test interfaces add a second architecture |
| Browse context and its live region stay composed above the tree | One less visible tree row | A first lazy item disappears at depth and cannot announce async state |
| Saveable list keys use row-kind plus immutable-snapshot ordinal | Identity is scoped to one picker snapshot | Raw or deterministically hashed paths can leak through Android saved state |
| One package-local descriptor interleaving proves requested-directory identity drift | That security proof follows the resolver seam | A public timing race is flaky or requires a test-only production hook |
| Permission acceptance requires a non-root Darwin or Linux run | Root runs leave that sub-proof `NOT_RUN` | Root bypass makes a green permission claim false |

## Capability contract

1. Directory choice enables only for an exact `InventoryState.Fresh` machine.
2. Opening snapshots safe, non-empty, distinct cwd values from that already
   owned inventory; it does not refresh or subscribe. Sort by case-insensitive
   full path with exact UTF-8 tie-break.
3. Places shows Browse Home, Active rows, and Enter exact path. Active selection
   performs no filesystem request; Create still validates it.
4. Browse Home posts literal `~`. Folder, Parent, and current-directory actions
   use only typed values returned by the server. Use selects the current token.
5. Filter matches basename by exact, prefix, contiguous substring, then ordered
   subsequence; shorter basename and server order break ties. Empty preserves
   server order. Use `Locale.ROOT`; never alter path bytes.
6. Hidden means basename-leading-dot. Show the toggle only when hidden rows
   exist. Search and hidden filtering are presentation, never access rules.
7. Header and system Back restore prior listing, filter, hidden state, and stable row/offset;
   retain 32 views, evict oldest-first, then return to Places. Parent always
   uses the server-provided structural parent. Back from Exact Path retains its
   draft and normalized origin: Places remains Places; loaded/failed Browse
   remains that Browse snapshot; loading returns to its retained Browse snapshot
   or Places when none exists. Restore non-animated before rows re-enable. Back
   from Places or visible Cancel returns to Forge unchanged.
8. Exact Path is single-line, URI keyboard, focused, with autocorrect/smart
   punctuation off. Its title, label, and guidance are `Enter exact path`,
   `Working directory`, and `Use an absolute path or ~/…`. Prefill Forge cwd
   once and retain edits within the chooser. Blank is pristine; IME Done on
   blank or invalid text marks and politely announces
   `Choose a valid working directory.` without leaving; valid IME Done performs
   Use path, never Create.
9. A choice writes only `ForgeForm.cwd`, clears its cwd error, and returns to the
   form. Profile, tmux name, and objective are unchanged.
10. Machine change retains the existing hard rule: clear cwd/profile, close the
    chooser, invalidate requests, preserve name/objective.
11. Accept a response only for the exact foreground generation, machine,
    chooser instance, and latest request sequence.
    Background invalidation settles an active Loading page to its retained
    Loaded view or Places; resume never strands Loading or retries implicitly.
12. Loading retains the prior path/rows, labels the requested target separately,
    and disables Parent/folder/Use; Back, Cancel, Exact Path, and access loss
    invalidate the request. Never relabel retained rows as target children.
13. Browse failure reactivates retained content. Its only new recovery actions
    are Try again and Enter exact path; there is no automatic retry.
14. Forge retains the typed definite Create rejection. A
    `WorkingDirectoryInvalid` or `WorkingDirectoryUnavailable` rejection
    preserves cwd and exposes Change into Exact Path without string matching;
    choosing a directory clears it. Existing outcome-unknown Create behavior is
    unchanged.

## Content and interaction contract

Before Android builders, one product/content designer signs this closed
projection. Content sign-off: approved 2026-08-31 after adversarial closure of
loading, return, copy, bidi, TalkBack/IME, and typed-rejection states.

```text
PickerContent =
  Places(title, machine, PlaceRow(Home | Active)*, exactPathAction)
  | Browse(title, machine,
      location=Current(path) | Requested(path),
      parent=Absent | AtHome | Available,
      hidden=Absent | Show | Hide,
      omission=None | Present,
      status=Ready | Loading(requested) | Empty |
        Failed(requested, Transport | Unavailable | TooLarge | Internal),
      FolderRow(Directory | SymbolicLink)*,
      useAction, exactPathAction)
  | ExactPath(title, machine, returnTo=Places | BrowseSnapshot,
      fieldLabel, guidance,
      validation=Pristine | Valid | Invalid, useAction)

Chrome = Back + visible Cancel
```

The picker replaces the Forge form within the sheet and uses its full available
height; do not nest another modal or compress the tree into the form. A Browse
location is `Current` only when its rows describe that path. With no retained
listing, loading/failure labels the candidate as `Requested` and exposes no
Parent, folder, or Use action; it never presents the candidate as opened.

One exhaustive pure mapping owns visual/spoken copy, tone, and recovery. Good
content is truthful, literal, machine-specific, scannable at large text, and
equivalent through TalkBack. Dynamic values are only validated machine labels,
exact paths, and safe basenames; render them with bidi isolation without
changing bytes. The designer separately signs Places orientation, Browse
location/action distinction, Exact Path expert guidance, and failure
cause/recovery.

| State | Literal content |
| --- | --- |
| No machine Forge | `Choose a machine to choose a working directory and profile.` |
| Empty/selected Forge | `Choose a working directory` / `Working directory on <machine>` + exact path + `Change` |
| Places | `Choose working directory`, `On <machine>`, `Browse Home`, `Active on <machine>`, `Enter exact path` |
| Browse | exact path, `Parent folder`, `Filter folders`, `Show hidden folders` / `Hide hidden folders`, `Use this folder` |
| Home / Exact Use | `Use Home` / `Use path` |
| Loading / empty / omissions | `Opening “<basename>”…` / `No visible folders here.` / `Some folders cannot be shown.` |
| Transport | existing `Could not reach this machine over your Tailnet.` + `Try again` |
| Unavailable | `This directory cannot be browsed. Enter the path instead.` |
| Too large | `This directory has too many folders to show. Enter the path instead.` |
| Internal | existing `Skíðblaðnir could not complete the request.` + `Try again` |

Home semantics: `Browse Home. Opens Home folders on <machine>.` Active
semantics: `Working directory <exact path>. Selects this directory on
<machine>.` Folder semantics: `Folder <name>. Opens folder.` Symlink semantics:
`Linked folder <name>. Opens folder.` Use semantics are `Use Home as working
directory on <machine>.` at Home and otherwise `Use <exact path> as working
directory on <machine>.` No icon/gesture/long-press-only action, toast, inferred
`project` label, or success message exists. Targets are at least 48dp. Exact
paths use a one-line horizontally scrollable mono region initially showing the
tail—never marquee/ellipsis—and expose the full value to TalkBack. Use one
`LazyColumn` for tree rows, keyed by closed row kind plus path-free ordinal;
typed paths stay in process memory for actions and viewport ownership. Browse
context and its sole live region stay composed between the header and list;
sticky Use stays outside.

Unavailable, TooLarge, Internal, transport, and exact validation use
`NoticeTone.Failure`; omissions use `NoticeTone.Degraded`; loading has no
failure tone. One polite
live region owns `Opening`, the accepted `Current folder <path>`, empty, and
failure announcements so TalkBack receives every async transition without a
focus trap. Visual and spoken dynamic values use the same raw owned value.
Machine labels reject C0/C1, U+2028/U+2029, and bidi controls at `MachineLabel`
ingress; labels are Unicode-isolated at composition. Paths already reject those
controls, render in an explicitly LTR isolated container, scroll logically to
the tail, and retain the raw full value for TalkBack.

## Host architecture

Create `internal/workdir`, a narrow semantic service and sole owner of Home,
cwd parsing/normalization, start validation, browse containment/projection,
ordering, and bounds:

```text
opaque WorkingDirectoryCandidate, WorkingDirectory, HomeDirectory, ParentDirectory
Listing { directory, parent=None|Some, children: Entry[], omissions=None|Present }
Entry { directory, kind=Directory|SymbolicLink }
ErrorCode = Invalid | Unavailable | TooLarge       # no path or raw OS text

New(home) -> Service
Service.ParseCandidate(string) -> WorkingDirectoryCandidate
Service.ParseBrowseDirectory(string) -> HomeDirectory
Service.ValidateStart(candidate) -> WorkingDirectory
Service.List(context, HomeDirectory) -> Listing
```

- Opaque values have package-private representation/construction.
- `New`: absolute, clean, searchable service-UID Home satisfying cwd grammar.
- Create grammar: input and normalized absolute value are each 1–4,096 UTF-8
  bytes; reject C0/C1, U+2028/U+2029, bidi controls, relative/`~user` input;
  expand only exact `~`/`~/`; otherwise require absolute; `filepath.Clean` once.
  Android checks the display/input subset; server owns normalization, expanded
  length, directory existence, and search permission.
- Browse grammar: only canonical `~` or `~/...`; reject absolute, relative,
  empty, dot-segment, repeated-separator, and trailing-separator forms.
- Open one `os.Root` per listing and close before return. Do not use prefix
  containment, `EvalSymlinks`, shell/`find`, or caller descriptors. Home is a
  pathname—not device—root; mounted directories beneath it remain in scope.
- Return immediate searchable directories only. Ignore files. Include only
  root-relative symlinks whose root-scoped resolution remains a directory
  inside Home; mark them `SymbolicLink`.
- Omit raced-away, escaping, unsafe, or over-limit candidate folders and return
  only whether omissions occurred. Never expose a count or escaped substitute.
- Return hidden folders; Android owns visibility. Sort as specified above.
- Read fixed batches and check cancellation between them. Exceeding any scan,
  child, or path-text bound returns `TooLarge` with no partial listing.
- Requested-directory read/search/root-resolution failure is `Unavailable`;
  malformed input is `Invalid`; cancellation returns the context error.
- A listing is advisory; Create always revalidates. No cache, watcher, retry,
  goroutine, persistent descriptor, mutation, or telemetry exists.

`main` creates one immutable, concurrency-safe concrete `*workdir.Service` and
passes it to `sessions.Manager` and `gateway.Gateway`; no interface wraps it.
Remove `sessions.Config.Home`. Sessions maps Invalid/Unavailable to existing
Create errors; listing maps Invalid to `InvalidRequest`. Keep cwd typed through
the exact tmux `-c` adapter. Delete the sessions-owned normalizer/tests; other
session validation remains in `sessions`.

## HTTP API

```http
POST /v1/directory-listings
Authorization: Bearer <credential>
Skidbladnir-Machine: <pinned handle>
Content-Type: application/json

{"directory":"~/Documents/code"}
```

The body has exactly one case-sensitive, non-duplicate required string key. Its
value is a canonical Home token, never absolute. Existing strict handling
rejects missing/unknown/alternate/duplicate/null/wrong-typed/noncanonical,
trailing, content-encoded, or oversized input.

`200 application/json`, `Cache-Control: no-store`:

```json
{
  "machine": {"handle": "mh-...", "platform": "Darwin"},
  "directory": "~/Documents/code",
  "parentDirectory": "~/Documents",
  "children": [
    {"directory": "~/Documents/code/skidbladnir", "kind": "Directory"},
    {"directory": "~/Documents/code/current", "kind": "SymbolicLink"}
  ],
  "omitted": false
}
```

`parentDirectory` is absent—never `null`—only for `~`; empty children is `[]`.
All other keys are required. Children are unique, direct, canonical, ordered,
and bounded. `omitted` maps only `Present`; no count crosses the boundary.

| New code | HTTP | Exact message |
| --- | ---: | --- |
| `DirectoryListingUnavailable` | 422 | `This directory cannot be browsed. Enter the path instead.` |
| `DirectoryListingTooLarge` | 422 | `This directory has too many folders to show. Enter the path instead.` |

The route additionally admits only existing `Unauthenticated`,
`InvalidRequest`, `RequestTooLarge`, `MachineIdentityMismatch`, and
`InternalError`. Authenticate and bind machine before parse/filesystem access.
Map unclassified failures to content-free Internal; cancellation is transport
cancellation. Encode success once before headers into a bounded buffer: over
64 KiB becomes TooLarge; otherwise write those exact bytes. Reuse machine DTO,
strict decoder, body ceiling, response/tracking primitives, and one closed route
template. Log only method/template/status/duration/error code—no new event or
count.

There is no GET/query form, alternate route, cursor, root selector, version,
compatibility schema, fallback, or directory data in `GET /v1/sessions`.

## Android architecture and composition

Decode strict wire DTOs into typed domain values. Prove machine equality,
canonical Home paths, parent/direct-child relationships, uniqueness, kinds,
omission state, and bounds; preserve server order. Same-system disagreement is
content-free `ProtocolDecodeException`. `HomeDirectory` derives basename and
hidden status; wire carries no duplicate `name`/`hidden`.

```text
ForgeState += surface: Form | DirectoryPicker(PickerState)
ForgeFailure = None | Definite(GatewayFailure.Api)
PickerState { machine, activeDirectories, exactDraft, page, history<=32, nextSequence }
Page = Places | Browsing(Load) | ExactPath
DirectoryView { listing, filter<=256 scalars, showHidden, viewport=Top|Anchor(path,offset) }
Load = Loading(sequence,candidate,retained=None|Present)
     | Loaded(view)
     | Failed(candidate,retained=None|Present,failure)
```

These are closed variants; do not replace them with nullable mode fields,
parallel booleans, or magic strings. Picker state is Forge/process memory only.

```text
Forge selection row -> Places -> Active writes cwd
                           |-> Browse Home -> GatewayClient POST
Gateway auth/machine -> workdir one-level List -> strict Android decode
-> machine/sequence acceptance -> picker content -> Use writes cwd -> Form
-> existing Create/outcome flow
```

Reuse the controller executor, foreground generation, credentials,
access-failure owner, Forge transition, strict decoder, error/tone mapping,
Nidavellir tokens, modal rules, URI keyboard, and Dashboard's typed stable-key/
offset restoration shape without sharing its state owner. Add only a
Forge-local request sequence—not a generic lane/cancellation framework. Typed
browse failure reactivates retained content. Auth/identity failure uses the
existing access transition, closes the picker, and disables Forge; stale
results cannot revive it.

## Hard cut and cleanup

- Extract oversized Forge presentation from `DashboardScreen.kt` to
  `ForgeSheet.kt`; move only Forge-owned helpers/imports.
- Add `WorkingDirectoryPickerScreen.kt`; Dashboard stays entrypoint, not owner.
- Replace `forge-cwd` with the selection row. Exact editing exists only inside
  the picker: no hidden legacy editor, flag, dual mode, or fallback.
- Replace Forge's string error with the typed definite failure above; cwd
  recovery actions branch on `ApiErrorCode`, never message text.
- Replace stale `profiles and paths` copy; delete raw-field tags/assertions.
- Move cwd ownership to `internal/workdir`; delete old normalizer/helpers/tests.
  Preserve expansion/existence/search semantics; intentionally hard-cut only
  canonical-length and display-safety behavior.
- Reuse strict DTO/error/machine/header/body/logging primitives. Add no codegen,
  generic filesystem/SFTP/repository model, dependency, or test-only seam.
- Do not generalize sheets, navigation, query lanes, content, or path types
  without a named second production consumer.
- Prove residue removal by search/static/compile checks, not source-text tests.

## Red / green / refactor and 80/20 proof

Run `./scripts/test verify` as baseline. Each builder writes and observes its
behavioral red before production edits; a minimal non-working compile skeleton
is allowed, but compilation failure is not the red.

| Ownership boundary | One owned red/green proof |
| --- | --- |
| Go workdir | Real temp tree: grammar/start validation, direct/hidden/safe Unicode rows, order, omissions, internal/absolute/external/broken symlinks, permissions, cancellation, and scan/child/path bounds; no shell/filesystem mock |
| Authenticated HTTP | Normal Gateway via `httptest`: unsupported-route red, exact request/response/errors, auth+machine before disclosure, encoded limit, no-store, content-free logs; real workdir/auth/machine, no private handler/workdir mock/tmux |
| Android protocol/state | One JVM fixture matrix: exact POST, strict decode/error set, machine mismatch, Active/filter/hidden rules, history/Parent/viewport, exact validation+retention, Cancel/machine clearing, latest sequence, retained failure, Use-to-Forge |
| Compose | With separate current-turn device approval, one semantic journey first fails on raw editor/absent chooser, then covers Places/Active/Home depth/Parent-vs-Back+viewport/filter/hidden/loading/failure/Exact/Use-vs-Create/exact 48dp/TalkBack-facing semantics and live regions/2× font-scale layout/IME; no fake navigator/private wiring |

Green in ownership order; run `./scripts/test unit` after each device-free
builder and `./scripts/test verify` integrated. Refactor only the named
workdir/Forge extractions, tighten types/mappings, remove retired owners, then
run residue search and `git diff --check`. Never weaken a red or add an
abstraction for tests.

Required gates:

- `verify`: static/build/Go/Android JVM/protocol.
- Separately approved `integration` on Darwin and Linux: existing real-tmux
  Create cwd consumer, isolated `-L` sockets only.
- Separately approved full `platform`: run the production-root six-level
  semantic journey, select, return to Forge, and never Create; evidence contains
  no path.
- Separately approved hands-on S22+ portrait/large-text/TalkBack/Nidavellir
  glance; automated semantics and font-scale assertions never substitute.
- Provider/live-host/release/full-product/second-phone/deploy/terminal/agent
  gates own no changed behavior and remain `NOT_RUN`.

Without current-turn tmux/device approval, those gates are `NOT_RUN`, never
pass. This is the 80/20 shape: four boundary proofs plus one existing tmux
consumer regression; no combinatorial suite, golden, recursive benchmark, or
live-provider journey.

## Acceptance criteria

1. On a fresh machine, Home or Active is selectable without opening a keyboard;
   no path crosses machines.
2. An unfamiliar six-level Home directory is reachable entirely by touch with
   exact machine/location/Parent context and explicit Use.
3. Current-folder filter order is deterministic; hidden folders are discoverable;
   duplicate basenames remain disambiguated by exact path.
4. Exact entry preserves every path valid under the new cwd grammar, including
   outside Home.
5. Auth, identity mismatch, malformed, unlistable, oversized, transport, empty,
   and omitted outcomes are truthful and never fall back.
6. Listing returns folders only and mutates nothing; it invokes no tmux, shell,
   agent, provider, crawler, watcher, or other gateway.
7. Root-relative internal links stay inside Home; absolute/escaping links are
   not browseable but remain available through exact entry/Create validation.
8. Back, Parent, Use, Cancel, machine/access change, and late responses preserve
   unrelated Forge fields and follow the state contract.
9. Create revalidates cwd; definite rejection preserves it; outcome-unknown is
   unchanged.
10. Content, tones, 48dp targets, bidi-safe identity/path presentation, IME,
    async TalkBack announcements, large text, and Nidavellir styling satisfy
    the closed content projection.
11. No sensitive value enters logs/evidence. Only the strict new schema exists:
    no raw-primary editor, compatibility path, fallback, duplicate validator,
    durable state, or dead code remains.

## Files and non-overlapping ownership

Work is sequential at these seams; no two builders edit one path.

| Order | Owner | Exclusive paths | Proof |
| --- | --- | --- | --- |
| 0 | Root + product/content designer | this doc; then `docs/architecture.md`, `docs/roadmap.md` | Scope/content/trade-offs/acceptance only |
| 1 | Go workdir/API | `cmd/skidbladnir/main.go`; new `internal/workdir/*`; `internal/strictjson/{decode.go,decode_test.go}`; `internal/sessions/{types.go,validation.go,manager.go,validation_test.go}` plus composition-only `activity_test.go`; `internal/gateway/{gateway.go,dto.go,dto_test.go}` + focused HTTP test; composition-only `internal/gateway/pairing_service_test.go`; `internal/logging/{logger.go,logger_test.go}`; composition-only `tests/integration/{gateway_test.go,multi_machine_test.go,provider_live_test.go,sessions_test.go,terminal_test.go}` + new `workdir_fixture_test.go` | Filesystem + authenticated HTTP; existing Create regression |
| 2 | Android protocol/state | `ProductModel.kt`, `GatewayClient.kt`, `SkidbladnirController.kt`, `SessionRename.kt`; relocation-only `LockedTerminalWebView.kt`; new `WorkingDirectoryPicker.kt`; `ProductContractTest.kt`, `MultiMachineContractTest.kt`, new `WorkingDirectoryPickerTest.kt` | JVM protocol/state |
| 3 | Compose | theme-owner/visibility-only `MainActivity.kt`; `DashboardScreen.kt`; new `ForgeSheet.kt`, `WorkingDirectoryPickerScreen.kt`, `WorkingDirectoryPickerInstrumentedTest.kt` | Real production-owned interaction/accessibility |
| 4 | Read-only verifier | none | Rules/diff/residue/exact-head/content review |

Only root changes canonical docs. Nobody changes `catalog/`, `scripts/test`,
dependencies/build files, host/deploy/release configuration, pairing/storage,
pressure, provider/hook, tmux client, cards, filters, terminal/Kill/Rename
contracts, or another owner's tests; the listed `SessionRename.kt` edit is only
closed-error exhaustiveness.

## Non-goals

Recents/favorites/pins; repository/workspace enrollment or Git discovery;
configured/multiple roots or `/`; recursive/global/content search; pagination,
prefetch, cache, watcher, index, database, preferences; cross-machine search;
folder/file create/rename/delete/upload/download/preview/metadata; SFTP,
`DocumentsProvider`, desktop handoff, voice, semantic/LLM ranking,
auto-selection, `Start another here`, aliases, telemetry, localization framework,
tablet/desktop tree, spatial navigation, new dependency, gateway-to-gateway
traffic, launch-profile inference, or any tmux/agent/release/deploy/pairing/
fleet/pressure/card/filter/terminal/Kill/Rename behavior change. The sole
pairing-adjacent change is stricter rejection of bidi-control machine labels at
the already-owned `MachineLabel` ingress; no pairing flow or storage shape
changes.
