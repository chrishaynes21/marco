package invoke_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// everySource is the whole list, so a test that sweeps it cannot quietly stop covering one.
var everySource = []invoke.Source{
	invoke.SourceTyped, invoke.SourceSpoken, invoke.SourceHotkey,
	invoke.SourceControlCentre, invoke.SourceCLI, invoke.SourceWeb,
}

// fakePlays answers for a fixed set of names and RECORDS every question asked of it.
//
// The recording is the point: "an explicit identity is never looked up again" is a claim about a
// call that must not happen, and the only way to prove a call did not happen is to count.
type fakePlays struct {
	known map[string]routes.Route
	asked []string
}

func (f *fakePlays) Resolve(app, name string) (routes.Route, bool) {
	f.asked = append(f.asked, app+"|"+name)
	rt, ok := f.known[routes.Slug(name)]
	return rt, ok
}

func newPlays(rts ...routes.Route) *fakePlays {
	f := &fakePlays{known: map[string]routes.Route{}}
	for _, rt := range rts {
		f.known[rt.Slug] = rt
	}
	return f
}

var mouse = routes.Route{App: "settings", Focus: true, Slug: "open-mouse-settings"}

// The same words from every entrance reach the same verdict.
//
// # The failure this guards
//
// Typing "Open Mouse Settings" ran the Play. Saying it went to Director and was reinterpreted
// against the screen, because the spoken branch sat in front of Play lookup and never consulted
// the registry at all. One request, two meanings, decided by which device the person used.
//
// Mutation: read req.Source anywhere in Decide. This fails.
func TestSourceCannotChangeTheDecision(t *testing.T) {
	cases := []struct {
		name string
		req  invoke.Request
		want invoke.Kind
	}{
		{"an exact play", invoke.Request{Text: "Open Mouse Settings", App: "discord"}, invoke.KindPlay},
		{"an unknown request", invoke.Request{Text: "turn bluetooth off", App: "settings"}, invoke.KindDirector},
		{"a control phrase", invoke.Request{Text: "stop"}, invoke.KindControl},
		{"an explicit identity", invoke.Request{Text: "anything at all", Play: &mouse}, invoke.KindPlay},
		{"an answer to a question", invoke.Request{Text: "the second one", Pending: true}, invoke.KindDirector},
		{"a play name while a question is pending", invoke.Request{Text: "Open Mouse Settings", Pending: true}, invoke.KindPlay},
	}
	for _, c := range cases {
		var first invoke.Decision
		for i, src := range everySource {
			req := c.req
			req.Source = src
			got := invoke.Decide(newPlays(mouse), nil, req)
			if got.Kind != c.want {
				t.Errorf("%s from %s: decided %q, want %q", c.name, src, got.Kind, c.want)
			}
			if i == 0 {
				first = got
				continue
			}
			// Not just the same KIND — the same verdict. A source that changed which
			// play, or what Director was told, would be the same defect wearing a hat.
			if got.Kind != first.Kind || got.Play != first.Play ||
				got.Phrase != first.Phrase || got.Explicit != first.Explicit {
				t.Errorf("%s: %s decided %+v, %s decided %+v — transport changed meaning",
					c.name, everySource[0], first, src, got)
			}
		}
	}
}

// Stop acts on what is running, from every entrance, whatever else is true.
//
// Typing "stop" used to find no play, miss, and be offered as something to record a demonstration
// of. Only a SPOKEN stop reached the Director. A person watching Marco do the wrong thing had to
// remember which device stops it.
func TestStopIsNeverOrdinaryText(t *testing.T) {
	// Even against a registry that HAS a play called stop, and while a question is pending,
	// and with an explicit identity attached — control still wins.
	stopPlay := routes.Route{Slug: "stop"}
	plays := newPlays(stopPlay, mouse)
	for _, word := range []string{"stop", "Stop", "STOP", "stop.", "stop it", "cancel", "cancel that", "abort", "halt", "  stop  "} {
		for _, src := range everySource {
			for _, extra := range []invoke.Request{
				{},
				{Pending: true},
				{Play: &mouse},
				{Pending: true, Play: &mouse},
			} {
				req := extra
				req.Text, req.Source = word, src
				got := invoke.Decide(plays, nil, req)
				if got.Kind != invoke.KindControl {
					t.Fatalf("%q from %s (pending=%v explicit=%v) decided %q — a stop that "+
						"is not a stop cannot stop anything",
						word, src, req.Pending, req.Play != nil, got.Kind)
				}
			}
		}
	}
	// And an ordinary phrase that merely CONTAINS stop is not a control phrase.
	if got := invoke.Decide(plays, nil, invoke.Request{Text: "stop the music"}); got.Kind == invoke.KindControl {
		t.Error("\"stop the music\" was taken as a cancellation")
	}
}

// A phrase Marco already has a Play for is performed, never interpreted.
func TestAnExactlyKnownPlayIsNeverInterpreted(t *testing.T) {
	plays := newPlays(mouse)
	// Every spelling routes.Slug already folds onto one identity. These are not new
	// normalization rules — they are the ones the store has always used, finally consulted
	// before anything else looks at the words.
	for _, text := range []string{
		"open mouse settings",
		"Open Mouse Settings",
		"OPEN MOUSE SETTINGS",
		"open-mouse-settings",
		"Open Mouse Settings!",
		"open mouse settings?",
		`"open mouse settings"`,
		"   open mouse settings   ",
		"open  mouse  settings",
	} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: text, Source: invoke.SourceSpoken, App: "discord"})
		if got.Kind != invoke.KindPlay {
			t.Errorf("%q decided %q — Director was asked about a play Marco already has", text, got.Kind)
			continue
		}
		if got.Play != mouse {
			t.Errorf("%q resolved to %+v, want %+v", text, got.Play, mouse)
		}
	}
}

// A near miss is a miss, and a miss belongs to Director.
//
// Play lookup must not become a second semantic planner. The intake used to fuzz a phrase to a
// slug at a 0.75 score — and then hand it to an optional external model — BEFORE the registry was
// consulted, so an exact durable identity could lose to a fuzzy neighbour with no confirmation.
//
// Mutation: make the lookup fuzzy. This fails.
func TestANearMissIsAMissAndGoesToDirector(t *testing.T) {
	// The fixture is built so that a guesser has something to find. "settings" and "open" are
	// real plays, so any matcher that reaches for a word, a prefix or a contained substring
	// lands on one — and a test whose registry offers nothing to grab at would pass against a
	// fuzzy intake by luck rather than by rule.
	plays := newPlays(
		routes.Route{Slug: "open-settings"},
		routes.Route{Slug: "open-the-settings"},
		routes.Route{Slug: "settings"},
		routes.Route{Slug: "open"},
	)
	for _, text := range []string{
		"open setting",          // a typo
		"open the settings now", // extra words
		"please open settings",  // politeness
		"open settings for me",  // a real play name plus a tail
		"just settings",         // a real play name plus a head
		"open the file menu",    // shares words with two plays and means neither
	} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: text})
		if got.Kind != invoke.KindDirector {
			t.Errorf("%q decided %q (play %q) — a guess was made where Director should have "+
				"read the screen", text, got.Kind, got.Play.Slug)
		}
	}
	// And the two neighbours each still answer to their own exact name, so the strictness
	// costs nothing that was working.
	for _, c := range []struct{ text, slug string }{
		{"open settings", "open-settings"},
		{"open the settings", "open-the-settings"},
		{"settings", "settings"},
		{"open", "open"},
	} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: c.text})
		if got.Kind != invoke.KindPlay || got.Play.Slug != c.slug {
			t.Errorf("%q decided %q/%q, want a play %q", c.text, got.Kind, got.Play.Slug, c.slug)
		}
	}
}

// A surface that already knows which Play does not have to say so in words.
//
// Mutation: drop Request.Play and resolve d.Text instead. This fails on the recorded calls.
func TestAnExplicitPlayIsNeverLookedUpAgain(t *testing.T) {
	// A registry that would answer DIFFERENTLY if it were asked, so a re-lookup cannot pass
	// by coincidence.
	other := routes.Route{App: "notepad", Slug: "open-mouse-settings"}
	plays := newPlays(other)

	got := invoke.Decide(plays, nil, invoke.Request{
		Text: "open mouse settings", Source: invoke.SourceControlCentre, App: "notepad",
		Play: &mouse,
	})
	if got.Kind != invoke.KindPlay {
		t.Fatalf("an explicit identity decided %q", got.Kind)
	}
	if got.Play != mouse {
		t.Fatalf("the surface said %+v and the intake performed %+v — identity was "+
			"re-guessed from the words", mouse, got.Play)
	}
	if !got.Explicit {
		t.Error("the decision does not record that the identity was explicit")
	}
	if len(plays.asked) != 0 {
		t.Fatalf("the registry was asked %v — an identity the surface already held was "+
			"turned back into text and looked up again", plays.asked)
	}
}

// While Director is waiting to be told which one, an ANSWER is the answer.
//
// It must not be resolved against the registry: "the second one" is a choice between things
// Director is holding, not the name of anything.
func TestAnAnswerToAPendingQuestionIsNotAPlay(t *testing.T) {
	// Registered under the exact words of an answer, so a registry lookup would find
	// something if one were made. ("cancel" is not in this list: it is a CONTROL phrase and
	// arm 1 claims it before a question is ever considered.)
	plays := newPlays(mouse, routes.Route{Slug: "the-second-one"})
	for _, text := range []string{"the second one", "the first", "1"} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: text, Pending: true})
		if got.Kind != invoke.KindDirector {
			t.Errorf("%q with a question pending decided %q — the question was discarded and "+
				"something else started", text, got.Kind)
		}
		if got.Phrase != text {
			t.Errorf("the answer reached Director as %q, want %q unchanged", got.Phrase, text)
		}
	}
}

// A question nobody answered does not capture everything said afterwards.
//
// # The failure this guards, measured live
//
// The first version of this claimed the words whenever ANY question was outstanding. A question
// left behind by some earlier command then hijacked every invocation after it: typing the name of
// a play you have got you `nothing matching "test" is present in the observed window`, because the
// words had been delivered as an answer to a question you had forgotten asking. Nothing said why,
// and the only way out was to answer or cancel a question you could no longer see.
//
// The rule is not new judgement. `intent.ParseClarification` is the same function the service runs
// on the other side, and it is strict: one word it does not recognise and the phrase is not an
// answer.
//
// Mutation: capture the words whenever Pending is set. This fails.
func TestAStalePendingQuestionDoesNotHijackAKnownPlay(t *testing.T) {
	plays := newPlays(mouse)
	for _, text := range []string{
		"open mouse settings", "Open Mouse Settings", "open-mouse-settings",
	} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: text, Pending: true})
		if got.Kind != invoke.KindPlay {
			t.Errorf("%q with a stale question pending decided %q — a play Marco has was "+
				"delivered to Director as an answer", text, got.Kind)
		}
		if got.Play != mouse {
			t.Errorf("%q resolved to %+v", text, got.Play)
		}
	}
	// A request Marco has no play for still reaches Director, question or no question.
	if got := invoke.Decide(plays, nil, invoke.Request{Text: "turn bluetooth off", Pending: true}); got.Kind != invoke.KindDirector {
		t.Errorf("an unknown request decided %q", got.Kind)
	}
	// And a control phrase still beats everything, including a pending question.
	if got := invoke.Decide(plays, nil, invoke.Request{Text: "stop", Pending: true}); got.Kind != invoke.KindControl {
		t.Errorf("stop with a question pending decided %q", got.Kind)
	}
}

// Director gets the Audience's sentence, not a rewrite of it.
func TestDirectorReceivesTheWordsAsSpoken(t *testing.T) {
	plays := newPlays(mouse)
	for _, text := range []string{
		"turn bluetooth off",
		"Click the File menu",
		"open dad's settings",
		"what's on screen?",
	} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: text})
		if got.Kind != invoke.KindDirector {
			t.Fatalf("%q decided %q", text, got.Kind)
		}
		if got.Phrase != text {
			t.Errorf("Director was told %q; the person said %q. Rewriting a sentence "+
				"changes what it asks for.", got.Phrase, text)
		}
	}
	// Only surrounding whitespace is taken off, because it is not part of anything.
	if got := invoke.Decide(plays, nil, invoke.Request{Text: "  turn bluetooth off  "}); got.Phrase != "turn bluetooth off" {
		t.Errorf("phrase %q", got.Phrase)
	}
}

// Two plays can share a name across scopes; which one answers is the store's settled rule, not
// this package's, and never a map's iteration order.
func TestAmbiguityIsResolvedByTheStoresRuleNotByOrder(t *testing.T) {
	inApp := routes.Route{App: "settings", Slug: "open-mouse-settings"}
	elsewhere := routes.Route{App: "notepad", Focus: true, Slug: "open-mouse-settings"}
	global := routes.Route{Slug: "open-mouse-settings"}

	// A real registry, so the answer is the product's documented scope priority rather than
	// anything this test arranged.
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}
	for _, rt := range []routes.Route{inApp, elsewhere, global} {
		if err := reg.Save(rt, "script main...\n  do nothing.\n"); err != nil {
			t.Fatal(err)
		}
	}
	// Same question, many times: the answer may not wander.
	first := invoke.Decide(reg, nil, invoke.Request{Text: "open mouse settings", App: "settings"})
	if first.Kind != invoke.KindPlay {
		t.Fatalf("decided %q", first.Kind)
	}
	for i := 0; i < 50; i++ {
		got := invoke.Decide(reg, nil, invoke.Request{Text: "open mouse settings", App: "settings"})
		if got.Play != first.Play {
			t.Fatalf("the same invocation chose %+v then %+v — the answer depends on "+
				"iteration order", first.Play, got.Play)
		}
	}
	if first.Play != inApp {
		t.Errorf("in-app invocation chose %+v, want the application's own play %+v",
			first.Play, inApp)
	}
	// And from elsewhere it is still deterministic.
	away := invoke.Decide(reg, nil, invoke.Request{Text: "open mouse settings", App: "discord"})
	for i := 0; i < 50; i++ {
		if got := invoke.Decide(reg, nil, invoke.Request{Text: "open mouse settings", App: "discord"}); got.Play != away.Play {
			t.Fatal("the from-elsewhere answer wanders")
		}
	}
}

// A staged play cannot answer an invocation, because the resolver cannot see it.
func TestAStagedPlayDoesNotInterceptAnInvocation(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}
	rt := routes.Route{App: "settings", Focus: routes.LearnedFocus, Slug: "open-bluetooth"}
	err := reg.SaveStaged(rt, "script main...\n  do nothing.\n",
		routes.Origin{Kind: routes.KindLearned, Application: "settings"})
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range everySource {
		got := invoke.Decide(reg, nil, invoke.Request{Text: "open bluetooth", Source: src, App: "settings"})
		if got.Kind != invoke.KindDirector {
			t.Fatalf("a saved-but-unregistered play answered a %s invocation as %q", src, got.Kind)
		}
	}
	// Registering it is what makes it answer.
	if err := reg.Register(rt); err != nil {
		t.Fatal(err)
	}
	if got := invoke.Decide(reg, nil, invoke.Request{Text: "open bluetooth", App: "discord"}); got.Kind != invoke.KindPlay {
		t.Fatal("a registered play still does not answer")
	}
}

// A play whose own name contains the invocation grammar is still reachable by that name.
//
// The grammar (" then " for a chain, " with " for arguments) used to be applied to the whole
// phrase before anything consulted the registry, so a play called "wait then click" was split
// into two commands, neither of which existed, and the person was offered the chance to learn it.
func TestAPlayNamedWithTheGrammarIsStillReachable(t *testing.T) {
	plays := newPlays(
		routes.Route{Slug: "wait-then-click"},
		routes.Route{Slug: "log-in-with-google"},
	)
	for _, c := range []struct{ text, slug string }{
		{"wait then click", "wait-then-click"},
		{"log in with google", "log-in-with-google"},
	} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: c.text})
		if got.Kind != invoke.KindPlay || got.Play.Slug != c.slug {
			t.Errorf("%q decided %q/%q — its own name was read as a chain or an argument "+
				"list before anybody checked whether it was a play", c.text, got.Kind, got.Play.Slug)
		}
	}
}

// The trace can answer the questions an acceptance run asks of it, and keeps no store.
func TestTheTraceSaysHowAnInvocationWasRouted(t *testing.T) {
	plays := newPlays(mouse)
	for _, c := range []struct {
		req  invoke.Request
		want []string
	}{
		{invoke.Request{Text: "open mouse settings", Source: invoke.SourceSpoken, App: "discord"},
			[]string{"source=spoken", "decision=play", "play=open-mouse-settings"}},
		{invoke.Request{Text: "turn bluetooth off", Source: invoke.SourceTyped},
			[]string{"source=typed", "decision=director", "phrase="}},
		{invoke.Request{Text: "stop", Source: invoke.SourceCLI},
			[]string{"source=cli", "decision=control"}},
		{invoke.Request{Text: "x", Source: invoke.SourceControlCentre, Play: &mouse},
			[]string{"decision=play", "explicit=yes"}},
	} {
		got := invoke.Decide(plays, nil, c.req).Trace(c.req)
		for _, want := range c.want {
			if !strings.Contains(got, want) {
				t.Errorf("trace %q is missing %q", got, want)
			}
		}
		if !strings.Contains(got, "why=") {
			t.Errorf("trace %q does not say which rule fired", got)
		}
	}
}

// Nothing said is not a control phrase and is not a play.
func TestNothingSaidDecidesNothing(t *testing.T) {
	plays := newPlays(mouse)
	for _, text := range []string{"", "   "} {
		got := invoke.Decide(plays, nil, invoke.Request{Text: text})
		if got.Kind != invoke.KindDirector || got.Phrase != "" {
			t.Errorf("%q decided %+v", text, got)
		}
		if len(plays.asked) != 0 {
			t.Errorf("an empty phrase was looked up: %v", plays.asked)
		}
	}
}

// ── what the person showed Marco by hand ─────────────────────────────────────

// fakeSights answers for a fixed set of demonstration names and RECORDS every question.
//
// Recorded for the same reason `fakePlays` records: the claims that matter here are about calls
// that must NOT happen. A control phrase, a pending answer, an explicit identity and a name that
// already resolves to a play must each be decided without this being asked anything at all.
type fakeSights struct {
	known map[string]invoke.Demonstrated
	asked []string
}

func (f *fakeSights) Demonstration(app, name string) (invoke.Demonstrated, bool) {
	f.asked = append(f.asked, app+"|"+name)
	// CASE-INSENSITIVELY, which is what the real store does: a person asking for
	// "ark graphics live" is asking for the thing they called "Ark Graphics Live".
	d, ok := f.known[strings.ToLower(strings.TrimSpace(name))]
	return d, ok
}

func newSights(ds ...invoke.Demonstrated) *fakeSights {
	f := &fakeSights{known: map[string]invoke.Demonstrated{}}
	for _, d := range ds {
		f.known[strings.ToLower(d.Name)] = d
	}
	return f
}

var arkGraphics = invoke.Demonstrated{Name: "ark graphics live", Application: "arkascended"}

// A NAME SOMEBODY DEMONSTRATED IS PERFORMED BY SIGHT.
//
// # The failure this guards
//
// A demonstration is the half of the product a person makes with their own hands: they show
// Marco a game menu, it saves the screens, the control and what the click did. Typing that name
// back reached arm five — Director — which read the words against the screen and reported that
// nothing matching them was present in the observed window. It was present. It had been pointed
// at. The words went to the half that guesses rather than the half that was told.
//
// Mutation: delete the arm, or move it behind Director. This fails.
func TestADemonstrationIsPerformedBySight(t *testing.T) {
	plays, sights := newPlays(mouse), newSights(arkGraphics)
	for _, text := range []string{"ark graphics live", "Ark Graphics Live", "  ark graphics live  "} {
		got := invoke.Decide(plays, sights, invoke.Request{Text: text,
			Source: invoke.SourceTyped, App: "arkascended"})
		if got.Kind != invoke.KindDemonstration {
			t.Fatalf("%q decided %q (%s), want a demonstration",
				text, got.Kind, got.Why)
		}
		// AND THE NAME IS THE STORE'S, not the words. The runner looks it up again,
		// and a name that reached it as typed would be a second chance to miss.
		if got.Demonstration.Name != arkGraphics.Name {
			t.Errorf("%q carried the name %q", text, got.Demonstration.Name)
		}
		// AND THE APPLICATION COMES WITH IT. Without it the runner watches the
		// foreground, and at the moment somebody types a name the foreground is
		// whatever they typed it into.
		if got.Demonstration.Application != arkGraphics.Application {
			t.Errorf("%q carried the application %q", text,
				got.Demonstration.Application)
		}
	}
	// AND IT SHOWS IN THE TRACE, which is how an acceptance run says where words went.
	line := invoke.Decide(plays, sights, invoke.Request{Text: "ark graphics live",
		Source: invoke.SourceSpoken}).Trace(invoke.Request{Source: invoke.SourceSpoken})
	for _, want := range []string{"decision=demonstration", `demonstration="ark graphics live"`} {
		if !strings.Contains(line, want) {
			t.Errorf("the trace %q does not carry %q", line, want)
		}
	}
}

// AND A PLAY OF THE SAME NAME STILL WINS, WITHOUT THE DEMONSTRATION BEING ASKED ABOUT.
//
// The order is the compatibility guarantee: every name that runs something today runs the same
// thing tomorrow. A person who has a play called "open the map" and later demonstrates something
// under that name has not silently replaced what the words do — and the proof is that the
// demonstration store is never even consulted.
func TestAPlayStillWinsOverADemonstrationOfTheSameName(t *testing.T) {
	both := routes.Route{App: "arkascended", Slug: "ark-graphics-live"}
	plays, sights := newPlays(both), newSights(arkGraphics)
	got := invoke.Decide(plays, sights, invoke.Request{Text: "ark graphics live"})
	if got.Kind != invoke.KindPlay {
		t.Fatalf("decided %q (%s); a name that already runs a play must keep "+
			"running it", got.Kind, got.Why)
	}
	if len(sights.asked) != 0 {
		t.Errorf("the demonstration store was asked %v for a name that resolved "+
			"to a play, so the two stores are being consulted in parallel "+
			"rather than in order", sights.asked)
	}
}

// AND NOTHING ABOVE THE ARM CONSULTS IT.
//
// Each of these is decided by an arm above five, and each one asking would be a bug of its own:
// "stop" looked up as a demonstration is a stop that went shopping, "the second one" looked up is
// an answer being matched as a name, and an explicit identity looked up is the lookup that
// ADR-029's door exists to make unnecessary.
func TestADemonstrationIsNotConsultedAboveItsArm(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  invoke.Request
	}{
		{"a control phrase", invoke.Request{Text: "stop"}},
		{"an answer to a question", invoke.Request{Text: "the second one", Pending: true}},
		{"an identity the surface holds", invoke.Request{Text: "ark graphics live",
			Play: &mouse}},
		{"nothing said at all", invoke.Request{Text: "   "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sights := newSights(arkGraphics)
			invoke.Decide(newPlays(), sights, tc.req)
			if len(sights.asked) != 0 {
				t.Errorf("the demonstration store was asked %v",
					sights.asked)
			}
		})
	}
}

// AND AN ENTRANCE WITH NO SIGHT OF DEMONSTRATIONS STILL ANSWERS.
//
// `nil` is not a crash and not a refusal: it is an entrance that cannot see that store, and the
// words then belong to Director exactly as they did before the arm existed. The production
// wiring is held where production is, beside the intake.
func TestAnEntranceWithNoSightOfDemonstrationsFallsThroughToDirector(t *testing.T) {
	got := invoke.Decide(newPlays(), nil, invoke.Request{Text: "ark graphics live"})
	if got.Kind != invoke.KindDirector {
		t.Fatalf("decided %q (%s)", got.Kind, got.Why)
	}
}
