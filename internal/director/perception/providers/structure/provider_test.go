package structure_test

// WHAT A PROPOSED RECTANGLE IS ALLOWED TO CLAIM.
//
// The provider exists so a surface with no accessibility tree stops producing zero objects —
// [[Experiment-051-the-stage-two-real-applications-produced]] measured 46 observations and no
// Props on a game menu. What it must never do is turn 228 rectangles into 228 controls, and
// every guard against that is a property of what it emits rather than a rule somewhere else.

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	structureprov "github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/structure"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// drawnPanels is a frame with two filled rectangles on a flat ground — the shape the proposer
// exists to find, and nothing else.
func drawnPanels() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	ground := color.RGBA{R: 20, G: 20, B: 24, A: 255}
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, ground)
		}
	}
	panel := color.RGBA{R: 230, G: 230, B: 235, A: 255}
	for _, r := range []image.Rectangle{
		image.Rect(40, 40, 240, 80),
		image.Rect(40, 120, 240, 160),
	} {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.Set(x, y, panel)
			}
		}
	}
	return img
}

type frame struct{ img image.Image }

func (f frame) CaptureWindow(context.Context, directorapi.Window) (capture.Image, error) {
	b := f.img.Bounds()
	return capture.Image{
		Image: f.img, Scale: 1, CapturedAt: time.Now(),
		Bounds:   directorapi.Rect{Width: b.Dx(), Height: b.Dy()},
		WindowID: "hwnd:1", Application: "fixture",
	}, nil
}

func provider() *structureprov.Provider {
	return structureprov.New(frame{drawnPanels()},
		func(context.Context) (directorapi.Window, bool) {
			return directorapi.Window{ID: "hwnd:1", Application: "fixture",
				Bounds: directorapi.Rect{Width: 400, Height: 300}}, true
		})
}

// IT COSTS A CAPTURE, SO IT IS ASKED FOR.
//
// An empty Sources list means "what the providers can CHEAPLY see". A pass over every pixel of
// a frame is not that, and a provider that ran anyway would make every command pay for it.
func TestNothingIsProposedUnlessTheRequestAsksForIt(t *testing.T) {
	got, err := provider().Observe(context.Background(), observation.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("%d observation(s) from a request that did not ask for structure",
			len(got))
	}
}

// AND WHEN IT IS ASKED, IT CLAIMS A SHAPE AND NOTHING ELSE.
func TestAProposedRegionClaimsNoRoleAndNoName(t *testing.T) {
	got, err := provider().Observe(context.Background(), observation.Request{
		Sources: []observation.Source{directorapi.SourceStructure}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("two drawn panels produced no proposal at all")
	}
	for _, o := range got {
		raw := o.(observation.Element).Raw
		switch {
		case raw.Source != directorapi.SourceStructure:
			t.Errorf("source is %q", raw.Source)
		case raw.Role != directorapi.RoleUnknown:
			t.Errorf("a proposed rectangle claims the role %q; the proposer "+
				"cannot say what a shape IS", raw.Role)
		case raw.Label != "":
			t.Errorf("a proposed rectangle claims the name %q", raw.Label)
		case raw.Bounds.Empty():
			t.Error("a proposal with no bounds says where nothing is")
		}
		// AND IT CARRIES ITS OWN EVIDENCE, rather than reducing it to one number.
		for _, k := range []string{"structure_kind", "structure_fill", "structure_parts"} {
			if _, ok := raw.Attributes[k]; !ok {
				t.Errorf("the proposal dropped %s", k)
			}
		}
	}
}

// AND NOTHING IT REPORTS MAY BE OPERATED.
//
// Two independent reasons, and the test asserts both because either alone would be enough and
// losing one silently is how this becomes unsafe: the source is not an actuating one, and the
// role affords nothing.
func TestAProposedRegionIsNotOperableOnItsOwnEvidence(t *testing.T) {
	if directorapi.ActuatingSource(directorapi.SourceStructure) {
		t.Fatal("the shape proposer is listed as a source that can say how to " +
			"OPERATE a control; it reports rectangles")
	}
	got, err := provider().Observe(context.Background(), observation.Request{
		Sources: []observation.Source{directorapi.SourceStructure}})
	if err != nil || len(got) == 0 {
		t.Fatalf("no proposals: %v", err)
	}
	raw := got[0].(observation.Element).Raw
	el := &directorapi.Element{
		ID: "e1", Role: raw.Role, Label: raw.Label, Bounds: raw.Bounds,
		Enabled: true, Visible: true,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: raw.ID, Source: raw.Source})

	if el.Actions().Actuable {
		t.Error("an element only the proposer reported is Actuable")
	}
	if el.Actions().Any() {
		t.Error("an element with no role affords something")
	}
	if el.Actions().Targetable() {
		t.Error("a proposed rectangle is a valid click target")
	}
	// AND AN EXPLICIT INSTRUCTION DOES NOT CHANGE THAT. The licence of ADR-202 governs
	// whether a pixel-only Prop may be operated; it cannot give a shape a capability.
	lic := directorapi.OperateLicence{Instruction: "click it", Verb: "click"}
	if ok, _ := el.OperableUnder(lic, "click"); ok && el.Actions().Any() {
		t.Error("an explicit instruction made a bare rectangle clickable")
	}
}
