package observe

import "time"

// WHAT EXTRA EVIDENCE IS WORTH ACQUIRING RIGHT NOW?
//
// # The question this answers, and the one it refuses to
//
// 37D built a sensor-neutral statement about the primary reading: sufficient, incomplete, or
// unobservable. It deliberately names no sensor, because deciding WHICH sensor to spend on has
// to weigh cost against what is actually missing, and a classifier that returned
// `UseScreenParser` would have decided the architecture of every future sensor by accident.
//
// This is where that weighing happens, and it is a budget decision rather than an evidence
// one. It produces no observation, admits nothing to fusion, and grants nothing. Its whole
// output is an answer to "is more perception worth paying for at this moment".
//
// # Why it still names no sensor
//
// [SpendMore] says the current reading cannot answer the question and more evidence is worth
// buying. It does not say which. That stays open to OCR, to a detector, to a targeted region
// capture, to a sensor nobody has written — and the caller that owns a particular sensor
// decides whether it is the one, because only it knows its own cost and what it can see.
//
// # The measured facts behind the rules
//
// 37C measured ScreenParser against healthy desktop accessibility over six coherent moments:
// of 473 detections, every one of the 302 that had no accessibility element at IoU still had
// its centre inside an element production already perceived. It adds no actionable semantic
// item to a reading that is already sufficient, and costs 645–1379ms to say so.
//
// So the first rule is the one that matters: a sufficient reading buys nothing. Not "buys
// little" — 37C looked for the something and did not find it.
//
// The second is about time rather than structure. A page mid-navigation is briefly indistinct
// from a page that failed to arrive, and a settle costs nothing while an inference costs
// most of a second. Waiting is the cheaper hypothesis and it is usually right.
type Spend string

const (
	// SpendNothing is a reading that already answers the question.
	SpendNothing Spend = "nothing"
	// SpendSettle is "look again shortly with what you already have". No new sensor, no
	// new cost beyond one more ordinary reading.
	SpendSettle Spend = "settle"
	// SpendMore is "this reading cannot answer it and more evidence is worth buying".
	// WHICH evidence is the caller's decision — see the note above.
	SpendMore Spend = "more"
	// SpendNothingAndRefuse is a reading nothing can be built on. Spending a detector on a
	// window the sensors never reached buys a picture of something Marco cannot attribute
	// to anything, and the honest answer to the caller is no.
	SpendNothingAndRefuse Spend = "refuse"
)

// Need is how much the caller actually requires, which changes what is worth buying.
type Need string

const (
	// NeedWatching is passive observation: no authority, no request, nobody waiting.
	// An incomplete reading here is a fact to record, not a problem to spend money on.
	NeedWatching Need = "watching"
	// NeedAnswer is somebody asked. A person waiting for "where am I" is worth spending
	// on in a way that background curiosity is not.
	NeedAnswer Need = "answer"
	// NeedToAct is evidence that will be acted upon. The strictest, and the one where
	// refusing is better than proceeding on a reading that does not represent the screen.
	NeedToAct Need = "act"
)

// settleWindow is how long a reading may be incomplete before waiting stops being the cheaper
// hypothesis.
//
// Not a threshold on degradation — structure decides that, and 37D's classifier never looks at
// a clock. This is only about whether the CHEAP remedy has had its chance. A page that has been
// blank for two seconds is not loading slowly any more.
const settleWindow = 2 * time.Second

// Escalation is the decision and why it was reached.
type Escalation struct {
	Spend Spend
	// Because is the sufficiency reason it was decided from, carried so a caller can say
	// what it is waiting for rather than only that it is waiting.
	Because SufficiencyReason
}

// EscalationOf decides what more, if anything, is worth acquiring.
//
// `incompleteFor` is how long the reading has been incomplete — zero when this is the first
// such reading. It corroborates; it never classifies. A caller that cannot track it passes
// zero and gets the settle first, which is the cheap answer and the safe one.
func EscalationOf(need Need, s Sufficiency, sem SemanticSufficiency,
	incompleteFor time.Duration) Escalation {

	switch s.State {
	case Unobservable:
		// Nothing was read, so there is nothing to add to. A detector pointed at a
		// window the sensors never reached produces pixels belonging to nothing.
		return Escalation{Spend: SpendNothingAndRefuse, Because: s.Reason}

	case Sufficient:
		// THE RULE 37C PAID FOR, AND THE CASE IT DID NOT MEASURE.
		//
		// 37C compared a detector against HEALTHY DESKTOP ACCESSIBILITY — screens
		// whose accessibility already said what everything was — and found it added
		// no actionable semantic item, at 645–1379ms an inference. That result is
		// sound and this preserves it exactly where it was taken: a reading that can
		// say which state it is buys nothing, forever.
		//
		// What it did not measure is a reading that describes an interface perfectly
		// and says nothing about WHICH state it is. "Vision adds little when
		// accessibility already understands the interface" became "do not buy vision
		// when accessibility structurally reaches the interface", and those are
		// different claims. The second one made every Xbox game one screen.
		//
		// So semantic silence may buy — and WHAT it buys is still nobody's business
		// here. The caller holding an expensive sensor decides whether it is the one,
		// and the caller holding a budget decides how often. This says only that the
		// reading cannot answer the question.
		//
		// Deleting the semantic arm must fail
		// TestASemanticallySilentReadingMayBuyOneRepair.
		if sem.State == StateSilent {
			return Escalation{Spend: SpendMore, Because: s.Reason}
		}
		return Escalation{Spend: SpendNothing, Because: s.Reason}

	case Incomplete:
		if incompleteFor < settleWindow {
			// Cheaper hypothesis first: it is still arriving.
			return Escalation{Spend: SpendSettle, Because: s.Reason}
		}
		if need == NeedWatching {
			// Nobody is waiting. An interface accessibility cannot represent is a
			// standing condition — a game, a canvas, an application that draws
			// itself — and buying a second of inference every cadence to keep
			// confirming it is the expense this whole phase exists to refuse.
			return Escalation{Spend: SpendSettle, Because: s.Reason}
		}
		return Escalation{Spend: SpendMore, Because: s.Reason}
	}
	return Escalation{Spend: SpendSettle, Because: s.Reason}
}

// Worth reports whether a caller holding an expensive sensor should run it.
//
// The convenience the one wiring in production needs, and narrow on purpose: it answers only
// "spend", and a caller wanting to know why asks the [Escalation].
func (e Escalation) Worth() bool { return e.Spend == SpendMore }
