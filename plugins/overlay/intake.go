package main

import (
	"os"
	"os/exec"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
)

// The overlay's ONE door out. Typed and spoken go through it together.
//
// # What used to happen instead
//
// `acts.go` had one `if`: a phrase that arrived as RunVoice ran `marco director <phrase>`
// and a phrase that arrived as Run ran `marco do <phrase>`. So the same words meant two
// different things depending on whether they had been said or typed — a play the person
// had learned, registered and could run by typing was not found by saying its name, and a
// request Director could have satisfied became "shall I learn this?" the moment it was
// typed instead of said. Transport was deciding meaning.
//
// It does not any more. Both build the SAME argv and differ in exactly one flag, which
// records how the words arrived and is never consulted to decide anything: the semantic
// decision lives in [[internal/invoke]], on the other side of this process boundary, and
// is made once for every entrance.
//
// The overlay keeps its own small vocabulary in front of this (voice on/off, learn, ui,
// edit, exit, bind/unbind, press, forget, rename, simplify) because that is the overlay
// configuring ITSELF and is not desktop intent. Everything past it comes here.
//
// Deleting the shared argv — giving spoken input its own command again — must fail
// TestTypedAndSpokenDifferOnlyBySource.

// intakeArgs is the argv for one invocation. The phrase goes LAST so that nothing in it
// can be read as a flag's value, and `--source=` is the only thing that varies.
func intakeArgs(phrase string, source invoke.Source) []string {
	return []string{"do", "--source=" + string(source), phrase}
}

// dispatchIntake sends one phrase to the engine's intake and renders what came back.
//
// # It returns immediately, for both sources
//
// A replay runs for minutes and a spoken "stop" arrives as a SECOND phrase while the
// first is still going. If this blocked the act handler, the stop could not be heard —
// cancellation would be impossible at exactly the moment it is wanted. That property
// belonged to the spoken path alone; unifying the two would have taken it away from
// speech rather than giving it to typing, so it is now a property of the door itself.
//
// # A phrase is never silently dropped
//
// Every phrase reaches a child, every child's outcome reaches the history, and the one
// case where nothing could take the request at all becomes an offer to learn.
func dispatchIntake(h *model, phrase string, source invoke.Source) {
	// A CONTROL PHRASE IS NOT TRACKED AS THE CANCELLABLE RUN. It is the thing doing the
	// cancelling; registering it in the single run slot would displace the child it was
	// sent to stop, so the leader key would then offer to cancel the cancellation.
	track := !intent.IsControlPhrase(phrase)
	args := intakeArgs(phrase, source)
	mlogI("overlay: intake", "source", string(source), "phrase", phrase, "args", args)
	go func() {
		r := spawnIntakeChild(h, phrase, track, args...)
		if r.offersLearn() {
			mlogI("overlay: nothing took it — offering learn", "phrase", phrase)
			offerLearn(h, phrase)
		}
	}()
}

// spawnIntakeChild is a package variable for the same reason the engine's own `submitPhrase`
// is one: the door has to be exercisable without spawning marco.exe, which on this surface
// would drive a real mouse. Production never reassigns it.
var spawnIntakeChild = runRecordTracked

// overlayVerb picks out the overlay's OWN commands — the ones that configure Marco rather
// than ask it to do something on the desktop. They are `marco` subcommands, not
// invocations, and they must not be handed to the intake: `forget my play` is an
// instruction about the catalogue, and Director reading it against the screen would be
// answering a question nobody asked.
//
// Returns the argv and true when the words are one of them.
func overlayVerb(name string, fields []string) ([]string, bool) {
	if len(fields) == 0 {
		return nil, false
	}
	switch {
	case fields[0] == "bind" || fields[0] == "unbind":
		return fields, true
	case len(fields) >= 2 && fields[0] == "press": // a key or chord: "press control c"
		return fields, true
	case len(fields) >= 2 && fields[0] == "forget":
		return append([]string{"forget"}, fields[1:]...), true
	case len(fields) >= 2 && fields[0] == "simplify":
		return fields, true
	case len(fields) >= 4 && fields[0] == "rename":
		// "rename old name to new name" — split on " to " and pass as separate args.
		// Without a " to " it is not the rename verb at all, so it falls through to the
		// intake rather than being run as a malformed one.
		rest := strings.TrimSpace(strings.TrimPrefix(name, "rename "))
		if idx := strings.Index(rest, " to "); idx > 0 {
			return []string{"rename", rest[:idx], "to", strings.TrimSpace(rest[idx+4:])}, true
		}
	}
	return nil, false
}

// stopTheDirector reaches the ACTIVE EXECUTION AUTHORITY.
//
// # Why killing the child is not enough
//
// For a learned play the child the overlay spawned is only holding a socket. The Director
// is doing the work, and it explicitly treats a dropped client as "not a cancellation —
// the work continues", which is correct: a front end that crashed must not abort a replay.
// So a local kill made the HUD go quiet while the desktop carried on being driven, and
// there was no longer anything on screen offering to stop it.
//
// # Why detached, and why nothing waits for it
//
// Cancellation has to feel instant, and the caller may be the action loop that the HUD
// draws from. Waiting on a socket dial there would freeze the window at the exact moment
// somebody is trying to stop something. It is also why the local kill happens FIRST and
// this is only started: immediacy is local, authority is remote.
//
// The load-bearing rule from CLAUDE.md is upstream of this: the keyboard hook never calls
// cancelRun itself — it pushes actCancelRun onto the action queue and returns — so nothing
// here runs on the hook thread. Deleting that indirection must fail the pump test.
//
// Deleting this call must fail TestCancelAlsoStopsTheDirector.
var stopTheDirector = func() {
	cmd := exec.Command(marcoBin(), "director", "stop")
	cmd.Stdout, cmd.Stderr = nil, nil
	cmd.Env = append(os.Environ(), "MARCO_NO_PANIC_STOP=1")
	if err := cmd.Start(); err != nil {
		mlogW2("overlay: could not ask the Director to stop", "err", err)
		return
	}
	// Reaped on its own goroutine so the process does not linger as a zombie, and so
	// nothing in the UI ever waits on it.
	go func() { _ = cmd.Wait() }()
}
