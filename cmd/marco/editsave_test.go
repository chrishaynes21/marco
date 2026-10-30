package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The control centre's SAVE, driven through the real handler over a real registry in a temp
// directory. Nothing here re-implements the rebuild or the gate: every assertion enters through
// /api/save, so deleting the gate fails these rather than a copy of them.

// findPlaySrc is an ordinary play whose one editable action is a BLOCK HEADER.
//
// It matters that this shape is reachable rather than contrived: the step editor renders
// `do OS's Find with p1...` as a single row labelled "Find target, then click", with a ✕ beside
// it, and deleting a Find header does NOT cascade to its arms the way a repeat header does. So one
// click on a button the page offers leaves `when ok?` with nothing to be an arm of.
const findPlaySrc = `use os.

the Opener is an actor.
this can Run.
this's Run does...
    the p1 is a Point with X 10, Y 20.
    do OS's Find with p1...
        when ok?
            do OS's Click with that.
        or?
            do OS's Click with p1.
    this is ok!
`

// openForEdit puts one play in front of the editor, exactly as runEdit and /api/load do.
func openForEdit(t *testing.T, e *editor, rt routes.Route) {
	t.Helper()
	e.rt, e.path = rt, e.reg.Path(rt)
	if err := e.loadSrc(); err != nil {
		t.Fatal(err)
	}
}

// postSave posts one save payload through the real handler.
func postSave(t *testing.T, e *editor, req saveReq) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	e.handleSave(w, httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(string(b))))
	return w
}

// lineOf finds the source line index of the first line containing want, so a save payload names
// the line the page would have named rather than a number copied into the test.
func lineOf(t *testing.T, src, want string) int {
	t.Helper()
	for i, l := range strings.Split(src, "\n") {
		if strings.Contains(l, want) {
			return i
		}
	}
	t.Fatalf("no line containing %q in:\n%s", want, src)
	return -1
}

// withoutLine drops one line from a source — all the rebuild does for a lone delete. It is here
// only to ask the compiler what it WOULD say, never to stand in for the handler's own rebuild.
func withoutLine(src string, at int) string {
	lines := strings.Split(src, "\n")
	return strings.Join(append(append([]string{}, lines[:at]...), lines[at+1:]...), "\n")
}

// A save that would not compile is REFUSED — nothing is written, and the compiler's own sentence
// comes back so the person can read it where they are working.
func TestASaveThatWouldNotCompileIsRefused(t *testing.T) {
	e := newTestEditor(t)
	rt := routes.Route{App: "notepad", Slug: "opener"}
	if err := e.reg.Save(rt, findPlaySrc); err != nil {
		t.Fatal(err)
	}
	openForEdit(t, e, rt)

	// The play as saved is one Marco accepts — otherwise this test proves nothing about the edit.
	if err := driver.CheckSource(findPlaySrc); err != nil {
		t.Fatalf("the unedited play must compile, or the refusal below is meaningless: %v", err)
	}

	// The delete the page offers: the Find row, whose arms are hidden and do not go with it.
	del := lineOf(t, findPlaySrc, "do OS's Find with p1...")
	w := postSave(t, e, saveReq{Deletes: []int{del}})

	if w.Code == http.StatusOK {
		t.Fatalf("POST /api/save accepted an edit that does not compile (%d): %s", w.Code, w.Body)
	}
	// THE COMPILER'S OWN SENTENCE, not a shrug. A refusal a person cannot act on is barely better
	// than the silence it replaced — so the response must carry the exact text the one gate
	// produced for the exact source the handler would otherwise have written, whatever that is.
	broken := withoutLine(findPlaySrc, del)
	want := driver.CheckSource(broken)
	if want == nil {
		t.Fatalf("the edited play compiles after all, so nothing here is a refusal:\n%s", broken)
	}
	if body := strings.TrimSpace(w.Body.String()); !strings.Contains(body, want.Error()) {
		t.Errorf("refusal did not carry the compiler's reason\n got: %q\n want it to contain: %q",
			body, want.Error())
	}
	// NOT WRITTEN ANYWAY. The whole defect was that the bad text reached disk and the person found
	// out days later, so the file is the assertion that matters.
	got, err := os.ReadFile(e.reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != findPlaySrc {
		t.Errorf("a refused save changed the file on disk:\n%s", got)
	}
	// And the session still holds what the person is editing, so the page can keep their work.
	if e.src != findPlaySrc {
		t.Errorf("a refused save changed the open source in the session")
	}
}

// A save Marco DOES accept still goes through, all the way to the file.
//
// The gate must refuse the broken edit without becoming a wall: this is the guard for the mutation
// "make the gate always refuse", which the refusal test alone would not catch.
func TestASaveThatCompilesIsWritten(t *testing.T) {
	e := newTestEditor(t)
	rt := routes.Route{App: "notepad", Slug: "opener"}
	if err := e.reg.Save(rt, playSrc); err != nil {
		t.Fatal(err)
	}
	openForEdit(t, e, rt)

	line := lineOf(t, playSrc, `do OS's Key with "enter".`)
	w := postSave(t, e, saveReq{Texts: map[string]string{strconv.Itoa(line): "escape"}})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/save = %d: %s", w.Code, w.Body)
	}
	got, err := os.ReadFile(e.reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `do OS's Key with "escape".`) {
		t.Errorf("the edit did not reach the file:\n%s", got)
	}
	if err := driver.CheckSource(string(got)); err != nil {
		t.Errorf("what was written does not compile: %v", err)
	}
}

// AND `marco edit` IS UNCHANGED BY STEP EDITING, WHICH IS A STATEMENT AND NOT AN OVERSIGHT.
//
// Step editing works over a routine's STEP LIST, written down beside it as the account of what
// its source is the render of (`routes.Registry.SaveSteps`). This editor works over SOURCE LINES
// and writes through `Registry.Save`, which touches the source and nothing else.
//
// So a save here leaves the account describing a file that has moved. What that costs, exactly:
// the routine still runs — execution reads the `.marco` and nothing else — and step editing
// refuses it afterwards, pointing at a rebuild review. Nothing is corrupted and nothing is
// silently believed, because the account carries a digest of the source it describes and a
// mismatch is not a state anything renders from.
//
// What it DOES cost is that the person's step edits are no longer the explanation of their file
// and this editor does not tell them so. Pinned down here, where the behaviour lives, so it
// cannot drift into something nobody decided. Routing this save through `Deps.Replace` is a
// separate, smaller slice.
//
// See [[Step-editing-proposal-2026-10-30]].
func TestTheLegacyEditorsSaveLeavesTheAccountMismatchedAndTheRoutineRunnable(t *testing.T) {
	e := newTestEditor(t)
	rt := routes.Route{App: "notepad", Slug: "opener"}
	if err := e.reg.Save(rt, playSrc); err != nil {
		t.Fatal(err)
	}
	// AN ACCOUNT BESIDE IT, describing the source as it stands.
	if err := e.reg.SaveRecording(rt, []byte(`[{"kind":"key"}]`)); err != nil {
		t.Fatal(err)
	}
	rec, _ := e.reg.LoadRecording(rt)
	if err := e.reg.SaveSteps(rt, routes.StoredSteps{
		Source:        routes.DigestOf(playSrc),
		Demonstration: routes.DigestOf(string(rec)),
		Steps:         []byte(`[{"kind":"key","key":"enter"}]`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, st := e.reg.Steps(rt); st != routes.StepsIntact {
		t.Fatalf("the fixture's account reads %q", st)
	}

	openForEdit(t, e, rt)
	line := lineOf(t, playSrc, `do OS's Key with "enter".`)
	w := postSave(t, e, saveReq{Texts: map[string]string{strconv.Itoa(line): "escape"}})
	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/save = %d: %s", w.Code, w.Body)
	}

	// THE ROUTINE IS THE EDITED ONE, AND IT RUNS: the one compile gate passed it, which
	// is what this editor has always been protected by.
	got, err := os.ReadFile(e.reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), `do OS's Key with "escape".`) {
		t.Errorf("the edit did not reach the file:\n%s", got)
	}
	if err := driver.CheckSource(string(got)); err != nil {
		t.Errorf("what was written does not compile: %v", err)
	}
	// AND THE ACCOUNT IS MISMATCHED, NOT BELIEVED AND NOT GONE. Not removed by this save
	// either: sweeping somebody's step list from a surface that knows nothing about it
	// would be this editor deciding something it has not been told about.
	if _, st := e.reg.Steps(rt); st != routes.StepsMismatched {
		t.Errorf("after a line edit the account reads %q, so either it is still "+
			"believed about a file that moved or it was swept by a surface that "+
			"does not know what it is", st)
	}
}
