package orchestrator_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Is the two-level screen model LOAD-BEARING, or only diagnostic?
//
// A learned play refuses to begin on the wrong screen and refuses to succeed on the wrong
// destination. Both questions go through one sentence — `do Screen's Showing with "…"` — which
// resolves a user's word to a durable subject and compares it with what is in front.
//
// Until the previous milestone, "in front" meant a whole application surface. Two places inside
// one application were one screen, so a play guarded on one of them would have started happily
// on the other and a destination check could not have told arrival from never having left.
//
// # What makes these different from the tests beside them
//
// `subj_a` and `subj_b` in destination_test.go are hand-written ids standing for two screens.
// These are not: the two subjects here are DERIVED, by a real observation session, from two
// states of ONE surface — the same enclosing shell, the same chrome, a replaced content region.
// If the durable derivation collapsed them, the store would hold one subject and every
// assertion below would fail on its premise rather than on its claim.

// ── two places inside one application, observed rather than declared ──────────

// placeSurface is an application shell with a smaller state-bearing region inside it.
//
// 0.714 whole-surface similarity between the two states, against a bar of 0.55: unambiguously
// the same surface, and not a fixture balanced on a threshold.
func placeSurface() screenfixture.Surface {
	return screenfixture.Surface{Chrome: 60, Content: 12, ContentRole: "list_item"}
}

// placeSampler plays a script of compositions with their read terms, as the live sampler does.
type placeSampler struct {
	frames []placeFrame
	calls  int
}

type placeFrame struct {
	regions []observe.ShadowRegion
	terms   []observe.InterfaceTerm
}

func (s *placeSampler) Sample(context.Context, observesession.SampleRequest) (observe.Sample, error) {
	f := s.frames[len(s.frames)-1]
	if s.calls < len(s.frames) {
		f = s.frames[s.calls]
	}
	s.calls++
	return observe.Sample{
		Structure: observe.StructuralView{
			Source: observe.StructureFused, Regions: f.regions,
		},
		Shadow: &observe.ShadowSample{
			Semantic: observe.SemanticEvidence{Observed: true, Terms: f.terms},
		},
	}, nil
}

// placeClock advances on demand, so the session runs to its bound without waiting.
type placeClock struct{ now time.Time }

func (c *placeClock) Now() time.Time { return c.now }

func (c *placeClock) After(d time.Duration) <-chan time.Time {
	c.now = c.now.Add(d)
	ch := make(chan time.Time, 1)
	ch <- c.now
	return ch
}

type steadyWindow struct{}

func (steadyWindow) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	return windowref.Ref{
		ID: "hwnd:1", Handle: 1, ProcessID: 3, Application: "testgame", Generation: 1,
	}, nil
}

func hold(r []observe.ShadowRegion, n int, terms ...observe.InterfaceTerm) []placeFrame {
	out := make([]placeFrame, 0, n)
	for range n {
		out = append(out, placeFrame{regions: r, terms: terms})
	}
	return out
}

// twoNamedPlaces observes one surface in two states and returns a store naming both.
//
// The whole point of the fixture: nothing here declares two screens. A session observes one
// application, the model decides there are two places in it, and the durable derivation is what
// turns that into two subjects a play can name.
func twoNamedPlaces(t *testing.T) (*semanticmemory.Store, string, string) {
	t.Helper()
	a := placeSurface()
	b := a.ContentReplaced("checkbox")

	var frames []placeFrame
	for range 2 {
		frames = append(frames, hold(a.Regions(), 5,
			observe.TermControls, observe.TermSettings)...)
		frames = append(frames, hold(b.Regions(), 5,
			observe.TermAudio, observe.TermDisplay)...)
	}

	bounds := observe.DefaultBounds()
	bounds.Duration = 10 * time.Second
	bounds.Interval = observe.MinInterval
	got, err := observesession.New(&placeClock{now: time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)}, steadyWindow{},
		&placeSampler{frames: frames}, observesession.NopEvents{}).
		Run(context.Background(), observesession.Config{
			Selector: windowref.Selector{Application: "testgame"}, Bounds: bounds,
		})
	if err != nil {
		t.Fatalf("observing: %v", err)
	}
	if n := len(got.Stats.Shadow.States); n != 2 {
		t.Fatalf("the session found %d place(s) in one surface; this test needs two", n)
	}
	surfaces := map[observe.SurfaceID]bool{}
	for _, st := range got.Stats.Shadow.States {
		surfaces[st.SurfaceOf()] = true
	}
	if len(surfaces) != 1 {
		t.Fatalf("the two places were assigned %d surfaces; this test is about two places "+
			"inside ONE", len(surfaces))
	}

	// One signature per state, by the SAME derivation a relationship endpoint uses.
	sigs := map[observe.ScreenStateID]observe.StructureSignature{}
	for _, h := range got.Hypotheses {
		if h.Subject.Kind != observe.SubjectState {
			continue
		}
		id := observe.ScreenStateID(h.Subject.Ref)
		if _, seen := sigs[id]; !seen {
			sigs[id] = observe.SignatureOf(h)
		}
	}

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	named := map[observe.InterfaceTerm]string{
		observe.TermSettings: "editor", observe.TermAudio: "settings",
	}
	var editor, settings string
	for _, sig := range sigs {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("remembering a place: %v", err)
		}
	}
	if n := len(store.Subjects()); n != 2 {
		t.Fatalf("two places inside one surface were stored as %d subject(s); the durable "+
			"derivation collapsed them and nothing below could be true", n)
	}
	for _, s := range store.Subjects() {
		for _, term := range s.Structure.Terms {
			name, ok := named[term]
			if !ok {
				continue
			}
			if err := store.NameSubject("testgame", s.ID, mustName(t, name)); err != nil {
				t.Fatalf("naming %s: %v", name, err)
			}
			if name == "editor" {
				editor = s.ID
			} else {
				settings = s.ID
			}
			break
		}
	}
	if editor == "" || settings == "" {
		t.Fatalf("the two places were not both named: editor=%q settings=%q", editor, settings)
	}
	if editor == settings {
		t.Fatal("both names resolved to one subject")
	}
	return store, editor, settings
}

func mustName(t *testing.T, s string) observe.ScreenName {
	t.Helper()
	n, err := observe.UserSuppliedScreenName(s)
	if err != nil {
		t.Fatalf("naming %q: %v", s, err)
	}
	return n
}

// ── the world the Screen host looks at ────────────────────────────────────────

// placeStage answers the Screen act from real durable memory.
//
// `SubjectNamed` is the store's own lookup. `CurrentSubject` returns whichever place the test
// says is in front — which is the one thing a fixture has to supply, because no real screen is
// present in a unit test.
type placeStage struct {
	store   *semanticmemory.Store
	current string
	after   string
	arrived *bool
	looks   *int
}

func (s *placeStage) Application() string { return "testgame" }

func (s *placeStage) SubjectNamed(application, name string) (string, bool) {
	r, ok := s.store.SubjectNamed(application, name)
	if !ok {
		return "", false
	}
	return r.ID, true
}

func (s *placeStage) CurrentSubject(string) (string, screenhost.Outcome) {
	*s.looks++
	if s.arrived != nil && *s.arrived && s.after != "" {
		return s.after, screenhost.Recognised
	}
	return s.current, screenhost.Recognised
}

// placePlay saves the production-generated play and returns everything needed to run it.
func placePlay(t *testing.T, sc *placeStage, startsOn, endsOn string) (
	orchestrator.Deps, *movingHost, *strings.Builder) {

	t.Helper()
	src, err := marcoexec.LowerPlayBetween("Shortcut", "Run", startsOn, endsOn,
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	reg := routes.Registry{Dir: t.TempDir()}
	if err := reg.SaveWithOrigin(routes.Route{App: "testgame", Slug: "shortcut"}, src,
		routes.Origin{
			Kind: routes.KindLearned, Application: "testgame",
			From: "from", To: "to", Sequence: 1, Evidence: "e1",
		}); err != nil {
		t.Fatalf("saving: %v", err)
	}

	host := &movingHost{arrived: sc.arrived, final: "confirm"}
	out := &strings.Builder{}
	d := orchestrator.Deps{
		Reg:   reg,
		Hosts: map[string]runtime.Host{"*": host, "Screen": screenhost.New(sc)},
		In:    strings.NewReader("y\n"),
		Out:   out,
		App:   func() string { return "testgame" },
	}
	d.Authority = orchestrator.AskFirst{Deps: d}
	return d, host, out
}

// ── M. the start guard distinguishes two places in one surface ────────────────

// Invoked from where it was learned, the play runs.
//
// The control. Without it the refusal below would be consistent with a guard that refuses
// everything, which is not a guarantee — it is a broken play.
func TestALearnedPlayRunsFromThePlaceItWasLearnedOn(t *testing.T) {
	store, editor, _ := twoNamedPlaces(t)
	arrived, looks := false, 0
	sc := &placeStage{store: store, current: editor, arrived: &arrived, looks: &looks}
	d, host, out := placePlay(t, sc, "editor", "editor")

	runSavedPlay(t, d, "shortcut")
	if len(host.pressed()) == 0 {
		t.Fatalf("a play invoked on the place it was learned on sent nothing: %q", out.String())
	}
}

// M. THE headline. Invoked from the OTHER place in the SAME surface, it refuses, and sends nothing.
//
// Same application, same window, same chrome, same enclosing surface. The only thing that
// differs is the place inside it. Before the two-level model this was indistinguishable from the
// test above, and a play learned in an editor would have fired inside a settings panel.
func TestALearnedPlayRefusesFromAnotherPlaceInTheSameSurface(t *testing.T) {
	store, editor, settings := twoNamedPlaces(t)
	if editor == settings {
		t.Fatal("the fixture holds one place, so this proves nothing")
	}
	arrived, looks := false, 0
	sc := &placeStage{store: store, current: settings, arrived: &arrived, looks: &looks}
	d, host, out := placePlay(t, sc, "editor", "editor")

	runSavedPlay(t, d, "shortcut")
	if n := len(host.pressed()); n != 0 {
		t.Fatalf("a play guarded on \"editor\" sent %d input(s) while \"settings\" was in "+
			"front of it — the same surface was enough to satisfy the guard, which is the "+
			"regression this milestone exists to prevent: %v", n, host.pressed())
	}
	if looks == 0 {
		t.Error("the guard never looked at the world")
	}
	if strings.Contains(out.String(), "done") {
		t.Errorf("the play reported success without running: %q", out.String())
	}
}

// ── N. the destination check distinguishes them too ───────────────────────────

// Arriving at the other place in the same surface is ARRIVING.
func TestAPlayArrivingAtAnotherPlaceInTheSameSurfaceSucceeds(t *testing.T) {
	store, editor, settings := twoNamedPlaces(t)
	arrived, looks := false, 0
	sc := &placeStage{
		store: store, current: editor, after: settings, arrived: &arrived, looks: &looks,
	}
	d, _, out := placePlay(t, sc, "editor", "settings")

	runSavedPlay(t, d, "shortcut")
	if !strings.Contains(out.String(), "done") {
		t.Fatalf("a play that reached its destination reported %q", out.String())
	}
	if looks < 2 {
		t.Errorf("the world was looked at %d time(s); the destination must be observed "+
			"fresh, after the effects", looks)
	}
}

// N. THE headline. Never leaving the place it started on is NOT arriving.
//
// The effects all happened. The application never moved, the enclosing surface matched
// throughout, and the play must still fail — because "the same application is in front" is not
// the claim the play made.
func TestAPlayThatNeverLeftItsPlaceFails(t *testing.T) {
	store, editor, settings := twoNamedPlaces(t)
	arrived, looks := false, 0
	// after == current: the route ran and nothing moved.
	sc := &placeStage{
		store: store, current: editor, after: editor, arrived: &arrived, looks: &looks,
	}
	d, host, out := placePlay(t, sc, "editor", "settings")

	runSavedPlay(t, d, "shortcut")
	// The effects DID happen. This is a wrong-destination case, not a wrong-start one.
	if len(host.pressed()) == 0 {
		t.Fatal("nothing was sent, so this is testing the start guard rather than the " +
			"destination check")
	}
	if strings.Contains(out.String(), "done") {
		t.Fatalf("a play that never left \"editor\" reported arriving at \"settings\"; the "+
			"enclosing surface matched and that was enough: %q", out.String())
	}
	_ = settings
}
