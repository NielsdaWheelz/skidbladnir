# Dvergatal curation and provenance

Status: frozen curation record for `internal/catalog/characters.json`, which is itself
the inventory: every entry carries its key, display name, tradition, and source
work and locus. This file owns the admission rules behind it. The 2026-08-25
v0 scope reset replaced the superseded raster portrait deliverable with
deterministic procedural Android icons; v0 has no portrait artifact or manifest
gate.

## Acceptance for good content

An entry is admissible only when a primary text names the individual and either
calls the individual a dwarf/Nibelung or places the individual in an explicitly
dwarven company. The source citation is the shortest stable work plus chapter,
scene, or stanza that proves that fact. A name is not admitted from a fan wiki,
adaptation-only list, generic race label, disputed attribution, or an inferred
translation.

The Old Norse set uses the named individuals in the `Dvergatal` of *Vǫluspá*
stanzas 10–16. The repeated `Eikinskjaldi` in stanzas 13 and 16 is one figure
and ships once. Lofarr ships once because stanza 14 places him at the end of
Dvalinn's dwarven kindred and stanza 16 names that lineage as Lofarr's. The
Tolkien set uses distinct textual characters from *The
Hobbit*, *The Lord of the Rings* Appendix A, *The Silmarillion*, and *The
Children of Húrin*. Display suffixes such as `of Erebor` and `Ironfoot` are
curatorial disambiguators, not additional figures. Tolkien's `Mîm` and
Wagner's `Mime` are separate authored characters and are retained under their
separate works; neither is a spelling-variant duplicate of the other.

The Germanic-operatic set is limited to named Nibelung dwarfs in Wagner's own
*Der Ring des Nibelungen* libretti: Alberich and Mime. Other Nibelungs are an
unnamed collective, and giants, humans, gods, dragons, and stage-only names are
excluded.

Keys use the catalogue's ASCII asset-safe grammar. Display names preserve a
single normalized source spelling or an explicit primary-text epithet and are
globally unique. `internal/catalog` embeds the file and panics at process start
on an invalid or duplicate key; nothing checks the file before release. The
other rules here (unique NFC display names, no `FairyTale` or `ModernMedia`
tradition, one primary-text locus per entry) are applied by review; no gate
re-checks them.

## Primary sources

- *Vǫluspá*, Codex Regius text, sts. 10–16. For a convenient transcription and
  translation see [Vǫluspá, literal text](https://www.voluspa.org/literal/voluspa.htm);
  the controlling scholarly editions are Ursula Dronke, *Poetic Edda II:
  Mythological Poems* (Oxford, 1997), and Edward Pettit, *The Poetic Edda: A
  Dual-Language Edition* (Oxford, 2023).
- J. R. R. Tolkien, *The Hobbit* (1937), ch. 1; *The Lord of the Rings*
  (1954–55), Appendix A III, “Durin's Folk”; *The Silmarillion* (1977), ed.
  Christopher Tolkien, chs. 16 and 20; and *The Children of Húrin* (2007), ch.
  11, “Of Mîm the Dwarf”. These are the primary authored texts, not adaptation
  guides.
- Richard Wagner, *Der Ring des Nibelungen* libretti (1848–74), *Das
  Rheingold*, Scene 3, and *Siegfried*, Act I. The [Naxos Siegfried
  libretto](https://www.naxos.com/libretti/660175.htm) is a convenient
  publisher-hosted text; the German libretto is authoritative for the
  character classification.

## V0 icon basis

Android derives a deterministic procedural dwarf landmark from the catalogue
key and uses the accessible display name as its authoritative label. v0 imports
no third-party imagery and owns no raster portrait pack, provenance manifest,
or rights claim. A raster portrait set requires a new scoped decision.

## Dedupe and exclusion record

- `Eikinskjaldi` appears twice in *Vǫluspá* but is one shipped Old Norse entry.
- `Lofarr` is named through the lineage clauses in stanzas 14 and 16 and is
  retained as the terminal member of the explicitly dwarven company.
- Diacritics are normalized, not multiplied into variants: each Old Norse
  source spelling appears once.
- Tolkien entries with names cognate with Old Norse names are separate authored
  characters only where the Tolkien text identifies a distinct individual;
  their display suffixes make the global display-name set unambiguous.
- `Mîm` and Wagner's `Mime` are separate characters in separate primary works;
  their source/locus and display forms remain distinct.
- No `FairyTale`, `ModernMedia`, generic Nibelung, unnamed dwarf, disputed dwarf,
  giant, dragon, human, god, elf, or invented catalogue filler is included.
