package fusion

import (
	"sort"
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/explain"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Explaining a cycle after the fact.
//
// The design question this file answers: when is an explanation computed?
//
// Recording one for every element on every cycle is the obvious approach and the wrong
// one. Every command observes, a warm editor reports a couple of thousand elements, and
// establishing why each of them did NOT merge with each of the others is quadratic. A
// diagnostics layer that made every command slower would be paid for by every user to
// benefit whoever happens to be debugging.
//
// So explanation is computed ON DEMAND, by re-running clustering over the retained
// cycle with recording enabled. That is sound because clustering is a pure,
// deterministic function of its observations — the same evidence produces the same
// clusters, in the same order, every time. The one thing that is NOT reproducible is
// element identity, which depends on the previous cycle's tracker state and is gone by
// the time anyone asks. So identity, and only identity, is recorded as it happens.
//
// The result is that `director explain` costs nothing until it is run, and what it
// prints is the same account that would have been recorded at the time.

// identityRecord is what must be captured live, because it cannot be recomputed.
type identityRecord struct {
	// byPrimary maps a cluster's primary observation to the element it became, and to
	// how that element got its identity. Keyed by observation rather than by position
	// because positions are an implementation detail of one run.
	byPrimary map[directorapi.ObservationID]identityEntry
}

type identityEntry struct {
	element  directorapi.ElementID
	identity explain.IdentityExplanation
}

// identityLog retains identity records for the most recent cycles.
//
// Bounded in lockstep with the observation history: an explanation is only useful for a
// cycle whose evidence is still around, and retaining more would be retaining something
// nothing can be said about.
type identityLog struct {
	mu    sync.RWMutex
	limit int
	order []observation.CycleID
	byID  map[observation.CycleID]identityRecord
}

func newIdentityLog(limit int) *identityLog {
	if limit <= 0 {
		limit = observation.DefaultHistory
	}
	return &identityLog{limit: limit, byID: map[observation.CycleID]identityRecord{}}
}

func (l *identityLog) put(id observation.CycleID, rec identityRecord) {
	if id == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, exists := l.byID[id]; !exists {
		l.order = append(l.order, id)
	}
	l.byID[id] = rec
	for len(l.order) > l.limit {
		delete(l.byID, l.order[0])
		l.order = l.order[1:]
	}
}

func (l *identityLog) get(id observation.CycleID) (identityRecord, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	rec, ok := l.byID[id]
	return rec, ok
}

// Explainer is an Engine that can account for what it did.
type Explainer interface {
	Engine
	// Explain reconstructs the reasoning behind every element of a cycle.
	//
	// The cycle must be one the engine actually fused, and one still retained: the
	// element ids and their identity reasons come from that run. A cycle the engine has
	// never seen is explained as far as clustering goes, with identity reported as
	// unknown rather than invented.
	Explain(cycle observation.Cycle) explain.CycleExplanation
}

var _ Explainer = (*engine)(nil)

// Explain reconstructs the reasoning behind one cycle's elements.
func (e *engine) Explain(cycle observation.Cycle) explain.CycleExplanation {
	out := explain.CycleExplanation{Cycle: string(cycle.ID)}

	raw := observation.Elements(cycle.Observations)
	if len(raw) == 0 {
		return out
	}

	rec := newRecorder()
	fused, _ := cluster(raw, cycle.ID, rec)
	// Text fusion re-runs with recording on, exactly as the live path ran it without.
	// It mutates the freshly-clustered elements, not the world — the labels it fills
	// here are the same ones it filled then, because the inputs and the rules are the
	// same.
	fuseText(fused, GroupLine(observation.Texts(cycle.Observations)), cycle.ID, cycle.Timestamp(), rec)
	fuseVisual(fused, observation.Visuals(cycle.Observations), cycle.ID, cycle.Timestamp(), rec)
	ident, haveIdentity := e.identities.get(cycle.ID)

	for ci, f := range fused {
		el := f.Element
		primary := f.Observations[0]

		ex := explain.ElementExplanation{
			Label:              el.Label,
			Role:               el.Role,
			PrimaryObservation: primary.Reference(string(cycle.ID)),
			MergeSteps:         rec.merges[ci],
			Rejected:           rec.rejects[ci],
			Fields:             rec.fields[ci],
			Confidence:         rec.confidence[ci],
		}
		for _, o := range f.Observations[1:] {
			ex.Supporting = append(ex.Supporting, o.Reference(string(cycle.ID)))
		}

		// Identity comes from the live run, keyed by the primary observation — the one
		// thing here that is a fact about history rather than about this evidence.
		if haveIdentity {
			if entry, ok := ident.byPrimary[primary.ID]; ok {
				ex.ElementID = entry.element
				ex.IdentityReason = entry.identity
			}
		}
		if ex.ElementID == "" {
			out.Unexplained++
			ex.IdentityReason = explain.IdentityExplanation{
				Rule: "unknown",
				Reason: "this cycle was not fused by this engine, or has aged out of the " +
					"identity log — the clustering below is reproducible, the element id is not",
			}
		}
		out.Elements = append(out.Elements, ex)
	}

	// Stable order, so two runs over the same cycle produce byte-identical output.
	// Clustering is already deterministic; this makes the PRESENTATION deterministic
	// too, which is what a caller diffing two explanations actually depends on.
	sortExplanations(out.Elements)
	return out
}

// sortExplanations orders by element id, falling back to the primary observation for
// elements the identity log could not name. Both are stable within a cycle, so two
// runs over the same evidence produce byte-identical output.
//
// The ids are minted counters, so they are compared NUMERICALLY. Sorting them as
// strings puts e10 before e2, which reads as a shuffled list and — worse — makes the
// order look non-deterministic to anyone scanning it, which is exactly the property
// this sort exists to demonstrate.
func sortExplanations(els []explain.ElementExplanation) {
	sort.SliceStable(els, func(a, b int) bool {
		na, nb := idNumber(els[a].ElementID), idNumber(els[b].ElementID)
		if na != nb {
			return na < nb
		}
		if els[a].ElementID != els[b].ElementID {
			return els[a].ElementID < els[b].ElementID
		}
		return els[a].PrimaryObservation.Observation < els[b].PrimaryObservation.Observation
	})
}

// idNumber is the counter in an element id ("e42" → 42), or -1 for anything that is
// not one — which sorts such ids first and leaves the string comparison to settle them.
func idNumber(id directorapi.ElementID) int {
	s := string(id)
	if len(s) < 2 || s[0] != 'e' {
		return -1
	}
	n := 0
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}
