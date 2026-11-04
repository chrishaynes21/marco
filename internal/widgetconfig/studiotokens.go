package widgetconfig

// STUDIO'S PAGE TOKENS, DERIVED FROM THE WIDGET'S PALETTE (S10,
// docs/S10-shared-themes-review-2026-11-04.md).
//
// Studio draws every colour on its page through a small set of CSS custom properties. This is
// the one place those are made from the theme the widget uses, so the two surfaces share one
// choice (`theme` in overlay.json) and one set of palettes (palettes.go). There is no Studio
// theme preference: `default` gives Studio the widget's default dark palette, and `light` is
// the light one.
//
// # The rules
//
//   - **Light or dark is the palette's**, decided by the luminance of its background, not by the
//     desktop.
//   - **Every text colour reads on every surface it can sit on**: the ground, a raised card, a
//     sunken field, and each tinted banner (ok, warning, refusal, the chosen row). A palette
//     colour that falls short is moved toward the ink until it passes; it is never used as it
//     is for words someone has to read. The accent is text too (an outlined Play label, a
//     link, a hint), so it is held to the same 4.5:1 as the rest, not the 3:1 of a border.
//   - **The ink is 7:1** on the ground, a card and a field, and 4.5:1 on a tint.
//   - **The text on an accent button is the ground**, which reads on the accent by the rule
//     above, and on the ink the button turns to under the pointer.
//   - **The accent may not borrow a semantic hue.** When the palette's own accent (`name`) is
//     within 30° of hue of ok, warning or refusal, its listening colour is the accent instead.
//     This is a heuristic about what a colour is taken to mean, not an accessibility guarantee;
//     labels and other non-colour cues carry the meaning, and the colour only supports them.
//   - **Warning is Studio's own amber.** The widget has no warning colour; the amber is lifted
//     for readability like every other text colour, and keeps its hue so it still reads as a
//     warning.
//
// TestEveryPaletteGivesStudioReadablePairs holds each rule against every palette.

import (
	"fmt"
	"math"
	"strconv"
)

// StudioTheme is what Studio applies: the theme it resolved, and the value of each token.
type StudioTheme struct {
	// Asked is the `theme` setting as the widget reads it; Theme is the palette it resolved to,
	// which is DefaultPalette for a name that is not one.
	Asked string `json:"asked"`
	Theme string `json:"theme"`
	// Light is whether the palette is a light one.
	Light bool `json:"light"`
	// AccentFrom is the palette role the accent came from: "name", or "listen" when the name
	// colour was too close to a semantic hue.
	AccentFrom string `json:"accentFrom"`
	// Tokens maps each CSS custom property ("--ground", …) to its value.
	Tokens map[string]string `json:"tokens"`
}

// The two warning ambers, Studio's own. The light one is for a light palette.
const (
	warnOnDark  = "e3a73c"
	warnOnLight = "8f5f07"
)

// The floors. TextFloor is for words; InkFloor for the main text on a plain surface;
// LineFloor for a divider, which nobody reads.
const (
	TextFloor = 4.5
	InkFloor  = 7.0
	LineFloor = 1.6
	// SemanticHueGap is how far, in degrees of hue, the accent must sit from ok, warning and
	// refusal.
	SemanticHueGap = 30.0
)

// PlainSurfaces and TintSurfaces are the backgrounds text is drawn on.
var (
	PlainSurfaces = []string{"--ground", "--raise", "--sink"}
	TintSurfaces  = []string{"--spot-bg", "--ok-bg", "--warn-bg", "--no-bg"}
	// TextTokens are the colours used for words.
	TextTokens = []string{"--ink", "--dim", "--faint", "--spot", "--ok", "--warn", "--no"}
)

// StudioTokens is the token set for the named theme, resolved by PaletteFor.
func StudioTokens(name string) StudioTheme {
	resolved, p := PaletteFor(name)
	bg := rgbOf(p.Bg)
	light := luminance(bg) > 0.5
	black, white := rgb{0, 0, 0}, rgb{255, 255, 255}

	ground := bg
	raise := mixRGB(bg, rgbOf(p.Cmd), 0.05)
	sink := mixRGB(bg, black, 0.25)
	toward := white
	tint := 0.14
	amber := rgbOf(warnOnDark)
	if light {
		raise, sink, toward, tint, amber = white, mixRGB(bg, black, 0.03), black, 0.10, rgbOf(warnOnLight)
	}
	plain := []rgb{ground, raise, sink}
	ink := readableOn(rgbOf(p.Cmd), plain, toward, InkFloor)

	accentFrom, accent := "name", rgbOf(p.Name)
	for _, sem := range []rgb{rgbOf(p.Run), rgbOf(p.Errc), amber} {
		if hueNear(accent, sem, SemanticHueGap) {
			accentFrom, accent = "listen", rgbOf(p.Listen)
			break
		}
	}

	// THE TEXT COLOURS AND THE TINTS DEPEND ON EACH OTHER: a tint is its colour over the ground,
	// and the colour must read on every tint. So lift against the plain surfaces and the tints
	// of the previous round until nothing moves.
	src := map[string]rgb{"--dim": rgbOf(p.Desc), "--faint": rgbOf(p.LastCmd), "--spot": accent,
		"--ok": rgbOf(p.Run), "--warn": amber, "--no": rgbOf(p.Errc)}
	tintOf := map[string]string{"--spot-bg": "--spot", "--ok-bg": "--ok", "--warn-bg": "--warn",
		"--no-bg": "--no"}
	got := map[string]rgb{}
	for k, c := range src {
		got[k] = c
	}
	tints := map[string]rgb{}
	for round := 0; round < 16; round++ {
		for t, c := range tintOf {
			tints[t] = mixRGB(bg, got[c], tint)
		}
		on := append([]rgb(nil), plain...)
		for _, t := range TintSurfaces {
			on = append(on, tints[t])
		}
		moved := false
		for k, c := range src {
			next := readableOn(c, on, ink, TextFloor)
			if next != got[k] {
				got[k], moved = next, true
			}
		}
		if !moved {
			break
		}
	}

	v := map[string]string{
		"--ground": hexOf(ground), "--raise": hexOf(raise), "--sink": hexOf(sink),
		"--line":     hexOf(readableOn(rgbOf(p.Sep), []rgb{ground}, ink, LineFloor)),
		"--ink":      hexOf(ink),
		"--spot-ink": hexOf(ground),
		"--spot-dim": hexOf(mixRGB(got["--spot"], bg, 0.35)),
	}
	for k, c := range got {
		v[k] = hexOf(c)
	}
	for t, c := range tints {
		v[t] = hexOf(c)
	}
	v["--lift"] = "0 1px 1px rgba(0,0,0,.25), 0 10px 26px -14px rgba(0,0,0,.55)"
	if light {
		v["--lift"] = "0 1px 1px rgba(0,0,0,.05), 0 10px 26px -16px rgba(0,0,0,.18)"
	}
	return StudioTheme{Asked: name, Theme: resolved, Light: light, AccentFrom: accentFrom, Tokens: v}
}

// ---- colour arithmetic ----

type rgb struct{ r, g, b float64 }

func rgbOf(h string) rgb {
	if len(h) > 0 && h[0] == '#' {
		h = h[1:]
	}
	if len(h) != 6 {
		panic(fmt.Sprintf("widgetconfig: %q is not a six-digit colour", h))
	}
	n, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		panic(fmt.Sprintf("widgetconfig: %q is not a colour: %v", h, err))
	}
	return rgb{float64(n >> 16 & 0xff), float64(n >> 8 & 0xff), float64(n & 0xff)}
}

func hexOf(c rgb) string {
	q := func(x float64) int { return int(math.Round(math.Max(0, math.Min(255, x)))) }
	return fmt.Sprintf("#%02x%02x%02x", q(c.r), q(c.g), q(c.b))
}

// mixRGB blends a toward b by t, rounded to whole channels so a token is the colour it names.
func mixRGB(a, b rgb, t float64) rgb {
	m := func(x, y float64) float64 { return math.Round(x + (y-x)*t) }
	return rgb{m(a.r, b.r), m(a.g, b.g), m(a.b, b.b)}
}

// luminance is WCAG 2's relative luminance.
func luminance(c rgb) float64 {
	ch := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.r) + 0.7152*ch(c.g) + 0.0722*ch(c.b)
}

// Contrast is WCAG 2's contrast ratio between two "#rrggbb" or "rrggbb" colours.
func Contrast(a, b string) float64 { return contrast(rgbOf(a), rgbOf(b)) }

func contrast(a, b rgb) float64 {
	x, y := luminance(a), luminance(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

// readableOn moves c toward `toward` in 5% steps until it meets min on every surface, and
// returns `toward` itself when nothing short of it does.
func readableOn(c rgb, on []rgb, toward rgb, min float64) rgb {
	for i := 0; i <= 20; i++ {
		out := mixRGB(c, toward, float64(i)/20)
		ok := true
		for _, s := range on {
			if contrast(out, s) < min {
				ok = false
				break
			}
		}
		if ok {
			return out
		}
	}
	return toward
}

// Hue is a colour's hue in degrees, and false for a grey, which has none.
func Hue(h string) (float64, bool) { return hue(rgbOf(h)) }

func hue(c rgb) (float64, bool) {
	r, g, b := c.r/255, c.g/255, c.b/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	d := mx - mn
	if d == 0 {
		return 0, false
	}
	var x float64
	switch mx {
	case r:
		x = math.Mod((g-b)/d, 6)
	case g:
		x = (b-r)/d + 2
	default:
		x = (r-g)/d + 4
	}
	return math.Mod(x*60+360, 360), true
}

// HueGap is the distance in degrees between two colours' hues, and false when either is grey.
func HueGap(a, b string) (float64, bool) {
	x, okx := Hue(a)
	y, oky := Hue(b)
	if !okx || !oky {
		return 0, false
	}
	d := math.Abs(x - y)
	return math.Min(d, 360-d), true
}

func hueNear(a, b rgb, gap float64) bool {
	d, ok := HueGap(hexOf(a), hexOf(b))
	return ok && d < gap
}
