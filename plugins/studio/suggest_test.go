package main

// SAY WHAT YOU WANT, AND NOTHING RUNS UNTIL SOMEBODY PRESSES PLAY.
//
// The policy half, exercised without a model and without a desktop: which routines are offered
// for a stated target, what labels they are offered under, which arm answered, and that answering
// starts nothing. The CONNECTION to a real resolver is a different claim and is held by
// suggestlive_test.go, because a stand-in Advisor establishes nothing about whether anything ever
// reaches Ollama — which is exactly the defect this slice was built on top of.

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// saidRoutine is one row as the listing would produce it, for the pure policy above the library.
func saidRoutine(slug, app string, focus bool) Routine {
	r := Routine{Slug: slug, App: app, Focus: focus, Name: strings.ReplaceAll(slug, "-", " "),
		Runnable: true}
	switch {
	case app == "":
		r.ID, r.Scope = "global/"+slug, string(orchestrator.ScopeEverywhere)
	case focus:
		r.ID, r.Scope = app+"/focus/"+slug, string(orchestrator.ScopeAppAnywhere)
	default:
		r.ID, r.Scope = app+"/context/"+slug, string(orchestrator.ScopeApp)
	}
	return r
}

// A STATED TARGET, AND NOT A RUNNING APPLICATION.
//
// The distinction the three scopes exist for. An in-place routine works only while ITS
// application is in front, so it is offered only when that application is the stated target —
// where "this application has a window" would have made every running application's in-place
// commands reachable from anywhere, which is what anywhere-in-application already means and is
// the whole difference between the two. Nothing here consults a window at all, which is the
// point: eligibility is not a fact about what is running.
func TestOnlyTheScopesTheTargetAllowsAreOffered(t *testing.T) {
	all := []Routine{
		saidRoutine("quit", "", false),                // global
		saidRoutine("graphics", "arkascended", true),  // anywhere-in-application
		saidRoutine("settings", "arkascended", false), // in place, in Ark
		saidRoutine("settings", "steam", false),       // in place, in Steam
		saidRoutine("library", "steam", true),         // anywhere-in-application
	}
	broken := saidRoutine("sound", "arkascended", false)
	broken.Runnable, broken.Why = false, "it states a window and switches nothing"
	all = append(all, broken)

	ids := func(target string) []string {
		var out []string
		for _, r := range eligible(all, target) {
			out = append(out, r.ID)
		}
		return out
	}

	// WITH ARK AS THE TARGET: global, both anywhere-in-application routines, and Ark's own
	// in-place one. NOT Steam's in-place one, however much of Steam is running.
	got := ids("arkascended")
	want := []string{"global/quit", "arkascended/focus/graphics",
		"arkascended/context/settings", "steam/focus/library"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("with Ark targeted the offer is\n  %v\nwant\n  %v", got, want)
	}
	for _, never := range []string{"steam/context/settings", "arkascended/context/sound"} {
		for _, id := range got {
			if id == never {
				t.Errorf("%s was offered and must not be", never)
			}
		}
	}

	// AND WITH NOTHING TARGETED, NO IN-PLACE ROUTINE AT ALL. Not a default to the first
	// application that happens to be running, and not Studio's own foreground: an
	// in-place routine needs an application in front, and with none stated nothing can
	// say which. The two that work from anywhere are still offered, because they do.
	got = ids("")
	want = []string{"global/quit", "arkascended/focus/graphics", "steam/focus/library"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("with nothing targeted the offer is\n  %v\nwant\n  %v", got, want)
	}

	// AND A TARGET NOBODY HAS ROUTINES FOR CHANGES ONLY THAT.
	if got := ids("notepad"); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("with an unrelated target the offer is\n  %v\nwant\n  %v", got, want)
	}
}

// AND EVERY LABEL OFFERED IS ONE THE MATCHER CAN TELL APART.
//
// Two routines called `settings` in two scopes are two routines, and a label is useless unless an
// answer naming it names exactly one of them. The check is on the DETERMINISTIC MATCHER'S OWN
// KEY, not on the string: a hyphen is a space to it and "in" is a stop word, so `settings-in-ark`
// and a routine genuinely slugged `settings-ark` are one candidate as far as it is concerned.
// That collision is the reason appending scope text is not on its own enough.
//
// THE ORDER HERE IS THE AWKWARD ONE ON PURPOSE. A global `settings` comes first and takes that
// name, so the in-place `settings` has to derive one — and what it would derive is the actual
// slug of a third routine, which is already offered under it. That is the case the counter
// exists for, and it is reachable only when the listing is in this order.
func TestEveryOfferedLabelIsDistinguishable(t *testing.T) {
	all := []Routine{
		saidRoutine("settings", "", false),                // takes its own name
		saidRoutine("settings", "arkascended", false),     // must derive one
		saidRoutine("settings-in-arkascended", "", false), // ...which is THIS one's name
		saidRoutine("settings", "arkascended", true),
		saidRoutine("graphics", "arkascended", false),
	}
	labels, byLabel := labelled(eligible(all, "arkascended"))

	if len(labels) != len(all) {
		t.Fatalf("%d of %d routines were offered: %v", len(labels), len(all), labels)
	}
	// EVERY KEY DISTINCT, which is the property. A duplicate here is two routines the
	// matcher reads as one, so an answer naming either names both.
	seen := map[string]string{}
	for _, l := range labels {
		key := nlu.Key(l)
		if first, dup := seen[key]; dup {
			t.Errorf("labels %q and %q both reduce to %q, so an answer naming "+
				"either names both", first, l, key)
		}
		seen[key] = l
	}
	// AND EVERY LABEL LEADS BACK TO EXACTLY THE ROUTINE IT WAS MADE FOR.
	ids := map[string]bool{}
	for _, l := range labels {
		r, ok := byLabel[l]
		if !ok {
			t.Errorf("label %q maps to no routine", l)
			continue
		}
		if ids[r.ID] {
			t.Errorf("label %q maps to %s, which another label already claimed",
				l, r.ID)
		}
		ids[r.ID] = true
	}
	if len(ids) != len(all) {
		t.Errorf("%d routines are reachable through the labels, want %d", len(ids), len(all))
	}
	// AND A ROUTINE KEEPS ITS OWN NAME, whatever order the listing is in: the one that
	// really is called `settings-in-arkascended` is offered under that, and the namesake
	// that would have derived it gets the next label along instead.
	label := func(id string) string {
		for l, r := range byLabel {
			if r.ID == id {
				return l
			}
		}
		return ""
	}
	if got := label("global/settings-in-arkascended"); got != "settings-in-arkascended" {
		t.Errorf("the routine actually named settings-in-arkascended is offered as %q, "+
			"having lost its own name to a label derived for another one", got)
	}
	// AND A DERIVED LABEL SAYS WHERE THE ROUTINE WORKS, which is the whole of what makes
	// two namesakes distinguishable to whoever reads them — a bare counter would be
	// unique and would tell nobody, person or model, which of the two they were picking.
	inPlace := label("arkascended/context/settings")
	if !strings.Contains(inPlace, "arkascended") {
		t.Errorf("the in-place settings is offered as %q, which does not say which "+
			"application it belongs to", inPlace)
	}
	// EXACTLY THE SCOPED FORM, with no counter on the end: its scoped label was free, so
	// reaching past it to `...-2` would offer a label implying a second routine that does
	// not exist. The counter is the last resort and has to stay one.
	if anywhere := label("arkascended/focus/settings"); anywhere != "settings-anywhere-in-arkascended" {
		t.Errorf("the anywhere-in-application settings is offered as %q, want the "+
			"scoped form that was free", anywhere)
	}
	if label("global/settings") != "settings" {
		t.Errorf("the global settings did not keep the plain name it had first claim "+
			"on: %q", label("global/settings"))
	}
}

// recordingAdvisor is a stand-in that says what it was asked and answers what it was told to.
type recordingAdvisor struct {
	asked  int
	offers dispatch.Decision
	ok     bool
	labels []string
}

func (r *recordingAdvisor) Advise(_ context.Context, _ string, labels []string,
	_ string) (dispatch.Decision, bool) {

	r.asked++
	r.labels = labels
	return r.offers, r.ok
}

// aLibraryWithOneRoutine saves one through the product's own path and hands back the surface.
//
// ANYWHERE-IN-APPLICATION, because that is the scope this recording can legitimately be saved
// under: every press in it checks the window it was made in, and an in-place routine brings no
// window forward, so Marco refuses that combination before it reaches disk. It is eligible
// whatever target is stated, which is exactly what the tests below need.
func aLibraryWithOneRoutine(t *testing.T) *Acts {
	t.Helper()
	return aSavedRoutine(t, string(orchestrator.ScopeAppAnywhere), nil)
}

// aLibraryWithAnInPlaceRoutine saves one that works only while its application is in front.
//
// Its press states no window, which is what lets an in-place routine be saved and still run:
// there is nothing for it to be checked against, so it checks nothing. The scope is the point
// here rather than the contract.
func aLibraryWithAnInPlaceRoutine(t *testing.T) *Acts {
	t.Helper()
	return aSavedRoutine(t, string(orchestrator.ScopeApp), func(p *recorder.RecordedEvent) {
		p.WinRel, p.RelTo = false, ""
	})
}

// AND THE ANSWER SAYS WHETHER THE RESOLVER TOOK PART.
//
// Three states, because two of them need no model and are indistinguishable in their answers: a
// window built on this could work, look right, and never once reach Ollama. The third is the one
// that used to be invisible — configured, asked, and silent, which looks exactly like never
// having configured one.
func TestTheAnswerSaysWhetherTheResolverTookPart(t *testing.T) {
	a := aLibraryWithOneRoutine(t)
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routine(s): %+v", len(rs), rs)
	}
	target := rs[0].App

	// NO RESOLVER AT ALL. Not a silence: a configuration fact, said as one.
	a.advisor = nil
	t.Setenv("MARCO_RESOLVER", "")
	t.Setenv("MARCO_ASSISTANT", "")
	said := a.SuggestRoutine("make the pictures nicer", target)
	if !strings.Contains(said.Resolver, "no resolver is configured") {
		t.Errorf("with nothing configured the answer says %q", said.Resolver)
	}
	if said.Source == dispatch.SourceAdvisor {
		t.Errorf("an answer was attributed to a resolver that does not exist: %+v", said)
	}

	// CONFIGURED AND SILENT. The deterministic arm answered, and it says so.
	silent := &recordingAdvisor{}
	a.advisor = silent
	said = a.SuggestRoutine("make the pictures nicer", target)
	if silent.asked != 1 {
		t.Errorf("the configured resolver was asked %d time(s)", silent.asked)
	}
	if said.Source != dispatch.SourceFallback {
		t.Errorf("a silent resolver produced source %q", said.Source)
	}
	if !strings.Contains(said.Resolver, "did not answer") {
		t.Errorf("a silent resolver is reported as %q", said.Resolver)
	}

	// CONFIGURED AND ANSWERING, with the identity it named.
	speaking := &recordingAdvisor{ok: true, offers: dispatch.Decision{
		Intent: dispatch.IntentRun, Route: rs[0].Slug, Reply: "Opening it."}}
	a.advisor = speaking
	said = a.SuggestRoutine("make the pictures nicer", target)
	if said.Source != dispatch.SourceAdvisor {
		t.Errorf("an answering resolver produced source %q: %+v", said.Source, said)
	}
	if said.Kind != dispatch.IntentRun || said.ID != rs[0].ID {
		t.Errorf("the suggestion is %+v, want a run of %s", said, rs[0].ID)
	}
	if !strings.Contains(said.Resolver, "answered this") {
		t.Errorf("an answering resolver is reported as %q", said.Resolver)
	}
	// AND IT WAS OFFERED LABELS, NEVER IDENTITIES. A path-shaped identity reduces to one
	// unmatchable word in the matcher, and the prompt asks for a slug.
	for _, l := range speaking.labels {
		if strings.Contains(l, "/") {
			t.Errorf("the resolver was offered %q, which is an identity and not a "+
				"label", l)
		}
	}
}

// AND AN EXACT NAME IS ANSWERED WITHOUT ASKING ANYTHING.
//
// Free, offline, and the common case. It is reported as `exact` even with a resolver configured,
// because the resolver was not consulted — an answer that claimed otherwise would be the same
// false impression this slice exists to remove, in the other direction.
func TestAnExactNameNeverReachesTheResolver(t *testing.T) {
	a := aLibraryWithOneRoutine(t)
	rs := a.Routines()
	if len(rs) != 1 {
		t.Fatalf("the library holds %d routine(s)", len(rs))
	}
	never := &recordingAdvisor{ok: true, offers: dispatch.Decision{
		Intent: dispatch.IntentRun, Route: "something-else"}}
	a.advisor = never

	said := a.SuggestRoutine(rs[0].Name, rs[0].App)
	if never.asked != 0 {
		t.Errorf("an exact name consulted the resolver %d time(s)", never.asked)
	}
	if said.Source != dispatch.SourceExact {
		t.Errorf("an exact name produced source %q", said.Source)
	}
	if said.ID != rs[0].ID {
		t.Errorf("an exact name selected %q, want %s", said.ID, rs[0].ID)
	}
}

// AND A SUGGESTION STARTS NOTHING.
//
// The whole of what makes this safe to call as somebody types. It names a routine; the thing that
// performs one is the Play button that was already there, with the guards it already had.
func TestASuggestionStartsNothing(t *testing.T) {
	a := aLibraryWithOneRoutine(t)
	rs := a.Routines()
	a.advisor = &recordingAdvisor{ok: true, offers: dispatch.Decision{
		Intent: dispatch.IntentRun, Route: rs[0].Slug, Reply: "Opening it."}}

	for i := 0; i < 3; i++ {
		if said := a.SuggestRoutine("make the pictures nicer", rs[0].App); said.ID == "" {
			t.Fatalf("nothing was suggested: %+v", said)
		}
	}
	a.mu.Lock()
	busy, running, routine := a.busy, a.running, a.routine
	a.mu.Unlock()
	if busy || running != nil || routine != nil {
		t.Errorf("suggesting claimed the run slot: busy=%v running=%v routine=%v",
			busy, running != nil, routine != nil)
	}
}

// AND AN ANSWER NAMING SOMETHING THAT WAS NOT OFFERED SELECTS NOTHING.
//
// `dispatch.validate` already refuses it, so this is the second of two checks rather than the
// only one — and it is here because the label is a key into a map and must never become a name to
// go looking up. A name is not unique and neither is a slug.
func TestAnUnofferedAnswerSelectsNothing(t *testing.T) {
	a := aLibraryWithOneRoutine(t)
	rs := a.Routines()
	a.advisor = &recordingAdvisor{ok: true, offers: dispatch.Decision{
		Intent: dispatch.IntentRun, Route: "a-routine-nobody-has", Reply: "Sure."}}

	said := a.SuggestRoutine("do the thing", rs[0].App)
	if said.ID != "" {
		t.Errorf("a routine was selected from an answer nobody offered: %+v", said)
	}
	if said.Kind == dispatch.IntentRun {
		t.Errorf("an unoffered answer became a run: %+v", said)
	}
}

// AND THE TARGETS ARE THE APPLICATIONS A TARGET CHANGES SOMETHING FOR.
func TestTheTargetsAreTheApplicationsWithInPlaceRoutines(t *testing.T) {
	a := aLibraryWithAnInPlaceRoutine(t)
	// NO WINDOWS AT ALL, so `Open` is false everywhere — and the application is still
	// offered as a target, because whether it is running is a hint and not a filter.
	a.listing = func() []windowref.Candidate { return nil }
	ts := a.SuggestTargets()
	if len(ts) != 1 || ts[0].App == "" || ts[0].Context != 1 {
		t.Fatalf("the targets are %+v, want the one application with an in-place "+
			"routine", ts)
	}
	if ts[0].Open {
		t.Errorf("%s is reported open with no windows at all", ts[0].App)
	}
}
