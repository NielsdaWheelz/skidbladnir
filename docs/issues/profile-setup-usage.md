# profile setup and phone account visibility

status: usability proposal from the [workspace review](workspace-coherence.md).

problem: multiple native profile commands already work and desktop usage already
exists, but configuring those profiles and seeing account evidence on the phone
remain separate burdens. a launch profile is not proof of the foreground account.

impact: choosing a subscription requires remembering commands, host setup and
account attribution. hiding uncertainty could cause the wrong account to be used
or suggest allowance that was never observed.

evidence: `internal/hostconfig/config.go`, `internal/agentruntime/profile.go`
(`ResolveProfiles`, `MatchProfile`, `AccountScope`), `internal/sessions/create.go`,
`internal/quota/quota.go`, `internal/gateway/usage.go`,
`internal/sessionui/usage.go` and phone `ForgeSheet.kt`. phone source had no native
quota/usage consumer in the reviewed baseline. the
[account contract](../architecture.md#account-quota-and-usage) requires one profile
key to mean one account across hosts; `accountScope` hashes provider plus canonical
home and is not verified provider account identity.

## recommended direction

provide a guided setup/check journey using named native profiles. show the profile,
execution machine and relevant allowance before launch, with observed age/source or
unknown state. remember explicit choices visibly. reuse the existing usage owner
for a compact phone disclosure rather than invent a phone quota collector.

distinguish configuration present, command availability, observed foreground
attribution and provider authentication. a badge proves only its defined check.
aliases/functions are valid commands: a filename check is not a sufficient
availability check, and arbitrary profile commands must not be executed as silent
preflight. adding provider-specific authentication checks is a separate explicit
capability; the current configuration contract has no provider preflight.

keep credentials and provider configuration on their host. normal shell startup,
aliases, functions and user-owned permissions remain effective. the gateway must
not silently inject flags, select a different account, start a quota daemon or
rewrite human yaml. use the existing configuration acceptance path; any settings
editor or new host-specific settings must be scoped separately.

usage is read-only evidence, never launch admission or readiness. preserve native
versus terminal estimates, stale/missing state and whole-source selection. do not
sum subscriptions, splice windows or treat matching home paths as matching real
accounts. account mapping needs explicit owner setup unless an attributable
provider identity source is separately adopted.

## competitor evidence and trade-offs

t3's claude instances use distinct configuration directories; an existing thread
can switch only among instances sharing its directory. codex's separate auth
overlay over shared conversation/configuration storage is a provider-specific
filesystem integration, not a general account portability primitive
([released claude guide](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/user/providers-claude.md),
[codex layout](https://github.com/pingdotgg/t3code/blob/6c9adfb2739205244b84e607736f6fd8b4611e69/apps/server/src/provider/Drivers/CodexHomeLayout.ts#L11)).

steal clear account selection and isolation. reject automatic rotation, live
subscription failover and promised conversation migration. cost: explicit setup
and slightly more context before launch; benefit: attributable choices without
owning provider credentials. native protocol or filesystem integrations add ongoing
provider-version maintenance and need justification beyond a nicer picker.

resolved when: the chosen setup/check contract is explicit; two profiles can be
configured and selected on the intended hosts; phone and desktop report attributable
usage or honest absence; a configured launch label never masquerades as observed
account identity. provider/account checks remain `NOT_RUN` until exercised at the
authorized boundary, and no quota observation changes execution policy.
