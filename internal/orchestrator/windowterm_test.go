package orchestrator

// A WINDOW HANDED FROM ONE PROCESS TO ANOTHER IS ONE TERM WITH BOTH HALVES.
//
// A handle is not an identity: Windows hands the number back out, and the program NAME does not
// separate two copies of one program. So the owner travels with the handle and the receiving end
// refuses when the handle no longer belongs to that process.
//
// ONE TERM AND NOT TWO FLAGS, which is what these hold: two flags are two things a caller can
// supply half of, and half an identity reads as a whole one.
//
// See [[ADR-272-the-window-a-run-acts-in-is-chosen-and-carried]].

import "testing"

// A TERM IS WRITTEN ONLY WHEN THERE IS A WHOLE IDENTITY TO WRITE.
func TestAWindowTermCarriesBothHalvesOrNeither(t *testing.T) {
	whole := Pickable{HWnd: 4660, PID: 77, App: "notepad"}
	if got := whole.Term(); got != "4660:77" {
		t.Errorf("a window's term is %q", got)
	}
	for _, half := range []Pickable{
		{HWnd: 4660, App: "notepad"},
		{PID: 77, App: "notepad"},
		{},
	} {
		if got := half.Term(); got != "" {
			t.Errorf("%+v was written as the term %q, which names a window "+
				"whose owner nothing can check", half, got)
		}
	}
}

// AND IT IS READ BACK AS THE WINDOW IT NAMES, OR NOT AT ALL.
//
// `ok` false is how a caller learns it was given no window, so everything that is not exactly
// `<handle>:<owner>` has to come back false rather than nearly-parsed. A term that does not parse
// is NOT then looked up as a name here: `TheWindow` resolves what a person typed out of the list
// in front of them, and this reads back what a surface already resolved.
func TestAWindowTermIsReadBackAsTheWindowItNames(t *testing.T) {
	hwnd, pid, ok := GivenWindow(Pickable{HWnd: 4660, PID: 77}.Term())
	if !ok || hwnd != 4660 || pid != 77 {
		t.Fatalf("a term this package wrote read back as %#x/%d (ok=%v)", hwnd, pid, ok)
	}
	// AND SURROUNDING SPACE IS NOT A DIFFERENT WINDOW, because a term crosses a command
	// line.
	if _, _, ok := GivenWindow("  4660:77\t"); !ok {
		t.Error("a term with space around it was not read")
	}
	for _, not := range []string{
		"",          // NO WINDOW AT ALL, which is the ordinary case.
		"4660",      // A HANDLE ALONE.
		":77",       // AN OWNER ALONE.
		"4660:",     // HALF A TERM.
		"0:77",      // A HANDLE THAT IS NOT ONE.
		"4660:0",    // AN OWNER THAT IS NOT ONE.
		"notepad",   // A NAME, which this is not for.
		"4660:77:1", // MORE THAN TWO HALVES.
		"-1:77",     // NOT A NUMBER THIS CAN MEAN.
		"4660:abc",  // NOR THIS.
		"0x1234:77", // AND NOT HEX, because one side writing hex and the other
		"4660 : 77", // decimal is two spellings of one window.
	} {
		if _, _, ok := GivenWindow(not); ok {
			t.Errorf("%q was read as a window somebody chose", not)
		}
	}
}
