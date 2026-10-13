package main

// Why a saved control was not found again.
//
// # The three answers, which need different work
//
// A control that matches on none of its looks has failed in one of three ways, and the owner was
// precise about why the distinction matters:
//
//	If the source evidence isn't available during reacquisition, the failure is an integration
//	or evidence-coverage issue. If the source evidence is present but the wrong control looks
//	equivalent, that's an identity issue.
//
//   - MISSING EVIDENCE. Nothing on any look is near the saved rectangle at all. The control was
//     never proposed in the second session, so there was nothing to match. An integration or
//     coverage problem, and no identity mechanism would help.
//   - INCOMPATIBLE GEOMETRY. Something is there, and it is not the same rectangle by the
//     relation this subsystem uses. The control is proposed at a different extent.
//   - COMPETING CANDIDATES. Something agrees with the saved reading AND with a region that
//     merely contained the original press, so it cannot tell the control from its container.
//     This is the identity problem, and it is the only one of the three that a richer
//     descriptor would address.
//
// Nothing here changes a threshold. The point is to say which of the three it is.

import (
	"fmt"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// controlDiagnosis is one saved control's account of itself over a fold.
type controlDiagnosis struct {
	target observe.StoredTarget

	looks int
	// candidates seen on those looks, by the cue that proposed them.
	byPixels, byDetector int
	// slowLooks is looks on which at least one detector candidate was available.
	slowLooks int

	// agreeing is looks where something was the same rectangle as a saved reading,
	// indistinct where that something was also the same as a container.
	agreeing, indistinct int

	// bestIoU is the closest any candidate ever came to a saved reading, and bestAt
	// the rectangle that did it.
	//
	// `iou` is diagnose.go's, which exists for exactly this and says so: a fraction for
	// reporting, never a decision. The matcher's relation is threshold-free and nothing
	// here changes it.
	bestIoU  float64
	bestAt   observe.Region
	bestFrom directorapi.ObservationSource

	// nearContainer is the closest any candidate came to a saved CONTAINER. A control
	// that is absent while its panel is plainly on screen is a different story from one
	// whose screen is not drawn at all.
	nearContainer float64
}

// verdict names which of the three failures this is.
func (d controlDiagnosis) verdict() string {
	switch {
	case d.agreeing > 0 && d.indistinct > 0:
		return "COMPETING CANDIDATES — identity"
	case d.agreeing > 0:
		return "found on some looks"
	case d.bestIoU >= 0.3:
		return "INCOMPATIBLE GEOMETRY — proposed at a different extent"
	case d.nearContainer >= 0.3:
		return "MISSING EVIDENCE — the container is here and the control is not"
	default:
		return "MISSING EVIDENCE — nothing near it was proposed at all"
	}
}

// observeControl folds one look's candidates into a diagnosis.
func (d *controlDiagnosis) observeControl(cands []observe.Candidate) {
	d.looks++
	slow := false
	agreed, indistinct := false, false
	for _, c := range cands {
		switch c.From {
		case directorapi.SourceVision:
			d.byDetector++
			slow = true
		default:
			d.byPixels++
		}
		for _, r := range d.target.Readings {
			if v := iou(r, c.Region); v > d.bestIoU {
				d.bestIoU, d.bestAt, d.bestFrom = v, c.Region, c.From
			}
		}
		for _, r := range d.target.Containers {
			if v := iou(r, c.Region); v > d.nearContainer {
				d.nearContainer = v
			}
		}
		if !sameControlRegions(d.target.Readings, c.Region) {
			continue
		}
		agreed = true
		if sameControlRegions(d.target.Containers, c.Region) {
			indistinct = true
		}
	}
	if slow {
		d.slowLooks++
	}
	if agreed {
		d.agreeing++
	}
	if indistinct {
		d.indistinct++
	}
}

// sameControlRegions asks the matcher's own question of one rectangle against a set.
//
// It goes through `observe.ReacquireStored` rather than reimplementing `sameRect`, so this
// diagnosis cannot disagree with the thing it is diagnosing. A single-reading target with no
// containers matches exactly when the rectangle agrees.
func sameControlRegions(set []observe.Region, r observe.Region) bool {
	if len(set) == 0 {
		return false
	}
	got := observe.ReacquireStored(observe.StoredTarget{
		ID: "probe", Screen: "probe", Readings: set,
	}, []observe.Region{r})
	return got.Result == observe.ReacquireMatched || got.Result == observe.ReacquireAmbiguous
}

// printControlDiagnoses reports each saved control's account of itself.
func printControlDiagnoses(diag map[observe.VisualTargetID]*controlDiagnosis) {
	fmt.Printf("\n  %s\n", repeat("-", 74))
	fmt.Printf("  WHY EACH SAVED CONTROL WAS OR WAS NOT FOUND AGAIN\n")
	fmt.Printf("  %s\n", repeat("-", 74))
	fmt.Printf("  `best` is the closest any candidate came to a saved reading, by overlap,\n")
	fmt.Printf("  and is REPORTING ONLY — the matcher's relation is unchanged. `container`\n")
	fmt.Printf("  is the closest anything came to a region that merely contained the\n")
	fmt.Printf("  original press: a control absent while its panel is plainly on screen is\n")
	fmt.Printf("  a different story from a screen that is not drawn.\n\n")

	ids := make([]string, 0, len(diag))
	for id := range diag {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	for _, id := range ids {
		d := diag[observe.VisualTargetID(id)]
		fmt.Printf("  %s on %s\n", id, d.target.Screen)
		fmt.Printf("    saved: %d reading(s), %d container(s)\n",
			len(d.target.Readings), len(d.target.Containers))
		for _, r := range d.target.Readings {
			fmt.Printf("      reading   %.4f,%.4f  %.4fx%.4f\n",
				r.X, r.Y, r.Width, r.Height)
		}
		if d.looks == 0 {
			fmt.Printf("    its screen never stood in this fold\n\n")
			continue
		}
		fmt.Printf("    fold B: %d look(s) of its screen; candidates %d from pixels, "+
			"%d from the detector\n", d.looks, d.byPixels, d.byDetector)
		fmt.Printf("      detector evidence available on %d of %d look(s)\n",
			d.slowLooks, d.looks)
		fmt.Printf("      agreeing with a saved reading: %d look(s); of those, "+
			"indistinct from a container: %d\n", d.agreeing, d.indistinct)
		fmt.Printf("      best overlap %.3f (%s, %.4f,%.4f %.4fx%.4f), "+
			"nearest container %.3f\n",
			d.bestIoU, sourceName(d.bestFrom), d.bestAt.X, d.bestAt.Y,
			d.bestAt.Width, d.bestAt.Height, d.nearContainer)
		fmt.Printf("    -> %s\n\n", d.verdict())
	}
}

func sourceName(s directorapi.ObservationSource) string {
	if s == "" {
		return "nothing"
	}
	return string(s)
}
