package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// WHICH PLACE IS IN FRONT, BY ITS DURABLE IDENTITY.
//
// # Why a command line needed this
//
// `ObserveShowing` has been the one "where am I standing" door out of this process since
// ADR-031 — a fresh look through the production path, resolved by `observe.PlaceNow` against
// the real semantic store, answering with a durable subject id or a named refusal. Exactly one
// caller used it, `cmd/marco`'s entry guard, and it never printed the id because a play does not
// need one.
//
// So there was no way to ask, from a terminal, WHICH place Marco takes itself to be in. Every
// other surface answers in prose on purpose — `sight` states outright that it names no subject
// reference, and `HerePlace` carries the handle so naming can address it while the surface
// never renders it. Both are right for a person and useless for the question 37G asks, which is
// whether two readings of one screen resolve to the SAME durable subject. Prose cannot answer
// that: two places described alike read as one.
//
// # What it is not
//
// Not a second recogniser, not a second look, not a second matcher. It sends the query that
// already exists and prints the reply that already exists. It performs no input, starts no
// learn session, mints no authority and writes nothing — `ShowingNow` takes a bounded passive
// look and `Episode.EstablishPlaces` is false on it, so a place cannot become durable by being
// asked about.
//
// The subject id is a DIAGNOSTIC handle and is deliberately not offered anywhere a person is
// making a decision. It is here because an experiment comparing identities has to be able to
// see them.

// runShowing is `director showing --application <app>`.
func runShowing(args []string) int {
	fs := flag.NewFlagSet("showing", flag.ExitOnError)
	application := fs.String("application", "", "which application to look in")
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	// EMPTY IS REFUSED HERE TOO, rather than sent. The Director already refuses it — a
	// screen guard satisfiable by the wrong application is worse than no guard — but a
	// person who forgot the flag is better served by the usage line than by a reply saying
	// no application was named.
	if *application == "" {
		fmt.Fprintln(os.Stderr,
			"director: --application is required\n"+
				"  example: director showing --application applicationframehost")
		return 2
	}
	return observationRequest(*jsonOut,
		service.ObserveQuery{Showing: &service.ObserveShowing{Application: *application}},
		renderShowing)
}

// renderShowing prints the outcome, and the id only when there is one.
//
// The outcome leads. A reader must check it rather than the emptiness of the subject — an id
// beside any other outcome is a bug, and printing the id first would invite reading it as the
// answer.
func renderShowing(raw json.RawMessage) string {
	var v service.ShowingView
	if err := json.Unmarshal(raw, &v); err != nil {
		return "director: the Director's reply could not be read\n"
	}
	out := fmt.Sprintf("%s in %s\n", v.Outcome, orUnknownApp(v.Application))
	if v.Subject != "" {
		out += "  place  " + v.Subject + "\n"
	}
	if v.Why != "" {
		out += "  why    " + v.Why + "\n"
	}
	return out
}
