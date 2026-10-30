package routes

// THE ACCOUNT FILE'S OWN RULES, AND WHAT EVERY OPERATION ON A ROUTINE DOES TO IT.
//
// What this file holds is narrow and load-bearing: an account is believed only while it still
// describes both files beside it, a file that cannot answer is never read as an absent one, and an
// account with no digests cannot be written at all — because a file nothing can check is the one
// state the whole design exists to prevent.
//
// See [[Step-editing-proposal-2026-10-30]].

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const accountSrc = "use os.\n\nthe App is a script.\n\ndo OS's Key with \"a\".\n"

const someSteps = `[{"kind":"key","key":"a"}]`

// anAccountedRoutine is a routine with a source, a demonstration, and an account of itself that
// describes them both.
func anAccountedRoutine(t *testing.T) (Registry, Route) {
	t.Helper()
	reg := Registry{Dir: t.TempDir()}
	rt := Route{App: "notepad", Focus: true, Slug: "press-a"}
	if err := reg.Save(rt, accountSrc); err != nil {
		t.Fatal(err)
	}
	if err := reg.SaveRecording(rt, []byte(`[{"kind":"key"}]`)); err != nil {
		t.Fatal(err)
	}
	rec, _ := reg.LoadRecording(rt)
	if err := reg.SaveSteps(rt, StoredSteps{Source: DigestOf(accountSrc),
		Demonstration: DigestOf(string(rec)), Steps: []byte(someSteps)}); err != nil {
		t.Fatal(err)
	}
	if _, st := reg.Steps(rt); st != StepsIntact {
		t.Fatalf("the fixture's account reads %q", st)
	}
	return reg, rt
}

// AN ACCOUNT IS BELIEVED ONLY WHILE IT STILL DESCRIBES BOTH FILES BESIDE IT.
//
// Six answers, and exactly one of them licenses rendering. A digest that no longer matches means
// the account is not about the routine in front of it — which is the whole of "a stale account
// must never silently replace the executable source's meaning".
func TestAnAccountIsBelievedOnlyWhileItDescribesWhatIsOnDisk(t *testing.T) {
	for _, c := range []struct {
		what   string
		want   StepsState
		break_ func(t *testing.T, reg Registry, rt Route)
	}{
		{"nothing changed", StepsIntact, func(*testing.T, Registry, Route) {}},
		{"the source moved", StepsMismatched, func(t *testing.T, reg Registry, rt Route) {
			if err := reg.Save(rt, accountSrc+"\n"); err != nil {
				t.Fatal(err)
			}
		}},
		{"the demonstration moved", StepsAdrift, func(t *testing.T, reg Registry, rt Route) {
			if err := reg.SaveRecording(rt, []byte(`[{"kind":"click"}]`)); err != nil {
				t.Fatal(err)
			}
		}},
		{"the demonstration went", StepsAdrift, func(t *testing.T, reg Registry, rt Route) {
			if err := os.Remove(reg.RecordingPath(rt)); err != nil {
				t.Fatal(err)
			}
		}},
		{"the source went", StepsOrphaned, func(t *testing.T, reg Registry, rt Route) {
			if err := reg.DeleteSource(rt); err != nil {
				t.Fatal(err)
			}
		}},
		{"it is not JSON", StepsUnreadable, func(t *testing.T, reg Registry, rt Route) {
			if err := os.WriteFile(reg.StepsPath(rt), []byte("{{{"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"it states a version from the future", StepsUnreadable, func(t *testing.T, reg Registry, rt Route) {
			if err := os.WriteFile(reg.StepsPath(rt),
				[]byte(`{"version":99,"source":"a","demonstration":"b",`+
					`"steps":[{"kind":"key"}]}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"it says which source but not which demonstration", StepsUnreadable,
			func(t *testing.T, reg Registry, rt Route) {
				if err := os.WriteFile(reg.StepsPath(rt),
					[]byte(`{"version":1,"source":"`+DigestOf(accountSrc)+
						`","steps":[{"kind":"key"}]}`), 0o644); err != nil {
					t.Fatal(err)
				}
			}},
		{"it has no steps in it", StepsUnreadable, func(t *testing.T, reg Registry, rt Route) {
			if err := os.WriteFile(reg.StepsPath(rt),
				[]byte(`{"version":1,"source":"a","demonstration":"b"}`),
				0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"it is there and cannot be read", StepsUnreadable, func(t *testing.T, reg Registry, rt Route) {
			// A DIRECTORY IN THE FILE'S PLACE. Every read error used to answer
			// StepsNone, which is the one answer that licenses rendering the
			// demonstration instead and writing over whatever the file says.
			if err := os.Remove(reg.StepsPath(rt)); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(reg.StepsPath(rt), "in-the-way"),
				0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"there is none", StepsNone, func(t *testing.T, reg Registry, rt Route) {
			if err := reg.DeleteSteps(rt); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(c.what, func(t *testing.T) {
			reg, rt := anAccountedRoutine(t)
			c.break_(t, reg, rt)
			_, st := reg.Steps(rt)
			if st != c.want {
				t.Errorf("the account reads %q, want %q", st, c.want)
			}
			if st.Usable() != (c.want == StepsIntact) {
				t.Errorf("%q is usable: %v", st, st.Usable())
			}
		})
	}
}

// AND THE DIGEST TELLS "THERE IS NONE" FROM "IT COULD NOT BE READ".
//
// A review binds itself to this digest and compares it again at the write. An unreadable file
// answering "" would answer the same as an absent one, so a review bound to "" would compare ""
// with "" and go ahead — having never read one of the three files it is about. Three answers, and
// the third is the whole reason this returns two values.
func TestTheAccountsDigestTellsAbsentFromUnreadable(t *testing.T) {
	reg, rt := anAccountedRoutine(t)
	at, readable := reg.StepsDigest(rt)
	if !readable || at == "" {
		t.Fatalf("a readable account reports (%q, %v)", at, readable)
	}

	// GENUINELY ABSENT: readable, and no digest.
	if err := reg.DeleteSteps(rt); err != nil {
		t.Fatal(err)
	}
	if gone, readable := reg.StepsDigest(rt); !readable || gone != "" {
		t.Errorf("no file at all reports (%q, %v)", gone, readable)
	}

	// THERE AND UNREADABLE: not readable, and NOT the same answer as absent.
	if err := os.MkdirAll(filepath.Join(reg.StepsPath(rt), "in-the-way"), 0o755); err != nil {
		t.Fatal(err)
	}
	if bad, readable := reg.StepsDigest(rt); readable {
		t.Errorf("a path that cannot be read reports (%q, readable), which is the "+
			"same answer an absent file gives", bad)
	}
}

// AND AN ACCOUNT NOTHING CAN CHECK CANNOT BE WRITTEN.
//
// A file with no digests is precisely the hazard every rule above exists to prevent: it would be
// believed about whatever happened to be beside it. Refused at the write rather than tolerated at
// the read, so no caller can produce one by forgetting.
func TestAnAccountThatSaysNothingAboutWhatItDescribesIsNotWritten(t *testing.T) {
	reg := Registry{Dir: t.TempDir()}
	rt := Route{App: "notepad", Slug: "press-a"}
	for _, c := range []struct {
		what string
		s    StoredSteps
		says string
	}{
		{"no source digest", StoredSteps{Demonstration: "b",
			Steps: []byte(someSteps)}, "which source"},
		{"no demonstration digest", StoredSteps{Source: "a",
			Steps: []byte(someSteps)}, "which source"},
		{"no steps", StoredSteps{Source: "a", Demonstration: "b"}, "no steps in it"},
	} {
		t.Run(c.what, func(t *testing.T) {
			err := reg.SaveSteps(rt, c.s)
			if err == nil {
				t.Fatal("it was written")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal does not say %q: %v", c.says, err)
			}
			if reg.HasSteps(rt) {
				t.Error("a file was left behind")
			}
		})
	}
	// AND A ROUTE WITH NO NAME IS NOT A ROUTE.
	if err := reg.SaveSteps(Route{}, StoredSteps{Source: "a", Demonstration: "b",
		Steps: []byte(someSteps)}); err == nil {
		t.Error("an account was written for a routine with no name")
	}
}

// AND THE VERSION IS SET BY THE WRITER, NOT BY THE CALLER.
//
// A caller cannot save a file claiming to be a shape this build does not write, which is what
// makes "a version this build does not know" mean what it says.
func TestTheWriterStatesTheVersion(t *testing.T) {
	reg, rt := anAccountedRoutine(t)
	rec, _ := reg.LoadRecording(rt)
	if err := reg.SaveSteps(rt, StoredSteps{Version: 99, Source: DigestOf(accountSrc),
		Demonstration: DigestOf(string(rec)), Steps: []byte(someSteps)}); err != nil {
		t.Fatal(err)
	}
	stored, st := reg.Steps(rt)
	if st != StepsIntact {
		t.Fatalf("the account reads %q", st)
	}
	if stored.Version != StepsVersion {
		t.Errorf("the written account states version %d", stored.Version)
	}
}

// RENAMING A ROUTINE CARRIES ITS ACCOUNT.
//
// The same argument the provenance sidecar's rename makes, unchanged: the digests are over the
// source and the demonstration, and renaming changes the contents of neither — so the account
// still describes the files at their new name. Left behind it would be an edited routine reading
// as one nobody explained, plus an orphan under the old slug for the next routine saved there.
func TestRenamingAnEditedRoutineCarriesItsAccount(t *testing.T) {
	reg, from := anAccountedRoutine(t)
	to := Route{App: from.App, Focus: from.Focus, Slug: "new-name"}
	if err := reg.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	stored, st := reg.Steps(to)
	if st != StepsIntact {
		t.Fatalf("the renamed routine's account reads %q", st)
	}
	// THE SAME STEPS, read back through the indentation `MarshalIndent` adds.
	if !strings.Contains(strings.Join(strings.Fields(string(stored.Steps)), ""),
		`"kind":"key"`) {
		t.Errorf("the account's steps are %s", stored.Steps)
	}
	if reg.HasSteps(from) {
		t.Error("renaming left an orphaned account under the old name")
	}
}

// DELETING A ROUTINE TAKES ITS ACCOUNT WITH IT.
//
// The digests would refuse the inheritance on their own — an orphan describes a source that is
// gone. The digest is the backstop; removal is the intent.
func TestDeletingARoutineTakesItsAccount(t *testing.T) {
	reg, rt := anAccountedRoutine(t)
	if err := reg.Delete(rt); err != nil {
		t.Fatal(err)
	}
	if reg.HasSteps(rt) {
		t.Error("the account is still there")
	}
}

// AND A MOVE'S REMOVAL TAKES IT, OR SAYS IT COULD NOT.
//
// By the time this runs the routine does not exist, so what is left is litter rather than a
// routine — it cannot fail the move. It is still worth saying: an account left under a dead slug
// is a file the next routine saved there would inherit.
func TestAMovesRemovalTakesTheAccountOrReportsIt(t *testing.T) {
	reg, rt := anAccountedRoutine(t)
	if err := reg.DeleteSource(rt); err != nil {
		t.Fatal(err)
	}
	if residue := reg.DeleteLeftovers(rt); len(residue) != 0 {
		t.Fatalf("the sweep reported %v", residue)
	}
	if reg.HasSteps(rt) {
		t.Error("the account survived the sweep")
	}

	// AND ONE IT CANNOT SWEEP IS NAMED. A directory in its place, which is the one way
	// to make a removal fail here that is the same answer on every platform.
	reg2, rt2 := anAccountedRoutine(t)
	if err := os.Remove(reg2.StepsPath(rt2)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(reg2.StepsPath(rt2), "in-the-way"),
		0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg2.DeleteSource(rt2); err != nil {
		t.Fatal(err)
	}
	residue := reg2.DeleteLeftovers(rt2)
	var said bool
	for _, r := range residue {
		if strings.Contains(r, "step account") {
			said = true
		}
	}
	if !said {
		t.Errorf("the sweep did not report the account it could not remove: %v",
			residue)
	}
}

// AND AN ACCOUNT IS NEVER WHAT A RUN READS.
//
// Execution reads the `.marco` and nothing else, so an account that is missing, old or unreadable
// cannot change what a routine does. Held here as a fact about this package: nothing in the
// reading path goes near the file.
func TestNothingAboutRunningARoutineReadsItsAccount(t *testing.T) {
	reg, rt := anAccountedRoutine(t)
	if err := os.WriteFile(reg.StepsPath(rt), []byte("{{{ not even JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		t.Fatalf("the routine could not be read: %v", err)
	}
	if string(raw) != accountSrc {
		t.Error("the source changed")
	}
	if !reg.Has(rt) {
		t.Error("the routine is not there any more")
	}
}
