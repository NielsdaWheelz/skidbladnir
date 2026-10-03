# raw launch options and shared provider behavior

recorded 2026-10-02; medium priority; [option ownership](../jarvis-orchestration.md#options-and-defaults).

source status: lexical bounds, literal provider argv and rejection of competing
profile model/effort arguments are implemented. temporary argv/TOML/ingress probes
and engineering checks pass. isolated darwin stock codex/claude launches qualify
omitted/model-only/effort-only/combined lowering and literal input. codex native
status confirms effective model and explicit low effort on the shared-daemon path;
claude renders the selected model/effort. linux stock providers pass omitted and
explicit model/effort launches and literal initial input on arch.
exact installed paired artifacts/fleet remain `NOT_RUN`, keeping this record open.

baseline problem: the source requires raw model/effort overrides and compatible defaults,
but leaves their validation and provider lowering unspecified. baseline
profiles carry an ordered argument bag; simply appending overrides can create
competing values. stock remote-new codex behavior must also be qualified.

impact: the wrong effective model/effort can run, unsupported combinations may
fail after terminal creation, or a launch change may undermine shared-runtime
ownership. provider-only selection is ambiguous across three codex accounts.

baseline evidence at skid `79679b4`, before the local changes:
[profile arguments](../../internal/agentruntime/profile.go),
[launch copying](../../internal/agentruntime/launch.go) lines 20–24 and
[codex argument construction](../../internal/sessions/manager.go) lines 234–237.
[stock connection](../native-agent-observation.md#2-composition-and-stock-connection)
requires the account's owning daemon and a normal remote-new terminal.

research 2026-10-02: [codex docs](https://learn.chatgpt.com/docs/config-file/config-advanced)
support literal `--model` plus a toml-encoded fixed configuration override;
[claude docs](https://code.claude.com/docs/en/cli-reference) support model and effort
flags. these establish candidate lowering, not installed remote-mode acceptance.
an arbitrary caller-supplied config key or expression is not a raw effort value.

counterexample: a profile supplies model/effort flags and start appends a second
value; provider parsing owns the accidental precedence. another: explicit model
inherits an incompatible effort value from the native account configuration.

owner decision 2026-10-02: explicit launch overrides, otherwise native account
defaults. no added skid profile-default layer; amend the source o1 accordingly.

selected contract: each explicit value is nonempty valid utf-8, at most 256 bytes,
with no whitespace or controls. reject rather than trim; encode codex effort as
one string under the fixed configuration key. omitted fields remain native-owned.

resolved when: the selected field semantics and bounds are implemented;
competing model/effort settings in skid profile arguments reject;
provider lowering uses fixed argv with no shell/config-expression interpolation.
qualify effective per-launch selection on installed providers, especially codex's
shared remote owner, preserving permissions/accounts/plugin. distinguish known
provider incompatibility from a later account/model rejection. no hidden model
substitution, duplicate model catalogue or private daemon fallback.
