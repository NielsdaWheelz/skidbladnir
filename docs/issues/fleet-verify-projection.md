# discarded fleet verification projection

problem: `fleet_evidence` validates the fleet, then prints three machine identity
rows. its only caller, `verify_fleet`, discards all output and prints success.

evidence: `scripts/fleet` contains one call to `fleet_evidence`, redirected to
`/dev/null`. no consumer uses the formatted handle/origin rows.

resolved when: inline the validation body in `verify_fleet` and delete its unused
projection, preserving remote/local checks, order, uniqueness admission, errors
and success output. verify caller/source equivalence and script checks; deleting
unobserved formatting does not require a live fleet operation.
