//go:build windows

package recorder

// WHAT THE HOOK PAYS FOR A CLICK, AS A NUMBER.
//
// The callback invariant is that a low-level hook callback must be bounded and FAST: past
// `LowLevelHooksTimeout` — 300ms by default — Windows stops calling it, silently, and the stop
// key stops working. So any work added to the hook path is a claim about that budget, and a claim
// about a budget should be a measurement rather than a sentence saying it is probably fine.
//
// This exists because the release edge of a click started being measured, for the guarded drag: a
// drag ends somewhere else, so its destination is a point in a window and has to be measured
// where it happened. That is one more `clickShape` per mouse event than the hook used to make.
//
// On the development machine (2026-10-26): `clickShape` 100.5 ns/op, both edges of a click
// through `newClick` 809.5 ns/op together. The budget is 300,000,000 ns. Six orders of magnitude,
// and the point of writing it down is that the next person adding something here can re-run this
// instead of re-reasoning about it — the hazard on this path is a PROCESS HANDLE or a screen
// capture, which costs milliseconds, not window data Windows already holds.
//
// Not a test and never asserted. A timing assertion in the default suite is a flake on a busy
// machine, and the measurement's value is the number a person reads.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// BenchmarkClickShape is the call the release edge added: `GetWindowRect` plus
// `GetDpiForWindow`, window data, no process handle opened.
func BenchmarkClickShape(b *testing.B) {
	hwnd, err := winctx.Bind("explorer")
	if hwnd == 0 {
		b.Skipf("no window to measure against: %v", err)
	}
	for b.Loop() {
		clickShape(hwnd)
	}
}

// BenchmarkNewClickBothEdges is what the hook actually calls, both edges, including the
// attribution each one makes and the foreground title the press reads.
func BenchmarkNewClickBothEdges(b *testing.B) {
	for b.Loop() {
		aClick("left", true, 500, 400)
		aClick("left", false, 600, 450)
	}
}
