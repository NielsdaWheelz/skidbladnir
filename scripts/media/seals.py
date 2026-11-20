# Port of Ornament.kt's sealSpec and DwarfPortrait to SVG, in dp units on a 36x36 viewBox. Writes one seal per
# catalogue key to $MEDIA_WORK/assets/seals.
# usage: uv run --no-project --with fonttools python scripts/media/seals.py KEY...
import hashlib
import json
import os
import sys

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.ttLib import TTFont

# Theme.kt
GOLD, BRONZE, BONE = "#D6A85F", "#CD7F32", "#F3F0E8"
OCTAGON_CUT_PERCENT = 29

# Ornament.kt SealMinerals, in index order.
MINERALS = [
    ("Garnet", "#830E0D"),
    ("Hematite", "#6D2114"),
    ("BlueGlass", "#26619C"),
    ("Basalt", "#2A2D33"),
    ("MossStone", "#333B34"),
    ("BronzeStone", "#43392C"),
    ("Porphyry", "#3A2C33"),
    ("Slate", "#2E3A3D"),
]

# Ornament.kt RuneSegments, in index order: (x0, y0, x1, y1), x in stave widths, y in 0..1 of the stave.
RUNES = [
    [(0, .14, .30, .02), (0, .30, .30, .18)],  # fé
    [(0, .10, .30, .28), (.30, .28, .30, .92)],  # úr
    [(0, .26, .28, .44), (.28, .44, 0, .62)],  # þurs
    [(0, .14, .30, .30), (0, .30, .30, .46)],  # óss
    [(0, .14, .26, .28), (.26, .28, 0, .42), (0, .42, .28, .68)],  # reið
    [(0, .22, .30, .46)],  # kaun
    [(-.26, .32, .26, .54), (-.26, .54, .26, .32)],  # hagall
    [(-.26, .34, .26, .54)],  # nauð
    [],  # íss
    [(-.26, .54, .26, .34)],  # ár
    [(0, .28, .24, .40), (.24, .40, .04, .52), (.04, .52, .28, .64)],  # sól
    [(-.26, .28, 0, .10), (0, .10, .26, .28)],  # týr
    [(0, .14, .28, .28), (.28, .28, 0, .42), (0, .46, .28, .60), (.28, .60, 0, .74)],  # bjarkan
    [(0, .18, -.28, .02), (0, .18, .28, .02)],  # maðr
    [(0, .10, .30, .26)],  # lǫgr
    [(0, .80, -.28, .96), (0, .80, .28, .96)],  # ýr
]

SIZE = 36.0  # DwarfPortrait's 36dp box
DENSITY = 3.0  # the beard hairline is 1 px, so its width in dp depends on the screen; xxhdpi here
FONT_SIZE, LINE_HEIGHT, INSET = 12.0, 16.0, 6.0
LETTER_SPACING = 0.5  # labelMedium's 0.5sp tracking, which Android splits half before and half after the glyph
FONT_LOCATION = {"wght": 650, "opsz": 72}  # DwarfPortrait asks for Black, but Display is one font fixed at 650; opsz stays at its default


def seal_spec(key):
    h = hashlib.sha256(key.encode("utf-8")).digest()  # bytes index as unsigned ints
    count = 2 + (h[4] & 1)
    runes = []
    for i in range(8):
        candidate = h[5 + i] % 16
        if candidate not in runes:
            runes.append(candidate)
        if len(runes) == count:
            break
    return {
        "mineral": h[0] % 8,
        "runes": runes,
        "teeth": 3 + h[8] % 4,
        "depth": h[9] % 4,
        "mask": h[12],
        "metal": "Gold" if h[14] & 1 else "Bronze",
    }


def n(v):
    s = f"{v:.4f}".rstrip("0").rstrip(".")
    return "0" if s in ("-0", "") else s


def p(x, y):
    return f"{n(x)} {n(y)}"


def seal_svg(key, name, font):
    spec = seal_spec(key)
    w = h = side = SIZE
    cut = side * OCTAGON_CUT_PERCENT / 100
    v = [(cut, 0), (w - cut, 0), (w, cut), (w, h - cut), (w - cut, h), (cut, h), (0, h - cut), (0, cut)]
    octagon = "M" + "L".join(p(*xy) for xy in v) + "Z"

    top, left, right, valley = h * .60, w * .24, w * .76, h * .88
    tip = valley - (.10 + spec["depth"] * .022) * h
    span = spec["teeth"] - 1
    tooth = (right - left) / span
    beard = [(left, top), (right, top), (right, tip)]
    for k in range(1, span + 1):
        beard += [(right - (k - .5) * tooth, valley), (right - k * tooth, tip)]
    beard = "M" + "L".join(p(*xy) for xy in beard) + "Z"

    sx, st, sb, rw = w * .5, h * .15, h * .58, w * .30
    rune = [f"M{p(sx, st)}L{p(sx, sb)}"]
    for r in spec["runes"]:
        for x0, y0, x1, y1 in RUNES[r]:
            rune.append(f"M{p(sx + x0 * rw, st + y0 * (sb - st))}L{p(sx + x1 * rw, st + y1 * (sb - st))}")

    edges = [f"M{p(*v[e])}L{p(*v[(e + 1) % 8])}" for e in range(8)]
    gilded = [edges[e] for e in range(8) if (spec["mask"] >> e) & 1]

    initial = name[:1].upper()
    glyphs = font.getGlyphSet(location=FONT_LOCATION)
    glyph = font.getBestCmap()[ord(initial)]
    pen = SVGPathPen(glyphs, ntos=lambda f: n(round(f, 1)))
    glyphs[glyph].draw(pen)
    upm = font["head"].unitsPerEm
    scale = FONT_SIZE / upm
    ascent, descent = font["hhea"].ascent * scale, -font["hhea"].descent * scale
    # A one-line Text at BottomEnd, inset 6dp: its box ends at (30, 30), one advance plus the tracking wide and
    # 16dp high, the font's extent centred in it (Material 3's LineHeightStyle). Android snaps both to whole pixels.
    line_top = h - INSET - LINE_HEIGHT
    baseline = line_top + (LINE_HEIGHT - ascent - descent) / 2 + ascent
    origin = w - INSET - LETTER_SPACING / 2 - glyphs[glyph].width * scale

    mineral_name, mineral = MINERALS[spec["mineral"]]
    metal = GOLD if spec["metal"] == "Gold" else BRONZE
    lines = [
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 36 36" width="36" height="36" aria-hidden="true">',
        f"<!-- {key}: mineral {spec['mineral']} {mineral_name}, runes {spec['runes']}, teeth {spec['teeth']},"
        f" depth {spec['depth']}, mask {spec['mask']}, metal {spec['metal']} -->",
        f'<clipPath id="octagon"><path d="{octagon}"/></clipPath>',
        '<g clip-path="url(#octagon)" stroke-linecap="butt">',
        f'<rect width="36" height="36" fill="{mineral}"/>',
        '<path d="M0 0H36L0 36Z" fill="#FFF" fill-opacity=".045"/>',
        f'<path d="M36 {n(h * .55)}V36H{n(w * .35)}Z" fill="#000" fill-opacity=".16"/>',
        f'<path d="{beard}" fill="#000" fill-opacity=".34" stroke="#FFF" stroke-opacity=".1" stroke-width="{n(1 / DENSITY)}"/>',
        f'<path d="{"".join(rune)}" fill="none" stroke="{metal}" stroke-width="{n(side * .045)}"/>',
        f'<path d="{"".join(edges)}" stroke="#3A3E45" stroke-width="{n(side * .012)}"/>',
    ]
    if gilded:
        lines.append(f'<path d="{"".join(gilded)}" stroke="{GOLD}" stroke-width="{n(side * .025)}"/>')
    lines += [
        f'<path d="{pen.getCommands()}" fill="{BONE}" transform="translate({p(origin, baseline)}) scale({n(scale)} -{n(scale)})"/>',
        "</g>",
        "</svg>",
    ]
    return spec, "\n".join(lines) + "\n"


def main():
    repo = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
    outdir = os.path.join(os.environ.get("MEDIA_WORK") or os.path.expanduser("~/.cache/skid-media"), "assets", "seals")
    os.makedirs(outdir, exist_ok=True)
    catalogue = os.path.join(repo, "internal", "catalog", "characters.json")
    names = {c["key"]: c["displayName"] for c in json.load(open(catalogue, encoding="utf-8"))}
    font = TTFont(os.path.join(repo, "android", "app", "src", "main", "res", "font", "big_shoulders.ttf"))
    for key in sys.argv[1:]:
        spec, svg = seal_svg(key, names[key], font)
        with open(f"{outdir}/{key}.svg", "w", encoding="utf-8") as f:
            f.write(svg)
        print(key, MINERALS[spec["mineral"]][0], spec)


if __name__ == "__main__":
    main()
