# skid must own its shell prerequisite

problem: dev-server's shared provider installer installs skid shell startup
blocks even without skid. a symlinked startup file can fail unrelated provider
maintenance. ownership belongs to skid apply, not shared provider installation.

source status: skid's guarded shell template selects existing accounts and
returns unchanged in ordinary/herdr shells. the deployment handoff requires
skid-owned startup installation preserving symlink nodes and existing contents.
dev-server owns the installer fix; original skid owns no host installer here.

resolved when: reviewed dev-server source removes shared provider and ordinary
managed shell dependencies on skid, installs startup blocks only with skid,
and disposable bash/zsh fixtures prove direct and symlinked startup files work
without losing user contents. require the deployment owner's acknowledgment.
