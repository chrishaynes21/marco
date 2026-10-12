//go:build windows

package screen

import "testing"

// The validity check is one bit, and it has to be the right bit.
//
// A blit that fails silently returns black. A configuration that was faster because it returned
// nothing would read as an improvement, and the only defence is asking whether the buffer held
// anything at all — which is deliberately the weakest question that answers it, and therefore
// also the cheapest.
//
// Deleting the loop, or returning the wrong way round, must fail this.
func TestTheValidityCheckIsOneBitAndTheRightOne(t *testing.T) {
	if anyNonZero(make([]byte, 64)) {
		t.Error("a buffer of zeroes reported that it held something. A failed blit " +
			"returns black, and this is what tells a fast capture from an empty one")
	}
	// Anything at all, anywhere, counts — including the last byte, which is where a
	// loop that stopped one short would miss it.
	b := make([]byte, 64)
	b[63] = 1
	if !anyNonZero(b) {
		t.Error("a buffer whose last byte is set reported empty")
	}
	b = make([]byte, 64)
	b[0] = 255
	if !anyNonZero(b) {
		t.Error("a buffer whose first byte is set reported empty")
	}
	// An empty slice is empty rather than a panic: a zero-area destination is refused
	// before this is reached, and it must not be the thing that proves it.
	if anyNonZero(nil) {
		t.Error("a nil buffer reported that it held something")
	}
}
