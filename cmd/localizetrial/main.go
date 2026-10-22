// Command localizetrial runs the local-regioning experiment and prints its result.
//
// One question, answered on a fixture somebody else can reproduce exactly:
//
//	Does local regioning distinguish the demonstrated control instance despite unrelated
//	screen noise?
//
//	localizetrial                      the table, the refusal reasons and the verdict
//	localizetrial -evidence <dir>      and the representative images, as PNGs
//
// It opens no window, takes no foreground, sends no input and reads nobody's store. Every case
// is an image rendered from a seed; a selection is a point computed from one. See
// internal/director/localizetrial for the protocol, the frozen parameters and where each of
// them came from.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/chaynes-simpleclouds/marco/internal/director/localizetrial"
)

func main() {
	real := flag.String("real", "",
		"a PNG of a real window a control was demonstrated in; with -then and "+
			"-click, locates it in the second image and prints what both "+
			"methods had to choose between")
	then := flag.String("then", "", "the PNG to locate the control in")
	click := flag.String("click", "", "where the control was clicked in -real, as x,y")
	region := flag.String("region", "",
		"the local region to keep around the click, as wxh. The frozen one is "+
			"sized for the fixture's own control; a real capture at another "+
			"scale needs the region the control actually has, and saying so "+
			"explicitly is better than a number that silently means something "+
			"else")
	calib := flag.String("calibrate", "",
		"a directory of PNG frames of one window; with -click and -region, "+
			"measures what the demonstrated control scores against itself "+
			"across them AND against the best thing near it, which is the "+
			"only honest way to place a threshold")
	by := flag.Int("downscale", 1,
		"measure the same control at 1/N of the capture resolution, which is "+
			"what a preview keeps; 1 leaves the frames alone")
	band := flag.Int("band", 160,
		"how far from the click to look for something that is not the control")
	evidence := flag.String("evidence", "",
		"a directory to write the representative images into; nothing is "+
			"written without one")
	flag.Parse()

	if *calib != "" {
		if err := calibrate(*calib, mustPoint(*click, "-click"),
			mustSize(*region, "-region"), *band, *by); err != nil {
			fmt.Fprintln(os.Stderr, "localizetrial:", err)
			os.Exit(1)
		}
		return
	}
	if *real != "" {
		var x, y int
		if _, err := fmt.Sscanf(*click, "%d,%d", &x, &y); err != nil {
			fmt.Fprintln(os.Stderr, "localizetrial: -real needs -click x,y")
			os.Exit(2)
		}
		keep := localizetrial.Frozen.Region
		if *region != "" {
			var w, h int
			if _, err := fmt.Sscanf(*region, "%dx%d", &w, &h); err != nil {
				fmt.Fprintln(os.Stderr, "localizetrial: -region wxh")
				os.Exit(2)
			}
			keep = image.Pt(w, h)
		}
		if err := probeReal(*real, *then, image.Pt(x, y), keep); err != nil {
			fmt.Fprintln(os.Stderr, "localizetrial:", err)
			os.Exit(1)
		}
		return
	}

	results := localizetrial.Evaluate(localizetrial.Cases(), localizetrial.Frozen)
	fmt.Print(localizetrial.Report(results))
	fmt.Println()
	fmt.Print(localizetrial.Reasons(results))
	fmt.Println()

	for _, m := range localizetrial.Methods() {
		right, of := localizetrial.Supported(results, m.Name)
		refused, unsupported := localizetrial.Unsupported(results, m.Name)
		fmt.Printf("%-14s %d/%d supported selected correctly, %d/%d "+
			"unsupported refused, %d wrong-instance selections\n",
			m.Name, right, of, refused, unsupported,
			localizetrial.WrongInstances(results, m.Name))
	}

	if *evidence == "" {
		return
	}
	if err := writeEvidence(*evidence); err != nil {
		fmt.Fprintln(os.Stderr, "localizetrial:", err)
		os.Exit(1)
	}
	fmt.Printf("\nevidence written to %s\n", *evidence)
}

// writeEvidence draws one image per kind of case, with what each method chose marked on it.
//
// The marks are drawn on a COPY. Nothing the methods were scored against is touched, so the
// pixels in these files are the pixels they were given plus an annotation layer.
func writeEvidence(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	demo, targets := localizetrial.Demonstration()
	target := targets[localizetrial.LeftApply]

	// THE DEMONSTRATION: the click, and the region kept around it.
	shot := copyOf(demo.Client)
	box(shot, target.Region(), color.RGBA{0x6f, 0x9b, 0xff, 0xff})
	cross(shot, target.At, color.RGBA{0x6f, 0x9b, 0xff, 0xff})
	if err := save(filepath.Join(dir, "00-demonstrated.png"), shot); err != nil {
		return err
	}

	for i, c := range pick() {
		look := localizetrial.Render(c.Scene)
		shot := copyOf(look.Client)
		// WHERE THE CONTROL REALLY IS, from the fixture's own truth.
		if r, ok := look.Where[c.Want.String()]; ok {
			box(shot, r, color.RGBA{0x4c, 0xaf, 0x62, 0xff})
		}
		for _, m := range localizetrial.Methods() {
			v := m.Run(target, look, localizetrial.Frozen)
			if !v.Found {
				continue
			}
			c := color.RGBA{0xe0, 0x5a, 0x4a, 0xff} // whole-window
			if m.Name == "local region" {
				c = color.RGBA{0x6f, 0x9b, 0xff, 0xff}
			}
			cross(shot, v.Chose, c)
		}
		name := fmt.Sprintf("%02d-%s.png", i+1, slug(string(c.Kind)))
		if err := save(filepath.Join(dir, name), shot); err != nil {
			return err
		}
	}
	return nil
}

// pick is one case of each kind, by the first seed of each.
func pick() []localizetrial.Case {
	seen := map[localizetrial.Kind]bool{}
	var out []localizetrial.Case
	for _, c := range localizetrial.Cases() {
		if seen[c.Kind] {
			continue
		}
		seen[c.Kind] = true
		out = append(out, c)
	}
	return out
}

func copyOf(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	return out
}

func box(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for t := 0; t < 2; t++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			set(img, x, r.Min.Y+t, c)
			set(img, x, r.Max.Y-1-t, c)
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			set(img, r.Min.X+t, y, c)
			set(img, r.Max.X-1-t, y, c)
		}
	}
}

// cross marks a chosen point. Three pixels thick, because these images are read at the size
// they were rendered and a one-pixel mark on a dark button is not evidence anybody can see.
func cross(img *image.RGBA, at image.Point, c color.RGBA) {
	for d := -9; d <= 9; d++ {
		for w := -1; w <= 1; w++ {
			set(img, at.X+d, at.Y+w, c)
			set(img, at.X+w, at.Y+d, c)
		}
	}
}

func set(img *image.RGBA, x, y int, c color.RGBA) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.SetRGBA(x, y, c)
	}
}

func save(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func slug(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+32)
		case len(out) > 0 && out[len(out)-1] != '-':
			out = append(out, '-')
		}
	}
	return string(out)
}
