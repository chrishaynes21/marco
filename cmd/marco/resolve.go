package main

// ASK DIRECTOR WHICH SAVED ROUTINE A REQUEST NAMED.
//
// `marco resolve "<phrase>" [--target <app>] [--json]`. A CLIENT and nothing else: it asks, it
// prints, and it performs nothing. The widget uses it at the one moment it already has — when
// nothing took a phrase and it is about to offer to learn one — so the overlay gains no Director
// client of its own and keeps its single door.
//
// # Why there is no local fallback and no implicit start
//
// Because exact matching is not a smaller question a front end may answer for itself. Which
// routines are eligible depends on what is in front, and which library the answer is about is
// Director's; a surface that matched exact names locally would be answering a different question
// with the same words, out of a library nobody checked. Director owns the whole of it, including
// the cheap half.
//
// And `AutoStart` is false. This is a read: starting a service that observes the desktop in order
// to answer whether a phrase names a saved routine would be a question with a side effect, which
// is the one thing this verb must not be.
//
// Three ways there is no answer, and they are told apart, because a person acts on them
// differently: it is not running, it is running and speaks another protocol, or it answered and
// refused. See [[ADR-267-director-owns-route-resolution]].

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

// resolveExit is what the shell learns, so a front end can branch without parsing prose.
//
//	0  an answer, whatever kind it is
//	3  there is no answer: Director could not be asked, or refused
//
// Three rather than one, so "Marco has nothing like that" and "nobody could be asked" are not the
// same exit code. Two is already the usage code everywhere in this command.
const resolveExit = 3

// runResolve asks Director which routine a request named and prints the answer.
func runResolve(args []string) {
	jsonMode, target := false, ""
	var rest []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--json":
			jsonMode = true
		case args[i] == "--target" && i+1 < len(args):
			i++
			target = args[i]
		case strings.HasPrefix(args[i], "--target="):
			target = strings.TrimPrefix(args[i], "--target=")
		default:
			rest = append(rest, args[i])
		}
	}
	request := strings.TrimSpace(strings.Join(rest, " "))
	if request == "" {
		fmt.Fprintln(os.Stderr,
			`usage: marco resolve "<phrase>" [--target <app>] [--json]`)
		os.Exit(2)
	}

	// NOT STARTED. See the note above: a read does not start a service.
	c, err := directorConnect(false)
	if err != nil {
		resolveFailed(jsonMode, whyNoDirectorHere(err))
		return
	}
	defer c.Close()

	out, err := c.ResolveRoute(service.ResolveRoutePayload{
		Request: request, Target: strings.TrimSpace(target),
		// THE LIBRARY THIS PROCESS BELIEVES IT IS TALKING ABOUT, so Director can say
		// when it is reading a different one rather than serving a routine of that
		// name out of somewhere else.
		Library: marcopaths.Routes(),
		// AND THE BUILT-IN SKILLS: the widget asks here, and reviews and runs one (skill.go).
		Skills: true,
	})
	if err != nil {
		resolveFailed(jsonMode, whyRefusedHere(err))
		return
	}
	if jsonMode {
		_ = json.NewEncoder(os.Stdout).Encode(out)
		return
	}
	a := out.Answer
	if out.Mismatch != "" {
		fmt.Println(out.Mismatch)
		os.Exit(resolveExit)
	}
	fmt.Printf("%s\n", a.Context.Says)
	switch a.Kind {
	case "run":
		// THE WHOLE IDENTITY, in the three parts an invocation is built from, so a
		// front end never takes the string apart and never looks a name up again.
		fmt.Printf("run: %s\n", a.ID)
		fmt.Printf("app: %s\nslug: %s\nfocus: %v\n", a.App, a.Slug, a.Focus)
	default:
		fmt.Printf("%s: %s\n", a.Kind, a.Reply)
	}
	fmt.Printf("(%d offered, %s, %s)\n", a.Offered, a.Source, a.Resolver)
}

// resolveFailed prints why there is no answer and exits with the code that says so.
func resolveFailed(jsonMode bool, why string) {
	if jsonMode {
		_ = json.NewEncoder(os.Stdout).Encode(struct {
			Why string `json:"why"`
		}{why})
	} else {
		fmt.Fprintln(os.Stderr, why)
	}
	os.Exit(resolveExit)
}

// whyNoDirectorHere tells "not running" from "running and unreachable from here".
func whyNoDirectorHere(err error) string {
	if _, ok := service.ReadEndpoint(directorDir()); !ok {
		return "the Director is not running, so nothing can resolve this request"
	}
	return "the Director is running and this command could not reach it, so nothing can " +
		"resolve this request: " + err.Error()
}

// whyRefusedHere is a request the Director answered and would not satisfy.
func whyRefusedHere(err error) string {
	var refused *service.RequestError
	if errors.As(err, &refused) {
		if refused.Incompatible() {
			return "the Director is running and speaks a different protocol from this " +
				"command, so nothing can resolve this request: " + refused.Message
		}
		return "the Director refused this request: " + refused.Error()
	}
	return "asking the Director which play this is failed: " + err.Error()
}
