package playbill_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// What the visibility representation must be true of, whatever the Director says.
//
// These are STRUCTURAL where they can be. A test that populates a View and checks the
// result proves the value it built; a test that walks the type graph proves that no
// value can be built. For a privacy boundary the second is the only useful kind — the
// leak that matters is the field somebody adds in six months, not the one in this file.

// ── F: private observation content cannot enter the representation ────────────

// forbiddenNames is the vocabulary of things this record must never carry.
//
// Deliberately about NAMES rather than types, for the reason the observation record's
// own privacy test gives: `RawKey int` is a keylogger and an int is not suspicious. A
// string field here is often perfectly legitimate — a Director-authored sentence, a
// screen name somebody typed — so a type-based rule would either miss the integer or
// condemn all of them.
//
// The list is the four classes the milestone refuses: what the user pressed, what was on
// screen, what a window is called, and what the platform calls its objects.
var forbiddenNames = []string{
	// keys and typed input
	"keycode", "scancode", "vkcode", "rawkey", "keystroke", "keypress", "keyname",
	"character", "charcode", "rune", "typed", "clipboard",
	// screen content
	"ocr", "rawtext", "raw_text", "screenshot", "image", "pixels", "frame", "bitmap",
	"caption", "tooltip", "placeholder", "innertext", "inner_text", "content",
	// window and platform identity
	"title", "windowtitle", "window_title", "hwnd", "handle", "pid", "processid",
	"process_id", "runtimeid", "runtime_id", "generation", "elementid", "element_id",
	// model output
	"prompt", "completion", "response_text", "reasoning",
}

// allowedSuffixes are field names that CONTAIN a forbidden word and are demonstrably
// something else.
//
// Kept tiny and explicit. Every entry here is a hole in the rule above, so each one has
// to be worth its own line — and "it was inconvenient" is not a reason to add one.
var allowedNames = map[string]bool{
	// Question.Answers is the CLOSED answer vocabulary — "confirmed", "declined" — and
	// the guard checks each one is a vocabulary word.
	"View.Question.Answers": true,
}

type walked map[reflect.Type]bool

func checkNames(t *testing.T, rt reflect.Type, path string, seen walked) {
	t.Helper()
	for rt.Kind() == reflect.Ptr || rt.Kind() == reflect.Slice ||
		rt.Kind() == reflect.Array || rt.Kind() == reflect.Map {

		if rt.Kind() == reflect.Map {
			checkNames(t, rt.Key(), path+"[key]", seen)
		}
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct || seen[rt] {
		return
	}
	seen[rt] = true
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		at := path + "." + f.Name
		if !allowedNames[at] {
			low := strings.ToLower(f.Name)
			for _, bad := range forbiddenNames {
				if strings.Contains(low, bad) {
					t.Errorf("%s names %q. The playbill is rendered on somebody's screen "+
						"for hours, screenshotted and pasted into issues. Passively "+
						"observed arbitrary content does not become showable because a "+
						"human wanted to debug something", at, bad)
				}
			}
		}
		checkNames(t, f.Type, at, seen)
	}
}

// F. Nothing in the shared visibility representation can hold private observation content.
func TestTheRepresentationHasNowhereToPutPrivateContent(t *testing.T) {
	checkNames(t, reflect.TypeOf(playbill.View{}), "View", walked{})
}

// A window title is the thing most likely to be wired in by accident, because it is
// right there on every window and it reads well. The guard refuses it on its shape.
func TestAWindowTitleIsRefusedWhereAnApplicationKeyBelongs(t *testing.T) {
	v := present()
	v.Current.Application = "Untitled - Notepad"

	if err := v.Admit(); err == nil {
		t.Fatal("a window title was admitted as an application key. Titles carry " +
			"document names, chat and account names, and this field is polled twice a " +
			"second into a panel left open for hours")
	}
	// And the refusal is not silent: the caller gets a playbill that SAYS so.
	out := v.Admitted()
	if out.Current.Application != "" || out.Why == "" {
		t.Errorf("a refused playbill still carried its content: %+v", out.Current)
	}
}

// Read text arrives with newlines in it, which is how a block of screen content gets
// into a field sized for a sentence.
func TestMultiLineTextIsRefusedWhereASentenceBelongs(t *testing.T) {
	v := present()
	v.Thinking.Readings = []playbill.Reading{{
		Says: "This might be a menu.", Standing: playbill.Tentative,
		Because: "New Game\nContinue\nSettings\nQuit",
	}}
	if err := v.Admit(); err == nil {
		t.Fatal("a multi-line block was admitted as a sentence")
	}
}

// ── E: visibility cannot create authority ─────────────────────────────────────

// E. There is nothing on this type that performs, authorises or can be replayed.
//
// A structural test, deliberately. "We reviewed it and there is no Run method" is true
// until somebody adds a convenience, and the convenience will be added by whoever is
// tired of switching to a terminal.
func TestNothingOnThePlaybillCanAct(t *testing.T) {
	forbidden := []string{
		"run", "execute", "perform", "invoke", "approve", "authorize", "authorise",
		"grant", "confirm", "rehearse", "start", "cancel", "stop", "answer", "respond",
		"apply", "force", "mark", "assume", "set",
	}
	rt := reflect.TypeOf(playbill.View{})
	for i := 0; i < rt.NumMethod(); i++ {
		name := strings.ToLower(rt.Method(i).Name)
		for _, bad := range forbidden {
			if strings.HasPrefix(name, bad) {
				t.Errorf("View has a method %q. A visibility surface that can act is "+
					"a second door into execution, and it is the door nobody audits",
					rt.Method(i).Name)
			}
		}
	}
	// The grant is reported as a STATE and never as a grant, so nothing downstream can
	// unmarshal authority out of a rendered panel.
	if strings.Contains(string(mustJSON(t, present())), "MaxInputs") {
		t.Error("a rehearsal grant's bounds reached the wire")
	}
}

// A question carries what is needed to answer through the EXISTING path, and nothing more.
func TestAQuestionMustNameAnExistingResponsePath(t *testing.T) {
	v := present()
	v.Question = &playbill.Question{
		ID: "q_abc", Asks: "Is this the pause menu?", Wants: playbill.WantsChoice,
		Answers: []string{"confirmed", "declined"},
	}
	if err := v.Admit(); err == nil {
		t.Fatal("a question with no response path was admitted. A surface rendering it " +
			"would have to invent a way to answer, which is the one thing this " +
			"representation must never invite")
	}
	v.Question.Via = playbill.ViaProposal
	if err := v.Admit(); err != nil {
		t.Fatalf("a well-formed question was refused: %v", err)
	}
	// An unanswerable question is refused too: rendered forever, it would look like
	// Marco waiting on somebody who has no way to reply.
	v.Question.ID = ""
	if err := v.Admit(); err == nil {
		t.Fatal("a question with no id was admitted")
	}
}

// ── G: the recent-event history is bounded ────────────────────────────────────

// G. However long a session runs, a playbill carries a bounded timeline.
func TestTheTimelineIsBounded(t *testing.T) {
	v := present()
	for i := 0; i < playbill.MaxMoments*4; i++ {
		v.Recent = append(v.Recent, playbill.Moment{
			Seq: uint64(i + 1), At: time.Now(), Says: "the screen changed"})
	}
	bounded := v.Bound()
	if len(bounded.Recent) != playbill.MaxMoments {
		t.Fatalf("timeline held %d moments, want the bound of %d",
			len(bounded.Recent), playbill.MaxMoments)
	}
	// The NEWEST are kept. A bound that dropped the newest would show a person the
	// beginning of the thing they are trying to understand and not the end of it.
	last := bounded.Recent[len(bounded.Recent)-1]
	if last.Seq != uint64(playbill.MaxMoments*4) {
		t.Errorf("the newest moment was dropped: last seq = %d", last.Seq)
	}
	// And the guard refuses an over-long one outright, so a producer that forgot to
	// bound cannot ship past it.
	if err := v.Admit(); err == nil {
		t.Fatal("an unbounded timeline was admitted")
	}
}

// ── H: repeated identical samples do not create events ────────────────────────

// H. An unchanging Director produces an unchanging digest, however often it is polled.
//
// The digest is what a surface coalesces on. If it moved with the clock or the sample
// count then every poll would look like news, and a person watching a still screen would
// see the panel churn — which trains them to stop reading it.
func TestPollingAStillDirectorProducesTheSameDigest(t *testing.T) {
	first := present().WithDigest()

	later := present()
	// Everything that moves on its own while nothing happens.
	later.TakenAt = first.TakenAt.Add(90 * time.Second)
	later.UptimeMS = first.UptimeMS + 90_000
	later.Current.Samples = first.Current.Samples + 180
	later.Current.FreshnessMS = 412
	later.Cursor = first.Cursor + 40
	later = later.WithDigest()

	if first.Digest != later.Digest {
		t.Fatalf("the digest moved while nothing changed: %s -> %s\n"+
			"a surface coalescing on this would redraw on every poll",
			first.Digest, later.Digest)
	}

	// And it DOES move when something a person would notice changes.
	changed := present()
	changed.Current.Recognition = playbill.Ambiguous
	if changed.WithDigest().Digest == first.Digest {
		t.Error("the digest did not move when the recognition verdict changed. " +
			"Coalescing on it would hide the one thing this panel exists to show")
	}
}

// ── J: an absent Director is unavailable, never stale certainty ───────────────

// J. When the Director disappears, the account says so rather than holding its last belief.
func TestAnAbsentDirectorIsNotStaleCertainty(t *testing.T) {
	for _, reach := range []playbill.Reach{playbill.Unreachable, playbill.Absent} {
		v := playbill.Unavailable(reach, "the Director service is not running")
		if err := v.Admit(); err != nil {
			t.Fatalf("%s: %v", reach, err)
		}
		if v.Current.Recognition != playbill.Unobservable {
			t.Errorf("%s reported a recognition verdict: %q", reach, v.Current.Recognition)
		}
		if v.Current.Watching || v.Current.Screen != "" {
			t.Errorf("%s claimed to be watching something", reach)
		}
		if v.Learning.Stage != playbill.NotLearning || v.Doing.Phase != playbill.NotDoing {
			t.Errorf("%s carried a live stage or phase", reach)
		}
		// The reduction says it too. A consumer surface must not read "Ready".
		if h := v.Normal(); h.Word == "Ready" || h.Attention {
			t.Errorf("%s reduced to %q", reach, h.Word)
		}
	}
}

// The zero value is NOT a valid account, which is why Unavailable exists.
//
// An empty View renders as a present Director that believes nothing — a real and very
// different state, and the one somebody is most likely to misread as "everything is fine".
func TestTheZeroValueIsNotMistakenForAWorkingDirector(t *testing.T) {
	var zero playbill.View
	if zero.Reach.Live() {
		t.Fatal("the zero value reports a live Director")
	}
}

// ── the register: no hedge is ever upgraded ───────────────────────────────────

// Every recognition verdict short of `recognised` reads as uncertainty.
//
// The most damaging bug this milestone could ship is a sentence that sounds more certain
// than the verdict behind it, because it would look like progress.
func TestUncertainVerdictsNeverReadAsCertainty(t *testing.T) {
	hedged := map[playbill.Recognition]bool{
		playbill.Candidate: true, playbill.Ambiguous: true,
		playbill.Unknown: true, playbill.Contested: true, playbill.Unobservable: true,
	}
	for verdict := range hedged {
		v := present()
		v.Current.Recognition = verdict
		v.Current.Screen = "the pause menu"

		text := renderText(v.Watch())
		if strings.Contains(text, "I recognise this as") {
			t.Errorf("%s rendered as recognition:\n%s", verdict, text)
		}
		if !mentionsDoubt(text) {
			t.Errorf("%s rendered with no hedge at all:\n%s", verdict, text)
		}
	}

	// And the one verdict that IS certainty says so.
	v := present()
	v.Current.Recognition = playbill.Recognised
	v.Current.Screen = "the pause menu"
	if !strings.Contains(renderText(v.Watch()), "I recognise this as “the pause menu”") {
		t.Error("an established match did not read as one")
	}
}

func mentionsDoubt(s string) bool {
	for _, w := range []string{"not certain", "can't tell", "don't recognise",
		"can't make out", "doesn't match"} {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// No percentage reaches a person. Discrete verdicts are the whole point.
func TestNoConfidencePercentageReachesWatch(t *testing.T) {
	v := present()
	v.Thinking.Readings = []playbill.Reading{{
		Says: "This might be a menu.", Standing: playbill.Supported,
		Because: "four controls were present in 40 of 41 looks",
	}}
	text := renderText(v.Watch())
	for _, bad := range []string{"%", "confidence", "0.", "score"} {
		if strings.Contains(strings.ToLower(text), bad) {
			t.Errorf("Watch rendered %q:\n%s", bad, text)
		}
	}
}

// No implementation identifier reaches Watch.
func TestNoInternalIdentifierReachesWatch(t *testing.T) {
	v := present()
	v.Question = &playbill.Question{
		ID: "q_9f3a1c", Asks: "Is this a settings screen?", Wants: playbill.WantsChoice,
		Via: playbill.ViaProposal, Answers: []string{"confirmed", "declined"},
	}
	text := renderText(v.Watch())
	for _, bad := range []string{"q_9f3a1c", "state_", "shadow_", "group_", "subject_",
		"fingerprint", "digest"} {
		if strings.Contains(text, bad) {
			t.Errorf("Watch rendered the internal identifier %q. A person watching Marco "+
				"must not have to know the implementation to read it:\n%s", bad, text)
		}
	}
	// The id is still CARRIED, because the ordinary answer path needs it.
	if v.Question.ID == "" {
		t.Error("the question lost the id its answer routes by")
	}
}

// ── NORMAL is total ───────────────────────────────────────────────────────────

// Every state reduces to a headline, so a consumer surface never has to handle
// "none of the above".
func TestEveryStateReducesToAHeadline(t *testing.T) {
	phases := []playbill.Phase{
		playbill.NotDoing, playbill.AwaitingPermission, playbill.CheckingStart,
		playbill.Performing, playbill.CheckingResult, playbill.Succeeded,
		playbill.Unverified, playbill.Failed, playbill.Refused, playbill.Cancelled,
	}
	stages := []playbill.Stage{
		playbill.NotLearning, playbill.Observing, playbill.AwaitingEvidence,
		playbill.Asking, playbill.Capturing, playbill.Comparing,
		playbill.RehearsalOffered, playbill.Rehearsing, playbill.Rehearsed,
		playbill.PlayAvailable,
	}
	for _, phase := range phases {
		for _, stage := range stages {
			v := present()
			v.Doing.Phase, v.Learning.Stage = phase, stage
			if h := v.Normal(); strings.TrimSpace(h.Word) == "" {
				t.Fatalf("phase %q + stage %q reduced to nothing", phase, stage)
			}
			if err := v.Admit(); err != nil {
				t.Fatalf("phase %q + stage %q was refused: %v", phase, stage, err)
			}
		}
	}
}

// A pending question outranks everything, because it is the only thing waiting on a person.
func TestAPendingQuestionIsWhatPullsAConsumerSurfaceForward(t *testing.T) {
	v := present()
	v.Doing.Phase = playbill.Performing
	v.Question = &playbill.Question{
		ID: "q_1", Asks: "Is this the pause menu?", Wants: playbill.WantsChoice,
		Via: playbill.ViaProposal, Answers: []string{"confirmed"},
	}
	h := v.Normal()
	if !h.Attention {
		t.Fatal("a pending question did not ask for attention")
	}
	if h.Detail != "Is this the pause menu?" {
		t.Errorf("the consumer surface did not show the question: %q", h.Detail)
	}

	// And ordinary watching does not.
	quiet := present()
	if quiet.Normal().Attention {
		t.Error("simply watching asked for the person's attention")
	}
}

// ── the same value drives all three readings ──────────────────────────────────

// Watch, Deep and Normal are functions of ONE value, and Deep begins with Watch.
//
// The architectural claim of the milestone, asserted rather than asserted-in-a-comment:
// there is no path by which the diagnostics reading could describe a different state
// from the human one.
func TestDeepIsWatchPlusEvidence(t *testing.T) {
	v := present()
	v.Diagnostics = &playbill.Diagnostics{
		Providers: []playbill.Provider{{Name: "uia", Available: true, Observations: 812}},
		Fusion:    playbill.Fusion{Observations: 812, Elements: 61, ProvenanceOK: true},
	}
	watch, deep := renderText(v.Watch()), renderText(v.Deep())
	if !strings.HasPrefix(deep, watch) {
		t.Fatal("Diagnostics is not Watch plus evidence. Two readings that can diverge " +
			"are two readings somebody has to compare by memory")
	}
	if !strings.Contains(deep, "812") {
		t.Error("Diagnostics carried no evidence")
	}
	// Watch itself never shows the machinery.
	if strings.Contains(watch, "fusion") || strings.Contains(watch, "provenance") {
		t.Error("Watch leaked the machinery")
	}
}

// A provider's own numeric metric is allowed in Diagnostics, LABELLED as the provider's
// own — and only then.
func TestAProviderScoreMustSayWhoseItIs(t *testing.T) {
	v := present()
	v.Diagnostics = &playbill.Diagnostics{
		Providers: []playbill.Provider{{Name: "vision", Available: true, Score: 0.82}},
	}
	if err := v.Admit(); err == nil {
		t.Fatal("a bare number was admitted with no metric name. That is exactly how a " +
			"detector's own output becomes 'confidence' in a reader's head")
	}
	v.Diagnostics.Providers[0].Metric = "detection_threshold"
	if err := v.Admit(); err != nil {
		t.Fatalf("a labelled provider metric was refused: %v", err)
	}
	if !strings.Contains(renderText(v.Deep()), "vision's own") {
		t.Error("a provider metric was not attributed to the provider")
	}
}

// ── the wire ──────────────────────────────────────────────────────────────────

// The account survives a JSON round trip unchanged, because that is how it reaches
// every surface that renders it.
func TestThePlaybillSurvivesTheWire(t *testing.T) {
	want := present()
	want.Question = &playbill.Question{
		ID: "q_1", Asks: "Is this a settings screen?", Wants: playbill.WantsChoice,
		Via: playbill.ViaProposal, Answers: []string{"confirmed", "declined"},
	}
	want = want.WithDigest()

	var got playbill.View
	if err := json.Unmarshal(mustJSON(t, want), &got); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if got.Digest != want.Digest {
		t.Errorf("digest changed across the wire: %s -> %s", want.Digest, got.Digest)
	}
	if renderText(got.Watch()) != renderText(want.Watch()) {
		t.Error("the account rendered differently after a round trip. The overlay and " +
			"the CLI decode the same bytes; if this differs they are two accounts")
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// present is an ordinary, healthy account: watching something it recognises.
func present() playbill.View {
	return playbill.View{
		Version: playbill.Version,
		Reach:   playbill.Present,
		Epoch:   "epoch_1",
		TakenAt: time.Date(2026, 8, 10, 12, 4, 11, 0, time.UTC),
		Current: playbill.Current{
			Watching: true, Application: "testgame.exe",
			Recognition: playbill.Recognised, Screen: "the pause menu",
			Samples: 41, FreshnessMS: 320,
		},
		Seeing: playbill.Seeing{
			Structure: 5, Looks: 41, Readable: 7, Unrecognised: 12,
			Terms: []string{"settings", "audio"}, Sources: []string{"button", "icon"},
		},
		Learning: playbill.Learning{Stage: playbill.Observing},
		Doing:    playbill.Doing{Phase: playbill.NotDoing},
		Cursor:   40,
	}
}

func renderText(lines []playbill.Line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(strings.Repeat("  ", l.Indent))
		b.WriteString(l.Text)
		b.WriteString("\n")
	}
	return b.String()
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	return b
}
