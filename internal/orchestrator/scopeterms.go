package orchestrator

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The three routine scopes, in the one vocabulary every surface uses, and the one default.
//
// # Why this file exists
//
// Where a routine works was being described in at least five phrasings — "only in notepad",
// "in notepad from anywhere", "everywhere", "In the app", "from anywhere, switching nothing" —
// and decided by five defaults that all agreed with each other and with nobody's ruling: Enter
// at the terminal's question, the unattended save, narration, Studio's select, and the voice
// path all meant "only in this application". The owner ruled the default is FROM ANYWHERE → THIS
// APPLICATION, and that the three scopes carry the same names wherever a person meets them. A
// ruling that lives in five places is five rulings, so it lives here.
//
// See docs/Surfaces-alignment-proposal-2026-11-02.md §0 (R2) and §3.

// ScopeLabel is a scope's name as a person reads it, with the application filled in.
//
// THE ARROW IS PART OF THE NAME. "From anywhere → notepad" and "From anywhere → current
// foreground" are both available from anywhere; what separates them is where the routine then
// presses, which is the half a person gets wrong when it is not said.
//
// An application is asked for only by the two scopes that name one; with none known they say
// "its application" rather than printing a blank.
func ScopeLabel(s Scope, app string) string {
	if strings.TrimSpace(app) == "" {
		app = "its application"
	}
	switch s {
	case ScopeApp:
		return "Only in " + app
	case ScopeAppAnywhere:
		return "From anywhere → " + app
	case ScopeEverywhere:
		return "From anywhere → current foreground"
	}
	return string(s)
}

// ScopeMeans is the one sentence beside a label: what Marco does when the routine runs.
func ScopeMeans(s Scope, app string) string {
	if strings.TrimSpace(app) == "" {
		app = "its application"
	}
	switch s {
	case ScopeApp:
		return "runs only while " + app + " is in front, and brings nothing forward"
	case ScopeAppAnywhere:
		return "brings " + app + " forward, then runs there"
	case ScopeEverywhere:
		return "runs into whatever is in front; it names no application and checks no window"
	}
	return ""
}

// DefaultScope is the scope a recording is offered under when nobody has chosen one yet, or ""
// and the reason when there is no default to offer.
//
//   - Otherwise, with an application known, FROM ANYWHERE → THIS APPLICATION. The ruled default.
//   - With NO application known there is no default. The ruling is explicit: ask or refuse, never
//     invent an application and never silently choose the current-foreground scope — which is
//     what every one of these sites used to do, because it was the only scope left.
//
// Deleting the no-application arm must fail TestNoApplicationIsNeverADefault.
func DefaultScope(dr *Draft) (Scope, string) {
	if dr == nil || strings.TrimSpace(dr.App) == "" {
		return "", "no application was recorded, so there is nothing for Marco to bring " +
			"forward; the only way it could be saved is " +
			fmt.Sprintf("%q, which %s", ScopeLabel(ScopeEverywhere, ""),
				ScopeMeans(ScopeEverywhere, ""))
	}
	return ScopeAppAnywhere, ""
}

// scopeOfMode is the stated scope for the older internal mode the narration path still uses.
func scopeOfMode(m scopeMode) Scope {
	switch m {
	case scopeFocus:
		return ScopeAppAnywhere
	case scopeGlobal:
		return ScopeEverywhere
	}
	return ScopeApp
}

// ── RUN-TIME ADMISSION ──

// InPlace is what a run knows, at the instant it is about to perform, about where it would act.
type InPlace struct {
	// HandedTo is the program that owns the window this run was HANDED, as the host
	// validated it (alive, owned by the process it was chosen from, image read from that
	// process, and in front). "" when no window was handed over — and a `--window` flag
	// the host refused never reaches here, because that run has already been refused.
	HandedTo string
	// Foreground is the application in front, read at this instant. Only consulted when no
	// window was handed over.
	Foreground string
}

// AdmitInPlace is whether an ONLY-IN routine may run now, or the sentence saying why not.
//
// # What the scope promises, and where that promise was kept
//
// "Only in notepad" promises the routine runs only while notepad is in front. The promise was
// kept by RESOLUTION: the phrase lookup and Director offer an in-place routine only while its
// application is the context. Nothing kept it at RUN time — the generated route has no
// `Activate` and no check — so an explicit identity (`--play`, a clicked Run, a binding) ran it
// into whatever was in front. From a terminal, that is the terminal.
//
// # What admits it
//
//   - a window HANDED to this run whose validated owner IS the routine's application. That is
//     Studio's Play: it raises a window of the routine's application and hands it over, and the
//     host has checked it is that window, alive and in front. A window of any other program is
//     refused — supplying a window is not admission, owning the right one is;
//   - otherwise the application in front NOW is the routine's application. An unreadable
//     foreground refuses rather than admits.
//
// Focus and current-foreground routines are not this rule's business: the first brings its
// application forward itself and the second promises nothing about one. Nothing here reads how
// the request arrived — a source is a diagnostic, never a permission.
//
// Pure, so every arm can be held without a desktop. Deleting either comparison must fail
// TestAHandedOverWindowOfAnotherProgramIsNotAdmission or
// TestAnOnlyInRoutineRefusesWithAnotherApplicationInFront.
func AdmitInPlace(rt routes.Route, at InPlace) string {
	if rt.App == "" || rt.Focus {
		return ""
	}
	want := ScopeLabel(ScopeApp, rt.App)
	if at.HandedTo != "" {
		if sameApplication(at.HandedTo, rt.App) {
			return ""
		}
		return fmt.Sprintf("this routine is %q, and the window it was given belongs to %s — "+
			"nothing was performed", want, at.HandedTo)
	}
	switch {
	case strings.TrimSpace(at.Foreground) == "":
		return fmt.Sprintf("this routine is %q, and Marco could not tell which application "+
			"is in front — nothing was performed", want)
	case !sameApplication(at.Foreground, rt.App):
		return fmt.Sprintf("this routine is %q, and %s is in front — nothing was performed. "+
			"Bring %s forward and ask again, or change where it works to %q", want,
			at.Foreground, rt.App, ScopeLabel(ScopeAppAnywhere, rt.App))
	}
	return ""
}

// sameApplication compares two application keys as `winctx` spells them: lowercased executable
// names without the extension. Folded again here so a caller that passes a display spelling is
// compared on the key, not refused on its case.
func sameApplication(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		return strings.TrimSuffix(s, ".exe")
	}
	return norm(a) != "" && norm(a) == norm(b)
}
