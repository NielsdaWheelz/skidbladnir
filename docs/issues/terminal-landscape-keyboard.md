# landscape with the keyboard leaves no terminal

problem: in landscape with gboard raised, the rail, the 2 × 7 key deck and the
keyboard together exceed the window height. the terminal gets zero rows and the
deck's second row is clipped.

impact: the phone terminal is unusable in landscape while typing; the
too-small recovery overlay appears instead. this predates the one-row rail,
which only reduced the deficit.

evidence: emulator at 1080 × 2340 px, 450 dpi, rotated; screenshot taken
2026-10-02 during the terminal chrome work (not retained). the deck needs
106 dp, the rail 48 dp plus the status bar, and gboard takes most of the
remaining ~384 dp height.

resolved when: landscape with gboard raised leaves at least the 20 × 5 minimum
grid on the s22+, with every deck key reachable. one candidate: a width rule
that lays the fourteen keys in one row when the deck is at least 706 dp wide
(14 × 48 dp plus gaps and padding), with no new mode; its ordering needs a
design pass, since one row loses the inverted-t arrow cluster.
