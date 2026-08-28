package directorapi

import (
	"fmt"
	"testing"
)

// VISUAL PRESENCE IS NOT LEGAL ACTIONABILITY.
//
// # The hole this closes, before anything could fall into it
//
// `Actions()` derives capability from ROLE: a thing whose role is `button` is `Invokable`.
// That is a claim about the interface — something shaped like a button affords pressing — and
// it was also, until now, the whole of what `Targetable()` asked.
//
// So an element whose only evidence was a learned detector classifying a rectangle as `button`
// read as legally targetable, with nothing anywhere having claimed a mechanism to press it. It
// was safe only because the ScreenParser detector is shadow-only and its evidence never reaches
// fusion — a safety property depending on an experiment's configuration, which would have
// evaporated silently the moment that experiment was admitted.
//
// Affordance and capability are now separate questions and the second is decided from
// provenance.
//
// Deleting the Actuable clause from Targetable must fail this.
func TestVisualPresenceIsNotLegalActionability(t *testing.T) {
	seen := func(sources ...ObservationSource) *Element {
		el := &Element{
			Role: RoleButton, Label: "Save",
			Enabled: true, Visible: true,
			Bounds: Rect{X: 10, Y: 10, Width: 80, Height: 24},
		}
		for i, s := range sources {
			el.Provenance.Add(ObservationReference{
				Source:      s,
				Observation: ObservationID(fmt.Sprintf("obs-%d", i)),
			})
		}
		return el
	}

	// A CAMERA SAW IT. It affords pressing and Marco has no way to press it.
	camera := seen(SourceVision)
	if !camera.Actions().Affords() {
		t.Error("a visible enabled button affords nothing, so the fixture is wrong")
	}
	if camera.Actionable() {
		t.Error("an element only a visual detector reported is legally targetable. Its " +
			"role says it looks pressable; nothing said Marco can press it.")
	}
	if camera.Addressable() {
		t.Error("an element only a visual detector reported is addressable")
	}

	// AND SO DID AN OCR READER, which is the same kind of evidence.
	if seen(SourceVision, SourceOCR).Actionable() {
		t.Error("pixels and text about pixels are still only pixels")
	}

	// SOMETHING THAT CAN OPERATE IT SAW IT. Unchanged.
	for _, s := range []ObservationSource{
		SourceAccessibility, SourceNative,
		SourceDOM, SourcePlugin,
	} {
		if !seen(s).Actionable() {
			t.Errorf("an element reported by %s is not targetable; the firewall is "+
				"refusing evidence that does say how to operate a control", s)
		}
		// AND CORROBORATION BY A CAMERA DOES NOT TAKE IT AWAY.
		if !seen(s, SourceVision).Actionable() {
			t.Errorf("%s plus a corroborating detector became untargetable", s)
		}
	}

	// AND AN ELEMENT WITH NO PROVENANCE IS UNCHANGED. "Nobody recorded where this came
	// from" is a hand-built query or a capability pack's enrichment; it is not the claim
	// "only a camera saw it", and refusing it would break every caller that constructs an
	// element rather than observing one.
	plain := &Element{
		Role: RoleButton, Label: "Save", Enabled: true, Visible: true,
		Bounds: Rect{X: 10, Y: 10, Width: 80, Height: 24},
	}
	if !plain.Actionable() {
		t.Error("an element with no provenance stopped being targetable")
	}
}

// AND THE LIST OF SOURCES THAT CAN OPERATE A CONTROL IS EXPLICIT.
//
// A rank comparison would silently admit whatever source somebody adds next. This is short,
// stated, and each entry is a source that exposes an invoke/toggle/select mechanism or speaks
// for an application that can act on its own behalf.
func TestOnlySourcesThatCanOperateAControlSaySo(t *testing.T) {
	for _, s := range []ObservationSource{
		SourceNative, SourceDOM,
		SourceAccessibility, SourcePlugin,
	} {
		if !ActuatingSource(s) {
			t.Errorf("%s cannot say how to operate a control", s)
		}
	}
	for _, s := range []ObservationSource{
		SourceVision, SourceOCR,
		SourceModel, SourceWindowSystem,
	} {
		if ActuatingSource(s) {
			t.Errorf("%s describes what the screen looks like and is treated as though "+
				"it knew how to work it", s)
		}
	}
}

// A WINDOW WITH ACCESSIBILITY IS NOT A WINDOW SEEN ONLY BY PIXELS.
//
// The diagnostic that separates the two halves of Blind() has to be able to say NO, or it
// describes every window as camera-only and the more specific refusal becomes noise on windows
// that are simply empty.
//
// Deleting the actuating-source check must fail this.
func TestAWindowWithAccessibilityIsNotSeenOnlyByPixels(t *testing.T) {
	button := func(id ElementID, sources ...ObservationSource) *Element {
		el := &Element{
			ID: id, Role: RoleButton, Label: "Save", Enabled: true, Visible: true,
			Bounds: Rect{X: 10, Y: 10, Width: 80, Height: 24},
		}
		for i, s := range sources {
			el.Provenance.Add(ObservationReference{
				Source: s, Observation: ObservationID(fmt.Sprintf("o%d", i)),
			})
		}
		return el
	}

	pixels := &WorldState{Elements: map[ElementID]*Element{
		"a": button("a", SourceVision),
		"b": button("b", SourceVision, SourceOCR),
	}}
	if !pixels.SeenOnlyByPixels() {
		t.Error("a window whose every control only a camera saw does not say so")
	}

	// ONE ELEMENT WITH ACCESSIBILITY IS ENOUGH TO SAY NO. The claim is about the window,
	// and a window with any operable evidence in it is not a window Marco cannot work.
	mixed := &WorldState{Elements: map[ElementID]*Element{
		"a": button("a", SourceVision),
		"b": button("b", SourceAccessibility),
	}}
	if mixed.SeenOnlyByPixels() {
		t.Error("a window with an accessibility-reported control is described as though " +
			"only a camera saw it. That refusal would then appear on ordinary windows.")
	}

	// AND AN EMPTY WINDOW IS NOT ONE SEEN BY A CAMERA EITHER. It is empty, and there is
	// nothing to explain.
	if (&WorldState{Elements: map[ElementID]*Element{}}).SeenOnlyByPixels() {
		t.Error("an empty window is described as seen only by pixels")
	}
	// NOR IS ONE HOLDING ONLY SCENERY. A heading affords nothing, so a window of headings
	// has nothing operable to explain away.
	scenery := &WorldState{Elements: map[ElementID]*Element{
		"h": {
			ID: "h", Role: RoleText, Label: "Settings", Visible: true, Enabled: true,
			Bounds: Rect{X: 1, Y: 1, Width: 50, Height: 10},
		},
	}}
	if scenery.SeenOnlyByPixels() {
		t.Error("a window of scenery is described as seen only by pixels")
	}
}
