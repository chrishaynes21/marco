package main

// WHICH APPLICATION IS IN FRONT, ANSWERED WITHOUT A RUNNING DIRECTOR.
//
// # Why this exists
//
//	Marco should inspect the computer before asking the user for information it can discover
//	itself. […] Do not require a running Director daemon simply to determine which application
//	is active.
//
// `director windows` answers this and is a SERVICE CLIENT: it calls `connect`, so a cold
// machine with no daemon gets an error rather than an answer. That made `--application`
// mandatory on every command that watches something, and made Marco ask a person for the one
// fact the operating system will hand over for free.
//
// `winprovider.AllCandidates` needs no service at all — it is the same enumeration the live
// observer uses, and each candidate already carries the executable image name, whether it is
// in the foreground, and whether it is minimised.
//
// # What it refuses to guess
//
// Four answers, not two. A guess here would pick the window a run then acts on, so every way
// of being unsure is its own outcome with its own sentence:
//
//	one foreground window                  the application, named
//	nothing in the foreground              no answer — a locked screen, a switch in progress
//	the foreground window is minimised     no answer, said differently: a minimised window is
//	                                       not something to act on
//	two candidates for one application     the application still, since the IMAGE is what a
//	                                       scope compares; which window is a separate question
//	                                       and `windowref.Selector` already asks it

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
)

// frontmost is the application in the foreground, and why there is no answer when there is not.
type frontmost struct {
	// Application is the executable image name, empty when nothing could be resolved.
	Application string
	// Title is what the window is called, for a person reading a report. NEVER identity —
	// see workflowstore.Scope for why a title is not one.
	Title string
	// Windows is how many top-level windows that application has in front.
	Windows int
	// Because is the sentence, set whether or not there is an answer.
	Because string
}

// Known reports whether an application was resolved.
func (f frontmost) Known() bool { return f.Application != "" }

// foregroundApplication asks the operating system which application is in front.
//
// Takes the provider so a test can supply candidates without a desktop; `winprovider.New()` is
// what the commands pass and it needs nothing running.
func foregroundApplication(ctx context.Context, p interface {
	AllCandidates(context.Context) []windowref.Candidate
}) frontmost {

	all := p.AllCandidates(ctx)
	if len(all) == 0 {
		return frontmost{Because: "no top-level windows were offered at all, so the " +
			"foreground cannot be read; name the application instead"}
	}
	var front []windowref.Candidate
	for _, c := range all {
		if c.Foreground {
			front = append(front, c)
		}
	}
	if len(front) == 0 {
		return frontmost{Because: fmt.Sprintf("none of the %d window(s) offered is in "+
			"the foreground — a locked screen, a switch in progress, or a "+
			"desktop with focus; name the application instead", len(all))}
	}
	// THE IMAGE IS THE ANSWER, and more than one window of one application is not an
	// ambiguity about WHICH APPLICATION. A scope compares images; which window to watch
	// is `windowref.Selector`'s question and it already refuses that ambiguity itself.
	images := map[string]int{}
	titles := map[string]string{}
	minimised := map[string]int{}
	for _, c := range front {
		img := strings.ToLower(strings.TrimSpace(c.Application))
		if img == "" {
			continue
		}
		images[img]++
		if c.Minimized {
			minimised[img]++
		}
		if titles[img] == "" {
			titles[img] = c.Title
		}
	}
	switch len(images) {
	case 0:
		return frontmost{Because: fmt.Sprintf("%d foreground window(s) and none of "+
			"them reported an executable name, so there is no application to "+
			"name", len(front))}
	case 1:
	default:
		// TWO APPLICATIONS CLAIMING THE FOREGROUND. The platform should not report
		// this; if it does, picking one is picking what a run acts on.
		names := make([]string, 0, len(images))
		for img := range images {
			names = append(names, img)
		}
		sort.Strings(names)
		return frontmost{Because: fmt.Sprintf("%d applications claim the foreground "+
			"(%s), so which one is meant cannot be read; name it instead",
			len(images), strings.Join(names, ", "))}
	}
	var img string
	for k := range images {
		img = k
	}
	if minimised[img] == images[img] {
		// MINIMISED, said as itself. A minimised window is not something to watch or
		// act on, and "no foreground" would send somebody to look at the wrong thing.
		return frontmost{Because: fmt.Sprintf("%q is in the foreground and every one "+
			"of its %d window(s) is minimised, so there is nothing on screen to "+
			"watch", img, images[img])}
	}
	return frontmost{
		Application: img, Title: titles[img], Windows: images[img],
		Because: fmt.Sprintf("%q is the application in front, with %d window(s)",
			img, images[img]),
	}
}

// resolveApplication is the application a command should act on, from the argument or the front.
//
// # The rule, and it is the owner's
//
//	An explicit application argument should select the intended target, not override a saved
//	workflow's application identity.
//
// So this resolves the TARGET only. It never consults an artifact and never decides whether a
// workflow may run — that is `Artifact.ScopeAllows`, asked separately and against the window
// that was actually acquired. Keeping the two apart is what stops a discovered application from
// quietly satisfying a scope check: discovery picks what to look at, the scope decides whether
// the file may act there, and a mismatch between them is a refusal rather than a negotiation.
func resolveApplication(ctx context.Context, given string) (string, string) {
	if strings.TrimSpace(given) != "" {
		return strings.ToLower(strings.TrimSpace(given)), fmt.Sprintf(
			"%q was named on the command line", strings.TrimSpace(given))
	}
	f := foregroundApplication(ctx, winprovider.New())
	if !f.Known() {
		return "", f.Because
	}
	return f.Application, f.Because
}
