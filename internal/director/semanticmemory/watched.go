package semanticmemory

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Candidate evidence: what watching has seen, kept until Marco decides whether to know it.
//
// # Two properties, and both of them are the reason this is separate from everything else here
//
// It is NOT KNOWLEDGE. Nothing plans over a candidate, nothing resolves one, and nothing here can
// turn one into a relationship — the interface has no method for it. Promotion happens somewhere
// else, through the same admission boundary an explicit Learn goes through.
//
// It is BOUNDED BY NOVELTY. A relationship seen ten thousand times is one record with a bigger
// number on it; the bound exists for the person who meets a thousand distinct controls over a
// month, and eviction is deterministic so two runs over the same evidence forget the same things.
//
// See [[ADR-095-repeated-observation-may-become-knowledge]].

var _ observe.WatchedStore = (*Store)(nil)

// RememberWatched writes one candidate summary, replacing any held under the same id.
//
// UPSERT, never append. Repeated evidence about one relationship is one record: that is what makes
// storage track how many different things somebody does rather than how long Marco watched, and it
// is the same rule every other record in this store follows.
//
// Deleting the replace must fail TestRepeatedEvidenceIsOneRecordNotAPile.
func (s *Store) RememberWatched(e observe.WatchedEdge) error {
	if s == nil {
		return fmt.Errorf("semanticmemory: no store")
	}
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("semanticmemory: a candidate with no id cannot be found again")
	}
	s.mu.Lock()
	if s.unavailable != "" {
		reason := s.unavailable
		s.mu.Unlock()
		return fmt.Errorf("semanticmemory: not writing: %s", reason)
	}
	replaced := false
	for i := range s.watched {
		if s.watched[i].ID == e.ID {
			s.watched[i] = e
			replaced = true
			break
		}
	}
	if !replaced {
		s.evictWatchedLocked()
		if len(s.watched) >= observe.MaxWatchedEdges {
			// EVERY candidate is at least as strong as this one, so the honest thing is
			// to refuse the new one rather than displace something better. Counted, so a
			// full store reads as full.
			s.dropped++
			s.mu.Unlock()
			return nil
		}
		s.watched = append(s.watched, e)
	}
	snapshot := s.snapshotLocked()
	s.mu.Unlock()
	return save(s.path, snapshot)
}

// evictWatchedLocked makes room for one more candidate by forgetting the weakest.
//
// # Deterministic, and it drops the weakest rather than the oldest
//
// A store that evicted by insertion order would forget a candidate one sighting from promotion in
// favour of a thing somebody did once this morning. The order is observe.WatchedEdge.WeakerThan:
// promoted last, then uncontradicted, then by independent occasions, then sightings, then
// last-seen, then id. Every tie breaks on something, so no eviction depends on how a slice
// happened to be laid out.
//
// Deleting the weakest-first rule must fail TestEvictionForgetsTheWeakestCandidateFirst.
func (s *Store) evictWatchedLocked() {
	if len(s.watched) < observe.MaxWatchedEdges {
		return
	}
	weakest := -1
	for i := range s.watched {
		if weakest < 0 || s.watched[i].WeakerThan(s.watched[weakest]) {
			weakest = i
		}
	}
	if weakest < 0 {
		return
	}
	// A PROMOTED candidate is never evicted to make room for a new one. It is the provenance
	// of something Marco now knows, and losing it would leave durable knowledge unable to say
	// where it came from.
	if !s.watched[weakest].Promoted.IsZero() {
		return
	}
	s.watched = append(s.watched[:weakest], s.watched[weakest+1:]...)
	s.dropped++
}

// Watched returns the candidate evidence for one application.
//
// Ordered deterministically — strongest first — because a caller looking for something to promote
// should meet the best evidence first, and because a list that reordered itself between two reads
// of an unchanged store would look like something had happened.
func (s *Store) Watched(application string) []observe.WatchedEdge {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []observe.WatchedEdge
	for _, w := range s.watched {
		if strings.EqualFold(w.Application, application) {
			out = append(out, w)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[b].WeakerThan(out[a]) })
	return out
}
