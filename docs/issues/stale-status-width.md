# stale status labels overflow the desktop status bay

problem: a stale desktop row's status cell reads `last observed: <label>`, up to
33 cells (`last observed: status unavailable`) against 18 for the widest fresh
label. spec §6 requires labels to fit the existing status bay.

impact: while a host's read has failed, the status column widens and, at 80
columns, session names shrink from 24 to 12 cells. a pending re-read stops
causing this once rows being re-read read faint `checking` again (`131c1df`,
pending merge after `6859010`); a failed host still does.

evidence: an 80×24 desktop model render with a failed host shows the 33-cell cell
and the 12-cell names; no line exceeds 80 cells.

resolved when: a shorter stale form, or an explicit spec §6 exemption with its
layout stated, is chosen and an 80×24 render with a failed host keeps names
legible; then delete this record.
