package observe

// What a person established by pressing something once.
//
// # The gap this closes
//
// [[Experiment-057-what-earns-a-click-on-five-interfaces]] measured the one thing no amount of
// looking supplies: which of two nested surfaces — a word's own region and the region containing
// it — is the control. Both rectangles are really there, neither is a misreading, and no
// threshold separates them without being fitted to an example.
//
// The owner's conclusion, and the whole design:
//
//	The workflow stack remembers where someone clicked. The semantic Stage remembers what they
//	clicked. Neither currently preserves both as one reusable fact.
//
// This is that fact. It joins a durable semantic target — a name, a kind and a place, written by
// `RememberTarget` — to the SURFACE a person demonstrated, and it does so without storing a
// rectangle as identity.
//
// # Why a relation and not a rectangle
//
// A coordinate is a fact about one window at one size on one monitor. What a demonstration
// actually establishes is relational and survives all three:
//
//	the press belongs to the region the words were read on       — the identity's own region
//	the press belongs to the region that CONTAINS those words     — a containing region
//
// So that is what is stored, with the nesting depth for the second case, and the surface is
// re-derived from whatever candidates exist at the moment of use. The rectangles ARE kept, in
// `SurfaceShape`, as EVIDENCE — normalised against the window, matched on by nothing, and present
// for exactly one purpose: so a later observation can CONTRADICT the record. A row that was 1.2%
// of the window and is now 48% of it resolves by relation and disagrees on evidence, and the
// disagreement is the answer.
//
// # What it may not contain, and how that is held
//
// This type is written to the file that outlives every session, so it takes the same sweep every
// other durable type takes — `TestNothingCapturedCanReachDurableMemory` walks it. In particular
// it carries NO label: the target is named by its durable subject id, which `RememberTarget`
// returns and which is a hash. The two decided durable-text exceptions are not widened by this,
// and `TestTheDurableTextExceptionsAreTheOnesThatWereDecided` would notice if they were.
//
// # And one demonstration is not a rule
//
// The owner's limit, stated as the scope of the record rather than as a warning on it:
//
//	I would keep this knowledge scoped to one target and place. One demonstration must not
//	silently teach Marco that every menu row in every application behaves identically.
//
// So a record is about ONE target in one application, found by that target's own id. Nothing here
// reaches a sibling, a role, a shape class or another application — the same discipline
// `workflowstore.Answer` keeps one tier down, where a clarification "is about `control_6` of this
// workflow and reaches nothing else".

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

// SurfaceRelation is which of the surfaces around an identity a demonstration pressed.
//
// A CLOSED vocabulary of two, because the measurement found exactly two kinds of candidate: the
// region the identifying words were read on, and a region containing it. A third value would be a
// claim no evidence supports.
type SurfaceRelation string

const (
	// RelationIdentityRegion: the press belongs to the region the words were read on.
	//
	// Often right — a toolbar button is about the size of its label — and
	// [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] is the record of it being
	// wrong. The point of storing it is that a person settled which.
	RelationIdentityRegion SurfaceRelation = "the identity's own region"
	// RelationContainingRegion: the press belongs to a region that contains the words.
	//
	// The Trackmania menu row and the Windows Settings row, both measured.
	RelationContainingRegion SurfaceRelation = "a region containing the identity"
)

// Known reports whether this is one of the two relations.
func (r SurfaceRelation) Known() bool {
	return r == RelationIdentityRegion || r == RelationContainingRegion
}

// SurfaceConfirmation is how well a demonstration was confirmed — and it cannot be asserted.
//
// # Why three, when a boolean looks like enough
//
// Because the owner named the conflation to avoid:
//
//	a screen transition attributed to a click supports that the interaction did something, but
//	it doesn't automatically prove the intended Settings page opened. For this first slice, I
//	would distinguish transition verified from goal verified rather than treating them as
//	interchangeable.
//
// A boolean would have to choose one of those to mean, and whichever it chose, the other reading
// would eventually be taken by somebody in a hurry.
//
// # Why it is a struct with no exported field
//
// Because the first version of this was a string a caller handed in, and the owner caught what
// that licenses:
//
//	`ConfirmationAttributed` must not be an assertion that an arbitrary caller can make. It
//	should be derived from a validated observation/transition result, or accepted only through a
//	narrowly controlled internal boundary. Otherwise the system can promote a demonstration to
//	learned knowledge without evidence that the demonstrated action did anything.
//
// So there is one constructor per level and no other way in — the discipline `ScreenName` already
// keeps, for the reason its note gives: passing something off as a confirmation is possible in
// the sense that any lie is possible, and it is not possible by accident. The ZERO VALUE confirms
// nothing and is not `Sufficient`, so a record built by forgetting to set this cannot aim
// anything.
type SurfaceConfirmation struct {
	level string
	// why is the evidence, in words. Required by both constructors: a confirmation with no
	// account of itself is indistinguishable from a default, and it is also what a person
	// reads when Marco says why it is about to press something its role says is not a
	// control.
	why string
}

// The three levels. Strings rather than an enum because they are written to a durable file and
// read by a person looking at it.
const (
	// confirmationObserved: a genuine press from a hand, inside the watched window, while
	// the question was open, landing on one surface and not the others.
	//
	// The weakest of the three and NOT enough to act on. A person can click the wrong
	// thing, and a click that reached nothing is indistinguishable here from one that
	// worked.
	confirmationObserved = "observed"
	// confirmationAttributed: and the screen then changed, and the existing crossing
	// correlator attributed the change to that press.
	//
	// Evidence that the interaction DID something. Not evidence that it did the right
	// thing.
	confirmationAttributed = "attributed"
	// confirmationVerified: and what the request asked for actually happened.
	//
	// NOTHING CONSTRUCTS THIS. It is named because the distinction above is the point, and
	// a vocabulary that stopped at `attributed` would make `attributed` mean this by
	// default. The day something can verify an intended outcome it gets a constructor of
	// its own, and this comment goes.
	confirmationVerified = "verified"
)

// TransitionWitness is what the crossing correlator knows about one screen change.
//
// The narrow boundary `TransitionAttributed` accepts, and its fields are exactly the ones the
// ambient crossing in `cmd/director` already produces — this is not a new observation, it is the
// shape of the one that exists. Nothing here is durable and nothing here is stored.
type TransitionWitness struct {
	// Application is the application the crossing happened in.
	Application string
	// From and To are the screens either side of it, as the crossing keyed them.
	From, To string
	// ByHuman says the crossing was attributed to a PERSON acting rather than to Marco.
	//
	// Load-bearing: a play running while watching is on moves the screen too, and offering
	// that back as something the person demonstrated is how a system comes to learn its own
	// behaviour from itself.
	ByHuman bool
	// Acts is how many human actions the crossing was attributed to.
	//
	// Exactly one is required. A crossing attributed to two actions does not single out the
	// press being asked about, and crediting it would be the same mistake as settling a
	// surface question with a press that was inside both candidates.
	Acts int
	// At is when the crossing was observed.
	At time.Time
}

// PressWasObserved is one of the two ways to make a confirmation: somebody pressed something.
//
// It confirms the press and nothing about its effect, so it is not `Sufficient` and cannot aim
// anything. It exists so that a demonstration nobody could confirm is still a thing that
// happened, with provenance, rather than being silently discarded.
func PressWasObserved(why string) (SurfaceConfirmation, error) {
	if strings.TrimSpace(why) == "" {
		return SurfaceConfirmation{}, fmt.Errorf("observe: a confirmation needs to say " +
			"what was observed")
	}
	return SurfaceConfirmation{level: confirmationObserved, why: strings.TrimSpace(why)}, nil
}

// TransitionAttributed is the other: the screen changed, and the change was attributed to it.
//
// THE ONLY WAY TO REACH A CONFIRMATION THAT CAN AIM ANYTHING, and every clause below is a way a
// witness could exist and mean nothing. A caller that wants this has to have a crossing in hand;
// a caller that merely wants to say so cannot.
func TransitionAttributed(w TransitionWitness) (SurfaceConfirmation, error) {
	switch {
	case strings.TrimSpace(w.Application) == "":
		return SurfaceConfirmation{}, fmt.Errorf("observe: a crossing with no " +
			"application is not about anything")
	case !w.ByHuman:
		return SurfaceConfirmation{}, fmt.Errorf("observe: the screen change was not " +
			"attributed to a person acting, so it is not evidence about what they " +
			"demonstrated")
	case w.Acts != 1:
		return SurfaceConfirmation{}, fmt.Errorf("observe: the crossing was attributed "+
			"to %d human action(s), so it does not single out the one being asked "+
			"about", w.Acts)
	case w.From == "" || w.To == "":
		return SurfaceConfirmation{}, fmt.Errorf("observe: a crossing needs both the " +
			"screen left and the screen arrived at")
	case w.From == w.To:
		return SurfaceConfirmation{}, fmt.Errorf("observe: the screen did not change, " +
			"so nothing confirms that the press did anything")
	case w.At.IsZero():
		return SurfaceConfirmation{}, fmt.Errorf("observe: a crossing needs to say when")
	}
	return SurfaceConfirmation{
		level: confirmationAttributed,
		why: fmt.Sprintf("the screen changed from %s to %s and the crossing was "+
			"attributed to one human action", w.From, w.To),
	}, nil
}

// Level is which of the three this is, empty for the zero value.
func (c SurfaceConfirmation) Level() string { return c.level }

// Why is the evidence, in words.
func (c SurfaceConfirmation) Why() string { return c.why }

// Stated reports whether this confirms anything at all.
func (c SurfaceConfirmation) Stated() bool { return c.level != "" && c.why != "" }

// Sufficient reports whether this is enough to ground a later press.
//
// `observed` is not. The owner's rule:
//
//	Do not persist a successful grounding rule merely because a click occurred.
//
// The zero value is not either, which is what makes a forgotten field safe rather than dangerous.
func (c SurfaceConfirmation) Sufficient() bool {
	if !c.Stated() {
		return false
	}
	return c.level == confirmationAttributed || c.level == confirmationVerified
}

// confirmationOnDisk is the serialised form.
//
// The struct's fields are unexported so that nothing can assign one, which also means nothing
// marshals them — so the two have to be written out. Both halves are kept: a level with no
// evidence is exactly the assertion this type exists to refuse, and `Stated` rejects it on the
// way back in.
type confirmationOnDisk struct {
	Level string `json:"level"`
	Why   string `json:"why"`
}

// MarshalJSON writes the level and the evidence for it.
func (c SurfaceConfirmation) MarshalJSON() ([]byte, error) {
	return json.Marshal(confirmationOnDisk{Level: c.level, Why: c.why})
}

// UnmarshalJSON reads one back, refusing a level this build does not know.
//
// # What this does and does not promise
//
// It does not make the FILE unforgeable. Somebody editing durable memory by hand can write
// `"level":"attributed"` and Marco will believe it, exactly as they could add a route or a place
// — the store trusts its own file and says so. What the constructors protect is the code path: no
// caller inside Marco can reach a usable confirmation without a crossing, which is where the
// owner's requirement bites and where a mistake would otherwise be invisible.
func (c *SurfaceConfirmation) UnmarshalJSON(b []byte) error {
	var on confirmationOnDisk
	if err := json.Unmarshal(b, &on); err != nil {
		return err
	}
	switch on.Level {
	case confirmationObserved, confirmationAttributed, confirmationVerified:
	case "":
		*c = SurfaceConfirmation{}
		return nil
	default:
		return fmt.Errorf("observe: %q is not a confirmation this build understands",
			on.Level)
	}
	*c = SurfaceConfirmation{level: on.Level, why: on.Why}
	return nil
}

// SurfaceShape is what the demonstrated surface and its identity LOOKED like, normalised.
//
// Every field is evidence and no field is a threshold. They exist so that a later observation can
// disagree — and [[Experiment-056-whether-a-row-is-one-of-a-set]] is why they are not rules: the
// sibling boundary fell exactly between a toolbar's one sibling and a browser's two, and fill and
// parts separated a background cleanly while failing on a toolbar band.
type SurfaceShape struct {
	// Surface is the demonstrated surface and Identity the region the words were read on,
	// both normalised against the window so neither is a fact about a monitor.
	Surface  Region `json:"surface"`
	Identity Region `json:"identity"`
	// Containment is how much of the identity the surface held, 0..1.
	Containment float64 `json:"containment"`
	// AreaShare is the fraction of the window the surface covered.
	AreaShare float64 `json:"area_share"`
	// Kind is the shape class the proposer assigned, from its own closed vocabulary.
	Kind string `json:"kind,omitempty"`
	// Fill is how rectangular the component was and Parts how many were merged into it.
	Fill  float64 `json:"fill,omitempty"`
	Parts int     `json:"parts,omitempty"`
	// Siblings is how many same-shaped neighbours the surface had, and DistinctNames how
	// many of them contained a different name. See Experiment-056.
	Siblings      int `json:"siblings,omitempty"`
	DistinctNames int `json:"distinct_names,omitempty"`
}

// MaxAreaShareDrift is how much of the window a demonstrated surface's area may differ by before
// the record is treated as describing something else.
//
// # Why this is a contradiction test and not a matching threshold
//
// Nothing is selected by it. The relation selects, and this only ever REFUSES: a surface chosen
// by relation whose size no longer resembles what was demonstrated is reported as contradicted,
// and the press does not happen. So the failure mode of a wrong number here is a refusal a person
// can see and answer again, never a press somewhere new.
//
// Generous on purpose — a tenth of the window is far more than a layout reflow moves and far less
// than the 1.2%-to-48% gap between a menu row and the background behind it, which is the
// confusion it exists to catch.
const MaxAreaShareDrift = 0.10

// MinContainmentNow is how much of the identity a surface must still hold.
//
// The same shape of test as above and the same direction: it refuses, it does not choose. A
// surface that no longer holds the words it was demonstrated around is not the surface those
// words sit on, whatever the relation says.
const MinContainmentNow = 0.80

// DemonstratedSurface is one durable fact: for this target, the press belongs HERE.
type DemonstratedSurface struct {
	// Application is which application this is about. Scope, and the outer half of the
	// owner's "one target in one application context".
	Application string `json:"application"`
	// Target is the durable subject id of the semantic target — what `RememberTarget`
	// returned for it.
	//
	// AN ID AND NOT A NAME. The name lives on the subject, where the two decided
	// durable-text exceptions already admit it under the Learn licence; repeating it here
	// would be a third exception, arriving as a field rather than as a decision.
	Target string `json:"target"`
	// Relation is which surface was pressed, and Depth which one when it was a containing
	// region.
	//
	// Depth counts outwards from the identity: 0 is the identity's own region, 1 the
	// tightest candidate containing it, 2 the next, and so on. Relational, so it survives a
	// resize; re-derived from whatever candidates exist at the moment of use, so it cannot
	// point at a rectangle that is no longer there.
	Relation SurfaceRelation `json:"relation"`
	Depth    int             `json:"depth"`
	// Shape is what it looked like. Evidence, never identity — see SurfaceShape.
	Shape SurfaceShape `json:"shape"`
	// Confirmed is how well this was confirmed, as three separate claims, and it cannot be
	// asserted — see SurfaceConfirmation.
	Confirmed SurfaceConfirmation `json:"confirmed"`
	// By is who demonstrated it and At when.
	//
	// An unattributed demonstration is indistinguishable from a default, which is the thing
	// every provenance in this system exists to prevent. `workflowstore.Answer` says it the
	// same way.
	By string    `json:"by"`
	At time.Time `json:"at"`
}

// Valid reports whether this record is one the store may keep, and why not.
//
// Refused rather than repaired, for the reason `workflowstore.Unmarshal` gives: a half-read
// record is worse than no record, because something downstream acts on it confidently.
func (d DemonstratedSurface) Valid() (bool, string) {
	switch {
	case strings.TrimSpace(d.Application) == "":
		return false, "a demonstrated surface needs the application it is about"
	case strings.TrimSpace(d.Target) == "":
		return false, "a demonstrated surface needs the durable target it is about"
	case !d.Relation.Known():
		return false, fmt.Sprintf("%q is not one of the two surface relations",
			d.Relation)
	case d.Relation == RelationIdentityRegion && d.Depth != 0:
		return false, "the identity's own region is at depth 0 by definition"
	case d.Relation == RelationContainingRegion && d.Depth < 1:
		return false, "a region containing the identity is at depth 1 or more"
	case !d.Confirmed.Stated():
		return false, "a demonstration that says nothing about how it was confirmed is " +
			"indistinguishable from one nobody checked"
	case strings.TrimSpace(d.By) == "":
		return false, "a demonstration nobody is recorded as having given is " +
			"indistinguishable from a default"
	case d.At.IsZero():
		return false, "a demonstrated surface needs to say when"
	}
	return true, ""
}

// Usable reports whether this record may ground a press, and why not.
//
// DERIVED on every read rather than stored as a flag, which is the discipline
// `RehearsalEvidence.VerifiedBy` established: a stored boolean goes on saying yes after the thing
// it was about has stopped being true.
func (d DemonstratedSurface) Usable() (bool, string) {
	if ok, why := d.Valid(); !ok {
		return false, why
	}
	if !d.Confirmed.Sufficient() {
		return false, fmt.Sprintf("the demonstration is %s (%s): a press was seen and "+
			"nothing confirmed that it did anything, so it is kept as evidence and "+
			"not used to aim", d.Confirmed.Level(), d.Confirmed.Why())
	}
	return true, ""
}

// Agrees reports whether a surface found NOW still resembles the one demonstrated.
//
// The contradiction test, and it runs in one direction only: it can refuse a surface the relation
// selected, and it can never select one. So every way this is wrong costs a refusal a person can
// answer, and no way it is wrong costs a press somewhere nobody pointed.
//
// Three things are asked, each with its own sentence, because a caller that gets one "no" cannot
// otherwise tell a person whether the layout moved, the control grew, or the words came loose
// from it.
func (d DemonstratedSurface) Agrees(now SurfaceShape) (bool, string) {
	if now.Surface.Width <= 0 || now.Surface.Height <= 0 {
		return false, "the surface found now has no extent"
	}
	if drift := math.Abs(now.AreaShare - d.Shape.AreaShare); drift > MaxAreaShareDrift {
		return false, fmt.Sprintf("the demonstrated surface covered %.1f%% of the "+
			"window and this one covers %.1f%%, which is too different to be the "+
			"same thing", 100*d.Shape.AreaShare, 100*now.AreaShare)
	}
	if now.Containment < MinContainmentNow {
		return false, fmt.Sprintf("this surface holds only %.0f%% of the words it is "+
			"supposed to be the control for", 100*now.Containment)
	}
	return true, ""
}

// SurfaceStore is durable storage for what a demonstration established about a surface.
//
// A separate interface from `Memory` and from `TargetStore`, for the reason `RehearsalStore` is
// separate from both: a caller that needs to ask "where does the press go" should not thereby
// gain the ability to write a subject, a judgement or a route.
type SurfaceStore interface {
	// RememberSurface keeps one demonstration's bounded evidence, refusing one it cannot
	// vouch for.
	RememberSurface(application string, d DemonstratedSurface) error
	// DemonstratedSurfaces returns what is stored for one application, in a stable order.
	DemonstratedSurfaces(application string) []DemonstratedSurface
}

// SurfaceFor is the demonstration that applies to one durable target, and why none does.
//
// The canonical lookup, so "which demonstration is about this target" has one answer. Exactly one
// or none: two records for one target is a contradiction rather than a preference, and picking the
// newer one would be this system deciding on a person's behalf which of their two answers they
// meant. The store keeps the newest by replacing, so two can only arrive through a corrupted file.
func SurfaceFor(stored []DemonstratedSurface, application, target string) (
	DemonstratedSurface, string) {

	var found []DemonstratedSurface
	for _, d := range stored {
		if !strings.EqualFold(d.Application, application) || d.Target != target {
			continue
		}
		found = append(found, d)
	}
	switch len(found) {
	case 0:
		return DemonstratedSurface{}, "nobody has demonstrated where to press this"
	case 1:
		return found[0], ""
	default:
		return DemonstratedSurface{}, fmt.Sprintf("%d demonstrations are stored for "+
			"this target and nothing here chooses between them", len(found))
	}
}
