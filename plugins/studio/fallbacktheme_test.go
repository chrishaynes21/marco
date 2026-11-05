package main

// THE FRAME BEFORE THE THEME ARRIVES IS THE DEFAULT PALETTE.
//
// Studio takes every colour from the widget's palette (S10): `acts.Theme()` answers and
// `applyTheme` sets each token. Until it answers, or if it never does, the stylesheet's own
// values are drawn. Those values are the widget's DEFAULT palette for dark and its LIGHT palette
// for light, exactly as `widgetconfig.StudioTokens` derives them. So the control centre's cyan
// is the default palette's accent and not a colour this file chose, and anybody on the default
// theme sees no change of colour when the answer lands. See
// [[ADR-310-studio-wears-the-control-centres-look]].

import (
	"regexp"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

func TestThePagesFallbackColoursAreTheWidgetsDefaultAndLightPalettes(t *testing.T) {
	page := thePage(t)
	css := page[:strings.Index(page, "</style>")]

	// THE DARK SIDE is the first `:root{` block; the light side is written twice, once
	// under the desktop's light preference and once for an explicit light choice.
	dark := blockAfter(t, css, " :root{")
	lightPref := blockAfter(t, css, `:root:not([data-theme="dark"]){`)
	lightPick := blockAfter(t, css, `:root[data-theme="light"]{`)

	for _, c := range []struct {
		what, block, palette string
	}{
		{"the dark fallback", dark, "default"},
		{"the light fallback under a light desktop", lightPref, "light"},
		{"the light fallback for an explicit light choice", lightPick, "light"},
	} {
		want := widgetconfig.StudioTokens(c.palette).Tokens
		for name, value := range want {
			if name == "--lift" {
				continue // a shadow, tuned per side and not a palette colour
			}
			got := tokenIn(c.block, name)
			if got == "" {
				t.Errorf("%s does not set %s at all", c.what, name)
				continue
			}
			if !strings.EqualFold(got, value) {
				t.Errorf("%s sets %s to %s, and the %s palette makes it %s. The "+
					"frame before the theme arrives would then be a different "+
					"window from the one after it.", c.what, name, got,
					c.palette, value)
			}
		}
	}
}

// blockAfter is the declarations between `open` and the next closing brace.
func blockAfter(t *testing.T, css, open string) string {
	t.Helper()
	i := strings.Index(css, open)
	if i < 0 {
		t.Fatalf("the stylesheet has no %q block", open)
	}
	rest := css[i+len(open):]
	return rest[:strings.Index(rest, "}")]
}

// tokenIn is the value a block gives one custom property, or "".
func tokenIn(block, name string) string {
	m := regexp.MustCompile(regexp.QuoteMeta(name) + `:\s*([^;]+);`).FindStringSubmatch(block)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}
