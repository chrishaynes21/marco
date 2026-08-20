package semanticmemory_test

import (
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Goals in the durable store: a name and a subject, folded on repeats, refused on conflict,
// and dropped when the subject they name is gone.

func goalStore(t *testing.T) (*semanticmemory.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.json")
	s, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("open: %s", why)
	}
	return s, path
}

func establishSubject(t *testing.T, s *semanticmemory.Store, terms ...observe.InterfaceTerm) string {
	t.Helper()
	id, err := s.EstablishPlace("app", observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4},
		Terms: terms, TermsKnown: true,
	})
	if err != nil || id == "" {
		t.Fatalf("establish: %q, %v", id, err)
	}
	return id
}

func TestAGoalIsRememberedAndFoldsRepeatDemonstrations(t *testing.T) {
	s, _ := goalStore(t)
	subject := establishSubject(t, s, observe.TermSettings)

	if err := s.RememberGoal("app", observe.Goal{Name: "open mouse settings",
		Subject: subject}); err != nil {
		t.Fatalf("remember: %v", err)
	}
	// The same outcome shown again is lineage on the one record, never a second record.
	if err := s.RememberGoal("app", observe.Goal{Name: "Open Mouse Settings",
		Subject: subject}); err != nil {
		t.Fatalf("repeat: %v", err)
	}
	goals := s.Goals("app")
	if len(goals) != 1 {
		t.Fatalf("%d goal(s), want 1", len(goals))
	}
	if goals[0].Demonstrations != 2 {
		t.Errorf("demonstrations = %d, want 2", goals[0].Demonstrations)
	}
	if goals[0].Subject != subject {
		t.Errorf("subject = %q, want %q", goals[0].Subject, subject)
	}
}

// Learning the same name again REBINDS it, and the name still means exactly one outcome.
//
// Refusing was tried first. Live, a goal left behind by a failed learn made its own name
// unusable: the person was told their words already meant somewhere else, because of an
// attempt of Marco's that had not worked. A person asking for a name again is saying what they
// mean by it now — the same rule NameSubject follows for screens.
func TestLearningTheSameNameAgainRebindsIt(t *testing.T) {
	s, _ := goalStore(t)
	first := establishSubject(t, s, observe.TermSettings)
	second := establishSubject(t, s, observe.TermAudio)
	if first == second {
		t.Fatal("fixture: the two subjects merged")
	}
	if err := s.RememberGoal("app", observe.Goal{Name: "open settings",
		Subject: first}); err != nil {
		t.Fatalf("remember: %v", err)
	}
	if err := s.RememberGoal("app", observe.Goal{Name: "open settings",
		Subject: second}); err != nil {
		t.Fatalf("re-learning the same name was refused: %v", err)
	}
	goals := s.Goals("app")
	if len(goals) != 1 {
		t.Fatalf("%d goal(s) under one name, want 1 — the name must still mean one outcome",
			len(goals))
	}
	if goals[0].Subject != second {
		t.Errorf("the name still reaches %q, want the newly learned %q",
			goals[0].Subject, second)
	}
	// The old binding's lineage does not carry over: it was about somewhere else.
	if goals[0].Demonstrations != 1 {
		t.Errorf("demonstrations = %d after a rebind, want 1", goals[0].Demonstrations)
	}
}

func TestAGoalNamingAnUnknownSubjectIsRefused(t *testing.T) {
	s, _ := goalStore(t)
	if err := s.RememberGoal("app", observe.Goal{Name: "open nowhere",
		Subject: "subj_nowhere"}); err == nil {
		t.Fatal("a goal about a subject memory does not hold was written")
	}
}

func TestAGoalSurvivesARestartAndItsSubjectGoingDropsIt(t *testing.T) {
	s, path := goalStore(t)
	subject := establishSubject(t, s, observe.TermSettings)
	if err := s.RememberGoal("app", observe.Goal{Name: "open mouse settings",
		Subject: subject}); err != nil {
		t.Fatalf("remember: %v", err)
	}

	// A new store over the same file: the goal is there.
	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopen: %s", why)
	}
	if got := reopened.Goals("app"); len(got) != 1 || got[0].Name != "open mouse settings" {
		t.Fatalf("after a restart, goals = %+v", got)
	}
}
