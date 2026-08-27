package main

import (
	"errors"
	"os"
	"path/filepath"
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
