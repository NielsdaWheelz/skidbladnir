# repeated launches require reconstructing work locations

status: product/configuration proposal from the
[workspace review](workspace-coherence.md); requires explicit scope acceptance.

problem: the phone's forge opens with no machine or profile. the owner repeatedly
selects machine, launch command and directory for familiar work. the current
configuration has profiles and a desktop default machine, not a persistent project
or launch preset concept.

impact: starting work requires remembering infrastructure. a project spanning
computers has no explicit mapping between its identity and host-local checkouts.

evidence: `SkidbladnirController.kt:400-406` (`openForge`),
`ForgeSheet.kt:118-161` (machine/profile selection), `internal/hostconfig/config.go`,
and [configuration](../architecture.md#5-configuration). machine changes currently
clear dependent launch/directory choices. existing groups are observed labels in
tmux, not durable project records.

## recommended direction

add explicit named launch presets to the existing accepted-configuration owner.
a preset selects a work location, machine, profile and group, and prefills a visible
draft. the final action names its execution machine and directory. retain an
ordinary shell and manual launch path. keep commands opaque and run them once
after normal shell startup, without inserted account, permission or model flags.

if one project genuinely spans machines, give the project a stable key with an
explicit host-to-directory mapping. a directory is a checkout/location, not project
identity. same repository url, folder name or branch does not prove identical
contents. changing the machine must resolve its configured location or ask for one;
never reuse another machine's absolute path silently.

start with the smallest useful preset concept. a project registry is justified
only by actual multi-location or retained-context requirements. do not create both
as independent stores of the same choices. do not turn ephemeral group labels into
project identifiers, infer git relationships, synchronize files or introduce
worktree automation under this issue.

the current hub owns accepted configuration and never edits human yaml; clients
cannot upload settings and phones receive no settings collection. choose how
presets become client-visible and how the owner edits them explicitly. do not
quietly add a settings writer or a second configuration authority. host removal,
renaming, missing profiles and invalid directories need visible outcomes; invalid
settings retain the last accepted configuration.

## trade-offs and rejected alternatives

cost: a small configuration concept, location maintenance and client presentation.
benefit: one familiar choice replaces repeated administrative recollection.
remembered choices must remain visible and deliberately overridable.

reject automatic placement/load balancing initially. at three or four machines,
an intelligible default is preferable to silently changing account, checkout or
environment. t3's optional balancing holds a selected draft stable and never moves
an existing thread; it is not evidence of transparent migration
([released remote guide](https://github.com/pingdotgg/t3code/blob/bf469bb19669671d0df2efda74ed73867a7137c0/docs/user/remote-access.md#balance-new-threads-across-machines)).

owners: `internal/hostconfig` and fleet acceptance/distribution for configuration;
existing forge/create forms for draft selection; sessions/workdir for host-side
validation and launch. [search](fleet-search.md) and
[human work notes](work-context-notes.md) must reuse any selected project identity.

resolved when: the owner selects the minimal model and configuration path; a
familiar launch is visibly targeted and economical on desktop and phone; different
host paths, offline/removed hosts, changed profiles and invalid locations behave
explicitly; a lost creation answer is never replayed. no claim of file or
conversation portability follows.
