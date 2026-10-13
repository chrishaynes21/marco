package observe

import (
	"sort"
	"strings"
)

// Watching what the player did, without watching what the player typed.
//
// # Why this exists
//
// Screen-state segmentation can already say that the composition changed and came back:
// `state_1 → state_2 ×4`. It cannot say what made it change. Correlating those transitions with
// the player's own navigation is what turns a list of screens into a graph with edges, and a
// graph with edges is the first thing that could be learned from rather than merely reported.
//
// # Why the vocabulary is closed and has no string in it
//
// A keyboard observer is the highest-privacy surface in the system, and a promise not to record
// text is worth less than an inability to. There is NO free-form field on InputEvent — not a
// key name, not a label, not a note. The intent vocabulary is a closed set of navigation
// meanings, so the type cannot express "the user typed p" whatever a future caller wants, and a
// platform layer that wanted to leak a character would have to add a field to do it.
//
// This mirrors the two-stage structural classifier the vision path uses: decide by ROLE what
// may cross the boundary, and let the shape of the type enforce the decision rather than the
// diligence of each caller.
//
// # What this is not
//
// Not execution, and not a step toward it. Observing that the player pressed "down" gives the
// Director no way to press it — see [[ADR-010-passive-observation-cannot-execute]], which this
// does not weaken: it adds an observation path, and observation paths are what that ADR is
// written to protect. Nothing here reaches World State, fusion, planning or policy.

// NavIntent is a navigation meaning, from a closed vocabulary.
//
// Meanings rather than keys, on purpose. "The player moved down" is what a state transition can
// be correlated with; which physical key produced it is a binding detail that differs per game,
// per controller and per player, and recording it would be recording the keyboard.
type NavIntent string

const (
	NavUp      NavIntent = "up"
	NavDown    NavIntent = "down"
	NavLeft    NavIntent = "left"
	NavRight   NavIntent = "right"
	NavConfirm NavIntent = "confirm"
	NavBack    NavIntent = "back"
	// NavPause is the intent that opens or closes an application's own menu.
	NavPause NavIntent = "pause"
	// NavPoint is a pointer press. Its position rides in PointerAt, normalised and
	// window-relative like every other coordinate here.
	NavPoint NavIntent = "point"
)

var navIntents = []NavIntent{
	NavUp, NavDown, NavLeft, NavRight, NavConfirm, NavBack, NavPause, NavPoint,
}

// Known reports whether an intent is in the closed vocabulary.
//
// Everything that enters is checked against this. An unrecognised intent is DROPPED rather
// than carried: a platform layer that invented a word would otherwise have found a way to put
// arbitrary text into a privacy-bounded record.
func (n NavIntent) Known() bool {
	for _, k := range navIntents {
		if k == n {
			return true
		}
	}
	return false
}

// NavIntents returns the vocabulary.
func NavIntents() []NavIntent { return append([]NavIntent{}, navIntents...) }

// NavDevice is the kind of hardware an observation came from, from a closed vocabulary.
//
// # Why provenance has to travel
//
// Because silence is the answer this record gives most often, and silence means nothing
// without knowing what was listening. A live four-minute game session reported `received: 0,
// unavailable: ""` — "navigation observation is working and nobody pressed anything" — while
// the only producer wired was a keyboard and mouse hook and the player's hands were on a
// controller. The sentence was not a lie about the keyboard; it was a claim about the PLAYER
// that nothing in the system was entitled to make.
//
// So a device is on the event, and the devices that were being watched are on the stats, and
// the question "can this session's quiet be read as nobody pressing anything" is answerable
// rather than assumed. See DeviceObservation.
//
// # Why it is three words and not a device name
//
// The same reason NavIntent is a meaning rather than a key. "A controller" is what a
// correlation needs; which pad, which vendor, which slot are binding details, and a field that
// could hold a product string is a field that will eventually hold something else. Checked on
// admission like every other vocabulary here.
type NavDevice string

const (
	DeviceKeyboard   NavDevice = "keyboard"
	DeviceMouse      NavDevice = "mouse"
	DeviceController NavDevice = "controller"
)

var navDevices = []NavDevice{DeviceKeyboard, DeviceMouse, DeviceController}

// Known reports whether a device is in the closed vocabulary.
func (d NavDevice) Known() bool {
	for _, k := range navDevices {
		if k == d {
			return true
		}
	}
	return false
}

// NavDevices returns the vocabulary.
func NavDevices() []NavDevice { return append([]NavDevice{}, navDevices...) }

// PointerAt is a normalised, window-relative pointer position.
type PointerAt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SemanticTarget is what a placed input event was aimed AT, when the evidence at the time
// identified it.
//
// # The enrichment, and the boundary it deliberately crosses
//
// A click at (0.42, 0.55) is evidence that the person clicked somewhere; a click on the
// list item called "Mouse" is evidence of what they MEANT. Resolving the second from the
// first is what turns a mouse-driven demonstration from an unlearnable coordinate into a
// semantic action — and it is an ENRICHMENT: the raw event survives whether or not this was
// ever filled in, and a target is never a prerequisite for capture.
//
// The Label is the one place observed interface text rides on an input event, and it is
// admitted under exactly two licences, both enforced where the evidence is pushed (the
// composition root) rather than promised here:
//
//   - a role the canonical plaintext allowlist already vouches for
//     (directorapi.ElementRole.NameablePlaintext) — a button's name is a fact about the
//     interface;
//   - during an explicit Learn demonstration, the control the person actually activated —
//     the same shape as [[ADR-047]]: an explicit human semantic event licenses retaining
//     the one identity it is about, and nothing else on the screen.
//
// Either way the text passes the standing shape filter before it may cross. A target that
// failed both keeps its Role and carries no Label, which is still more than a coordinate.
type SemanticTarget struct {
	// Role is the control's kind, shape-checked like every structural role.
	Role string `json:"role,omitempty"`
	// Label is the control's own name, when admitted; empty when withheld.
	Label string `json:"label,omitempty"`
}

// Named reports whether the target can be said out loud.
func (t SemanticTarget) Named() bool { return t.Label != "" }

// InputEvent is one observed navigation action.
//
// Deliberately small, and with one deliberate exception to "no string": Target may carry a
// control's admitted name. See SemanticTarget for the two licences and the shape filter —
// widening them is a privacy change and must be reviewed as one.
type InputEvent struct {
	Intent NavIntent `json:"intent"`
	// AtMS is milliseconds since the session began. Relative, so a record cannot be
	// correlated with anything outside the session it came from.
	AtMS int64 `json:"at_ms"`
	// Where is the pointer position for NavPoint, and zero for every other intent.
	Where PointerAt `json:"where,omitzero"`
	// Conditional says this intent came from a key that is only navigation in context.
	//
	// W/A/S/D and Space move a selection in a menu and move the player during gameplay.
	// They are admitted only while the screen looked like a set of choices, which is a
	// JUDGEMENT rather than an observation — the screen was assessed up to one inference
	// ago, and the player may have left. So the doubt travels with the evidence instead of
	// being resolved silently at the boundary.
	//
	// It is a bool, and that is the whole extent of what it adds. It says nothing about
	// WHICH key was pressed — `up` from an arrow and `up` from W are indistinguishable
	// here, which is the point: the privacy boundary is unchanged and only the strength of
	// the claim is qualified. Admitting this field required deliberately widening the type
	// allowlist in TestRawKeyIdentityCannotCrossTheBoundary; see the note there.
	Conditional bool `json:"conditional,omitempty"`
	// Target is what the event was aimed at, when the evidence at the time identified it.
	// Nil is the ordinary case and never a failure of capture. See SemanticTarget for what
	// may cross here and under which licences; admitting it widened the type allowlist in
	// TestRawKeyIdentityCannotCrossTheBoundary, deliberately and reviewed as such.
	Target *SemanticTarget `json:"target,omitempty"`
	// Device is the hardware this observation came from, empty when the producer did not
	// say. See NavDevice: a closed vocabulary of three words, checked on admission.
	//
	// Empty is not a defect and not a fourth device — it is a producer that did not state
	// one, and it is treated as unattributed rather than guessed.
	Device NavDevice `json:"device,omitempty"`
}

// admissibleInputs filters a batch down to the closed vocabulary.
//
// Bounded as well as filtered. A hook stream can deliver thousands of events between two
// inferences, and a correlation only needs to know WHICH intents preceded a change — retaining
// every repeat of a held key would make a session's memory a function of how hard somebody was
// holding down.
func admissibleInputs(in []InputEvent) []InputEvent {
	if len(in) == 0 {
		return nil
	}
	out := make([]InputEvent, 0, len(in))
	for _, e := range in {
		if !e.Intent.Known() {
			continue
		}
		if e.Intent != NavPoint {
			// Positions belong to pointer presses only. Zeroed here rather than trusted,
			// so a caller cannot smuggle a coordinate in on a keyboard intent.
			e.Where = PointerAt{}
		}
		// Admission, not trust, for the target too: a role that is not shaped like a role
		// is text in the wrong field, and a label past the bound is a paragraph wearing a
		// name's clothes. Dropping the TARGET, never the event — enrichment failing is not
		// capture failing.
		if e.Target != nil {
			t := *e.Target
			if !roleShaped(t.Role) || len([]rune(t.Label)) > MaxTargetLabelLength {
				e.Target = nil
			} else {
				e.Target = &t
			}
		}
		// The device word, admitted the same way: NavDevice's underlying type is a string,
		// so a platform layer could construct one holding anything. An unrecognised word is
		// CLEARED and the event kept — provenance is an attribution and losing it is not
		// losing the observation, exactly as with Target above.
		if !e.Device.Known() {
			e.Device = ""
		}
		out = append(out, e)
		if len(out) >= MaxBufferedInputs {
			break
		}
	}
	return out
}

// MergeInputs combines two producers' observations into one time-ordered stream.
//
// # Why the merge is here and not in either producer
//
// Because neither producer can do it: a keyboard hook knows nothing about a controller poller
// and the reverse, and a correlation asks one question — what did the player do before this
// changed — whose answer spans every device they touched. Putting the merge in the consumer's
// own package is also what keeps the ORDER a property of this type rather than of whichever
// producer happened to be drained second.
//
// Stable on equal timestamps, so two devices reporting the same millisecond keep the order
// they were drained in rather than being shuffled by the sort.
func MergeInputs(a, b []InputEvent) []InputEvent {
	switch {
	case len(a) == 0:
		return b
	case len(b) == 0:
		return a
	}
	out := make([]InputEvent, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].AtMS < out[j].AtMS })
	return out
}

// AdmissibleInputsForTest exposes the admission filter, which is where the closed vocabularies
// on an input event are actually enforced. Exported for the tests that assert the enforcement
// rather than the declaration: a typed field whose words are never checked is a convention.
func AdmissibleInputsForTest(in []InputEvent) []InputEvent { return admissibleInputs(in) }

// MaxTargetLabelLength bounds a semantic target's label — generous for a control's name and
// far too small for captured content, like MaxScreenNameLength.
const MaxTargetLabelLength = 60

// MaxBufferedInputs bounds the events retained between two valid inferences.
const MaxBufferedInputs = 64

// MaxIntentsPerTransition bounds the per-transition correlation table.
const MaxIntentsPerTransition = 8

// IgnoreReason says why a physical event produced no navigation intent.
//
// A closed vocabulary, and it lives HERE rather than in the platform adapter that produces it,
// because it is a thing that CROSSES the boundary into a report. The consumer defines what may
// arrive; a producer that wanted to say something else would have to add a constant to this
// file, which is a reviewable act rather than a free-form string appearing in a diagnostic.
//
// None of these names a key. "Why was that ignored" has to be answerable without a diagnostic
// that reintroduces exactly the identity the boundary exists to discard.
type IgnoreReason string

const (
	// IgnoreUnsupported is a key with no navigation meaning — the overwhelming majority,
	// including every character key.
	IgnoreUnsupported IgnoreReason = "unsupported_navigation"
	// IgnoreAmbiguous is a key that navigates in menus and moves the player in gameplay.
	// Counted so the refusal is visible rather than silent.
	IgnoreAmbiguous IgnoreReason = "ambiguous_gameplay_key"
	// IgnoreRepeat is auto-repeat while a key is held.
	IgnoreRepeat IgnoreReason = "repeat"
	// IgnoreNoSession is navigation observed while nothing was watching.
	IgnoreNoSession IgnoreReason = "no_session"
	// IgnoreKeyRelease is the releasing edge of a keypress.
	//
	// Not navigation — emitting on both edges would double every intent — but it IS an
	// event the producer received, and until 2026-10-12 it was the one path through the
	// classifier that incremented `Received` and then returned without landing in any
	// subset of it. The first live observation is what found it: eleven events received,
	// eight classified, one ignored, and two with no account at all. Two key releases.
	//
	// A total that does not add up is worse than no total, because a reader who checks it
	// once and finds a gap stops checking. So the quiet edge gets a name.
	IgnoreKeyRelease IgnoreReason = "key_release"
	// IgnoreUnplaceablePointer is a pointer press Marco could not place inside the window
	// it was watching.
	//
	// A pointer press means nothing without a position, and a position means nothing
	// without the frame it is relative to. Both cases land here: no recent window bounds to
	// normalise against, and a press that landed outside the window. Counted rather than
	// admitted with a zero, because a press at the origin and a press nobody could place
	// are different things and only one of them happened at the origin.
	IgnoreUnplaceablePointer IgnoreReason = "unplaceable_pointer"
	// IgnoreMarcoOwned is input the person aimed at MARCO — its own control surface —
	// while a session was watching something else.
	//
	// # Why ownership is a capture-time question
	//
	// Pressing "Start Learning" is a click. So is pressing Stop, and so is clicking Marco to
	// see what it thinks. Every one of them is a real physical press that the hook sees, and
	// none of them is part of the task the person is demonstrating: they are how the person
	// OPERATES Marco, not something Marco is being shown.
	//
	// Geometry nearly hides this and does not solve it. A press outside the watched window's
	// rectangle is already refused as unplaceable, which covers a control panel sitting
	// beside the application — but Marco's overlay is a full-screen surface lying OVER the
	// watched window, so a press on it is inside the rectangle by construction and would be
	// admitted as though the person had clicked the application underneath.
	//
	// The alternative — discarding what was captured when the lifecycle buttons are pressed —
	// is worse than useless: it throws away real evidence to compensate for having admitted
	// evidence that was never real. Ownership is decided when the press is classified, from
	// context pushed by the same validated cycle every other piece of evidence is scoped to.
	IgnoreMarcoOwned IgnoreReason = "marco_owned"
)

var ignoreReasons = []IgnoreReason{
	IgnoreUnsupported, IgnoreAmbiguous, IgnoreRepeat, IgnoreNoSession,
	IgnoreUnplaceablePointer, IgnoreMarcoOwned, IgnoreKeyRelease,
}

// Known reports whether a reason is in the closed vocabulary.
func (r IgnoreReason) Known() bool {
	for _, k := range ignoreReasons {
		if k == r {
			return true
		}
	}
	return false
}

// InputStats is what the navigation producer did, in semantic terms only.
//
// Counts and reasons, never keys. It exists because a correlation that reported no edges has
// two very different explanations — the player pressed nothing, or the producer was not running
// — and a report that cannot tell them apart invites the reading this project has already made
// twice: infrastructure silence read as behaviour.
//
// Cumulative and monotone, so a snapshot taken on any slot supersedes the one before it.
type InputStats struct {
	// Received is every physical event the producer was offered.
	Received int `json:"received"`
	// Classified is how many became an intent inside an active session.
	Classified int `json:"classified"`
	// Ignored counts the rest, by reason.
	Ignored map[IgnoreReason]int `json:"ignored,omitempty"`
	// Conditional counts the classified intents that were admitted only because the screen
	// looked like a set of choices at the time.
	//
	// A subset of Classified, reported beside it so a reader can see how much of a session's
	// navigation rests on a judgement rather than on an unambiguous key.
	Conditional int `json:"conditional,omitempty"`
	// Dropped counts events the bounded hook queue refused. Non-zero means the classifier
	// fell behind; it is a diagnostic about Marco and never about the user.
	//
	// Reported rather than hidden because the alternative to dropping is blocking a
	// low-level keyboard hook, which is how Windows silently unhooks it.
	Dropped int `json:"dropped_buffer_full"`
	// PointerDeclined is BUTTON presses the platform callback saw and would not offer —
	// a middle click, a side button, a wheel notch — and PointerInjected those it
	// discarded as not coming from a hand.
	//
	// # Why these are not folded into Ignored
	//
	// Because they happen BEFORE the queue, on the callback itself, so `Received` never
	// counted them and no refusal on this side could have. They are the difference
	// between "the producer lost your presses" and "those presses were never
	// navigation", and until they existed a recording had no account of that difference
	// at all — which is the one question a person asks about a demonstration they made
	// with their own hand: why is this fewer clicks than I made?
	//
	// A mouse MOVE is in neither, and the callback has no branch that sees one. A count
	// of moves is a record of a hand moving.
	PointerDeclined int `json:"pointer_declined,omitempty"`
	PointerInjected int `json:"pointer_injected,omitempty"`
	// Unavailable explains why no navigation can be observed at all, empty when it can.
	Unavailable string `json:"unavailable,omitempty"`
	// ControlsOffered is how many actionable controls the producer was last given to
	// resolve a press against, and PointerUnresolved how many placed presses matched none
	// of them.
	//
	// # Why these are counted separately from the refusals above
	//
	// A press that was PLACED and then resolved to nothing is not ignored — it is admitted,
	// it becomes evidence, and it is exactly the case that decides whether a click can be
	// learned. Until these existed the silence had two causes and neither was nameable:
	// nothing was on offer, or the press landed somewhere nothing claimed. Live, a
	// demonstration arrived complete and was refused `unresolved_pointer_target` with no way
	// to tell which had happened — the same shape of lost diagnosis this subsystem has now
	// hit four times.
	ControlsOffered   int `json:"controls_offered,omitempty"`
	PointerUnresolved int `json:"pointer_unresolved,omitempty"`
	// PointerUnnamed is presses that DID land on a control whose name was withheld — the
	// label gate working, not a failure to find one. The ordinary case for passive
	// observation, where only the plaintext role allowlist admits a name; an explicit Learn
	// licence is what widens it to the control a person activated.
	PointerUnnamed int `json:"pointer_unnamed,omitempty"`
	// PointerResolved is placed presses that DID name a control. The positive half, so a
	// reader can tell "resolution never works here" from "it worked and this one missed".
	PointerResolved int `json:"pointer_resolved,omitempty"`
	// PointerAttributions is how the most recent presses were resolved, one record each.
	//
	// # Why the tallies above are not enough
	//
	// They say how often resolution succeeded and never what it decided. When a click on a
	// perceived `menu "Library"` was attributed to the enclosing window instead, the tallies
	// read `0 landed on nothing` — technically true, and it took a code trace to discover
	// that the thing the person actually pressed had never been a candidate. Four diagnoses
	// in this subsystem have now been reconstructed from aggregates and one of them was
	// wrong. A reader should be able to see one press and its candidates.
	//
	// Transient diagnostic evidence. Bounded, session-local, written to nothing durable, and
	// carrying no absolute desktop coordinate: the position is the same window-relative one
	// the event itself already carries, and a candidate's SIZE is what explains why the
	// smallest containing control won. See PointerAttribution.
	PointerAttributions []PointerAttribution `json:"pointer_attributions,omitempty"`
	// VisualLease says what became of still-current visual evidence on this acquisition:
	// whether it was renewed, borrowed, or dropped and why.
	//
	// Reported because its absence is invisible. A pixel-derived control that quietly stops
	// being offered looks exactly like one that was never perceived, and the whole point of
	// the lease is that those are different — see visualLease.
	VisualLease string `json:"visual_lease,omitempty"`
	// Devices is what each producer was doing, one record per device source wired.
	//
	// THE FIELD THAT MAKES SILENCE READABLE. Every other counter here is about events that
	// happened; this one is about the devices that could have produced them, and it is the
	// only thing standing between "nobody pressed anything" and "nothing was watching the
	// thing they pressed". See DeviceObservation.
	Devices []DeviceObservation `json:"devices,omitempty"`
	// Escalation is why the last evidence-purchase decision went the way it did: the
	// structural and semantic verdicts, and whether a pass and a reading were bought.
	//
	// Reported because the decision is otherwise invisible. "No pixel controls were on
	// offer" has several causes with different fixes — the screen never settled, the repair
	// was already spent, or the reading was judged unnecessary — and they are
	// indistinguishable from the World alone.
	Escalation string `json:"escalation,omitempty"`
}

// DeviceObservation is what one device source did, and whether its silence is evidence.
//
// Four states a producer can be in, and only the last makes quiet meaningful:
//
//   - not wired at all — no record appears, and Unobserved names the device;
//   - wired and Unavailable — the platform cannot read it in this process;
//   - wired, available, and the hardware is not there — a controller nobody plugged in;
//   - wired, available, present — and NOW zero events means zero presses.
//
// The third is the one that cost a live run: a controller source reporting zero on a machine
// with no pad attached is indistinguishable from a player sitting still, and reporting it as
// the latter puts a fabricated fact ("they pressed nothing") next to a real screen change.
type DeviceObservation struct {
	Device NavDevice `json:"device"`
	// Watched says a producer for this device was running during the session.
	Watched bool `json:"watched"`
	// Unavailable explains why this device cannot be observed here, empty when it can.
	Unavailable string `json:"unavailable,omitempty"`
	// Present says the hardware was there to be read. For a hook-based device this is
	// whether the hook is installed; for a polled controller it is whether one answered.
	Present bool `json:"present"`
	// Why qualifies Present when it is false, in Marco's own words. Never a device name.
	Why string `json:"why,omitempty"`

	Received   int `json:"received"`
	Classified int `json:"classified"`
	Dropped    int `json:"dropped,omitempty"`
}

// Observable reports whether this device's silence is EVIDENCE that nothing was pressed.
func (d DeviceObservation) Observable() bool {
	return d.Watched && d.Unavailable == "" && d.Present
}

// Unobserved names every device in the vocabulary whose silence proves nothing.
//
// A device with no record at all counts: not being wired is the strongest form of not being
// observed, and it is the form that is easiest to forget, because nothing reports it.
func (s InputStats) Unobserved() []NavDevice {
	seen := make(map[NavDevice]bool, len(s.Devices))
	var out []NavDevice
	for _, d := range s.Devices {
		seen[d.Device] = true
		if !d.Observable() {
			out = append(out, d.Device)
		}
	}
	for _, d := range navDevices {
		if !seen[d] {
			out = append(out, d)
		}
	}
	return out
}

// CanAttributeSilence reports whether no observed input may be read as no input.
//
// True only when every device in the vocabulary was watched, available and present. The
// vocabulary rather than the wired set, deliberately: a session that never wired a controller
// has not established anything about controllers, and asking only the producers that exist is
// how a system comes to answer a question nobody could answer.
func (s InputStats) CanAttributeSilence() bool { return len(s.Unobserved()) == 0 }

// SilenceMeans says, in one sentence, what an absence of observed input supports.
func (s InputStats) SilenceMeans() string {
	un := s.Unobserved()
	if len(un) == 0 {
		return "every device was watched, so no observed input means no input"
	}
	words := make([]string, 0, len(un))
	for _, d := range un {
		words = append(words, string(d))
	}
	return "no observed input says nothing about the player: " +
		strings.Join(words, ", ") + " " + wasWere(len(un)) + " not observed"
}

func wasWere(n int) string {
	if n == 1 {
		return "was"
	}
	return "were"
}

// PointerAttribution is one press and what it was decided to have landed on.
type PointerAttribution struct {
	// At is the window-relative position, 0..1 on each axis — never a desktop pixel.
	At PointerAt `json:"at"`
	// Candidates are the offered controls that contained the press, SMALLEST FIRST, which
	// is the order the resolver applies. Empty means nothing on offer contained it.
	Candidates []AttributionCandidate `json:"candidates,omitempty"`
	// Chosen indexes Candidates, or -1 when nothing was attributed.
	Chosen int `json:"chosen"`
	// Why the decision went the way it did, in the resolver's own words.
	Why string `json:"why,omitempty"`
}

// AttributionCandidate is one control that contained a press.
//
// Role and the ADMITTED label only — an empty Label means the name was withheld, which is the
// label gate working rather than a failure to find one. Width and height are sizes, not
// positions: they locate nothing and they are the whole reason one candidate beat another.
type AttributionCandidate struct {
	Role  string `json:"role"`
	Label string `json:"label,omitempty"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	// Leased says this candidate reached the offer on still-current visual evidence from an
	// earlier acquisition rather than on the one that preceded the press. It changes nothing
	// about resolution and exists so a reader can tell which evidence explained a click.
	Leased bool `json:"leased,omitempty"`
}

// Observed reports whether the producer was actually running.
func (s InputStats) Observed() bool { return s.Unavailable == "" && s.Received > 0 }

// IgnoredTotal is every physical event that produced no intent.
func (s InputStats) IgnoredTotal() int {
	n := 0
	for _, v := range s.Ignored {
		n += v
	}
	return n
}

// admissibleStats filters a producer snapshot down to the closed reason vocabulary.
//
// The same treatment admissibleInputs gives events, for the same reason: IgnoreReason's
// underlying type is a string, so a platform layer could construct one holding anything.
func admissibleStats(in InputStats) InputStats {
	out := in
	out.Devices = nil
	for _, d := range in.Devices {
		// A producer that named a device outside the vocabulary has said nothing
		// admissible about any device, so the record is dropped rather than carried with
		// an unrecognised word in a reported field.
		if !d.Device.Known() {
			continue
		}
		out.Devices = append(out.Devices, d)
	}
	out.Ignored = nil
	for r, n := range in.Ignored {
		if !r.Known() {
			continue
		}
		if out.Ignored == nil {
			out.Ignored = make(map[IgnoreReason]int, len(in.Ignored))
		}
		out.Ignored[r] = n
	}
	return out
}
