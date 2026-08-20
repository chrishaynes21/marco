package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// ============================================================================
// THE BRIDGE — a learned play is PERFORMED by the Director, not RUN here.
//
// Every test enters through dispatchDo, the function `marco do` actually calls.
// Nothing here rebuilds the wiring: deleting the fork in dispatchDo, or the call
// to performLearned, must fail these — which is the whole point of entering
// through production (docs/Wiring-Tests.md).
//
// No test starts a Director. dialPerformer is swapped for a fake endpoint, so a
// failure here can never be a real click on the real desktop.
// ============================================================================

// fakeDirector is a Director that records what it was asked and answers as told.
type fakeDirector struct {
	asked  []service.PerformQuery
	view   service.PerformView
	err    error
	closed bool
}

func (f *fakeDirector) Perform(q service.PerformQuery) (service.PerformView, error) {
	f.asked = append(f.asked, q)
	return f.view, f.err
}

func (f *fakeDirector) Close() error { f.closed = true; return nil }

// useFakeDirector points the bridge at a fake endpoint for one test.
//
// Mandatory in every test that can reach the fork: the production dialler AUTO-STARTS
// director.exe, and a test that performs real input is the one thing this suite must never do.
func useFakeDirector(t *testing.T, f *fakeDirector) *fakeDirector {
	t.Helper()
	prev := dialPerformer
	dialPerformer = func() (directorPerformer, error) { return f, nil }
	t.Cleanup(func() { dialPerformer = prev })
	return f
}

// refuseToDial makes the Director unreachable, with the reason it gave for refusing to start.
func refuseToDial(t *testing.T, because error) {
	t.Helper()
	prev := dialPerformer
	dialPerformer = func() (directorPerformer, error) { return nil, because }
	t.Cleanup(func() { dialPerformer = prev })
}

// allowGate is the authority door saying yes, so these tests are about the FORK and not about
// ADR-029. The door's own behaviour is covered by authoritybypass_test.go.
type allowGate struct{}

func (allowGate) Allow(orchestrator.Resolved) orchestrator.Decision {
	return orchestrator.Decision{Verdict: orchestrator.Allowed,
		Reason: orchestrator.ReasonLearnedFirstUse}
}

// learnedWorld is a registered learned play with intact provenance, wired the way `marco do`
// wires itself: a recording host in place of the OS, and a recogniser that would let the play
// run perfectly well HERE — so nothing but the fork can be the reason it does not.
func learnedWorld(t *testing.T) (orchestrator.Deps, *recordHost, *strings.Builder) {
	t.Helper()
	return worldWithPlay(t, true)
}

// ordinaryWorld is the same play saved with NO provenance: an authored play.
func ordinaryWorld(t *testing.T) (orchestrator.Deps, *recordHost, *strings.Builder) {
	t.Helper()
	return worldWithPlay(t, false)
}

func worldWithPlay(t *testing.T, learned bool) (orchestrator.Deps, *recordHost, *strings.Builder) {
	t.Helper()
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu", "controller settings",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "mute-volume"}
	if learned {
		if err := reg.SaveWithOrigin(rt, src, routes.Origin{
			Kind: routes.KindLearned, Application: "testgame",
			From: "subj_a", To: "subj_b", Sequence: 1, Evidence: "e1",
		}); err != nil {
			t.Fatalf("saving: %v", err)
		}
		if r := orchestrator.Classify(reg, rt, "mute-volume"); !r.Learned() {
			t.Fatalf("fixture is not a learned play (kind=%s prov=%s)", r.Kind, r.Provenance)
		}
	} else if err := reg.Save(rt, src); err != nil {
		t.Fatalf("saving: %v", err)
	}
	host := &recordHost{}
	out := &strings.Builder{}
	d := orchestrator.Deps{
		Reg: reg,
		Hosts: map[string]runtime.Host{"*": host,
			"Screen": screenhost.New(&recognisingStage{app: "testgame", current: "subj_a"})},
		In:        strings.NewReader(""),
		Out:       out,
		App:       func() string { return "testgame" },
		Authority: allowGate{},
	}
	return d, host, out
}

// arrived is a Director that walked the whole route and confirmed where it ended.
func arrived(steps int) service.PerformView {
	v := service.PerformView{Application: "testgame", Goal: "mute volume",
		From: "subj_a", To: "subj_b", Arrived: true, Say: "Done."}
	for i := 0; i < steps; i++ {
		v.Steps = append(v.Steps, service.PerformStep{From: "subj_a", To: "subj_b", Verified: true})
	}
	return v
}

// ── M5: a learned play must not be run locally ───────────────────────────────

// THE FORK. A learned play with intact provenance goes to the Director.
//
// # The failure this exists to catch
//
// `marco` has no perception. A learned play's first line asks the Screen whether the place it
// begins on is showing; this process answers "I could not check", the play refuses at line one,
// and the GENERATED play catches that refusal and logs it — so the process exits 0 and the
// overlay reports success for a play that never ran.
//
// Mutation this kills (M5): delete the `if resolved.Learned()` branch in dispatchDo, so a
// learned Play is invoked locally and bypasses the Director entirely.
func TestALearnedPlayIsPerformedByTheDirector(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d, host, _ := learnedWorld(t)
	fake := useFakeDirector(t, &fakeDirector{view: arrived(1)})

	if _, err := doAsProduct(t, d, "mute-volume", nil, nil); err != nil {
		t.Fatalf("marco do: %v", err)
	}

	if len(fake.asked) != 1 {
		t.Fatalf("the Director was asked %d times, want once.\n"+
			"A learned play was invoked and the Director never heard about it — so it ran "+
			"in a process with no perception, refused at its own first line, and exited 0.",
			len(fake.asked))
	}
	if got := fake.asked[0].Name; !strings.EqualFold(got, "mute volume") {
		t.Errorf("the Director was asked for %q, want the outcome's own words", got)
	}
	if got := fake.asked[0].Application; got != "testgame" {
		t.Errorf("the request was scoped to %q, want the sidecar's application", got)
	}
	if pressed := host.pressed(); len(pressed) != 0 {
		t.Fatalf("the learned play ALSO ran locally and pressed %v; delegation is instead of "+
			"running here, never as well as", pressed)
	}
	if !fake.closed {
		t.Error("the connection to the Director was left open")
	}
}

// ── the fork must not swallow ordinary plays ─────────────────────────────────

// An authored or taught play still runs HERE, exactly as it always has.
//
// The fork splits on PROVENANCE, and a fork that delegated everything would take every play a
// person wrote away from the engine that runs it.
func TestAnOrdinaryPlayIsNotDelegated(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d, host, _ := ordinaryWorld(t)
	fake := useFakeDirector(t, &fakeDirector{view: arrived(1)})

	if _, err := doAsProduct(t, d, "mute-volume", nil, nil); err != nil {
		t.Fatalf("marco do: %v", err)
	}
	if len(fake.asked) != 0 {
		t.Fatalf("an authored play was sent to the Director (%d request(s)); the Director has "+
			"no record of a play somebody wrote and would refuse it as not_learned",
			len(fake.asked))
	}
	if got := host.pressed(); len(got) != 2 {
		t.Fatalf("an authored play pressed %v, want the 2 calls it always did", got)
	}
}

// A learned play the user has EDITED is an ordinary play, and runs locally.
//
// `Learned()` is false the moment the source digest stops matching the sidecar. That is not a
// corruption: the user was invited to edit the play and did, and what they have now is their own
// writing — which the Director has no record of and could not perform.
func TestAnEditedLearnedPlayRunsLocallyAsBefore(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d, host, _ := learnedWorld(t)
	rt := routes.Route{App: "testgame", Slug: "mute-volume"}

	// The user's own edit: one more line, so the digest no longer matches the sidecar.
	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatalf("reading the play: %v", err)
	}
	if err := d.Reg.Save(rt, string(src)+"\n// mine now\n"); err != nil {
		t.Fatalf("editing the play: %v", err)
	}
	if r := orchestrator.Classify(d.Reg, rt, "mute-volume"); r.Learned() {
		t.Fatalf("the fixture is still learned after an edit (prov=%s)", r.Provenance)
	}
	fake := useFakeDirector(t, &fakeDirector{view: arrived(1)})

	if _, err := doAsProduct(t, d, "mute-volume", nil, nil); err != nil {
		t.Fatalf("marco do: %v", err)
	}
	if len(fake.asked) != 0 {
		t.Fatalf("an EDITED learned play was sent to the Director; the file is the user's "+
			"writing now and the Director has no record of what it says (%d request(s))",
			len(fake.asked))
	}
	if got := host.pressed(); len(got) != 2 {
		t.Fatalf("an edited learned play pressed %v, want the 2 calls it always did", got)
	}
}

// ── M11: a plan is not a performance ─────────────────────────────────────────

// "You're already there" is a true answer and NOT a performed play.
//
// `PerformGoal` reports Arrived with no steps when the Audience was already standing on the
// outcome. Nothing was performed. Reporting that as "ran" credits the play with a state of the
// world it did not produce — and the Audience, who asked for something to happen, is told it did.
//
// Mutation this kills (M11): drop the plan-only branch in renderPerform and let bare Arrived mean
// success, so a plan-only outcome is reported as an execution.
func TestPlanOnlyIsNotReportedAsAPerformedPlay(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d, _, out := learnedWorld(t)
	useFakeDirector(t, &fakeDirector{view: service.PerformView{
		Application: "testgame", Goal: "mute volume",
		Arrived: true, Say: "You're already there.",
	}})

	if _, err := doAsProduct(t, d, "mute-volume", nil, nil); err != nil {
		t.Fatalf("marco do: %v", err)
	}
	said := out.String()
	if !strings.Contains(said, "performed no steps") {
		t.Fatalf("a plan-only outcome reads as:\n%s\nwant it to say plainly that no steps "+
			"were performed — the Audience asked for something to happen", said)
	}
	if strings.Contains(said, ": performed ") {
		t.Fatalf("a plan-only outcome is reported as a performed play:\n%s\n"+
			"Nothing ran. Saying it did credits the play with a state of the world it did "+
			"not produce.", said)
	}
}

// ── M12: half a route is not a success ───────────────────────────────────────

// A multi-edge play whose FIRST edge verified must not report success.
//
// Mutation this kills (M12): widen renderPerform's success test from "arrived, and every step
// verified" to "some step verified", so the first working edge carries the whole play.
func TestAPartlyWalkedRouteIsNotASuccess(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d, _, out := learnedWorld(t)
	useFakeDirector(t, &fakeDirector{view: service.PerformView{
		Application: "testgame", Goal: "mute volume", From: "subj_a",
		Steps: []service.PerformStep{
			{From: "subj_a", To: "subj_b", Verified: true},
			{From: "subj_b", To: "subj_c", Refusal: "input_failed",
				Detail: "expected subj_c, saw nothing"},
		},
		Refusal: "step_unverified",
		Say:     "I got as far as the settings list and stopped.",
	}})

	_, err := doAsProduct(t, d, "mute-volume", nil, nil)
	if err == nil {
		t.Fatalf("a route that stopped half way reported SUCCESS.\n"+
			"`marco do` exits 0 on a nil error and the overlay reads the exit code, so the "+
			"Audience is told it worked. Output was:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "1 of 2") {
		t.Errorf("the failure does not say how far it got: %v", err)
	}
	if !strings.Contains(err.Error(), "step_unverified") {
		t.Errorf("the failure does not carry the Director's own refusal: %v", err)
	}
	said := out.String()
	for _, want := range []string{"step 1 of 2: verified", "step 2 of 2: input_failed",
		"expected subj_c, saw nothing"} {
		if !strings.Contains(said, want) {
			t.Errorf("the step report is missing %q:\n%s", want, said)
		}
	}
}

// Every refusal is a failure, whatever its word.
//
// The closed vocabulary is the Director's; what matters here is that none of it exits 0. A
// refusal reported as success is the Audience being told it worked when nothing happened.
func TestEveryRefusalIsReportedAsAFailure(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	for _, tc := range []struct {
		refusal string
		say     string
	}{
		{"not_learned", `I haven't learned how to reach "mute volume".`},
		{"application_not_available", "I couldn't bring testgame to the front"},
		{"place_unknown", "I can't tell which screen is in front right now"},
		{"no_known_route", "I know that outcome and can't get there from here."},
		{"did_not_arrive", "Every step worked and this isn't where I expected to end up."},
		{"no_authority", ""},
		{"no_actuator", ""},
	} {
		t.Run(tc.refusal, func(t *testing.T) {
			d, _, _ := learnedWorld(t)
			useFakeDirector(t, &fakeDirector{view: service.PerformView{
				Application: "testgame", Goal: "mute volume",
				Refusal: tc.refusal, Say: tc.say,
			}})
			_, err := doAsProduct(t, d, "mute-volume", nil, nil)
			if err == nil {
				t.Fatalf("refusal %q exited 0 — the overlay reads that as success", tc.refusal)
			}
			if !strings.Contains(err.Error(), tc.refusal) {
				t.Errorf("the refusal word %q is not in the failure: %v", tc.refusal, err)
			}
			if tc.say != "" && !strings.Contains(err.Error(), tc.say) {
				t.Errorf("the Director's own sentence is missing: %v", err)
			}
		})
	}
}

// ── an unreachable Director fails honestly ───────────────────────────────────

// No Director, no local fallback, no pretending.
//
// Falling back to a local run looks generous and is not: the play refuses at its own first line
// with "Marco could not check", the generated play catches that refusal, the process exits 0, and
// the overlay reports SUCCESS. The honest answer is that the Director is not available — and it
// carries the Director's OWN reason for refusing to start, which its stderr now delivers.
func TestAnUnavailableDirectorFailsHonestlyAndRunsNothingLocally(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d, host, _ := learnedWorld(t)
	refuseToDial(t, errStartupRefusal)

	_, err := doAsProduct(t, d, "mute-volume", nil, nil)
	if err == nil {
		t.Fatal("an unreachable Director reported success")
	}
	if !strings.Contains(err.Error(), "Director") {
		t.Errorf("the failure does not name the Director: %v", err)
	}
	if !strings.Contains(err.Error(), errStartupRefusal.Error()) {
		t.Errorf("the Director's own reason for not starting was dropped: %v", err)
	}
	if got := host.pressed(); len(got) != 0 {
		t.Fatalf("with no Director the play was run LOCALLY and pressed %v.\n"+
			"That path refuses at Screen's Showing, the generated play catches the refusal, "+
			"and the process exits 0 — a silent success for a play that never ran.", got)
	}
}

// errStartupRefusal stands in for the sentence a Director prints when it will not start.
var errStartupRefusal = &startupRefusal{}

type startupRefusal struct{}

func (*startupRefusal) Error() string {
	return "director: accessibility bridge not found at C:\\nowhere\\uia.exe"
}

// ── arguments are said out loud, not dropped ─────────────────────────────────

// A learned play takes no arguments, and says so rather than ignoring them.
//
// Director lowers a verified walk into fixed capability calls; there are no {{name}}/{{N}}
// placeholders and nowhere for `with a, b` to go. Dropping them silently would let somebody watch
// the wrong thing happen and believe they had asked for it.
func TestALearnedPlayRefusesArgumentsRatherThanDroppingThem(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	for _, tc := range []struct {
		name       string
		named      map[string]string
		positional []string
	}{
		{"named", map[string]string{"level": "3"}, nil},
		{"positional", nil, []string{"3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, _, _ := learnedWorld(t)
			fake := useFakeDirector(t, &fakeDirector{view: arrived(1)})

			_, err := doAsProduct(t, d, "mute-volume", tc.named, tc.positional)
			if err == nil {
				t.Fatal("arguments to a learned play were accepted and silently dropped")
			}
			if !strings.Contains(err.Error(), "takes no arguments") {
				t.Errorf("the refusal does not say why: %v", err)
			}
			if len(fake.asked) != 0 {
				t.Errorf("the play was performed anyway, without the arguments asked for")
			}
		})
	}
}

// ── the join the bridge depends on ───────────────────────────────────────────

// The play's slug turns back into the outcome's own name.
//
// # Why this is load-bearing
//
// `PerformQuery` carries a NAME, and the Director matches it case-insensitively against
// `Goal.Name` — the words the person used when the behaviour was learned. The play's slug came
// from `routes.Slug` of those same words. So the bridge recovers the name by turning the slug
// back into words, and if that inverse ever stops holding, a learned play resolves locally and
// then cannot be performed at all: the Director answers `not_learned` about a behaviour it
// demonstrably learned.
//
// Measured against the real artifact on the development machine: the stored Goal.Name was
// "open mouse settings", routes.Slug of it is "open-mouse-settings", and the bridge derives
// "open mouse settings" — an exact match.
//
// The `want: false` rows are the LIMIT OF THE NAME, and they are still recorded here because the
// name is still what a log and a refusal say. They no longer decide anything: the play.s IDENTITY
// is now `Origin.To`, the durable remembered subject, carried on `PerformQuery.Subject` and matched
// against `Goal.Subject` before any words are compared. A behaviour learned as "open Steve.s
// downloads" is found by its subject and reported by its own name.
//
// So this table proves the property the FALLBACK rests on — a goal with no sidecar is still
// reachable by name — and TestTheSubjectIsSentSoAnAwkwardNameStillReachesItsGoal proves that the
// awkward rows no longer depend on it.
func TestTheLearnedPlayNameJoinRoundTrips(t *testing.T) {
	for _, tc := range []struct {
		taught string
		want   bool
	}{
		{"open mouse settings", true}, // the real goal on the development machine
		{"Open Mouse Settings", true}, // mixed case: EqualFold covers it
		{"open downloads", true},
		{"open the downloads folder", true},
		{"mute volume", true},
		{"open mouse settings 2", true},
		{"Open Mouse Settings!", false},   // punctuation is discarded
		{"open mouse  settings", false},   // a run collapses to one dash
		{"e-mail steve", false},           // a hyphen is indistinguishable from a space
		{"open Steve's downloads", false}, // an apostrophe is discarded
		{"open mouse settings (win11)", false},
	} {
		t.Run(tc.taught, func(t *testing.T) {
			reg := routes.Registry{Dir: t.TempDir()}
			rt := routes.Route{App: "testgame", Slug: routes.Slug(tc.taught)}
			d := orchestrator.Deps{Reg: reg}
			r := orchestrator.Resolved{Route: rt, Phrase: tc.taught,
				Kind: routes.KindLearned, Provenance: routes.OriginIntact}

			name, _, _ := performIdentity(d, r)
			got := strings.EqualFold(name, tc.taught)
			if got != tc.want {
				if tc.want {
					t.Fatalf("the bridge would ask the Director for %q, and the goal is "+
						"called %q.\nThe join is by name; a learned play that resolves "+
						"locally and then cannot be named to the Director can never be "+
						"performed.", name, tc.taught)
				}
				t.Fatalf("%q now round-trips to %q, which the table says it should not.\n"+
					"That is an IMPROVEMENT — update this row deliberately, and say whether "+
					"the durable Origin.To → Goal.Subject join is still worth building.",
					tc.taught, name)
			}
		})
	}
}

// The application comes from the sidecar, which is what Director wrote.
//
// Not from the route's folder: a folder is where somebody could later move the file, and the
// sidecar is the record of what the play was learned against.
func TestTheRequestIsScopedByTheSidecarsApplication(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "moved-here", Slug: "mute-volume"}
	if err := reg.SaveWithOrigin(rt, "the App is a script.\n", routes.Origin{
		Kind: routes.KindLearned, Application: "testgame", From: "subj_a", To: "subj_b",
	}); err != nil {
		t.Fatalf("saving: %v", err)
	}
	d := orchestrator.Deps{Reg: reg}
	_, app, _ := performIdentity(d, orchestrator.Classify(reg, rt, "mute volume"))
	if app != "testgame" {
		t.Fatalf("the request is scoped to %q, want the sidecar's %q — a folder is where "+
			"somebody moved the file, not what it was learned against", app, "testgame")
	}
}

// ── the confirmation the overlay has to be able to answer ────────────────────

// The run-confirmation must be a prompt the overlay can detect and answer.
//
// # Why this is a test and not a comment
//
// A learned play is the ONE play that asks before it runs (ADR-029), and from the overlay there
// is no terminal. `streamChild` in plugins/overlay/acts.go detects a prompt on the partial,
// newline-less line by exactly two properties — it contains "? [" and it ends with ": " — arms
// the single-key y/n answer path, and writes the answer back down the child's stdin.
//
// So the SHAPE of this sentence is a contract with another module, and nothing else states it.
// If the wording drifts to "Run it now? (y/n)" the prompt stops being detected, the child blocks
// on a stdin nobody is writing to, and a learned play can no longer be run from the product
// surface at all.
func TestTheRunConfirmationIsAPromptTheOverlayCanAnswer(t *testing.T) {
	r := orchestrator.Resolved{Phrase: "open mouse settings",
		Kind: routes.KindLearned, Provenance: routes.OriginIntact}
	// The literal AskFirst writes: the question, then the options, then the answer cursor.
	prompt := orchestrator.AskToRun(r) + " [y]es / [n]o: "

	if !strings.Contains(prompt, "? [") {
		t.Errorf("the overlay detects a prompt by the bracketed options after the question "+
			"mark; %q has none, so it is never shown and never answered", prompt)
	}
	if !strings.HasSuffix(prompt, ": ") {
		t.Errorf("the overlay waits for the answer cursor %q to know the menu is complete; "+
			"%q does not end with one", ": ", prompt)
	}
	for _, key := range []string{"y", "n"} {
		if !strings.Contains(prompt, "["+key+"]") {
			t.Errorf("the overlay offers single-key %q; the prompt does not: %q", key, prompt)
		}
	}
}

// A bridge failure still announces the route it resolved.
//
// # The cross-module contract this pins
//
// The overlay decides an unknown command by ELIMINATION: `runRecord` returns the route announced
// on the "[route] " line, and both dispatch sites treat `failed` with no route as "nothing
// resolved — offer to learn it" (plugins/overlay/acts.go and director.go). So if the bridge
// failed before the announce, a learned play that refused would be reported to the Audience as
// a command Marco has never heard of, and they would be invited to learn it again.
//
// Mutation this kills: moving the `[route]` announce in dispatchDo after the learned fork.
func TestABridgeFailureStillAnnouncesTheResolvedRoute(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1")
	d, _, _ := learnedWorld(t)
	useFakeDirector(t, &fakeDirector{view: service.PerformView{
		Application: "testgame", Goal: "mute volume",
		Refusal: "place_unknown", Say: "I can't tell which screen is in front right now",
	}})

	said, err := captureStdout(t, func() error {
		_, e := doAsProduct(t, d, "mute-volume", nil, nil)
		return e
	})
	if err == nil {
		t.Fatal("a refused bridge run reported success")
	}
	if !strings.Contains(said, "[route] mute volume") {
		t.Fatalf("the resolved route was never announced:\n%s\n"+
			"The overlay reads a failure with no [route] as an unknown command and offers "+
			"to learn it — so a learned play that refused would be presented as one Marco "+
			"has never heard of.", said)
	}
}

// captureStdout runs fn with os.Stdout redirected, and returns what was written to it.
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("pipe: %v", perr)
	}
	prev := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	err := fn()
	os.Stdout = prev
	_ = w.Close()
	said := <-done
	_ = r.Close()
	return said, err
}

// A play whose name does not survive its own slug still reaches its goal, by subject.
//
// # Why this is the test that matters
//
// The row above says the NAME cannot be recovered for these. That used to mean the play could not
// be performed at all: it resolved locally, was announced, delegated — and the Director answered
// `not_learned` about a behaviour it had itself watched, verified and written down. The durable
// key was on disk on both sides the whole time, written in the same breath by the same learn pass,
// and simply never carried.
//
// Mutation: drop Subject from the PerformQuery, or stop reading Origin.To in performIdentity.
// This fails.
func TestTheSubjectIsSentSoAnAwkwardNameStillReachesItsGoal(t *testing.T) {
	for _, taught := range []string{
		"open Steve's downloads", "Open Mouse Settings!", "e-mail steve",
		"open mouse  settings", "open mouse settings (win11)",
	} {
		t.Run(taught, func(t *testing.T) {
			reg := routes.Registry{Dir: t.TempDir()}
			rt := routes.Route{App: "testgame", Focus: true, Slug: routes.Slug(taught)}
			const subject = "subj_0000f699ea69"
			err := reg.SaveWithOrigin(rt, "script main...\n  do nothing.\n", routes.Origin{
				Kind: routes.KindLearned, Application: "testgame", From: "subj_start", To: subject,
			})
			if err != nil {
				t.Fatal(err)
			}
			d := orchestrator.Deps{Reg: reg}
			r := orchestrator.Classify(reg, rt, taught)

			// THE REQUEST THAT ACTUALLY GOES OUT, not just the value a helper computed.
			// Reading Origin.To and then not putting it on the wire is the same defect
			// with an extra step.
			fake := useFakeDirector(t, &fakeDirector{view: service.PerformView{
				Arrived: true, Steps: []service.PerformStep{{Verified: true}},
			}})
			if err := performLearned(d, r, nil, nil); err != nil {
				t.Fatalf("performLearned: %v", err)
			}
			if len(fake.asked) != 1 {
				t.Fatalf("the Director was asked %d times", len(fake.asked))
			}
			q := fake.asked[0]
			if q.Subject != subject {
				t.Fatalf("the request carries subject %q; the durable subject beside "+
					"the play is %q. Without it the Director has only a name that "+
					"cannot be recovered from this slug, and answers not_learned "+
					"about a play it learned.", q.Subject, subject)
			}
			if q.Application != "testgame" {
				t.Errorf("application %q", q.Application)
			}
			// The name still travels, because a refusal with only a subject id in it is
			// unreadable, and because a goal with no sidecar is found by name.
			if strings.TrimSpace(q.Name) == "" {
				t.Error("the request carries no name at all")
			}
		})
	}
}

// A play with no sidecar carries no subject, and the name join is what is left.
func TestAPlayWithNoPastCarriesNoSubject(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "hand-written"}
	if err := reg.Save(rt, "script main...\n  do nothing.\n"); err != nil {
		t.Fatal(err)
	}
	d := orchestrator.Deps{Reg: reg}
	name, _, subject := performIdentity(d, orchestrator.Classify(reg, rt, "hand written"))
	if subject != "" {
		t.Errorf("a play with no provenance produced a subject %q out of nowhere", subject)
	}
	if name != "hand written" {
		t.Errorf("name %q", name)
	}
}
