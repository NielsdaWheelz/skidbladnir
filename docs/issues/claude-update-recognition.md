# running claude loses recognition after a native update

problem: foreground recognition compares the running executable with the current
target of the configured launch symlink. claude updates move that symlink while
existing sessions keep their older executable. a valid running provider then
becomes an ordinary terminal in skid. refreshing or restarting the gateway
repeats the same comparison; restarting claude happens to select the new image.

impact: provider recognition, registered profile/conversation projection,
terminal status, footer and usage eligibility disappear together. the provider
and terminal remain alive. usage recovery still requires a supported visible
usage display; process recognition alone cannot establish quota values.

evidence, 2026-10-15:

- authorized read-only macbook metadata inspection found a foreground claude
  image at native version `2.1.296`; the configured launch symlink resolved to
  `2.1.300`. both files existed. native `proc_pidpath` and the process's text
  mapping agreed on the older image.
- native process metadata confirmed the foreground group, pane terminal, and
  registration's pid/kernel-start identity still matched. registration loss and
  an unlinked image were not this incident's cause. no terminal text was read.
- the active gateway was installed `v0.16.0`, source
  `d8c3c5574569ddf6d4c43747f72961dc14be0eb1`. that source and baseline `b253c05` share
  the failing predicate in
  [runtime.go](../../internal/agentruntime/runtime.go), `matchesSignature`.
- `Project` rejects the provider before accepting registration;
  [terminal sampling](../../internal/agentcontrol/terminal.go) then skips capture,
  and [usage selection](../../internal/sessionui/usage.go) excludes the session.
- this is an explicit accepted limitation in
  [terminal observation](../terminal-observation.md), its
  [qualification](../terminal-agent-control-qualification.md), and the
  [deployment contract](../dev-server-handoff.md). the accepted correction now
  supersedes that continuity tradeoff; source qualification now passes.

reproduction: with an approved isolated tmux socket, run native image a through
a configured symlink, move the symlink to image b without stopping a, and sample
recognition. a becomes unrecognized; a fresh b is recognized. restart the isolated
gateway to demonstrate that refreshing observation does not repair the mismatch.

the accepted [spec and plan](../claude-update-recognition.md) replaces only the
mutable launch-path predicate with direct-child matching in the configured
canonical native versions directory. foreground leader selection, codex basename
matching, registration, lifetime checks, controls, parsers and clients remain
unchanged. [source qualification](../claude-update-recognition.md#qualification)
passes; release and installed continuity remain pending. wrapper discovery is
tracked [separately](provider-wrapper-recognition.md).

resolved when old and new supported images remain recognizable concurrently,
including direct launches without hooks and discovery after a cold gateway
restart; an already-running affected process recovers on the next inventory;
status/usage capture becomes eligible again; shell transitions, images outside
the configured provider namespace, ambiguous providers and stale registrations
retain honest outcomes. qualify the
affected darwin/linux boundaries and installed-session continuity separately.

boundary still requiring separate evidence: removal of a live image is distinct
from a symlink update. [darwin process observation](../../internal/process/process_darwin.go)
currently rejects `proc_pidpath` returning `ENOENT`; an installation-directory
predicate cannot restore missing image evidence. do not claim deleted-image
support from the relink fix. the existing
[conversation-switch issue](claude-resume-identity.md) is also independent.
