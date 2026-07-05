package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runVision passes `marco vision <args…>` straight through to the vision resolver plugin
// (e.g. `marco vision detect <screenshot.png>` runs the debug spike). The plugin owns the
// ONNX detector, so the zero-dep engine just locates and execs it, sharing stdio. It finds
// the binary via $MARCO_VISION (the same var newDeps wires the host from), else the
// conventional plugins/vision/vision.exe beside the working dir.
func runVision(args []string) {
	bin := strings.TrimSpace(os.Getenv("MARCO_VISION"))
	if bin == "" {
		bin = filepath.Join("plugins", "vision", "vision.exe")
	}
	if _, err := os.Stat(bin); err != nil {
		fmt.Fprintf(os.Stderr, "vision plugin not found at %q — set $MARCO_VISION, or build it: go -C plugins/vision build -o vision.exe .\n", bin)
		os.Exit(1)
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "vision:", err)
		os.Exit(1)
	}
}
