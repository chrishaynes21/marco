package main

// WHAT THE PROPOSER SAW ON ONE FRAME, DRAWN.
//
// [[ADR-182-two-controls-that-could-never-be-found-again]] established that `target_2` fails
// upstream of matching: the TRAINING tile is never offered as a candidate at all. The owner drew
// the consequence:
//
//	Marco didn't fail to recognize the TRAINING tile. It never detected the tile as a candidate
//	control in the first place. […] I'd investigate a second candidate-generation path for large,
//	visually bounded controls, rather than loosening `sameRect` or replacing the existing
//	proposer.
//
// Before any of that, the miss has to be explained rather than assumed. This renders one frame
// with every proposal outlined and lists the ones near a rectangle of interest, so "the proposer
// does not see tiles" is something a reader can check against the pixels instead of a story that
// fits the numbers.
//
// Decides nothing, proposes nothing, changes nothing.

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// reportProposals draws and lists what the production proposer found on one frame.
//
// `near` is an optional "x,y,w,h" in frame pixels; candidates overlapping it are listed with how
// much of it they cover, which is the question "was this control proposed" asked precisely.
func reportProposals(dir string, look int, near string, out string) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if look < 1 || look > len(frames) {
		return fmt.Errorf("look %d is not in a recording of %d frames", look, len(frames))
	}
	fr := frames[look-1]
	img, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
	if err != nil {
		return err
	}
	res := structure.Propose(img, structure.DefaultOptions())
	b := img.Bounds()

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("WHAT THE PROPOSER FOUND ON LOOK %d OF %s\n", look, filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))
	fmt.Printf("  %dx%d px, %d candidate(s)\n", b.Dx(), b.Dy(), len(res.Candidates))

	byKind := map[structure.Kind]int{}
	var biggest []structure.Candidate
	for _, c := range res.Candidates {
		byKind[c.Kind]++
		biggest = append(biggest, c)
	}
	for _, k := range []structure.Kind{structure.Block, structure.Line} {
		fmt.Printf("    %-6s %d\n", k, byKind[k])
	}
	sort.Slice(biggest, func(i, j int) bool {
		return areaOf(biggest[i].Bounds) > areaOf(biggest[j].Bounds)
	})
	fmt.Printf("  the ten largest, because a tile would be among them if it were here:\n")
	for i, c := range biggest {
		if i >= 10 {
			break
		}
		fmt.Printf("    %s\n", describeCandidate(c))
	}

	var want image.Rectangle
	if near != "" {
		if want, err = parseRect(near); err != nil {
			return err
		}
		fmt.Printf("\n  candidates overlapping %v (%dx%d px):\n",
			want.Min, want.Dx(), want.Dy())
		n := 0
		for _, c := range res.Candidates {
			in := c.Bounds.Intersect(want)
			if in.Empty() {
				continue
			}
			n++
			fmt.Printf("    %s  covers %.0f%% of it, %.0f%% of itself\n",
				describeCandidate(c),
				100*float64(areaOf(in))/float64(areaOf(want)),
				100*float64(areaOf(in))/float64(areaOf(c.Bounds)))
		}
		if n == 0 {
			fmt.Printf("    NONE. Nothing the proposer found touches it at all.\n")
		}
	}

	if out != "" {
		dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		for _, c := range res.Candidates {
			col := color.RGBA{0, 200, 255, 255} // block
			if c.Kind == structure.Line {
				col = color.RGBA{255, 220, 0, 255}
			}
			outline(dst, c.Bounds.Sub(b.Min), col)
		}
		if !want.Empty() {
			outline(dst, want.Sub(b.Min), color.RGBA{255, 40, 40, 255})
		}
		name := filepath.Join(out, fmt.Sprintf("proposals-look%d.png", look))
		if err := writePNG(name, dst); err != nil {
			return err
		}
		fmt.Printf("\n  drawn to %s — blocks in cyan, lines in yellow\n", name)
	}
	return nil
}

func areaOf(r image.Rectangle) int { return r.Dx() * r.Dy() }

func describeCandidate(c structure.Candidate) string {
	return fmt.Sprintf("%-5s %4d,%4d %4dx%-4d fill %.2f contrast %.2f parts %d",
		c.Kind, c.Bounds.Min.X, c.Bounds.Min.Y, c.Bounds.Dx(), c.Bounds.Dy(),
		c.Fill, c.Contrast, c.Parts)
}

// parseRect reads "x,y,w,h" in frame pixels.
func parseRect(s string) (image.Rectangle, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return image.Rectangle{}, fmt.Errorf("want x,y,w,h, got %q", s)
	}
	var v [4]int
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return image.Rectangle{}, fmt.Errorf("%q is not a number", p)
		}
		v[i] = n
	}
	return image.Rect(v[0], v[1], v[0]+v[2], v[1]+v[3]), nil
}
