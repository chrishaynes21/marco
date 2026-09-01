package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/uiaclient"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WHICH PARTS OF WHAT MARCO SEES DEFINE THE STATE, AND WHICH ARE JUST PRESENTATION?
//
// # Why this exists
//
// Place identity is one static fingerprint — role counts, grid cells, a total — and the probes
// showed it failing in both directions at once. It FRAGMENTS when presentation changes and meaning
// does not (four durable Places for one Discord channel, three for one Settings page) and it
// COLLAPSES when meaning changes and structure does not (every Xbox game, one Place).
//
// Adding fields to the fingerprint cannot fix both: more evidence makes the fragmentation worse,
// and the collapsed case has no destination claim to add. What is missing is not a field. It is
// the ability to tell state-bearing evidence from presentation — which is a question about TIME,
// not about a single reading.
//
// So this is a measuring instrument, not a mechanism. It takes repeated readings of one window and
// reports what each evidence dimension DID: whether it held still, and what it held still AT. Run
// it on a state, on another state, and on the first again, and the three reports answer:
//
//	stable while you stay          a candidate discriminator
//	changed when you moved         a candidate discriminator
//	came back when you came back   a strong candidate discriminator
//	churned while you stayed       presentation
//
// # It reads and cannot do anything else
//
// No input, no session lifecycle beyond a bounded read, nothing durable, no authority. It is the
// same shape as `name-probe`: it calls the production representations rather than parsing the
// world itself, so what it reports and what production believes cannot come apart.
//
// # No application ever appears in this file
//
// The dimensions below are generic properties of an interface. `--title` names a window because a
// probe has to be pointed at something; nothing downstream of that knows or cares which program it
// is.

// evidenceDimension is one generic thing about a reading that might, or might not, define the
// semantic state.
//
// Deliberately a DIGEST per reading rather than the content. What the instrument needs to know is
// whether a dimension held still and whether it came back to a value it held before — both of
// which are equality questions. Printing the content would also put somebody's screen into a
// terminal, which is the thing the whole classifier exists to avoid; the label dimensions carry
// admitted text only, and the free ones carry a digest and a count.
type evidenceDimension struct {
	name string
	// of produces the value for one reading. Empty means the dimension had nothing to say,
	// which is itself informative — Xbox has no selected destination at all.
	of func(directorapi.WorldState) string
}

// evidenceDimensions is the whole vocabulary of the experiment.
//
// Every one is derived from what a World already holds. Nothing here is a new sensor, a new pass,
// or a second perception stack — the point is to find out which of the evidence Marco ALREADY has
// predicts semantic state, before anything is changed about how it is used.
func evidenceDimensions() []evidenceDimension {
	return []evidenceDimension{
		// THE FINGERPRINT AS IT STANDS. Everything else is measured against this,
		// because this is what identity is today.
		{"structural roles", func(w directorapi.WorldState) string {
			return digestCounts(rolesOf(w))
		}},
		// WHERE THINGS SIT, which is the other half of the fingerprint. Derived from the
		// same bounds identity uses, at the same grid resolution.
		{"structural cells", func(w directorapi.WorldState) string {
			return digestCounts(cellsOf(w))
		}},
		{"element count", func(w directorapi.WorldState) string {
			n := 0
			for _, el := range w.Elements {
				if el != nil && el.Visible && !el.Offscreen {
					n++
				}
			}
			return fmt.Sprintf("%d", n)
		}},
		// WHAT THE INTERFACE SAYS ABOUT WHERE YOU ARE. The claim rule's own answer,
		// asked through the production explainer so the probe cannot disagree with it.
		{"destination claim", func(w directorapi.WorldState) string {
			return observe.ExplainPlaceName(placeNameEvidence(w)).Name
		}},
		// SELECTION, wider than the claim rule: every selected element, whatever its role.
		// The claim rule needs Selected AND Navigable; this asks the first half alone,
		// because Xbox showed the conjunction can be empty when the screen plainly is not.
		{"selected labels", func(w directorapi.WorldState) string {
			var out []string
			for _, el := range w.Elements {
				if el == nil || !el.Visible || el.Offscreen || !el.Selected {
					continue
				}
				out = append(out, string(el.Role)+":"+el.Label)
			}
			return digestList(out)
		}},
		// THE HEADINGS AND PROMINENT LABELS a screen puts up about itself.
		{"headings", func(w directorapi.WorldState) string {
			var out []string
			for _, el := range w.Elements {
				if el == nil || !el.Visible || el.Offscreen {
					continue
				}
				if el.Role == directorapi.RoleHeading && strings.TrimSpace(el.Label) != "" {
					out = append(out, el.Label)
				}
			}
			return digestList(out)
		}},
		// WHAT CAN BE AIMED AT. The set, not the count: a screen whose buttons change
		// identity is a different screen from one whose buttons merely move.
		{"actionable set", func(w directorapi.WorldState) string {
			var out []string
			for _, el := range w.Elements {
				if el == nil || !el.Visible || el.Offscreen || !el.Role.Clickable() {
					continue
				}
				out = append(out, string(el.Role)+":"+el.Label)
			}
			return digestList(out)
		}},
		// AND EVERYTHING WRITTEN ON THE SCREEN, as a digest. The dimension most likely to
		// be pure presentation, and the one it is most important to measure rather than
		// assume: a chat window churns it constantly and a game's detail pane may not.
		{"all text", func(w directorapi.WorldState) string {
			var out []string
			for _, el := range w.Elements {
				if el == nil || !el.Visible || el.Offscreen {
					continue
				}
				if t := strings.TrimSpace(el.Label); t != "" {
					out = append(out, t)
				}
			}
			return digestList(out)
		}},
		// THE WINDOW ITSELF. Context rather than content, and the cheapest thing that
		// changes when a person moves between programs.
		{"window title", func(w directorapi.WorldState) string {
			for _, win := range w.Windows {
				if t := strings.TrimSpace(win.Title); t != "" {
					return t
				}
			}
			return ""
		}},
	}
}

// runEvidence takes repeated readings of one window and reports what each dimension did.
func runEvidence(args []string) int {
	fs := flag.NewFlagSet("evidence", flag.ExitOnError)
	title := fs.String("title", "", "the window whose title contains this")
	app := fs.String("application", "", "or the application to read")
	samples := fs.Int("samples", 6, "how many readings to take")
	gap := fs.Duration("gap", 1500*time.Millisecond, "how long to wait between readings")
	label := fs.String("label", "", "what to call this state in the report")
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	_ = fs.Parse(flagsFirst(args))

	if *app == "" && *title == "" {
		fmt.Fprintln(os.Stderr,
			"evidence: name a window with --title or an application with --application")
		return 2
	}
	if *samples < 2 {
		fmt.Fprintln(os.Stderr,
			"evidence: at least two readings, or there is no stability to measure")
		return 2
	}

	dims := evidenceDimensions()
	seen := make([]map[string]string, 0, *samples)
	for i := 0; i < *samples; i++ {
		if i > 0 {
			time.Sleep(*gap)
		}
		world, err := readWorldOnce(*bridgeFlag, *app, *title)
		if err != nil {
			fmt.Fprintf(os.Stderr, "evidence: %v\n", err)
			return 1
		}
		one := map[string]string{}
		for _, d := range dims {
			one[d.name] = d.of(world)
		}
		seen = append(seen, one)
	}

	name := strings.TrimSpace(*label)
	if name == "" {
		name = strings.TrimSpace(*title + *app)
	}
	fmt.Printf("\nSTATE %q — %d readings, %v apart\n\n", name, len(seen), *gap)
	fmt.Printf("  %-20s %-8s %s\n", "DIMENSION", "HELD", "VALUE (digest)")
	for _, d := range dims {
		values := map[string]int{}
		for _, s := range seen {
			values[s[d.name]]++
		}
		held := "yes"
		if len(values) > 1 {
			held = fmt.Sprintf("no (%d)", len(values))
		}
		// The value is only meaningful when it held still: a dimension that churned has
		// no value, which is the whole finding about it.
		shown := "—"
		if len(values) == 1 {
			shown = seen[0][d.name]
			if shown == "" {
				shown = "(nothing)"
			}
		}
		fmt.Printf("  %-20s %-8s %s\n", d.name, held, shown)
	}
	// AND THE DECISION THE GATE WOULD MAKE, from the same evidence. Printed because the
	// whole point of the instrument is to show which readings can say what they are — and a
	// reading that cannot is the one that may now buy a repair.
	claimed := ""
	for _, s := range seen {
		if s["destination claim"] != "" {
			claimed = s["destination claim"]
			break
		}
	}
	if claimed != "" {
		fmt.Printf("\n  SEMANTICS     this reading says which state it is (%q)\n", claimed)
		fmt.Println("                a repair would buy nothing")
	} else {
		fmt.Println("\n  SEMANTICS     this reading describes the interface and does not say")
		fmt.Println("                which state it is — one repair per settled screen")
	}
	fmt.Println()
	fmt.Println("  Run this on one state, on another, and on the first again.")
	fmt.Println("  A dimension that HELD on each and differed between them is a candidate")
	fmt.Println("  discriminator. One that did not hold is presentation.")
	fmt.Println()
	return 0
}

// digestCounts and digestList reduce a dimension to something comparable.
//
// A DIGEST, not the content. The instrument asks equality questions — did this hold still, did it
// come back — and printing what was on somebody's screen to answer them would put their display
// into a terminal. The count travels because "eleven things, and they changed" and "eleven things,
// unchanged" are different findings.
func digestCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%d;", k, m[k])
	}
	return shortDigest(b.String(), len(m))
}

func digestList(in []string) string {
	if len(in) == 0 {
		return ""
	}
	sorted := append([]string{}, in...)
	sort.Strings(sorted)
	return shortDigest(strings.Join(sorted, "\x00"), len(in))
}

func shortDigest(s string, n int) string {
	if s == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%s/%d", hex.EncodeToString(sum[:])[:8], n)
}

// rolesOf counts the roles a world reports, for the dimension that mirrors identity today.
func rolesOf(w directorapi.WorldState) map[string]int {
	out := map[string]int{}
	for _, el := range w.Elements {
		if el == nil || !el.Visible || el.Offscreen {
			continue
		}
		out[string(el.Role)]++
	}
	return out
}

// cellsOf is where things sit, at the grid resolution identity already uses.
//
// The same question `ScreenSignature.Cells` asks, asked of the fused world directly so the probe
// needs no shadow sample. An element with no rectangle sits nowhere and is left out, exactly as
// the production signature leaves it out.
func cellsOf(w directorapi.WorldState) map[string]int {
	out := map[string]int{}
	for _, el := range w.Elements {
		if el == nil || !el.Visible || el.Offscreen {
			continue
		}
		if el.Bounds.Width <= 0 || el.Bounds.Height <= 0 {
			continue
		}
		out[fmt.Sprintf("%s@%d,%d", el.Role, el.Bounds.X/100, el.Bounds.Y/100)]++
	}
	return out
}

// readWorldOnce fuses one reading of a window, through the production collector and engine.
//
// Lifted from `name-probe` unchanged in substance: the same tracker, the same providers, the same
// fusion engine. A probe that built its own world would be answering a question about itself.
func readWorldOnce(bridge, app, title string) (directorapi.WorldState, error) {
	ctx := context.Background()
	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	selector := windowref.Selector{Application: app}
	if title != "" {
		selector = windowref.Selector{Title: title}
	}
	ref := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), selector)
	if !ref.State.OK() {
		return directorapi.WorldState{}, fmt.Errorf("%s (%s)", ref.Reason, ref.State)
	}
	host := bridgehost.New(bridge)
	defer host.Close()
	uia := uiaclient.New(host)
	if !uia.Available(ctx) {
		return directorapi.WorldState{},
			fmt.Errorf("the accessibility provider reports it is unavailable")
	}
	collector := providers.NewCollector(
		providers.NewAccessibility(uia).WithTargetResolver(newTargetResolver(tracker)),
		providers.NewWindowSystem(windows),
	)
	window := ref.Ref.ID
	cycle := collector.Collect(ctx, observation.Request{
		Window: &window, Target: expectedTarget(ref.Ref),
	})
	world, _, err := fusion.NewEngine().Fuse(cycle)
	if err != nil {
		return directorapi.WorldState{}, fmt.Errorf("fusing: %w", err)
	}
	return world, nil
}
