package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// ASKING WHETHER MARCO IS WATCHING STARTS NOTHING.
//
// # The question that must not answer itself
//
// `marco observe` is a request for something to happen, so it may start the Director that will do
// it. `marco observe status` is not, and neither is `marco observe stop`.
//
// A status read that autostarted would be the worst possible behaviour for this particular
// command: somebody checking whether Marco is watching them would, by asking, make it start. And
// `stop` bringing a Director into existence in order to stop it is absurd on its own terms.
//
// The check is that no Director exists afterwards. `directorDir` points at a temporary home with
// no endpoint file, so an autostart would have to spawn one — and a spawned Director would write
// an endpoint there.
//
// Deleting the `directorConnect(q.Enable)` distinction must fail this.
func TestAskingWhetherMarcoIsWatchingStartsNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	// A binary that does not exist, so an autostart could not silently succeed against a
	// real one sitting beside the test.
	t.Setenv("DIRECTOR_BIN", filepath.Join(home, "no-such-director.exe"))

	for _, verb := range []string{"status", "stop"} {
		t.Run(verb, func(t *testing.T) {
			if code := runObserve([]string{verb}); code != 0 {
				t.Fatalf("`observe %s` with nothing running exited %d. Nothing is "+
					"watching, which is a complete and honest answer rather "+
					"than a failure.", verb, code)
			}
			if _, err := os.Stat(filepath.Join(home, "director-service.json")); err == nil {
				t.Fatalf("`observe %s` started a Director. Asking whether Marco is "+
					"watching must never be answered by making it watch.", verb)
			}
		})
	}
}

// AND AN UNKNOWN VERB IS REFUSED RATHER THAN TREATED AS "START".
//
// The default branch matters more here than it usually would: `marco observe stpo` silently
// turning watching ON is the kind of mistake somebody would not notice.
func TestAMisspeltObserveVerbDoesNotStartWatching(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("DIRECTOR_BIN", filepath.Join(home, "no-such-director.exe"))

	if code := runObserve([]string{"stpo"}); code == 0 {
		t.Fatal("a misspelt verb was accepted; if it fell through to `start`, a typo " +
			"turns ambient watching on and says nothing about it")
	}
	if _, err := os.Stat(filepath.Join(home, "director-service.json")); err == nil {
		t.Fatal("a misspelt verb started a Director")
	}
}

// AND THE AUTOSTART FLAG IS THE CLAIM, not the absence of a spawned process.
//
// The test above points DIRECTOR_BIN at a binary that does not exist, so an autostart FAILS and
// leaves no endpoint — which means it passes whether or not the autostart was attempted.
// Measured: the mutation that lets `observe status` autostart survives it.
//
// So the flag itself is asserted, through the one dialler this command uses.
func TestOnlyAskingMarcoToWatchMayStartADirector(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)

	var asked []bool
	restore := observeDial
	observeDial = func(autoStart bool) (*service.Client, error) {
		asked = append(asked, autoStart)
		return nil, errors.New("nothing running")
	}
	t.Cleanup(func() { observeDial = restore })

	for _, c := range []struct {
		args     []string
		mayStart bool
		why      string
	}{
		{args: nil, mayStart: true,
			why: "`marco observe` is a request for something to happen, so it may " +
				"start the Director that will do it"},
		{args: []string{"status"}, mayStart: false,
			why: "a question about whether Marco is watching must never be answered " +
				"by making it watch"},
		{args: []string{"stop"}, mayStart: false,
			why: "asking it to stop must never bring something into existence to stop"},
	} {
		asked = nil
		runObserve(c.args)
		if len(asked) != 1 {
			t.Fatalf("`observe %v` dialled %d times", c.args, len(asked))
		}
		if asked[0] != c.mayStart {
			t.Errorf("`observe %v` asked for autostart=%v, want %v: %s",
				c.args, asked[0], c.mayStart, c.why)
		}
	}
}

// WATCHING SAYS WHETHER IT IS ALSO LEARNING.
//
// The sentence that must not be missing. Somebody who typed `marco observe` and walked away is
// entitled to know whether Marco is building durable memory from what it sees — and a line that
// appeared only when it was would make its absence mean two things at once.
//
// Deleting either arm must fail this.
func TestWatchingSaysWhetherItIsAlsoLearning(t *testing.T) {
	quiet := captureObserveOutput(t, func() {
		printObserving(service.AmbientView{Watching: true, Application: "settings"})
	})
	if !strings.Contains(quiet, "isn't remembering") {
		t.Errorf("watching-only does not say it is not learning:\n%s", quiet)
	}
	if !strings.Contains(quiet, "marco observe learn") {
		t.Errorf("it does not say how to let it learn:\n%s", quiet)
	}

	loud := captureObserveOutput(t, func() {
		printObserving(service.AmbientView{Watching: true, Application: "settings",
			Learning: true, Noticed: 4, Learned: 2})
	})
	if !strings.Contains(loud, "learning from what it sees") {
		t.Errorf("learning does not say so:\n%s", loud)
	}
	if !strings.Contains(loud, "2 things remembered") {
		t.Errorf("it does not say how much it has remembered:\n%s", loud)
	}
	// COUNTS ONLY. What Marco has learned is a size a person may want; WHICH things is a
	// question for the store, and naming one here would be this command reading somebody's
	// own afternoon back to them.
	for _, leak := range []string{"subj_", "Bluetooth", "watched_"} {
		if strings.Contains(loud, leak) {
			t.Errorf("it printed %q:\n%s", leak, loud)
		}
	}
}

// captureObserveOutput runs fn with os.Stdout redirected and returns what it printed.
func captureObserveOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	os.Stdout = saved
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// TURNING LEARNING OFF IS ITS OWN VERB.
//
// `marco observe learn` and `marco observe learn off` are two different requests, and the second
// must never be the first. A person switching learning off and being given more of it is the worst
// possible misreading of a command about permanence.
//
// The decision is asserted where it is MADE. Everything else in the command needs a Director on
// the other end of a socket; what verb means what does not, and a test that spawned a service to
// find out would be testing the socket.
//
// Deleting the off arm must fail this.
func TestTurningLearningOffIsItsOwnVerb(t *testing.T) {
	for _, c := range []struct {
		sub  string
		rest []string
		want service.ObserveAmbient
		why  string
	}{
		{sub: "learn", want: service.ObserveAmbient{Learn: true},
			why: "`observe learn` asks Marco to learn from what it sees"},
		{sub: "learn", rest: []string{"off"}, want: service.ObserveAmbient{Unlearn: true},
			why: "`observe learn off` asks it to stop, and must never ask for more"},
		{sub: "learn", rest: []string{"stop"}, want: service.ObserveAmbient{Unlearn: true},
			why: "stop means off here, as it does everywhere else"},
		{sub: "status", want: service.ObserveAmbient{},
			why: "a status read asks for neither lifecycle"},
		{sub: "", want: service.ObserveAmbient{Enable: true},
			why: "watching is its own lifecycle and must not turn learning on with it"},
		{sub: "stop", want: service.ObserveAmbient{Disable: true},
			why: "stopping watching is not a statement about memory"},
	} {
		got, ok := observeRequest(c.sub, c.rest)
		if !ok {
			t.Errorf("`observe %s %v` was refused", c.sub, c.rest)
			continue
		}
		if got != c.want {
			t.Errorf("`observe %s %v` asked %+v, want %+v: %s",
				c.sub, c.rest, got, c.want, c.why)
		}
	}
	if _, ok := observeRequest("stpo", nil); ok {
		t.Error("a misspelt verb was accepted; if it fell through to something that changes " +
			"state, a typo does it silently")
	}
}
