package observe

import "testing"

// A window is not a place.
//
// # The live over-mint, reproduced
//
// Windows Settings minted a fresh durable subject for the same page on almost every visit. Three
// families of twins in one real store each differed from their named original by exactly three
// buttons, and `director inspect -chrome` named them: the frame's own Minimize, Restore and Close,
// which the accessibility hierarchy reports under a `title_bar`.
//
// The fixture is that shape — a page, a title bar with its three caption buttons, and a scroll bar
// with its arrows. Whether the frame parts come back in a snapshot is a fact about the tree walk,
// so a signature that counts them is a signature that changes when nothing on the page did.

// page is the composition of the screen itself, and never varies here.
func page() []ShadowRegion {
	var out []ShadowRegion
	add := func(role string, n int) {
		for i := 0; i < n; i++ {
			out = append(out, ShadowRegion{Role: role,
				Region: Region{X: 0.1, Y: 0.1 + float64(i)*0.01, Width: 0.05, Height: 0.02}})
		}
	}
	add("button", 10)
	add("list_item", 20)
	add("group", 10)
	add("text", 24)
	return out
}

// frame is the window's own machinery, classified by hierarchy upstream.
func frame() []ShadowRegion {
	var out []ShadowRegion
	add := func(role string, n int) {
		for i := 0; i < n; i++ {
			out = append(out, ShadowRegion{Role: role, Chrome: true,
				Region: Region{X: 0.9, Y: float64(i) * 0.01, Width: 0.02, Height: 0.02}})
		}
	}
	add("title_bar", 1)
	add("button", 3) // Minimize, Restore, Close
	add("scroll_bar", 1)
	add("button", 4) // the scroll bar's arrows
	return out
}

// The same page is the same signature whether or not the frame came back.
//
// Deleting the chrome skip in NewScreenSignature must fail this, with button 17 against 10 —
// which is the live symptom that produced three twin families.
func TestAWindowsOwnMachineryIsNotPartOfThePlace(t *testing.T) {
	bare := NewScreenSignature(page())
	framed := NewScreenSignature(append(page(), frame()...))

	for role, n := range bare.Roles {
		if framed.Roles[role] != n {
			t.Errorf("role %s: %d without the frame, %d with it", role, n, framed.Roles[role])
		}
	}
	for role, n := range framed.Roles {
		if _, ok := bare.Roles[role]; !ok {
			t.Errorf("the frame contributed %d %s to the page's identity", n, role)
		}
	}
	if bare.Total != framed.Total {
		t.Errorf("total %d without the frame, %d with it", bare.Total, framed.Total)
	}
}

// And the matcher then calls them the same place, which is the whole point.
func TestTheSamePageWithAndWithoutItsFrameRecallsTheSame(t *testing.T) {
	sig := func(r []ShadowRegion) StructureSignature {
		return StructureSignature{Subject: SubjectState, Roles: NewScreenSignature(r).Roles,
			Terms: []InterfaceTerm{TermBack, TermSettings}, TermsKnown: true}
	}
	c := ExplainStructure(sig(append(page(), frame()...)), sig(page()))
	if c.Verdict != MatchSame {
		t.Fatalf("the same page compares %s depending on whether its window frame was "+
			"walked.\nEvery visit therefore mints a new subject and a learned route can "+
			"never be attempted again.\nwhy: %+v", c.Verdict, c.Why)
	}
}

// PAGE CONTENT IS NEVER FILTERED BY GEOMETRY.
//
// The rule that replaced a geometric one, and the reason it had to. Windows Settings reports
// legitimate content — a combo box, links, nineteen pieces of text — with `0,0 0x0` bounds, and an
// earlier zero-area exclusion stripped them and broke recall on real screens.
//
// Excluding zero-area content must fail this.
func TestPageContentWithNoRectangleIsStillPartOfThePlace(t *testing.T) {
	odd := []ShadowRegion{
		{Role: "combo_box", Region: Region{X: 0.2, Y: 0.2}},
		{Role: "link", Region: Region{X: 0.3, Y: 0.3}},
		{Role: "text", Region: Region{X: 0.4, Y: 0.4}},
	}
	sig := NewScreenSignature(odd)
	for _, role := range []string{"combo_box", "link", "text"} {
		if sig.Roles[role] != 1 {
			t.Errorf("page content with no rectangle was dropped: %s counted %d.\n"+
				"Geometry describes how something is laid out at one instant; ownership "+
				"describes what it belongs to, and only the second is the question.",
				role, sig.Roles[role])
		}
	}
}

// Two genuinely different pages stay different.
//
// The failure this design fears most is over-merging. Excluding the frame must not make two
// screens compare equal — only the same screen compare equal to itself.
func TestTwoDifferentPagesRemainDistinct(t *testing.T) {
	other := append(page(), ShadowRegion{Role: "slider",
		Region: Region{X: 0.5, Y: 0.5, Width: 0.1, Height: 0.02}})
	sig := func(r []ShadowRegion) StructureSignature {
		return StructureSignature{Subject: SubjectState, Roles: NewScreenSignature(r).Roles,
			Terms: []InterfaceTerm{TermBack, TermSettings}, TermsKnown: true}
	}
	if c := ExplainStructure(sig(other), sig(page())); c.Verdict == MatchSame {
		t.Error("two pages differing by a real control compare the same; excluding the " +
			"frame has made identity fuzzier, which it must never do")
	}
}

// A chrome control is still OBSERVED. It is left out of one thing only.
//
// Marco must still be able to see and press Close. The classification decides what makes a screen
// that screen, not what exists.
func TestChromeIsStillObservedAndOnlyLeftOutOfIdentity(t *testing.T) {
	regions := append(page(), frame()...)
	var chrome int
	for _, r := range regions {
		if r.Chrome {
			chrome++
		}
	}
	if chrome != len(frame()) {
		t.Fatalf("%d chrome region(s) survived into the sample, want %d", chrome, len(frame()))
	}
	// ...and the identity projection is the only thing that skips them.
	if got := NewScreenSignature(regions).Total; got != NewScreenSignature(page()).Total {
		t.Errorf("identity counted %d, the page has %d", got, NewScreenSignature(page()).Total)
	}
}

// Three real Settings pages stay three places.
//
// The compositions below are measured, not invented: they are the durable signatures a live store
// held for Home, Bluetooth & devices and Mouse. Excluding the window's machinery must not bring
// any two of them together — over-merging is the failure this design fears most, because two
// screens that differ only in what they are FOR would become one place and a route through either
// would be promoted for arriving at the other.
func TestThreeRealSettingsPagesStayThreePlaces(t *testing.T) {
	place := func(terms []InterfaceTerm, roles map[string]int) StructureSignature {
		return StructureSignature{Subject: SubjectState, Roles: roles,
			Terms: terms, TermsKnown: true}
	}
	backSettings := []InterfaceTerm{TermBack, TermSettings}
	backControls := []InterfaceTerm{TermBack, TermControls, TermSettings}

	pages := map[string]StructureSignature{
		"Home": place(backSettings, map[string]int{
			"button": 15, "combo_box": 1, "group": 20, "image": 14, "link": 1,
			"list": 2, "list_item": 22, "pane": 3, "text": 32, "text_field": 1,
			"unknown": 1, "window": 4}),
		"Bluetooth": place(backSettings, map[string]int{
			"button": 10, "group": 10, "image": 13, "list": 1, "list_item": 20,
			"menu": 1, "menu_item": 1, "pane": 3, "text": 24, "text_field": 1,
			"unknown": 2, "window": 4}),
		"Mouse": place(backControls, map[string]int{
			"button": 13, "combo_box": 3, "group": 9, "image": 13, "list_item": 15,
			"menu": 1, "menu_item": 1, "pane": 3, "slider": 2, "text": 21,
			"text_field": 1, "unknown": 2, "window": 4}),
	}
	names := []string{"Home", "Bluetooth", "Mouse"}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			a, b := names[i], names[j]
			if c := ExplainStructure(pages[a], pages[b]); c.Verdict == MatchSame {
				t.Errorf("%s and %s are the same place.\nTwo Settings pages merging is "+
					"worse than two twins: a route through one would be credited with "+
					"arriving at the other.", a, b)
			}
		}
	}
	// And each is itself, with its window's machinery attached or not.
	for _, n := range names {
		withFrame := place(pages[n].Terms, map[string]int{})
		for role, k := range pages[n].Roles {
			withFrame.Roles[role] = k
		}
		if c := ExplainStructure(withFrame, pages[n]); c.Verdict != MatchSame {
			t.Errorf("%s does not recall itself: %s (%+v)", n, c.Verdict, c.Why)
		}
	}
}
