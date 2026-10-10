# notification sdk memory qualification

problem: two physical runs changed the aggregate byte hash of android shared
preferences. their causes and owning preference files are unqualified.

impact: those runs cannot claim protected cleanup. the three intervening
foreground cases pass their complete guards. do not restore unknown preference
bytes or waive registration protection.

evidence: `android-foreground-sweep-791b42cd`, controller `9c2d9bf9`, observes
actual ready output acknowledgement, ordinary detach, dashboard focus and a
fresh exact native question. 23 of 24 cleanup checks pass; `sdkUnchanged` is
false. receiver, desired subscription, encrypted pairings, apk identities,
permissions, audio, channel, routes and production remain exact; working state,
native zero and zero owned clients are restored. the guard hashes every
`shared_prefs/*.xml`, including storage not necessarily owned by unifiedpush.
connector 3.3.5 stores its current identity in `unifiedpush-connector` sqlite;
this guard does not read it. the phone has no sqlite cli available under the qa
uid, so copied database/journal bytes cannot supply a coherent identity read.
subsequent dashboard, other-session and focus-sheet runs pass all 24 protections
with seven coherent preference captures unchanged byte-for-byte. this qualifies
those current boundaries; it does not explain the earlier mutation.

the first repaired ready-projection run `48ed70ad` also passes behaviour but
fails preference byte equality: 24 of 25 cleanup guards pass. exact native
delivery, pairings, configuration, desired subscription, receiver, permissions,
audio, routes, working state and zero clients remain qualified. its preference
baseline also existed only in ram and is lost. fresh coherent per-file
diagnostics must identify any reproduced drift; they cannot recover that baseline.

fresh `7b63c901` passes the same repaired behaviour and all 25 cleanup guards.
its wider diagnostic interval captures all seven preference files coherently;
raw bytes, canonical entries and key sets are unchanged. this qualifies the
current repair, not either historical mutation. current sdk database identity
remains outside those captures.

resolved when: identify the changed storage and distinguish meaningful sdk
identity changes from normal non-sdk state or xml serialization. use its actual
owned contract, rerun the physical boundary and verify complete cleanup.
the affected runs remain `NOT_RUN`; their before-hash lists existed only in ram.
