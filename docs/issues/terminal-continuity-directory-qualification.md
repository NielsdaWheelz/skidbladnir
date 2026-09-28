# directory search qualification

problem: the named `v0.10.3` phone journey used `z` to choose a non-home local
directory and changed remote cwd. it did not qualify all host-backed search
and selection failure paths.

impact: a stale or removed result, unavailable zoxide, oversized result, or
machine switch could select the wrong cwd or obscure an honest error. no such
failure has been observed.

evidence: [pr 10](https://github.com/NielsdaWheelz/skidbladnir/pull/10)
records the successful phone path. [pr 4](https://github.com/NielsdaWheelz/skidbladnir/pull/4)
reports temporary desktop directory-page checks, not final-release live host
qualification. ranked results from each host's existing database, literal path
handling, no matches, disabled or missing tool, bounds, removed directory,
and host-switch races in the
[directory row](../terminal-continuity.md#6-delivery-and-adversarial-acceptance)
have no recorded final-release live result and remain `NOT_RUN`.

resolved when: with current-turn approval, run those cases against test-owned
directories and the actual configured host zoxide paths. prove selection edits
only the draft cwd and creation revalidates the path. retain content-free
outcomes, then remove this record.
