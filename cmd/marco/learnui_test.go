package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The Learn panel's server side, proved through the production constructor.
//
// Every test here enters through learnAPI — the same call main makes — rather than through a mux
// the test built itself. A wiring test that registers its own routes proves the handlers compile,
// which is not the failure anybody has ever had. See the memory note "prove wiring by deleting
// it": three recorded cases of complete code that was never invoked.

// learnMux is the panel's routing table, built the way the product builds it.
func learnMux() *http.ServeMux {
	mux := http.NewServeMux()
	learnAPI(mux)
	return mux
}

// Every verb the panel offers is reachable.
//
// Not "the handler exists" — reachable at the path the page posts to. A renamed route is a button
// that silently does nothing, which is how the naming failure stayed unrepairable: the operation
// existed at the Director and had no way in.
//
// Mutation: change any path here. This fails naming the one that moved.
func TestEveryLearnVerbIsReachable(t *testing.T) {
	mux := learnMux()
	for _, path := range []string{
		"/api/learn",
		"/api/learn/start",
		"/api/learn/stop",
		"/api/learn/try",
		"/api/learn/cancel",
		"/api/learn/name",
		"/api/learn/skip",
		"/api/learn/rename",
		"/api/learn/answer",
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("{}"))
		h, pattern := mux.Handler(req)
		if h == nil || pattern != path {
			t.Errorf("%s is not wired (matched %q). The page posts there; nothing answers, "+
				"and the person sees a button that does nothing.", path, pattern)
		}
	}
}

// A verb only acts on a POST.
//
// A GET that acted would mean a link, a prefetch or a refresh could start a demonstration or take
// a name back. Learning is not something a page load may do.
func TestALearnVerbRefusesAGet(t *testing.T) {
	mux := learnMux()
	for _, path := range []string{"/api/learn/start", "/api/learn/stop", "/api/learn/rename"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s answered %d, want it refused. A refresh must not be able to "+
				"start learning or take a name back.", path, w.Code)
		}
	}
}

// The rename request carries WHICH place, and the name as typed.
//
// The whole 34C contract at this boundary. Losing Place makes the Director guess which screen was
// meant — the exact failure — and losing Called makes every rename a retraction.
func TestTheRenameEndpointCarriesWhichPlace(t *testing.T) {
	q := renameRequest("subj_abc", "Bluetooth Settings")
	if !q.Rename {
		t.Error("the request is not a rename")
	}
	if q.Place != "subj_abc" {
		t.Errorf("the request names place %q, want the one the person picked. Without it "+
			"the Director has to guess which screen was meant, which is the failure.",
			q.Place)
	}
	if q.Called != "Bluetooth Settings" {
		t.Errorf("the request carries the name %q", q.Called)
	}
	// And it is not confused with answering an open question: that is a different request,
	// bound to a proposal, and routing one through the other would let a rename settle a
	// question the person was never asked.
	if q.Start || q.Stop || q.Try || q.Cancel || q.Skip {
		t.Error("a rename also asks for something else")
	}
}

// An empty name is a RETRACTION and reaches the Director as one.
//
// Mutation: refuse an empty name at the edge. Taking a name back becomes impossible from the
// panel, and the only repair is editing semantic-memory.json — which is what happened.
func TestAnEmptyNameReachesTheDirectorAsARetraction(t *testing.T) {
	q := renameRequest("subj_abc", "")
	if !q.Rename || q.Place != "subj_abc" {
		t.Fatalf("a retraction is not a rename of that place: %+v", q)
	}
	if q.Called != "" {
		t.Errorf("the retraction carries a name (%q)", q.Called)
	}
}

// The panel's read is a read.
//
// Polling it must send no verb at all. A refresh that carried Start would begin a demonstration
// every few seconds, and the person would never find out why.
func TestPollingTheLearnPanelAsksForNothing(t *testing.T) {
	var q service.ObserveLearn
	if q.Start || q.Stop || q.Try || q.Cancel || q.Skip || q.Rename ||
		q.Name != "" || q.Called != "" || q.Place != "" {
		t.Fatal("the zero request is not empty, so polling would do something")
	}
}

// The answer endpoint is reachable and says WHICH question.
//
// Marco raises questions during a teach pass, blocks the rehearsal behind them, and reported
// "Questions open: 3" at somebody with no control that could settle any of them. This is the
// control.
func TestTheAnswerEndpointSaysWhichQuestion(t *testing.T) {
	mux := learnMux()
	req := httptest.NewRequest(http.MethodPost, "/api/learn/answer", strings.NewReader("{}"))
	if h, pattern := mux.Handler(req); h == nil || pattern != "/api/learn/answer" {
		t.Fatalf("the answer endpoint is not wired (matched %q)", pattern)
	}

	q := answerRequest("q_group", "observe_1", "confirmed")
	if q.Question != "q_group" {
		t.Errorf("the request names question %q. Answering \"the current one\" would "+
			"settle whichever happened to be first when the button was pressed.",
			q.Question)
	}
	if q.Session != "observe_1" {
		t.Errorf("the request carries session %q; an answer is routed to the session that "+
			"raised the question", q.Session)
	}
	if q.Answer != "confirmed" {
		t.Errorf("the request carries the answer %q", q.Answer)
	}
	// And it is not confused with the other verbs.
	if q.Start || q.Stop || q.Try || q.Cancel || q.Skip || q.Rename {
		t.Error("an answer also asks for something else")
	}
}

// Answering refuses a GET.
//
// A refresh must not settle a question about what Marco believes.
func TestAnsweringRefusesAGet(t *testing.T) {
	w := httptest.NewRecorder()
	learnMux().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/learn/answer", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/learn/answer answered %d, want it refused", w.Code)
	}
}

// Learn's account of a play and the Plays listing's account are the same account.
//
// # Why this test names both surfaces
//
// Because the failure it guards is not a broken handler — it is two surfaces telling a person two
// different things about one file. The Learn panel used to say "Saved. It is in the Routes tab."
// on the strength of a play existing on disk, while the tab it named could not contain a staged
// play at all. The fix is not a better sentence; it is that both surfaces read the same value out
// of internal/plays.
//
// Mutation: give addLifecycle its own words, or drop it, and this fails.
func TestLearnReportsTheSameStandingThePlaysListWould(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}
	src := "script main...\n  do nothing.\n"

	ready := routes.Route{App: "settings", Focus: routes.LearnedFocus, Slug: "registered-one"}
	if err := reg.SaveStaged(ready, src, routes.Origin{Kind: routes.KindLearned, Application: "settings"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(ready); err != nil {
		t.Fatal(err)
	}
	waiting := routes.Route{App: "settings", Focus: routes.LearnedFocus, Slug: "staged-one"}
	if err := reg.SaveStaged(waiting, src, routes.Origin{Kind: routes.KindLearned, Application: "settings"}); err != nil {
		t.Fatal(err)
	}

	listed := map[string]plays.Life{}
	for _, p := range plays.List(reg) {
		listed[p.Slug] = p.Life
	}

	for _, c := range []struct {
		what            string
		saved, learned  bool
		slug            string
		mustNotSayReady bool
	}{
		{"saved and registered", true, true, "registered-one", false},
		{"saved and not registered", true, false, "staged-one", true},
	} {
		view := map[string]any{}
		if c.saved {
			view["saved"] = true
		}
		if c.learned {
			view["learned"] = true
		}
		addLifecycle(view)

		got, ok := view["life"].(string)
		if !ok {
			t.Fatalf("%s: the panel reports no standing at all", c.what)
		}
		if want := string(listed[c.slug]); got != want {
			t.Errorf("%s: Learn says %q, the Plays list says %q — two accounts of one play",
				c.what, got, want)
		}
		word, _ := view["life_word"].(string)
		says, _ := view["life_says"].(string)
		if want := listed[c.slug].Word(); word != want {
			t.Errorf("%s: Learn's badge is %q, the listing's is %q", c.what, word, want)
		}
		if says == "" {
			t.Errorf("%s: Learn reports a standing with no sentence", c.what)
		}
		if c.mustNotSayReady {
			if strings.Contains(strings.ToLower(word), "ready") ||
				strings.Contains(strings.ToLower(says), "you can ask") {
				t.Errorf("%s: Learn calls an unregistered play askable: %q / %q",
					c.what, word, says)
			}
		}
		// And whatever it says, it may not send anybody to a tab under an old name.
		for _, s := range []string{word, says} {
			if strings.Contains(strings.ToLower(s), "route") {
				t.Errorf("%s: Learn still says route: %q", c.what, s)
			}
		}
	}

	// Nothing saved, nothing claimed.
	empty := map[string]any{"stage": "watching"}
	addLifecycle(empty)
	if _, claimed := empty["life"]; claimed {
		t.Error("the panel reports a standing for a play that was never written down")
	}
}
