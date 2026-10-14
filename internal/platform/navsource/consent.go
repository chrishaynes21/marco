package navsource

// Answering a question Marco asked, without taking the keyboard away from the application.
//
// # Why this is here and not in a new package
//
// Because a person supervising an action cannot answer in a terminal. Typing makes the terminal
// the foreground window, and the foreground is exactly the precondition a click depends on — a
// click carries no address and lands in whatever is in front. So the act of granting permission
// destroys the thing the permission was for, and the run refuses itself. The way out is an
// answer that does not move the foreground, which means a key observed by a hook rather than
// read from a console.
//
// A hook already exists in this package, and installing a second one is not available: Windows
// delivers a low-level hook callback only while the installing thread is in a message wait, the
// tree is already walked for hook sites by `pump_test.go`, and a fourth install is a fourth
// chance to get the pump wrong and silently unhook the other three.
//
// # Why it is a separate path from navigation, in the same hook
//
// Everything else this package produces is an OBSERVATION of a person operating an application:
// it is evidence, it feeds correlation, and it is bounded to the keys whose navigation meaning
// is conventional. A consent key is not that. It is an ANSWER, addressed to Marco, about
// something Marco has just asked. Folding it into the navigation vocabulary would put a person's
// reply to a prompt into the record of what they demonstrated, and would let a correlator treat
// "yes" as a menu action.
//
// So a consent key is recognised before classification, never becomes a NavIntent, never reaches
// a subscription, and is counted as ignored like every other admitted-and-not-classified event —
// the total has to add up, and `IgnoreKeyRelease` is the note explaining what happens to a
// producer whose total does not.
//
// # What this does not widen
//
// Three keys, chosen because they are keys nothing types. The hook still refuses injected
// events at the callback, so Marco cannot authorise Marco; it still never suppresses, so the
// application receives the keypress as it always would; and it still observes no key beyond this
// table and the navigation one. This is not a channel for arbitrary input and there is nothing
// here to extend it into one.

import "github.com/chaynes-simpleclouds/marco/internal/director/observe"

// Virtual-key codes for the consent keys. Listed beside the navigation ones in keys.go for the
// same reason: the admission decision should be reviewable without reading hook code.
const (
	vkF7 = 0x76
	vkF8 = 0x77
	vkF9 = 0x78
)

// The consent keys, exported for the same reason the navigation ones are: a test that restated
// 0x76 would be asserting against a magic number rather than against the table.
const (
	KeyF7 = vkF7
	KeyF8 = vkF8
	KeyF9 = vkF9
)

// Consent is an answer to a question Marco asked.
//
// Deliberately three answers and not two. "No" and "stop" are different sentences: refusing one
// action leaves the run free to ask about the next, and a person who wants it to end should not
// have to refuse every remaining question to get there.
type Consent string

const (
	// ConsentApprove permits the ONE action currently being asked about. It is never
	// permission for the next one — see workflowrun.Authorisation.
	ConsentApprove Consent = "approve"
	// ConsentRefuse declines the one action currently being asked about.
	ConsentRefuse Consent = "refuse"
	// ConsentStop ends the run, whether or not anything is being asked.
	ConsentStop Consent = "stop"
)

// ConsentKeys names the physical keys, for a prompt that has to tell somebody what to press.
//
// Exported as a map rather than three constants so a caller cannot print one binding and listen
// for another.
var ConsentKeys = map[Consent]string{
	ConsentApprove: "F7",
	ConsentRefuse:  "F8",
	ConsentStop:    "F9",
}

// consent is the admitted vocabulary.
var consent = map[uint16]Consent{
	vkF7: ConsentApprove,
	vkF8: ConsentRefuse,
	vkF9: ConsentStop,
}

// ConsentDepth is how many unanswered consent keys are held.
//
// Small, and small on purpose. A consent key is only meaningful next to the question it answers,
// so a deep queue would let presses from a minute ago answer a question asked now. The consumer
// drains what it finds before asking anything, which is the other half of the same rule.
const ConsentDepth = 4

// Consents is the stream of answers. Nil when this platform has no hook.
//
// A receive-only channel rather than a callback, so the thing waiting for an answer is an
// ordinary select alongside a deadline and a cancellation, and so nothing can be invoked on the
// classifier worker by a consumer that blocks.
func (s *Source) Consents() <-chan Consent { return s.consents }

// offerConsent hands an answer to whoever is waiting, and drops it when nobody is.
//
// Non-blocking, like everything else on the path from the hook: the worker must not wait on a
// consumer. Dropping is correct rather than merely safe — a consent key pressed while no
// question is open answers nothing, and holding it would mean answering the NEXT question with a
// keypress made before it was asked.
func (s *Source) offerConsent(c Consent) {
	if s.consents == nil {
		return
	}
	select {
	case s.consents <- c:
	default:
	}
}

// consentOf resolves a key to an answer.
func consentOf(code uint16) (Consent, bool) {
	c, ok := consent[code]
	return c, ok
}

// ReasonConsent is a key that answered a question Marco asked.
//
// Counted as ignored because that is what it is: this package's output is navigation, and a
// consent key produces none. It is NOT IgnoreUnsupported — a reader looking at a run where
// somebody pressed F7 four times needs to see four answers, not four unrecognised keys.
const ReasonConsent = observe.IgnoreConsent
