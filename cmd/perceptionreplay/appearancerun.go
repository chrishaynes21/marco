package main

// Driving the appearance experiment: what it collects, and what it compares.

import (
	"path/filepath"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// maxAppearanceLooks bounds the crops taken per control.
//
// Each one re-reads a 1920x1080 PNG. Twenty looks is enough to see whether a margin keeps its
// sign and few enough that the experiment finishes; a descriptor that needs a hundred looks to
// show a separation has not shown one.
const maxAppearanceLooks = 20

// appearanceExperiment collects the looks geometry refused and then asks about them.
type appearanceExperiment struct {
	trials map[observe.VisualTargetID]*appearanceTrial
	order  []observe.VisualTargetID
}

func newAppearanceExperiment(targets []observe.StoredTarget) *appearanceExperiment {
	a := &appearanceExperiment{trials: map[observe.VisualTargetID]*appearanceTrial{}}
	for _, st := range targets {
		// ONLY CONTROLS THAT HAVE A CONTAINER. A control with nothing recorded around
		// it cannot be confused with one, so there is no ambiguity to examine and
		// including it would pad the result with cases that were never hard.
		if len(st.Readings) == 0 || len(st.Containers) == 0 {
			continue
		}
		a.trials[st.ID] = &appearanceTrial{
			id: st.ID, screen: st.Screen,
			reading: st.Readings[0], container: st.Containers[0],
		}
		a.order = append(a.order, st.ID)
	}
	return a
}

// collect records one look IF the geometric matcher refused this control for indistinctness.
//
// The gate is the matcher's own verdict, asked of the real candidate set, so appearance can only
// ever see looks where geometry had already given up. A case that geometry resolved is not
// offered here at all.
func (a *appearanceExperiment) collect(st observe.StoredTarget, look int, cands []observe.Candidate) {
	t := a.trials[st.ID]
	if t == nil || len(t.cases) >= maxAppearanceLooks {
		return
	}
	got := observe.ReacquireStored(st, observe.Regions(cands))
	if got.Result != observe.ReacquireUnobservable || got.Indistinct == 0 {
		return
	}
	// THE PROPOSAL THAT CAUSED IT. Re-found rather than carried out of the matcher,
	// because the matcher reports a count and not which rectangle — and a diagnosis that
	// guessed would be describing a different rectangle from the one that was refused.
	for _, c := range cands {
		if !sameControlRegions(st.Readings, c.Region) {
			continue
		}
		if !sameControlRegions(st.Containers, c.Region) {
			continue
		}
		t.cases = append(t.cases, ambiguousCase{look: look, cand: c.Region})
		return
	}
}

// run takes the reference crops from fold A, the test crops from fold B, and reports.
func (a *appearanceExperiment) run(dir string, frames []frameRecord,
	regions [][]observe.Region, foldA fold,
	labels map[observe.VisualScreenID]string, b workflowstore.Binding) error {

	// ── THE REFERENCES, FROM FOLD A ──
	//
	// Cropped from looks of the control's OWN screen in the fold that learned it, so
	// every comparison below is across the fold boundary. A reference taken from fold B
	// would be comparing a descriptor with itself.
	byLabel := map[string]observe.VisualScreenID{}
	for id, l := range labels {
		byLabel[l] = id
	}
	for _, id := range a.order {
		t := a.trials[id]
		if len(t.cases) == 0 {
			continue
		}
		// THE SAME CONTROL'S SCREEN IN FOLD A. The binding says which fold-B
		// screen a label became; this walks it back to the label and then to the
		// fold-A screen the label was allocated for, which is where the reference
		// crops have to come from.
		label := ""
		for l, sid := range b.Screens {
			if sid == t.screen {
				label = l
				break
			}
		}
		srcScreen := byLabel[label]
		t.refScreen = srcScreen

		// THE LOOKS, IN ORDER, EVENLY SPACED.
		//
		// `foldA.at` is a map and ranging it is randomised, so the first version took
		// twenty arbitrary looks and reported a stability of 0.000-1.000 — a number that
		// changed every run and could not be reproduced, let alone argued with. Sorted
		// and spread across the whole span the screen stood for, so the sample is the
		// same twice and covers the screen's life rather than a clump of it.
		var looks []int
		for look, screen := range foldA.at {
			if screen == srcScreen && srcScreen != "" {
				looks = append(looks, look)
			}
		}
		sort.Ints(looks)
		t.refLooks = len(looks)
		step := 1
		if len(looks) > maxAppearanceLooks {
			step = len(looks) / maxAppearanceLooks
		}
		for i := 0; i < len(looks) && len(t.refReading) < maxAppearanceLooks; i += step {
			look := looks[i]
			img, err := readPNG(filepath.Join(dir, filepath.FromSlash(frames[look].File)))
			if err != nil {
				continue
			}
			t.refReading = append(t.refReading, describeCrop(img, t.reading, look))
			t.refContainer = append(t.refContainer, describeCrop(img, t.container, look))
		}
	}

	// ── THE TESTS, FROM FOLD B ──
	for _, id := range a.order {
		t := a.trials[id]
		for i := range t.cases {
			c := &t.cases[i]
			img, err := readPNG(filepath.Join(dir,
				filepath.FromSlash(frames[c.look].File)))
			if err != nil {
				c.simReading, c.simContainer = -1, -1
				continue
			}
			d := describeCrop(img, c.cand, c.look)
			c.simReading = bestAgainst(d, t.refReading)
			c.simContainer = bestAgainst(d, t.refContainer)
		}
	}

	// ── THE NEGATIVE CASE: neighbouring controls on one screen ──
	//
	// Rows of a menu are the same button art with different words. Taken from fold A's
	// busiest screen, which on this recording is the main menu's list.
	var neighbours []neighbourPair
	var busiest observe.VisualScreenID
	count := map[observe.VisualScreenID]int{}
	for _, sc := range foldA.at {
		count[sc]++
	}
	for sc, n := range count {
		if sc != "" && n > count[busiest] {
			busiest, _ = sc, n
		}
	}
	for look, sc := range foldA.at {
		if sc != busiest || len(neighbours) > 0 {
			continue
		}
		img, err := readPNG(filepath.Join(dir, filepath.FromSlash(frames[look].File)))
		if err != nil {
			continue
		}
		rows := menuRows(regions[look])
		for i := range rows {
			for j := i + 1; j < len(rows); j++ {
				di := describeCrop(img, rows[i], look)
				dj := describeCrop(img, rows[j], look)
				if s := similarity(di, dj); s >= 0 {
					neighbours = append(neighbours,
						neighbourPair{a: rows[i], b: rows[j], sim: s})
				}
			}
		}
	}

	var list []*appearanceTrial
	for _, id := range a.order {
		list = append(list, a.trials[id])
	}
	printAppearance(list, neighbours)
	return nil
}

// bestAgainst is the strongest similarity to any reference crop.
//
// The strongest rather than the mean, deliberately generous: this experiment exists to find out
// whether appearance CAN separate these, so it is given the best case. A separation that fails
// under the kindest reading has failed.
func bestAgainst(d descriptor, refs []descriptor) float64 {
	best := -1.0
	for _, r := range refs {
		if s := similarity(d, r); s > best {
			best = s
		}
	}
	return best
}

// menuRows picks a handful of similarly-shaped, similarly-sized rectangles stacked vertically —
// the shape a menu list makes, and the controls most likely to be confused with one another.
func menuRows(all []observe.Region) []observe.Region {
	var rows []observe.Region
	for _, r := range all {
		if r.Width < 0.08 || r.Width > 0.35 || r.Height < 0.01 || r.Height > 0.08 {
			continue
		}
		dup := false
		for _, have := range rows {
			if have.Y-r.Y < 0.01 && r.Y-have.Y < 0.01 {
				dup = true
				break
			}
		}
		if !dup {
			rows = append(rows, r)
		}
		if len(rows) >= 8 {
			break
		}
	}
	return rows
}
