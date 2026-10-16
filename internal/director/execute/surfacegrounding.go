package execute

// Asking what somebody has demonstrated about where a press goes.
//
// Three lines of wiring and one honest gap, which is the whole of what this file is for.
//
// # The gap, named rather than papered over
//
// A durable semantic target is identified by `{label, kind, place}`, and
// `StructureSignature.Discriminating` refuses one with no place: "a target with no name and no
// place could never be found again". So grounding needs the durable subject id of the SCREEN
// Marco is standing on — and the semantic request path does not know it. Places are established
// by ambient observation, in `observesession`, on its own cadence; a request arriving through
// this pipeline carries a fused world and no durable screen identity at all.
//
// Rather than invent one here — a second screen-identity mechanism is exactly what
// `semanticmemory`'s whole design refuses — the pipeline takes it from the composition root, like
// `Monitors` and `Confirmer`. Without one, grounding is asked with no place, finds nothing, and
// says so; the question is still asked, so the FIRST half of the interaction works with no
// durable memory at all.

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ground asks what has been demonstrated about this request, and leaves the pipeline unchanged
// when nothing has.
//
// The zero `surface.Grounded` is the answer for a pipeline with no grounder: no demonstration, no
// question, and resolution proceeds exactly as it did before this existed.
func (p *Pipeline) ground(w *directorapi.WorldState, in directorapi.Intent,
	request string, q directorapi.ElementQuery) surface.Grounded {

	if p.Surfaces == nil {
		return surface.Grounded{}
	}
	// THE APPLICATION THIS IS ABOUT, from the window the request is scoped to.
	//
	// The query's own application when it named one, and the active window's otherwise —
	// which is the same default `inScope` applies when deciding which Props a request can
	// even see, so grounding and resolution are talking about the same window.
	application := q.Application
	if application == "" && w != nil && w.ActiveWindow != nil {
		if win, ok := w.Window(*w.ActiveWindow); ok {
			application = win.Application
		}
	}
	return p.Surfaces.Ground(w, application, p.place(w), request, in.Verb, q)
}

// place is the durable subject id of the screen Marco is standing on, empty when nothing says.
//
// Empty is a real answer and not a failure: it means no durable place has been established for
// what is on screen, so a target on it has no durable identity and nothing stored could be about
// it. Grounding reports that in words rather than matching more loosely — see Grounder.Ground on
// why a miss costing a question is the right way round.
func (p *Pipeline) place(w *directorapi.WorldState) string {
	if p.Place == nil || w == nil {
		return ""
	}
	return p.Place(*w)
}
