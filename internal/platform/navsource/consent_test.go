package navsource

// A consent key is an ANSWER, and an answer is not an observation.
//
// The whole of this package's output is a record of a person operating an application. A reply
// addressed to Marco — permit this, refuse this, stop — is a different kind of thing, and the
// separation is what these assert: it reaches whoever asked the question, it reaches no
// subscription, it produces no intent, and the totals still add up.

import (
	"testing"
	"time"
)

// A CONSENT KEY REACHES WHOEVER ASKED, AND REACHES NO SESSION.
//
// The second half is the one worth asserting. A session is open and watching; somebody answers a
// question Marco put up; and nothing about that answer appears in what the session saw. If it
// did, a reply to a prompt would be in the record of what the person demonstrated, and a
// correlator would be free to treat "yes" as a menu action.
func TestAConsentKeyAnswersMarcoAndEntersNoSession(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	b.press(vkF7)

	select {
	case got := <-s.Consents():
		if got != ConsentApprove {
			t.Fatalf("F7 arrived as %q, want %q", got, ConsentApprove)
		}
	case <-time.After(time.Second):
		t.Fatal("F7 never reached the thing that asked the question")
	}

	time.Sleep(50 * time.Millisecond)
	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("an answer entered the session as %d intent(s): %+v", len(got), got)
	}
	if n := s.Stats().Classified; n != 0 {
		t.Errorf("%d answer(s) were classified as navigation", n)
	}
	// AND IT IS COUNTED AS WHAT IT IS. Not "unsupported": somebody reading a run where
	// four questions were answered needs to see four answers, not four keys nobody
	// recognised.
	if n := s.Stats().Ignored[ReasonConsent]; n != 1 {
		t.Errorf("consent count %d, want 1 (ignored: %v)", n, s.Stats().Ignored)
	}
	if n := s.Stats().Ignored[ReasonUnsupported]; n != 0 {
		t.Errorf("%d answer(s) were counted as keys nobody recognised", n)
	}
}

// EVERY ADMITTED KEY IS AN ANSWER AND NOTHING ELSE IS.
//
// The table, asserted against rather than restated. A key added to `consent` without being
// thought about is a key this package starts observing, which is the boundary the package exists
// to hold.
func TestOnlyTheThreeConsentKeysProduceAnAnswer(t *testing.T) {
	for _, tc := range []struct {
		code uint16
		want Consent
	}{
		{vkF7, ConsentApprove},
		{vkF8, ConsentRefuse},
		{vkF9, ConsentStop},
	} {
		t.Run(string(tc.want), func(t *testing.T) {
			s, b := newTestSource(t)
			b.press(tc.code)
			select {
			case got := <-s.Consents():
				if got != tc.want {
					t.Fatalf("got %q, want %q", got, tc.want)
				}
			case <-time.After(time.Second):
				t.Fatal("no answer arrived")
			}
		})
	}

	// AND A NAVIGATION KEY IS NOT AN ANSWER, or opening a menu would permit a press.
	s, b := newTestSource(t)
	s.Open(time.Now())
	for _, code := range []uint16{vkReturn, vkEscape, vkDown, vkSpace} {
		b.press(code)
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case got := <-s.Consents():
		t.Fatalf("a navigation key answered a question as %q", got)
	default:
	}
}

// AND A HELD ANSWER IS ONE ANSWER.
//
// A low-level hook reports a held key as a stream of key-downs with no repeat flag. Without the
// held set, somebody resting a finger on the permit key would permit an action every few
// milliseconds for as long as they leaned on it.
func TestAHeldConsentKeyProducesExactlyOneAnswer(t *testing.T) {
	s, b := newTestSource(t)

	b.offer(rawEvent{code: vkF7, down: true, at: time.Now()})
	for i := 0; i < 39; i++ {
		b.offer(rawEvent{code: vkF7, down: true, at: time.Now()})
	}
	b.offer(rawEvent{code: vkF7, down: false, at: time.Now()})
	time.Sleep(80 * time.Millisecond)

	n := 0
	for {
		select {
		case <-s.Consents():
			n++
			continue
		default:
		}
		break
	}
	if n != 1 {
		t.Fatalf("a held key produced %d answers, want 1", n)
	}
	if got := s.Stats().Ignored[ReasonRepeat]; got != 39 {
		t.Errorf("repeat count %d, want 39", got)
	}
}

// AND NOBODY LISTENING MUST NOT STALL THE CLASSIFIER.
//
// The consent queue is small on purpose — an answer is only meaningful beside its question — so
// overflowing it is the ordinary case rather than the exceptional one. A blocking send would not
// stall the HOOK, because the hook-facing offer is already non-blocking and would simply start
// dropping; it would stall the worker behind it, and a stalled worker processes nothing else
// ever again. So the proof is that ordinary navigation still arrives after the consent queue has
// been overflowed many times over.
//
// A mutation run found this: the first version of this test overflowed the queue and asserted
// that the presses returned, which they did with a blocking send in place, because the thing
// that returns is the hook and the hook was never the part at risk.
func TestAnswersNobodyIsWaitingForDoNotStallTheClassifier(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	for i := 0; i < ConsentDepth*10; i++ {
		b.press(vkF7)
	}
	// AND THE WORKER IS STILL WORKING.
	b.press(vkDown)
	if got := drainWithin(t, sub, 1); len(got) != 1 {
		t.Fatalf("after %d unread answers the classifier delivered %d intent(s), "+
			"want 1 — the worker is stalled behind a consent nobody read",
			ConsentDepth*10, len(got))
	}
}
