package observe

import "testing"

// A place is not its viewport.
//
// # The measurement
//
// Windows Settings, one window, three heights. The accessibility tree is 155 elements at every
// one of them — CONSTANT. What moves is which items report a rectangle, because an item scrolled
// out of the viewport reports `0x0`:
//
//	h=700   zero-area: button 9  group 16 list_item 7 combo_box 1
//	h=900   zero-area: button 7  group 8  list_item 1 combo_box 1
//	h=1040  zero-area: button 6  group 7  list_item 0 combo_box 0
//
// While extent decided membership, the same Home established as three different places at three
// window sizes, and every twin family in the live store turned out to be one page seen at two
// sizes.
//
// The compositions below are the measured page content at each height, chrome excluded.

func region(i int, area bool) Region {
	if !area {
		// Exactly what an off-viewport item reports.
		return Region{}
	}
	return Region{X: 0.1, Y: 0.05 + float64(i%15)*0.05, Width: 0.05, Height: 0.02}
}

// atHeight builds one page's regions with `offscreen` of them reporting no rectangle.
func atHeight(roles map[string]int, offscreen map[string]int) []ShadowRegion {
	var out []ShadowRegion
	for role, n := range roles {
		gone := offscreen[role]
		for i := 0; i < n; i++ {
			out = append(out, ShadowRegion{Role: role, Region: region(i, i >= gone)})
		}
	}
	return out
}

func homeRoles() map[string]int {
	return map[string]int{"button": 18, "combo_box": 1, "group": 27, "image": 14, "link": 3,
		"list": 2, "list_item": 22, "menu": 1, "menu_item": 1, "pane": 4, "text": 49,
		"text_field": 1, "unknown": 1, "window": 4}
}
func bluetoothRoles() map[string]int {
	return map[string]int{"button": 10, "group": 12, "image": 13, "list": 2, "list_item": 21,
		"menu": 1, "menu_item": 1, "pane": 4, "text": 26, "text_field": 1, "unknown": 1,
		"window": 4}
}
func mouseRoles() map[string]int {
	return map[string]int{"button": 14, "combo_box": 3, "group": 14, "image": 13, "link": 6,
		"list_item": 15, "menu": 1, "menu_item": 1, "pane": 4, "slider": 2, "text": 29,
		"text_field": 1, "unknown": 1, "window": 4}
}

// The three measured viewports, as counts of items that report no rectangle.
var (
	small  = map[string]int{"button": 9, "group": 16, "list_item": 7, "combo_box": 1, "text": 20}
	medium = map[string]int{"button": 7, "group": 8, "list_item": 1, "combo_box": 1, "text": 20}
	large  = map[string]int{"button": 6, "group": 7, "text": 17}
)

func placeAt(roles, offscreen map[string]int, terms []InterfaceTerm) StructureSignature {
	return StructureSignature{Subject: SubjectState,
		Roles: NewScreenSignature(atHeight(roles, offscreen)).Roles,
		Terms: terms, TermsKnown: true}
}

var backSettings = []InterfaceTerm{TermBack, TermSettings}
var backControls = []InterfaceTerm{TermBack, TermControls, TermSettings}

// The same page at three window heights is ONE place.
//
// Reinstating an extent test in the projection must fail this.
func TestOnePageAtThreeViewportsIsOnePlace(t *testing.T) {
	for _, p := range []struct {
		name  string
		roles map[string]int
		terms []InterfaceTerm
	}{
		{"Home", homeRoles(), backSettings},
		{"Bluetooth", bluetoothRoles(), backSettings},
		{"Mouse", mouseRoles(), backControls},
	} {
		s := placeAt(p.roles, small, p.terms)
		m := placeAt(p.roles, medium, p.terms)
		l := placeAt(p.roles, large, p.terms)
		for _, pair := range []struct {
			a, b StructureSignature
			what string
		}{{s, m, "small vs medium"}, {m, l, "medium vs large"}, {s, l, "small vs large"}} {
			if c := ExplainStructure(pair.a, pair.b); c.Verdict != MatchSame {
				t.Errorf("%s %s => %s.\nThe window's height has been written into the "+
					"page's identity, so resizing invents a new place.\nwhy: %+v",
					p.name, pair.what, c.Verdict, c.Why)
			}
		}
	}
}

// And three different pages stay three places, at every pairing of viewports.
//
// The hard half. A projection that made Home and Bluetooth agree would be worse than the bug it
// replaced: a route through one would be credited with arriving at the other.
func TestThreePagesStayDistinctAtEveryViewport(t *testing.T) {
	pages := []struct {
		name  string
		roles map[string]int
		terms []InterfaceTerm
	}{
		{"Home", homeRoles(), backSettings},
		{"Bluetooth", bluetoothRoles(), backSettings},
		{"Mouse", mouseRoles(), backControls},
	}
	views := []struct {
		name string
		off  map[string]int
	}{{"small", small}, {"medium", medium}, {"large", large}}

	for i := 0; i < len(pages); i++ {
		for j := i + 1; j < len(pages); j++ {
			for _, va := range views {
				for _, vb := range views {
					a := placeAt(pages[i].roles, va.off, pages[i].terms)
					b := placeAt(pages[j].roles, vb.off, pages[j].terms)
					if ExplainStructure(a, b).Verdict == MatchSame {
						t.Errorf("%s@%s and %s@%s are the same place",
							pages[i].name, va.name, pages[j].name, vb.name)
					}
				}
			}
		}
	}
}

// A control with no rectangle is part of the page, and contributes no ARRANGEMENT.
//
// Roles always; cells only where there is a rectangle to speak of. Piling every off-viewport item
// at the origin would distort the arrangement the segmenter compares states by.
func TestAnOffViewportControlCountsButIsNotPlaced(t *testing.T) {
	sig := NewScreenSignature([]ShadowRegion{
		{Role: "button", Region: Region{X: 0.1, Y: 0.1, Width: 0.05, Height: 0.02}},
		{Role: "button", Region: Region{}},
	})
	if sig.Roles["button"] != 2 {
		t.Errorf("an off-viewport button was dropped from the page: %v", sig.Roles)
	}
	if len(sig.Cells) != 1 {
		t.Errorf("%d cell(s); a control with no rectangle sits nowhere and must not be "+
			"placed at the origin", len(sig.Cells))
	}
}

// Viewport size itself is never stored.
//
// A resized page is the same place. Nothing about width, height or a viewport class may become a
// discriminator.
func TestNoViewportDimensionReachesTheSignature(t *testing.T) {
	sig := placeAt(homeRoles(), small, backSettings)
	for role := range sig.Roles {
		switch role {
		case "viewport", "width", "height", "size", "small", "medium", "large":
			t.Errorf("the signature carries %q", role)
		}
	}
	// And the same page at another viewport produces the identical stored value.
	if got, want := len(sig.Roles), len(placeAt(homeRoles(), large, backSettings).Roles); got != want {
		t.Errorf("the role set differs by viewport: %d vs %d", got, want)
	}
}
