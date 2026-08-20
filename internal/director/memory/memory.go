// Package memory reduces world states to the digests the action graph stores.
// Two snapshots with the same digest describe the same screen, which is how the
// pipeline answers "did anything change?" without retaining either one. Action
// history lives in internal/director/actiongraph.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Summarise reduces a world state to the digest kept in history.
//
// Keeping a digest rather than the snapshot is what makes an unbounded action
// history affordable: a Chrome window is thousands of elements, and retaining every
// one for every action would exhaust memory within an afternoon.
func Summarise(w directorapi.WorldState) directorapi.WorldStateSummary {
	s := directorapi.WorldStateSummary{
		Timestamp:    w.Timestamp,
		ElementCount: len(w.Elements),
		WindowCount:  len(w.Windows),
		Confidence:   w.Confidence,
	}
	if w.ActiveApp != nil {
		s.Application = w.ActiveApp.Name
	}
	if win, ok := w.FocusedWindow(); ok {
		s.WindowID = win.ID
		s.WindowTitle = win.Title
		s.MonitorID = win.MonitorID
		bounds := win.Bounds
		s.WindowBefore = &bounds
	}
	for id, el := range w.Elements {
		if el.Focused {
			focused := id
			s.FocusedElement = &focused
			s.FocusedLabel = el.Label
			break
		}
	}
	s.Fingerprint = Fingerprint(w)
	return s
}

// Fingerprint is a cheap digest of what is on screen.
//
// Two snapshots with the same fingerprint describe the same screen, which is how
// "nothing changed" is answered without keeping either snapshot. It covers identity,
// label, position and the state flags a user would notice — deliberately not
// confidence or timestamps, which change without the screen changing.
func Fingerprint(w directorapi.WorldState) string {
	keys := make([]string, 0, len(w.Elements))
	for id, el := range w.Elements {
		keys = append(keys, fmt.Sprintf("%s|%s|%s|%d,%d,%d,%d|%t%t%t%t",
			id, el.Role, el.Label,
			el.Bounds.X, el.Bounds.Y, el.Bounds.Width, el.Bounds.Height,
			el.Enabled, el.Visible, el.Focused, el.Selected))
	}
	// Map iteration is randomised, so the digest must be order-independent or two
	// identical screens would fingerprint differently.
	sort.Strings(keys)

	h := sha256.New()
	for _, win := range sortedWindows(w) {
		fmt.Fprintf(h, "w|%s|%s|%d,%d,%d,%d|%t%t\n", win.ID, win.Title,
			win.Bounds.X, win.Bounds.Y, win.Bounds.Width, win.Bounds.Height,
			win.Minimized, win.Maximized)
	}
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func sortedWindows(w directorapi.WorldState) []directorapi.Window {
	out := append([]directorapi.Window(nil), w.Windows...)
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}
