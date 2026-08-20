package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
)

// Reading a live shadow trace back, and refusing to interpret a bad one.
//
// The order here is deliberate and is the whole discipline of this command: integrity first,
// raw stream second, interpretation last. A trace that cannot say how many slots were skipped
// cannot support a cadence claim, and a report that renders the geometry anyway invites the
// reader to believe the part that is sound and the part that is not with equal confidence.

// loadTrace reads the JSONL slots, reporting a malformed line rather than skipping it.
func loadTrace(path string) ([]shadowreplay.Slot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var slots []shadowreplay.Slot
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8<<20)
	for line := 1; sc.Scan(); line++ {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		var s shadowreplay.Slot
		if err := json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, fmt.Errorf("line %d is malformed: %w", line, err)
		}
		if s.Outcome == "" {
			return nil, fmt.Errorf("line %d has no outcome — this trace predates "+
				"whole-slot recording and cannot answer the cadence question", line)
		}
		slots = append(slots, s)
	}
	return slots, sc.Err()
}

// reportTraceIntegrity is Part 3, and it runs before anything is interpreted.
func reportTraceIntegrity(slots []shadowreplay.Slot) error {
	cad := shadowreplay.MeasureCadence(slots)
	gens := map[uint64]int{}
	roles := map[string]int{}
	var detections int
	for _, s := range slots {
		if s.Generation != 0 {
			gens[s.Generation]++
		}
		for _, r := range s.Regions {
			roles[r.Role]++
			detections++
		}
	}

	fmt.Println("── trace integrity ──")
	fmt.Printf("  trace lines            %d\n", len(slots))
	fmt.Printf("  valid inferences       %d\n", cad.Valid)
	fmt.Printf("  skipped slots          %d\n", cad.Skipped)
	fmt.Printf("  failed inferences      %d\n", cad.Failed)
	fmt.Printf("  unproven (provenance)  %d\n", cad.Unproven)
	fmt.Printf("  window generations     %d %v\n", len(gens), sortedGenerations(gens))
	fmt.Printf("  detections             %d\n", detections)
	for _, r := range sortedCounts(roles) {
		fmt.Printf("    %-10s %d\n", r, roles[r])
	}

	// Privacy is asserted from the SHAPE of the data, not from intent. The trace schema has
	// no field for a label, so the check that matters is that geometry really is normalised
	// and window-relative: an absolute desktop coordinate cannot hide inside [0,1].
	var outside int
	for _, s := range slots {
		for _, r := range s.Regions {
			if r.Region.X < -0.001 || r.Region.Y < -0.001 ||
				r.Region.X+r.Region.Width > 1.001 || r.Region.Y+r.Region.Height > 1.001 {
				outside++
			}
		}
	}
	fmt.Printf("  geometry outside [0,1] %d\n", outside)
	if outside > 0 {
		return fmt.Errorf("%d region(s) are not window-relative normalised geometry; "+
			"refusing to interpret a trace that may carry desktop coordinates", outside)
	}
	if cad.Valid == 0 {
		return fmt.Errorf("no valid inference in %d slot(s) — nothing to analyse", len(slots))
	}
	// The semantic evidence, stated precisely rather than waved at. The trace does carry
	// something derived from on-screen text, and a blanket "no OCR text" would now be false
	// — which is worse than carrying it, because the next reader would trust the claim.
	terms := map[observe.InterfaceTerm]int{}
	for _, s := range slots {
		for _, t := range s.Semantic.Terms {
			terms[t]++
		}
	}
	fmt.Println("  no labels, no OCR text, no pixels: the schema carries none")
	if len(terms) > 0 {
		names := make([]string, 0, len(terms))
		for t := range terms {
			names = append(names, string(t))
		}
		sort.Strings(names)
		fmt.Printf("  interface concepts     %d, from the closed generic vocabulary: %s\n",
			len(terms), strings.Join(names, " "))
		fmt.Println("  these are matched WORDS, not the text they came from; a name typed " +
			"into a search box matches nothing and is not recorded")
	}
	fmt.Println()
	return nil
}

func runShadowTrace(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: director shadow-trace <trace.jsonl>")
		return 2
	}
	slots, err := loadTrace(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := reportTraceIntegrity(slots); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	cad := shadowreplay.MeasureCadence(slots)
	infs := shadowreplay.InferencesFrom(slots)
	res := shadowreplay.Run(infs, shadowreplay.Production())

	fmt.Println("── cadence, from recorded timestamps ──")
	fmt.Printf("  span %.1fs   valid %d   skipped %d   median gap %dms   max gap %dms\n\n",
		float64(cad.SpanMS)/1000, cad.Valid, cad.Skipped, cad.MedianMS, cad.MaxGapMS)

	fmt.Println("── live button stream (production geometry, unreinterpreted) ──")
	fmt.Printf("  %-5s %8s %6s %6s %6s %6s %6s  %-10s %s\n",
		"inf", "at(ms)", "conf", "x", "y", "w", "h", "track", "new/reason")
	for _, a := range res.Assignments {
		if a.Role != "button" {
			continue
		}
		at := int64(0)
		for _, in := range infs {
			if in.Index == a.Inference {
				at = in.AtMS
			}
		}
		fmt.Printf("  %-5d %8d %6.2f %6.3f %6.3f %6.3f %6.3f  %-10s %s\n",
			a.Inference, at, a.Confidence, a.Region.X, a.Region.Y,
			a.Region.Width, a.Region.Height, a.Track, a.Reason)
	}

	for _, role := range []string{"button", "icon"} {
		regions := shadowreplay.Cluster(infs, role, res)
		if len(regions) == 0 {
			continue
		}
		var recurring, tracks, dets int
		for _, r := range regions {
			if r.Recurring() {
				recurring++
			}
			tracks += len(r.Tracks)
			dets += r.Detections
		}
		fmt.Printf("\n── apparent %s regions (clustered at IoU %.2f, NOT the tracker's) ──\n",
			role, shadowreplay.ClusterIoU)
		fmt.Printf("  %d detections → %d apparent regions (%d recurring) → %d tracks\n",
			dets, len(regions), recurring, tracks)
		fmt.Printf("  %-6s %6s %6s %6s %6s %5s %5s %6s %s\n",
			"y", "x", "w", "h", "dets", "infs", "trks", "medIoU", "verdict")
		for _, r := range regions {
			verdict := "one-off — the tracker was right to mint it"
			if r.Recurring() && len(r.Tracks) == 1 {
				verdict = "recurring, one track — correct"
			} else if r.Recurring() && len(r.Tracks) > 1 {
				verdict = "RECURRING BUT SPLIT — tracking lost it"
			}
			fmt.Printf("  %-6.3f %6.3f %6.3f %6.3f %6d %5d %5d %6.2f %s\n",
				r.Mean.Y, r.Mean.X, r.Mean.Width, r.Mean.Height,
				r.Detections, r.Inferences, len(r.Tracks), r.MedianIoU, verdict)
		}
	}

	reportStates(slots)

	fmt.Println("\n── why each button track was created ──")
	reasons := map[string]int{}
	for _, a := range res.Assignments {
		if a.Role == "button" && a.NewTrack {
			reasons[a.Reason]++
		}
	}
	for _, r := range sortedCounts(reasons) {
		fmt.Printf("  %-36s %d\n", r, reasons[r])
	}
	return 0
}

// reportStates replays the trace through the PRODUCTION analyzer and reports state-local
// evidence beside the global figures.
//
// Deliberately `observe.ShadowTotals.Add` — the same call the live session path makes, screen
// segmentation and all — rather than a reimplementation here. A second analyzer written for the
// report would be free to agree with itself, which is the one thing that would make this
// worthless; the mirror in `shadowreplay` exists for the questions production cannot answer,
// and "what does production conclude" is not one of them.
func reportStates(slots []shadowreplay.Slot) {
	var totals observe.ShadowTotals
	for _, s := range slots {
		totals.Add(sampleFromSlot(s))
	}
	if len(totals.States) == 0 {
		fmt.Println("\n── screen states ──\n  none: no valid inference could be placed")
		return
	}

	fmt.Println("\n── screen states (production analyzer, generic and unnamed) ──")
	fmt.Printf("  %-10s %10s %9s %7s  %s\n", "state", "inferences", "episodes", "tracks",
		"composition")
	for _, st := range totals.States {
		roles := make([]string, 0, len(st.Roles))
		for r := range st.Roles {
			roles = append(roles, r)
		}
		sort.Strings(roles)
		var parts []string
		for _, r := range roles {
			parts = append(parts, fmt.Sprintf("%s×%d", r, st.Roles[r]))
		}
		fmt.Printf("  %-10s %10d %9d %7d  %s\n",
			st.ID, st.Inferences, st.Episodes, st.Tracks, strings.Join(parts, " "))
	}
	if len(totals.Transitions) > 0 {
		// The edges, with what preceded them. Correlation, never cause — the wording is the
		// same as the live report's on purpose, because the two must not be readable as
		// different strengths of claim.
		fmt.Println("\n  changes — the state graph's edges; navigation is CORRELATED, " +
			"never claimed as the cause")
		for _, tr := range totals.Transitions {
			line := fmt.Sprintf("    %s→%s ×%d", tr.From, tr.To, tr.Count)
			if intent, n := tr.Dominant(); intent != "" {
				line += fmt.Sprintf("   after %s in %d/%d", intent, n, tr.Count)
			}
			if tr.Unattributed > 0 {
				line += fmt.Sprintf("   no input in %d", tr.Unattributed)
			}
			if tr.ConditionalOnly > 0 {
				line += fmt.Sprintf("   context-admitted in %d", tr.ConditionalOnly)
			}
			fmt.Println(line)
			// Competing evidence, printed rather than summarised away. An edge preceded by
			// pause three times and confirm twice has no single cause, and a report that
			// showed only the dominant one would be inventing certainty it does not have.
			for _, intent := range competingIntents(tr) {
				fmt.Printf("      also %s in %d/%d\n", intent, tr.Preceded[intent], tr.Count)
			}
			for _, s := range tr.Sequences {
				if len(s.Intents) < 2 {
					continue
				}
				fmt.Printf("      order %s ×%d\n", joinIntents(s.Intents), s.Count)
			}
		}
	}
	renderTraceInputProducer(totals.Input)
	reportAdmissionOpportunity(slots)

	fmt.Println("\n── global presence vs state-local presence ──")
	fmt.Printf("  %-10s %-7s %-12s %-9s   %-12s %-9s %s\n",
		"track", "role", "global", "shape", "state-local", "shape", "state")
	for _, t := range totals.Tracks {
		st, ok := t.PrimaryState()
		if !ok || t.Seen < 2 {
			continue
		}
		fmt.Printf("  %-10s %-7s %2d/%-2d %5.0f%%  %-9s   %2d/%-2d %5.0f%%  %-9s %s\n",
			t.ID, t.Role, t.Seen, t.Eligible, t.PresenceRatio()*100, t.Shape,
			st.Seen, st.Eligible, st.PresenceRatio()*100, st.Shape, st.State)
	}

	for _, g := range totals.Groups {
		fmt.Printf("\n  %s — %d tracks co-occurring in %s over %d episode(s), "+
			"spacing %.3f uniformity %.2f\n", g.ID, len(g.Members), g.State, g.Episodes,
			g.MeanSpacing, g.Uniformity)
		fmt.Printf("    members %s\n", strings.Join(g.Members, " "))
		fmt.Printf("    envelope x %.3f y %.3f w %.3f h %.3f   nameable %d\n",
			g.Envelope.X, g.Envelope.Y, g.Envelope.Width, g.Envelope.Height, g.Nameable)
	}
}

// sampleFromSlot turns one traced slot back into the sample the session path produced.
//
// The four outcomes map to the four states a sample can be in, and the mapping matters: a
// skipped slot must come back as Ran=false rather than as an empty inference, or the replay
// would tell the tracker every element had vanished and manufacture the exact absence the
// cadence design exists to avoid.
// competingIntents is every correlated intent on an edge except the dominant one, most
// supported first.
//
// Its existence is the point: a single "dominant" line answers "what usually preceded this",
// and a reader will hear "what causes this" unless the alternatives are on the page beside it.
func competingIntents(tr observe.ScreenTransition) []observe.NavIntent {
	dominant, _ := tr.Dominant()
	out := make([]observe.NavIntent, 0, len(tr.Preceded))
	for intent := range tr.Preceded {
		if intent != dominant {
			out = append(out, intent)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if tr.Preceded[out[a]] != tr.Preceded[out[b]] {
			return tr.Preceded[out[a]] > tr.Preceded[out[b]]
		}
		return out[a] < out[b]
	})
	return out
}

// joinIntents renders one observed order.
func joinIntents(in []observe.NavIntent) string {
	parts := make([]string, 0, len(in))
	for _, i := range in {
		parts = append(parts, string(i))
	}
	return strings.Join(parts, " → ")
}

// renderTraceInputProducer reports the producer's own counters, restored from the trace.
//
// Printed even when it says "no producer was wired", because that sentence is the difference
// between a session in which the player pressed nothing and one in which nothing was listening
// — and every unattributed edge above reads differently depending on which it was.
func renderTraceInputProducer(in observe.InputStats) {
	fmt.Println("\n── navigation producer ──")
	switch {
	case in.Unavailable != "":
		fmt.Printf("  unavailable            %s\n", in.Unavailable)
		fmt.Println("  every edge above is unattributed for that reason, not because " +
			"nothing was pressed")
		return
	case in.Received == 0:
		fmt.Println("  no physical input was recorded in this trace")
		fmt.Println("  either no producer was wired when it was captured, or the trace " +
			"predates input recording")
		return
	}
	fmt.Printf("  intents classified     %d of %d events\n", in.Classified, in.Received)
	fmt.Printf("  ignored                %d\n", in.IgnoredTotal())
	for _, r := range sortedIgnoreReasons(in.Ignored) {
		fmt.Printf("    %-20s %d\n", r, in.Ignored[observe.IgnoreReason(r)])
	}
	// Reported separately from the total, because these are the intents whose reading
	// depended on Marco's assessment of the screen rather than on the key alone.
	fmt.Printf("  context-admitted       %d of %d classified\n", in.Conditional, in.Classified)
	fmt.Printf("  dropped (queue full)   %d\n", in.Dropped)
	if in.Dropped > 0 {
		fmt.Println("  the hook shed events under backpressure; the correlation above is " +
			"thinner than what the player actually did")
	}
	fmt.Println("  no key identity is recorded anywhere in this trace: the schema has no " +
		"field for one")
}

// reportAdmissionOpportunity says how much of a session looked like a set of choices.
//
// # Why this is in the report at all
//
// State-conditional admission reads an ambiguous key as navigation only while a choice screen is
// up, so "how often was one up" is the number that decides whether the mechanism can do anything
// in a given application. Without it, a session where every W was refused is indistinguishable
// from one where the relaxation was never given an opportunity to fire.
//
// # What it cannot tell you
//
// Whether a PARTICULAR past refusal would now be admitted. Admission is decided inside the
// producer at the moment of the press, and a trace records the outcome rather than the keystroke
// — raw key identity dies in the platform adapter by construction
// ([[ADR-013-navigation-is-meaning-not-keys]]). So a trace captured before this mechanism
// existed can bound the opportunity and cannot replay it. This is an upper bound on what
// changed, never a measurement of it.
func reportAdmissionOpportunity(slots []shadowreplay.Slot) {
	var valid, choices int
	for _, s := range slots {
		if s.Outcome != "valid" {
			continue
		}
		valid++
		if observe.MenuLike(s.Regions) {
			choices++
		}
	}
	if valid == 0 {
		return
	}
	fmt.Printf("  choice screens         %d of %d valid inferences (%.0f%%)\n",
		choices, valid, 100*float64(choices)/float64(valid))
	fmt.Println("  that share BOUNDS how often an ambiguous key could be read as navigation. " +
		"It does not say")
	fmt.Println("  which past refusals would now be admitted: a trace records outcomes, " +
		"not keystrokes")
}

func sortedIgnoreReasons(in map[observe.IgnoreReason]int) []string {
	out := make([]string, 0, len(in))
	for r := range in {
		out = append(out, string(r))
	}
	sort.Strings(out)
	return out
}

func sampleFromSlot(s shadowreplay.Slot) observe.ShadowSample {
	out := observe.ShadowSample{Detector: "screenparser", LatencyMS: s.LatencyMS}
	// Navigation is restored ahead of the switch, for the same reason the recorder writes it
	// ahead of one: three of the four outcomes return early, and the player kept playing
	// through every one of them.
	out.Inputs, out.InputStats, out.Semantic = s.Inputs, s.InputStats, s.Semantic
	switch s.Outcome {
	case "skipped":
		return out
	case "failed":
		out.Ran, out.Unavailable = true, s.Reason
		return out
	case "unproven":
		out.Ran = true
		return out
	}
	out.Ran, out.TargetProven = true, true
	out.Regions = s.Regions
	out.Detections = len(s.Regions)
	out.Roles = map[string]int{}
	for _, r := range s.Regions {
		out.Roles[r.Role]++
		if r.Nameable {
			out.Nameable++
		}
	}
	return out
}

func sortedCounts(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedGenerations(m map[uint64]int) []uint64 {
	out := make([]uint64, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}
