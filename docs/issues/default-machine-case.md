# default_machine admits one case and matches another

problem: `default_machine` is admitted case-insensitively but matched exactly.
`hostconfig.Config.ValidateMachines` (`internal/hostconfig/config.go:184`) and
fleet's apply (`internal/fleet/service.go:192`) accept it when any machine name
equals it under `strings.EqualFold`; `fleetclient.Adopt`
(`internal/fleetclient/config.go:44`) selects the default with `==`. a config
naming `default_machine: Arch` for the machine `arch` is applied, then ignored.

impact: the desktop browser's `n`/`N` creation and quick terminal default to
self instead of the configured machine, with no error or notice. names are
unique under the same fold (`internal/fleet/hub.go:143`), so the fold never
picks between two machines; only the exact comparison disagrees.

evidence: 2026-10-22, code reading at 3ef8c2e; not reproduced on a live fleet.

fix: fleet's apply stores the matched machine's exact name in
`settings.DefaultMachine`, so every reader sees the canonical spelling and
`Adopt` stays exact. it changes which machine the browser preselects, so it is
an owner-visible change and lives in `internal/fleet`, outside the desktop
cleanup that recorded it. the same change can move `ValidateMachines`, a pure
fleet rule today kept in `hostconfig`, into `internal/fleet`.

resolved when: with `default_machine: Arch` and a machine named `arch`, the
browser's creation form preselects `arch`, and the admitted value and the
adopted default come from one comparison.
