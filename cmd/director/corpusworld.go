package main

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A RECORDED APPLICATION, PUT BACK IN FRONT OF THE REAL RESOLVER.
//
// The desktop perception corpus holds what production believed about real windows — Windows
// Settings on two pages, File Explorer, a browser, a hand-written canvas application — read
// by the real accessibility walk and fused by the real engine, then reviewed by a person and
// committed. See [[Experiment-016-desktop-perception-corpus]].
//
// This turns one of those samples back into a World State so the production resolver can be
// asked production questions about it. That is what makes the rewording evaluation a
// measurement rather than a demonstration: the same code answers, over the same applications,
// with no desktop in front of it and no window to bring to the foreground — so it is
// repeatable, it is a gate, and it can be run against an application nobody tuned against.
//
// # What a sample does not carry, and what that costs
//
// Three things, and each is named here because the measurement has to be read with them in
// mind rather than discovered later:
//
// NO PARENT LINKS. A sample records elements, not the tree they sat in, so "the second item
// of the navigation list" cannot be asked. The structural reading in this evaluation is
// therefore ROLE and GEOMETRY only, which is a floor on what structure can contribute rather
// than a ceiling — a live world has the links.
//
// NO PER-ELEMENT CONFIDENCE. Held at corpusElementConfidence below, which scales every
// candidate's score by the same factor. Orderings are unaffected; the ambiguity margin is an
// absolute difference, so it shifts by that factor and a near-tie could in principle fall on
// the other side of it. Checked against the live measurement: the exact-label scores this
// produces are the 0.97 the live Settings window produced.
//
// NO SELECTION STATE. `Element.Selected` is not in the sample format, which is why the
// already-satisfied column of this evaluation measures what it does. The field was added to
// the capture so a later sample can carry it; the committed ones predate it.
//
// # Why the confidence comes from fusion
//
// Because the resolver's first question of any world is whether the world could answer at
// all, and a world with an assumed confidence answers it wrongly. A zero WorldConfidence is
// BLIND — nothing here can be operated — so every request against a fixture built that way
// returns UNOBSERVABLE and the evaluation measures the adapter instead of the resolver.
func desktopWorld(s desktopSample) directorapi.WorldState {
	win := directorapi.WindowID(s.Window)
	if win == "" {
		win = directorapi.WindowID("hwnd:" + s.ID)
	}
	frame := s.Bounds
	if frame.Width <= 0 || frame.Height <= 0 {
		frame = directorapi.Rect{Width: s.Width, Height: s.Height}
	}

	els := make(map[directorapi.ElementID]*directorapi.Element, len(s.Elements))
	for _, e := range s.Elements {
		el := &directorapi.Element{
			ID:         directorapi.ElementID(e.ID),
			WindowID:   win,
			Role:       directorapi.ElementRole(e.Role),
			Label:      e.Label,
			Bounds:     absolute(e.Bounds, frame),
			Enabled:    e.Enabled,
			Visible:    e.Visible,
			Selected:   e.Selected,
			Confidence: corpusElementConfidence,
		}
		for _, name := range e.Sources {
			src := directorapi.ObservationSource(name)
			el.Sources = append(el.Sources, src)
			// Provenance, not just Sources, because Actionability.Actuable is read off
			// provenance: an element only a camera saw affords whatever its shape
			// affords and still may not be operated. A sample whose provenance was
			// left empty would report every pixel-only detection as operable, which
			// is the one thing ADR-202 exists to refuse.
			el.Provenance.Add(directorapi.ObservationReference{
				Observation: directorapi.ObservationID(e.ID + ":" + name),
				Source:      src,
			})
		}
		els[el.ID] = el
	}

	w := directorapi.WorldState{
		Elements: els,
		Windows: []directorapi.Window{{
			ID: win, Application: s.Application, Title: titleOf(s),
			Bounds: frame, Focused: true, Visible: true,
		}},
		ActiveWindow: &win,
		Timestamp:    s.ReadAt,
	}
	w.Confidence = fusion.ConfidenceOf(&w)
	return w
}

// corpusElementConfidence is what a recorded element's confidence is taken to be.
//
// 0.9, which is what a single structured source corroborated by the window system reaches in
// production, and which reproduces the exact-label score the live Settings measurement
// printed: 0.7 + 0.3 x 0.9 = 0.97.
const corpusElementConfidence = 0.9

// titleOf is the window's caption as the sample recorded it, which is the window element's
// own label: a sample names its window by handle and its application by process.
func titleOf(s desktopSample) string {
	for _, e := range s.Elements {
		if directorapi.ElementRole(e.Role) == directorapi.RoleWindow && e.Label != "" {
			return e.Label
		}
	}
	return s.Application
}

// absolute turns a region expressed as a proportion of the window back into pixels.
func absolute(r observe.Region, frame directorapi.Rect) directorapi.Rect {
	if frame.Width <= 0 || frame.Height <= 0 {
		return directorapi.Rect{}
	}
	return directorapi.Rect{
		X:      frame.X + int(r.X*float64(frame.Width)+0.5),
		Y:      frame.Y + int(r.Y*float64(frame.Height)+0.5),
		Width:  int(r.Width*float64(frame.Width) + 0.5),
		Height: int(r.Height*float64(frame.Height) + 0.5),
	}
}
