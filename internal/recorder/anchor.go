package recorder

import (
	"os"
	"strings"
)

// DefaultAnchorKey is the key you TAP during a demonstration to start an image
// anchor; the next click ends it — that click is captured as an image anchor.
// Codegen turns it into a Find-by-image click with the recorded coordinate as
// fallback, so it survives a menu moving. Tap-then-click is one-handed (no holding).
// F12 by default; it's swallowed during recording so it never reaches the app. When
// F12 is also the stop key (the CLI default), stopping wins and anchoring is off
// until you point MARCO_ANCHOR_KEY at another key — in the overlay the leader stops
// the demo, so F12 is free to anchor.
const DefaultAnchorKey = "f12"

// AnchorKey returns the configured anchor-key name (lowercase), or "" when
// disabled via MARCO_ANCHOR_KEY=off. Read once at recorder init and surfaced in the
// teach prompt so the gesture is discoverable.
func AnchorKey() string {
	switch v := strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_ANCHOR_KEY"))); v {
	case "":
		return DefaultAnchorKey
	case "off":
		return ""
	default:
		return v
	}
}
