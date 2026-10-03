# working-directory browsing

[directory and group entry](directory-group-entry.md) owns the primary inline
field, live search, active-directory choices, literal admission and focus.
this document owns its secondary Home browser, host path grammar and bounded
listing protocol. selection edits the visible forge cwd; only the separate
create action invokes `POST /v1/sessions`, which revalidates it before tmux.
[terminal continuity](terminal-continuity.md#directory-search) owns zoxide.

## scope and final state

browse Home opens directly inside the existing forge sheet on the selected
fresh machine. the primary directory draft remains unchanged until explicit
use. browsing has one root, immediate children, parent, local filter, hidden
folders and bounded Back history. the enter-path recovery action returns to the
primary field. browse state and drafts stay in process memory.

no directory mutation, recursive search, extra inventory read, persistent
recency, crawler, watcher, index, new dependency or filesystem product exists.
Home is a navigation scope; the primary field accepts valid paths outside it.

## capability contract

1. directory actions require the selected exact fresh machine. use retained
   inventory and existing foreground/credential admission; a response belongs
   only to its captured machine, picker instance and latest sequence.
2. opening posts literal `~`. folder, parent and use consume typed server values;
   android never constructs a filesystem path. folder taps navigate; explicit
   use selects the current browse token and returns to the form.
3. filter matches basename by exact equality, prefix, substring, then ordered
   subsequence. shorter basename and server order break ties; empty input
   preserves server order. match with `Locale.ROOT`, preserving path bytes.
4. hidden means basename-leading-dot. return folders once, hide them locally
   by default and expose the toggle when hidden rows exist. filter is at most
   256 unicode scalars; history retains at most 32 decoded views, oldest evicted.
5. Back restores listing, filter, hidden state and row/offset before enabling
   actions. parent follows the returned structural parent independently of Back.
   root Back and cancel return to the unchanged primary draft.
6. loading retains the prior path/rows, separately labels its requested target,
   and disables parent/folder/use. Back from loading restores the retained
   snapshot as loaded; with none, it returns to the unchanged primary field.
   background invalidation has the same state result and requests no focus or
   search. resume neither strands loading nor retries.
7. failed browsing reactivates retained content. with none, it presents the
   requested path without pretending it opened. retry is explicit. unavailable
   and oversized listings offer enter path. transport/internal failures offer
   retry as well. Back/cancel/enter path invalidate pending replies.
8. explicit use edits only `ForgeForm.cwd`, clears a cwd rejection and returns
   to the focused primary field. group, launch, name and objective stay with the
   form. machine change closes browsing, clears cwd/profile while retaining a
   terminal choice, invalidates replies and preserves the other drafts.
9. exact cwd admission follows the primary field contract. existence,
   normalization, expanded length and search permission remain host checks.
   typed invalid/unavailable creation rejection preserves the visible draft.

## content and interaction contract

one full-height browser replaces the form within the same sheet. show Back,
cancel, the captured host, current/requested folder context and one bounded tree.
keep the context and its polite live region composed above the tree; explicit
use stays below it. current means the rows belong to that path. loading/failure
without a retained snapshot says requested and exposes no parent/folder/use.

use literal machine-specific copy: `browse home`, `parent folder`, `filter
folders`, `show hidden folders` / `hide hidden folders`, `enter path`, and
`use home` / `use this folder`. loading names the requested folder; empty says
`no visible folders here`; omissions say `some folders cannot be shown`.
existing typed failure copy and tones are retained.

folder and linked-folder semantics identify their kind and navigation effect;
use identifies the full directory and host. targets are at least 48dp. full
spoken paths retain exact values; visual machine/name values use unicode
isolation and paths use an explicitly left-to-right region with horizontal
scroll initially at the tail. tree keys use row kind plus snapshot ordinal;
exact paths remain transient action/viewport values, never saved-state keys.

## decisions and tradeoffs

- Home-only browsing keeps navigation small; other absolute paths use the
  primary field. root-relative internal links are traversable and marked;
  absolute or escaping links remain usable through literal entry and creation.
- folder navigation plus explicit use costs one extra tap and prevents a tap
  from changing cwd unexpectedly. parent and Back serve distinct structural
  and historical navigation.
- bounded listing rejects excessive folders; literal entry remains available.
  there is no partial listing presented as complete and no automatic retry.
- the modal window Back callback distinguishes browse navigation from scrim
  dismissal. its lifetime belongs to the composed modal, with no navigator.

## host architecture

`internal/workdir` is the narrow semantic service and sole owner of Home,
cwd parsing/normalization, start validation, browse containment/projection,
ordering, and bounds:

```text
opaque WorkingDirectoryCandidate, WorkingDirectory, HomeDirectory, ParentDirectory
Listing { directory, parent=None|Some, children: Entry[], omissions=None|Present }
Entry { directory, kind=Directory|SymbolicLink }
ErrorCode = Invalid | Unavailable | TooLarge       # no path or raw OS text

New(home, zoxidePath) -> Service
Service.ParseCandidate(string) -> WorkingDirectoryCandidate
Service.ParseBrowseDirectory(string) -> HomeDirectory
Service.ValidateStart(candidate) -> WorkingDirectory
Service.List(context, HomeDirectory) -> Listing
```

- Opaque values have package-private representation/construction.
- `New`: absolute, clean, searchable service-UID Home satisfying cwd grammar;
  zoxide is an absolute clean path, or empty when disabled.
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
- Return hidden folders; Android owns visibility. sort by ascii-folded basename,
  then exact utf-8 basename. local filtering preserves server order for ties.
- Read fixed batches and check cancellation between them. Exceeding any scan,
  child, or path-text bound returns `TooLarge` with no partial listing.
- Requested-directory read/search/root-resolution failure is `Unavailable`;
  malformed input is `Invalid`; cancellation returns the context error.
- A listing is advisory; Create always revalidates. No cache, watcher, retry,
  goroutine, persistent descriptor, mutation, or telemetry exists.

`main` creates one immutable, concurrency-safe concrete `*workdir.Service` and
passes it to `sessions.Manager` and `gateway.Gateway`; no interface wraps it.
sessions maps Invalid/Unavailable to existing Create errors; listing maps Invalid
to `InvalidRequest`. cwd stays typed through the exact tmux `-c` adapter. other
session validation remains in `sessions`.

## http api

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
compatibility schema, fallback, or listing data in `GET /v1/sessions`.

## android ownership and qualification

`WorkingDirectoryPicker.kt` owns the load-only state and pure browse transitions;
`SkidbladnirController.kt` owns requests and exact reply admission.
`WorkingDirectoryPickerScreen.kt` maps that state to browser content and keeps
viewport restoration with the existing typed anchors. new listing snapshots and
filter/hidden changes restore before scroll capture and actions resume; viewport
updates retain their listing snapshot and do not restart restoration. explicit controller
return values tell the form when to focus directory; background changes emit
no focus action. the primary field and inline request state have separate owners
under [directory and group entry](directory-group-entry.md).

the earlier chooser's Places/Search/ExactPath pages, private exact draft,
`z <words>` entrance and redundant callbacks are removed. prior chooser evidence
is historical and does not qualify this replacement; current changed-boundary
evidence belongs to [directory and group entry](directory-group-entry.md#qualification).
[testing policy](rules/testing.md) governs temporary tests and their removal.
release and production installation remain separate work.
