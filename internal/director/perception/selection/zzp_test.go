package selection_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

func TestZZDwellBoundary(t *testing.T) {
	for _, dwell := range []int{3, 4, 5, 6, 8} {
		for _, varied := range []bool{false, true} {
			tr := selection.NewTracker()
			mark := 0
			for i := range 240 {
				if i >= 12 && i%dwell == 0 {
					mark = (mark + 1) % 5
				}
				sh := shadesWithMark(5, mark)
				if varied {
					for r := range sh {
						base := 50 + float64(r)*9
						sh[r] = selection.Shade{base, base + 30}
						if r == mark {
							sh[r] = selection.Shade{base + 150, base + 180}
						}
					}
				}
				tr.ObserveNavigated(listRows(0, 5, 100, 100), sh, false)
			}
			for _, e := range tr.Evidence() {
				t.Logf("dwell %-2d varied=%-5v excl=%-4d shared=%-4d quiet=%-4d moves=%d",
					dwell, varied, e.Exclusive, e.Shared, e.Quiet, e.Transfers)
			}
		}
	}
}
