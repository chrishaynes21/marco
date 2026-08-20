package observation

import "github.com/chaynes-simpleclouds/marco/pkg/directorapi"

// Telling a window's own machinery from the page inside it.
//
// # What this is for
//
// A durable PLACE is what a screen is made of. A window's title bar and its scroll bars are not
// part of that: they exist because there is a window, and they come and go with the window rather
// than with the page. Counted into a place's identity they make the same page look like several.
//
// That is measured, not assumed. Windows Settings minted a fresh durable subject for the same
// page on almost every visit, and three families of twins in one store each differed from their
// named original by exactly three buttons — the frame's own Minimize, Restore and Close.
//
// # Hierarchy, not geometry and not names
//
// An earlier attempt excluded elements with a degenerate rectangle. It was wrong, and the way it
// was wrong is the reason this file exists: Windows Settings reports legitimate page content —
// a combo box, links, nineteen pieces of text — with `0,0 0x0` bounds, and excluding those broke
// recall on real screens. Geometry describes how something happens to be laid out at one instant.
// OWNERSHIP describes what it belongs to, and that is the question being asked.
//
// It is also not a name rule. "Minimize", "Restore" and "Close" are one operating system's words
// in one language, and a classifier built on them would silently stop working on the next.
//
// So: an element is chrome when it IS, or descends from, a window-frame part. The hierarchy is
// already there — the accessibility provider supplies `ParentNativeID` for every element — and
// this reads it. Nothing here is specific to an application.
//
// # What this does NOT do
//
// It does not remove anything. Chrome is still observed, still fused, still addressable, and
// still shown by `director inspect` and by Sight. A person asking Marco to press Close must still
// be able to. The classification exists so that ONE consumer — the durable place signature — can
// leave it out of what makes a screen that screen. See [[ADR-062-a-scroll-bar-is-not-a-screen]].

// chromeParts are the roles that ARE a window's machinery, and whose subtrees therefore are too.
//
// Deliberately tiny, and deliberately not "things that seem unimportant". Each has the same
// property: it exists because of the window, not because of the page. Resize the window and the
// scroll bar goes away without a single thing on the page changing; the title bar belongs to the
// frame that hosts the page rather than to the page. This list may only grow on that evidence.
var chromeParts = map[directorapi.ElementRole]bool{
	directorapi.RoleScrollBar: true,
	directorapi.RoleTitleBar:  true,
}

// ChromeIn is every element in a cycle that belongs to the window's own machinery, by native id.
//
// The ONE classifier. `director inspect -chrome` reports with it and the observation pipeline
// carries its answer forward, so what a person is shown and what identity uses cannot disagree —
// two classifiers would eventually be two answers to "is this part of the page".
//
// Orphans — elements naming a parent the snapshot does not contain — are treated as CONTENT. A
// snapshot with a broken hierarchy must not silently start dropping page structure from identity;
// failing open here means an incomplete tree produces a place that is merely harder to match,
// never one that is missing half its composition.
func ChromeIn(obs []Observation) map[string]bool {
	nodes := make([]ChromeNode, 0, len(obs))
	for _, o := range obs {
		el, ok := o.(Element)
		if !ok || el.Raw.NativeID == "" {
			continue
		}
		nodes = append(nodes, ChromeNode{
			ID: el.Raw.NativeID, Parent: el.Raw.ParentNativeID, Role: el.Raw.Role,
		})
	}
	return Chrome(nodes)
}

// ChromeNode is one element of a hierarchy, reduced to what the rule reads.
//
// The two callers name their elements differently — an observation has a provider.s native id, a
// fused world element has the Director.s own — so the rule is expressed over this and adapted to,
// rather than written twice. There is one walk.
type ChromeNode struct {
	ID, Parent string
	Role       directorapi.ElementRole
}

// Chrome is the classification itself: every node that IS, or descends from, a window-frame part.
func Chrome(nodes []ChromeNode) map[string]bool {
	type node struct {
		role   directorapi.ElementRole
		parent string
	}
	byID := make(map[string]node, len(nodes))
	order := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.ID == "" {
			continue
		}
		byID[n.ID] = node{role: n.Role, parent: n.Parent}
		order = append(order, n.ID)
	}
	out := map[string]bool{}
	for _, id := range order {
		n := byID[id]
		if chromeParts[n.role] {
			out[id] = true
			continue
		}
		// Walk up. Bounded by the node count so a cycle in a malformed tree cannot hang.
		cur, steps := n.parent, 0
		for cur != "" && steps < len(byID)+1 {
			p, ok := byID[cur]
			if !ok {
				break // an orphan is content; see above
			}
			if chromeParts[p.role] {
				out[id] = true
				break
			}
			cur, steps = p.parent, steps+1
		}
	}
	return out
}

// IsChromePart reports whether a role is itself a window-frame part.
//
// Exported for diagnostics that want to say WHY something was attributed, never as a substitute
// for ChromeIn: a button is not chrome, and a button inside a scroll bar is.
func IsChromePart(r directorapi.ElementRole) bool { return chromeParts[r] }
