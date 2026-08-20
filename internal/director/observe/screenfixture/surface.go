package screenfixture

import "github.com/chaynes-simpleclouds/marco/internal/director/observe"

// Generic surfaces, named for what they DO rather than for what product does it.
//
// # Why these replace the product archetypes
//
// The earlier fixtures in this package were called `Editor`, `Browser` and `Settings`. That was
// the mistake the live measurement exposed: modelling a product rather than a behaviour meant
// the fixture encoded a guess about that product's structure, and when the guess was wrong
// ("a settings page replaces the content region") the whole prediction was wrong with it.
//
// A real surface was 365 structures of which a whole different view changed about forty. The
// fixtures below are parameterised by the properties that actually decide the answer — how much
// chrome persists, how big the state-bearing region is, and what happens inside it — so a test
// says "a surface with persistent chrome and a replaced content region" and names nothing.
//
// Everything here is role and window-relative normalised geometry. No labels, no text, no
// titles, no content.

// Surface is a generic application surface: persistent chrome around one content region.
//
// The shape essentially every windowed application has. `Chrome` is the part that does not
// change when you go somewhere within the application — a rail, a tab strip, a status bar — and
// `Content` is the part that does.
type Surface struct {
	// Chrome is how many persistent structures surround the content.
	Chrome int
	// Content is how many structures fill the state-bearing region.
	Content int
	// ContentRole is what the content is made of. Changing it models going somewhere with a
	// different KIND of content; keeping it and moving the regions models scrolling.
	ContentRole string
	// Offset shifts the content region's items, modelling a scroll position.
	Offset float64
	// ContentMoved puts the same content in a different part of the surface — a panel docked
	// on the other side, a two-pane layout swapped, a list that moved when a view changed.
	//
	// The case that separates what the two comparisons can see. Nothing about the structure
	// changes: the same roles in the same numbers, somewhere else. A comparison reading
	// `role@col,row` notices; one reading role composition alone cannot.
	ContentMoved bool
	// Extra is a bounded region laid over the content — a modal, a dropdown, a panel.
	Extra ExtraRegion
}

// ExtraRegion is something that appears over or beside the surface.
type ExtraRegion struct {
	Role string
	// Count is how many structures it contains, and Height its share of the window.
	Count  int
	Height float64
	// Beside places it in the left margin rather than over the centre.
	Beside bool
}

// Regions renders the surface as structural evidence.
func (s Surface) Regions() []observe.ShadowRegion {
	role := s.ContentRole
	if role == "" {
		role = "text"
	}
	out := make([]observe.ShadowRegion, 0, s.Chrome+s.Content+s.Extra.Count)

	// Chrome: a left rail, a top strip and a bottom strip. Identical in every state.
	rail := s.Chrome / 3
	out = append(out, column("button", rail, 0.00, 0.10, 0.04, 0.60)...)
	out = append(out, spread("tab", rail, 0.24, 0.02, 0.60, 0.03)...)
	out = append(out, spread("pane", s.Chrome-2*rail, 0.00, 0.95, 1.00, 0.04)...)

	// Content: a vertical run inside the state-bearing region, shifted by the offset.
	//
	// `ContentMoved` puts the identical run in the opposite half of the surface. Same roles,
	// same count, different place — everything a role histogram can see is unchanged.
	cx, cw := 0.26, 0.68
	if s.ContentMoved {
		cx, cw = 0.05, 0.30
	}
	out = append(out, column(role, s.Content, cx, 0.08+s.Offset, cw, 0.84)...)

	// Anything laid over or beside it.
	if s.Extra.Count > 0 {
		x, w := 0.30, 0.40
		if s.Extra.Beside {
			x, w = 0.06, 0.16
		}
		out = append(out, column(s.Extra.Role, s.Extra.Count,
			x, 0.14, w, s.Extra.Height)...)
	}
	return out
}

// Rich is the reference surface: persistent chrome around a substantial content region.
//
// Sized from what a real accessibility tree produced — a few hundred structures with most of
// them in the content — without being any particular application.
func Rich() Surface {
	return Surface{Chrome: 30, Content: 120, ContentRole: "list_item"}
}

// Scrolled advances a VIEWPORT over the content, the way scrolling actually works.
//
// # Why this is not "move the regions"
//
// The first version of this shifted every content region by a fraction of the window, and it
// was wrong in a way that mattered: it made scrolling look like displacement. A scrolling
// viewport does not move its boxes. It keeps a fixed run of rows at a fixed pitch, shifts them
// by the sub-pitch remainder, and recycles one or two at the edges as items enter and leave.
//
// So a scroll is structurally a small offset plus a little churn — NOT a large displacement.
// Modelling it the other way round made harmless activity look more structurally violent than a
// whole new view, which is the mistake this milestone exists to stop making.
//
// `turns` is how many item-heights have gone past.
func (s Surface) Scrolled(turns float64) Surface {
	if s.Content <= 0 {
		return s
	}
	pitch := 0.84 / float64(s.Content)
	// The sub-pitch remainder is all the geometry a viewport actually shows.
	frac := turns - float64(int(turns))
	s.Offset += frac * pitch
	// And one row enters or leaves as the run crosses a boundary.
	if int(turns)%2 == 1 {
		s.Content--
	}
	return s
}

// Churned is the same surface with a few structures gained and lost.
func (s Surface) Churned(by int) Surface {
	s.Content += by
	return s
}

// ContentReplaced is the same surface showing a different KIND of content.
//
// The counterexample from the milestone: chrome unchanged, content region replaced, global
// similarity still high because the chrome dominates nothing and the content is the same size.
func (s Surface) ContentReplaced(role string) Surface {
	s.ContentRole = role
	return s
}

// Overlaid puts a bounded region over the content, leaving everything else present.
func (s Surface) Overlaid(role string, count int, height float64) Surface {
	s.Extra = ExtraRegion{Role: role, Count: count, Height: height}
	return s
}

// Beside puts a substantial region in the margin — a sidebar appearing.
func (s Surface) Beside(role string, count int) Surface {
	s.Extra = ExtraRegion{Role: role, Count: count, Height: 0.70, Beside: true}
	return s
}
