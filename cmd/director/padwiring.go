package main

import (
	"context"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/platform/padsource"
)

// Giving the Director eyes on the controller, and nothing else.
//
// # Why this is wired now
//
// Because a live game session reported `received: 0, unavailable: ""` for four minutes —
// "navigation observation is working and nobody pressed anything" — while the only producer
// wired was a keyboard and mouse hook. On a game the player's hands are very often on a pad,
// and 40 observed screen-state transitions were attributed to nothing at all. A milestone about
// associating input with transitions cannot start on top of that.
//
// The adapter already existed and was already read-only; `padsource`'s own note explains why it
// is a poll rather than a hook and why it lives beside navsource rather than inside it. Nothing
// here invents a controller subsystem: this file opens a subscription per session, drains it on
// the sampler's tick, and merges it with the keyboard and mouse stream the session already had.
//
// # Read-only, structurally, and that argument is not weakened here
//
// The backend binds XInputGetState and not XInputSetState, so this cannot actuate a controller
// even by mistake. Wiring it into the Director adds an OBSERVATION path, which is what
// [[ADR-010-passive-observation-cannot-execute]] protects rather than what it forbids. The
// Director gains no way to press anything: there is no setter to call.

// padObservationEnv turns controller observation off, for a machine where polling XInput is
// unwelcome.
//
// On by default, unlike the shadow detector, because the costs are not comparable: this is one
// function call every 8ms against a 1.25GB model and a two-second inference. The switch exists
// so a user who does not want their pad read has a way to say so, not because the default is in
// doubt.
const padObservationEnv = "MARCO_NO_CONTROLLER"

// padObservationDisabled reports whether the user asked for no controller observation.
func padObservationDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(padObservationEnv))) {
	case "", "0", "no", "false", "off":
		return false
	}
	return true
}

// newPadSource builds the controller observer, or nil with the reason it was not built.
//
// A nil source is reported rather than silent, and the reason travels into the session's own
// device records — which is the whole point. "Nothing was watching the pad" has to be a thing a
// report can say, because the alternative is a report that says the player sat still.
func newPadSource() (*padsource.Source, string) {
	if padObservationDisabled() {
		return nil, "controller observation is off ($" + padObservationEnv + ")"
	}
	src := padsource.New()
	if why := src.Unavailable(); why != "" {
		return nil, why
	}
	return src, ""
}

// startPadSource begins polling, if one was built.
func (r *Runtime) startPadSource(ctx context.Context) {
	if r == nil || r.padSource == nil {
		return
	}
	r.padSource.Start(ctx)
}

// stopPadSource ends polling.
func (r *Runtime) stopPadSource() {
	if r == nil || r.padSource == nil {
		return
	}
	r.padSource.Stop()
}

// padDeviceRecord is what to say about the controller when there is no source to ask.
//
// THE line that stops a missing producer from reading as a quiet player. A Director with no pad
// observer must report the controller as unobserved, by name, with the reason — not omit it, and
// certainly not let the keyboard's confidence stand in for it.
func padDeviceRecord(why string) observe.DeviceObservation {
	if why == "" {
		why = "no controller observer is wired"
	}
	return observe.DeviceObservation{
		Device:      observe.DeviceController,
		Watched:     false,
		Unavailable: why,
		Why:         "the controller was not observed, so its silence is not evidence",
	}
}

// mergeDeviceStats folds the controller producer's account into the keyboard producer's.
//
// The counters stay the keyboard hook's, because they are what every existing reader means by
// them and a sum across a hook and a poll is a number about nothing. What merges is the DEVICE
// RECORDS, which is the part a reader needs to know what the silence covers.
func mergeDeviceStats(base observe.InputStats, pad observe.InputStats,
	padWhy string) observe.InputStats {

	out := base
	out.Devices = append(append([]observe.DeviceObservation(nil), base.Devices...))
	if len(pad.Devices) > 0 {
		out.Devices = append(out.Devices, pad.Devices...)
		return out
	}
	out.Devices = append(out.Devices, padDeviceRecord(padWhy))
	return out
}

// padStats adds the controller's account to the keyboard producer's snapshot.
//
// A method on the sampler so the decision is made where the session's own producers are known.
// Either there is a pad source and it speaks for itself, or there is not and the record says
// which device nobody watched and why.
func (s *liveSampler) padStats(base observe.InputStats) observe.InputStats {
	if s == nil || s.rt == nil || s.rt.padSource == nil {
		why := ""
		if s != nil && s.rt != nil {
			why = s.rt.padUnavailable
		}
		return mergeDeviceStats(base, observe.InputStats{}, why)
	}
	return mergeDeviceStats(base, s.rt.padSource.Stats(), "")
}
