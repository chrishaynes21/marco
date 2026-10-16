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
	// WindowID is which window that is.
	//
	// Carried because a question is bound to one window: a press that arrived while a
	// different window was in front is not an answer to this, and the holder needs the id
	// to say so. The rectangle above cannot: two windows can share one.
	WindowID directorapi.WindowID `json:"window_id,omitempty"`
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
	if h.Unsettled != unsettledWhichSurface {
		return Question{}, false
	}
	q := Question{
		Request: h.Request, Verb: h.Verb, Identity: *h.Identity,
		Window: h.Window, WindowID: h.WindowID,
		Options: h.surfaceOptions(),
	}
	if len(q.Options) < 2 {
		return Question{}, false
	}
	// AND ONLY WHEN A PRESS COULD ANSWER IT.
	//
	// Two options are not a question. A question needs somewhere a person could press
	// that would single ONE of them out, and on a live accessibility tree that is
	// routinely untrue: a control and its own wrapper arrive at identical bounds with
	// identical press points, or two pixels apart, and every point either offers lies
	// inside both. Asking then is asking for a demonstration that cannot exist, and the
	// person's correct press comes back as `undiscriminating` — which reads as their
	// mistake. Measured on Windows Settings: e49 {908 54 98 38} against e50 {908 54 98
	// 38}, same press point, and three options on the Personalisation panel of which
	// two differed by two pixels.
	//
	// Deleting this must fail TestAQuestionNoPressCouldAnswerIsNotAsked.
	if _, can := aPressCouldTellThemApart(q.Options); !can {
		return Question{}, false
	}
	assignDepth(q.Options)
	q.Asked = q.sentence()
	return q, true
}

// WhereToPress is the place a press would single out exactly one of this question's options.
//
// For telling somebody where to click, which is the one thing the instruction sentence
// cannot do: "on a part the words do not cover" is a description, and a rectangle is an
// address. The same rectangle the admission gate found, so the place offered and the reason
// the question exists are one fact.
//
// False only for a question that should not have been asked, which `Question` refuses to
// build — so a caller holding an open question can rely on this.
func (q Question) WhereToPress() (directorapi.Rect, bool) {
	return aPressCouldTellThemApart(q.Options)
}

// surfaceOptions is the defensible surfaces as options, in candidate order.
//
// ONE BUILDER, used by Question and by the evaluator's own explanation of why it did not
// ask, so the question that is offered and the reason given for withholding it are about
// the same set of surfaces.
func (h Hypothesis) surfaceOptions() []Option {
	var out []Option
	for _, c := range h.Candidates {
		if !c.Defensible() {
			continue
		}
		out = append(out, Option{
			ElementID: c.ElementID, Bounds: c.Bounds, Point: c.Point,
			Evidence: c.Evidence,
		})
	}
	return out
}

// aPressCouldTellThemApart reports where somebody could press to single out exactly one of
// these surfaces.
//
// # Why a region and not a point
//
// The first version of this asked whether any option's own middle fell inside exactly one
// option, on the reasoning that those are the only points this system itself would press. It was
// wrong, and a fixture said so: two peer surfaces overlapping in the middle are told apart by a
// press near either outer edge, and neither middle is in exactly one of them. A person presses
// where they like, so the question is whether a usable PLACE exists, not whether this system
// would have nominated it.
//
// # What makes a place usable, without a new number
//
// The only pointer-safety rule in force anywhere in this tree is `ClickableByBounds`:
// a non-empty rectangle is somewhere real to aim. That alone would admit a two-pixel margin
// between two rectangles that differ by two pixels, which is exactly the case the owner named as
// not usable.
//
// So the minimum comes from the evidence on screen rather than from a constant: the
// discriminating area must be at least as large, in both directions, as the SMALLEST SURFACE
// THIS QUESTION IS ALREADY WILLING TO PRESS. If a 24x24 icon is a legitimate target here, a
// 24x24 exclusive area is equally pointable; if the exclusive area cannot hold the smallest
// thing on the menu, it is not a target. Nothing is tuned to any particular screen and no
// absolute size appears, which is the property that matters: a threshold chosen to make one
// application pass is a threshold that will fail the next one silently.
//
// # The search is greedy, and therefore conservative
//
// Each surface is clipped by every other, keeping the largest of the four rectangles that
// survive each clip. That can miss a qualifying area a full plane sweep would find, and the
// direction is deliberate: refusing a question that was barely answerable costs an instruction,
// while asking one that was not costs somebody's trust in the request.
//
// Measured against the cases that mattered. Admits e28 {637 424 126 18} inside
// e27 {613 400 492 66}, where the clear part of the card is 342x66. Admits the two peer
// surfaces of the contest fixture, at 50x28. Refuses e49 {908 54 98 38} against
// e50 {908 54 98 38}, which leaves nothing at all, and refuses e58 {908 240 116 94} against
// e59 {910 242 112 90}, whose widest exclusive strip is 116x2 against a smallest surface of 24.
func aPressCouldTellThemApart(opts []Option) (directorapi.Rect, bool) {
	least := 0
	for _, o := range opts {
		side := o.Bounds.Width
		if o.Bounds.Height < side {
			side = o.Bounds.Height
		}
		if side <= 0 {
			// A surface with no rectangle is somewhere nobody can aim, and it
			// cannot set the bar for anybody else either.
			continue
		}
		if least == 0 || side < least {
			least = side
		}
	}
	if least == 0 {
		return directorapi.Rect{}, false
	}
	for i, o := range opts {
		clear := o.Bounds
		for j, other := range opts {
			if i == j {
				continue
			}
			clear = outside(clear, other.Bounds)
			if clear.Empty() {
				break
			}
		}
		if clear.Width >= least && clear.Height >= least {
			return clear, true
		}
	}
	return directorapi.Rect{}, false
}

// outside is the largest part of r that b does not cover.
//
// One of the four rectangles r is cut into by b, or nothing when b covers r. Half-open, like
// `covers` and like every other rectangle in this package, so a shared edge is not an overlap.
func outside(r, b directorapi.Rect) directorapi.Rect {
	if r.Empty() || b.Empty() {
		return r
	}
	// NO OVERLAP IS NO CLIP. Checked first because the common case on a real screen is
	// seven surfaces of which one contains the identity and the rest are elsewhere.
	if b.X >= r.X+r.Width || b.X+b.Width <= r.X ||
		b.Y >= r.Y+r.Height || b.Y+b.Height <= r.Y {
		return r
	}
	parts := []directorapi.Rect{
		{X: r.X, Y: r.Y, Width: b.X - r.X, Height: r.Height},
		{X: b.X + b.Width, Y: r.Y, Width: r.X + r.Width - (b.X + b.Width),
			Height: r.Height},
		{X: r.X, Y: r.Y, Width: r.Width, Height: b.Y - r.Y},
		{X: r.X, Y: b.Y + b.Height, Width: r.Width,
			Height: r.Y + r.Height - (b.Y + b.Height)},
	}
	best := directorapi.Rect{}
	for _, p := range parts {
		if p.Width <= 0 || p.Height <= 0 {
			continue
		}
		if p.Width*p.Height > best.Width*best.Height {
			best = p
		}
	}
	return best
}

// covers is this package's one containment rule: half-open, like every rectangle here.
func covers(b directorapi.Rect, x, y int) bool {
	return x >= b.X && y >= b.Y && x < b.X+b.Width && y < b.Y+b.Height
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
		if !covers(o.Bounds, x, y) {
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
