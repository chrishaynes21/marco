// Package world queries the World Model: the Director's structured belief about what
// currently exists on the computer.
//
// It used to build the world as well. It no longer does. Construction moved to
// internal/director/perception/fusion, which is the only component permitted to turn
// evidence into belief; what is left here is the belief-side vocabulary everything
// downstream shares — reading order, per-window filtering, and the one-line summary
// that goes in an explanation log.
//
// The distinction is worth the split. Anything in this package may be called by the
// planner, the target resolver, verification and replay, all of which reason over
// elements and must have no way to ask which source produced one. Anything in fusion
// knows about observations. Nothing knows about both.
package world

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
	"sort"
)

// Elements returns the snapshot's elements in a stable order: by window, then by
// reading order (top to bottom, left to right), then by ID.
//
// Map iteration in Go is deliberately randomised, so anything that ranks, lists or
// numbers elements ("the second tab") must sort first or it will produce different
// answers on identical input.
func Elements(w *directorapi.WorldState) []*directorapi.Element {
	out := make([]*directorapi.Element, 0, len(w.Elements))
	for _, el := range w.Elements {
		out = append(out, el)
	}
	fusion.SortElements(out)
	return out
}

// InWindow returns the elements belonging to one window, in reading order.
func InWindow(w *directorapi.WorldState, id directorapi.WindowID) []*directorapi.Element {
	all := Elements(w)
	out := all[:0:0]
	for _, el := range all {
		if el.WindowID == id {
			out = append(out, el)
		}
	}
	return out
}

// Summarise renders a one-line description of the snapshot for the explanation log:
// what the Director believed was on screen when it decided.
func Summarise(w *directorapi.WorldState) string {
	app := "unknown app"
	if w.ActiveApp != nil && w.ActiveApp.Name != "" {
		app = w.ActiveApp.Name
	}
	title := ""
	if win, ok := w.FocusedWindow(); ok {
		title = win.Title
	}

	seen := map[directorapi.ObservationSource]bool{}
	for _, o := range w.Observations {
		seen[o.Source] = true
	}
	sources := make([]string, 0, len(seen))
	for s := range seen {
		sources = append(sources, string(s))
	}
	sort.Strings(sources)

	s := app
	if title != "" {
		s += " — " + title
	}
	s += ", " + itoa(len(w.Elements)) + " elements"
	if len(sources) > 0 {
		s += ", sources: " + join(sources, "+")
	}
	if len(w.Degraded) > 0 {
		s += ", degraded: " + itoa(len(w.Degraded))
	}
	return s
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
