# terminal model and effort display

2026-10-15: preceding source [qualified](#qualification) at `6974851`.
the subsequent owner-requested refinement parses codex's model/effort phrase
instead of copying its entire bar. [current qualification](desktop-browser-simplification.md#location-refinement-qualification)
records 452 passing frames and the unchanged installed native footer, including
interactive effort changes. [browser simplification](desktop-browser-simplification.md)
owns current layout and delivery. this contract owns optional terminal text and
its meaning. [v0.17.2 installed qualification](desktop-browser-simplification.md#location-refinement-installed-release)
records fleet/phone deployment and owner acceptance of the revised rows.

the preceding source shipped in v0.16.0; [installed qualification](workspace-checkpoint-reliability.md#installed-qualification)
verifies the matching fleet and android generation. production cloud inference
and per-request settings remain outside that deployment check.

## capability and schema

show the recognized model/effort phrase from the sampled terminal. claude keeps
its managed phrase. codex recognizes one complete item in its anchored footer
using the bounded grammar below. preserve native spelling and supported
service-tier wording. neither source establishes
effective per-inference settings or that the current process painted every cell.

extend only the session dto in `GET /v1/sessions`, `POST /v1/sessions` and
`POST /v1/sessions/{id}/shell` responses; keep their existing envelopes:

```text
Session += { terminalFooter?: string }
```

one optional string; no split model/effort, enum, source tag, clock or stored
record. the existing agent provider defines its interpretation. `terminalContext`
already owns remote context and must not acquire this second meaning.

| contract | requirement |
| --- | --- |
| value | 1–1,024 utf-8 bytes; valid unicode scalar text; exclude `Cc`, `Cf`, `Zl`, `Zp` as existing `attention.SingleLine` does, and leading/trailing whitespace; preserve internal spacing; no normalization |
| absence | omit the field; reject explicit null, empty/whitespace-only, wrong type or oversized wire values; invalid captured text omits successfully |
| attachment | detected local codex/claude agent, no connection; terminal status source `terminal`, reason `recognized`, `partial`, `layout_unknown`, `evidence_clipped` or `evidence_conflict` |
| replacement | each sample replaces the entire value, including absence; failed/changed-target capture clears it; no earlier-value carry-forward |
| identity/time | containing session lifetime, pane/process checks and observation envelope; no independent cache or timestamp |

define a small named string `sessions.TerminalFooter` in `sessions/footer.go`,
with value/admission validation shared by go producers and consumers. optional
session fields use `*TerminalFooter`; go wire fields use `omitempty`. reject
explicit null at the enclosing strict ingress, where pointer unmarshalling alone
cannot distinguish it from omission. android mirrors this scalar contract with
`String?`, required-string/type checks when present and the same attachment rules.
reuse the successful-sample reason predicate with `TerminalUsage` if extraction
removes duplication; do not broaden either capability's provider rules.

## observation composition

```text
existing exact-target ObservePane -> parseScreen -> provider footer extraction
  -> reading/detection -> inventory enrichment -> session dto -> fleetclient / android ingress
  -> desktop row and info
```

extend `reading` and `detection` with the optional value; inventory enrichment
(`agentcontrol.Inventory`) assigns it
beside status/usage from the same sample. reuse existing deadline, capture and
foreground revalidation. no additional capture, provider request, polling,
config/history read, launch-default substitution or publisher. extraction cannot
change activity, interaction, notice, composer, diagnostic rules or attention.
an intact footer may coexist with unrelated unknown/clipped status evidence.

the provider grammar documents own the exact [codex](terminal-observation-codex.md#8-displayed-footer-extension)
and [claude](terminal-observation-claude.md#7-displayed-footer-extension)
productions. inspect only their already anchored ordinary footer. no transcript,
scrollback, picker selection, wrapped-fragment joining or runtime model catalogue.
unsupported or ambiguous evidence omits the field without failing inventory.

codex's supported phrase is one ` · `-delimited item: a case-insensitive `gpt-` model
identifier with ascii alphanumeric components separated by `-` or `.`, or
`codex-auto-review` / `Codex Auto Review`; one space; effort `default`, `minimal`, `low`, `medium`,
`high`, `xhigh`, `max`, `ultra` or `persistent`; optional literal ` Fast` suffix.
require exactly one complete match and preserve the observed case. item order
and colors do not matter.
reject clipped candidates and duplicate matches; a complete candidate may survive
truncation of a later unrelated item. unrelated run state, directory and quota
items never enter the value. no independent model/effort columns are required.
the final item's right-edge check uses the existing conservative byte-bound
approach: preceding unicode can cause omission despite sufficient display cells.
an explicit following item separator establishes completion independently.

this is deliberately bounded text inference, authorized by the owner. custom
aliases, custom effort/tier names, separate model/effort items and incomplete
phrases remain unsupported. an unrelated item that exactly imitates this grammar
can be mistaken for a model phrase; terminal text supplies no semantic item ids.
these values are display evidence only, never identity or control admission.

`terminal/inspect` stays status-only. normal `info` retains inventory metadata;
`info --explain` clears this field beside `TerminalUsage` when substituting the
inspection's status/time. desktop creation-result insertion also clears both
before attaching the new session to an older peer receipt; the next inventory
supplies them. actual create/shell responses still transport their own sample.

gateway mapping, go host/session decoding and projection, and android
`TmuxSession` (`Sessions.kt`) must agree for list/create/shell. go clients keep
their strict response boundary; the phone decodes tolerantly
([rules](rules.md)). no new endpoint, native helper, execution-context field or
phone view.
structured cli output carries the shared field; human cli formatting is unchanged.

## content contract

the observation content designer owns these labels and reviews their actual
rendering. good content distinguishes a visible string from an authoritative
setting and explains missing evidence without guessing its cause.

| context | row | info |
| --- | --- | --- |
| fresh local codex with footer | recognized model/effort phrase | `displayed model / effort`: full accepted phrase |
| fresh local claude with footer | native model/effort phrase | `displayed model / effort`: full accepted phrase |
| detected agent without usable observation | `—` | provider label with `—` and explanation below |
| plain terminal | blank | omit this fact |
| known remote agent | `—` | provider-specific label with `—`; `remote terminal text is not observed` |
| unknown remote context | blank | existing `remote unknown`; no remembered footer |

unavailable/checking local rows suppress retained footer text and show `—` when
an agent remains known. info does likewise, retaining existing stale/checking
context. a pending refresh may retain the latest admitted value under existing
freshness rules; there is no new expiry timer.

local info explanation: `text from the sampled terminal. — means no usable
observation is available.` existing freshness context explains stale/checking
rows; fresh absence adds `the footer may be hidden, clipped, ambiguous or unsupported.`
both add: `native wording may include service tier.` no invented absence-reason
field. the row uses a single ellipsis
within its 30-cell budget; info wraps the full accepted value in its scrolling
page. neither path interprets text as terminal escapes.

## acceptance and costs

| boundary | required proof |
| --- | --- |
| retained provider frames | codex extracts only one complete supported phrase, including a composer-anchored one-row footer; reordered items/colors, default and effort variants, optional Fast, unrelated truncation pass; aliases, duplicates, candidate truncation and shortcut-only rows omit; existing status/composer/rules stay unchanged |
| claude frames | intact managed phrase with/without effort or quota; aliases preserved; wrong/missing boundaries, duplicate candidate, custom/wrapped/clipped suffix omit; quota and footer detection remain independent |
| isolation | transcript/picker decoys never substitute; missing row, invalid encoding/value, failed capture and changed process/pane omit; unrelated clipped transcript can retain intact footer; existing status/composer/rule outputs unchanged |
| transport | real gateway serialization → go decoder and actual android decoder for list/create/shell; absent/valid values accepted exactly; null, nonstring, empty, oversized, controls and invalid agent/remote/status attachments rejected |
| replacement/display | two sessions sharing an account remain distinct; present→absent clears; stale/checking suppress; creation insertion and explained inspection clear; normal info/structured output retain; row truncates, info preserves |
| live source | scoped installed-provider checks in the row plan prove actual surfaces and interactive changes; authored frames or injected gateway responses alone never qualify provider behavior |

the original field introduction required a coordinated go/android cutover because
older strict readers rejected it. the current refinement reuses that scalar and
schema; it requires no android change. optional absence means missing observation,
not version negotiation or a fallback. release and installed-fleet qualification
remain separate from source completion.

codex's model/effort may be absent or clipped in its native bar. a draft can hide whether
its visible footer belongs to the main or a side conversation. claude supports the
managed formatter's ordinary footer only; other layouts show a dash. its
conservative width test can omit an intact unicode footer. values over 1,024
bytes or containing format controls omit, keeping accepted text unchanged by
the existing single-line renderer. terminal repaint is not atomic, footer contents can lag, and
visible cells cannot prove per-request execution settings. these are accepted
limits of reading existing terminal evidence without creating another state
owner. no structured provider getter, hook, managed-config change, remote
attribution, model selector or effort normalization is included.

## qualification

2026-10-15: source `6974851`, branch `codex/terminal-model-effort`, from `16a6ed9`.
the compiling baseline fails the requested extraction, strict ingress and display
behavior. current source passes:

- 421 retained authored frames, including 81 explicit footer assertions: 31
  present and 50 absent. the same corpus against original production fails the
  31 positives; the original 388 captures and status/composer/rule/usage
  expectations are unchanged. L22 explicitly injects an unsupported writer
  sequence after capture; it proves parser ambiguity handling only.
- actual gateway list/create/shell serialization through go and the actual
  android jvm decoder: 306 shared cases, including 18 gateway-serialized samples.
  absence and exact unicode/spacing survive; malformed scalars, null, controls,
  sizes and attachments reject. raw invalid utf-8 also rejects on all three go
  routes. normal info retains metadata; explained inspection and desktop
  creation insertion clear both footer and usage.
- [desktop qualification](https://github.com/NielsdaWheelz/skidbladnir/blob/12b15163d9faf529416d486e0156b8b83821fa4a/docs/desktop-session-rows.md#qualification), including
  stale/checking suppression, present-to-absent replacement and full info text.
- installed native codex 0.162.1 and claude 2.1.300 on macos 26.4.1 arm64/tmux
  3.8 and ubuntu 24.04 arm64 in a disposable container. linux used packaged
  tmux; its exact version was not recorded. public sessions observation and
  enrichment preserve exact native text, distinguish two shared-home sessions,
  follow interactive model/effort changes and omit pickers/hidden footers.
  codex's model-free custom items and native ellipsis remain exact; unsupported
  claude formats and clipped phrases omit.
  retained nonempty samples clear after canceled capture, native process
  replacement and active-pane change on both platforms; existing reason/source
  distinctions remain intact.

provider checks used temporary shared homes, a loopback api mock and direct
native executables, including codex's no-daemon mode. they sent no inference
request and copied no account credentials. this qualifies installed rendering
and interactive settings; account authentication, configured-daemon composition,
real-service inference and deployed-fleet qualification remain separate.

both designers and independent adversarial review pass. every recorded native
pid exited; owned sessions, mock servers, container/image, disposable probes and
artifacts were removed. only the retained provider corpus/replay remains.
`go test ./internal/agentcontrol` and `scripts/check verify all` pass after
repository probe deletion. release and coordinated deployment were not part of
this isolated qualification; the installed record above owns those later checks.
