package main

import (
	"fmt"
	"os"
)

// Whether the Accessibility Actor has anything behind it, and how to say so.
//
// # The Director used to refuse to boot over this
//
// `director serve` did an `os.Stat` on the bridge path and exited 1 when it missed. So a machine
// with sight, with text, with OS input and with a perfectly good Director could not observe AT
// ALL, because one Actor's provider binary had not been built. That is the wrong trade in both
// directions: it costs everything to protect one thing, and it fails at the moment least useful to
// the person — before anything has told them what Marco can still do.
//
// One missing Actor's provider should cost you that Actor. It should not cost you the Director.
//
// # Why a reason and not a bool
//
// "Accessibility is unavailable" sends somebody to search for a setting. "the accessibility bridge
// is not at plugins/uia/uia.exe — build it with: powershell -File plugins/uia/build.ps1" is a
// sentence they can act on. This is the same shape `ocrUnavailable` and `visionUnavailable` already
// have, for the same reason, and it is what lets the service SAY which Actor is missing instead of
// dying or, worse, reporting a roster that claims to be ready.

// bridgeUnavailable is why the accessibility bridge cannot be reached, empty when it can.
//
// A path check and nothing more. It deliberately does NOT spawn the bridge to find out: starting a
// subprocess to answer a question about a file would make constructing a Director an act with
// side effects, and every test in this package builds one against a path that does not exist.
func bridgeUnavailable(path string) string {
	if path == "" {
		return "no accessibility bridge is configured"
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Sprintf(
			"the accessibility bridge is not at %s\n"+
				"         build it with: powershell -File plugins/uia/build.ps1", path)
	}
	return ""
}

// AccessibilityUnavailable is why the Accessibility Actor cannot act, empty when it can.
//
// On the service.Runtime interface beside OCRUnavailable and VisionUnavailable, so a client asking
// what this Director can do gets one answer in one shape for all three.
func (r *Runtime) AccessibilityUnavailable() string {
	if r == nil {
		return ""
	}
	return r.accessibilityUnavailable
}
