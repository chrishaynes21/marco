package service

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// THE DIRECTOR'S REFUSAL REACHES THE PERSON WHO CAUSED IT.
//
// # The live failure
//
// `director serve` refuses to start when the accessibility provider is missing, and says so
// precisely: `director: accessibility bridge not found at <path>`. One line, on stderr, naming
// the exact file.
//
// The client that auto-started it set `cmd.Stderr = nil`. So that line went nowhere, the wait
// timed out, and the user was told "the Director service did not become ready within 20s — try
// running `director serve` in another terminal to see why". The answer had already been given,
// on their machine, a second earlier, and they were being sent to fetch it again by hand.
//
// Mutation this kills: restoring `cmd.Stderr = nil` in startService, or dropping the startup log
// from the error waitForService returns.
func TestTheDirectorsStartupRefusalReachesTheCaller(t *testing.T) {
	const refusal = "director: accessibility bridge not found at C:\\nowhere\\uia.exe"

	// The "service" is this test binary re-invoked to run the helper below, which prints the
	// refusal and exits without ever publishing an endpoint — exactly what a Director that
	// cannot start does. A real executable is needed because the claim is about a CHILD
	// process's stderr crossing the detached-spawn boundary.
	t.Setenv(startupChildEnv, refusal)

	_, err := Connect(ConnectOptions{
		Dir:          t.TempDir(),
		ServiceBin:   os.Args[0],
		ServiceArgs:  []string{"-test.run=TestStartupLogHelperProcess", "-test.v=false"},
		AutoStart:    true,
		StartTimeout: 2 * time.Second,
		DialTimeout:  100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("Connect succeeded against a service that never started")
	}
	if !strings.Contains(err.Error(), refusal) {
		t.Fatalf("the Director's own account of why it would not start never reached the "+
			"caller.\n  got:  %v\n  want it to contain: %s\n"+
			"Discarding the child's stderr turns a sentence naming the missing file into "+
			"a timeout that tells the user to go and find it themselves.", err, refusal)
	}
}

const startupChildEnv = "MARCO_TEST_STARTUP_REFUSAL"

// TestStartupLogHelperProcess is the spawned "Director". Not a test: it is the child half of the
// test above, run by name in a second process, and it does nothing at all when invoked normally.
func TestStartupLogHelperProcess(t *testing.T) {
	refusal := os.Getenv(startupChildEnv)
	if refusal == "" {
		t.Skip("not the spawned child")
	}
	fmt.Fprintln(os.Stderr, refusal)
}

// The capture is BOUNDED and keeps draining past the bound.
//
// A Director that logs steadily must not grow the client that watched it start, and a client
// that stopped reading would leave the pipe to fill — which blocks the writer, so the
// diagnostic would wedge the very service it was there to explain.
//
// Mutation this kills: removing the startupLogLimit clamp in add, or making text() report the
// whole buffer without marking that it was cut.
func TestTheStartupLogIsBounded(t *testing.T) {
	l := &startupLog{}
	line := strings.Repeat("x", 1024) + "\n"
	for i := 0; i < 64; i++ { // 64 KiB, eight times the bound
		l.add([]byte(line))
	}
	if len(l.buf) > startupLogLimit {
		t.Errorf("the startup log grew to %d bytes, past its %d-byte bound; a chatty "+
			"service would grow every client that ever started one",
			len(l.buf), startupLogLimit)
	}
	got := l.text()
	if !strings.HasSuffix(got, "…") {
		t.Errorf("a truncated log did not say it was truncated: %q", got[max(0, len(got)-40):])
	}
}

// Nothing said is nothing reported.
//
// A service that started fine and simply lost the race to the deadline must not be described by
// an empty quotation — the generic sentence, with its timeout and its next step, is the honest
// answer when the child said nothing.
func TestASilentStartupKeepsTheGenericSentence(t *testing.T) {
	if got := (&startupLog{}).text(); got != "" {
		t.Errorf("an empty startup log rendered as %q", got)
	}
	if got := (*startupLog)(nil).text(); got != "" {
		t.Errorf("a nil startup log (this client did not spawn it) rendered as %q", got)
	}
}
