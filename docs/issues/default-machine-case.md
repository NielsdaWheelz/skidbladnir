# default_machine admits one case and matches another

problem: `default_machine` is admitted case-insensitively but matched exactly.
`hostconfig.Config.ValidateMachines` (`internal/hostconfig/config.go:184`) and
fleet's resolve (`internal/fleet/service.go:181`) accept it when a machine name
equals it under `strings.EqualFold`; the client selects the default with `==`
(`internal/fleetclient/config.go:44`). `default_machine: Arch` for the machine
`arch` is applied, then ignored.

impact: the browser's `n`/`N` creation and quick terminal default to this machine
instead of the configured one, with no notice. names are unique under the same
fold (`internal/fleet/hub.go:143`), so only the exact comparison disagrees. code
reading; not reproduced live.

fix: fleet's resolve stores the matched machine's exact name in
`settings.DefaultMachine`, so every reader sees the canonical spelling and the
client stays exact. it changes which machine the browser preselects, so it needs
the owner. `ValidateMachines`, a fleet rule kept in `hostconfig`, can move to
`internal/fleet` in the same change.

resolved when: with `default_machine: Arch` and a machine named `arch`, the
creation form preselects `arch`, and admission and adoption use one comparison.
