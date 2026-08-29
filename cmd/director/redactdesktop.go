package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// PRIVACY IS A STEP, NOT A HABIT.
//
// The 37C corpus is captured from a real desktop, and a real desktop has the person on it.
// The two wide Settings frames carry an account header — a name and an email address, drawn
// on screen and repeated as three element labels. The Explorer sample carries the person's
// OneDrive and phone in its navigation tree. None of that may be committed.
//
// The temptation is to fix it by hand: paint over two rectangles, delete three strings, move
// on. That produces a clean corpus once. It does not produce a corpus that STAYS clean when
// the next capture is added, and it leaves nothing for a reviewer to check.
//
// So redaction is a command with a recorded result, and the corpus carries a test
// (TestTheCommittedCorpusCarriesNoPersonalInformation) that fails on the artifacts rather
// than trusting that the step was run.
//
// Two rules, and the second is the one worth stating:
//
//   - Redaction is GEOMETRIC. A region is blacked out in the picture, and every element whose
//     box lies inside it loses its label. Nothing here matches on the person's name, because
//     a scrubber that knows the name has to be given the name, and source carrying the name
//     is the leak it was written to prevent.
//
//   - Elements outside the frame are DROPPED, and counted. This is a privacy fix and a
//     measurement fix at the same time. Accessibility reports the whole virtualised tree,
//     including a nav pane scrolled far off screen — that is where the personal items were,
//     at y = -1.95 of the window height, present in the reading and absent from the picture.
//     A pixel detector cannot see them, so scoring ScreenParser against them would charge it
//     for missing what was never on screen. Keeping the count keeps the finding; dropping the
//     labels keeps the promise.

// redaction is one blacked-out rectangle, as a proportion of the window frame.
type redaction struct {
	Region observe.Region `json:"region"`
	Reason string         `json:"reason"`
	// Labels is how many element labels this region cleared, so a reviewer can tell a region
	// that did something from one that missed.
	Labels int `json:"labels_cleared"`
}

// emailish is the one pattern the test asserts on, because an email address has a shape and a
// person's name does not. It is a check on the RESULT of geometric redaction, not the
// mechanism: if this ever matches, the geometry was wrong.
var emailish = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// runRedactDesktopSample is `director redact-desktop-sample --dir <sample> --redact x,y,w,h`.
//
// Regions are given in PIXELS of the captured frame, because a person reads them off the
// picture. They are stored as proportions, like every other box in the sample.
func runRedactDesktopSample(args []string) int {
	fs := flag.NewFlagSet("redact-desktop-sample", flag.ExitOnError)
	dir := fs.String("dir", "", "the sample directory to redact, in place")
	reason := fs.String("reason", "personal information", "why, recorded in the sample")
	var regions multiRegion
	fs.Var(&regions, "redact", "black this out: x,y,w,h in pixels (repeatable)")
	keepOffscreen := fs.Bool("keep-offscreen", false,
		"keep elements outside the frame (they are dropped and counted by default)")
	_ = fs.Parse(flagsFirst(args))

	if *dir == "" {
		fmt.Fprintln(os.Stderr,
			"director: --dir is required\n"+
				"  example: director redact-desktop-sample "+
				"--dir .tmp/desktop-corpus-review/settings-mouse-wide --redact 13,50,290,76")
		return 2
	}

	path := filepath.Join(*dir, "production.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	var s desktopSample
	if err := json.Unmarshal(raw, &s); err != nil {
		fmt.Fprintf(os.Stderr, "director: %s: %v\n", path, err)
		return 1
	}

	// The regions arrive in pixels and this is the only place that knows the frame size.
	proportional := make([]observe.Region, 0, len(regions))
	for _, r := range regions {
		if s.Width <= 0 || s.Height <= 0 {
			fmt.Fprintf(os.Stderr, "director: %s has no frame size to scale a region by\n", s.ID)
			return 1
		}
		proportional = append(proportional, observe.Region{
			X:      r.X / float64(s.Width),
			Y:      r.Y / float64(s.Height),
			Width:  r.Width / float64(s.Width),
			Height: r.Height / float64(s.Height),
		})
	}

	res, err := redactSample(&s, proportional, *reason, *keepOffscreen,
		filepath.Join(*dir, s.ID+".png"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	out, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	fmt.Printf("%s: %d region(s), %d label(s) cleared, %d element(s) dropped as offscreen\n",
		s.ID, len(proportional), res.labels, res.dropped)
	if leaks := scanForEmails(&s); len(leaks) > 0 {
		// Reported, never silently accepted. A redaction pass that leaves an address behind
		// has told us its geometry is wrong, which is worth more than a clean exit code.
		fmt.Fprintf(os.Stderr, "director: %d label(s) still look like an email address: %s\n",
			len(leaks), strings.Join(leaks, ", "))
		return 1
	}
	return 0
}

type redactResult struct{ labels, dropped int }

// redactSample applies the geometry to the reading and to the picture.
//
// The picture is only rewritten if it exists — a sample whose PNG was already removed can
// still have its reading cleaned, which is what makes this safe to run twice.
func redactSample(s *desktopSample, regions []observe.Region, reason string,
	keepOffscreen bool, pngPath string) (redactResult, error) {

	var res redactResult

	kept := s.Elements[:0:0]
	for _, e := range s.Elements {
		if !keepOffscreen && !withinFrame(e.Bounds) {
			res.dropped++
			continue
		}
		kept = append(kept, e)
	}
	s.Elements = kept

	for i := range regions {
		n := 0
		for j := range s.Elements {
			if s.Elements[j].Label != "" && insideRegion(s.Elements[j].Bounds, regions[i]) {
				s.Elements[j].Label = ""
				n++
			}
		}
		res.labels += n
		s.Redactions = append(s.Redactions, redaction{
			Region: regions[i], Reason: reason, Labels: n,
		})
	}
	s.OffscreenDropped += res.dropped

	// GEOMETRY FIRST, then the picture, so a half-applied pass leaves the reading cleaner
	// than the frame rather than the other way round.
	if len(regions) > 0 {
		if err := blackOut(pngPath, regions); err != nil {
			return res, err
		}
	}
	return res, nil
}

// withinFrame is whether any part of the box is inside the window rectangle.
//
// Not "is the top-left inside": a list item half below the fold is genuinely on screen, and a
// detector can genuinely see the visible half.
func withinFrame(b observe.Region) bool {
	return b.X < 1 && b.Y < 1 && b.X+b.Width > 0 && b.Y+b.Height > 0
}

// insideRegion is containment of the element's CENTRE, not overlap.
//
// Overlap would clear the label of the window itself, whose box is the whole frame and which
// therefore intersects every region — and the window's title is the sample's identity.
func insideRegion(b, r observe.Region) bool {
	cx, cy := b.X+b.Width/2, b.Y+b.Height/2
	return cx >= r.X && cx <= r.X+r.Width && cy >= r.Y && cy <= r.Y+r.Height
}

// blackOut paints the regions opaque and rewrites the frame.
func blackOut(path string, regions []observe.Region) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	src, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, src, b.Min, draw.Src)
	for _, r := range regions {
		px := image.Rect(
			b.Min.X+int(r.X*float64(b.Dx())),
			b.Min.Y+int(r.Y*float64(b.Dy())),
			b.Min.X+int((r.X+r.Width)*float64(b.Dx())),
			b.Min.Y+int((r.Y+r.Height)*float64(b.Dy())),
		)
		draw.Draw(dst, px.Intersect(b), image.NewUniform(color.Black), image.Point{}, draw.Src)
	}

	tmp := path + ".redacting"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := png.Encode(out, dst); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// scanForEmails reports labels that still look like an address.
func scanForEmails(s *desktopSample) []string {
	var out []string
	for _, e := range s.Elements {
		if emailish.MatchString(e.Label) {
			out = append(out, e.ID)
		}
	}
	return out
}

// multiRegion collects repeated --redact flags, in pixels of the captured frame.
type multiRegion []observe.Region

func (m *multiRegion) String() string { return fmt.Sprintf("%d region(s)", len(*m)) }

func (m *multiRegion) Set(v string) error {
	parts := strings.Split(v, ",")
	if len(parts) != 4 {
		return fmt.Errorf("want x,y,w,h in pixels, got %q", v)
	}
	var n [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return fmt.Errorf("%q is not a number", p)
		}
		n[i] = f
	}
	*m = append(*m, observe.Region{X: n[0], Y: n[1], Width: n[2], Height: n[3]})
	return nil
}
