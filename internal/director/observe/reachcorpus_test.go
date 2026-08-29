package observe_test

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE REAL DESKTOP, RUN THROUGH THE REAL CLASSIFIER.
//
// # Why this is worth more than another synthetic fixture
//
// Everything else in this package's tests is reconstructed: the shell fixture is the live
// Settings failure written down from a run nobody can repeat, and the dialog, the palette and
// the toolbar-heavy window were composed to sit on either side of a boundary. That is the
// right way to pin a rule, and it has one weakness — the fixtures were written by the same
// hand as the rule, so a rule that is subtly wrong about real applications can still pass all
// of them.
//
// 37C captured six coherent desktop moments from real applications and committed them:
// Settings at two widths, a second Settings Place, Explorer over a synthetic directory, and a
// browser fixture wide and narrow. Every one of those readings is a HEALTHY reading — the
// windows were on screen, in front, and doing what they normally do.
//
// So every one must classify as content reached. Not because a fixture says so, but because
// that is what those windows were.
//
// This kills, on real evidence rather than construction:
//
//	element count      54 through 140 elements, all sufficient
//	responsive reflow  the same Settings Place at 84 and 54 elements, both sufficient
//	application name   four applications, one rule
//	acquisition cost   Explorer took 1.5s to read and is not degraded for it
//
// See docs/experiments/Experiment-016-desktop-perception-corpus.md.

// corpusSample is the part of a committed 37C sample this test needs.
type corpusSample struct {
	ID       string `json:"id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Elements []struct {
		ID     string         `json:"id"`
		Role   string         `json:"role"`
		Bounds observe.Region `json:"bounds"`
	} `json:"elements"`
}

// asTotals presents a captured reading as one settled screen state.
//
// A corpus sample is a SINGLE reading, and ShadowTotals is normally accumulated over many. The
// conversion is deliberately dull — every element becomes one track, present, seen in one
// state — because anything cleverer would be this test inventing evidence. What survives the
// conversion is exactly what ReachOfState looks at: how many structures there are, and where.
func (c corpusSample) asTotals() observe.ShadowTotals {
	tracks := make([]observe.ShadowTrack, 0, len(c.Elements))
	for _, e := range c.Elements {
		tracks = append(tracks, observe.ShadowTrack{
			ID: e.ID, Role: e.Role, Present: true, Reference: e.Bounds,
			Seen: 3, Eligible: 3,
			States: []observe.TrackState{{State: "state_1", Seen: 3, Eligible: 3}},
		})
	}
	return observe.ShadowTotals{
		CurrentState: "state_1", Tracks: tracks,
		States: []observe.ScreenState{{ID: "state_1", Inferences: 3, Settled: true}},
	}
}

func readCorpusSamples(t *testing.T) []corpusSample {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "fixtures", "perception", "desktop", "corpus")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the 37C corpus is not readable at %s: %v", dir, err)
	}
	var out []corpusSample
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name(), "production.json"))
		if err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		var s corpusSample
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if len(out) < 6 {
		t.Fatalf("read %d corpus samples, want the six 37C captured. A test that reads "+
			"nothing passes for the wrong reason.", len(out))
	}
	return out
}

// Every healthy desktop reading 37C captured is sufficient.
func TestTheCapturedDesktopIsSufficient(t *testing.T) {
	for _, s := range readCorpusSamples(t) {
		t.Run(s.ID, func(t *testing.T) {
			got, v, _ := observe.ReachOfState(s.asTotals(), "state_1")
			if got != observe.ReachContent {
				t.Errorf("reach = %q, want %q\n"+
					"%s is a real reading of a real window that was on screen and "+
					"working. Calling it shell-only means the rule now fires on "+
					"healthy desktop applications.\n(elements %d, vacancy %+v)",
					got, observe.ReachContent, s.ID, len(s.Elements), v)
			}
		})
	}
}

// The same Settings Place at two widths is sufficient at both.
//
// 37C measured 84 elements wide and 54 narrow — a 36% drop from responsive layout alone. A
// classifier that reads a drop like that as collapse would refuse the narrow window of every
// application that reflows, which is most of them.
//
// This is the mutation gate for "element count decides", stated on real evidence: the two
// readings differ by thirty elements and must classify identically.
func TestResponsiveReflowIsNotCollapse(t *testing.T) {
	var wide, narrow *corpusSample
	for i, s := range readCorpusSamples(t) {
		switch s.ID {
		case "settings-mouse-wide":
			wide = &readCorpusSamples(t)[i]
		case "settings-mouse-narrow":
			narrow = &readCorpusSamples(t)[i]
		}
	}
	if wide == nil || narrow == nil {
		t.Fatal("the Settings reflow pair is missing from the corpus")
	}
	if len(wide.Elements) <= len(narrow.Elements) {
		t.Fatalf("the pair does not show a reflow drop (%d wide, %d narrow); this test "+
			"proves nothing unless the counts genuinely differ",
			len(wide.Elements), len(narrow.Elements))
	}

	w, wv, _ := observe.ReachOfState(wide.asTotals(), "state_1")
	n, nv, _ := observe.ReachOfState(narrow.asTotals(), "state_1")
	if w != n {
		t.Errorf("the same Settings Place classified %q at %d elements and %q at %d.\n"+
			"Responsive layout moved and hid controls; it did not stop the page being "+
			"read. A rule that separates these is counting.\nwide %+v\nnarrow %+v",
			w, len(wide.Elements), n, len(narrow.Elements), wv, nv)
	}
	if w != observe.ReachContent {
		t.Errorf("both classified %q, want %q", w, observe.ReachContent)
	}
}

// Classification does not depend on the order elements arrive in.
//
// A world is a map, and the tracks behind a reading come out of one. If the answer moved with
// iteration order the same screen would be sufficient and incomplete on alternate readings,
// and the escalation signal 37E is meant to consume would rattle.
func TestClassificationIsIndependentOfElementOrder(t *testing.T) {
	for _, s := range readCorpusSamples(t) {
		base := s.asTotals()
		want, wantV, wantR := observe.ReachOfState(base, "state_1")
		rng := rand.New(rand.NewSource(1))
		for i := 0; i < 8; i++ {
			shuffled := observe.ShadowTotals{
				CurrentState: base.CurrentState, States: base.States,
				Tracks: append([]observe.ShadowTrack(nil), base.Tracks...),
			}
			rng.Shuffle(len(shuffled.Tracks), func(a, b int) {
				shuffled.Tracks[a], shuffled.Tracks[b] =
					shuffled.Tracks[b], shuffled.Tracks[a]
			})
			got, gotV, gotR := observe.ReachOfState(shuffled, "state_1")
			if got != want || gotV != wantV || gotR != wantR {
				t.Fatalf("%s: shuffling the elements changed the answer from %q to %q\n"+
					"before %+v\nafter  %+v", s.ID, want, got, wantV, gotV)
			}
		}
	}
}
