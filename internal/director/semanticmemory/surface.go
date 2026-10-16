package semanticmemory

// Where a person demonstrated that the press goes.
//
// The durable half of [[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]], implemented
// by the store that already holds places, routes, targets and rehearsals. There is no second
// database and no second matcher: a record is found by the target subject id the one
// `RememberTarget` returned, and the lookup rule itself lives in `observe.SurfaceFor` so that the
// store and the reader cannot drift apart.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// MaxDemonstratedSurfaces bounds how many of these one store keeps.
//
// Growth is per TARGET, not per demonstration: showing Marco the same control ten times is one
// record, because a later demonstration REPLACES an earlier one. A person who changes their mind
// has not added a fact, they have corrected one — and two records for one target is exactly the
// contradiction `observe.SurfaceFor` refuses rather than resolves.
//
// Smaller than MaxRehearsals and far smaller than MaxSubjects, because this is the one durable
// record that only ever arrives by somebody being asked a question and answering it. A store with
// a hundred of these describes a person who has answered a hundred questions.
const MaxDemonstratedSurfaces = 128

// RememberSurface keeps what one demonstration established about one target's surface.
//
// # What it refuses, and why each refusal is here rather than at the reader
//
// A record it cannot vouch for is refused outright — `observe.DemonstratedSurface.Valid` holds
// that list, so the rule is stated once and the store does not grow a second opinion. A record
// whose target this store has never heard of is refused too: the id is the whole of how it will
// be found again, and a surface pointing at nothing is a saved aiming decision with nothing to
// aim at.
//
// What it does NOT refuse is an unconfirmed demonstration. A press somebody made that nothing
// could confirm is a real thing that happened, and discarding it would make the person show
// Marco again for nothing. It is kept and `Usable` refuses to aim with it — the same shape as
// `RehearsalEvidence.VerifiedBy`, where the judgement is derived on every read rather than
// decided once at the door.
//
// One record per target: a later demonstration replaces an earlier one, because the question a
// reader asks is "where does the press go now", not "how many times has this been shown".
func (s *Store) RememberSurface(application string, d observe.DemonstratedSurface) error {
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	d.Application = application
	if ok, why := d.Valid(); !ok {
		return fmt.Errorf("semanticmemory: %s; not stored", why)
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	known := false
	for _, r := range s.subjects {
		if r.ID == d.Target {
			known = true
			break
		}
	}
	if !known {
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: no durable target %q is known, so a surface "+
			"for it could never be found again; not stored", d.Target)
	}
	replaced := false
	for i, existing := range s.surfaces {
		if existing.Target == d.Target &&
			strings.EqualFold(existing.Application, application) {
			s.surfaces[i] = d
			replaced = true
			break
		}
	}
	if !replaced {
		if len(s.surfaces) >= MaxDemonstratedSurfaces {
			s.dropped++
			s.mu.Unlock()
			return fmt.Errorf("semanticmemory: at the %d-surface bound; not stored",
				MaxDemonstratedSurfaces)
		}
		s.surfaces = append(s.surfaces, d)
	}
	// By target id, so two readings of one store produce the same account.
	sort.SliceStable(s.surfaces, func(i, j int) bool {
		if !strings.EqualFold(s.surfaces[i].Application, s.surfaces[j].Application) {
			return s.surfaces[i].Application < s.surfaces[j].Application
		}
		return s.surfaces[i].Target < s.surfaces[j].Target
	})
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// DemonstratedSurfaces returns what is stored for one application, in a stable order.
func (s *Store) DemonstratedSurfaces(application string) []observe.DemonstratedSurface {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []observe.DemonstratedSurface
	for _, d := range s.surfaces {
		if strings.EqualFold(d.Application, application) {
			out = append(out, d)
		}
	}
	return out
}
