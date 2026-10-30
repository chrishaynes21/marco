package routes

// A FRESH ROUTINE IS NOT PUBLISHED BESIDE PHRASES THAT COULD NOT BE CLEARED.
//
// Windows, because the case is: a file that can be READ and cannot be DELETED. That is not a
// hypothetical state on this platform and it is not reachable portably — a share mode that admits
// readers and refuses deletion is what something holding the file open looks like, and it is what
// `DeleteAliases` meets when a sync, an editor or another Marco has it.
//
// The ordinary unreadable cases are in aliases_test.go. This is the one that reads as `intact`
// the moment a source appears beside it, which is why ignoring the removal's error was wrong.

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

// heldOpenForReading holds a file the way something else on the desktop holds one: readable by
// anybody, deletable by nobody, until the handle goes.
//
// `FILE_SHARE_READ` and nothing else. Go's own `os.Open` passes read, write AND delete, so a file
// opened through it can still be removed — which is why this goes through the syscall rather than
// through the standard library, and why a test that used `os.Open` would prove nothing.
func heldOpenForReading(t *testing.T, path string) {
	t.Helper()
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ,
		nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("holding %s open: %v", path, err)
	}
	t.Cleanup(func() { _ = syscall.CloseHandle(h) })
}

// A READABLE PHRASE FILE THAT WILL NOT GO REFUSES THE CREATION, AND NO ROUTINE IS PUBLISHED.
//
// The whole of the correction in one case. Ignoring the removal's error meant this file matched
// the new identity the instant the source existed and `Aliases` reported it `intact` — a routine
// answering, from its first moment, to a phrase somebody attached to something else. A refusal
// naming the file is the only honest answer: it is one sentence, and the alternative is silent.
func TestAPhraseFileThatWillNotGoRefusesTheCreation(t *testing.T) {
	rt := Route{App: "notepad", Slug: "press-b"}
	reg := aRoutineAnsweringTo(t, rt, "tap bee")
	// THE ROUTINE GOES AND THE PHRASES DO NOT, which is what a sweep that could not
	// remove the file leaves — and then something holds the file open.
	if err := reg.DeleteSource(rt); err != nil {
		t.Fatal(err)
	}
	heldOpenForReading(t, reg.AliasesPath(rt))
	// IT IS STILL READABLE, which is the half that makes this dangerous rather than
	// merely untidy: an unreadable file would be refused by every reader anyway.
	if s, st := reg.Aliases(rt); st != AliasesOrphaned || len(s.Phrases) != 1 {
		t.Fatalf("the held file reads as %q with %d phrases, and this case is about "+
			"one that can be read", st, len(s.Phrases))
	}

	err := reg.Create(rt, "use os.\n\nthe App is a script.\n\ndo OS's Key with \"z\".\n")
	if err == nil {
		t.Fatal("a routine was created beside phrases that could not be cleared, so " +
			"it answers to a phrase nobody gave it")
	}
	// AND THE REFUSAL SAYS WHAT TO DO ABOUT IT.
	for _, want := range []string{"press-b.aliases.json", "could not be removed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
	// AND NOTHING WAS PUBLISHED. The source is what makes a routine exist, so this is
	// the difference between a refusal and a routine that answers wrongly.
	if reg.Has(rt) {
		t.Error("the source was published anyway")
	}
	if _, err := os.Stat(reg.Path(rt)); err == nil {
		t.Error("a source file was left at the identity the creation refused")
	}
}
