package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// The Director insight panel: what the Director BELIEVES, not what it said.
//
// The overlay already streams the Director's event lines into the transcript, which is a
// log — it answers "what happened". This panel answers the different question the log
// cannot: what does the Director currently perceive, from which sources, and what did
// fusion make of it.
//
// # Why polling this is safe
//
// `marco director perception` reads the SERVICE's own observation history rather than
// observing afresh. A diagnostic that took its own snapshot would report on a cycle
// nothing ever planned against, and would attach a second accessibility client to the
// desktop to do it. Because this reads what already happened, the panel cannot perturb
// the session it describes — which is the property that makes a live view possible at all
// on a system where taking focus changes what is observed.
//
// # Why it polls only while open
//
// Each refresh spawns marco.exe. That is the overlay's normal way of reaching the engine,
// but a background poll running forever would spawn a process every couple of seconds for
// the entire session to render a panel nobody is looking at.
//
// # Why the anonymous share is not on the poll
//
// It comes from the per-element explanation, which the diagnostics package builds
// separately because it is quadratic in the observation count. `deep` fetches it once, on
// request. Polling it would pay that cost forever for one line.

// insightInterval is the refresh rate while the panel is open.
//
// A perception cycle is a few hundred milliseconds, so this is comfortably slower than
// the thing it reports and fast enough to feel live.
const insightInterval = 2 * time.Second

// insight is the decoded reply from `marco director perception --json`.
//
// Field names mirror the engine's directorInsight; only what the panel renders is
// declared, so a new field on the engine side is ignored rather than breaking the decode.
type insight struct {
	Running bool   `json:"running"`
	PID     int    `json:"pid"`
	Uptime  string `json:"uptime"`
	Active  string `json:"active"`
	Deep    bool   `json:"deep"`

	// World is the Director's current belief. Rendered, never recomputed: the overlay
	// does not decide what is actionable, how confident anything is, or whether a
	// label may be shown — all of that arrived decided.
	World struct {
		Believed    bool   `json:"believed"`
		App         string `json:"app"`
		Total       int    `json:"total"`
		Truncated   bool   `json:"truncated"`
		FreshnessMS int64  `json:"freshness_ms"`
		Entities    []struct {
			Identity string `json:"identity"`
			Role     string `json:"role"`
			Label    struct {
				Text     string `json:"text"`
				Digest   string `json:"digest"`
				Length   int    `json:"length"`
				Redacted bool   `json:"redacted"`
			} `json:"label"`
			Confidence   float64  `json:"confidence"`
			Sources      []string `json:"sources"`
			Actionable   bool     `json:"actionable"`
			StableCycles int      `json:"stable_cycles"`
			Stable       bool     `json:"stable"`
			AgeMS        int64    `json:"age_ms"`
		} `json:"entities"`
	} `json:"world"`

	// Events is the perception log slice, with the numbers that make loss detectable.
	Events struct {
		Epoch  string `json:"epoch"`
		Newest uint64 `json:"newest"`
		Oldest uint64 `json:"oldest"`
		Events []struct {
			Seq      uint64 `json:"seq"`
			Kind     string `json:"kind"`
			Source   string `json:"source"`
			Count    int    `json:"count"`
			Detail   string `json:"detail"`
			Duration int64  `json:"duration"`
		} `json:"events"`
	} `json:"events"`

	Perception struct {
		Providers []struct {
			Name         string   `json:"name"`
			Sources      []string `json:"sources"`
			Observations int      `json:"observations"`
		} `json:"providers"`
		Cycles []struct {
			Count    int      `json:"count"`
			Duration int64    `json:"duration"` // nanoseconds
			Scoped   bool     `json:"scoped"`
			Failures []string `json:"failures"`
		} `json:"cycles"`
		Fusion struct {
			ObservationCount int      `json:"observation_count"`
			ElementCount     int      `json:"element_count"`
			Merged           int      `json:"merged"`
			Rejected         int      `json:"rejected"`
			Degraded         []string `json:"degraded"`
		} `json:"fusion"`
		ProvenanceOK bool `json:"provenance_ok"`

		Observations []struct {
			Label string `json:"label"`
		} `json:"observations"`
		ObservationsTotal int `json:"observations_total"`
	} `json:"perception"`

	// err is set when the engine could not be reached at all, as distinct from the
	// Director not running. The panel says which, because "marco.exe is missing" and
	// "the Director is not started" want different things done about them.
	err string
}

// fetchInsight runs the thin client and decodes one reply.
//
// cursor is the highest event sequence already seen, so the service returns only what is
// new. One process spawn carries status, world and events together.
func fetchInsight(deep bool, cursor uint64) insight {
	sub := "perception"
	if deep {
		sub = "explain"
	}
	cmd := exec.Command(marcoBin(), "director", sub, "--json",
		fmt.Sprintf("--cursor=%d", cursor))
	out, err := cmd.Output()
	// A non-zero exit is expected when the Director is not running, and the payload is
	// still valid JSON saying so — so the output is decoded before the error is judged.
	var in insight
	if jsonErr := json.Unmarshal(out, &in); jsonErr != nil {
		if err != nil {
			return insight{err: "engine unreachable"}
		}
		return insight{err: "unreadable reply"}
	}
	return in
}

// feed is the rolling activity log the panel renders.
//
// Held here rather than in the model because it is derived entirely from what the Director
// sent: the overlay appends what arrived and drops the oldest. It never invents an entry
// and never infers one from a difference between polls.
type feed struct {
	lines  []string
	cursor uint64
	epoch  string
	// missed is set when the log rolled past the cursor. Reported rather than papered
	// over: dropped overlay frames are acceptable, dropped Director events are not.
	missed bool
}

const feedLines = 40

// absorb folds one reply's events into the feed and advances the cursor.
//
// The three cases are distinguished by the payload, not by guessing:
//   - a different epoch means the service RESTARTED — sequence numbers begin again, so
//     the cursor is reset rather than compared.
//   - an oldest newer than cursor+1 means the ring rolled PAST us and events were lost.
//   - otherwise the slice is simply what is new, possibly empty.
func (f *feed) absorb(in insight) {
	ev := in.Events
	if ev.Epoch == "" {
		return
	}
	if ev.Epoch != f.epoch {
		if f.epoch != "" {
			f.lines = append(f.lines, "— director restarted —")
		}
		f.epoch, f.cursor, f.missed = ev.Epoch, 0, false
	}
	if f.cursor > 0 && ev.Oldest > f.cursor+1 {
		f.missed = true
		f.lines = append(f.lines, fmt.Sprintf("— missed events %d..%d —",
			f.cursor+1, ev.Oldest-1))
	}
	for _, e := range ev.Events {
		f.lines = append(f.lines, renderEvent(e.Seq, e.Kind, e.Source, e.Detail,
			e.Count, e.Duration))
		if e.Seq > f.cursor {
			f.cursor = e.Seq
		}
	}
	if ev.Newest > f.cursor {
		f.cursor = ev.Newest
	}
	if len(f.lines) > feedLines {
		f.lines = f.lines[len(f.lines)-feedLines:]
	}
}

// renderEvent turns one event into a line. Presentation only — the event already says
// what happened, and this chooses how much of it fits.
func renderEvent(seq uint64, kind, source, detail string, count int, durNS int64) string {
	head := fmt.Sprintf("%4d %-17s", seq, kind)
	switch {
	case source != "" && count > 0:
		return fmt.Sprintf("%s %s %d", head, truncate(source, 10), count)
	case source != "":
		return head + " " + truncate(source, 14)
	case durNS > 0:
		return fmt.Sprintf("%s %d in %s", head, count,
			time.Duration(durNS).Round(time.Millisecond))
	case count > 0:
		return fmt.Sprintf("%s %d", head, count)
	case detail != "":
		return head + " " + truncate(detail, 14)
	}
	return head
}

// pollInsight refreshes the panel while it is open, and sleeps cheaply while it is not.
//
// Polling stops the moment the panel closes: each refresh spawns marco.exe, and a
// background poll for a panel nobody is looking at is pure cost. The Director neither
// knows nor cares whether anyone is polling — a disconnected overlay changes nothing.
func pollInsight(m *model) {
	f := &feed{}
	for {
		if !m.insightPolls() {
			time.Sleep(300 * time.Millisecond)
			continue
		}
		in := fetchInsight(false, f.cursor)
		f.absorb(in)
		m.setInsight(insightLines(in, f, m.inspectorOpen()))
		time.Sleep(insightInterval)
	}
}

// refreshInsightDeep takes the expensive explanation once, off the UI path.
//
// It freezes the panel first, so the live poll cannot overwrite the snapshot two seconds
// later — an expensive answer that flickers past is worse than not offering it.
func refreshInsightDeep(m *model) {
	m.freezeInsight()
	go func() {
		in := fetchInsight(true, 0)
		m.setInsight(insightLines(in, &feed{}, true))
	}()
}

// insightLines renders the reply as the panel's rows.
//
// Rendering to []string keeps this consistent with the help and config panels, which the
// view already knows how to lay out and scroll — a bespoke drawing path for one panel
// would be a second layout to maintain.
// This is the TEMPORARY inspector: it exists to prove the contracts, not to look like
// anything. Every value shown is printed as the Director sent it, so a discrepancy between
// this and `marco director world` is visible immediately rather than hidden behind
// formatting. The permanent HUD is built on these payloads once they are trusted.
func insightLines(in insight, f *feed, inspector bool) []string {
	if in.err != "" {
		return []string{"director  " + in.err}
	}
	if !in.Running {
		return []string{
			"director  not running",
			"",
			"start it with:  director serve",
		}
	}

	head := "director  running"
	if in.Uptime != "" {
		head += " " + in.Uptime
	}
	if in.Active != "" {
		head += "   " + truncate(in.Active, 24)
	} else {
		head += "   idle"
	}
	lines := []string{head, ""}

	p := in.Perception
	if len(p.Providers) == 0 {
		lines = append(lines, "no providers registered")
	}
	for _, pr := range p.Providers {
		note := ""
		// The single most useful thing this panel can surface. A provider that is
		// present and healthy but reaching none of the samples is the exact shape of
		// the unpinned-accessibility defect, and it is invisible in every other view.
		if pr.Observations == 0 {
			note = "  <- none"
		}
		lines = append(lines, fmt.Sprintf("%-10s %-13s %5d obs%s",
			truncate(pr.Name, 10), truncate(strings.Join(pr.Sources, ","), 13),
			pr.Observations, note))
	}

	fus := p.Fusion
	lines = append(lines, "",
		fmt.Sprintf("fusion  %d obs -> %d elements", fus.ObservationCount, fus.ElementCount),
		fmt.Sprintf("        merged %d  rejected %d", fus.Merged, fus.Rejected))
	if len(fus.Degraded) > 0 {
		lines = append(lines, "        degraded: "+truncate(strings.Join(fus.Degraded, ", "), 26))
	}
	if !p.ProvenanceOK {
		lines = append(lines, "        PROVENANCE INCOMPLETE")
	}

	if len(p.Cycles) > 0 {
		c := p.Cycles[0]
		scope := ""
		if c.Scoped {
			scope = " (scoped)"
		}
		lines = append(lines, fmt.Sprintf("cycle   %d obs in %s%s",
			c.Count, time.Duration(c.Duration).Round(time.Millisecond), scope))
		if len(c.Failures) > 0 {
			lines = append(lines, "        failed: "+truncate(strings.Join(c.Failures, "; "), 26))
		}
	}

	lines = append(lines, worldLines(in, inspector)...)
	lines = append(lines, feedRows(in, f)...)

	// The anonymous share only exists on the deep path, so the panel says so rather
	// than showing a zero that would read as "nothing is anonymous" — which is the
	// opposite of what is currently true.
	if in.Deep && len(p.Observations) > 0 {
		anon := 0
		for _, o := range p.Observations {
			if strings.TrimSpace(o.Label) == "" {
				anon++
			}
		}
		lines = append(lines, "",
			fmt.Sprintf("anonymous %d%% of %d shown", anon*100/len(p.Observations), len(p.Observations)))
		if p.ObservationsTotal > len(p.Observations) {
			// The browser view is bounded server-side. Saying so keeps the
			// percentage from reading as a figure for the whole cycle.
			lines = append(lines, fmt.Sprintf("        (cycle held %d)", p.ObservationsTotal))
		}
		lines = append(lines, "frozen — Esc, reopen for live")
	} else if in.Deep {
		lines = append(lines, "", "no observations to sample", "frozen — Esc, reopen for live")
	} else {
		lines = append(lines, "", "anonymous share: type 'explain'")
	}
	return lines
}

// worldLines renders the believed world.
//
// Every field is the Director's. The overlay does not decide what is actionable, compute a
// confidence, or choose whether a label may be shown — a redacted label arrives redacted
// and is rendered as a digest because that is all there is.
func worldLines(in insight, inspector bool) []string {
	w := in.World
	if !w.Believed {
		// "Has not looked" is not "looked and found nothing", and a HUD that showed
		// 0 entities for the first would be lying about the second.
		return []string{"", "world   not observed yet"}
	}

	head := fmt.Sprintf("world   %d entities  %dms old", w.Total, w.FreshnessMS)
	if w.App != "" {
		head = fmt.Sprintf("world   %s  %d entities  %dms", truncate(w.App, 12), w.Total, w.FreshnessMS)
	}
	lines := []string{"", head}
	if w.Truncated {
		lines = append(lines, fmt.Sprintf("        showing %d of %d", len(w.Entities), w.Total))
	}

	shown := len(w.Entities)
	if !inspector && shown > worldPreviewRows {
		shown = worldPreviewRows
	}
	for _, e := range w.Entities[:shown] {
		mark := " "
		if e.Actionable {
			mark = "*"
		}
		stable := " "
		if e.Stable {
			stable = "="
		}
		row := fmt.Sprintf("%s%s %-9s %-18s %.2f", mark, stable,
			truncate(e.Role, 9), entityLabel(e.Label.Redacted, e.Label.Text,
				e.Label.Digest, e.Label.Length), e.Confidence)
		if inspector {
			// The inspector adds the fields a validation pass needs to compare
			// against `marco director world`: identity, run length, age, sources.
			row += fmt.Sprintf("  %s %2dc %dms %s", e.Identity, e.StableCycles,
				e.AgeMS, truncate(strings.Join(e.Sources, ","), 16))
		}
		lines = append(lines, row)
	}
	if shown < len(w.Entities) {
		lines = append(lines, fmt.Sprintf("        +%d more (inspector)", len(w.Entities)-shown))
	}
	return lines
}

// entityLabel renders a privacy-classified label.
//
// A withheld label shows its digest, not a blank: the digest is what lets somebody watch
// the same unnamed thing persist across polls without ever learning what it says.
func entityLabel(redacted bool, text, digest string, length int) string {
	if redacted {
		if length == 0 {
			return "(unnamed)"
		}
		// A PREFIX, not a truncation: truncate() spends a character on an ellipsis,
		// and for a digest every character is the meaning. Eight hex digits is enough
		// to tell two withheld things apart by eye.
		return fmt.Sprintf("(%d %s)", length, prefix(digest, 8))
	}
	if text == "" {
		return "(unnamed)"
	}
	return truncate(text, 18)
}

// feedRows renders the activity log plus the cursor state a validation pass checks.
func feedRows(in insight, f *feed) []string {
	lines := []string{"", fmt.Sprintf("events  cursor %d  newest %d  oldest %d",
		f.cursor, in.Events.Newest, in.Events.Oldest)}
	if f.missed {
		lines = append(lines, "        EVENTS WERE MISSED")
	}
	if len(f.lines) == 0 {
		return append(lines, "        (quiet)")
	}
	rows := f.lines
	if len(rows) > feedPreviewRows {
		rows = rows[len(rows)-feedPreviewRows:]
	}
	for _, l := range rows {
		lines = append(lines, "  "+l)
	}
	return lines
}

const (
	// worldPreviewRows and feedPreviewRows bound the normal panel. The inspector shows
	// everything it was sent; the HUD shows a glance.
	worldPreviewRows = 6
	feedPreviewRows  = 8
)

// prefix returns the first n characters, with no ellipsis.
func prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "."
}
