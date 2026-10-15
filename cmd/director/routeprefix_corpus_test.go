//go:build corpus

package main

// THE WHOLE DECISION PATH, OVER RECORDED PIXELS, EXECUTING NOTHING.
//
// [[Experiment-050-two-readings-of-one-button]] is here because the offline check before it
// exercised the label matcher alone and said both controls resolved. The live run then refused
// at step 0 for two reasons the matcher never sees:
//
//	re-acquisition refused, because the demonstration had learned PLAY as two identities
//	and `MaybeOneControl` correctly will not act on a pair;
//	and the re-grounder refused, because "play" also matched the rotating news panel.
//
// So this drives the production pieces the live loop drives, in the order it drives them:
//
//	fastobserve.Observer      the real fold, over regions the real proposer found
//	reacquireStored           the real re-acquisition, including its ambiguity refusal
//	promptGrounder            the real establishment path
//	regrounder                the real repair, including semantic grounding and surface
//	                          selection
//	workflowrun.Runner        the real coordinator, with a notebook where the mouse would be
//
// Behind `corpus` because it decodes several hundred PNGs. `go test ./...` does not run it;
// `go test -tags corpus ./cmd/director/` does, and a control-identity change is required to.

import (
	"bufio"
	"context"
	"encoding/json"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"

	_ "image/png"
)

const recording = `../../.tmp/game-capture-review/rl-freeplay-1`

// The learned artifact and the detections it was learned with, beside the recording.
//
// Fixtures rather than flags: this test exists to be RUN, and one that needs two environment
// variables set correctly is one that quietly never runs. Both are produced by
//
//	perceptionreplay -frames -workflow -stride 3 -onnx <detections> -save <artifact>
//
// and the test skips, saying so, when they are not there.
const (
	learnedArtifact = `../../.tmp/route/learned.workflow.json`
	learnedWith     = `../../.tmp/route/detections.jsonl`
)

type corpusFrame struct {
	File             string `json:"file"`
	AtMS             int64  `json:"at_ms"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	WindowGeneration uint64 `json:"window_generation"`
}

type corpusDetections struct {
	Width    int `json:"width"`
	Height   int `json:"height"`
	Elements []struct {
		Label      string  `json:"label"`
		Score      float64 `json:"score"`
		X, Y, W, H int
	} `json:"elements"`
}

type corpusWords struct {
	Look    int              `json:"look"`
	Phrases []string         `json:"phrases,omitempty"`
	Boxes   []observe.Region `json:"boxes,omitempty"`
}

func corpusJSONL[T any](t *testing.T, path string) []T {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("the recording is not here: %v", err)
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var v T
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	return out
}

// alwaysGranted stands where the per-action authorisation is. SIMULATED.
type alwaysGranted struct{ epoch, generation uint64 }

func (a *alwaysGranted) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	return workflowrun.Authorisation{
		Step: in.Step, By: "SIMULATED fixture",
		// THE OBSERVATION IT WAS ASKED ABOUT, which is what a granter stamps.
		Epoch: in.Epoch, Generation: in.Generation,
		Window:  in.Window,
		Expires: time.Now().Add(time.Minute),
	}, true
}

// routeRig is everything the live loop builds, over recorded pixels.
type routeRig struct {
	obs     *fastobserve.Observer
	run     *workflowrun.Runner
	session *workflowstore.SessionBinding
	ground  *promptGrounder
	art     workflowstore.Artifact
	frames  []corpusFrame
	words   map[int][]observe.ReadWord
	// detections is what the learned detector found on each frame.
	//
	// The live run of [[Experiment-050-two-readings-of-one-button]] had `--learned`
	// set, so the detector ran beside the proposer and its boxes were candidates for
	// re-acquisition like any other. A rig without them is not the path that ran: the
	// Training tile's saved rectangle IS a detector box, and the proposer's reading of
	// the same tile does not agree with it.
	detections map[int][]observe.Region
	out        *strings.Builder
	// lastWords is the reading of the look being folded, handed to the repair the way
	// the live loop hands it whatever OCR has answered.
	lastWords []observe.ReadWord
}

func newRouteRig(t *testing.T, artifact, from string, step int) *routeRig {
	t.Helper()
	body, err := os.ReadFile(artifact)
	if err != nil {
		t.Skipf("the artifact is not here: %v", err)
	}
	art, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatal(err)
	}
	// THE WALK FROM THIS STEP ON. A runner starts at its first step, and a recording
	// cannot be navigated into the middle of itself, so the artifact is trimmed to begin
	// where the case begins. Nothing else about it changes: the screens, the controls and
	// their evidence are the learned ones.
	if step > 0 && step < len(art.Steps) {
		art.Steps = append([]workflowstore.Step{}, art.Steps[step:]...)
	}
	frames := corpusJSONL[corpusFrame](t, filepath.Join(recording, "frames.jsonl"))
	rows := corpusJSONL[corpusWords](t, filepath.Join(recording, "words.jsonl"))
	words := map[int][]observe.ReadWord{}
	for _, r := range rows {
		if len(r.Boxes) != len(r.Phrases) {
			continue
		}
		var w []observe.ReadWord
		for i, p := range r.Phrases {
			w = append(w, observe.ReadWord{Text: p, Where: r.Boxes[i]})
		}
		words[r.Look] = w
	}

	dets := map[int][]observe.Region{}
	path := learnedWith
	if v := os.Getenv("MARCO_ROUTE_DETECTIONS"); v != "" {
		path = v
	}
	if _, err := os.Stat(path); err == nil {
		for i, d := range corpusJSONL[corpusDetections](t, path) {
			if d.Width == 0 {
				continue
			}
			rs := make([]image.Rectangle, 0, len(d.Elements))
			for _, e := range d.Elements {
				rs = append(rs, image.Rect(e.X, e.Y, e.X+e.W, e.Y+e.H))
			}
			dets[i] = observe.NormaliseRects(
				image.Rect(0, 0, d.Width, d.Height), rs)
		}
	}

	obs, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range obs.Updates() {
		}
	}()
	t.Cleanup(obs.Close)

	out := &strings.Builder{}
	session := workflowstore.NewSessionBinding()
	for i := range art.Steps {
		if err := session.ConfirmStep(i, "SIMULATED fixture"); err != nil {
			t.Fatal(err)
		}
	}
	ground := &promptGrounder{art: art, out: nil, standing: from,
		claim: from, silent: true}
	// The grounder prints; a Builder is not an *os.File, so it is given a real temp file
	// and the text is read back where a test needs it.
	f, ferr := os.CreateTemp(t.TempDir(), "ground*.txt")
	if ferr != nil {
		t.Fatal(ferr)
	}
	t.Cleanup(func() { f.Close() })
	ground.out = f

	book := recordhost.New()
	var marco directorapi.MarcoRunner = marcorunner.New(
		map[string]runtime.Host{"OS": book})

	run := workflowrun.New(art, nil, ground, &alwaysGranted{}).
		WithExecutor(marcoexec.New(marco), false).
		WithConfirm(func(context.Context, workflowrun.Sighting) (bool, string) {
			return true, ""
		})
	return &routeRig{obs: obs, run: run, session: session, ground: ground,
		art: art, frames: frames, words: words, detections: dets, out: out}
}

// look folds one recorded frame and advances the coordinator over it, exactly as the loop does.
func (r *routeRig) look(t *testing.T, index int, reg *regrounder) workflowrun.Attempt {
	t.Helper()
	fr := r.frames[index]
	img, err := readCorpusPNG(filepath.Join(recording, filepath.FromSlash(fr.File)))
	if err != nil {
		t.Fatal(err)
	}
	res := structure.Propose(img, structure.DefaultOptions())
	rects := make([]image.Rectangle, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		rects = append(rects, c.Bounds)
	}
	lk := fastobserve.Look{
		At: time.Unix(1700000000, 0).Add(time.Duration(index) * 100 * time.Millisecond),
		Window: fastobserve.WindowRef{ID: "hwnd:recorded",
			Generation: fr.WindowGeneration,
			Bounds:     directorapi.Rect{Width: fr.Width, Height: fr.Height}},
		Front:   true,
		Regions: observe.NormaliseRects(img.Bounds(), rects),
		Words:   r.words[index],
	}
	if d := r.detections[index]; len(d) > 0 {
		lk.Slow = observe.Proposed(d, index, fr.WindowGeneration, r.obs.Epoch())
	}
	if t := r.words[index]; len(t) > 0 {
		var phrases []string
		for _, w := range t {
			phrases = append(phrases, w.Text)
		}
		lk.Text = observe.TextOf(directorapi.SourceOCR, phrases)
	}

	outcomes := reacquireStored(r.art, r.session, r.obs, lk)
	if os.Getenv("MARCO_ROUTE_DEBUG") != "" {
		b := r.session.Binding(r.obs.Epoch(), fr.WindowGeneration)
		t.Logf("  look %3d  epoch %d  bound %v  outcomes %d",
			index, r.obs.Epoch(), b.Screens, len(outcomes))
		for _, o := range outcomes {
			t.Logf("      %s on %s: %s — %s", o.Target, o.Screen, o.Result,
				o.Because)
		}
	}
	up := r.obs.Observe(lk)
	r.ground.arrangement = observe.ArrangeLook(lk.Regions)
	r.ground.read = lk.Text

	at := workflowrun.Sighting{
		Screen: up.Screen, Epoch: up.Epoch, Generation: fr.WindowGeneration,
		Window: "hwnd:recorded",
		Bounds: directorapi.Rect{Width: fr.Width, Height: fr.Height},
		Front:  true, Outcomes: outcomes, At: lk.At,
		Candidates: lk.Regions,
		// A LOOK THAT IS NOT EVIDENCE SAYS SO. Without this a warming look
		// reads as a usable one, and the `--from` claim -- which is spent once,
		// by design -- goes on an observation that has no screen in it yet.
		Unobserved: up.Unobserved,
		Missing:    lk.Missing,
	}
	for _, w := range lk.Words {
		at.Words = append(at.Words, w)
	}
	if reg != nil {
		reg.candidates = lk.Regions
	}
	return r.run.Advance(context.Background(), r.session, at)
}

func readCorpusPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// newRegrounder is the production repair, reading the words this look carries.
func (r *routeRig) newRegrounder(t *testing.T) *regrounder {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "reground*.txt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	g := &regrounder{art: r.art, out: f}
	g.words = func(context.Context) ([]readWord, image.Rectangle, bool) {
		if len(g.candidates) == 0 {
			return nil, image.Rectangle{}, false
		}
		var out []readWord
		for _, w := range r.lastWords {
			out = append(out, readWord{Text: w.Text,
				X: int(w.Where.X * 1920), Y: int(w.Where.Y * 1080),
				W: int(w.Where.Width * 1920), H: int(w.Where.Height * 1080)})
		}
		if len(out) == 0 {
			return nil, image.Rectangle{}, false
		}
		return out, image.Rect(0, 0, 1920, 1080), true
	}
	return g
}

// THE TWO STEPS OF THE PREFIX, EACH DRIVEN TO ITS PRESS.
//
// What a recording CAN validate is everything up to the press: the screen established, the
// control re-acquired or re-grounded, the choice authorised, the program lowered and run. What
// it cannot is what the application does next — the frames are fixed, and pressing a recorded
// pixel moves nothing. So each step is driven over the recorded frames OF THE SCREEN IT STARTS
// ON, and the assertions are about where the press would land.
//
// Both live refusals were in this stretch, which is why it is the stretch that is checked.
func TestEachStepOfThePrefixReachesItsPressThroughTheProductionPath(t *testing.T) {
	artifact := learnedArtifact
	if v := os.Getenv("MARCO_ROUTE_ARTIFACT"); v != "" {
		artifact = v
	}
	for _, c := range []struct {
		name      string
		from      string
		step      int
		band      [2]int
		mustPress string
	}{{
		name: "main menu -> Play, pressing PLAY and never the news panel",
		from: "screen_a", step: 0, band: [2]int{0, 28}, mustPress: "control_1",
	}, {
		name: "Play -> Training, pressing the tile and not its caption",
		from: "screen_b", step: 1, band: [2]int{78, 112}, mustPress: "control_2",
	}} {
		t.Run(c.name, func(t *testing.T) {
			rig := newRouteRig(t, artifact, c.from, c.step)

			// ONE IDENTITY FOR THE CONTROL THIS STEP PRESSES. The defect was
			// two, and re-acquisition will not act on a pair.
			st := rig.art.Steps[0]
			if st.Uncertain {
				t.Fatalf("this step's control is an unresolved pair: %v",
					st.Unresolved)
			}
			reg := rig.newRegrounder(t)
			rig.run.WithRegrounder(reg)

			seen := map[string]bool{}
			var pressed *workflowrun.Intent
			for i := c.band[0]; i < c.band[1] && pressed == nil; i++ {
				rig.lastWords = rig.words[i]
				a := rig.look(t, i, reg)
				key := string(a.Outcome) + "|" + string(a.Stage)
				if !seen[key] {
					seen[key] = true
					t.Logf("look %3d  %-10s %-34s %s", i, a.Outcome,
						a.Stage, a.Because)
				}
				if a.Outcome == workflowrun.OutcomePressed && a.Intent != nil {
					pressed = a.Intent
				}
			}
			if pressed == nil {
				t.Fatalf("no press was ever reached over %d look(s) of %s",
					c.band[1]-c.band[0], c.from)
			}
			if pressed.Control != c.mustPress {
				t.Fatalf("pressed %s, want %s", pressed.Control, c.mustPress)
			}

			// AND IT LANDS WHERE THE DEMONSTRATION SAYS THE CONTROL IS.
			//
			// This is the news-panel assertion and the caption assertion at
			// once: the point has to be inside a rectangle this control was
			// actually demonstrated at, not inside any region that merely
			// carries the same word.
			at := observe.PointerAt{
				X: pressed.Where.X + pressed.Where.Width/2,
				Y: pressed.Where.Y + pressed.Where.Height/2,
			}
			var inside bool
			for _, r := range rig.art.Readings(c.mustPress) {
				if at.X >= r.X && at.X <= r.X+r.Width &&
					at.Y >= r.Y && at.Y <= r.Y+r.Height {
					inside = true
				}
			}
			if !inside {
				t.Errorf("the press would land at %.3f,%.3f, which is inside "+
					"none of the %d demonstrated reading(s) of %s. %s",
					at.X, at.Y, len(rig.art.Readings(c.mustPress)),
					c.mustPress, pressed.Because)
			}
			t.Logf("%s pressed at %.3f,%.3f of %+v — %s", pressed.Control,
				at.X, at.Y, pressed.Where, pressed.Because)
		})
	}
}
