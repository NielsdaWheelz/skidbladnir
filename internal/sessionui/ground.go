package sessionui

import (
	"image/color"
	"math"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

const groundQuery = ansi.RequestForegroundColor + ansi.RequestBackgroundColor + ansi.RequestPrimaryDeviceAttributes

var plainPigments = [bronzeRole + 1]color.Color{
	mutedRole: color.RGBA{R: 0xAA, G: 0xA6, B: 0x9D, A: 0xff},
	goldRole:  ansi.BrightYellow,
	emberRole: ansi.BrightRed,
	mossRole:  color.RGBA{R: 0x76, G: 0xB0, B: 0x82, A: 0xff},
	frostRole: color.RGBA{R: 0x78, G: 0xA9, B: 0xC6, A: 0xff},
}

type groundExpired struct{}

type ground struct {
	profile      colorprofile.Profile
	noColor      bool
	note         string // empty while the probe is pending
	ink, base    color.Color
	raise, forge color.Color
	pigments     [bronzeRole + 1]color.Color
}

func (g *ground) attachRefusal(key string) string {
	if g.note == "" {
		return "reading the terminal's colours; press " + key + " again."
	}
	return ""
}

func (g *ground) settle(answered bool) {
	defer func() { g.ink, g.base = nil, nil }()
	switch {
	case !answered || g.ink == nil || g.base == nil:
		g.note = "colours: plain; the terminal did not report its own."
		return
	case g.profile != colorprofile.TrueColor:
		g.note = "colours: plain; the terminal profile is not truecolor."
		return
	}

	ink := color.NRGBAModel.Convert(g.ink).(color.NRGBA)
	base := color.NRGBAModel.Convert(g.base).(color.NRGBA)
	white, black := color.NRGBA{255, 255, 255, 255}, color.NRGBA{0, 0, 0, 255}
	end := black
	pigments := [...]color.NRGBA{
		{0x8A, 0x64, 0x16, 255}, {0xB2, 0x3B, 0x24, 255}, {0x3B, 0x74, 0x48, 255},
		{0x2C, 0x66, 0x88, 255}, {0x94, 0x56, 0x1A, 255},
	}
	inkLight := luminance(g.ink)
	if luminance(g.base) < inkLight {
		end = white
		pigments = [...]color.NRGBA{
			{0xD6, 0xA8, 0x5F, 255}, {0xE4, 0x6C, 0x55, 255}, {0x76, 0xB0, 0x82, 255},
			{0x78, 0xA9, 0xC6, 255}, {0xCD, 0x7F, 0x32, 255},
		}
	}
	raise, forge := mix(base, ink, 7), mix(base, pigments[0], 10)
	onRaise, onForge := luminance(raise), luminance(forge)
	inkContrast := contrast(inkLight, onRaise)
	// One palette owns both fills, so the ink and every role clear both.
	if inkContrast < 4.5 || contrast(inkLight, onForge) < 4.5 {
		g.note = "colours: plain; the terminal's ink has too little contrast."
		return
	}

	pick := func(start, end color.NRGBA, required float64) color.NRGBA {
		for step := 0; step < 100; step++ {
			candidate := mix(start, end, step)
			value := luminance(candidate)
			if contrast(value, onRaise) >= required && contrast(value, onForge) >= 4.5 {
				return candidate
			}
		}
		return end
	}
	quiet := pick(raise, ink, max(4.5, inkContrast/2))
	g.pigments[quietRole], g.pigments[mutedRole] = quiet, quiet
	for i, pigment := range pigments {
		g.pigments[goldRole+role(i)] = pick(pigment, end, 4.5)
	}
	g.raise, g.forge = raise, forge
	g.note = "colours: full; derived from the terminal's own colours."
}

func mix(from, to color.NRGBA, percent int) color.NRGBA {
	mixChannel := func(a, b uint8) uint8 {
		return uint8((int(a)*(100-percent) + int(b)*percent + 50) / 100)
	}
	return color.NRGBA{R: mixChannel(from.R, to.R), G: mixChannel(from.G, to.G), B: mixChannel(from.B, to.B), A: 255}
}

func luminance(c color.Color) float64 {
	linear := func(channel uint32) float64 {
		x := float64(channel) / 65535
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	r, g, b, _ := c.RGBA()
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func contrast(a, b float64) float64 {
	return (max(a, b) + 0.05) / (min(a, b) + 0.05)
}

func (l line) filled(background color.Color, width int) line {
	if background == nil {
		return l
	}
	for i := range l {
		l[i].background = background
		width -= ansi.StringWidth(l[i].text)
	}
	if width > 0 {
		l = append(l, span{text: strings.Repeat(" ", width), background: background})
	}
	return l
}
