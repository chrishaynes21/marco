// Command perceptionreplay re-runs a recorded session through the production perception path.
//
// # Why this exists
//
// `shadowtrace.go` records every slot's regions and shapes IN FULL, and says why:
//
//	a replay that reconstructed a slot without its shapes would feed the tracker a
//	different screen from the one production fed it — and the whole purpose of this trace
//	is that production and replay are the same measurement.
//
// Nothing replayed it. Experiment-025 failed its acceptance test, and answering "why" meant
// regrouping the run's geometry with an ad-hoc script that was not the production rule — then
// walking one of its conclusions back. The evidence to do it properly was on disk the whole
// time, in a format designed for it.
//
// # What it replays, and what it cannot
//
// Three levels of evidence, and this reads the middle one:
//
//	L0  frames, inputs, window identity          the pixels. A recorder writes these.
//	L1  detector output per frame                regions, shapes, shading, inputs.  <- here
//	L2  Director's own conclusions               beliefs, hypotheses, relationships.
//
// From L1 this exercises grouping, candidate tracking, the selection reader and the interface
// belief — every stage Experiment-025 failed at — against the real recorded screen. It CANNOT
// test a change to the detector, to admission, or to how shading is measured, because those
// produced L1 and are upstream of it. Those need L0, and no recording has L0 yet.
//
// A replay also proves only how well Marco interprets a recording. It cannot establish that
// Marco would interact successfully with the live application.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// slot is the subset of a traced slot a replay needs.
//
// Declared here rather than imported from cmd/director: that package is a main, and a replay
// that shared its struct would couple a diagnostic to a binary. The JSON tags are the
// contract, and replayContract_test.go holds them against the recorder's own type.
type slot struct {
	N          int                    `json:"n"`
	AtMS       int64                  `json:"at_ms"`
	Outcome    string                 `json:"outcome"`
	Generation uint64                 `json:"generation"`
	LatencyMS  int64                  `json:"latency_ms"`
	Detections int                    `json:"detections"`
	Reason     string                 `json:"reason"`
	Regions    []observe.ShadowRegion `json:"regions"`
	Shapes     []observe.ShadowShape  `json:"shapes"`
	Inputs     []observe.InputEvent   `json:"inputs"`
}

func main() {
	verbose := flag.Bool("v", false, "print every look, not just the ones that changed something")
	frames := flag.Bool("frames", false,
		"replay the original PIXELS of a capture-game-session recording (L0), "+
			"rather than a shadow trace (L1)")
	stride := flag.Int("stride", 1, "with -frames, reprocess every Nth frame")
	limit := flag.Int("limit", 0, "with -frames, stop after N frames (0 = all)")
	observer := flag.Bool("observer", false,
		"with -frames, fold the recording through the PRODUCTION fast observer and "+
			"measure what it sustains, with and without a consumer keeping up")
	flow := flag.Bool("workflow", false,
		"with -frames, learn a workflow from the recording and plan it back against "+
			"the same pixels: record, learn, plan, lower, verify. Executes nothing")
	propose := flag.Int("propose", 0,
		"with -frames, draw and list every candidate the production proposer found on "+
			"one look (1-based), with -near to ask about one rectangle")
	near := flag.String("near", "",
		"with -propose, a rectangle in frame pixels as x,y,w,h: the candidates that "+
			"overlap it, and how much of it each covers")
	bindscreens := flag.Bool("bind", false,
		"with -frames, split the recording into two independent observation "+
			"folds that share no screen identities, and measure whether a screen "+
			"learned in one can be recognised in the other by arrangement alone")
	diagnose := flag.String("diagnose", "",
		"with -frames, explain WHY each named target (comma separated, empty = all) "+
			"is or is not found again on later looks of its own screen: the "+
			"geometric relation between the rectangle it was pressed as and the "+
			"rectangles the proposer offers later")
	crops := flag.String("crops", "",
		"with -diagnose, write the pixels behind the geometry to this directory: "+
			"the reading, its containers and the closest later proposal, outlined")
	reacquire := flag.Bool("reacquire", false,
		"with -frames, measure whether a demonstrated click target can be found again "+
			"on later looks of its own screen, from the proposals alone")
	capturecost := flag.Bool("capturecost", false,
		"with -frames, measure where an observation's time goes and what a smaller "+
			"one keeps. Captures nothing from the screen")
	cadence := flag.Bool("cadence", false,
		"with -frames, measure the screen model at several sampling rates and phases "+
			"instead of replaying once")
	authorise := flag.Bool("authorise", false,
		"with -readiness, supply a SIMULATED execution authorisation; nothing is executed either way")
	readiness := flag.Bool("readiness", false,
		"follow a reloaded workflow through the production contracts, executing nothing")
	anchor := flag.Bool("anchor", false,
		"measure how many explicit screen confirmations a saved route needs in a fresh observer")
	screenid := flag.Bool("screenid", false,
		"ask whether a demonstrated screen can be recognised again from saved visual evidence")
	clarify := flag.String("clarify", "",
		"print what Marco would ask about a saved workflow artifact, and what each answer would change")
	appearance := flag.Bool("appearance", false,
		"with -reload, ask whether appearance separates a control from its container")
	reload := flag.Bool("reload", false,
		"learn and save from the first half, then bind and re-acquire it from a fresh observer over the second")
	save := flag.String("save", "",
		"with -workflow, write the learned workflow as a durable artifact and read it back")
	onnx := flag.String("onnx", "",
		"with -workflow, replay recorded detections as a second candidate source")
	onnxdelay := flag.Int("onnxdelay", 8,
		"with -onnx, how many looks behind its own frame a detection arrives")
	inspect := flag.Int("inspect", -1,
		"print the grouping of one frame index against the labelled rows, and stop")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: perceptionreplay [-v] <run directory>...\n\n"+
			"Replays each run's slots.jsonl through the production interface watch and\n"+
			"reports what it discovered. Several runs are compared side by side.\n")
	}
	flag.Parse()
	// AN ARTIFACT IS NOT A RECORDING. -clarify reads a saved workflow and asks
	// nothing of the frames, so it runs before the loop that wants a capture
	// directory — and needs no directory argument at all.
	if *clarify != "" {
		if err := reportClarifications(*clarify, 1920, 1080); err != nil {
			fmt.Fprintf(os.Stderr, "perceptionreplay: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var results []result
	for _, dir := range flag.Args() {
		if *inspect >= 0 {
			if err := inspectFrame(dir, *inspect); err != nil {
				fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
				os.Exit(1)
			}
			continue
		}
		if *frames {
			if *observer {
				for _, consume := range []bool{true, false} {
					if err := reportObserverOverRecording(dir, *limit, consume); err != nil {
						fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
						os.Exit(1)
					}
				}
				continue
			}
			if *flow {
				if err := reportWorkflowOverRecording(dir, *limit, *onnx, *onnxdelay, *save); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *readiness {
				if err := reportReadiness(dir, *limit, *onnx, *authorise); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *anchor {
				if err := reportAnchoring(dir, *limit, *onnx); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *screenid {
				if err := reportScreenRecognition(dir, *limit, *onnx); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *reload {
				if err := reportReloadOverRecording(dir, *limit, *onnx,
					*onnxdelay, *appearance); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *bindscreens {
				if err := reportScreenBinding(dir, *limit); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *propose > 0 {
				if err := reportProposals(dir, *propose, *near,
					*crops); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *diagnose != "" || *crops != "" {
				if err := reportTargetDiagnosis(dir, *limit, *diagnose,
					*crops); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *reacquire {
				if err := reportReacquisitionOverRecording(dir, *limit, *onnx, *onnxdelay); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *capturecost {
				if err := reportCaptureCost(dir, *limit); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if *cadence {
				if err := reportCadence(dir, *limit); err != nil {
					fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
					os.Exit(1)
				}
				continue
			}
			if _, err := replayFrames(dir, *stride, *limit, *verbose); err != nil {
				fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
				os.Exit(1)
			}
			continue
		}
		r, err := replay(dir, *verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "perceptionreplay: %s: %v\n", dir, err)
			os.Exit(1)
		}
		results = append(results, r)
	}
	if len(results) > 1 {
		compare(results)
	}
}

// minUsableStack is the smallest group the selection tracker will consider at all.
//
// Mirrors selection's own unexported minStack. Duplicated with that stated, because the
// alternative is exporting a perception threshold so a diagnostic can print it: the
// diagnostic being wrong about it is a reporting bug, and the tracker being wrong about it is
// a perception bug. replaycontract_test.go holds the two together.
const minUsableStack = 3

// result is what one replayed run produced.
type result struct {
	Name string
	// Looks is slots offered to the watch; Observed, the ones that carried evidence.
	Looks, Observed int
	// Grouped is looks on which grouping proposed at least one stack, and Rows the total
	// rows it proposed. The first stage, and the one Experiment-025 never got past.
	Grouped, Rows int
	// Stacks is the most distinct groups proposed on any one look.
	Stacks int
	// WithUsable is looks that proposed at least one group the tracker will even
	// consider, and NoUsableStack the looks that proposed only groups below its
	// minimum. Together they are the grouping finding: a screen fragmented into pairs
	// and singletons is refused before the selection rule is ever reached.
	WithUsable, NoUsableStack int
	// BiggestStack is the largest group on any look, and StackSizes the distribution of
	// the usable ones. The distribution is the diagnosis: many threes is fragmentation,
	// one six is a menu.
	BiggestStack int
	StackSizes   map[int]int
	// Retired is how many candidates were dropped for not being found again.
	Retired int
	// Identified is the look on which a single list first qualified, or -1.
	Identified int
	// Selections is how many looks reported a selected entry at all.
	Selections int
	// Moments is the belief journal's length: how many times the answer changed.
	Moments int
	// Candidates is the final per-candidate evidence.
	Candidates []observe.InterfaceCandidate
	// Shortfalls counts, across every candidate, which requirement failed.
	Shortfalls map[string]int
	Belief     observe.InterfaceBelief
	Presses    int
}

func replay(dir string, verbose bool) (result, error) {
	path := filepath.Join(dir, "slots.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return result{}, err
	}
	defer f.Close()

	r := result{Name: filepath.Base(dir), Identified: -1,
		Shortfalls: map[string]int{}, StackSizes: map[int]int{}}
	watch := observe.NewInterfaceWatch()
	// A deterministic clock. The trace carries at_ms from the producer's start, and a
	// replay that used time.Now would make the belief's timestamps depend on when it was
	// run — which is the difference between a regression corpus and a log.
	base := time.Unix(1700000000, 0).UTC()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<26)
	line := 0
	for sc.Scan() {
		line++
		b := strings.TrimSpace(sc.Text())
		if b == "" {
			continue
		}
		var s slot
		if err := json.Unmarshal([]byte(b), &s); err != nil {
			return result{}, fmt.Errorf("%s line %d: %w", path, line, err)
		}
		r.Looks++
		r.Presses += len(s.Inputs)

		// THE PRODUCTION DOOR. Observe is what the live cycle calls, with a look built
		// the way shadow.go builds one. Rebuilding the look here rather than calling an
		// internal helper is deliberate: the fields are the recording's contract, and a
		// replay that reached past them would stop being a measurement of production.
		look := observe.InterfaceLook{
			Sequence: s.N,
			At:       base.Add(time.Duration(s.AtMS) * time.Millisecond),
			Observed: s.Outcome == "valid",
			Why:      s.Reason,
			Regions:  s.Regions,
			Shapes:   s.Shapes,
		}
		if look.Observed {
			r.Observed++
		}
		before := watch.Belief()
		watch.Observe(look)
		b2 := watch.Belief()

		// What grouping made of this look, measured through the same call the watch
		// makes so the two cannot disagree.
		sizes := observe.GroupedForLook(look)
		rows, stacks, usable := 0, len(sizes), 0
		for _, n := range sizes {
			rows += n
			if n >= minUsableStack {
				usable++
				r.StackSizes[n]++
			}
		}
		if stacks > 0 {
			r.Grouped++
			r.Rows += rows
			if stacks > r.Stacks {
				r.Stacks = stacks
			}
		}
		if usable > 0 {
			r.WithUsable++
			if sizes[0] > r.BiggestStack {
				r.BiggestStack = sizes[0]
			}
		} else {
			r.NoUsableStack++
		}
		if b2.Selected >= 0 {
			r.Selections++
		}
		if verbose || before.Status != b2.Status || before.Selected != b2.Selected {
			fmt.Printf("  look %-4d %-12s sel %-3d  rows %-3d stacks %-2d  %s\n",
				s.N, b2.Status, b2.Selected, rows, stacks, trim(b2.Because, 92))
		}
	}
	if err := sc.Err(); err != nil {
		return result{}, err
	}

	r.Belief = watch.Belief()
	r.Identified = r.Belief.After
	r.Moments = len(watch.Moments())
	r.Candidates = watch.Candidates()
	r.Retired = watch.RetiredCandidates()
	for _, c := range r.Candidates {
		for _, u := range c.Unmet {
			r.Shortfalls[u.Requirement]++
		}
	}
	report(r)
	return r, nil
}

func report(r result) {
	fmt.Printf("\n%s\n", strings.Repeat("=", 78))
	fmt.Printf("REPLAY %s   (L1: detector output, not pixels)\n", r.Name)
	fmt.Printf("%s\n", strings.Repeat("=", 78))
	fmt.Printf("  looks              %d, of which %d carried evidence\n", r.Looks, r.Observed)
	fmt.Printf("  inputs observed    %d\n", r.Presses)
	fmt.Printf("  grouping           %d look(s) produced any group, %d rows total, "+
		"most groups on one look %d\n", r.Grouped, r.Rows, r.Stacks)
	fmt.Printf("  usable stacks      %d look(s) had a group of >=%d; %d had only "+
		"smaller ones\n", r.WithUsable, minUsableStack, r.NoUsableStack)
	fmt.Printf("  biggest group      %d rows\n", r.BiggestStack)
	if len(r.StackSizes) > 0 {
		sizes := make([]int, 0, len(r.StackSizes))
		for n := range r.StackSizes {
			sizes = append(sizes, n)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
		var parts []string
		for _, n := range sizes {
			parts = append(parts, fmt.Sprintf("%d rows x%d", n, r.StackSizes[n]))
		}
		fmt.Printf("  stack sizes        %s\n", strings.Join(parts, "   "))
	}
	fmt.Printf("  identified after   %d\n", r.Identified)
	fmt.Printf("  looks with a selection  %d\n", r.Selections)
	fmt.Printf("  belief             %s, selected %d, reading %s\n",
		r.Belief.Status, r.Belief.Selected, r.Belief.Reading)
	fmt.Printf("  because            %s\n", trim(r.Belief.Because, 160))
	if len(r.Candidates) == 0 {
		fmt.Printf("  candidates         none were ever tracked\n")
		return
	}
	fmt.Printf("  candidates         %d\n", len(r.Candidates))
	fmt.Printf("    %-5s %-6s %-6s %-5s %-6s %-5s %-6s %-5s %-7s %s\n",
		"rows", "frames", "judged", "excl", "shared", "quiet", "unread", "moves",
		"holders", "verdict")
	for _, c := range r.Candidates {
		verdict := "refused"
		if c.Qualifies {
			verdict = "IDENTIFIED"
		}
		fmt.Printf("    %-5d %-6d %-6d %-5d %-6d %-5d %-6d %-5d %-7d %s\n",
			c.Members, c.Frames, c.Judged, c.Exclusive, c.Shared, c.Quiet,
			c.Unreadable, c.Transfers, c.Holders, verdict)
	}
	if len(r.Shortfalls) > 0 {
		keys := make([]string, 0, len(r.Shortfalls))
		for k := range r.Shortfalls {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s %d", k, r.Shortfalls[k]))
		}
		fmt.Printf("  shortfalls         %s\n", strings.Join(parts, "   "))
	}
}

// compare puts two or more replays beside each other on the numbers that matter.
//
// The point of a corpus is version comparison, and a comparison a person has to do by reading
// two reports is one that will not be done.
func compare(rs []result) {
	fmt.Printf("\n%s\nCOMPARISON\n%s\n", strings.Repeat("=", 78), strings.Repeat("=", 78))
	fmt.Printf("  %-22s %7s %7s %7s %8s %7s %s\n",
		"run", "looks", "grouped", "rows", "ident@", "sel", "belief")
	for _, r := range rs {
		fmt.Printf("  %-22s %7d %7d %7d %8d %7d %s\n",
			trim(r.Name, 22), r.Looks, r.Grouped, r.Rows, r.Identified,
			r.Selections, r.Belief.Status)
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
