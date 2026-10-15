# terminal model and effort display

2026-10-15: source implemented and [qualified](#qualification) at `6974851`.
[session rows](desktop-session-rows.md) owns layout, exclusive file ownership
and delivery. this contract owns optional terminal text and its meaning.

## capability and schema

show what the sampled terminal displays. claude supplies its identifiable
model/effort phrase; codex supplies its whole native status line because its
configurable, unlabelled items cannot reliably identify a model field. preserve
native spelling, order and service-tier wording. neither source establishes
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
  -> reading/detection -> Enrich -> session dto -> fleetclient / android ingress
  -> desktop row and info
```

extend `reading` and `detection` with the optional value; `Enrich` assigns it
beside status/usage from the same sample. reuse existing deadline, capture and
foreground revalidation. no additional capture, provider request, polling,
config/history read, launch-default substitution or publisher. extraction cannot
change activity, interaction, notice, composer, diagnostic rules or attention.
an intact footer may coexist with unrelated unknown/clipped status evidence.

the provider grammar documents own the exact [codex](terminal-observation-codex.md#8-displayed-footer-extension)
and [claude](terminal-observation-claude.md#7-displayed-footer-extension)
productions. inspect only their already anchored ordinary footer. no transcript,
scrollback, picker selection, wrapped-fragment joining or model-name catalogue.
unsupported or ambiguous evidence omits the field without failing inventory.

`terminal/inspect` stays status-only. normal `info` retains inventory metadata;
`info --explain` clears this field beside `TerminalUsage` when substituting the
inspection's status/time. desktop creation-result insertion also clears both
before attaching the new session to an older peer receipt; the next inventory
supplies them. actual create/shell responses still transport their own sample.

gateway mapping, go host/session decoding and projection, and android
`WireTmuxSession`/`TmuxSession`, `requireSessionOptionalFields` and `acceptSession`
must agree for list/create/shell. malformed wire evidence fails the existing
strict response boundary; it is not silently sanitized. keep unknown-field
rejection. no new endpoint, native helper, execution-context field or phone view.
structured cli output carries the shared field; human cli formatting is unchanged.

## content contract

the observation content designer owns these labels and reviews their actual
rendering. good content distinguishes a visible string from an authoritative
setting and explains missing evidence without guessing its cause.

| context | row | info |
| --- | --- | --- |
| fresh local codex with footer | native status line | `native status line`: full accepted text |
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
rows; fresh absence adds `the footer may be hidden, clipped, ambiguous or unsupported.` codex
adds: `includes other status-line items; model and effort appear only when shown
there. native truncation is preserved.` claude adds: `native wording may include
service tier.` no invented absence-reason field. the row uses a single ellipsis
within its 30-cell budget; info wraps the full accepted value in its scrolling
page. neither path interprets text as terminal escapes.

## acceptance and costs

| boundary | required proof |
| --- | --- |
| retained provider frames | codex whole anchored line preserved, including internal spacing, reordered items, aliases, tier/run-state/badges and native ellipsis; no guessed model extraction; unsupported one-row footer omits |
| claude frames | intact managed phrase with/without effort or quota; aliases preserved; wrong/missing boundaries, duplicate candidate, custom/wrapped/clipped suffix omit; quota and footer detection remain independent |
| isolation | transcript/picker decoys never substitute; missing row, invalid encoding/value, failed capture and changed process/pane omit; unrelated clipped transcript can retain intact footer; existing status/composer/rule outputs unchanged |
| transport | real gateway serialization → go decoder and actual android decoder for list/create/shell; absent/valid values accepted exactly; null, nonstring, empty, oversized, controls and invalid agent/remote/status attachments rejected |
| replacement/display | two sessions sharing an account remain distinct; present→absent clears; stale/checking suppress; creation insertion and explained inspection clear; normal info/structured output retain; row truncates, info preserves |
| live source | scoped installed-provider checks in the row plan prove actual surfaces and interactive changes; authored frames or injected gateway responses alone never qualify provider behavior |

hard cut all shared producers/consumers together. older strict readers may reject
the new field; coordinated go/android deployment is required. optional absence
means missing observation, not a version-negotiation or fallback path. release
and installed-fleet qualification are separate from source completion.

codex may duplicate status and include directories or other configured items;
its model may be absent or beyond the compact column. a draft can hide whether
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
- [desktop qualification](desktop-session-rows.md#qualification), including
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
repository probe deletion. release and coordinated deployment were not performed.
