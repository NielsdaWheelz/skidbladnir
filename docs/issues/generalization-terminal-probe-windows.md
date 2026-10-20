# terminal probe window cleanup

problem: early failed apple-terminal qualification attempts may have left blank
windows whose exact creation receipts were not captured. subsequent corrected
probes closed only their own proven windows.

impact: a blank terminal window may remain open. no sessions or windows were
closed by guessed title, tty, position or current selection.

evidence: isolated terminal-adapter qualification on 2026-10-20; failed receipt
capture preceded the corrected tty-readiness proof. ownership of any remaining
blank window is unproven.

resolved when: the owner confirms no unwanted probe window remains, or identifies
and closes one. do not close an unreceipted window automatically.
