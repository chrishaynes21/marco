//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

// A forget that stops partway says which parts went and which are still there, read off the disk.
// Windows refuses to remove a file somebody has open, which stops the forget at its last step,
// the routine's own source, after everything beside it has gone.
func TestAPartialForgetSaysWhatIsStillThere(t *testing.T) {
	a, rt := aForgettableRoutine(t)
	v := a.ReviewForgetRoutine(rt.ID())
	f, err := os.Open(a.reg().Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	done := a.ForgetRoutine(v.Token)
	f.Close()
	if done.Removed || !strings.Contains(done.Says, "only partly forgotten") {
		t.Fatalf("a forget that stopped partway: %+v", done)
	}
	state := map[string]bool{}
	for _, p := range done.Parts {
		state[p.What] = p.Gone
	}
	if state["the routine itself"] {
		t.Errorf("the routine is reported gone while its file is still there: %+v", done.Parts)
	}
	if !state["the shortcut `c in notepad"] || !state["the demonstration"] {
		t.Errorf("parts that did go are not reported gone: %+v", done.Parts)
	}
	if !a.reg().Has(rt) {
		t.Error("the fixture did not hold the source")
	}
}
