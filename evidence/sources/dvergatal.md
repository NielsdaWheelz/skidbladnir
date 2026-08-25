# Dvergatal curation and provenance

Status: frozen P0 curation record for `catalog/characters.json`. This file owns
the catalogue evidence; portrait artifacts remain a P5 deliverable.

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

Every line below is machine-parseable as six pipe-delimited fields:
`ENTRY|key=...|displayName=...|tradition=...|work=...|locus=...`.
Keys use the catalogue's ASCII asset-safe grammar. Display names preserve a
single normalized source spelling or an explicit primary-text epithet and are
globally unique. Count and uniqueness are checked by the P0 static gate before
the catalogue is frozen.

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

## Portrait production and rights basis

Portraits are not sourced from these texts as images. P5 must produce one
original square portrait per key and record the exact artist or generation tool
and version, generation date, input prompt if applicable, source digest, and
rights record. No scraped image, book cover, film still, stage photograph,
rights-holder depiction, or third-party character art is admissible.

- **OldNorse:** repository-owned original vector/painted portraits based only
  on the cited textual name and public-domain textual motifs; the artist's
  written assignment gives the repository the necessary reproduction and
  modification rights.
- **Tolkien:** repository-owned original portraits derived from textual
  attributes only, with no Tolkien Estate, publisher, film, game, or illustrator
  artwork used as a visual source; the commissioned work-for-hire/assignment
  record must cover repository distribution and derivative resizing.
- **GermanicOperatic:** repository-owned original portraits based on Wagner's
  libretto descriptions, never a production still, costume design, recording
  cover, or other performance asset; the same written assignment and asset
  digest requirements apply.

These are the production rules, not evidence that portraits already exist.
Portrait evidence remains open until P5 records the named artist or exact tool
version and the corresponding rights assignment. Portrait bytes land in P5,
not in this source record.

## Portrait set acceptance

P0 freezes the set-level production brief and rights method; P5 creates the
portrait bytes and the per-key `portrait-manifest.v1` records. A good set is
producible from that brief without importing an unrecorded likeness or a
missing rights decision:

- Each portrait is an original, single-figure square WebP at a common minimum
  size of 256×256 pixels. It uses the same head-and-shoulders card framing,
  neutral background treatment, lighting direction, and restrained detail
  level across all 101 entries. No text, logo, watermark, border, or extra
  character appears in the image.
- The cited work and locus constrain the depiction. A name, epithet, and
  explicitly described role may inform the face, silhouette, clothing, or one
  restrained prop; an unsupported modern likeness, actor, illustrator,
  production costume, film/game design, or invented canonical biography may
  not. A visual distinction must be explainable by the entry's source or by a
  neutral compositional choice, not by borrowing a third-party depiction.
- Every portrait must remain distinguishable from the other 100 at the card
  thumbnail size and in grayscale: face or silhouette, value grouping, and
  one source-safe distinguishing cue must survive 64×64 downsampling. Hue
  alone is never the distinguishing cue. Review records must identify the cue
  and reject near-duplicates before P5 freezes the manifest; this review
  criterion does not add a manifest field.
- The subject/background boundary and facial features must remain legible at
  64×64 and 200% display scale, with no essential cue at the extreme edge.
  The portrait supplies no required text or color-only signal; the card's
  accessible display name remains the authoritative label.
- For each key, P5 records the exact production method, artist or tool and
  version, creation date, applicable prompt, source digest, and rights basis
  and assignment in the manifest. Static QA then checks one record and one
  square WebP per key, matching SHA-256 and documented asset path, with no
  orphan record or asset. A failed thumbnail, provenance, digest, or rights
  check is red; it is never repaired with a placeholder or silently reused
  portrait.

## Dedupe and exclusion record

- `Eikinskjaldi` appears twice in *Vǫluspá* but is one shipped Old Norse entry.
- `Lofarr` is named through the lineage clauses in stanzas 14 and 16 and is
  retained as the terminal member of the explicitly dwarven company.
- Diacritics are normalized, not multiplied into variants: each Old Norse
  source spelling below appears once.
- Tolkien entries with names cognate with Old Norse names are separate authored
  characters only where the Tolkien text identifies a distinct individual;
  their display suffixes make the global display-name set unambiguous.
- `Mîm` and Wagner's `Mime` are separate characters in separate primary works;
  their source/locus and display forms remain distinct.
- No `FairyTale`, `ModernMedia`, generic Nibelung, unnamed dwarf, disputed dwarf,
  giant, dragon, human, god, elf, or invented catalogue filler is included.

## Frozen inventory (101 entries)

```text
ENTRY|key=norse.modsognir|displayName=Móðsognir|tradition=OldNorse|work=Vǫluspá|locus=st. 10
ENTRY|key=norse.durinn|displayName=Durinn|tradition=OldNorse|work=Vǫluspá|locus=st. 10
ENTRY|key=norse.nyi|displayName=Nýi|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.nidi|displayName=Niði|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.nordri|displayName=Norðri|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.sudri|displayName=Suðri|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.austri|displayName=Austri|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.vestri|displayName=Vestri|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.althjofr|displayName=Alþjófr|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.dvalinn|displayName=Dvalinn|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.nar|displayName=Nár|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.nainn|displayName=Náinn|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.nipingr|displayName=Nípingr|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.dainn|displayName=Dáinn|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.bivurr|displayName=Bívurr|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.bavurr|displayName=Bávurr|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.bomburr|displayName=Bömburr|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.nori|displayName=Nóri|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.ann|displayName=Ánn|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.anarr|displayName=Ánarr|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.oinn|displayName=Óinn|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.mjodvitnir|displayName=Mjöðvitnir|tradition=OldNorse|work=Vǫluspá|locus=st. 11
ENTRY|key=norse.veggr|displayName=Veggr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.gandalfr|displayName=Gandalfr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.vindalfr|displayName=Vindalfr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.thorinn|displayName=Þorinn|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.thrar|displayName=Þrár|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.thrainn|displayName=Þráinn|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.thekkr|displayName=Þekkr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.litr|displayName=Litr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.vitr|displayName=Vitr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.nyr|displayName=Nýr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.nyradr|displayName=Nýráðr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.reginn|displayName=Reginn|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.radsvidr|displayName=Ráðsviðr|tradition=OldNorse|work=Vǫluspá|locus=st. 12
ENTRY|key=norse.fili|displayName=Fíli|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.kili|displayName=Kíli|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.fundinn|displayName=Fundinn|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.nali|displayName=Náli|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.hefti|displayName=Hefti|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.vili|displayName=Víli|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.hannar|displayName=Hannar|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.sviurr|displayName=Svíurr|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.billingr|displayName=Billingr|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.bruni|displayName=Brúni|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.bildr|displayName=Bíldr|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.buri|displayName=Buri|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.frar|displayName=Frár|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.hornbori|displayName=Hornbori|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.fraegr|displayName=Frægr|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.loni|displayName=Lóni|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.aurvangr|displayName=Aurvangr|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.jari|displayName=Jari|tradition=OldNorse|work=Vǫluspá|locus=st. 13
ENTRY|key=norse.eikinskjaldi|displayName=Eikinskjaldi|tradition=OldNorse|work=Vǫluspá|locus=st. 13 and st. 16
ENTRY|key=norse.draupnir|displayName=Draupnir|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.dolgthrasir|displayName=Dolgþrasir|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.har|displayName=Hár|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.haugspori|displayName=Haugspori|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.hlevangr|displayName=Hlévangr|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.gloinn|displayName=Glóinn|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.dori|displayName=Dóri|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.ori|displayName=Óri|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.dufr|displayName=Dúfr|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.andvari|displayName=Andvari|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.skirfir|displayName=Skirfir|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.virfir|displayName=Virfir|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.skafidr|displayName=Skáfiðr|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.ai|displayName=Ái|tradition=OldNorse|work=Vǫluspá|locus=st. 15
ENTRY|key=norse.alfr|displayName=Alfr|tradition=OldNorse|work=Vǫluspá|locus=st. 16
ENTRY|key=norse.yngvi|displayName=Yngvi|tradition=OldNorse|work=Vǫluspá|locus=st. 16
ENTRY|key=norse.fjalarr|displayName=Fjalarr|tradition=OldNorse|work=Vǫluspá|locus=st. 16
ENTRY|key=norse.frosti|displayName=Frosti|tradition=OldNorse|work=Vǫluspá|locus=st. 16
ENTRY|key=norse.finnr|displayName=Finnr|tradition=OldNorse|work=Vǫluspá|locus=st. 16
ENTRY|key=norse.ginnarr|displayName=Ginnarr|tradition=OldNorse|work=Vǫluspá|locus=st. 16
ENTRY|key=norse.lofarr|displayName=Lofarr|tradition=OldNorse|work=Vǫluspá|locus=sts. 14 and 16
ENTRY|key=tolkien.balin|displayName=Balin|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.bifur|displayName=Bifur|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.bofur|displayName=Bofur|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.bombur-erebor|displayName=Bombur of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.dwalin-erebor|displayName=Dwalin of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.fili-erebor|displayName=Fíli of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.kili-erebor|displayName=Kíli of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.dori-erebor|displayName=Dori of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.nori-erebor|displayName=Nori of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.ori-erebor|displayName=Ori of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.oin-erebor|displayName=Óin of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.gloin-erebor|displayName=Glóin of Erebor|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.thorin-oakenshield|displayName=Thorin Oakenshield|tradition=Tolkien|work=The Hobbit|locus=ch. 1
ENTRY|key=tolkien.thrain-ii|displayName=Thráin II|tradition=Tolkien|work=The Lord of the Rings|locus=Appendix A.III
ENTRY|key=tolkien.dain-ironfoot|displayName=Dáin Ironfoot|tradition=Tolkien|work=The Lord of the Rings|locus=Appendix A.III
ENTRY|key=tolkien.durin-deathless|displayName=Durin the Deathless|tradition=Tolkien|work=The Lord of the Rings|locus=Appendix A.III
ENTRY|key=tolkien.gimli|displayName=Gimli|tradition=Tolkien|work=The Lord of the Rings|locus=Appendix A.III
ENTRY|key=tolkien.azaghal|displayName=Azaghâl|tradition=Tolkien|work=The Silmarillion|locus=ch. 20
ENTRY|key=tolkien.telchar|displayName=Telchar|tradition=Tolkien|work=The Silmarillion|locus=ch. 10
ENTRY|key=tolkien.mim-amon-rudh|displayName=Mîm of Amon Rûdh|tradition=Tolkien|work=The Children of Húrin|locus=ch. 11
ENTRY|key=tolkien.khim|displayName=Khîm|tradition=Tolkien|work=The Children of Húrin|locus=ch. 11
ENTRY|key=tolkien.ibun|displayName=Ibûn|tradition=Tolkien|work=The Children of Húrin|locus=ch. 11
ENTRY|key=tolkien.frerin|displayName=Frerin|tradition=Tolkien|work=The Lord of the Rings|locus=Appendix A.III
ENTRY|key=tolkien.groin|displayName=Gróin|tradition=Tolkien|work=The Lord of the Rings|locus=Appendix A.III
ENTRY|key=germanic-operatic.alberich|displayName=Alberich of Nibelheim|tradition=GermanicOperatic|work=Das Rheingold|locus=Scene 3
ENTRY|key=germanic-operatic.mime|displayName=Mime of Nibelheim|tradition=GermanicOperatic|work=Siegfried|locus=Act I
```

Inventory count: 75 `OldNorse` + 24 `Tolkien` + 2 `GermanicOperatic` = 101.
