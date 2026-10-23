# the hub admits profile labels every host then refuses

problem: profile rules have two owners that disagree. `skid apply` admits the
hub's `config.yaml` through `hostconfig.Parse` (`internal/fleet/hub.go:40-49`),
whose label rule is 1-64 characters with no C0/C1 control
(`internal/hostconfig/config.go:153-157`). every commit then resolves settings
with `hostconfig.Resolve` and `agentruntime.ResolveProfiles`
(`internal/fleet/service.go:171-184`). `ResolveProfiles` repeats validate's key,
duplicate, command, provider and home rules, and its label rule also refuses
non-nfc text, bidi controls and u+2028/u+2029
(`internal/agentruntime/profile.go:63-72`). a member adopts the hub's settings
with no check of its own and resolves them the same way
(`internal/fleet/member.go:107-112`).

impact: a label like `"Café"` (decomposed) or one containing u+200e is
accepted. the apply answers 200 and raises the config revision. then every
computer keeps its previous applied settings and reports `invalid_host` with
"settings unavailable on this host; fix its shell or local paths"
(`service.go:173`), which names the wrong cause. nothing is lost; the previous
profiles keep running.

evidence: observation-refactors (#156) review, 2026-10-22; code reading at
`defbf3f`; not reproduced on a live fleet.

repro: on the hub, give a profile `label: "Café"` in `config.yaml` and run
`skid apply`; `skid status` shows every application `invalid_host` with the
shell/path reason.

fix: #156 proposed that members validate adopted settings with hostconfig and
that `ResolveProfiles` only translate. afaict [rules](../rules.md#trust-boundaries)
put the whole rule at the ingress instead: hostconfig's validate takes the
stricter label rule, `ResolveProfiles` translates only, and members trust the
hub's admitted settings as same-release data.

resolved when: an apply with that label answers 400 `ConfigInvalid` naming
`profiles[i].label` and leaves the accepted settings unchanged, and
`ResolveProfiles` holds no rule hostconfig lacks.
