# The fret band is not the 45° chevron its spec names

Problem: `Ornament.kt` calls `FretCell` "one 45° chevron unit", and design
language §7 allows "45°/90° turns only", but `drawFretBand` draws each unit as
wide as the band is high. Its legs then rise at about 63.4°, meeting at a 53.1°
apex.

Impact: the frozen geometry and the documents that describe it disagree; anyone
redrawing the band from the spec gets a flatter chevron than the app.

Evidence: `FretCell` runs from (0, 1) to (0.5, 0) to (1, 1) in a unit as wide as
it is high (`drawFretBand`: `unitWidth = size.height`); atan(1 / 0.5) ≈ 63.4°.
Found while porting the band to `site/assets/fret-band.svg` on 2026-10-26.

Resolved when: the comment and §7 state the angle the frozen geometry draws, or
the owner changes the geometry and accepts that every fret band changes.
