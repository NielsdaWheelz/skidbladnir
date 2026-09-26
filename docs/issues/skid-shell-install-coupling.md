# skid must own its shell prerequisite

problem: dev-server's shared provider installer installs skid shell startup
blocks even without skid. a symlinked startup file can fail unrelated provider
maintenance. ownership belongs to skid apply, not shared provider installation.

source status: skid's guarded shell template selects existing accounts and
returns unchanged in ordinary/herdr shells. the deployment handoff requires
skid-owned startup installation preserving symlink nodes and existing contents.
dev-server owns the installer fix; original skid owns no host installer here.

resolved when: reviewed dev-server source removes skid startup edits/validation
from shared provider maintenance and skid installs its own startup blocks.
the fully managed zshrc may retain an inert optional source guarded before file
access in ordinary/herdr shells, so ordinary dotfile reapply preserves support.
disposable bash/zsh fixtures must cover direct and symlinked startup files,
preserved user contents, and late login-file overrides. require the deployment
owner's acknowledgment. the static zsh integration is an explicit trade-off;
ordinary maintenance must not require an installed skid generation.
