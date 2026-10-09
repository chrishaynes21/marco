package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// `director read-selection` — run the visual selection reader over a captured session.
//
// The measuring surface for [[Experiment-023-menu-selection-state]]. It reads frames in
// order, asks the reader what is chosen on each, and compares that with an annotation file,
// keeping three things apart that a single accuracy number would blend into mush:
//
//	correct            the reader named the entry the annotation names
//	confidently wrong  the reader named a DIFFERENT entry — the failure that matters,
//	                   because something acting on it would act on the wrong thing
//	uncertain          the reader declined, which on a transition is the right answer
//
// Evaluation is over CONTIGUOUS segments. Shuffling frames would put a frame's own
// neighbours on both sides of the split, and a reader that uses temporal evidence would be
// scored partly on information the split was supposed to withhold.

type annotation struct {
	Session    string `json:"session"`
	Provenance string `json:"provenance"`
	Intervals  []struct {
		From  int     `json:"from"`
		To    int     `json:"to"`
		State string  `json:"state"`
		Entry string  `json:"entry"`
		Y     float64 `json:"y"`
	} `json:"intervals"`
}

type frameLine struct {
	Index int    `json:"index"`
	File  string `json:"file"`
	AtMS  int64  `json:"at_ms"`
}

// tally is one segment's counts.
type tally struct {
	truthSelected, truthUncertain          int
	correct, confidentlyWrong, declined    int
	confidentOnTransition, correctlyUnsure int
	noneSelected                           int
}

func (t *tally) add(o tally) {
	t.truthSelected += o.truthSelected
	t.truthUncertain += o.truthUncertain
	t.correct += o.correct
	t.confidentlyWrong += o.confidentlyWrong
	t.declined += o.declined
	t.confidentOnTransition += o.confidentOnTransition
	t.correctlyUnsure += o.correctlyUnsure
	t.noneSelected += o.noneSelected
}

func runReadSelection(args []string) int {
	fs := flag.NewFlagSet("read-selection", flag.ExitOnError)
	dir := fs.String("session", "", "a captured session directory")
	annPath := fs.String("annotations", "", "annotation file (default <session>/annotations.json)")
	region := fs.String("region", "", "normalised x0,y0,x1,y1 to look for rows in (default the whole frame)")
	// Rows supplied by hand are HARNESS INPUT, never production logic. They exist to
	// separate two questions that a single number confuses: can the reader tell which row
	// is chosen, and can the band finder locate the rows in the first place. A failure of
	// the second would otherwise be reported as a failure of the first.
	givenRows := fs.String("rows", "", "harness only: groups of normalised row centres, "+
		"groups separated by | and centres by , (e.g. 0.44,0.49|0.90)")
	rowHeight := fs.Float64("row-height", 0.03, "harness only: normalised height of a supplied row")
	segments := fs.Int("segments", 3, "contiguous segments to report separately")
	limit := fs.Int("limit", 0, "stop after this many frames (0 = all)")
	_ = fs.Parse(flagsFirst(args))

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "director: --session is required\n"+
			"  example: director read-selection --session .tmp/game-capture-review/x")
		return 2
	}
	if *annPath == "" {
		*annPath = filepath.Join(*dir, "annotations.json")
	}

	frames, err := readFrameIndex(filepath.Join(*dir, "frames.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	ann, err := readAnnotations(*annPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	truth := map[int]struct {
		state string
		y     float64
	}{}
	for _, iv := range ann.Intervals {
		for i := iv.From; i <= iv.To; i++ {
			truth[i] = struct {
				state string
				y     float64
			}{iv.State, iv.Y}
		}
	}

	var box image.Rectangle
	useBox := *region != ""
	var rx [4]float64
	if useBox {
		parts := strings.Split(*region, ",")
		if len(parts) != 4 {
			fmt.Fprintln(os.Stderr, "director: --region wants x0,y0,x1,y1")
			return 2
		}
		for i, p := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "director: --region: %v\n", err)
				return 2
			}
			rx[i] = v
		}
	}

	if *limit > 0 && *limit < len(frames) {
		frames = frames[:*limit]
	}
	fmt.Printf("%d frames, annotation: %s\n", len(frames), ann.Provenance)
	if ann.Provenance == "" {
		fmt.Printf("WARNING: the annotation states no provenance.\n")
	}
	fmt.Println()

	reader := selection.New()
	segSize := (len(frames) + *segments - 1) / *segments
	segs := make([]tally, *segments)
	var rowsSeen []int
	var worst []string

	for n, fr := range frames {
		img, err := loadPNG(filepath.Join(*dir, filepath.FromSlash(fr.File)))
		if err != nil {
			fmt.Fprintf(os.Stderr, "director: frame %d: %v\n", fr.Index, err)
			return 1
		}
		b := img.Bounds()
		if useBox {
			box = image.Rect(
				b.Min.X+int(rx[0]*float64(b.Dx())), b.Min.Y+int(rx[1]*float64(b.Dy())),
				b.Min.X+int(rx[2]*float64(b.Dx())), b.Min.Y+int(rx[3]*float64(b.Dy())))
		} else {
			box = b
		}
		var rows []selection.Row
		if *givenRows != "" {
			rows = suppliedRows(*givenRows, *rowHeight, b, box)
		} else {
			rows = selection.FindRows(img, box)
		}
		rowsSeen = append(rowsSeen, len(rows))
		res := reader.Read(img, rows)

		t, ok := truth[fr.Index]
		if !ok {
			continue
		}
		seg := &segs[min(n/segSize, *segments-1)]
		switch {
		case t.state == "uncertain":
			seg.truthUncertain++
			if res.Outcome == selection.Selected {
				seg.confidentOnTransition++
			} else {
				seg.correctlyUnsure++
			}
		default:
			seg.truthSelected++
			switch res.Outcome {
			case selection.Selected:
				centre := float64(rows[res.Row].Bounds.Min.Y+rows[res.Row].Bounds.Max.Y) /
					2 / float64(b.Dy())
				if absf(centre-t.y) <= 0.028 {
					seg.correct++
				} else {
					seg.confidentlyWrong++
					if len(worst) < 12 {
						worst = append(worst, fmt.Sprintf(
							"frame %d: said y=%.3f, annotated y=%.3f (%s)",
							fr.Index, centre, t.y, res.Why))
					}
				}
			case selection.NoneSelected:
				seg.noneSelected++
			default:
				seg.declined++
			}
		}
	}

	sort.Ints(rowsSeen)
	fmt.Printf("CANDIDATE ROWS PER FRAME  min %d  median %d  max %d\n\n",
		rowsSeen[0], rowsSeen[len(rowsSeen)/2], rowsSeen[len(rowsSeen)-1])

	var all tally
	fmt.Printf("  %-10s %9s %9s %9s %9s %9s %9s\n", "SEGMENT",
		"SEL", "CORRECT", "WRONG", "DECLINED", "TRANS", "TRANS-OK")
	for i, s := range segs {
		all.add(s)
		fmt.Printf("  %-10d %9d %9d %9d %9d %9d %9d\n", i+1,
			s.truthSelected, s.correct, s.confidentlyWrong, s.declined,
			s.truthUncertain, s.correctlyUnsure)
	}
	fmt.Printf("  %-10s %9d %9d %9d %9d %9d %9d\n\n", "all",
		all.truthSelected, all.correct, all.confidentlyWrong, all.declined,
		all.truthUncertain, all.correctlyUnsure)

	if all.truthSelected > 0 {
		fmt.Printf("On annotated selections: %.1f%% correct, %.1f%% CONFIDENTLY WRONG, %.1f%% declined\n",
			pct(all.correct, all.truthSelected), pct(all.confidentlyWrong, all.truthSelected),
			pct(all.declined, all.truthSelected))
	}
	if all.truthUncertain > 0 {
		fmt.Printf("On annotated transitions: %.1f%% correctly unsure, %d claimed a selection\n",
			pct(all.correctlyUnsure, all.truthUncertain), all.confidentOnTransition)
	}
	// Said plainly rather than left to be inferred from a zero.
	fmt.Printf("NoneSelected: emitted %d times. This session annotates NO none-selected frames,\n"+
		"so that path is UNVALIDATED here whatever the count.\n", all.noneSelected)

	if len(worst) > 0 {
		fmt.Printf("\nCONFIDENT WRONG SELECTIONS (first %d)\n", len(worst))
		for _, w := range worst {
			fmt.Printf("  %s\n", w)
		}
	}
	return 0
}

// suppliedRows builds rows from normalised centres the operator typed.
//
// Harness input. Nothing in the reader, and nothing in production, may know these numbers;
// they exist so a failure to FIND rows can be told apart from a failure to read them.
func suppliedRows(spec string, height float64, b, box image.Rectangle) []selection.Row {
	var out []selection.Row
	for g, group := range strings.Split(spec, "|") {
		for _, s := range strings.Split(group, ",") {
			c, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				continue
			}
			top := b.Min.Y + int((c-height/2)*float64(b.Dy()))
			bot := b.Min.Y + int((c+height/2)*float64(b.Dy()))
			out = append(out, selection.Row{
				Bounds: image.Rect(box.Min.X, top, box.Max.X, bot),
				Group:  g,
			})
		}
	}
	return out
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) * 100 / float64(b)
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func readFrameIndex(path string) ([]frameLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []frameLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var fl frameLine
		if err := json.Unmarshal(sc.Bytes(), &fl); err != nil {
			return nil, err
		}
		out = append(out, fl)
	}
	return out, sc.Err()
}

func readAnnotations(path string) (annotation, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return annotation{}, err
	}
	var a annotation
	if err := json.Unmarshal(b, &a); err != nil {
		return annotation{}, fmt.Errorf("%s: %w", path, err)
	}
	if len(a.Intervals) == 0 {
		return annotation{}, fmt.Errorf("%s: no intervals", path)
	}
	return a, nil
}
