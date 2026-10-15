package main

import (
	"context"
	"testing"
)

// (0,0) IS NOT A POSITION.
//
// `winctx.CursorPos` answers (0,0) from its off-Windows stub, so reading that as a real reading
// would fail every action on every platform that cannot answer — turning a gate against one
// defect into a refusal to act at all. The gate's own rule is that nobody-could-ask permits the
// press, and this is where that rule is kept for the platform.
//
// That the production command installs the gate at all is held by
// TestPerformWorkflowInstallsItsGates, beside the other calls whose deletion would leave a
// runner that compiles, runs, presses, and is missing a guarantee.
func TestAPointerAtTheOriginIsReportedAsUnknownAndNotAsAPosition(t *testing.T) {
	if x, y, ok := pointerNow(context.Background()); ok && x == 0 && y == 0 {
		t.Error("(0,0) was reported as a measured position, and it is the stub's answer")
	}
}

// AND A STOPPED RUN MEASURES NOTHING.
//
// A position taken after somebody pressed stop would license a press on it. Cancellation is not
// evidence about the pointer, and the gate treats it as the one thing it treats as permissive —
// so it has to be the unknown answer, not a reading.
func TestACancelledRunDoesNotReportAPointerPosition(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, ok := pointerNow(ctx); ok {
		t.Error("the run was stopped and the pointer was still reported as measured")
	}
}
