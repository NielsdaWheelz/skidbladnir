# phone session-preservation uncertainty

problem: one unrelated source session lifetime in the hashed baseline was
absent after the extended physical-phone test window. its cause is unknown.

impact: the named phone journey passed, but preservation of every unrelated
session across that window cannot be claimed.

evidence: the installed phone apk matched the published signed `v0.10.3`
asset. the phone created a non-home terminal from `z` search, changed local and
remote cwd, entered mosh to devbox, observed remote provider and configured
home profile, used `/exit` to retain the remote shell, replaced the provider,
detached/reopened, and displayed source and destination labels. the remote
header opened a source-scoped terminal form; source-only close removed the
exact test session. the three named phone test sessions present in the final
window were closed through their exact phone dialogs. a baseline of 12
unrelated hashed session ids had 11 survivors afterward. no direct tmux
command targeted the production
socket. the missing lifetime was not identified, so causation is unproven.

resolved when: attribute the missing lifetime or repeat preservation proof on
a quiescent baseline with all unrelated lifetimes retained. remove this record
then.
