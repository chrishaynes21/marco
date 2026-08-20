package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/timeline"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// runDirector is Marco's thin client for the Director service.
//
// This is the stable entry point a front-end calls — the overlay, and through it
// voice. It builds no Director: construction lives in one place (cmd/director), and
// this process only submits a phrase and renders what comes back.
//
//	marco director "click File"     run a phrase
//	marco director stop             cancel whatever is running
//	marco director status           what the service is doing
//	marco director history          recent actions
//	marco director last             the most recent action
//
// # Wiring voice to this
//
// The overlay already dispatches by spawning marco.exe. Routing final voice phrases
// here is one call in plugins/overlay: where a recognised phrase currently becomes
// a route lookup, run `marco director "<phrase>"` instead (or as a fallback when no
// route matches). Nothing about Vosk recognition changes.
//
// The important property for that path: this client may be spawned fresh for every
// spoken phrase, and none of the Director's state lives here. A phrase spoken while
// a replay is running reaches the SAME service, which is what makes "stop" work.
func runDirector(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, `usage: marco director "<phrase>" | stop | status | history | last | shutdown
       marco director watch | diagnose | normal                [--json] [--cursor=N]
       marco director perception | explain | world | events   [--json] [--cursor=N]`)
		os.Exit(2)
	}

	jsonMode := false
	// eventCursor lets a polling front-end ask for only what it has not seen. Parsed
	// here rather than with the flag package because this command's first argument is a
	// free-text PHRASE, and flag parsing would try to interpret one that began with a
	// dash.
	var eventCursor uint64
	var rest []string
	for _, a := range args {
		switch {
		case a == "--json":
			jsonMode = true
		case strings.HasPrefix(a, "--cursor="):
			if n, err := strconv.ParseUint(strings.TrimPrefix(a, "--cursor="), 10, 64); err == nil {
				eventCursor = n
			}
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, `usage: marco director "<phrase>"`)
		os.Exit(2)
	}

	// Sub-commands are single bare words. A multi-word phrase is never one, so
	// "stop that" is not swallowed here — it goes through submission and is routed as
	// a control phrase there, which is the same outcome by the same rule.
	if len(rest) == 1 {
		switch strings.ToLower(rest[0]) {
		case "stop", "cancel":
			os.Exit(directorStop(jsonMode))
		case "status":
			os.Exit(directorStatus(jsonMode))
		case "history":
			os.Exit(directorHistory(jsonMode))
		case "last":
			os.Exit(directorHistoryN(jsonMode, 1))
		case "perception":
			os.Exit(directorPerception(jsonMode, false, eventCursor))
		case "explain":
			os.Exit(directorPerception(jsonMode, true, eventCursor))
		case "world":
			os.Exit(directorWorld(jsonMode))
		case "events":
			os.Exit(directorEvents(jsonMode, eventCursor))
		case "watch", "playbill":
			os.Exit(directorWatch(jsonMode, false, eventCursor))
		case "diagnose", "diagnostics":
			os.Exit(directorDiagnose(jsonMode, eventCursor))
		case "normal":
			os.Exit(directorNormal(jsonMode))
		case "live-analysis", "live":
			os.Exit(directorLiveAnalysis(jsonMode))
		case "observation-events":
			os.Exit(directorObservationEvents(jsonMode, eventCursor))
		case "shutdown":
			// Without this, `marco director shutdown` is submitted as a PHRASE and the
			// Director goes looking for something on screen called "shutdown" — which
			// is a surprising thing to happen to someone trying to stop a service.
			os.Exit(directorShutdown())
		}
	}
	os.Exit(directorSubmit(strings.Join(rest, " "), jsonMode))
}

// Exit codes, because the overlay dispatches this as a process and a status line is
// not something a script can branch on.
const (
	// exitOK — the command completed and was verified.
	exitOK = 0
	// exitNotVerified — the Director ran and reported something other than success.
	// The phrase WAS handled; there is nothing to fall back to.
	exitNotVerified = 1
	// exitUnavailable — the Director could not be reached at all. This is the only
	// code on which a caller should try its legacy path, because it is the only one
	// where the phrase was never delivered.
	exitUnavailable = 3
)

// directorDir is where the Director keeps its endpoint and graph.
func directorDir() string {
	if dir := os.Getenv("MARCO_HOME"); dir != "" {
		return dir
	}
	if home, err := os.UserConfigDir(); err == nil {
		return filepath.Join(home, "marco")
	}
	return "."
}

// directorBin locates the Director service executable.
//
// Next to marco.exe by default, because that is how Marco's binaries ship together.
// $DIRECTOR_BIN overrides it, matching the $MARCO_BIN convention the overlay uses.
func directorBin() string {
	if b := os.Getenv("DIRECTOR_BIN"); b != "" {
		return b
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "director.exe")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "director.exe"
}

func directorConnect(autoStart bool) (*service.Client, error) {
	return service.Connect(service.ConnectOptions{
		Dir:          directorDir(),
		ServiceBin:   directorBin(),
		ServiceArgs:  []string{"serve", "--quiet"},
		AutoStart:    autoStart,
		StartTimeout: 20 * time.Second,
		DialTimeout:  2 * time.Second,
	})
}

// directorReach connects, starting the service if needed, and retrying ONCE.
//
// Once, and no more. A retry covers the case worth covering — a service that had just
// exited, or one whose published endpoint was a moment stale — and a longer loop would
// only make a spoken phrase hang while the user waits for something to happen. Beyond
// one retry the honest answer is that the Director is unavailable, said quickly.
func directorReach() (*service.Client, error) {
	c, err := directorConnect(true)
	if err == nil {
		return c, nil
	}
	first := err
	if c, err = directorConnect(true); err == nil {
		return c, nil
	}
	return nil, fmt.Errorf("%v (after a retry; first attempt: %v)", err, first)
}

// directorSubmit sends one phrase and renders what comes back.
//
// The phrase is ROUTED, not interpreted: the client asks the service what it is doing,
// and that answer decides whether this is a cancellation, an answer to a pending
// question, or a new request. Which control "the second one" means is never decided
// here.
func directorSubmit(phrase string, jsonMode bool) int {
	c, err := directorReach()
	if err != nil {
		// Loud, and on stderr with the phrase included: a recognised phrase that
		// cannot be delivered must never vanish. A caller reading exitUnavailable
		// knows it is free to try its own path with this exact text.
		if jsonMode {
			_ = directorPrintJSON(map[string]any{
				"delivered": false, "phrase": phrase, "error": err.Error(),
			})
		} else {
			fmt.Fprintf(os.Stderr, "marco: the Director is unavailable: %v\n", err)
			fmt.Fprintf(os.Stderr, "marco: the phrase was not delivered: %q\n", phrase)
		}
		return exitUnavailable
	}
	defer c.Close()

	res, err := c.Submit(phrase, directorRender(jsonMode))
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		fmt.Fprintf(os.Stderr, "marco: phrase was %q\n", phrase)
		return exitNotVerified
	}
	if jsonMode {
		return directorPrintJSON(res)
	}

	switch {
	case res.Cancel != nil:
		fmt.Println(res.Cancel.Message)
		return exitOK

	case res.Clarification != nil:
		// The question and its candidates are printed as offered. Say the answer and
		// it becomes a CLARIFY for this command id — the numbering below is what "the
		// second one" refers to.
		fmt.Printf("CLARIFICATION_REQUIRED: %s\n", res.Clarification.Question)
		for _, cand := range res.Clarification.Candidates {
			if cand.Role != "" {
				fmt.Printf("  %d. %s (%s)\n", cand.Index, cand.Label, cand.Role)
			} else {
				fmt.Printf("  %d. %s\n", cand.Index, cand.Label)
			}
		}
		return exitNotVerified

	case res.Outcome != nil:
		out := res.Outcome
		fmt.Printf("%s: %s\n", strings.ToUpper(string(out.State)), out.Message)
		if out.CompletedActions > 0 {
			fmt.Printf("(%d action(s) performed)\n", out.CompletedActions)
		}
		if out.State == service.CommandCompleted {
			return exitOK
		}
		return exitNotVerified
	}
	fmt.Fprintln(os.Stderr, "marco: the Director returned nothing")
	return exitNotVerified
}

// directorRender turns the service's event stream into lines a person can follow.
//
// All seven events a front-end can receive are handled here, so nothing arrives
// unlabelled: ACKNOWLEDGED, PROGRESS, CLARIFICATION_REQUIRED, COMPLETED, UNVERIFIED,
// FAILED, CANCELLED. The terminal four are re-rendered by the caller with their
// counts; here they are only announced as they arrive, which is what makes a long
// replay legible while it runs.
func directorRender(jsonMode bool) func(service.ResponseEnvelope) {
	return func(ev service.ResponseEnvelope) {
		if jsonMode {
			return
		}
		switch ev.Type {
		case service.ResponseAcknowledged:
			var a service.AcceptedPayload
			if ev.Decode(&a) == nil {
				fmt.Printf("heard: %s\n", a.Phrase)
			}
		case service.ResponseProgress:
			var p service.ProgressPayload
			if ev.Decode(&p) == nil && p.Detail != "" {
				if p.Total > 0 {
					fmt.Printf("[%d/%d] %s\n", p.Iteration, p.Total, p.Detail)
				} else {
					fmt.Printf("%s\n", p.Detail)
				}
			}
		}
	}
}

func directorStop(jsonMode bool) int {
	// Never auto-start: asking a service to exist so it can be told to stop would
	// start a Director the user did not ask for.
	c, err := directorConnect(false)
	if err != nil {
		if jsonMode {
			fmt.Println(`{"accepted":false,"message":"the Director is not running"}`)
		} else {
			fmt.Println("nothing to stop — the Director is not running")
		}
		return 0
	}
	defer c.Close()

	res, err := c.Cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	if jsonMode {
		return directorPrintJSON(res)
	}
	fmt.Println(res.Message)
	return 0
}

func directorShutdown() int {
	c, err := directorConnect(false)
	if err != nil {
		fmt.Println("the Director is not running")
		return 0
	}
	defer c.Close()
	if err := c.Shutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	fmt.Println("Director service stopped")
	return 0
}

func directorStatus(jsonMode bool) int {
	c, err := directorConnect(false)
	if err != nil {
		if jsonMode {
			fmt.Println(`{"running":false}`)
		} else {
			fmt.Println("Director: not running")
		}
		return 1
	}
	defer c.Close()

	st, err := c.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	if jsonMode {
		return directorPrintJSON(st)
	}
	fmt.Printf("Director: running (pid %d, up %s)\n", st.PID, st.UptimeStr)
	if st.Active != nil {
		fmt.Printf("Active: %s", st.Active.Phrase)
		if st.Active.Total > 0 {
			fmt.Printf(" (%d/%d)", st.Active.Iteration, st.Active.Total)
		}
		fmt.Println()
	}
	// A waiting question is the most actionable thing status can report — it is the
	// service asking the user for something, and it changes what the next phrase means.
	if q := st.Clarification; q != nil {
		fmt.Printf("Waiting on an answer to %q: %s\n", q.Phrase, q.Question)
		for _, cand := range q.Candidates {
			fmt.Printf("  %d. %s\n", cand.Index, cand.Label)
		}
	}
	for _, p := range st.Providers {
		fmt.Printf("  %s: %s, %d elements\n", p.App, p.Status, p.Elements)
	}
	return 0
}

// directorPerception reports what the providers observed and what fusion made of it.
//
// # Why this belongs on the THIN client
//
// The full diagnostic surface lives on `director` itself, which is the right home for it:
// it is a developer tool with a dozen sub-commands. But the overlay shells to `marco`, not
// to `director`, and a front-end that cannot see the world model can only ever show what
// the Director SAID — never what it BELIEVES. That is the difference between a log and an
// instrument.
//
// # Why polling this is safe
//
// `Client.Perception` reads the service's own history rather than observing afresh. A
// diagnostic that took its own snapshot would report on a cycle nothing ever planned
// against, and would attach a second accessibility client to the desktop to do it. Because
// this reads what already happened, a HUD may poll it at whatever rate it likes without
// perturbing the session it is describing — which is the property that makes a live
// insight panel possible at all.
//
// deep asks for the per-element explanation, which `explain` sets and `perception` does
// not. The split is the diagnostics package's own: reconstructing an explanation is
// quadratic in the observation count, and it is the ONLY thing that fills Observations —
// so the anonymous share is available from `explain` and not from a poll. A HUD that
// polled the deep path would pay that cost forever to render one line.
func directorPerception(jsonMode, deep bool, eventCursor uint64) int {
	c, err := directorConnect(false)
	if err != nil {
		if jsonMode {
			// A shape the caller can parse either way. A front-end polling this gets
			// "not running" as data rather than as a parse error on empty output.
			// The exit code still says 1, matching `status` — the payload is for a
			// parser, the code is for a script, and they should not disagree.
			directorPrintJSON(directorInsight{Running: false})
		} else {
			fmt.Println("Director: not running")
		}
		return 1
	}
	defer c.Close()

	p, err := c.Perception()
	if deep {
		p, err = c.Explain()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}

	if jsonMode {
		// Status, world and events ride along on the SAME connection. A front-end
		// needs all of them to render one panel, and separate sub-commands would mean
		// four process spawns per refresh — the round trips are cheap, the spawns are
		// not. The payloads are the protocol's own, unmodified, so the CLI and the
		// overlay are looking at identical state.
		out := directorInsight{Running: true, Deep: deep, Perception: &p}
		if st, err := c.Status(); err == nil {
			out.PID, out.Uptime = st.PID, st.UptimeStr
			if st.Active != nil {
				out.Active = st.Active.Phrase
			}
		}
		if w, err := c.World(service.WorldPayload{Limit: insightWorldLimit}); err == nil {
			out.World = &w
		}
		if ev, err := c.Events(service.EventsPayload{
			Cursor: eventCursor, Limit: insightEventLimit}); err == nil {
			out.Events = &ev
		}
		return directorPrintJSON(out)
	}
	directorRenderPerception(p)
	return 0
}

// directorWorld reports what the Director currently believes.
//
// Cheap and read-only: the service copies the world it already holds, so this cannot start
// an observation however often it is called.
func directorWorld(jsonMode bool) int {
	c, code := directorRead(jsonMode)
	if c == nil {
		return code
	}
	defer c.Close()

	w, err := c.World(service.WorldPayload{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	if jsonMode {
		return directorPrintJSON(w)
	}
	directorRenderWorld(w)
	return 0
}

// directorEvents reports the perception event log from a cursor.
func directorEvents(jsonMode bool, cursor uint64) int {
	c, code := directorRead(jsonMode)
	if c == nil {
		return code
	}
	defer c.Close()

	ev, err := c.Events(service.EventsPayload{Cursor: cursor})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	if jsonMode {
		return directorPrintJSON(ev)
	}
	fmt.Printf("epoch %s   newest %d   oldest %d\n", ev.Epoch, ev.Newest, ev.Oldest)
	for _, e := range ev.Events {
		fmt.Print("  " + directorRenderEvent(e) + "\n")
	}
	return 0
}

// directorLiveAnalysis reports what a passive observation session has learned so far.
//
// Queries cached service state. Starts no observation, computes no analysis, mutates
// nothing.
func directorLiveAnalysis(jsonMode bool) int {
	c, code := directorRead(jsonMode)
	if c == nil {
		return code
	}
	defer c.Close()

	res, err := c.LiveAnalysis(service.LiveAnalysisPayload{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	if jsonMode {
		return directorPrintJSON(res)
	}
	directorRenderLive(res)
	return 0
}

// directorObservationEvents reports a session's findings after a cursor.
func directorObservationEvents(jsonMode bool, after uint64) int {
	c, code := directorRead(jsonMode)
	if c == nil {
		return code
	}
	defer c.Close()

	res, err := c.ObservationEvents(service.ObservationEventsPayload{After: after})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	if jsonMode {
		return directorPrintJSON(res)
	}
	if !res.Available {
		// Deliberately not "no session has run". A session that FINISHED also has no
		// live feed — its recorder retired with it — and saying otherwise would tell
		// somebody their completed session never happened.
		fmt.Println("no live event stream (no session is running)")
		fmt.Println("a finished session's findings: marco director live-analysis")
		return 0
	}
	fmt.Printf("session %s   newest %d   oldest %d   active %v\n",
		res.SessionID, res.Newest, res.Oldest, res.Active)
	if res.Gap {
		fmt.Println("  GAP — events were missed; refetch the analysis snapshot")
	}
	for _, e := range res.Events {
		fmt.Printf("  %4d %-38s %s\n", e.Sequence, e.Kind, liveEventDetail(e))
	}
	return 0
}

// liveEventDetail renders the safe part of one live event.
func liveEventDetail(e observe.LiveEvent) string {
	switch {
	case e.Concept != "":
		return fmt.Sprintf("%s band %d", e.Concept, e.ConfidenceBand)
	case e.TransitionKind != "":
		return string(e.TransitionKind)
	case e.Role != "":
		return fmt.Sprintf("%s %.0f%%", e.Role, e.PresenceRatio*100)
	}
	return e.Observed
}

// directorRenderLive prints the live analysis for a person.
//
// Sections mirror what a HUD shows, and for the same reason: direct evidence, aggregate
// uncertainty, and tentative interpretation are three different kinds of claim and must not
// read as one.
func directorRenderLive(res service.LiveAnalysisResponse) {
	if !res.Available {
		// Never zero-valued findings. "No session has run" and "a session found
		// nothing" are different, and printing empty sections for the first would be
		// inventing a result.
		fmt.Println("Live analysis unavailable")
		fmt.Println("Start a passive observation session to see temporal findings.")
		return
	}
	s := res.Summary
	// Three states, and the middle one matters most: a session that STOPPED without
	// completing still holds real evidence, and calling it either "complete" or
	// "unavailable" would misrepresent what is on screen.
	state := "ACTIVE"
	switch {
	case res.Active:
		state = "ACTIVE"
	case s.Complete:
		state = "SESSION COMPLETE"
	default:
		state = "INCOMPLETE SESSION — evidence retained; conclusions remain provisional"
		if s.TerminalReason != "" {
			state += "\n  ended: " + s.TerminalReason
		}
	}
	fmt.Printf("Session %s   samples %d   updated %dms ago   %s\n",
		s.SessionID, s.Samples, s.FreshnessMS, state)

	fmt.Printf("\nOBSERVED  (%d stable)\n", s.StableTotal)
	for _, e := range s.Stable {
		fmt.Printf("  %-10s %-22s %d/%d samples  band %d  last seen %dms\n",
			e.Role, safeLabelText(e.Label), e.SamplesSeen, e.SamplesTotal,
			e.ConfidenceBand, e.LastSeenMS)
	}

	u := s.Unreliable
	fmt.Println("\nUNRELIABLE")
	fmt.Printf("  %d entities did not hold still\n", u.Entities)
	fmt.Printf("  %d anonymous, %d withheld, %d flickering\n",
		u.Anonymous, u.Withheld, u.Flickering)

	fmt.Printf("\nPOSSIBLE  (%d active, %d withdrawn)\n",
		s.HypothesesTotal, s.WithdrawnHypotheses)
	if len(s.Hypotheses) == 0 {
		// Silence is a legitimate result. A session with insufficient evidence should
		// say so rather than reach for something to display.
		fmt.Println("  nothing yet — the evidence does not support an interpretation")
	}
	for _, h := range s.Hypotheses {
		fmt.Printf("  %-34s %d%%\n", h.Kind, h.ConfidenceBand*10)
		if h.Observed != "" {
			fmt.Printf("      supporting: %s\n", h.Observed)
		}
		for _, c := range h.Contradictions {
			fmt.Printf("      against:    %s\n", c)
		}
		if h.RecommendedValidation != "" {
			fmt.Printf("      validate:   %s\n", h.RecommendedValidation)
		}
	}
}

// safeLabelText renders a privacy-classified label.
func safeLabelText(l observe.SafeLabel) string {
	if l.Redacted {
		if l.Length == 0 {
			return "(unnamed)"
		}
		return fmt.Sprintf("(withheld %d)", l.Length)
	}
	if l.Text == "" {
		return "(unnamed)"
	}
	return l.Text
}

// directorRead connects for a read-only query, reporting "not running" consistently.
//
// A nil client means the caller should return the accompanying code; it is not an error
// worth a stack of duplicated messages in five commands.
func directorRead(jsonMode bool) (*service.Client, int) {
	c, err := directorConnect(false)
	if err != nil {
		if jsonMode {
			directorPrintJSON(directorInsight{Running: false})
		} else {
			fmt.Println("Director: not running")
		}
		return nil, 1
	}
	return c, 0
}

// directorRenderWorld prints the believed world for a person.
func directorRenderWorld(w service.WorldResponse) {
	if !w.Believed {
		// Distinct from an empty world, and the distinction matters: one means the
		// Director has not looked, the other that it looked and found nothing.
		fmt.Println("no world observed yet")
		return
	}
	fmt.Printf("world of %s   %d entities   %dms old\n",
		orDash(w.App), w.Total, w.FreshnessMS)
	if w.Truncated {
		fmt.Printf("  (showing %d of %d)\n", len(w.Entities), w.Total)
	}
	for _, e := range w.Entities {
		mark := " "
		if e.Actionable {
			mark = "*"
		}
		fmt.Printf("  %s %-10s %-12s %-26s conf %.2f  %2dc %5dms  %s\n",
			mark, e.Identity, e.Role, directorLabelText(e), e.Confidence,
			e.StableCycles, e.AgeMS, strings.Join(e.Sources, ","))
	}
}

// directorLabelText renders a privacy-classified label.
//
// A withheld label shows its digest and length rather than nothing, so a reader can still
// tell two withheld things apart and watch one persist — which is the whole reason the
// digest exists.
func directorLabelText(e service.WorldEntity) string {
	if e.Label.Redacted {
		if e.Label.Length == 0 {
			return "(unnamed)"
		}
		return fmt.Sprintf("(withheld %d, %s)", e.Label.Length, e.Label.Digest)
	}
	if e.Label.Text == "" {
		return "(unnamed)"
	}
	return e.Label.Text
}

// directorRenderEvent turns one event into a line.
func directorRenderEvent(e timeline.Event) string {
	head := fmt.Sprintf("%4d  %-18s", e.Seq, e.Kind)
	switch {
	case e.Source != "" && e.Count > 0:
		return fmt.Sprintf("%s %s (%d)", head, e.Source, e.Count)
	case e.Source != "":
		return head + " " + e.Source
	case e.Duration > 0:
		return fmt.Sprintf("%s %d in %s", head, e.Count, e.Duration.Round(time.Millisecond))
	case e.Count > 0:
		return fmt.Sprintf("%s %d", head, e.Count)
	case e.Detail != "":
		return head + " " + e.Detail
	}
	return head
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// directorInsight is the thin client's own JSON shape for a front-end panel.
//
// A wrapper rather than the bare Perception because a panel has to render "the Director
// is not running" as a normal state, and an absent process is not an error the way a
// failed round trip is.
type directorInsight struct {
	Running    bool                    `json:"running"`
	PID        int                     `json:"pid,omitempty"`
	Uptime     string                  `json:"uptime,omitempty"`
	Active     string                  `json:"active,omitempty"`
	Deep       bool                    `json:"deep,omitempty"`
	Perception *diagnostics.Perception `json:"perception,omitempty"`

	// World and Events are the protocol's own payloads, passed through unmodified.
	// Unmodified deliberately: if this reshaped them, the overlay and `marco director
	// world` would be looking at two different things and only one could be right.
	World  *service.WorldResponse  `json:"world,omitempty"`
	Events *service.EventsResponse `json:"events,omitempty"`
}

// Bounds for the combined front-end payload.
//
// Smaller than the service's own caps because this one is fetched on a timer: a HUD wants
// enough to render, not everything the Director holds. Both payloads report their true
// totals, so a bounded view is never mistaken for the whole picture.
const (
	insightWorldLimit = 60
	insightEventLimit = 40
)

// directorRenderPerception prints the picture for a person.
//
// Ordered so the load-bearing facts come first: a degraded source or a provider producing
// nothing explains every downstream number, and reading it last means re-reading the rest.
func directorRenderPerception(p diagnostics.Perception) {
	if len(p.Providers) == 0 {
		fmt.Println("no providers registered")
		return
	}
	fmt.Println("providers")
	for _, pr := range p.Providers {
		note := ""
		// A provider contributing nothing is the single most useful thing this
		// command can surface — it is the shape of the unpinned-accessibility defect,
		// where a source is present, healthy and reaching none of the samples.
		if pr.Observations == 0 {
			note = "  ← contributed nothing"
		}
		fmt.Printf("  %-14s %-28s %6d obs%s\n",
			pr.Name, strings.Join(pr.Sources, ","), pr.Observations, note)
	}

	f := p.Fusion
	fmt.Printf("\nfusion   %d observations → %d elements (merged %d, rejected %d)\n",
		f.ObservationCount, f.ElementCount, f.Merged, f.Rejected)
	if len(f.Degraded) > 0 {
		fmt.Printf("         degraded: %s\n", strings.Join(f.Degraded, ", "))
	}
	for _, c := range f.Conflicts {
		fmt.Printf("         conflict on %s: %s beat %s %s\n",
			c.Field, c.Winner, c.Loser, c.Values)
	}
	if !p.ProvenanceOK {
		fmt.Println("         PROVENANCE INCOMPLETE — a belief cannot be traced to its evidence")
	}

	fmt.Printf("\ncycles   %d kept of %d\n", p.CyclesKept, p.CyclesTotal)
	for _, c := range p.Cycles {
		scope := ""
		if c.Scoped {
			scope = " scoped"
		}
		fmt.Printf("  %-10v %6d obs in %-10s%s", c.ID, c.Count, c.Duration.Round(time.Millisecond), scope)
		if len(c.Failures) > 0 {
			fmt.Printf("  failures: %s", strings.Join(c.Failures, "; "))
		}
		fmt.Println()
	}

	if len(p.Observations) > 0 {
		fmt.Printf("\nobservations (%d of %d)\n", len(p.Observations), p.ObservationsTotal)
		for _, o := range p.Observations {
			label := o.Label
			if label == "" {
				// Withheld or genuinely unnamed. Either way an anonymous element is
				// one no request can name, so it is worth seeing as such.
				label = "(anonymous)"
			}
			fmt.Printf("  %-8s %-10s %-9s %-24s %.2f %s\n",
				o.Source, o.Kind, o.Role, label, o.Confidence, o.Bounds)
		}
	}
}

func directorHistory(jsonMode bool) int { return directorHistoryN(jsonMode, 20) }

func directorHistoryN(jsonMode bool, limit int) int {
	c, err := directorConnect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marco: the Director is not running")
		return 1
	}
	defer c.Close()

	resp, err := c.History(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	if jsonMode {
		return directorPrintJSON(resp.Entries)
	}
	for _, e := range resp.Entries {
		state := "failed"
		if e.Success {
			state = "ok"
		}
		fmt.Printf("%-10s %-8s %-26s %s\n", e.ID, state, e.Phrase, e.Reason)
	}
	return 0
}

// directorDiagnose is the Director's evidence PLUS this machine's Theater.
//
// # Why the roster belongs in a diagnostic
//
// Because "nothing happened" is the hard silence, and one of its explanations is not about the
// screen or the phrase at all: this machine has nothing that can act. That is a real refusal —
// `no_actor_available` — but it is only reachable by RUNNING a play, so the first time anybody
// finds out the Theater is empty is when a play they have just taught does nothing. `diagnose` is
// where a person goes when nothing happened, so it is where the answer should be.
//
// # Whose Theater this is
//
// THIS process's, built the way `marco do` builds it — the same accessibilityBridge() discovery,
// the same newTheaterHost. Not the service's: a Director in another process has its own actors,
// and reporting them here would answer a question nobody asked. What this says is "if you ran a
// play right now, from this Marco, here is who could perform it".
//
// Host.Last() rides along and is usually empty for the same reason: it is the refusal from the
// most recent production IN THIS PROCESS, and a fresh diagnose has not put one on. Shown when it
// is not empty rather than as a permanent blank line.
func directorDiagnose(jsonMode bool, cursor uint64) int {
	if !jsonMode {
		code := directorWatch(false, true, cursor)
		printTheater(theaterDiagnostics())
		return code
	}
	// One JSON document, not two: a consumer parses `diagnose --json` as a single object, so
	// the theater goes in as an added key rather than as a second document on stdout.
	view, code := fetchPlaybill(true, cursor)
	if b, err := json.Marshal(view); err == nil {
		var doc map[string]any
		if json.Unmarshal(b, &doc) == nil {
			doc["theater"] = theaterDiagnostics()
			directorPrintJSON(doc)
			return code
		}
	}
	directorPrintJSON(view)
	return code
}

// theaterReport is what this machine could cast, and why the last production did not go on.
type theaterReport struct {
	// Bridge is the accessibility provider that was FOUND, empty when there is none. The
	// path is the point: "no bridge" and "a bridge at a path you did not expect" are
	// different problems and they look identical from a play that did nothing.
	Bridge string `json:"bridge"`
	// Roster is every Actor and whether it can act, in casting order.
	Roster []theaterhost.Player `json:"roster"`
	// Last is the most recent refusal in this process, usually empty.
	Last string `json:"last,omitempty"`
}

func theaterDiagnostics() theaterReport {
	var accessibility runtime.Host
	bridge := accessibilityBridge()
	if bridge != "" {
		accessibility = bridgehost.New(bridge)
	}
	// Built exactly as newDeps builds it, over an empty act map: the roster asks each Actor
	// whether it could act, which no Actor answers by touching the desktop. Nothing here
	// launches the bridge, presses anything, or observes anything.
	h := newTheaterHost(map[string]runtime.Host{}, accessibility)
	return theaterReport{Bridge: bridge, Roster: h.Roster(context.Background()), Last: h.Last()}
}

func printTheater(r theaterReport) {
	fmt.Println()
	fmt.Println("theater")
	if r.Bridge == "" {
		fmt.Println("  accessibility provider: NOT FOUND")
		fmt.Println("    Nothing here can act on a target, so a learned play will do nothing.")
		fmt.Println("    Build it with .\\setup.ps1, or name one with $MARCO_UIA_BRIDGE.")
	} else {
		fmt.Println("  accessibility provider: " + r.Bridge)
	}
	if len(r.Roster) == 0 {
		fmt.Println("  cast: nobody — every learned play refuses no_actor_available")
	}
	for _, p := range r.Roster {
		state := "cannot act"
		if p.Available {
			state = "ready"
		}
		fmt.Printf("  %s: %s\n", p.Name, state)
	}
	if r.Last != "" {
		fmt.Println("  last refusal: " + r.Last)
	}
}

func directorPrintJSON(v any) int {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		return 1
	}
	return 0
}
