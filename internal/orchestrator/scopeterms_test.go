package orchestrator

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The three scopes have one vocabulary, and it is the ruled one.
//
// Deleting or rewording an arm of ScopeLabel must fail this, because the label is what Studio,
// the terminal and the widget will all print.
func TestTheThreeScopesHaveOneVocabulary(t *testing.T) {
	for _, tc := range []struct {
		s    Scope
		want string
	}{
		{ScopeApp, "Only in notepad"},
		{ScopeAppAnywhere, "From anywhere → notepad"},
		{ScopeEverywhere, "From anywhere → current foreground"},
	} {
		if got := ScopeLabel(tc.s, "notepad"); got != tc.want {
			t.Errorf("ScopeLabel(%q) = %q, want %q", tc.s, got, tc.want)
		}
		if ScopeMeans(tc.s, "notepad") == "" {
			t.Errorf("scope %q has no sentence beside its label", tc.s)
		}
	}
	// No application never prints a blank where one belongs.
	if got := ScopeLabel(ScopeAppAnywhere, ""); strings.HasSuffix(got, "→ ") {
		t.Errorf("an app-less label left a blank: %q", got)
	}
	// And the line after a save uses the same table.
	if got := scopeLabel(scopeFocus, "notepad"); got != "From anywhere → notepad" {
		t.Errorf("the save line says %q, not the shared label", got)
	}
}

// The default is From anywhere → this application, and with no application there is none.
func TestTheDefaultScopeIsFromAnywhereToTheApplication(t *testing.T) {
	if got, why := DefaultScope(&Draft{App: "notepad"}); got != ScopeAppAnywhere || why != "" {
		t.Errorf("an unguarded recording in notepad defaults to %q (%q), want %q",
			got, why, ScopeAppAnywhere)
	}
	guarded := &Draft{App: "notepad", Steps: []macroir.Step{{Kind: macroir.StepClick,
		RelTo: "notepad"}}}
	if got, _ := DefaultScope(guarded); got != ScopeAppAnywhere {
		t.Errorf("a guarded recording defaults to %q, want the one scope it can run under", got)
	}
}

// With no application there is no default, and the reason is said.
//
// The arm the ruling is about: every site used to fall through to the current-foreground scope
// here, because it was the only one left. Deleting the arm must fail this.
func TestNoApplicationIsNeverADefault(t *testing.T) {
	for _, dr := range []*Draft{nil, {}, {App: "   "}} {
		got, why := DefaultScope(dr)
		if got != "" {
			t.Errorf("DefaultScope(%+v) = %q; with no application there is no default", dr, got)
		}
		if !strings.Contains(why, "no application") {
			t.Errorf("the reason does not say what is missing: %q", why)
		}
	}
}

// ── RUN-TIME ADMISSION ──

var onlyInNotepad = routes.Route{App: "notepad", Slug: "save-draft"}

func TestAnOnlyInRoutineRefusesWithAnotherApplicationInFront(t *testing.T) {
	why := AdmitInPlace(onlyInNotepad, InPlace{Foreground: "windowsterminal"})
	if why == "" {
		t.Fatal("an only-in-notepad routine was admitted with the terminal in front")
	}
	for _, want := range []string{"Only in notepad", "windowsterminal is in front",
		"nothing was performed"} {
		if !strings.Contains(why, want) {
			t.Errorf("the refusal does not say %q: %s", want, why)
		}
	}
}

func TestAnOnlyInRoutineRunsWithItsApplicationInFront(t *testing.T) {
	for _, fg := range []string{"notepad", "Notepad", "notepad.exe"} {
		if why := AdmitInPlace(onlyInNotepad, InPlace{Foreground: fg}); why != "" {
			t.Errorf("foreground %q was refused: %s", fg, why)
		}
	}
}

// SUPPLYING A WINDOW IS NOT ADMISSION; OWNING THE RIGHT ONE IS.
//
// The validated owner of a handed-over window is compared with the routine's application, and it
// wins over the foreground reading — a window of another program is refused even with notepad in
// front, because the run would act in the window it was given.
func TestAHandedOverWindowOfAnotherProgramIsNotAdmission(t *testing.T) {
	why := AdmitInPlace(onlyInNotepad, InPlace{HandedTo: "chrome", Foreground: "notepad"})
	if why == "" {
		t.Fatal("a handed-over chrome window admitted an only-in-notepad routine")
	}
	if !strings.Contains(why, "belongs to chrome") {
		t.Errorf("the refusal does not name the window's program: %s", why)
	}
}

// Studio's handoff: Studio is the foreground reading, the validated window is notepad's.
func TestAHandedOverWindowOfTheApplicationIsAdmission(t *testing.T) {
	if why := AdmitInPlace(onlyInNotepad,
		InPlace{HandedTo: "notepad", Foreground: "studio"}); why != "" {
		t.Errorf("Studio's validated handoff to a notepad window was refused: %s", why)
	}
}

// An empty reading would also fail the comparison below it, so what this arm owns is the
// SENTENCE: "  is in front" naming nobody would be a refusal that explains nothing.
func TestAnUnreadableForegroundRefuses(t *testing.T) {
	why := AdmitInPlace(onlyInNotepad, InPlace{})
	if why == "" {
		t.Fatal("an only-in routine was admitted with no foreground reading at all")
	}
	if !strings.Contains(why, "could not tell which application is in front") {
		t.Errorf("the refusal does not say the foreground could not be read: %s", why)
	}
}

func TestFocusAndCurrentForegroundRoutinesAreNotChecked(t *testing.T) {
	for _, rt := range []routes.Route{
		{App: "notepad", Focus: true, Slug: "x"}, // From anywhere → notepad
		{Slug: "copy"},                           // From anywhere → current foreground
	} {
		if why := AdmitInPlace(rt, InPlace{Foreground: "game"}); why != "" {
			t.Errorf("%+v was checked as though it were only-in: %s", rt, why)
		}
	}
}
