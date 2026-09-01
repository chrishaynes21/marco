package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// One perception substrate, and everything stands on it.
//
// # What these hold, and why they read source
//
// The claim is architectural: Observe, Learn, Perform and 36F recovery all consume the same fused
// semantic reading, and no production consumer has a private interpretation of the sensors. That
// is a claim about WIRING — about which functions exist and what they call — and the honest way to
// hold it is to check the wiring.
//
// They are weaker than a behavioural test and they say so. What they can see is that there is one
// door and that nothing else opens one; what they cannot see is what the door computes. The
// behavioural half lives in `internal/director/perception/fusion`, which drives the engine
// directly over hand-built cycles.
//
// The precedent is `TestEveryLiveWalkerChecksTheForeground`, which reads source for the same
// reason and is honest about the same limit.
//
// See [[ADR-100-marco-sees-through-evidence]].

// directorSource is every Go file in the Director command, read once.
func directorSource(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		out[name] = string(b)
	}
	if len(out) == 0 {
		t.Fatal("no production source was read, so this proves nothing")
	}
	return out
}

// THERE IS ONE DOOR FROM SENSORS TO BELIEF.
//
// # The asymmetry the whole perception stack rests on
//
// Providers emit observations and cannot create an Element. Fusion is the only thing that turns an
// observation into a belief. Everything above — Place recognition, Target resolution, the planner,
// policy, verification — sees Elements and cannot ask a sensor anything.
//
// So every production path from the desktop to a semantic reading goes through `Engine.Fuse`, and
// the number of places that call it is the number of places that could ever disagree about what is
// on screen. Four, and each is named here with what it serves:
//
//	observewiring.go  the observation SESSION sampler — Observe, Learn, Perform's fresh look,
//	                  and 36F's post-failure look, all of which run sessions
//	runtime.go        the foreground pipeline — waits, commands, diagnostics
//	main.go           a one-shot reading for the inspect commands
//	ocrwiring.go      the text-reading path
//	capturedesktop.go the 37C corpus capture — measurement, not behaviour
//	walkaudit.go      the 37E acquisition audit — measurement, not behaviour
//
// A fifth would not be wrong on its own. It would be a fifth thing to keep in step, and this test
// exists so that adding one is a decision somebody makes rather than a line somebody adds.
//
// # The fifth, decided
//
// capturedesktop.go is a fifth, and it was admitted on purpose. It exists to record what
// production perception BELIEVED at a moment, beside the screenshot of that moment, so a
// detector can be measured against it (37C). A corpus built from a second, private reading of
// the sensors would be a corpus about that second reading — which is exactly the mistake this
// test guards against, arriving from the other direction.
//
// It is allowed because it drives nothing: it is a command a person runs, it writes to a
// scratch directory, and no belief it produces reaches a Stage, a plan or an input. If that
// ever stops being true it belongs behind observewiring.go like everything else.
//
// Adding a Fuse call without naming it here must fail this.
func TestFusionIsTheOnlyDoorFromSensorsToBelief(t *testing.T) {
	known := map[string]bool{
		"observewiring.go": true, // the observation session sampler
		"runtime.go":       true, // the foreground pipeline
		"main.go":          true, // the one-shot inspect reading
		"ocrwiring.go":     true, // the text-reading path
		// Measurement, not behaviour. See the note above; neither drives anything.
		"capturedesktop.go": true,
		// Counts and times the walks one window costs, through the production
		// collector and engine on purpose — an audit of a second reading would be an
		// audit of that second reading. Reads; writes nothing; performs nothing.
		"walkaudit.go": true,
		// Asks what the screen in front says it is called, and why. It fuses for the
		// same reason walk-audit does: the naming rule reads the FUSED world, where
		// selection and parentage survive, so a probe over anything else would be
		// explaining a different world from the one production names. It prints; it
		// establishes nothing and performs nothing.
		"nameprobe.go": true,
		// Measures which parts of a reading hold still and which churn, over repeated
		// readings of one window. It fuses for the same reason name-probe does — the
		// dimensions it measures are properties of the FUSED world, and measuring them
		// over anything else would characterise a world production does not use.
		//
		// It is an instrument for deciding what semantic state IS, before anything about
		// identity changes. It prints digests, writes nothing, establishes nothing and
		// performs nothing.
		"evidenceprobe.go": true,
	}
	fuse := regexp.MustCompile(`\.Fuse\(`)
	for name, src := range directorSource(t) {
		if !fuse.MatchString(src) {
			continue
		}
		if !known[name] {
			t.Errorf("%s fuses a cycle and is not one of the four places that do. A "+
				"fifth reading of the sensors is a fifth thing that can disagree "+
				"about what is on screen — decide it deliberately and name it here.",
				name)
		}
	}
	// AND ALL FOUR ARE STILL THERE. A door that quietly stopped being used is the other
	// half: a consumer that grew its own sensor reading would leave one of these unused.
	for name := range known {
		src, ok := directorSource(t)[name]
		if !ok {
			t.Errorf("%s is gone; the wiring this test describes has moved", name)
			continue
		}
		if !fuse.MatchString(src) {
			t.Errorf("%s no longer fuses. Whatever it reads the screen with now is a "+
				"second interpretation of the sensors.", name)
		}
	}
}

// AND THE OBSERVATION SESSION IS THE ONE READING OBSERVE, LEARN, PERFORM AND RECOVERY SHARE.
//
// # Four consumers, one sampler
//
// They ask differently — ambient watching runs a long unlicensed session, Learn runs a licensed
// one, execution takes a short look, recovery takes another after a failure — and every one of
// them is an observation session, which means every one of them goes through `liveSampler.Sample`
// and therefore through fusion.
//
// What this can see is that no consumer builds an `observe.Sample` of its own. That is the exact
// shape the defect would take: a caller that reads the accessibility tree directly and constructs
// a sample from it has a private interpretation of the sensors, and everything above it would then
// be planning over two different worlds.
//
// Deleting the session sampler's fusion call, or building a sample anywhere else, must fail this.
func TestOnlyTheSessionSamplerBuildsAReading(t *testing.T) {
	builds := regexp.MustCompile(`observe\.Sample\{[^}]`)
	allowed := map[string]bool{
		// The one converter, fed by the fused world.
		"observesnapshot.go": true,
		// Its caller's error returns, which are empty samples and not readings.
		"observewiring.go": true,
	}
	for name, src := range directorSource(t) {
		if !builds.MatchString(src) {
			continue
		}
		if !allowed[name] {
			t.Errorf("%s builds an observe.Sample. A reading assembled anywhere but "+
				"from the fused world is a private interpretation of the sensors, "+
				"and everything above it would plan over a different screen.", name)
		}
	}
	// AND THE CONVERTER IS FED BY THE FUSED WORLD. `buildSample` takes the world as its
	// first argument, so a version that read a provider directly would not compile against
	// this signature — which is what makes the one-door claim structural rather than
	// conventional.
	src := directorSource(t)["observesnapshot.go"]
	if !strings.Contains(src, "func buildSample(world directorapi.WorldState") {
		t.Error("buildSample no longer takes the fused world as its input")
	}
	if !strings.Contains(directorSource(t)["observewiring.go"], "buildSample(world, cycle") {
		t.Error("the session sampler no longer converts the world it fused")
	}
}

// AND SENSOR EVIDENCE DOES NOT REACH THE SEMANTIC LAYERS AT ALL.
//
// The other half of the asymmetry, from above rather than below: the packages that decide what a
// Place IS, what a Goal means and how to plan must not be able to ask a sensor anything. If they
// could, "the screen" would have as many definitions as there are callers.
//
// Checked by imports, which is what makes it structural: `internal/director/observe` cannot import
// a provider, so no amount of editing inside it can grow a second reading.
func TestTheSemanticLayersCannotReachASensor(t *testing.T) {
	for _, pkg := range []string{
		"../../internal/director/observe",
		"../../internal/director/semanticmemory",
		"../../internal/director/ambient",
	} {
		entries, err := os.ReadDir(pkg)
		if err != nil {
			t.Fatalf("reading %s: %v", pkg, err)
		}
		read := 0
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") ||
				strings.HasSuffix(name, "_test.go") {
				continue
			}
			b, err := os.ReadFile(pkg + "/" + name)
			if err != nil {
				t.Fatalf("reading %s/%s: %v", pkg, name, err)
			}
			read++
			for _, banned := range []string{
				"perception/providers", "perception/fusion", "perception/capture",
				"platform/screen", "platform/winctx",
			} {
				if strings.Contains(string(b), banned) {
					t.Errorf("%s/%s imports %s. The layers that decide what a Place "+
						"IS must not be able to ask a sensor anything — otherwise "+
						"\"the screen\" has as many definitions as it has callers.",
						pkg, name, banned)
				}
			}
		}
		if read == 0 {
			t.Errorf("no source was read from %s", pkg)
		}
	}
}

// EVIDENCE FROM ANOTHER WINDOW NEVER REACHES THE READING.
//
// # The outer guard, and why it is not decoration
//
// A session describes ONE window. The fused world may hold elements from others — the foreground
// pipeline fuses whatever a cycle collected, and a cycle can span more than one — so the narrowing
// into a reading is where the boundary is drawn.
//
// Folding in a neighbour's controls would put another application's buttons into this Place's
// structure, and the durable signature built from it would describe a screen that never existed.
// It is the same failure as fusing a screenshot of one window with an accessibility tree of
// another, one layer along.
//
// Deleting the window comparison in buildSample must fail this.
func TestEvidenceFromAnotherWindowNeverReachesTheReading(t *testing.T) {
	frame := directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600}
	window := directorapi.Window{ID: "hwnd:100", Application: "settings", Bounds: frame}
	req := observesession.SampleRequest{
		Sequence: 1,
		Window: windowref.Ref{
			ID: "hwnd:100", Application: "settings", Generation: 1, Bounds: frame,
		},
	}
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"mine": {
			ID: "mine", Role: directorapi.RoleButton, Label: "Bluetooth & devices",
			WindowID: "hwnd:100", Visible: true, Enabled: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 10, Y: 20, Width: 200, Height: 40},
		},
		"theirs": {
			ID: "theirs", Role: directorapi.RoleButton, Label: "Send",
			WindowID: "hwnd:999", Visible: true, Enabled: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 10, Y: 80, Width: 200, Height: 40},
		},
	}}

	sample := buildSample(world, observation.Cycle{}, window, req)
	for _, e := range sample.Entities {
		if e.Label.Text == "Send" {
			t.Errorf("a control from another window reached this reading: %+v. A session "+
				"describes ONE window, and a Place built from a neighbour's controls "+
				"describes a screen that never existed.", e)
		}
	}
	if len(sample.Entities) != 1 {
		t.Errorf("%d entit(y/ies) in the reading, want the one that belongs to this window: "+
			"%+v", len(sample.Entities), sample.Entities)
	}
}

// A READING OF ONE SCENE IS THE SAME READING EVERY TIME.
//
// # Why order is part of the reading and not presentation
//
// `buildSample` sorts the fused world's element ids before converting them, and its own comment
// says why: "so two identical scenes produce identical samples and a fixture replays byte-for-byte".
// Nothing held it. A world is a MAP, so without the sort the entity order is whatever Go's
// iteration happened to give — and a recorded fixture would replay differently from the production
// reading it was captured from, which is the whole basis of the replay harness.
//
// It is also the shape a subtler defect would take: any downstream reader that takes "the first"
// of anything would be taking a different one each time.
//
// Deleting the id sort must fail this.
func TestAReadingOfOneSceneIsTheSameReadingEveryTime(t *testing.T) {
	frame := directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600}
	window := directorapi.Window{ID: "hwnd:100", Application: "settings", Bounds: frame}
	req := observesession.SampleRequest{
		Sequence: 1,
		Window: windowref.Ref{
			ID: "hwnd:100", Application: "settings", Generation: 1, Bounds: frame,
		},
	}
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	for i := 0; i < 64; i++ {
		id := directorapi.ElementID(fmt.Sprintf("e%03d", i))
		world.Elements[id] = &directorapi.Element{
			ID: id, Role: directorapi.RoleButton, Label: fmt.Sprintf("Item %d", i),
			WindowID: "hwnd:100", Visible: true, Enabled: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 10, Y: 10 * i, Width: 100, Height: 8},
		}
	}

	first := buildSample(world, observation.Cycle{}, window, req)
	if len(first.Entities) != 64 {
		t.Fatalf("%d entit(y/ies) in the reading, want 64", len(first.Entities))
	}
	for run := 0; run < 20; run++ {
		again := buildSample(world, observation.Cycle{}, window, req)
		if len(again.Entities) != len(first.Entities) {
			t.Fatalf("run %d read %d entities where the first read %d",
				run, len(again.Entities), len(first.Entities))
		}
		for i := range again.Entities {
			if again.Entities[i].Identity != first.Entities[i].Identity {
				t.Fatalf("run %d put %q at position %d where the first put %q. One "+
					"scene must read the same way every time, or a recorded "+
					"fixture replays differently from what it captured.",
					run, again.Entities[i].Identity, i, first.Entities[i].Identity)
			}
		}
	}
}
