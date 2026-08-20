package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// runVision is `director vision [--region] [--last]`.
//
// A fresh pass CAPTURES THE SCREEN, which is why it is a command a person runs. `--last`
// reads the previous pass instead and captures nothing.
func runVision(args []string) int {
	fs := flag.NewFlagSet("vision", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	last := fs.Bool("last", false, "show the previous pass without taking a new one")
	region := fs.String("region", "", "narrow to x,y,w,h in desktop coordinates")
	windowID := fs.String("window-id", "", "target the window with this ephemeral id (see `director windows`)")
	title := fs.String("window-title", "", "target the window whose title contains this")
	application := fs.String("application", "", "target this application's window")
	processID := fs.Int("process", 0, "target this process's window")
	_ = fs.Parse(flagsFirst(args))

	p := service.VisionPayload{Last: *last, Target: windowref.Selector{
		EphemeralID: *windowID, Title: *title,
		Application: *application, ProcessID: uint32(*processID),
	}}
	if !p.Target.Zero() {
		if err := p.Target.Validate(); err != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", err)
			return 2
		}
	}
	if *region != "" {
		r, rerr := parseRegion(*region)
		if rerr != nil {
			fmt.Fprintf(os.Stderr, "director: %v\n", rerr)
			return 2
		}
		p.Region = &r
	}

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.Vision(p)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(out.Diagnostics)
	}
	fmt.Print(renderVision(out.Diagnostics))
	return 0
}

// runFrames is `director frames`.
func runFrames(args []string) int {
	fs := flag.NewFlagSet("frames", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.Vision(service.VisionPayload{Frames: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(out.Frames)
	}
	fmt.Print(renderFrames(out.Frames))
	return 0
}

// runExplainVision is `director explain vision` — the last pass, without taking one.
func runExplainVision(args []string) int {
	return runVision(append([]string{"--last"}, args...))
}
