//go:build corpus

package main

// THE WHOLE PATH, FROM A REAL CAPTURE TO A REAL DECISION.
//
// [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] was validated
// against hand-built Stages: elements constructed in a test, with provenance written by hand. That
// proves the policy and nothing about the integration, and the owner was exact about the
// difference:
//
//	Avoid hand-constructing Props for the primary tests.
//
// So this builds a Stage the way the Director builds one — the real `ocr.Provider` and the real
// `vision.Provider`, run by the real `providers.Collector`, fused by the real `fusion.Engine` —
// and the only substitution is WHERE THE PIXELS COME FROM. `capture.WindowCapture` is a
// one-method interface, so a recorded frame is served through it with no production change at
// all.
//
// What a recording can establish is which Prop the Director selects and where it would press.
// It cannot establish that the application would respond, and nothing here claims it does.
//
// Behind `corpus`, and it skips rather than fails when a plugin is absent: the OCR bridge and
// the ONNX detector are separate builds, and a machine without them has not failed this test.

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	ocrprov "github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	structureprov "github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/structure"
	visionprov "github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/platform/ocrclient"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"

	_ "image/png"
)

const tmRecording = `../../.tmp/game-capture-review/tm-menu-steps-sanitized`

// recordedFrame serves one decoded PNG wherever the Director would capture a window.
//
// The one substitution in this test, and the only one possible without rewriting perception:
// `capture.WindowCapture` asks for a window and gets back an image, and a recording is a
// sequence of images of a window. Nothing downstream can tell the difference, which is the
// point — if it could, this would be testing a different pipeline from the one that runs.
type recordedFrame struct {
	img  image.Image
	size directorapi.Rect
}

func (r recordedFrame) CaptureWindow(_ context.Context, w directorapi.Window) (
	capture.Image, error) {

	// STAMPED NOW, because a capture older than two seconds is correctly refused as
	// stale — the recording's own clock is from another day, and an image nobody took
	// recently is not evidence about anything. The pixels are the recording's; the
	// moment is this one, and the test says so rather than back-dating the guard.
	return capture.Image{
		Image: r.img, Bounds: r.size, Scale: 1, CapturedAt: time.Now(),
		WindowID: w.ID, Application: w.Application,
	}, nil
}

func loadFrame(t *testing.T, path string) recordedFrame {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("the recording is not here: %v", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	return recordedFrame{img: img,
		size: directorapi.Rect{Width: b.Dx(), Height: b.Dy()}}
}

// theWindow is the one window on this Stage: the recorded application, in front.
func theWindow(size directorapi.Rect) directorapi.Window {
	return directorapi.Window{
		ID: "hwnd:recorded", Application: "trackmania", Title: "Trackmania",
		Bounds: size, Focused: true, Visible: true,
	}
}

// realOCR builds the production OCR provider, or skips.
func realOCR(t *testing.T, cap capture.WindowCapture, win directorapi.Window) (
	*ocrprov.Provider, *ocrclient.Engine) {
	t.Helper()
	bridge := os.Getenv("DIRECTOR_OCR")
	if bridge == "" {
		for _, c := range []string{
			filepath.Join("..", "..", "plugins", "ocr", "ocr.exe"),
			filepath.Join("..", "..", "plugins", "ocr", "ocr"),
		} {
			if _, err := os.Stat(c); err == nil {
				bridge = c
				break
			}
		}
	}
	if bridge == "" {
		t.Skip("no OCR plugin: build plugins/ocr or set $DIRECTOR_OCR")
	}
	h := bridgehost.New(bridge)
	for _, c := range []string{
		os.Getenv("MARCO_TESSERACT"),
		`C:\Program Files\Tesseract-OCR\tesseract.exe`,
	} {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			h = h.WithEnv("MARCO_TESSERACT=" + c)
			break
		}
	}
	engine := ocrclient.New(h)
	return ocrprov.New(engine, cap,
		func(context.Context) (directorapi.Window, bool) { return win, true }), engine
}

// realVision builds the production vision provider over the vendored detector, or skips.
func realVision(t *testing.T, cap capture.WindowCapture, win directorapi.Window,
	engine *ocrclient.Engine) *visionprov.Provider {

	t.Helper()
	exe := filepath.Join("..", "..", "plugins", "vision", "vision.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Skip("no vision plugin: run plugins/vision/build.cmd")
	}
	// THE PRODUCTION WIRING, pointed at the vendored model and runtime by absolute
	// path: `newVisionDetector` finds them relative to the working directory, which for
	// a test is this package rather than the repository root.
	for env, rel := range map[string]string{
		"MARCO_SCREENPARSER_MODEL": filepath.Join("..", "..", "tools",
			"vision-export", "weights", "screenparser-1280.onnx"),
		"MARCO_ONNXRUNTIME": filepath.Join("..", "..", "tools", "onnxruntime",
			"onnxruntime-win-x64-1.28.0", "lib", "onnxruntime.dll"),
	} {
		if os.Getenv(env) != "" {
			continue
		}
		abs, err := filepath.Abs(rel)
		if err != nil || !exists(abs) {
			t.Skipf("%s is not vendored here (%s)", env, rel)
		}
		t.Setenv(env, abs)
	}
	det, host, why := newVisionDetector(exe)
	if det == nil {
		t.Skip("the vision detector is not usable here: " + why)
	}
	t.Cleanup(func() {
		if host != nil {
			host.Close()
		}
	})
	eyes := visionprov.New(det, cap,
		func(context.Context) (directorapi.Window, bool) { return win, true })
	// THE LABEL READER, which the runtime attaches and without which every
	// non-structural detection is rejected as "nothing readable inside". A box with no
	// name is a shape nobody can ask for, and on a surface like this one there is
	// nothing else to supply a name.
	eyes.Reader = newLabelReader(engine)
	return eyes
}

// stageFrom runs one real observation cycle over a recorded frame and fuses it.
func stageFrom(t *testing.T, frame recordedFrame, withVision bool) (
	directorapi.WorldState, observation.Cycle) {

	t.Helper()
	win := theWindow(frame.size)
	// NO WINDOW-SYSTEM PROVIDER: it is a refiner rather than a producer — the platform's
	// enumeration lives inside the accessibility bridge — and there is no accessibility
	// on this surface at all. The window itself is stated below, as the recording's own
	// metadata; it is not evidence about any control.
	reader, engine := realOCR(t, frame, win)
	// AND THE GENERIC SHAPE PROPOSER, the production provider, unmodified.
	shapes := structureprov.New(frame,
		func(context.Context) (directorapi.Window, bool) { return win, true })
	sources := []observation.Provider{reader, shapes}
	var eyes *visionprov.Provider
	if withVision {
		eyes = realVision(t, frame, win, engine)
		sources = append(sources, eyes)
	}
	// ASKED FOR EXPLICITLY. An empty Sources list means "everything the providers can
	// CHEAPLY see", and neither a screen capture plus a recognition pass nor a detector
	// inference is cheap — so an ordinary command cycle runs neither. On a surface with
	// no accessibility at all, they are the only sources there are.
	req := observation.Request{Sources: []observation.Source{
		directorapi.SourceOCR, directorapi.SourceStructure}}
	if withVision {
		req.Sources = append(req.Sources, directorapi.SourceVision)
	}
	cycle := providers.NewCollector(sources...).Collect(context.Background(), req)
	if _, sd, serr := shapes.Propose(context.Background(), req); true {
		t.Logf("structure: %dx%s accepted %d of %d component(s) "+
			"(small %d, large %d, thin %d, not-rect %d, below-scale %d) err=%v",
			1, sd.ImageSize, sd.Counters.Accepted, sd.Counters.Components,
			sd.Counters.TooSmall, sd.Counters.TooLarge, sd.Counters.TooThin,
			sd.Counters.NotRectangular, sd.Counters.BelowHumanScale, serr)
	}
	if eyes != nil {
		_, vd, verr := eyes.Look(context.Background(), req)
		t.Logf("vision: %+v err=%v", vd, verr)
	}
	// WHAT THE READER ITSELF SAYS IT DID. Asked always, because "the Stage saw
	// nothing" and "the reader never ran" are different findings and the cycle alone
	// cannot tell them apart.
	if _, diag, err := reader.Read(context.Background(), req); true {
		t.Logf("ocr: available=%v engine=%q window=%q image=%s counters=%+v err=%v %s",
			diag.Available, diag.Engine, diag.Application, diag.ImageSize,
			diag.Counters, err, diag.Unavailable+diag.Error)
	}
	world, report, err := fusion.NewEngine().Fuse(cycle)
	if err != nil {
		t.Fatal(err)
	}
	byKind := map[observation.Kind]int{}
	for _, o := range cycle.Observations {
		byKind[o.Kind()]++
	}
	t.Logf("fusion: %d observation(s) by kind %v -> %d element(s); "+
		"text filled %d, reinforced %d, supporting %d, standalone %d, conflict %d",
		len(cycle.Observations), byKind, len(world.Elements),
		report.Text.FilledLabel, report.Text.Reinforced, report.Text.Supporting,
		report.Text.Standalone, report.Text.RejectedConflict)
	active := win.ID
	world.ActiveWindow = &active
	world.Windows = []directorapi.Window{win}
	world.Timestamp = time.Now()
	return world, cycle
}

// describeStage prints what the Stage actually produced, which is most of what this test is for.
func describeStage(t *testing.T, w directorapi.WorldState, cycle observation.Cycle) {
	t.Helper()
	bySource := map[observation.Source]int{}
	for _, o := range cycle.Observations {
		bySource[o.Source()]++
	}
	t.Logf("the Stage: %d observation(s) %v -> %d Prop(s)",
		len(cycle.Observations), bySource, len(w.Elements))
	named, pixels := 0, 0
	for _, el := range w.Elements {
		if el.Label != "" {
			named++
		}
		if el.Provenance.OnlyDescribesPixels() {
			pixels++
		}
	}
	t.Logf("  %d named, %d described only by pixels", named, pixels)
}

// THE TRACKMANIA PAUSE MENU, THROUGH THE REAL STAGE.
//
// A surface with no accessibility tree at all: every Prop on it is pixel-only, which is exactly
// the case the licence exists for. The entry asked for is one the menu really has.
func TestARecordedGameStageProducesPropsThatAreNotClickable(t *testing.T) {
	frame := loadFrame(t, filepath.Join(tmRecording, "frames",
		"tm-menu-steps-0100.png"))
	world, cycle := stageFrom(t, frame, true)
	describeStage(t, world, cycle)

	// WHAT THE STAGE SAW, in the words a person would recognise.
	for _, want := range []string{"settings", "scoreboard", "resume"} {
		if el := findLabelled(world, want); el != nil {
			t.Logf("  %-12s %v  bounds %v  sources %v", want, el.ID, el.Bounds,
				sourcesOf(el))
		} else {
			t.Logf("  %-12s NOT ON THE STAGE", want)
		}
	}

	// AND WHAT THE DIRECTOR MAKES OF A REQUEST FOR ONE.
	const asked = "settings"
	lic := directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"}
	q := directorapi.ElementQuery{Text: asked}

	unlicensed := target.NewResolver().Resolve(&world, q)
	licensed := target.NewResolver().ResolveUnder(&world, q, lic, "click")

	t.Logf("unlicensed: %s — %s", unlicensed.Status, unlicensed.Explanation)
	t.Logf("licensed:   %s — %s", licensed.Status, licensed.Explanation)
	for _, c := range licensed.Candidates {
		if c.Rejected != "" {
			t.Logf("   rejected %q %q: %s", c.ElementID, c.Label, c.Rejected)
		}
	}

	// ── WHAT THE STRUCTURAL PROVIDER ADDED, AND WHAT IT DID NOT ──
	//
	// Measured on this frame: 228 Props where there were none, and the SETTINGS and
	// SCOREBOARD entries each fuse a structural region with the OCR reading of their
	// own text into ONE Prop carrying both sources. RESUME stays nameless, because a
	// highlighted entry is dark text on white and OCR never read it — the Stage can say
	// a region is there and cannot say what it is called, which is the honest answer.
	//
	// And NOTHING BECAME CLICKABLE. Every structural Prop carries RoleUnknown, which
	// affords nothing, so `Actionability.Any()` is false and `Targetable()` is false
	// whether or not a licence is granted. The resolver refuses for a precise reason
	// and the licence is never consulted.
	if len(world.Elements) == 0 {
		t.Fatal("PERCEPTION GAP: the Stage carries no Props at all; the structural " +
			"provider contributed nothing")
	}
	named := 0
	for _, el := range world.Elements {
		if el.Label != "" {
			named++
		}
	}
	t.Logf("%d Prop(s), %d of them named", len(world.Elements), named)

	// THE SAFETY PROPERTY, asserted over every Prop on the Stage rather than over a
	// chosen one: not a single structural region is targetable, licensed or not, and
	// that includes the oversized background regions among them.
	var biggest *directorapi.Element
	for _, el := range world.Elements {
		if !el.Provenance.Has(directorapi.SourceStructure) {
			continue
		}
		if ok, _ := el.OperableUnder(lic, "click"); ok && el.Actions().Any() {
			t.Errorf("a structural Prop is operable: %q at %v, sources %v",
				el.Label, el.Bounds, sourcesOf(el))
		}
		if biggest == nil || el.Bounds.Area() > biggest.Bounds.Area() {
			biggest = el
		}
	}
	if biggest != nil {
		t.Logf("the largest structural Prop is %v (%.1f%% of the window), "+
			"targetable %v", biggest.Bounds,
			100*float64(biggest.Bounds.Area())/float64(frame.size.Width*frame.size.Height),
			biggest.Actions().Targetable())
		if biggest.Actions().Targetable() {
			t.Error("an oversized decorative region is a valid click surface")
		}
	}

	// AND THE REQUEST REFUSES, for a reason that names what is missing.
	if licensed.Status == directorapi.ResolutionResolved {
		t.Fatalf("a structural Prop resolved as a click target: %+v", licensed.Target)
	}
	if licensed.Status != unlicensed.Status {
		t.Errorf("the licence changed the outcome although nothing is operable: "+
			"%s vs %s", unlicensed.Status, licensed.Status)
	}
	t.Logf("GAP: Props exist and none is operable. %q", licensed.Explanation)
}

// findLabelled is the first Prop whose label contains `want`, case-insensitively.
func findLabelled(w directorapi.WorldState, want string) *directorapi.Element {
	for _, el := range w.Elements {
		if strings.Contains(strings.ToLower(el.Label), want) {
			return el
		}
	}
	return nil
}

func sourcesOf(el *directorapi.Element) []directorapi.ObservationSource {
	var out []directorapi.ObservationSource
	for _, s := range el.Provenance.Sources {
		out = append(out, s.Source)
	}
	return out
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func centreOf(r directorapi.Rect) string {
	return fmt.Sprintf("(%d,%d)", r.X+r.Width/2, r.Y+r.Height/2)
}

// A SECOND, UNRELATED INTERFACE, to show the provider is generic rather than fitted.
//
// A real Explorer window: a different toolkit, a different density, and no game anywhere near
// it. What is asserted is the same two things — that Props appear, and that none of them is
// clickable on structural evidence alone.
func TestAnUnrelatedRecordedInterfaceAlsoProducesPropsThatAreNotClickable(t *testing.T) {
	frame := loadFrame(t, `../../.tmp/desktop-corpus-review/explorer-fixture-wide/`+
		`explorer-fixture-wide.png`)
	world, cycle := stageFrom(t, frame, false)
	describeStage(t, world, cycle)

	if len(world.Elements) == 0 {
		t.Fatal("the structural provider contributed no Props to this interface")
	}
	named := 0
	for _, el := range world.Elements {
		if el.Label != "" {
			named++
		}
	}
	t.Logf("%d Prop(s), %d of them named", len(world.Elements), named)

	lic := directorapi.OperateLicence{Instruction: "click Details", Verb: "click"}
	for _, el := range world.Elements {
		if !el.Provenance.Has(directorapi.SourceStructure) {
			continue
		}
		if ok, _ := el.OperableUnder(lic, "click"); ok && el.Actions().Any() {
			t.Errorf("a structural Prop is operable here too: %q at %v",
				el.Label, el.Bounds)
		}
	}
	res := target.NewResolver().ResolveUnder(&world,
		directorapi.ElementQuery{Text: "details"}, lic, "click")
	t.Logf("click Details: %s — %s", res.Status, res.Explanation)
	if res.Status == directorapi.ResolutionResolved {
		t.Fatalf("a structural Prop resolved as a click target: %+v", res.Target)
	}
}
