package resolveroute

// THE RESOLUTION POLICY, HELD WHERE IT NOW LIVES.
//
// These moved here with the code. They were `plugins/studio`'s, where the widget could not reach
// them — which is the whole reason this package exists — and they are the same claims: which
// routines are eligible for which context, what labels they are offered under, which arm
// answered, and that nothing is selected from an answer nobody offered.
//
// What is new is the CONTEXT half, because the thing that decides eligibility is no longer a word
// a surface made up. See where_test.go.

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/nlu"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aCandidate is one row of a library, as Director would build it.
func aCandidate(slug, app string, focus bool) Candidate {
	rt := routes.Route{App: app, Focus: focus, Slug: slug}
	c := Candidate{Route: rt, Name: strings.ReplaceAll(slug, "-", " "), Runnable: true}
	switch {
	case app == "":
		c.ID, c.Scope = "global/"+slug, "everywhere"
	case focus:
		c.ID, c.Scope = app+"/focus/"+slug, "app-anywhere"
	default:
		c.ID, c.Scope = app+"/context/"+slug, "app"
	}
	return c
}

// at is a context a person named, which is the basis that admits everything.
func at(app string) Context { return Context{App: app, Basis: Stated} }

// AND ONLY THE SCOPES THE CONTEXT ALLOWS ARE OFFERED.
//
// The distinction the three scopes exist for. An in-place routine works only while ITS
// application is in front, so it is offered only when that application is the context AND the
// context is one that may be relied on.
func TestOnlyTheScopesTheContextAllowsAreOffered(t *testing.T) {
	all := []Candidate{
		aCandidate("quit", "", false),                // global
		aCandidate("graphics", "arkascended", true),  // anywhere-in-application
		aCandidate("settings", "arkascended", false), // in place, in Ark
		aCandidate("settings", "steam", false),       // in place, in Steam
		aCandidate("library", "steam", true),         // anywhere-in-application
	}
	broken := aCandidate("sound", "arkascended", false)
	broken.Runnable = false
	all = append(all, broken)

	ids := func(where Context) []string {
		var out []string
		for _, c := range Eligible(all, where) {
			out = append(out, c.ID)
		}
		return out
	}

	// WITH ARK AS THE CONTEXT: global, both anywhere-in-application routines, and Ark's
	// own in-place one. NOT Steam's, however much of Steam is running.
	got := ids(at("arkascended"))
	want := []string{"global/quit", "arkascended/focus/graphics",
		"arkascended/context/settings", "steam/focus/library"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("with Ark stated the offer is\n  %v\nwant\n  %v", got, want)
	}
	for _, never := range []string{"steam/context/settings", "arkascended/context/sound"} {
		for _, id := range got {
			if id == never {
				t.Errorf("%s was offered and must not be", never)
			}
		}
	}

	// ── AND EVERY BASIS THAT IS NOT KNOWN TO BE IN FRONT OFFERS NO IN-PLACE ROUTINE ──
	//
	// Which is the whole of correction one. The two that work from anywhere are still
	// offered, because neither depends on what is in front.
	anywhere := []string{"global/quit", "arkascended/focus/graphics", "steam/focus/library"}
	for _, where := range []Context{
		{Basis: Unknown},
		{Basis: LastExternal},
		// AND NOT EVEN WITH AN APPLICATION ATTACHED, which cannot be produced by
		// `Where` and is asserted anyway: the filter must read the BASIS, not the
		// presence of a name.
		{App: "arkascended", Basis: LastExternal},
		{App: "arkascended", Basis: Unknown},
	} {
		if got := ids(where); strings.Join(got, " ") != strings.Join(anywhere, " ") {
			t.Errorf("with basis %q the offer is\n  %v\nwant\n  %v",
				where.Basis, got, anywhere)
		}
	}

	// AND A CORROBORATED FOREGROUND DOES ADMIT THEM, which is the other half: without
	// this, refusing every in-place routine would pass every assertion above.
	got = ids(Context{App: "arkascended", Basis: Observed})
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("with Ark observed in front the offer is\n  %v\nwant\n  %v", got, want)
	}
}

// AND EVERY LABEL OFFERED IS ONE THE MATCHER CAN TELL APART.
func TestEveryOfferedLabelIsDistinguishable(t *testing.T) {
	all := []Candidate{
		aCandidate("settings", "", false),                // takes its own name
		aCandidate("settings", "arkascended", false),     // must derive one
		aCandidate("settings-in-arkascended", "", false), // ...which is THIS one's name
		aCandidate("settings", "arkascended", true),
		aCandidate("graphics", "arkascended", false),
	}
	labels, byLabel := labelled(Eligible(all, at("arkascended")))
	if len(labels) != len(all) {
		t.Fatalf("%d of %d routines were offered: %v", len(labels), len(all), labels)
	}
	seen := map[string]string{}
	for _, l := range labels {
		if first, dup := seen[nlu.Key(l)]; dup {
			t.Errorf("labels %q and %q are one candidate to the matcher", first, l)
		}
		seen[nlu.Key(l)] = l
	}
	ids := map[string]bool{}
	for _, l := range labels {
		c, ok := byLabel[l]
		if !ok {
			t.Errorf("label %q maps to no routine", l)
			continue
		}
		if ids[c.ID] {
			t.Errorf("label %q maps to %s, which another label already claimed", l, c.ID)
		}
		ids[c.ID] = true
	}
	if len(ids) != len(all) {
		t.Errorf("%d routines are reachable, want %d", len(ids), len(all))
	}
	label := func(id string) string {
		for l, c := range byLabel {
			if c.ID == id {
				return l
			}
		}
		return ""
	}
	// A ROUTINE KEEPS ITS OWN NAME, whatever order the listing is in.
	if got := label("global/settings-in-arkascended"); got != "settings-in-arkascended" {
		t.Errorf("the routine actually named settings-in-arkascended is offered as %q", got)
	}
	// AND A DERIVED LABEL SAYS WHERE ITS ROUTINE WORKS, exactly, with no counter when the
	// scoped form was free.
	if got := label("arkascended/focus/settings"); got != "settings-anywhere-in-arkascended" {
		t.Errorf("the anywhere-in-application settings is offered as %q", got)
	}
	if got := label("arkascended/context/settings"); !strings.Contains(got, "arkascended") {
		t.Errorf("the in-place settings is offered as %q, which does not say which "+
			"application it belongs to", got)
	}
	// AND THE BARE SHARED NAME BELONGS TO NOBODY.
	for _, l := range labels {
		if nlu.Key(l) == nlu.Key("settings") {
			t.Errorf("label %q still answers to the bare shared name", l)
		}
	}
}

// AND A DERIVED LABEL THE MATCHER CANNOT TELL FROM A REAL SLUG IS ESCALATED.
func TestADerivedLabelTheMatcherCannotTellApartIsEscalated(t *testing.T) {
	all := []Candidate{
		aCandidate("settings-ark", "", false),
		aCandidate("settings", "ark", false),
		aCandidate("settings", "", false),
	}
	labels, byLabel := labelled(Eligible(all, at("ark")))
	if len(labels) != len(all) {
		t.Fatalf("%d of %d routines were offered: %v", len(labels), len(all), labels)
	}
	seen := map[string]string{}
	for _, l := range labels {
		if first, dup := seen[nlu.Key(l)]; dup {
			t.Errorf("labels %q and %q both reduce to %q", first, l, nlu.Key(l))
		}
		seen[nlu.Key(l)] = l
	}
	if byLabel["settings-ark"].ID != "global/settings-ark" {
		t.Errorf("the routine actually named settings-ark is offered as %+v",
			byLabel["settings-ark"])
	}
	for _, l := range labels {
		if l == "settings-in-ark" {
			t.Error("a label was derived that the matcher cannot tell from the real " +
				"slug settings-ark")
		}
	}
}

// AND NEITHER NAMESAKE IS SILENTLY PRESELECTED, WHICHEVER ORDER THE LISTING IS IN.
func TestNeitherNamesakeIsSilentlyPreselected(t *testing.T) {
	ark := aCandidate("settings", "arkascended", false)
	anywhere := aCandidate("settings", "arkascended", true)
	global := aCandidate("settings", "", false)
	for _, order := range []struct {
		say string
		cs  []Candidate
	}{
		{"global first", []Candidate{global, ark, anywhere}},
		{"in-place first", []Candidate{ark, anywhere, global}},
	} {
		labels, byLabel := labelled(Eligible(order.cs, at("arkascended")))
		if len(labels) != 3 {
			t.Fatalf("%s: %d of 3 offered: %v", order.say, len(labels), labels)
		}
		for _, l := range labels {
			if nlu.Key(l) == nlu.Key("settings") {
				t.Errorf("%s: %q answers to the bare name, so a request for it "+
					"selects %s and nobody was asked", order.say, l,
					byLabel[l].ID)
			}
		}
		if m := nlu.Resolve("settings", labels); m.Exact {
			t.Errorf("%s: %q is still an exact match for the bare name",
				order.say, m.Route)
		}
	}
}

// recordingAdvisor says what it was asked and answers what it was told to.
type recordingAdvisor struct {
	asked  int
	offers dispatch.Decision
	ok     bool
	labels []string
	app    string
}

func (r *recordingAdvisor) Advise(_ context.Context, _ string, labels []string,
	app string) (dispatch.Decision, bool) {

	r.asked++
	r.labels, r.app = labels, app
	return r.offers, r.ok
}

// aLibrary is one routine and the resolver that reads it.
func aLibrary(t *testing.T, a dispatch.Advisor) (Resolver, []Candidate) {
	t.Helper()
	return Resolver{Reg: routes.Registry{Dir: t.TempDir()}, Advisor: a},
		[]Candidate{aCandidate("ark-graphics", "arkascended", true)}
}

// AND THE ANSWER SAYS WHETHER THE RESOLVER TOOK PART.
func TestTheAnswerSaysWhetherTheResolverTookPart(t *testing.T) {
	t.Setenv("MARCO_RESOLVER", "")
	t.Setenv("MARCO_ASSISTANT", "")

	// NO RESOLVER AT ALL. Not a silence: a configuration fact, said as one.
	r, cs := aLibrary(t, nil)
	said := r.Resolve(context.Background(), "make the pictures nicer", cs, at("arkascended"))
	if !strings.Contains(said.Resolver, "no resolver is configured") {
		t.Errorf("with nothing configured the answer says %q", said.Resolver)
	}
	if said.Source == dispatch.SourceAdvisor {
		t.Errorf("an answer was attributed to a resolver that does not exist: %+v", said)
	}

	// CONFIGURED AND SILENT.
	silent := &recordingAdvisor{}
	r, cs = aLibrary(t, silent)
	said = r.Resolve(context.Background(), "make the pictures nicer", cs, at("arkascended"))
	if silent.asked != 1 {
		t.Errorf("the configured resolver was asked %d time(s)", silent.asked)
	}
	if said.Source != dispatch.SourceFallback {
		t.Errorf("a silent resolver produced source %q", said.Source)
	}
	if !strings.Contains(said.Resolver, "did not answer") {
		t.Errorf("a silent resolver is reported as %q", said.Resolver)
	}

	// CONFIGURED AND ANSWERING, with the identity it named and every part of it.
	speaking := &recordingAdvisor{ok: true, offers: dispatch.Decision{
		Intent: dispatch.IntentRun, Route: "ark-graphics", Reply: "Opening it."}}
	r, cs = aLibrary(t, speaking)
	said = r.Resolve(context.Background(), "make the pictures nicer", cs, at("arkascended"))
	if said.Source != dispatch.SourceAdvisor || said.Kind != dispatch.IntentRun {
		t.Fatalf("an answering resolver produced %+v", said)
	}
	if said.ID != cs[0].ID || said.App != "arkascended" || said.Slug != "ark-graphics" ||
		!said.Focus {
		t.Errorf("the answer does not carry every part of the identity: %+v", said)
	}
	if !strings.Contains(said.Resolver, "answered this") {
		t.Errorf("an answering resolver is reported as %q", said.Resolver)
	}
	// AND IT WAS OFFERED LABELS, NEVER IDENTITIES, and told which application.
	for _, l := range speaking.labels {
		if strings.Contains(l, "/") {
			t.Errorf("the resolver was offered %q, which is an identity", l)
		}
	}
	if speaking.app != "arkascended" {
		t.Errorf("the resolver was told the application is %q", speaking.app)
	}
}

// AND AN EXACT NAME IS ANSWERED WITHOUT ASKING ANYTHING.
func TestAnExactNameNeverReachesTheResolver(t *testing.T) {
	never := &recordingAdvisor{ok: true, offers: dispatch.Decision{
		Intent: dispatch.IntentRun, Route: "something-else"}}
	r, cs := aLibrary(t, never)
	said := r.Resolve(context.Background(), "ark graphics", cs, at("arkascended"))
	if never.asked != 0 {
		t.Errorf("an exact name consulted the resolver %d time(s)", never.asked)
	}
	if said.Source != dispatch.SourceExact {
		t.Errorf("an exact name produced source %q", said.Source)
	}
	if said.ID != cs[0].ID {
		t.Errorf("an exact name selected %q, want %s", said.ID, cs[0].ID)
	}
	// AND THE SENTENCE SAYS IT WAS NOT ASKED, which is not the same as having answered.
	if !strings.Contains(said.Resolver, "was NOT asked") ||
		strings.Contains(said.Resolver, "answered this") {
		t.Errorf("an exact offline match reports the resolver as %q", said.Resolver)
	}
}

// AND AN ANSWER NAMING SOMETHING THAT WAS NOT OFFERED SELECTS NOTHING.
func TestAnUnofferedAnswerSelectsNothing(t *testing.T) {
	r, cs := aLibrary(t, &recordingAdvisor{ok: true, offers: dispatch.Decision{
		Intent: dispatch.IntentRun, Route: "a-routine-nobody-has", Reply: "Sure."}})
	said := r.Resolve(context.Background(), "do the thing", cs, at("arkascended"))
	if said.ID != "" || said.Kind == dispatch.IntentRun {
		t.Errorf("a routine was selected from an answer nobody offered: %+v", said)
	}
}

// AND A SHARED NAME IS A QUESTION THAT NAMES THE ALTERNATIVES.
func TestASharedNameIsAQuestionAndNotASelection(t *testing.T) {
	cs := []Candidate{
		aCandidate("settings", "arkascended", false),
		aCandidate("settings", "arkascended", true),
	}
	r := Resolver{Reg: routes.Registry{Dir: t.TempDir()}, Advisor: &recordingAdvisor{}}
	said := r.Resolve(context.Background(), "settings", cs, at("arkascended"))
	if said.ID != "" || said.Kind != dispatch.IntentClarify {
		t.Errorf("a request for a name two routines answer to produced %+v", said)
	}
	for _, says := range []string{"is the name of 2 routines", "already in front",
		"brings arkascended forward"} {
		if !strings.Contains(said.Reply, says) {
			t.Errorf("the question does not say %q: %s", says, said.Reply)
		}
	}
	// AND IT NAMES THE CANDIDATES BY IDENTITY, so a surface can offer exactly those two.
	if len(said.Candidates) != 2 {
		t.Errorf("the question narrowed to %v, want both identities", said.Candidates)
	}
	for _, id := range said.Candidates {
		if !strings.Contains(id, "/") {
			t.Errorf("candidate %q is not a whole identity", id)
		}
	}
}

// AND THE LIBRARY IS NAMED IN EVERY ANSWER.
//
// So a surface can tell that Director resolved against a different one rather than silently
// selecting from somewhere else. The same reasoning `Prepared` already carries for a review.
func TestEveryAnswerNamesTheLibraryItResolvedAgainst(t *testing.T) {
	r, cs := aLibrary(t, nil)
	for _, request := range []string{"", "ark graphics", "something nobody has"} {
		said := r.Resolve(context.Background(), request, cs, at("arkascended"))
		if said.Library != r.Reg.Dir {
			t.Errorf("%q answered with library %q, want %q", request, said.Library,
				r.Reg.Dir)
		}
	}
}
