# remove directory selection from session creation

problem: the owner previously approved removing the interactive directory
picker/search, but clarified on 2026-10-20 that unrelated product changes must
use separate prs. directory-picker removal is not required for generalization.

impact: keep existing creation controls in the generalization work. this
follow-up must not block standalone installation, config, or fleet support.

evidence: the [generalization decision record](../generalization.md) records the
earlier preference and the narrowed scope. [quick terminal](../desktop-quick-terminal.md)
already provides home-shell creation; the full creation form and its helpers
remain separate.

resolved when: a separate scoped product pr implements and verifies the agreed
picker removal, or the owner withdraws that preference. preserve plain-shell
and profile creation, explicit cli/api cwd, new-terminal-here, current-directory
display, and workspace recovery.
