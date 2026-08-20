package main

import "testing"

// `rehearse --live` reaches the live path.
//
// # The defect
//
// `runRehearse` declared -step, -live and -json, parsed them, and then handed the SAME unconsumed
// argument slice to `observationQuery`, whose flag set knows only -json. Every invocation carrying
// a flag died before it reached the service:
//
//	director rehearse --live               flag provided but not defined: -live
//	director rehearse observe_2 --live     flag provided but not defined: -live
//
// So the only path to learned input — the thing the whole authority mechanism exists to gate —
// could not be invoked at all, while `director rehearse` with no flags worked perfectly and made
// the command look healthy. Found live, holding a rehearsal grant that could not be spent.
//
// It survived because the arguments were never turned into a value anything could assert on: they
// were parsed for their side effects and passed on to be parsed again. The fix separates the
// parse from the request, and this test asserts the parse.
func TestRehearseArgumentsReachTheLivePath(t *testing.T) {
	// EVERY shape a person actually types, including the two that were impossible.
	for _, c := range []struct {
		name    string
		args    []string
		wantID  string
		live    bool
		step    int
		jsonOut bool
	}{
		{name: "bare", args: nil, step: 1},
		{name: "session only", args: []string{"observe_2"}, wantID: "observe_2", step: 1},
		{name: "live, no session", args: []string{"--live"}, live: true, step: 1},
		{name: "session then live", args: []string{"observe_2", "--live"},
			wantID: "observe_2", live: true, step: 1},
		{name: "live then session", args: []string{"--live", "observe_2"},
			wantID: "observe_2", live: true, step: 1},
		{name: "single dash", args: []string{"observe_2", "-live"},
			wantID: "observe_2", live: true, step: 1},
		{name: "step and live", args: []string{"observe_2", "--step", "2", "--live"},
			wantID: "observe_2", live: true, step: 2},
		{name: "json", args: []string{"observe_2", "--json"},
			wantID: "observe_2", step: 1, jsonOut: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			q, jsonOut := rehearseQuery(c.args)
			if q.Rehearse == nil {
				t.Fatal("no rehearsal was requested at all")
			}
			if q.Rehearse.Live != c.live {
				t.Errorf("live = %v, want %v.\nWhen this is false for an argument list "+
					"containing --live, the authority path cannot be reached and a "+
					"granted rehearsal cannot be spent", q.Rehearse.Live, c.live)
			}
			if q.Rehearse.Step != c.step {
				t.Errorf("step = %d, want %d", q.Rehearse.Step, c.step)
			}
			if q.ID != c.wantID {
				t.Errorf("session = %q, want %q", q.ID, c.wantID)
			}
			if jsonOut != c.jsonOut {
				t.Errorf("json = %v, want %v", jsonOut, c.jsonOut)
			}
		})
	}
}

// No command parses its arguments, then passes the same slice on to be parsed again.
//
// The shape rather than the instance. `rehearse` was the only one, and the reason it was worth a
// test of its own is that the failure is silent until somebody uses a flag: the command works,
// its --help is correct, and one specific invocation is impossible.
//
// This walks the source rather than naming functions, so a NEW command with the same shape fails
// it without anybody remembering to come back here.
func TestNoCommandParsesItsArgumentsTwice(t *testing.T) {
	for _, fn := range parseTwiceOffenders(t) {
		t.Errorf("%s declares its own flag set and then passes its arguments to a helper "+
			"that parses them again.\nAny flag it declares will be rejected by the second "+
			"parse, so that invocation is unreachable — see rehearseQuery for the fix: "+
			"parse once, pass the decision.", fn)
	}
}
