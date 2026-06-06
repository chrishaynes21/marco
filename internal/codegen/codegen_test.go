package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

var sample = []macroir.Step{
	{Kind: macroir.StepClick, X: 100, Y: 200, Button: "left"},
	{Kind: macroir.StepWait, Ms: 350},
	{Kind: macroir.StepClick, X: 400, Y: 500, Button: "left"},
	{Kind: macroir.StepKey, Key: "e", Count: 3},
	{Kind: macroir.StepType, Text: `say "hi"`},
}

func TestRouteStructure(t *testing.T) {
	src, err := Route("open chest", sample)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"the OpenChest is an actor.",
		"the p1 is a Point with X 100, Y 200.",
		"the p2 is a Point with X 400, Y 500.",
		"do OS's Click with p1.",
		"do OS's Sleep with 350.",
		"do OS's Click with p2.",
		"repeat 3 times...",
		`do OS's Key with "e".`,
		`do OS's Type with "say \"hi\"".`,
		"do OpenChest's Run...",
		`log "open chest: done".`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated route missing %q\n--- source ---\n%s", want, src)
		}
	}
}

// TestGeneratedRoutesCompileAndRun feeds codegen output back through the driver
// (under the dryrun host) to guarantee every generated route is valid Marco.
func TestGeneratedRoutesCompileAndRun(t *testing.T) {
	cases := map[string][]macroir.Step{
		"open chest": sample,
		"login": {
			{Kind: macroir.StepClick, X: 10, Y: 20, Button: "left"},
			{Kind: macroir.StepType, Text: "user"},
			{Kind: macroir.StepKey, Key: "tab"},
			{Kind: macroir.StepType, Text: "pass"},
			{Kind: macroir.StepKey, Key: "enter"},
		},
		"cycle": {
			{Kind: macroir.StepLoop, Count: 4, Steps: []macroir.Step{
				{Kind: macroir.StepClick, X: 5, Y: 5, Button: "left"},
				{Kind: macroir.StepWait, Ms: 50},
			}},
		},
		"right click": {
			{Kind: macroir.StepClick, X: 0, Y: 0, Button: "right"},
		},
	}
	osSrc, err := os.ReadFile(filepath.Join("..", "..", "programs", "os.marco"))
	if err != nil {
		t.Fatalf("read os.marco: %v", err)
	}
	for name, steps := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
				t.Fatal(err)
			}
			src, err := Route(name, steps)
			if err != nil {
				t.Fatal(err)
			}
			routePath := filepath.Join(dir, "route.marco")
			if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			// Static check.
			if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
				t.Fatalf("generated route failed to compile: %v\n--- source ---\n%s", err, src)
			}
			// Run under the dryrun host; must succeed and log the done line.
			var out bytes.Buffer
			if err := driver.RunFileWithHosts(routePath, &out, nil); err != nil {
				t.Fatalf("generated route failed to run: %v\n--- source ---\n%s", err, src)
			}
			if !strings.Contains(out.String(), name+": done") {
				t.Fatalf("route %q did not complete; output:\n%s", name, out.String())
			}
		})
	}
}
