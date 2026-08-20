package directorapi

import "time"

// WorldConfidence describes how good a basis a snapshot is for acting, across five
// independent dimensions.
//
// It replaces a single aggregate number, which live testing showed to be actively
// misleading. A Discord window reported eight high-quality accessibility
// observations — every one a genuine, confidently observed element — and scored
// exactly as well as a Notepad window with twenty-one usable controls. It was all
// anonymous panes. Nothing in it could be named, and nothing could be clicked.
//
// The failure was structural, not a bad constant: averaging collapses "I saw
// clearly" and "I saw enough to act" into one number, and then lets the first
// compensate for the second. These are different questions with different answers,
// and the policy engine needs them separately:
//
//   - ObservationQuality — how trustworthy were the sources that reported?
//   - Coverage           — did we see INTO the application, or only its shell?
//   - Actionability      — of what can be acted on, how much can we actually reach?
//   - IdentityDurability — would these elements still be recognisable later?
//   - Freshness          — is this snapshot still describing the present?
//
// High ObservationQuality with low Coverage is the browser case: perfectly reliable
// reporting about the tab strip, and total blindness to the page. High Coverage with
// zero Actionability is a document viewer: fully described, nothing to press.
// Neither is a licence to act, and neither can be rescued by the other.
type WorldConfidence struct {
	// ObservationQuality is how much the sources behind this snapshot are worth,
	// 0..1. Structured sources score high; a world guessed from pixels does not.
	ObservationQuality float64 `json:"observation_quality"`

	// Coverage is how completely the application was observed, 0..1. It falls when a
	// walk was truncated, when an expected source did not report, and — the case
	// that matters most in practice — when the tree is all containers and no
	// content, which is what an application that has not enabled accessibility
	// looks like from outside.
	Coverage float64 `json:"coverage"`

	// Actionability is the share of INTERACTIVE elements that can actually be
	// targeted: named, enabled, and on screen. It is a share rather than a count so
	// a two-button dialog scores as highly as a full toolbar — being small is not
	// the same as being unusable. Zero means there is nothing here to act on.
	Actionability float64 `json:"actionability"`

	// IdentityDurability is the share of elements that would still be recognisable
	// after the UI is rebuilt — those with a unique application-authored id, or a
	// label unique enough to match on. It bounds how much a later "do that again"
	// can be trusted, and it is deliberately NOT part of Overall: a low value should
	// not block acting now, only referring back later.
	IdentityDurability float64 `json:"identity_durability"`

	// Freshness is how much of this snapshot's currency remains, 0..1. A snapshot is
	// a claim about a moving target; by the time it is acted on the screen may have
	// moved on.
	Freshness float64 `json:"freshness"`
}

// staleAfter is the age at which a snapshot retains none of its currency.
//
// Two seconds because that is roughly the scale on which desktop UIs change without
// anybody touching them — a page finishes loading, a notification slides in, a
// progress dialog closes itself. Beyond it, the Director should re-observe rather
// than act on a memory.
const staleAfter = 2 * time.Second

// FreshnessOf maps a snapshot's age onto the Freshness dimension: 1 when just taken,
// falling linearly to 0 at staleAfter.
func FreshnessOf(age time.Duration) float64 {
	if age <= 0 {
		return 1
	}
	if age >= staleAfter {
		return 0
	}
	return 1 - float64(age)/float64(staleAfter)
}

// Overall is a single summary for logging and display.
//
// Quality and coverage combine as a MINIMUM, not an average: a snapshot is only as
// good a basis for acting as its weakest necessary condition, and averaging is what
// let excellent observation quality hide the fact that there was nothing to observe.
//
// Actionability enters as a GATE plus a modifier rather than as a third term in the
// minimum, and that distinction was forced by live measurement. A Chrome window with
// its page content exposed reported 2248 elements, 128 of them named, reachable
// controls — plainly workable — but an actionability SHARE of only 0.17, because a
// web page is full of list items and links whose accessible name lives on a child
// node. Feeding that share into the minimum scored a thoroughly usable window at
// 0.17, which is as wrong in one direction as Discord's old 0.90 was in the other.
//
// What actually matters for "can I act here?" is whether there is anything
// addressable at all — a question Blind() answers — with the share as a secondary
// influence on how comfortable to be about it.
//
// Policy does not call this. Policy reads the dimensions, because "this world is
// weak" and "this world is weak in the specific way that makes your plan unsafe"
// are different findings and only the second produces a useful explanation.
func (c WorldConfidence) Overall() float64 {
	if c.Blind() {
		return 0
	}
	base := min2(c.ObservationQuality, c.Coverage)
	return base * (0.5 + 0.5*clamp01(c.Actionability)) * c.Freshness
}

// Blind reports whether nothing in this world can be acted on. Distinct from a
// low score: it is the difference between "I am not sure what I am looking at" and
// "there is nothing here to press".
func (c WorldConfidence) Blind() bool { return c.Actionability <= 0 }

// Shallow reports whether the world looks like an application whose interior was
// never exposed — high-quality reporting about a shell with nothing inside it.
// This is the Electron and browser signature, and it is the case where "I could not
// find the Save button" must never be reported as "there is no Save button".
func (c WorldConfidence) Shallow() bool {
	return c.Coverage < shallowCoverage && c.ObservationQuality >= trustworthyQuality
}

// Stale reports whether the snapshot has aged out of usefulness.
func (c WorldConfidence) Stale() bool { return c.Freshness <= 0 }

// Thresholds separating a usable world from one that needs another source or a
// fresh look. Derived from live measurement rather than chosen a priori: Discord
// sits near zero coverage, Chrome around 0.55 with its page content missing
// entirely, and readable desktop applications above 0.6.
const (
	shallowCoverage    = 0.35
	trustworthyQuality = 0.75
)

func min2(a, b float64) float64 {
	if b < a {
		a = b
	}
	return clamp01(a)
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
