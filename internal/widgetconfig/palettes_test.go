package widgetconfig

import (
	"reflect"
	"strings"
	"testing"
)

// THE PALETTES ARE THE WIDGET'S OWN, value for value.
//
// They moved here from plugins/overlay/view.go so Studio could use them. This is the table that
// file held, copied by hand, so a value tuned here for Studio's sake (which would change what the
// widget draws) fails rather than ships.
func TestThePalettesAreTheWidgetsOwn(t *testing.T) {
	shipped := map[string][11]string{
		//                    bg        name      cmd       lastCmd   sep       desc      idle      listen    run       errc      logInfo
		"default":          {"0A0A0A", "00E5CC", "DDDDDD", "4A8A8A", "2A4A4A", "6A9A98", "A0A8A5", "5AB4F5", "4DC94D", "F56565", "8AC8DE"},
		"dracula":          {"282A36", "BD93F9", "F8F8F2", "6272A4", "44475A", "6272A4", "9291CD", "8BE9FD", "50FA7B", "FF5555", "8BE9FD"},
		"solarized-dark":   {"002B36", "2AA198", "EEE8D5", "2AA198", "0F4A5A", "586E75", "6F979B", "268BD2", "859900", "DC322F", "93A1A1"},
		"monokai":          {"272822", "A6E22E", "F8F8F2", "75715E", "3E3D32", "75715E", "9AA470", "66D9E8", "A6E22E", "F92672", "F8F8F2"},
		"nord":             {"2E3440", "88C0D0", "ECEFF4", "4C566A", "434C5E", "4C566A", "7287A0", "81A1C1", "A3BE8C", "BF616A", "88C0D0"},
		"tokyo-night":      {"1A1B26", "BB9AF7", "C0CAF5", "565F89", "292E42", "565F89", "6A6A97", "7AA2F7", "9ECE6A", "F7768E", "7AA2F7"},
		"catppuccin-mocha": {"1E1E2E", "CBA6F7", "CDD6F4", "6C7086", "313244", "7F849C", "8E8BAA", "89B4FA", "A6E3A1", "F38BA8", "89DCEB"},
		"gruvbox-dark":     {"282828", "FE8019", "EBDBB2", "7C6F64", "3C3836", "928374", "A78461", "83A598", "B8BB26", "FB4934", "83A598"},
		"rose-pine":        {"191724", "C4A7E7", "E0DEF4", "6E6A86", "26233A", "908CAA", "7F76A0", "9CCFD8", "31748F", "EB6F92", "9CCFD8"},
		"light":            {"FAFAFA", "6F42C1", "1A1A2E", "606060", "A0A8B0", "3A4A54", "505050", "0969DA", "1A7F37", "CF222E", "0969DA"},
	}
	if len(palettes) != len(shipped) {
		t.Errorf("there are %d palettes; the widget shipped %d", len(palettes), len(shipped))
	}
	for name, want := range shipped {
		_, p := PaletteFor(name)
		got := [11]string{p.Bg, p.Name, p.Cmd, p.LastCmd, p.Sep, p.Desc, p.Idle, p.Listen, p.Run, p.Errc, p.LogInfo}
		if _, ok := palettes[name]; !ok || got != want {
			t.Errorf("palette %q is %v; the widget shipped %v", name, got, want)
		}
	}
	// And the order the widget cycles is the order it always cycled.
	order := []string{"default", "dracula", "solarized-dark", "monokai", "nord",
		"tokyo-night", "catppuccin-mocha", "gruvbox-dark", "rose-pine", "light"}
	if !reflect.DeepEqual(PaletteNames(), order) {
		t.Errorf("PaletteNames is %v, want %v", PaletteNames(), order)
	}
}

// AN UNKNOWN NAME RESOLVES THE WAY THE WIDGET ALWAYS RESOLVED IT: case-insensitively, and to the
// default otherwise. Studio goes through the same function, so the two cannot disagree about
// what "Nord" or "solarised" or "" means.
func TestAThemeNameResolvesAsTheWidgetResolvesIt(t *testing.T) {
	for in, want := range map[string]string{
		"nord": "nord", "Nord": "nord", "LIGHT": "light", "": "default",
		"solarised": "default", " nord": "default", "default": "default",
	} {
		got, p := PaletteFor(in)
		if got != want || p != palettes[want] {
			t.Errorf("PaletteFor(%q) resolved to %q, want %q", in, got, want)
		}
		if st := StudioTokens(in); st.Theme != want || st.Asked != in {
			t.Errorf("StudioTokens(%q) says theme %q asked %q, want %q and %q", in, st.Theme, st.Asked, want, in)
		}
	}
	if !reflect.DeepEqual(StudioTokens("no such theme").Tokens, StudioTokens("default").Tokens) {
		t.Error("an unknown theme gives Studio different tokens from the default")
	}
}

// EVERY PALETTE GIVES STUDIO PAIRS THAT READ, measured on the pairs the page draws.
//
// Not each colour against the ground alone: text sits on cards, fields and tinted banners, and
// the accent is ordinary text in an outlined Play label, a link and a hint. So every text colour
// is measured on every surface; the label on an accent button on the accent and on the ink it
// turns to under the pointer; and the focus ring, which is the accent, against every surface.
func TestEveryPaletteGivesStudioReadablePairs(t *testing.T) {
	for _, name := range PaletteNames() {
		st := StudioTokens(name)
		tk := st.Tokens
		for _, k := range append(append(append([]string(nil), PlainSurfaces...), TintSurfaces...),
			append(TextTokens, "--line", "--spot-ink", "--spot-dim")...) {
			if !strings.HasPrefix(tk[k], "#") || len(tk[k]) != 7 {
				t.Fatalf("%s: token %s is %q, not a colour", name, k, tk[k])
			}
		}
		pair := func(fg, bg string, min float64, what string) {
			if r := Contrast(tk[fg], tk[bg]); r < min {
				t.Errorf("%s: %s (%s) on %s (%s) is %.2f:1, under %.1f:1, so %s",
					name, fg, tk[fg], bg, tk[bg], r, min, what)
			}
		}
		for _, fg := range TextTokens {
			for _, bg := range PlainSurfaces {
				min := TextFloor
				if fg == "--ink" {
					min = InkFloor
				}
				pair(fg, bg, min, "words in that colour do not read on that surface")
			}
			for _, bg := range TintSurfaces {
				pair(fg, bg, TextFloor, "words in that colour do not read on that banner or row")
			}
		}
		pair("--spot-ink", "--spot", TextFloor, "the primary button's label does not read")
		pair("--spot-ink", "--ink", TextFloor, "the primary button's label vanishes under the pointer")
		for _, bg := range PlainSurfaces {
			pair("--spot", bg, 3, "the focus ring is not visible")
		}
		pair("--line", "--ground", LineFloor, "dividers disappear")
		// A light palette is a light page and a dark one a dark page.
		if l := luminance(rgbOf(tk["--ground"])) > 0.5; l != st.Light {
			t.Errorf("%s: Light is %v for a ground of %s", name, st.Light, tk["--ground"])
		}
	}
}

// THE ACCENT IS NOT A SEMANTIC HUE, including where it fell back to `listen`.
//
// Swapping the role does not establish the difference: the fallback is a palette colour too, and
// the lifted tokens are what is drawn. So the final accent is measured against the final ok,
// warning and refusal, for every palette. A heuristic about meaning, not an accessibility
// guarantee; the labels carry the meaning.
func TestTheAccentStaysClearOfOkWarningAndRefusal(t *testing.T) {
	for _, name := range PaletteNames() {
		st := StudioTokens(name)
		for _, sem := range []string{"--ok", "--warn", "--no"} {
			d, ok := HueGap(st.Tokens["--spot"], st.Tokens[sem])
			if ok && d < SemanticHueGap {
				t.Errorf("%s: the accent %s (from %s) is %.0f° of hue from %s %s",
					name, st.Tokens["--spot"], st.AccentFrom, d, sem, st.Tokens[sem])
			}
		}
	}
	// The two palettes the render found colliding take their accent from `listen`, and the rest
	// keep their own.
	for _, name := range PaletteNames() {
		want := "name"
		if name == "gruvbox-dark" || name == "monokai" {
			want = "listen"
		}
		if got := StudioTokens(name).AccentFrom; got != want {
			t.Errorf("%s: the accent comes from %q, want %q", name, got, want)
		}
	}
}

// THE WARNING KEEPS ITS HUE. It is lifted for readability, and lifting moves it toward the ink,
// so it must still be the amber it started as and not drift to the ink's colour.
func TestTheWarningIsStillAmber(t *testing.T) {
	for _, name := range PaletteNames() {
		st := StudioTokens(name)
		base := warnOnDark
		if st.Light {
			base = warnOnLight
		}
		if d, ok := HueGap(st.Tokens["--warn"], base); !ok || d > 12 {
			t.Errorf("%s: the warning %s is %.0f° of hue from Studio's amber %s", name, st.Tokens["--warn"], d, base)
		}
	}
}
