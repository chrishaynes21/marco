package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The control centre as a PRODUCT SHELL: which views exist, which of them are normal, which are
// advanced, and what words a normal one is allowed to say.
//
// Everything here reads `editPage` — the page the server actually serves — or drives a real
// handler. Nothing re-describes the page in the test's own terms, because a second description of
// the nav is exactly the kind of thing that stays green while the nav says something else.

// sectionRE finds one rendered view and its id.
var sectionRE = regexp.MustCompile(`<section id="view-([a-z]+)"`)

// navLinkRE finds one nav entry and the view it opens.
var navLinkRE = regexp.MustCompile(`<a data-view="([a-z]+)"`)

// tagRE is every HTML tag, so a section can be reduced to the words a person actually reads.
// Attributes go with it, which is the point: an id is an identifier and not a product word.
var tagRE = regexp.MustCompile(`<[^>]*>`)

// pageViews is every section the served page renders.
func pageViews(t *testing.T) []string {
	t.Helper()
	m := sectionRE.FindAllStringSubmatch(editPage, -1)
	if len(m) < 5 {
		t.Fatalf("the control centre page has %d sections; it is not the page this test is about", len(m))
	}
	var out []string
	for _, g := range m {
		out = append(out, g[1])
	}
	sort.Strings(out)
	return out
}

// sectionText is one view's visible words: markup stripped, entities left alone.
func sectionText(t *testing.T, view string) string {
	t.Helper()
	open := `<section id="view-` + view + `"`
	i := strings.Index(editPage, open)
	if i < 0 {
		t.Fatalf("the page has no %s section", view)
	}
	j := strings.Index(editPage[i:], "</section>")
	if j < 0 {
		t.Fatalf("the %s section is never closed", view)
	}
	return tagRE.ReplaceAllString(editPage[i:i+j], " ")
}

// EVERY VIEW THE PAGE RENDERS CAN BE OPENED BY NAME.
//
// The failure this holds: `uiView` accepted five views and the page had six, so `marco ui learn`
// silently landed on the Plays browser and the overlay had no way to open the Learn surface at
// all. That is not a wrong mapping — it is a view the rest of the product cannot point at, and it
// is invisible unless something walks the page rather than the switch statement.
//
// Mutation: delete any arm of uiView. This goes red naming the view that became unreachable.
func TestEveryViewInTheControlCentreCanBeOpenedByName(t *testing.T) {
	for _, v := range pageViews(t) {
		if got := uiView([]string{v}); got != v {
			t.Errorf("uiView(%q) = %q — the page renders a %s view that nothing can open", v, got, v)
		}
	}
	// And the words a person would actually type reach the view they name.
	for arg, want := range map[string]string{
		"plays": "routes", "settings": "config", "cast": "advanced",
		"source": "edit", "here": "here", "learn": "learn", "activity": "activity",
		"advanced": "advanced", "help": "help", "bindings": "bindings",
	} {
		if got := uiView([]string{arg}); got != want {
			t.Errorf("uiView(%q) = %q, want %q", arg, got, want)
		}
	}
	if got := uiView([]string{"nonsense"}); got != "" {
		t.Errorf("uiView(nonsense) = %q, want the default view", got)
	}
}

// EVERY VIEW ALSO HAS A WAY IN FROM THE NAV.
//
// A deep link is not a home. A section nothing in the drawer opens is a section a person can only
// arrive at by typing an argument they would have to already know about.
func TestEveryViewHasANavEntry(t *testing.T) {
	linked := map[string]bool{}
	for _, g := range navLinkRE.FindAllStringSubmatch(editPage, -1) {
		linked[g[1]] = true
	}
	for _, v := range pageViews(t) {
		if !linked[v] {
			t.Errorf("the %s view has no nav entry", v)
		}
	}
}

// advancedNav is the part of the drawer under the Advanced heading.
func advancedNav(t *testing.T) string {
	t.Helper()
	i := strings.Index(editPage, `<div class="navgroup">Advanced</div>`)
	if i < 0 {
		t.Fatal("the nav has no Advanced group — there is no normal/advanced split at all")
	}
	j := strings.Index(editPage[i:], "</nav>")
	if j < 0 {
		t.Fatal("the nav is never closed")
	}
	return editPage[i : i+j]
}

// THE STEP EDITOR IS ADVANCED, AND IT IS STILL REACHABLE.
//
// Both halves matter and they pull in opposite directions, which is why they are one test. The
// audit's cheapest structural fix is that a step table over a .marco file is not a normal
// destination. But the step editor is also the documented remedy for a screen that loads slowly —
// bump the wait before the click — so DEMOTING it must not become REMOVING it: the Plays row's
// edit button, the Edit view's own picker and `marco ui edit` all keep working.
//
// Mutation: move the edit nav entry back above the Advanced heading, or drop the Plays row's edit
// button. Either goes red.
func TestTheStepEditorIsAdvancedAndStillReachable(t *testing.T) {
	adv := advancedNav(t)
	if !strings.Contains(adv, `data-view="edit"`) {
		t.Error("the step editor is not under the Advanced heading")
	}
	normal := editPage[strings.Index(editPage, `<nav id="drawer">`):strings.Index(editPage, `<div class="navgroup">Advanced</div>`)]
	if strings.Contains(normal, `data-view="edit"`) {
		t.Error("the step editor is still a normal nav item")
	}
	// STILL REACHABLE, three ways.
	if got := uiView([]string{"edit"}); got != "edit" {
		t.Errorf("marco ui edit = %q; the documented remedy for a slow screen lost its deep link", got)
	}
	if !strings.Contains(editPage, `e.textContent='edit'; e.onclick=()=>openRoute(p.name)`) {
		t.Error("the Plays row no longer offers edit")
	}
	if !strings.Contains(editPage, `nav('edit'); // switch to the editor`) {
		t.Error("opening a play no longer lands on the editor")
	}
}

// HERE IS ITS OWN VIEW, and the observation controls came with it.
//
// Seeing is not acquiring. The HERE bar — what Marco can see and whether it recognises it — lived
// inside the Learn section, so the only way to ask "what does Marco see?" was to open the surface
// for showing it something new. Watch, Remember and the list of screens Marco knows are all
// observation and all moved with it.
//
// Mutation: put lherebar back inside view-learn. This goes red on both halves.
func TestHereIsItsOwnViewWithTheObservationControls(t *testing.T) {
	here := sectionText(t, "here")
	rawHere := editPage[strings.Index(editPage, `<section id="view-here"`):]
	rawHere = rawHere[:strings.Index(rawHere, "</section>")]
	i := strings.Index(editPage, `<section id="view-learn"`)
	rawLearn := editPage[i:]
	rawLearn = rawLearn[:strings.Index(rawLearn, "</section>")]

	for _, id := range []string{`id="lherebar"`, `id="wbar"`, `id="wstop"`, `id="lplaces"`, `id="ltrail"`} {
		if !strings.Contains(rawHere, id) {
			t.Errorf("the Here view is missing %s", id)
		}
		if strings.Contains(rawLearn, id) {
			t.Errorf("%s is still inside the Learn view", id)
		}
	}
	if !strings.Contains(here, "NOW") {
		t.Error("the Here view does not say what it is showing")
	}
	// And Learn is still Learn: the acquisition controls did NOT move.
	if !strings.Contains(rawLearn, `id="lstart"`) || !strings.Contains(rawLearn, `id="ltry"`) {
		t.Error("the acquisition controls left the Learn view")
	}
}

// HERE RENDERS WITH NO LEARN SESSION AND NO SERVICE.
//
// The state a cold machine is actually in. The read must answer 200 with a state the page can
// render — not an HTTP error, not a blank — and it must say so in the words the product uses.
//
// $MARCO_HOME is a temp dir so this cannot reach a Director running on the developer's own
// desktop, and the read never auto-starts one.
func TestHereRendersWithoutALearnSession(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	w := httptest.NewRecorder()
	handleLearnState(w, httptest.NewRequest(http.MethodGet, "/api/learn", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/learn = %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if got["stage"] != "unavailable" {
		t.Errorf("stage = %v, want unavailable", got["stage"])
	}
	if got["available"] != false {
		t.Errorf("available = %v, want false", got["available"])
	}
	saying, _ := got["saying"].(string)
	if saying != "I'm not watching anything right now." {
		t.Errorf("saying = %q — the page tells a person about a process, not about themselves", saying)
	}
}

// backstage are the words a normal page may not say.
//
// Not because they are wrong — every one of them is the right word for the thing it names — but
// because the thing it names is Marco's own machinery and the Audience has no use for it. §18 of
// the 34F audit counted them: Director appeared 23 times in user-facing text.
var backstage = []string{
	"director", "theater", "theatre", "stage", "subject", "transition",
	"repertoire", "casting", "rehearsal", "rehearse",
}

// NO NORMAL SURFACE NAMES THE BACKSTAGE.
//
// Two checks, because the words arrive two ways. The sections are markup, so their visible text is
// what a person reads once the tags are stripped — an id like `lstage` is an identifier and is
// deliberately not counted. The specific retired strings are checked against the WHOLE page,
// comments included, because a test that had to reason about which occurrence was a comment is a
// test nobody would trust.
//
// Mutation: put back any of the four literals below. This goes red naming it.
func TestNoNormalSurfaceNamesTheBackstage(t *testing.T) {
	for _, v := range pageViews(t) {
		if v == "advanced" {
			continue // the whole point of that page is to name them
		}
		text := strings.ToLower(sectionText(t, v))
		for _, w := range backstage {
			if regexp.MustCompile(`\b` + w + `\b`).MatchString(text) {
				t.Errorf("the %s view says %q to a normal person", v, w)
			}
		}
	}
	// The four exact strings this change retired, gone from the page entirely.
	for _, gone := range []string{
		"DIRECTOR NOT RUNNING",
		"Target locked:",
		"Transition: ",
		"Marco's Director is not running.",
	} {
		if strings.Contains(editPage, gone) {
			t.Errorf("the control centre still says %q", gone)
		}
	}
	if strings.Contains(readRepoFile(t, "cmd/marco/learnui.go"), "Marco's Director is not running.") {
		t.Error("the Learn endpoint still reports a process name at the person")
	}
	// And the replacements are really there, so this cannot pass by deleting the lines.
	for _, want := range []string{
		"NOT WATCHING YET", "The way there: ", "Marco has settled on a window: ",
	} {
		if !strings.Contains(editPage, want) {
			t.Errorf("the plain wording %q is not on the page", want)
		}
	}
}

// refusalDecl matches one declared refusal code in the Director's closed vocabulary.
var refusalDecl = regexp.MustCompile(`(?m)^\s*[A-Za-z]+\s+Refusal\s*=\s*"([a-z_]+)"`)

// EVERY REFUSAL CODE HAS PLAIN ENGLISH.
//
// The panel mapped two of them and let a fallback un-underscore the rest, which is not translation
// — it is the identifier with its punctuation combed. "no subject", "no tail", "not lowerable",
// "not assessable", "not armed" and "action not attributed" all reached a person that way.
//
// The list is read from the Director's own declarations rather than typed here, so a refusal added
// upstream fails this test instead of quietly falling back.
//
// Mutation: delete any LREFUSED entry. This goes red naming the code.
func TestEveryRefusalCodeHasPlainEnglish(t *testing.T) {
	src := readRepoFile(t, "internal/director/learn/learn.go")
	codes := refusalDecl.FindAllStringSubmatch(src, -1)
	if len(codes) < 20 {
		t.Fatalf("found %d refusal codes; this test is not reading the vocabulary it thinks it is", len(codes))
	}
	i := strings.Index(editPage, "const LREFUSED={")
	if i < 0 {
		t.Fatal("the page has no refusal vocabulary at all")
	}
	block := editPage[i:]
	block = block[:strings.Index(block, "};")]
	for _, g := range codes {
		if !strings.Contains(block, g[1]+":") {
			t.Errorf("%s has no words, so a person is shown the code", g[1])
		}
	}
	if !strings.Contains(editPage, "refusalWords(v.refused)") {
		t.Error("the panel prints the refusal code without turning it into words")
	}
}

// A NORMAL SURFACE CAN START THE SERVICE IT DEPENDS ON — and only when pressed.
//
// Two halves, and the second is the one that keeps the first honest. A read may not pay for a
// service start (see cmd/marco/intake.go's pendingQuestion), so the seam must stay untouched by
// GET /api/learn; a press is the person asking, so POST /api/learn/wake must reach it.
//
// Mutation: point handleWake at directorConnect(false), or make handleLearnState auto-start.
// Either half goes red.
func TestANormalSurfaceCanStartTheServiceItDependsOn(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	started := 0
	old := wakeDirector
	wakeDirector = func() (*service.Client, error) {
		started++
		return nil, errors.New("nothing to connect to in this test")
	}
	defer func() { wakeDirector = old }()

	// A READ pays for nothing.
	handleLearnState(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/learn", nil))
	if started != 0 {
		t.Fatalf("a read started the service %d time(s)", started)
	}
	// A PRESS does.
	w := httptest.NewRecorder()
	handleWake(w, httptest.NewRequest(http.MethodPost, "/api/learn/wake", nil))
	if started != 1 {
		t.Fatalf("the wake button started the service %d time(s), want 1", started)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/learn/wake = %d: %s", w.Code, w.Body.String())
	}
	// It answers with the STATE, so the page renders what is true afterwards.
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if _, ok := got["stage"]; !ok {
		t.Error("the wake button answers something other than the session state")
	}
	// And the page really offers the control, only while it is needed.
	if !strings.Contains(editPage, `onclick="wakeMarco()"`) {
		t.Error("there is no control a person can press to start it")
	}
	if !strings.Contains(editPage, "v.available===false") {
		t.Error("the wake control is not tied to the unavailable state")
	}
}
