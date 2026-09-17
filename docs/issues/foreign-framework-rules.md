# imported rules prescribe absent runtime machinery

problem: `operation-types.md`, `database.md`, `resource-lifecycle.md`,
`layers.md` and `effect-services.md` describe a managed replay/database/layer
framework absent from this go/kotlin application. other rule files still require
those primitives.

impact: the guidance can contradict the actual contract. operation-types forbids
product-facing unknown outcomes, while terminal input must preserve uncertain
delivery and never replay it automatically.

resolved when: remove the absent framework and its dependent prescriptions;
preserve useful rules for ownership, concurrency, clocks, resource lifetime and
failure classification in their existing owners. audit incoming links and
references. the separate module specifications have already been removed.
