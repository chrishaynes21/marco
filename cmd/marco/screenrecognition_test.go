package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/platform/screenhost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Which place is showing right now, asked by the process that runs the play.
//
// # The hole this closes
//
// `liveScreens.CurrentSubject` returned `screenhost.Unavailable` unconditionally, with a comment
// explaining — correctly — that standalone Marco has no perception. The consequence was not
// survivable. Every learned play's generated Marco opens with `do Screen's Showing with "<place>"`.
// A learned play with intact provenance is delegated to the Director and never runs that line
// here; an EDITED one is not, because `orchestrator.Resolved.Learned()` means learned AND
// provenance-verified. So the Plays view offered an edit button on every row, the authority seam
// told the person in as many words "You have changed this one since Marco wrote it, so it runs
// like anything else you have written" — and then the play refused at its own first line with
// "Marco could not check".
//
// # The half of this file that may never be weakened
//
// From 34F's own risk table: "The fastest way to make this go away is to let CurrentSubject guess.
// That destroys ADR-031-the-user-names-the-stage and the 'silence is never yes' invariant in one
// line." So there are more refusal tests here than success tests, and they are the ones to defend.
// A future change that makes TestTheDirectorsAnswerSatisfiesTheEntryGuard pass by weakening
// TestADirectorThatDoesNotRecogniseTheScreenRefuses has made the product worse, not better.

// ── a Director on the wire, and nothing else ─────────────────────────────────

// showingDirector is a real socket speaking the real protocol, answering SHOWING with a scripted
// view.
//
// A socket rather than a substituted function, deliberately. What broke here was a WIRE question —
// can this process ask the Director what it can see — so the thing under test is the endpoint file,
// the handshake, the request type, the payload shape and the decode. A stubbed `CurrentSubject`
// would have proved none of it, and the protocol version paragraph this milestone added is about
// precisely the failure a stub cannot see.
type showingDirector struct {
	mu sync.Mutex
	// asked is every SHOWING query that arrived, so a test can assert what was asked as well
	// as what came back.
	asked []service.ObserveShowing
	// view is the answer. `raw`, when set, is sent instead — for the malformed-reply cases,
	// which are the ones a version disagreement actually produces.
	view service.ShowingView
	raw  json.RawMessage
	// other counts requests that were NOT a SHOWING query, so "it asked the Director to do
	// something" is distinguishable from "it asked the Director a question".
	other int
}

func (d *showingDirector) queries() []service.ObserveShowing {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]service.ObserveShowing(nil), d.asked...)
}

func (d *showingDirector) otherRequests() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.other
}

// serveShowingDirector publishes a fake Director in `home` and answers until the test ends.
func serveShowingDirector(t *testing.T, home string, d *showingDirector) *showingDirector {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	token, err := service.NewToken()
	if err != nil {
		t.Fatalf("minting a token: %v", err)
	}
	if err := service.WriteEndpoint(home, service.Endpoint{
		ProtocolVersion: service.ProtocolVersion,
		Address:         ln.Addr().String(),
		Token:           token,
		PID:             os.Getpid(),
		StartedAt:       time.Now(),
	}); err != nil {
		t.Fatalf("publishing the endpoint: %v", err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go d.serve(conn, token)
		}
	}()
	return d
}

func (d *showingDirector) serve(conn net.Conn, token string) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != token {
		return
	}
	if _, err := conn.Write([]byte("ok\n")); err != nil {
		return
	}
	enc := json.NewEncoder(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		var req service.RequestEnvelope
		if json.Unmarshal([]byte(line), &req) != nil {
			return
		}
		if req.Type != service.RequestObservation {
			d.mu.Lock()
			d.other++
			d.mu.Unlock()
			_ = enc.Encode(service.NewResponse(req.RequestID, service.ResponseStatus, nil))
			continue
		}
		var q service.ObserveQuery
		_ = req.Decode(&q)
		if q.Showing == nil {
			d.mu.Lock()
			d.other++
			d.mu.Unlock()
			_ = enc.Encode(service.NewResponse(req.RequestID, service.ResponseError,
				service.ErrorPayload{Code: "observation", Message: "not a showing query"}))
			continue
		}
		d.mu.Lock()
		d.asked = append(d.asked, *q.Showing)
		raw, view := d.raw, d.view
		d.mu.Unlock()

		resp := service.NewResponse(req.RequestID, service.ResponsePerception, view)
		if raw != nil {
			resp.Payload = raw
		}
		_ = enc.Encode(resp)
	}
}

// coldHome is a Marco whose home, routes and memory are all this test's own.
//
// MANDATORY in every test in this file. Without it `directorConnect` reads the developer's
// $MARCO_HOME and can reach the live Director running on the machine that is executing the suite —
// which is slow, non-deterministic, and a look taken at somebody's actual desktop.
func coldHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_MEMORY", "")
	return home
}

// stagedApplication is the PRODUCTION reader with exactly one thing overridden: which application
// is in front.
//
// `liveScreens.Application` is `winctx.Active()` — the real foreground process on the machine
// running the suite. There is no honest way to make that deterministic, and no reason to: the
// question this file is about is `CurrentSubject`, which is embedded here unmodified along with
// `SubjectNamed`. What is faked is the one input a test cannot control; what is tested is the real
// code reading the real store and talking over the real socket.
type stagedApplication struct {
	*liveScreens
	app string
}

func (s *stagedApplication) Application() string { return s.app }

// productionRecogniser builds the real backing through the real composition root.
func productionRecogniser(t *testing.T) *liveScreens {
	t.Helper()
	rec := newScreenRecognition()
	if rec == nil {
		t.Fatal("the production composition root built no recogniser")
	}
	live, ok := rec.(*liveScreens)
	if !ok {
		t.Fatalf("the production recogniser is a %T; this file tests liveScreens", rec)
	}
	return live
}

// ── refusal: no Director, no sight, and no service start either ──────────────

// WITH NO DIRECTOR REACHABLE, MARCO CANNOT SEE, AND SAYS SO.
//
// Two claims, and the second is as load-bearing as the first.
//
// It REFUSES. `unavailable` and no subject id — never a guess, never the last place it knew, never
// "the person asked for it so it is probably fine". This is [[ADR-031-the-user-names-the-stage]]
// Decision 4 unchanged: a Marco that cannot establish where it is "does not skip the guard, assume
// ok, fall back to OCR text matching, or degrade into blind replay."
//
// And it does not START one. `directorConnect(false)` is the whole of that decision, and the
// evidence is the startup lock: `service.Connect` takes `director-service.lock` before it spawns
// anything, so a run that leaves no lock and no endpoint behind never tried. A read must not pay
// for a twenty-second service launch — the reasoning `pendingQuestion` in intake.go states, and
// the cost here is charged to a play's first line.
//
// Changing `directorConnect(false)` to `true` must fail this.
func TestNoDirectorMeansMarcoCannotSeeAndSaysSo(t *testing.T) {
	home := coldHome(t)
	// A binary that does not exist, so a spawn attempt is loud rather than a real Director.
	t.Setenv("DIRECTOR_BIN", filepath.Join(home, "no-such-director.exe"))
	rec := productionRecogniser(t)

	for _, tc := range []struct {
		name string
		// stale writes an endpoint file pointing at a port nothing answers on — a service
		// that was killed without cleaning up, which is the case a client must not treat as
		// permission to start a new one.
		stale bool
	}{
		{"nothing has ever run here", false},
		{"a dead service left its endpoint file behind", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service.RemoveEndpoint(home)
			if tc.stale {
				writeDeadEndpoint(t, home)
			}
			id, outcome := rec.CurrentSubject("testgame")
			if outcome != screenhost.Unavailable {
				t.Fatalf("with no Director reachable the answer is %q/%q; it must be %q. "+
					"Anything else is Marco reporting a look it never took.",
					id, outcome, screenhost.Unavailable)
			}
			if id != "" {
				t.Fatalf("a subject id (%q) came back from a Director that is not there", id)
			}
			if _, err := os.Stat(filepath.Join(home, "director-service.lock")); err == nil {
				t.Fatal("a startup lock was left behind: asking what is on screen tried to " +
					"LAUNCH a Director. A read must not cost a service start.")
			}
			if tc.stale {
				// THE STRUCTURAL PROOF, and the reason this subtest exists.
				//
				// `service.Connect` with AutoStart takes the startup lock, re-reads the
				// endpoint under it, and DELETES a stale one before spawning — and it
				// releases the lock on the way out, so the lock file itself is gone by the
				// time a test can look at it. The endpoint file is not: it survives a
				// connect that never tried to start anything, and only that.
				//
				// Changing `directorConnect(false)` to `true` must fail here.
				if _, err := os.Stat(service.EndpointPath(home)); err != nil {
					t.Fatalf("the stale endpoint file is gone (%v). Only the AUTO-START path "+
						"removes it, so asking what is on screen tried to launch a Director.",
						err)
				}
			} else {
				if _, err := os.Stat(service.EndpointPath(home)); err == nil {
					t.Fatal("an endpoint file appeared; something started a service")
				}
			}
		})
	}
}

// writeDeadEndpoint publishes an endpoint pointing at a port nothing is listening on.
func writeDeadEndpoint(t *testing.T, home string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close() // the port is now dead, and was certainly free
	if err := service.WriteEndpoint(home, service.Endpoint{
		ProtocolVersion: service.ProtocolVersion, Address: addr,
		Token: "stale", PID: os.Getpid(), StartedAt: time.Now(),
	}); err != nil {
		t.Fatalf("publishing a dead endpoint: %v", err)
	}
}

// ── the success case: the Director's eyes satisfy the play's guard ───────────

// A DIRECTOR THAT RECOGNISES THE PLACE SATISFIES THE ENTRY CONDITION.
//
// The whole chain, through production parts: the composition root opens semantic memory, the store
// resolves the name the person gave, `CurrentSubject` dials the endpoint and asks a SHOWING query
// over the real protocol, and `screenhost.Host` compares the two ids and answers `ok`.
//
// Deleting the Director query from CurrentSubject — returning `screenhost.Unavailable` as it used
// to — must fail this.
func TestTheDirectorsAnswerSatisfiesTheEntryGuard(t *testing.T) {
	home := coldHome(t)
	here := namedPlace(t, filepath.Join(home, "semantic-memory.json"), "testgame", "the pause menu")
	d := serveShowingDirector(t, home, &showingDirector{
		view: service.ShowingView{
			Application: "testgame", Outcome: string(screenhost.Recognised), Subject: here,
		},
	})

	h := screenhost.New(&stagedApplication{liveScreens: productionRecogniser(t), app: "testgame"})
	status, _, err := h.Invoke(runtime.HostCall{
		Act: "Screen", Action: "Showing", Input: runtime.Text("the pause menu"), Out: os.Stderr,
	})
	if err != nil {
		t.Fatalf("asking Screen's Showing: %v", err)
	}
	if status != "ok" {
		t.Fatalf("the guard answered %q (%s) while the Director reported the very place the "+
			"play names. Marco has eyes here and did not use them.", status, h.Why())
	}
	asked := d.queries()
	if len(asked) != 1 {
		t.Fatalf("the Director was asked %d time(s); the guard is answered by exactly one "+
			"question", len(asked))
	}
	if asked[0].Application != "testgame" {
		t.Errorf("the look was scoped to %q, not to the application the play is in. A guard "+
			"satisfiable by another program is worse than no guard.", asked[0].Application)
	}
	if d.otherRequests() != 0 {
		t.Errorf("%d request(s) other than a SHOWING query went out. Asking where you are "+
			"must not be a way of doing something.", d.otherRequests())
	}
}

// ── refusal: everything that is not a positive identification ────────────────

// A DIRECTOR THAT DOES NOT RECOGNISE THE SCREEN MAKES THE PLAY REFUSE.
//
// Every way of failing to establish that this IS the named place, including the ones that arrive
// looking like a success. There is no path through `CurrentSubject` that answers `ok` without a
// durable subject id that positively matches the name the person gave.
//
// The last three cases are the version-disagreement shapes the ProtocolVersion 9 paragraph argues
// about: a reply whose outcome word this build has never heard of, a reply that is some other
// type's JSON entirely (what a version-8 Director would send back), and a reply claiming
// recognition with nothing recognised. All of them are "I could not check", and none of them is a
// match.
//
// Loosening any arm of the switch in CurrentSubject — a default that falls through to Recognised,
// or trusting Subject without checking Outcome — must fail this.
func TestADirectorThatDoesNotRecogniseTheScreenRefuses(t *testing.T) {
	home := coldHome(t)
	here := namedPlace(t, filepath.Join(home, "semantic-memory.json"), "testgame", "the pause menu")
	script := &showingDirector{}
	serveShowingDirector(t, home, script)

	h := screenhost.New(&stagedApplication{liveScreens: productionRecogniser(t), app: "testgame"})

	// `why` is the diagnostic sentence the refusal must carry.
	//
	// Asserted as well as the refusal itself, because the two are separate claims and only one of
	// them is visible in `status`. "I could not look" and "I looked and it was different" both
	// answer `failed` at the language boundary and send a person to OPPOSITE fixes — one is "start
	// the Director", the other is "you are on the wrong screen". Collapsing them would be
	// undetectable without this column, and the empty-subject arm below is a case where it is the
	// ONLY thing that changes.
	for _, tc := range []struct {
		name string
		view service.ShowingView
		raw  json.RawMessage
		why  string
	}{
		{name: "a different screen", why: "this is a different screen",
			view: service.ShowingView{
				Outcome: string(screenhost.Recognised), Subject: "subj_somewhere_else"}},
		{name: "nothing remembered matches", why: "Marco does not recognise this screen",
			view: service.ShowingView{Outcome: string(screenhost.Unknown)}},
		{name: "the look could not be taken", why: "Marco could not see the screen",
			view: service.ShowingView{Outcome: string(screenhost.Unobservable)}},
		{name: "the screen could be more than one",
			why:  "this screen could be more than one Marco remembers",
			view: service.ShowingView{Outcome: string(screenhost.Ambiguous)}},
		{name: "the Director says it cannot check", why: "Marco could not check",
			view: service.ShowingView{Outcome: string(screenhost.Unavailable)}},
		{name: "recognised, with nothing recognised", why: "Marco could not check",
			view: service.ShowingView{Outcome: string(screenhost.Recognised), Subject: ""}},
		{name: "an outcome word this build has never heard of", why: "Marco could not check",
			view: service.ShowingView{Outcome: "probably", Subject: here}},
		{name: "some other type's JSON", why: "Marco could not check",
			raw: json.RawMessage(`{"id":"observe_1","application":"testgame","state":"observing"}`)},
		{name: "not JSON at all", why: "Marco could not check",
			raw: json.RawMessage(`"nonsense"`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script.mu.Lock()
			script.view, script.raw = tc.view, tc.raw
			script.mu.Unlock()

			status, _, err := h.Invoke(runtime.HostCall{
				Act: "Screen", Action: "Showing", Input: runtime.Text("the pause menu"),
				Out: os.Stderr,
			})
			if err != nil {
				t.Fatalf("asking Screen's Showing: %v", err)
			}
			if status != "failed" {
				t.Fatalf("%s answered %q. The named place was NOT positively identified, and "+
					"the only honest answer to that is a refusal — letting this through is "+
					"how a play begins on the wrong screen.", tc.name, status)
			}
			if got := h.Why(); got != tc.why {
				t.Errorf("the refusal reads %q, want %q. Both are refusals, and they send a "+
					"person to different fixes; reporting one as the other is a bug report "+
					"about the wrong thing.", got, tc.why)
			}
		})
	}
}

// ── the reason the work exists ───────────────────────────────────────────────

// AN EDITED LEARNED PLAY RUNS ITS OWN EDITED SOURCE, AND ITS GUARD IS ANSWERED.
//
// The end-to-end claim, entered where every invocation enters: `runInvocation`.
//
// Three things have to be true at once, and before this milestone the third was false:
//
//  1. Editing a learned play makes it ORDINARY. `Resolved.Learned()` is false the moment the
//     source stops matching its provenance, and the authority seam allows it with the sentence
//     the person is shown.
//  2. An ordinary play runs LOCALLY, from its own file. Not re-planned by the Director from the
//     goal — `performLearned` sends only Name/Application/Subject, so delegating an edited play
//     would silently discard the edit. The assertion below is that the key the person changed is
//     the key that goes out.
//  3. Its entry guard is ANSWERED. The Director is asked what is on screen, says so, and the play
//     begins. This is the line that used to refuse.
//
// Reverting CurrentSubject to an unconditional `screenhost.Unavailable` must fail this, and it
// fails on the assertion that matters: zero keys.
func TestAnEditedLearnedPlayRunsItsOwnSourceAndItsGuardIsAnswered(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1") // no global hooks in a test
	home := coldHome(t)
	here := namedPlace(t, filepath.Join(home, "semantic-memory.json"), "testgame", "the pause menu")
	d := serveShowingDirector(t, home, &showingDirector{
		view: service.ShowingView{
			Application: "testgame", Outcome: string(screenhost.Recognised), Subject: here,
		},
	})

	// The play Director would actually write, guard and all, from the production generator.
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "the pause menu", "",
		[][]string{{"down", "confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if !strings.Contains(src, `do Screen's Showing with "the pause menu"...`) {
		t.Fatalf("the generated play carries no entry condition:\n%s", src)
	}
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "testgame", Slug: "volume"}
	if err := reg.SaveWithOrigin(rt, src, routes.Origin{
		Kind: routes.KindLearned, Application: "testgame",
		From: "subj_a", To: "subj_b", Sequence: 1, Evidence: "e1",
	}); err != nil {
		t.Fatalf("saving: %v", err)
	}
	if r := orchestrator.Classify(reg, rt, "volume"); !r.Learned() {
		t.Fatalf("the fixture is not a learned play (kind=%s provenance=%s)", r.Kind, r.Provenance)
	}

	// THE PERSON EDITS IT. One key changed, which is the smallest edit that is observable in
	// what the host is asked to do.
	edited := strings.Replace(src, `do OS's Navigate with "down".`, `do OS's Navigate with "up".`, 1)
	if edited == src {
		t.Fatalf("the fixture edit changed nothing; the generated shape moved:\n%s", src)
	}
	if err := os.WriteFile(reg.Path(rt), []byte(edited), 0o644); err != nil {
		t.Fatalf("writing the edit: %v", err)
	}

	// PREMISE 1: it is now an ordinary play, and the seam says so in the person's words.
	resolved := orchestrator.Classify(reg, rt, "volume")
	if resolved.Provenance != routes.OriginEdited {
		t.Fatalf("provenance is %q after an edit, want %q", resolved.Provenance, routes.OriginEdited)
	}
	if resolved.Learned() {
		t.Fatal("an edited play still reads as learned; it would be delegated to the Director " +
			"and the person's edit discarded")
	}
	decision := orchestrator.Authorize(resolved, nil)
	if !decision.Allow() || decision.Reason != orchestrator.ReasonEditedSinceLearned {
		t.Fatalf("the door answered %+v; an edited learned play is the person's own writing", decision)
	}

	host := &recordHost{}
	deps := orchestrator.Deps{
		Reg: reg,
		Hosts: map[string]runtime.Host{
			"*": host,
			// The Screen act wired as the composition root wires it, over the real socket.
			"Screen": screenhost.New(&stagedApplication{
				liveScreens: productionRecogniser(t), app: "testgame",
			}),
		},
		In:  strings.NewReader(""),
		Out: &strings.Builder{},
		App: func() string { return "testgame" },
	}

	outcome, err := doAsProduct(t, deps, "volume", nil, nil)
	if err != nil {
		t.Fatalf("marco do: %v", err)
	}
	if outcome != OutcomePerformed {
		t.Fatalf("the invocation reported %q, want %q", outcome, OutcomePerformed)
	}

	// PREMISE 3, and the headline: the guard was answered, so the play began.
	pressed := host.pressed()
	want := []string{`OS's Navigate with up`, `OS's Navigate with confirm`}
	if len(pressed) != len(want) {
		t.Fatalf("the host was asked for %v, want %v. Zero calls means the entry guard refused "+
			"— which is the whole defect: the Plays view offers an edit button, and an edited "+
			"play could not run.", pressed, want)
	}
	for i := range want {
		if pressed[i] != want[i] {
			t.Fatalf("call %d is %q, want %q — the play did not run its OWN edited source",
				i+1, pressed[i], want[i])
		}
	}
	if n := len(d.queries()); n != 1 {
		t.Errorf("the Director was asked %d time(s) what is on screen; the play's one guard is "+
			"one question", n)
	}
}
