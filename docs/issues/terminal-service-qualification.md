# terminal service native qualification is incomplete

problem: the independent terminal runtime and signed identity need qualification at
native service and mac privacy boundaries. the owner permits disposable docker
verification and directs unavailable native cases to be `NOT_RUN`.

impact: isolated process tests do not establish launchd crash cleanup, mac permission
attribution or consent persistence. arm64 docker does not qualify supported linux
amd64 installation. release signature changes need the existing signing credential
to be usable without an unresolved keychain prompt.

evidence: private tmux probes reproduced unintended sessions with raw `tmux -D` and
verified collision refusal using `-D -c 'exit 78'`, including concurrent starters.
the latter can run the existing default shell's initialization on collision; it
does not promise arbitrary shell code has no effects. isolated host termination,
unexpected completion and bounded kill/reap pass. native arm64 docker proves basic
systemd pid ownership and exit-78 restart suppression. emulated amd64 systemd's
dbus/logind/user services failed; its acceptance is `NOT_RUN`.

temporary native arm64 integration cases also passed: actual terminal-exec creation,
empty inventory, unavailable/absent/unmanaged http503, lost completion with one
creation and unknown delivery, source-loss fencing and managed restoration. public
uninstall refused occupied work, valid nonempty or unreadable checkpoints and new
work during shutdown; clean removal preserved checkpoint data. these qualify the
observed boundaries on that container, not supported-host installation or mac privacy.
generated gateway-job readiness, orderly/abrupt gateway restart survival, separate
cgroups and native tmux reattachment also passed there. browser reattachment, input
replay, real provider access and the full signed installation/update journeys remain
`NOT_RUN`.

resolved when: the implementation contract's native ownership, crash cleanup,
signer/requirement and prior permission grant/denial continuity, clean installation,
update/removal and recovery cases pass on supported hosts. the macbook energy
budget remains tracked in [exec overhead](tmux-exec-overhead.md); no uncontrolled
sample closes it. use disposable content; native tests require current-turn approval
under [AGENTS.md](../../AGENTS.md).
