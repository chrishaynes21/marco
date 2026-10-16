package surface

// Asking the one thing looking cannot settle, and reading the answer.
//
// # Why the question is worth asking at all
//
// [[Experiment-057-what-earns-a-click-on-five-interfaces]] ended with the evidence sufficient to
// refuse every indefensible surface and insufficient to choose between a word and the row it sits
// on. The owner's conclusion was not to look harder:
//
//	Marco should be able to ask one useful question, learn from one answer, and apply that
//	answer later without pretending the screen is unchanged.
//
// So an AMBIGUOUS hypothesis becomes a question with named options, and a person's own press is
// the answer. Nothing here presses anything, and nothing here is durable: a Question is a value
// derived from one reading of the screen, and it stops being about anything the moment that
// reading does.
//
// # The thing that nearly made this useless
//
// A person shown "is it the word or the row?" and told "click it" will click the WORD, because
// that is where the word is — and the word is inside the row, so the press is inside both
// options and settles nothing. The existing smallest-containing rule — the one the press
// correlator applies when it decides which proposal a demonstration landed on, and the one
// `actionables.containing` applies to a live press — would have answered "the word" every single
// time, confidently, and the teaching would have learned nothing while appearing to work.
//
// So a press settles the question only when EXACTLY ONE option contains it. A press inside
// several is a real answer — "that does not distinguish them" — and the question says which part
// of the screen would. That is the whole of why this reads the press against the question's own
// options rather than through the generic attribution path.

import (
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Option is one candidate surface, as something a person can be asked about.
//
// The same candidate the evaluator judged, plus the RELATION — which is the only part of it that
// can outlive this reading of the screen. See observe.DemonstratedSurface for why that is the
// part that gets stored.
type Option struct {
	ElementID directorapi.ElementID `json:"element_id"`
	Bounds    directorapi.Rect      `json:"bounds"`
	// Point is the candidate's own middle, which is the only point it offers. Never
	// invented, and never nudged: see ADR-205.
	Point directorapi.Point `json:"point"`
	// Relation and Depth are how this surface stands to the identity. Depth counts outwards:
	// 0 is the region the words were read on, 1 the tightest surface containing it, and so
	// on.
	Relation observe.SurfaceRelation `json:"relation"`
	Depth    int                     `json:"depth"`
	Evidence Evidence                `json:"evidence"`
}

// Question is one outstanding surface question, scoped to one request.
type Question struct {
	Request string `json:"request"`
	Verb    string `json:"verb"`
	// Identity is the Prop the request named, which is settled. A question is never about
	// WHO was meant — that is an identity problem and the evaluator refuses it before any
	// surface is considered.
	Identity Identity `json:"identity"`
	// Window is the window the options are in, carried so a press can be placed without
	// asking the world again.
	Window directorapi.Rect `json:"window"`
	// Options are the defensible surfaces, nested smallest first.
	Options []Option `json:"options"`
	// Asked is the sentence to put to the person.
	Asked string `json:"asked"`
}

// Open reports whether this is a question somebody could actually answer.
func (q Question) Open() bool { return len(q.Options) > 1 && q.Asked != "" }

// Question is the question this hypothesis leaves open, when it leaves one a press could settle.
//
// Only AMBIGUOUS-with-two-or-more-surfaces produces one. The other unsettled answer — "whether
// the words are the control or its caption", which the evaluator reports whenever the lone
// survivor is the identity's own region — is NOT a question of this shape: there is only one
// option, so there is nothing for a press to choose between, and offering it would be asking
// somebody to confirm the only thing on the menu. That case needs a different interaction and
// does not get a wrong one here.
func (h Hypothesis) Question() (Question, bool) {
	if h.Decision != DecisionAmbiguous || h.Identity == nil || h.Unsettled == "" {
		return Question{}, false
	}
	// An unsettled IDENTITY is not a surface question. The evaluator reports it with the
	// same decision, and a press cannot answer it: two Props named alike are two things a
	// person could have meant, not two places one press could land.
	if h.Unsettled != "which surface is the control" {
		return Question{}, false
	}
	q := Question{
		Request: h.Request, Verb: h.Verb, Identity: *h.Identity,
		Window: h.Window,
	}
	for _, c := range h.Candidates {
		if !c.Defensible() {
			continue
		}
		q.Options = append(q.Options, Option{
			ElementID: c.ElementID, Bounds: c.Bounds, Point: c.Point,
			Evidence: c.Evidence,
		})
	}
	if len(q.Options) < 2 {
		return Question{}, false
	}
	assignDepth(q.Options)
	q.Asked = q.sentence()
	return q, true
}

// assignDepth numbers the options outwards from the identity.
//
// By AREA ASCENDING among the surfaces that are not the identity's own region, which is the
// ordering `fusion.Owners` already produces for nested candidates and is re-derived here rather
// than assumed from slice order: a depth that depended on the order a caller happened to build a
// list in would be a durable number resting on an accident.
//
// Area and not nesting-by-containment, because every option here already contains the identity —
// that is what made it an option — so area ordering IS nesting order for them.
func assignDepth(opts []Option) {
	for i := range opts {
		if opts[i].Evidence.IsIdentity {
			opts[i].Relation = observe.RelationIdentityRegion
			opts[i].Depth = 0
			continue
		}
		rank := 1
		for j := range opts {
			if j == i || opts[j].Evidence.IsIdentity {
				continue
			}
			if opts[j].Bounds.Area() < opts[i].Bounds.Area() {
				rank++
			}
		}
		opts[i].Relation = observe.RelationContainingRegion
		opts[i].Depth = rank
	}
}

// sentence is what to put to the person.
//
// It names what was found, says plainly that where to press is not settled, and asks for the one
// thing that would settle it. It does NOT offer to guess, and it does not describe the options by
// their evidence: "the row has 12 parts and 81% fill" is a sentence about Marco's internals, and
// the person is being asked about their own screen.
func (q Question) sentence() string {
	s := fmt.Sprintf("I found %q, but I cannot tell which area is the control: ",
		q.Identity.Label)
	for i, o := range q.Options {
		if i > 0 {
			s += ", or "
		}
		switch o.Relation {
		case observe.RelationIdentityRegion:
			s += fmt.Sprintf("the %dx%d region the words themselves were read on",
				o.Bounds.Width, o.Bounds.Height)
		default:
			s += fmt.Sprintf("the %dx%d area around them",
				o.Bounds.Width, o.Bounds.Height)
		}
	}
	s += ". Click it once and I will learn where to aim"
	// AND WHERE TO CLICK SO THAT THE ANSWER MEANS SOMETHING. The nested case is the
	// measured one, and a press on the words is inside both options — see the package note.
	if _, ok := q.innermost(); ok {
		s += "; click a part of it the words do not cover, or the press will not " +
			"tell the two apart"
	}
	return s
}

// innermost is the identity's own region, when it is one of the options.
func (q Question) innermost() (Option, bool) {
	for _, o := range q.Options {
		if o.Relation == observe.RelationIdentityRegion {
			return o, true
		}
	}
	return Option{}, false
}

// Outcome is what a person's press did to the question.
//
// Three, matching `workflowstore.AnswerOutcome`'s three one tier down, and for the same reason: a
// caller that collapsed them could not tell a person whether to press again, press somewhere
// else, or stop.
type Outcome string

const (
	// OutcomeSettled: exactly one option contained the press.
	OutcomeSettled Outcome = "settled"
	// OutcomeUndiscriminating: several did, so the press is consistent with all of them.
	//
	// A real answer and the one a person gives first — see the package note.
	OutcomeUndiscriminating Outcome = "undiscriminating"
	// OutcomeElsewhere: none did. The person pressed something that is not one of the
	// options, which usually means the question was about the wrong thing.
	OutcomeElsewhere Outcome = "elsewhere"
)

// Answer is what one press said about the question.
type Answer struct {
	Outcome Outcome `json:"outcome"`
	// At is where the press landed, window-relative — the form the capture produces, and
	// never a desktop coordinate.
	At observe.PointerAt `json:"at"`
	// Inside is EVERY option that contained the press, smallest first, kept whole.
	//
	// All of them on purpose, which is the rule the press correlator already keeps about the
	// regions containing a demonstrated press: "a press that kept only its favourite would
	// have thrown away the thing that could later contradict it".
	Inside []Option `json:"inside,omitempty"`
	// Settled is the one option the press chose, and is set only for OutcomeSettled.
	Settled *Option `json:"settled,omitempty"`
	Because string  `json:"because"`
}

// Answer reads one press as an answer to this question.
//
// The press arrives window-relative, which is the form `navsource` produces and the only form
// that crosses that boundary — so this is not a second input path and there is no desktop
// coordinate to place. Nothing is pressed, nothing is stored, and the question is unchanged: an
// answer is a value derived from a press.
func (q Question) Answer(at observe.PointerAt) Answer {
	a := Answer{At: at, Outcome: OutcomeElsewhere}
	if !q.Open() {
		a.Because = "there is no open question for this press to answer"
		return a
	}
	// INTO THE SPACE THE OPTIONS ARE IN. The options are desktop rectangles because Props
	// are; the press is window-relative because that is all the capture may emit. One
	// multiplication, in the one direction the window's own rectangle supports.
	if q.Window.Width <= 0 || q.Window.Height <= 0 {
		a.Because = "the window has no rectangle, so the press cannot be placed in it"
		return a
	}
	x := q.Window.X + int(at.X*float64(q.Window.Width)+0.5)
	y := q.Window.Y + int(at.Y*float64(q.Window.Height)+0.5)
	for _, o := range q.Options {
		if x < o.Bounds.X || y < o.Bounds.Y ||
			x >= o.Bounds.X+o.Bounds.Width || y >= o.Bounds.Y+o.Bounds.Height {
			continue
		}
		a.Inside = append(a.Inside, o)
	}
	// Smallest first, like every other list of containing candidates in this system.
	for i := 1; i < len(a.Inside); i++ {
		for j := i; j > 0 && a.Inside[j].Bounds.Area() < a.Inside[j-1].Bounds.Area(); j-- {
			a.Inside[j], a.Inside[j-1] = a.Inside[j-1], a.Inside[j]
		}
	}
	switch len(a.Inside) {
	case 0:
		a.Because = fmt.Sprintf("the press at (%.3f, %.3f) is not inside any of the "+
			"%d surfaces this question is about", at.X, at.Y, len(q.Options))
	case 1:
		a.Outcome = OutcomeSettled
		a.Settled = &a.Inside[0]
		a.Because = fmt.Sprintf("the press is inside %s and inside none of the other "+
			"%d surface(s), so it tells them apart", a.Inside[0].ElementID,
			len(q.Options)-1)
	default:
		a.Outcome = OutcomeUndiscriminating
		a.Because = fmt.Sprintf("the press is inside %d of these surfaces at once, so "+
			"it is consistent with all of them and settles nothing; a press on a "+
			"part of the control the words do not cover would tell them apart",
			len(a.Inside))
	}
	return a
}

// Given is everything about a demonstration that this package cannot observe for itself.
//
// Target is the durable subject id of the semantic target, which the caller got from
// `RememberTarget` — not a name, because the name belongs to the subject and repeating it here
// would widen what durable memory holds.
//
// Confirmed is how well the press was confirmed, and it is emphatically the CALLER's to state:
// this package watches nothing happen and so cannot know whether the screen changed. A default
// here would be this package vouching for something it did not see.
type Given struct {
	Application string
	Target      string
	By          string
	At          time.Time
	Confirmed   observe.SurfaceConfirmation
}

// Demonstrated turns a settled answer into the one durable fact it established.
//
// Refuses anything but a settled answer. A press that did not tell the options apart has
// established nothing, and writing it down as though it had is the exact failure the package note
// describes.
func (q Question) Demonstrated(a Answer, g Given) (observe.DemonstratedSurface, error) {
	if a.Outcome != OutcomeSettled || a.Settled == nil {
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: the press %s, so it "+
			"established nothing to remember", a.Outcome)
	}
	o := *a.Settled
	d := observe.DemonstratedSurface{
		Application: g.Application, Target: g.Target,
		Relation: o.Relation, Depth: o.Depth,
		Shape:     shapeOf(o, q.Identity.Bounds, q.Window),
		Confirmed: g.Confirmed, By: g.By, At: g.At.UTC(),
	}
	if ok, why := d.Valid(); !ok {
		return observe.DemonstratedSurface{}, fmt.Errorf("surface: %s", why)
	}
	return d, nil
}

// shapeOf is what a surface looked like, normalised against the window.
//
// EVIDENCE, and the comment is the contract: nothing matches on these rectangles. They are here
// so a later observation can contradict the record — see observe.SurfaceShape.
func shapeOf(o Option, identity, window directorapi.Rect) observe.SurfaceShape {
	return observe.SurfaceShape{
		Surface:       normalise(o.Bounds, window),
		Identity:      normalise(identity, window),
		Containment:   o.Evidence.Containment,
		AreaShare:     o.Evidence.AreaShare,
		Kind:          o.Evidence.StructureKind,
		Fill:          o.Evidence.Fill,
		Parts:         o.Evidence.Parts,
		Siblings:      o.Evidence.Siblings,
		DistinctNames: o.Evidence.DistinctTexts,
	}
}

// normalise converts a desktop rectangle into the window-relative space `observe` speaks.
//
// The same arithmetic as `observe.relativeTo`, origin included — which `locates` got wrong once
// and which `TestABandIsStillABandWhenTheWindowIsNotAtTheOrigin` now holds.
func normalise(r, window directorapi.Rect) observe.Region {
	if window.Width <= 0 || window.Height <= 0 {
		return observe.Region{}
	}
	return observe.Region{
		X:      float64(r.X-window.X) / float64(window.Width),
		Y:      float64(r.Y-window.Y) / float64(window.Height),
		Width:  float64(r.Width) / float64(window.Width),
		Height: float64(r.Height) / float64(window.Height),
	}
}
