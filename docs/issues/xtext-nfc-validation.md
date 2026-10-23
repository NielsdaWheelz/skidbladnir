# x/text nfc validation

problem: session objectives (`sessions/validation.go` `validateObjective`) and
profile labels (inline in `agentruntime.ResolveProfiles`) require
`norm.NFC.IsNormalString` from `golang.org/x/text` v0.41. that is not plain
UAX #15 nfc: it composes across a backward-combining starter and treats more
than 30 nonstarters as unnormalized (its stream-safe form inserts u+034f). group
labels avoid both through `group.normalizeNFC`; these two validators do not.

impact: an objective or profile label that is already plain nfc, and that the
phone's icu normalization leaves unchanged, is rejected: `ObjectiveInvalid` on
create, or a host configuration error. rare text only (balinese/javanese vowel
signs before combining marks, long mark stacks); no data is lost.

evidence: `norm.NFC.IsNormalString("sᬵ̩́")` is false and
`norm.NFC.String` turns it into `"śᬵ̩"`, while plain nfc leaves
it unchanged (`group.ParseDraft` stores it as sent); u+00e1 followed by 30
u+0301, also plain nfc, is likewise rejected.

repro: `POST /v1/sessions` with `{"kind":"terminal","cwd":"~","objective":"sᬵ̩́"}`
answers 422 `ObjectiveInvalid`.

fix: validate both with the group package's plain nfc (one exported
`IsNormal`-style predicate beside `normalizeNFC`), not x/text's whole-string
check.

resolved when: that objective is accepted and stored unchanged, and a profile
label with the same text loads.
