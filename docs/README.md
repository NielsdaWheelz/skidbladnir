# Documentation

The [README](../README.md) says what Skíðblaðnir is and whether it suits you. The
pages below either help you use skid or explain how it is built.

## Using skid

- [Install](install.md): installing skid on macOS and Linux, joining computers into
  one fleet, configuring the fleet, updating, uninstalling and troubleshooting.
- [Phone](phone.md): checking the Android app's signer, installing it by sideloading
  or through Obtainium, pairing it, updating it and setting up optional push
  notifications.
- `skid --help`: every command, the session browser's keys, and the automation
  guide's targets, output and exit codes; `skid help COMMAND` explains one command.
- [Configuration](architecture.md#5-configuration): `config.yaml` on the hub, its
  fields and how the hub accepts them.

## How it is built

- [Architecture](architecture.md): the contract, from five constraints down to
  concepts, trust, interfaces, invariants, owners and error semantics.
- [Codebase map](codebase-map.md): where each responsibility lives, after a reading
  path of eight files that carry the system's ideas.
- [History](history.md): a timeline from the first commit, and the decisions that
  shaped the system, each with its constraint and the cost accepted.
- [Design language](design-language.md): Niðavellir, the visual identity: color,
  shape, ornament, type, motion, and a primary source for every motif.
- [Rules](rules.md): how code is written and proven: trust boundaries, errors,
  identities, retries, concurrency, timing, code shape and testing.
- [Terminal observation](terminal-observation.md): what a terminal's status means
  and how it is inferred from the screen, with the
  [Codex](terminal-observation-codex.md) and
  [Claude Code](terminal-observation-claude.md) screen grammars.
- [Workspace recovery](session-recovery.md): how a computer rebuilds lost tmux
  sessions from a private checkpoint and requests native conversation resumes.
- [tmux exec budget](tmux-exec-budget.md): the bound on tmux commands per inventory,
  and the service's scheduling priority on macOS.
- [Roadmap](roadmap.md): what is next and what is deferred.
- [Issues](issues): open problems, one per file, each with its evidence and what
  proves it resolved.

Vulnerabilities go to the [security policy](../SECURITY.md); issues and the
pull request policy are in [contributing](../CONTRIBUTING.md).
