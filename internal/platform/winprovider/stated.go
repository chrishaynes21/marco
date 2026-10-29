package winprovider

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// DesktopForTest states the whole window system, for a test that must not borrow the owner's
// screen.
//
// # One setter, because three would disagree
//
// The desktop is read three ways — the list a person could mean, the lookup of one handle, and
// whether a process is alive — and a fixture that stated only the first would be answered by the
// REAL machine for the other two. That is not a hypothetical: the same shape, a fixture window
// list under a live foreground reading, made `orchestrator`'s ordering test pass while the rule
// it names did nothing. So the seam has no individual knobs. Everything comes out of this list,
// a handle that is not in it does not exist, and `ProcessAlive` is true exactly for the owners
// of the windows given.
//
// # Ownership is answered as "not Marco's"
//
// A stated window is a window the person could have meant, which is what a fixture is for. The
// real `IsOwnedSurface` reads a property off a live handle and would answer false for every
// handle here anyway; stating it keeps the answer from depending on which handles the machine
// happens to have.
//
// Exported because the tests that need it are in `cmd/director`, and `testing.TB` rather than a
// bare setter so the restore cannot be forgotten: a package-level seam left pointing at a
// fixture would make every later test in that binary resolve against a desktop that is not
// there.
func DesktopForTest(t testing.TB, windows []winctx.LiveWindow) {
	t.Helper()
	wasWindows, wasLookUp := liveWindows, lookUpWindow
	wasAlive, wasOwned := processAlive, ownedSurface
	t.Cleanup(func() {
		liveWindows, lookUpWindow = wasWindows, wasLookUp
		processAlive, ownedSurface = wasAlive, wasOwned
	})

	stated := make([]winctx.LiveWindow, len(windows))
	copy(stated, windows)

	liveWindows = func() []winctx.LiveWindow { return stated }
	lookUpWindow = func(handle uintptr) (winctx.LiveWindow, bool) {
		for _, w := range stated {
			if w.Handle == handle {
				return w, true
			}
		}
		return winctx.LiveWindow{}, false
	}
	processAlive = func(pid uint32) bool {
		for _, w := range stated {
			if w.ProcessID == pid {
				return true
			}
		}
		return false
	}
	// THIS ARM IS UNFALSIFIABLE AND STAYS ANYWAY, which the mutation gate recorded
	// rather than contrived against. The handles a fixture states do not exist, so the
	// real `IsOwnedSurface` reads a property off nothing and answers false for them
	// already — deleting this line changes no behaviour any test can reach.
	//
	// It stays because the two are false for different reasons: the real one is false
	// because the property could not be read, and this is false because a stated window
	// IS a window the person could have meant. A fixture handle that happened to collide
	// with a live Marco surface would otherwise vanish from its own desktop, and the
	// collision is not something a test can arrange.
	ownedSurface = func(uintptr) bool { return false }
}
