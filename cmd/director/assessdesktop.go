package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHAT DID THE PRIMARY SENSOR ACTUALLY SHOW US?
//
// A developer looking at a perception problem needs three things in one line: whether the
// reading represents the interface, why, and what it cost. The third is beside the first two
// and deliberately not part of them — 37C measured Explorer's accessibility walk at 1.5
// seconds for the richest reading in the corpus, and a diagnostic that put cost and quality in
// the same number would report it as the worst.
//
// This reads a captured sample and says. It classifies through the production judgement —
// observe.PlaceNow over observe.ReachOfState — rather than a second copy of the rule, which is
// the whole reason it is worth having: pointed at a fresh capture it is a live check on the
// real classifier, and pointed at the committed corpus it prints what the tests assert.
//
// It has no memory. Recognition is a different question and needs a store; this answers only
// whether the reading was worth asking it about.
//
// # What this cannot tell you
//
// The conversion below is enough for the sufficiency judgement, which reads only how many
// structures there are and where. It is NOT enough for identity. A captured sample records
// elements; a StructureSignature is built from tracked evidence a session accumulates — roles
// aggregated over repeated inferences, interface terms read from labels, whether text was
// legible at all. Run through this adapter every sample signs as empty, and CompareStructure
// then answers "candidate" for any pair, including two obviously different Settings pages.
//
// So this command may not be used to claim two readings are the same Place. Checked, and it is
// the reason 35D's resize acceptance is still recorded as unmeasured rather than passed on the
// strength of a wide and a narrow capture both being sufficient. Sufficiency is half the claim;
// identity needs a live session with a store behind it.

// runAssessDesktopSample is `director assess-desktop-sample --dir <corpus-or-sample>`.
func runAssessDesktopSample(args []string) int {
	fs := flag.NewFlagSet("assess-desktop-sample", flag.ExitOnError)
	dir := fs.String("dir", "fixtures/perception/desktop/corpus",
		"a sample directory, or a directory of them")
	_ = fs.Parse(flagsFirst(args))

	samples, err := loadDesktopSamples(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if len(samples) == 0 {
		fmt.Fprintf(os.Stderr, "director: no samples in %s\n", *dir)
		return 1
	}

	fmt.Printf("%-26s %-13s %-9s %8s %9s %7s\n",
		"sample", "sufficiency", "elements", "collect", "fuse", "gap")
	worst := 0
	for _, s := range samples {
		p := observe.PlaceNow(desktopTotals(s), s.Application, recallsNothing{},
			observe.DefaultHypothesisThresholds())
		a := observe.SufficiencyOf(p)
		fmt.Printf("%-26s %-13s %-9d %6dms %7dms %5dms\n",
			s.ID, a.State, len(s.Elements), s.CollectMillis, s.FuseMillis, s.GapMillis)
		fmt.Printf("    %s\n", a.Describe())
		if a.State != observe.Sufficient {
			worst = 1
		}
	}
	// A non-zero exit when anything is not sufficient, so this is usable as a check and not
	// only as something to read.
	return worst
}

// recallsNothing recognises nothing.
//
// Recognition needs a store and is a different question: this reports whether the READING was
// worth asking memory about. Handing it a real store would let "I know this screen" quietly
// become part of the answer, which is the collapse ADR-103 exists to prevent.
type recallsNothing struct{}

func (recallsNothing) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{}
}

// desktopTotals presents a captured reading as one settled screen state.
//
// The same dull conversion the corpus tests use, and dull on purpose: every element becomes
// one present track in one state. Anything cleverer would be this command inventing evidence
// the capture did not record.
func desktopTotals(s desktopSample) observe.ShadowTotals {
	tracks := make([]observe.ShadowTrack, 0, len(s.Elements))
	for _, e := range s.Elements {
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

// loadDesktopSamples reads one sample directory or a directory of them.
func loadDesktopSamples(dir string) ([]desktopSample, error) {
	if s, err := readDesktopSample(dir); err == nil {
		return []desktopSample{s}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []desktopSample
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := readDesktopSample(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func readDesktopSample(dir string) (desktopSample, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "production.json"))
	if err != nil {
		return desktopSample{}, err
	}
	var s desktopSample
	if err := json.Unmarshal(raw, &s); err != nil {
		return desktopSample{}, fmt.Errorf("%s: %w", dir, err)
	}
	return s, nil
}
