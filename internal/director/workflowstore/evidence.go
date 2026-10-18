package workflowstore

// WHAT THE PERSON DID, KEPT SEPARATELY FROM WHAT MARCO MADE OF IT.
//
// # The owner's architecture, in three layers
//
//	Raw demonstration    every actual click, keypress, release, coordinate, timestamp, focus
//	                     change and associated observation. Preserved durably.
//	Learned procedure    the subset Marco interpreted as intentional and repeatable.
//	                     Supports corrections, uncertainty, names and step ordering.
//	Guarded execution    the actions Marco can ground and safely perform, with verification
//	                     and immediate cancellation.
//
// `Artifact` in this package is the middle layer and stays exactly what it was. This file is
// the first one — and it had no implementation at all. The production teaching path kept
// COUNTERS: `received: 11, classified: 6, key_release: 3`. A count cannot be put in order, so
// a session that recorded eleven events could not say what any of them was, when it happened,
// or which of the person's own clicks Marco had failed to understand.
//
//	Marco remembers what it thinks the user taught it better than it remembers exactly what
//	the user did. That needs to change before we invest much more in interpretation.
//
// # Why it is a companion record and not a field on the artifact
//
// Because the two have different jobs and one of them must never grow a way to act.
// [[ADR-181-a-demonstration-marco-can-keep]] keeps a press coordinate out of the artifact, and
// [[ADR-183-remembering-recognising-and-being-allowed-to-act]] keeps the artifact unable to
// authorise anything. A coordinate in the executable interpretation would be a replay
// instruction one field away from being followed; a coordinate HERE is what it has always been
// claimed to be — evidence of where somebody acted, on a record nothing executes.
//
//	A demonstrated coordinate is authoritative evidence of where the person acted. It must not
//	become an unconditional coordinate replay instruction.
//
// So the artifact is unchanged, the evidence sits beside it under the same name and the same
// `SavedAt`, and the link is checked rather than assumed — see `Evidence.Describes`.
//
// # What is not here
//
// No pixels. No frames, no screenshots, no window titles, no typed text, no key identity. The
// inputs are `observe.InputEvent`, whose whole design is that it CANNOT carry a character —
// and the quiet edges that make a hold distinguishable from a tap carry their press's own
// closed intent and nothing more. See observe/input.go, which is the privacy boundary, and
// `observe.InputEdge`, admitted on 2026-10-18 with the argument recorded in both boundary
// tests.

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// EvidenceVersion is the companion record's own version, independent of the artifact's.
//
// Independent on purpose: the raw record can gain a field without touching the executable
// format, which is the whole reason for keeping them apart. A reader that cannot vouch for a
// version refuses the file rather than half-reading it, exactly as `Unmarshal` does.
const EvidenceVersion = 1

// MaxEvidenceMoments bounds how many looks of a session are kept.
//
// A bound rather than a session: the owner asked for the recording to be bounded, and a
// demonstration is a minute of somebody's hands rather than an afternoon. At ten looks a second
// this is five minutes of looks THAT CARRIED INPUT — looks where nothing happened are not
// moments and are never stored.
const MaxEvidenceMoments = 3000

// Evidence is the raw demonstration: what was physically done, in the order it was done.
type Evidence struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	// SavedAt is the artifact's own SavedAt, to the microsecond, and is the LINK.
	//
	// Not a separate clock. A record that stamped itself would drift from the artifact it
	// describes, and the one question a reader asks of a companion file is whether it
	// belongs to the thing beside it. See Describes.
	SavedAt time.Time `json:"saved_at"`
	// Window is what was being watched and how it was named, as the person named it.
	Window WindowWatched `json:"window"`
	// Moments is every look that carried an input, in order, each naming the look and the
	// screen that was standing. The look is the reference to the visual observation.
	Moments []Moment `json:"moments,omitempty"`
	// Presses is EVERY pointer press with its coordinate, its look and what the proposer
	// had found there — including the presses Marco understood, which used to be the only
	// ones it threw away.
	Presses []DemonstratedPress `json:"presses,omitempty"`
	// Counts is the producer's own account of what it saw, kept beside the events.
	//
	// Both, not one. The events are what was admitted; the counts are what was refused and
	// why, and a record with only the first cannot answer the question a person asks about
	// a demonstration they made with their own hand: why is this fewer clicks than I made?
	Counts observe.InputStats `json:"counts"`
	// Looks is how many looks the session took, so a moment's look index has a denominator.
	Looks int `json:"looks"`
	// Truncated is how many moments were dropped at the write because the record hit its
	// bound. Zero for every ordinary demonstration.
	//
	// Stored because a truncated record that does not say so is worse than a short one:
	// the gap reads as a person who stopped doing anything. See MaxEvidenceMoments.
	Truncated int `json:"truncated,omitempty"`
}

// WindowWatched is which window a demonstration was made against.
//
// The SELECTOR as the person typed it, and the window's generation. Not the live title: a game
// window's title is the commonest place an account name appears, which is why
// `capture-game-session` refuses to record one. A person who typed `--window-title Ark` has
// already chosen those words.
type WindowWatched struct {
	Application string `json:"application,omitempty"`
	Title       string `json:"title,omitempty"`
	// Generations is how many distinct window identities the session ran against.
	//
	// More than one means the window was closed and reopened, or replaced, DURING the
	// demonstration — which invalidates comparing geometry across the join and is exactly
	// the kind of thing a record is for. Reported, never repaired.
	//
	// A count and not the identities themselves: a generation number means nothing
	// outside the process that minted it, and a field holding one would be a session
	// identity in a durable file for the second time in this record's short life.
	Generations int `json:"generations,omitempty"`
}

// DemonstratedPress is one pointer press, as the person made it.
//
// # Why this is not the click tier's own observation type
//
// Because a session-local visual identity may not be written to a durable file, and
// `TestAVisualScreenCannotReachDurableMemory` is right to say so: a target identified by a
// rectangle on a screen identified by an arrangement is two weak identities stacked, and
// writing `visual_1` into a file invites a later reader to match on it across sessions. That is
// the exact failure [[ADR-181-a-demonstration-marco-can-keep]] exists to prevent.
//
// So this carries the ARTIFACT's own labels — `screen_a`, `control_1` — which point at real
// entries in the routine beside it, and the LOOK, which is the precise reference into the
// observations and is stronger than a screen name anyway. A press made on a screen the saved
// route does not contain has an empty label and still has its look, its position and its
// candidate count.
type DemonstratedPress struct {
	// Look is the look it was folded on, and the reference into the visual evidence.
	Look int `json:"look"`
	// At is where it landed, normalised and window-relative.
	//
	// DEMONSTRATION EVIDENCE. Nothing replays this. See evidence.go's own header and
	// [[ADR-235-three-records-and-only-one-of-them-may-act]].
	At observe.PointerAt `json:"at"`
	// Screen is the routine's label for the screen that was standing, empty when the
	// saved route does not contain it. Empty is a real answer, not a capture failure.
	Screen string `json:"screen,omitempty"`
	// Control is the routine's label for what the press was taken to be, empty when the
	// press landed on nothing the proposer found — which is kept rather than discarded,
	// and is the subject of [[ADR-234-the-person-does-not-have-to-prove-the-click]].
	Control string `json:"control,omitempty"`
	// Containing is how many proposed regions contained the press. Zero is the press
	// nobody could segment; more than one is the press that was inside a control AND its
	// panel, which is why Ambiguous is beside it rather than derived from it.
	Containing int  `json:"containing"`
	Ambiguous  bool `json:"ambiguous,omitempty"`
}

// Moment is one look that carried input, and what was standing at the time.
type Moment struct {
	// Look is the look index, which is the reference INTO the visual observations: the
	// screen below, the arrangement in the artifact, and the proposals the presses
	// landed in all belong to this look.
	Look int `json:"look"`
	// Screen is the routine's label for the screen that was standing when these inputs
	// arrived, empty when none had settled or the saved route does not contain it.
	// Unknown is a real answer and is not a failure of capture.
	Screen string `json:"screen,omitempty"`
	// AtMS is when the look was taken, relative to the session's start.
	AtMS int64 `json:"at_ms"`
	// Front says the watched application was the window IN FRONT when these inputs
	// arrived.
	//
	// False is not a defect and nothing is discarded for it: a press made while
	// something covered the target is a real press that may have landed on the covering
	// window, and a window capture is not occlusion-safe — so the look it arrived on can
	// hold somebody else's pixels under the target's own name. The owner's list names
	// focus changes, and this is the form the evidence takes: a property of each moment,
	// which is what the loop observes, rather than an event of its own, which it does
	// not.
	Front bool `json:"front"`
	// Inputs is what arrived on this look, in time order: presses and the quiet edges
	// of presses together, merged by `observe.MergeInputs`.
	Inputs []observe.InputEvent `json:"inputs"`
}

// Describes reports whether this evidence belongs to that artifact.
//
// Name and instant, both. A name alone would match a routine that has since been
// re-demonstrated, and reading the old session's hands as the new session's evidence is the
// one way a companion record could lie about what somebody did.
func (e Evidence) Describes(a Artifact) bool {
	return e.Name == a.Name && e.SavedAt.Equal(a.SavedAt)
}

// Inputs is every event of every moment, in order, flattened.
//
// For the readings that do not care which look an event arrived on — a hold spans two looks
// whenever it is longer than a look, and a chord can too.
func (e Evidence) Inputs() []observe.InputEvent {
	n := 0
	for _, m := range e.Moments {
		n += len(m.Inputs)
	}
	out := make([]observe.InputEvent, 0, n)
	for _, m := range e.Moments {
		out = append(out, m.Inputs...)
	}
	return out
}

// Keystroke is one physical press of one key, assembled from its edges.
//
// A READING of the record and never stored: the edges are what was observed, and this is what
// they mean. Storing it would let the summary drift from the evidence it summarises, which is
// the defect `ClickEvidence.MaybeOneControl` is derived to avoid.
type Keystroke struct {
	Intent observe.NavIntent `json:"intent"`
	// AtMS is when it went down, and HeldMS how long until it came up. HeldMS is -1 when
	// no release was observed — a key held past the end of the session, which is a real
	// outcome and must not read as a zero-length press.
	AtMS   int64 `json:"at_ms"`
	HeldMS int64 `json:"held_ms"`
	// Repeats is how many auto-repeats arrived while it was down. Non-zero is somebody
	// leaning on a key, which is a different act from tapping it.
	Repeats int `json:"repeats,omitempty"`
	// Conditional carries the press's own doubt: admitted only because the screen looked
	// like a set of choices at the time.
	Conditional bool `json:"conditional,omitempty"`
	// Overlaps is the other keystrokes of this session whose intervals overlap this one,
	// by index. A chord, observed — and NOT a chord, concluded: two intents held together
	// are what a chord looks like and are also what a slow hand looks like. The ambiguity
	// is preserved rather than resolved here.
	Overlaps []int `json:"overlaps,omitempty"`
}

// Keystrokes assembles the key events into presses with their durations.
//
// One keystroke per press, in the order pressed. A release is matched to the most recent
// unclosed press of the same intent, which is the only matching the record supports — two
// `down` keys held at once are indistinguishable by design, and the privacy boundary is the
// reason. That ambiguity is real and is left where it is rather than resolved by guessing.
func (e Evidence) Keystrokes() []Keystroke {
	var out []Keystroke
	open := map[observe.NavIntent][]int{}
	for _, ev := range e.Inputs() {
		if ev.Intent == observe.NavPoint {
			continue
		}
		switch ev.Edge {
		case observe.EdgePress:
			out = append(out, Keystroke{
				Intent: ev.Intent, AtMS: ev.AtMS, HeldMS: -1,
				Conditional: ev.Conditional,
			})
			open[ev.Intent] = append(open[ev.Intent], len(out)-1)
		case observe.EdgeRepeat:
			if i, ok := newest(open, ev.Intent); ok {
				out[i].Repeats++
			}
		case observe.EdgeRelease:
			if i, ok := newest(open, ev.Intent); ok {
				out[i].HeldMS = ev.AtMS - out[i].AtMS
				open[ev.Intent] = open[ev.Intent][:len(open[ev.Intent])-1]
			}
		}
	}
	// THE OVERLAPS, after every interval is known. An unreleased press is open until the
	// end of the record, because that is what was observed.
	end := func(k Keystroke) int64 {
		if k.HeldMS < 0 {
			return 1 << 62
		}
		return k.AtMS + k.HeldMS
	}
	for i := range out {
		for j := range out {
			if i == j {
				continue
			}
			if out[j].AtMS < end(out[i]) && out[i].AtMS < end(out[j]) {
				out[i].Overlaps = append(out[i].Overlaps, j)
			}
		}
	}
	return out
}

// newest is the most recent unclosed press of an intent.
func newest(open map[observe.NavIntent][]int, in observe.NavIntent) (int, bool) {
	s := open[in]
	if len(s) == 0 {
		return 0, false
	}
	return s[len(s)-1], true
}

// MarshalEvidence writes the record, refusing one that describes nothing.
func MarshalEvidence(e Evidence) ([]byte, error) {
	if e.Name == "" {
		return nil, fmt.Errorf("workflowstore: evidence with no routine to belong to")
	}
	e.Version = EvidenceVersion
	if len(e.Moments) > MaxEvidenceMoments {
		// THE BOUND, APPLIED AT THE WRITE and reported in the record itself: a
		// truncated record that does not say it was truncated is worse than a short
		// one, because the gap reads as a person who stopped doing anything.
		e.Truncated = len(e.Moments) - MaxEvidenceMoments
		e.Moments = e.Moments[:MaxEvidenceMoments]
	}
	return json.MarshalIndent(e, "", "  ")
}

// UnmarshalEvidence reads one back, refusing a version this build cannot vouch for.
func UnmarshalEvidence(raw []byte) (Evidence, error) {
	var e Evidence
	if err := json.Unmarshal(raw, &e); err != nil {
		return Evidence{}, fmt.Errorf("workflowstore: reading evidence: %w", err)
	}
	if e.Version != EvidenceVersion {
		return Evidence{}, fmt.Errorf(
			"workflowstore: evidence version %d, and this build reads %d",
			e.Version, EvidenceVersion)
	}
	return e, nil
}
