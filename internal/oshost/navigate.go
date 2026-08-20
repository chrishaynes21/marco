package oshost

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// `OS's Navigate` — a navigation MEANING, and the one place a meaning becomes a key.
//
// # Why this exists at all
//
// The Director learned `confirm`. It did not learn Enter, and it must not: a caller that said
// "press Enter" would have decided a binding, and a binding is a property of a device and an
// application, not of what somebody meant. [[ADR-013-navigation-is-meaning-not-keys]] settled
// that for the WATCHING direction — `internal/platform/navsource` refuses to admit a key whose
// navigation meaning is not conventional. This is the same rule pointed the other way.
//
// So the mapping lives HERE, backstage, beside the code that already knows how to press things,
// and the table below is the only place in the repository where "confirm" becomes a keystroke on
// the acting side.
//
// # Why the table is not navsource's, reversed
//
// The two directions are not inverses of one another, and pretending they were would be a bug.
// Watching admits W/A/S/D and Space as CONDITIONAL evidence — they mean navigation only while the
// screen looked like a set of choices, and they arrive marked as such. Acting cannot be
// conditional: there is exactly one key to press, and pressing W when the arrow key was meant
// would drive the car. Each meaning therefore maps to its single unambiguous key, and the
// conditional surrogates appear nowhere.
var navigationKeys = map[string]string{
	"up":      "up",
	"down":    "down",
	"left":    "left",
	"right":   "right",
	"confirm": "enter",
	"back":    "backspace",
	"pause":   "escape",
}

// doNavigate presses the key one navigation meaning corresponds to.
//
// `point` is deliberately absent: a pointer press needs a position, and a position is not a
// meaning. A caller wanting one asks for `OS's Click` with a Point, which is the capability that
// already exists for it.
func (h *Host) doNavigate(c runtime.HostCall) (string, runtime.Value, error) {
	intent := strings.ToLower(strings.TrimSpace(c.Input.AsText()))
	if intent == "" {
		return fail("navigate needs a navigation meaning, e.g. \"confirm\"")
	}
	key, known := navigationKeys[intent]
	if !known {
		// A closed vocabulary, refused rather than guessed. An unknown meaning that fell
		// through to "press the text" would turn a typo into a keystroke.
		return fail(fmt.Sprintf("navigate has no meaning %q", intent))
	}
	return ok(h.b.key(c.Ctx, key, 0))
}
