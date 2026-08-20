package palworld

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Evidence this pack can produce and the Director cannot.
//
//	Verification must remain semantic.
//	Craft → evidence: craft queue changed, inventory changed, item count increased.
//	Not: pixels moved.
//
// The Director, watching a deposit, sees a window whose element count changed. That is
// weak evidence of something. What it cannot see is that the player's inventory went from
// eleven filled slots to none while the container went from none to eleven — which is not
// weak evidence of something, it is what depositing IS.
//
// So this reads the two worlds through the entities this pack's observer contributed, and
// says what changed in terms of them. It cannot act, cannot observe, cannot refuse an
// action and cannot outweigh the Director's own reading: contributed evidence is capped at
// verify.MaxContributedWeight and only ever added.
//
// # Every claim here is about entities, never about pixels
//
// There is no screenshot comparison in this file and no way to reach one. The only inputs
// are two World States, and the only things read out of them are entities — which came from
// observations, which came from the ordinary perception pipeline.

// evidence contributes Palworld-specific verification evidence.
type evidence struct{}

func (*evidence) Name() string { return Name }

// Evidence reports what changed between two worlds, in this pack's terms.
//
// Returns nothing for an action it cannot judge, and nothing when neither world carries
// entities — which is the ordinary case outside the game and the case where this pack has
// no business having an opinion.
func (*evidence) Evidence(_ directorapi.Action, _ directorapi.ResolvedTarget,
	before, after directorapi.WorldState) []directorapi.Evidence {

	var out []directorapi.Evidence

	beforeInv := game.ReadInventory(before, "")
	afterInv := game.ReadInventory(after, "")
	if beforeInv.Slots == 0 && afterInv.Slots == 0 {
		// Neither world was modelled. Nothing to say, and saying nothing is what lets the
		// Director's own verdict stand unchanged.
		return nil
	}

	// The player's own holdings changing is the clearest thing this pack can see.
	if beforeInv.Filled != afterInv.Filled {
		out = append(out, directorapi.Evidence{
			Kind: "inventory_changed", Observed: true, Weight: 0.65,
			Detail: fmt.Sprintf("the inventory went from %d filled slot(s) to %d",
				beforeInv.Filled, afterInv.Filled),
		})
	}

	// An item's count going UP is what a craft looks like from here, and it is a
	// different claim from "something changed": it says the thing that was asked for now
	// exists in greater number than it did.
	for _, gained := range gainedItems(beforeInv, afterInv) {
		out = append(out, directorapi.Evidence{
			Kind: "item_count_increased", Observed: true, Weight: 0.7,
			Detail: gained,
		})
	}

	// A queue gaining or losing work.
	if detail, changed := queueChanged(before, after); changed {
		out = append(out, directorapi.Evidence{
			Kind: "craft_queue_changed", Observed: true, Weight: 0.65, Detail: detail,
		})
	}

	// A station reaching a new state — an incubator that started, a craft that finished.
	for _, detail := range stationStates(before, after) {
		out = append(out, directorapi.Evidence{
			Kind: "station_state_changed", Observed: true, Weight: 0.6, Detail: detail,
		})
	}
	return out
}

// gainedItems reports the items whose readable count went up.
//
// Readable in BOTH worlds. An item that could not be counted before and can be now says
// only that it became readable, which is a fact about perception rather than about the
// game — and reporting it as a gain would verify a craft that never ran.
func gainedItems(before, after game.Inventory) []string {
	was := countsOf(before)
	var out []string
	for name, now := range countsOf(after) {
		then, known := was[name]
		if !known || now <= then {
			continue
		}
		out = append(out, fmt.Sprintf("%s went from %d to %d", name, then, now))
	}
	return out
}

// countsOf totals each named item's readable quantity.
func countsOf(inv game.Inventory) map[string]int {
	out := map[string]int{}
	for _, it := range inv.Items {
		n, ok := it.Entity.Count()
		if !ok || it.Entity.Name == "" {
			continue
		}
		out[it.Entity.Name] += n
	}
	return out
}

// queueChanged reports a change in how much work is queued.
func queueChanged(before, after directorapi.WorldState) (string, bool) {
	b, bok := queueDepth(before)
	a, aok := queueDepth(after)
	if !bok || !aok || a == b {
		return "", false
	}
	return fmt.Sprintf("the crafting queue went from %d entr(y/ies) to %d", b, a), true
}

// queueDepth counts the queue entries a world reports.
func queueDepth(w directorapi.WorldState) (int, bool) {
	found, n := false, 0
	for _, el := range w.Elements {
		e := el.Entity
		if e.Known() && e.Kind == directorapi.EntityQueue {
			found = true
			if c, ok := e.Count(); ok {
				n += c
			} else {
				n++
			}
		}
	}
	return n, found
}

// stationStates reports the stations whose state changed.
func stationStates(before, after directorapi.WorldState) []string {
	was := map[string]string{}
	for _, s := range game.Stations(before) {
		was[s.Name] = s.State
	}
	var out []string
	for _, s := range game.Stations(after) {
		then, known := was[s.Name]
		if !known || then == s.State || s.State == "" {
			continue
		}
		out = append(out, fmt.Sprintf("%s went from %q to %q", s.Name, then, s.State))
	}
	return out
}
