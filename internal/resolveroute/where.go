package resolveroute

// WHICH APPLICATION A REQUEST IS READ AGAINST, AND HOW THAT WAS KNOWN.
//
// # Why the live foreground is the only thing that grants it
//
// Because Director's world model cannot be trusted for this on its own, and both ways it fails
// were traced rather than guessed at:
//
//   - The window-system provider takes its foreground claim from `winprovider.offered()`, which
//     drops every Marco-owned surface. So with Studio in front there is no candidate carrying
//     `Foreground` and NO application observation is produced at all — the last one stands,
//     naming an application that is no longer in front, with nothing in the value saying so. An
//     observation can therefore be seconds old and already wrong.
//   - The accessibility provider takes the bridge's first window with no such exclusion, so a
//     fresh observation can name a MARCO window as the active application.
//
// So age is evidence and not proof. What grants `Observed` is a foreground reading taken NOW
// through the path that excludes Marco's own surfaces: if it names an application, that
// application is in front, and if it names nothing then one of ours is — which is exactly the
// case that must not unlock a routine that works only in place.
//
// # And a basis that cannot be relied on carries no application at all
//
// Not "the app, with a warning beside it". The field that anything acts on — the eligibility
// filter, and the context the resolver plugin is told — is filled ONLY when it is known. What was
// last seen goes into the sentence a person reads, as prose, where nothing can mistake it for a
// fact about now.
//
// That is also what makes a Marco-owned observation harmless without a list of Marco's own window
// names to check against: it cannot be corroborated, so it is not a context, so it names nothing.
//
// See [[ADR-267-director-owns-route-resolution]].

import (
	"fmt"
	"strings"
	"time"
)

// Seen is one reading of which application is in front.
type Seen struct {
	App string
	// At is when the reading was taken, zero when it is not known.
	At time.Time
	// OK is whether there is a reading at all. An absent reading is a real answer and is
	// never the same as an empty name.
	OK bool
}

// Where decides which application a request is read against, and how that was arrived at.
//
// `stated` is what a person named, `observed` is Director's world model, and `foreground` is a
// reading taken now through the path that excludes Marco's own surfaces. A pure function of the
// three, so the rule can be held without a desktop — which matters here more than usual, because
// the desktop states this is about are ones nobody can arrange on purpose.
func Where(stated string, observed, foreground Seen, now time.Time) Context {
	if s := strings.TrimSpace(stated); s != "" {
		return Context{App: s, Basis: Stated,
			Says: fmt.Sprintf("read against %s, which you named", s)}
	}
	// CORROBORATED NOW. The reading that excludes our own windows named something, so
	// something of the person's is in front and this is it.
	if foreground.OK && strings.TrimSpace(foreground.App) != "" {
		app := strings.TrimSpace(foreground.App)
		return Context{App: app, Basis: Observed,
			Says: fmt.Sprintf("read against %s, which is in front now", app)}
	}
	// NOT CORROBORATED. Either one of Marco's own surfaces has the foreground, or the
	// question could not be asked. Either way nothing is known to be in front, so no
	// routine that works only in place is offered — and the application is NOT carried.
	if observed.OK && strings.TrimSpace(observed.App) != "" {
		return Context{Basis: LastExternal, Says: fmt.Sprintf(
			"nothing is known to be in front, so no in-place play is offered. "+
				"The last application Marco saw was %s%s, and a Marco window "+
				"has the foreground now — name an application to read this "+
				"against one.", strings.TrimSpace(observed.App),
			agoOf(observed.At, now))}
	}
	return Context{Basis: Unknown, Says: "nothing is known to be in front, so no " +
		"in-place play is offered — name an application to read this against one."}
}

// agoOf is how long before `now` a reading was taken, or "" when it does not say.
//
// Decoration on a sentence and nothing else. It is deliberately NOT part of any decision: an
// observation's age was the first thing this was going to be built on, and a fresh observation of
// an application that has since lost the foreground is exactly as wrong as an old one.
func agoOf(at, now time.Time) string {
	if at.IsZero() || now.IsZero() || now.Before(at) {
		return ""
	}
	return fmt.Sprintf(" (%s ago)", now.Sub(at).Round(time.Second))
}
