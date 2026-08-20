//go:build !windows

package oshost

import "errors"

// Clipboard access is Windows-first, like the rest of this package's OS surfaces.
//
// A stub that FAILS rather than one that silently succeeds. A clipboard write that
// reported success without writing would let the Director's clipboard strategy paste
// stale content and verify it as correct — worse than not offering the capability at
// all, because the failure would be invisible.
func clipboardText() (string, bool, bool, error) {
	return "", false, false, errors.New("clipboard: not implemented on this platform")
}

func setClipboardText(string) error {
	return errors.New("clipboard: not implemented on this platform")
}

func sleepMs(int) {}
