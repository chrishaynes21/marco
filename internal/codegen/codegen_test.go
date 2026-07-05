package codegen

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// tinyPNG returns the bytes of a small checkerboard PNG, standing in for a
// captured click target.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			if (x+y)%2 == 0 {
				img.Set(x, y, color.RGBA{255, 0, 0, 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestImageClickRoute: a click carrying a captured Template becomes an image
// find with a coordinate fallback; the template is returned as an asset, the
// route compiles, and it runs to completion under the dryrun host.
func TestImageClickRoute(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 640, Y: 480, Button: "left", Template: tinyPNG(t)},
		{Kind: macroir.StepType, Text: "hi"},
	}
	src, assets, err := Route("open menu", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"the Anchor is a set.",
		"do OS's Find with a1...",  // scored resolve
		"when ok?",                 // confident → click the scored location...
		"do OS's Click with that.", //   ...which a static screen resolves to the recorded point
		"or?",                      // unsure →
		"do OS's Click with p1.",   //   ...fall back to the recorded coordinate
		"the p1 is a Point with X 640, Y 480.",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("image-click route missing %q\n--- source ---\n%s", want, src)
		}
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 template asset, got %d", len(assets))
	}
	for path, data := range assets {
		if !strings.HasSuffix(path, "open-menu-anchor-1.png") {
			t.Errorf("unexpected asset name %q", path)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("image-click route failed to compile: %v\n--- source ---\n%s", err, src)
	}
	var out bytes.Buffer
	if err := driver.RunFileWithHosts(routePath, &out, nil); err != nil {
		t.Fatalf("image-click route failed to run: %v\n--- source ---\n%s", err, src)
	}
	if !strings.Contains(out.String(), "open menu: done") {
		t.Fatalf("route did not complete; output:\n%s", out.String())
	}
}

// TestImageClickRightButton: a non-left anchored click moves to the located image
// (OS's Click with a Point is always a left click) then clicks that button — in both
// the found and the coordinate-fallback arms. The route compiles and runs.
func TestImageClickRightButton(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 640, Y: 480, Button: "right", Template: tinyPNG(t)},
	}
	src, assets, err := Route("ctx menu", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"do OS's Find with a1...",     // scored resolve
		`do OS's Click with "right".`, // right-click...
		"do OS's Move with that.",     // confident → at the scored location
		"do OS's Move with p1.",       // unsure → at the recorded coordinate
	} {
		if !strings.Contains(src, want) {
			t.Errorf("right-button image-click missing %q\n--- source ---\n%s", want, src)
		}
	}
	for path, data := range assets {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("right-button image-click failed to compile: %v\n--- source ---\n%s", err, src)
	}
	if err := driver.RunFileWithHosts(routePath, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("right-button image-click failed to run: %v\n--- source ---\n%s", err, src)
	}
}

// TestImageClickColorResolver: a click that captured a pixel colour emits it as the
// anchor's secondary resolver — Color + the recorded X,Y on the Anchor, plus the
// Color field on the Anchor type. The recorded coordinate is still what gets clicked.
func TestImageClickColorResolver(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 640, Y: 480, Button: "left", Template: tinyPNG(t), Color: "0x1A2B3C"},
	}
	src, assets, err := Route("open menu", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"this's Color is a text.",        // Anchor type carries the resolver
		`with Image "`,                   // image resolver still emitted
		`Color "0x1A2B3C", X 640, Y 480`, // colour resolver on the instance
		"do OS's Click with p1.",         // still clicks the recorded coordinate
	} {
		if !strings.Contains(src, want) {
			t.Errorf("colour-resolver route missing %q\n--- source ---\n%s", want, src)
		}
	}
	// The colour-bearing Anchor literal must actually parse and run (a malformed set
	// would only surface here, not in the source assertions above).
	for path, data := range assets {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("colour-resolver route failed to compile: %v\n--- source ---\n%s", err, src)
	}
	if err := driver.RunFileWithHosts(routePath, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("colour-resolver route failed to run: %v\n--- source ---\n%s", err, src)
	}
}

// TestImageClickWindowContext: a click that captured a window title emits it as the
// anchor's Window context — the Window field on the Anchor type and the literal — so the
// scorer can refuse a confident click in the wrong window of a multi-window app.
func TestImageClickWindowContext(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 640, Y: 480, Button: "left", Template: tinyPNG(t), Window: "Friends List"},
	}
	src, assets, err := Route("open chat", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"this's Window is a text.", // Anchor type carries the context field
		`Window "Friends List"`,    // the context on the instance
	} {
		if !strings.Contains(src, want) {
			t.Errorf("window-context route missing %q\n--- source ---\n%s", want, src)
		}
	}
	// The Window-bearing Anchor literal must parse and compile.
	for path, data := range assets {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("window-context route failed to compile: %v\n--- source ---\n%s", err, src)
	}
}

// TestImageClickNoColor: without a captured colour, the anchor carries no Color field
// (the colour signal is opt-in). X,Y are always emitted now — they feed the position
// signal — but Color isn't.
func TestImageClickNoColor(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 640, Y: 480, Button: "left", Template: tinyPNG(t)},
	}
	src, _, err := Route("open menu", "", steps, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(src, ", Color \"") {
		t.Errorf("anchor without a captured colour should not emit a Color field:\n%s", src)
	}
}

// TestTextOnlyClickRoute: a narrated "click the text Mute" (no template) emits a
// Text (OCR) locator that clicks where the word IS, falling back to the recorded
// coordinate. It imports `use text.`, compiles, and runs (under dryrun the Text host
// resolves ok so the located-click arm runs).
func TestTextOnlyClickRoute(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 100, Y: 200, Button: "left", AnchorText: "Mute"},
	}
	src, _, err := Route("mute discord", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"use text.",                            // the OCR resolver surface is imported
		"this's Text is a text.",               // the Anchor type carries the locator
		`the t1 is an Anchor with Text "Mute"`, // text anchor declared
		"do Text's Find with t1...",            // OCR locate
		"do OS's Click with that.",             // click where the text IS
		"do OS's Click with p1.",               // ...else the recorded coordinate
	} {
		if !strings.Contains(src, want) {
			t.Errorf("text-only route missing %q\n--- source ---\n%s", want, src)
		}
	}
	// A narrated text-only anchor has no recorded coordinate, so it searches the WHOLE
	// screen — no region box on the anchor (the `, X1 ` field, distinct from the type decl).
	if strings.Contains(src, ", X1 ") {
		t.Errorf("text-only anchor should not restrict the search region:\n%s", src)
	}
	// No image gate for a text-only anchor.
	if strings.Contains(src, "do OS's Find with a1") {
		t.Errorf("text-only anchor should not emit an OS image gate:\n%s", src)
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("text-only route failed to compile: %v\n--- source ---\n%s", err, src)
	}
	if err := driver.RunFileWithHosts(routePath, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("text-only route failed to run: %v\n--- source ---\n%s", err, src)
	}
}

// TestGateAndTextClickRoute: an anchor with BOTH a template and text emits the image
// gate first; only when the gate fails does it fall through to the OCR locator (the
// target moved). Both compile and the route runs.
func TestGateAndTextClickRoute(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 640, Y: 480, Button: "left", Template: tinyPNG(t), Color: "0x1A2B3C", AnchorText: "Play"},
	}
	src, assets, err := Route("start game", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"do OS's Find with a1...", // image/colour gate first
		`Color "0x1A2B3C"`,        // colour gate present
		`the t1 is an Anchor with Text "Play"`,
		// A demonstrated anchor restricts the OCR search to a box around the recorded
		// click (640,480 ± TextSearchMargin 400) instead of the whole screen.
		"X1 240, Y1 80, X2 1040, Y2 880",
		"do Text's Find with t1...", // text locator as the gate's fallback
	} {
		if !strings.Contains(src, want) {
			t.Errorf("gate+text route missing %q\n--- source ---\n%s", want, src)
		}
	}
	for path, data := range assets {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("gate+text route failed to compile: %v\n--- source ---\n%s", err, src)
	}
	if err := driver.RunFileWithHosts(routePath, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("gate+text route failed to run: %v\n--- source ---\n%s", err, src)
	}
}

// TestGateTextVisionClickRoute: an anchor with an image gate AND a text label AND a vision
// class emits the full resolver chain — gate, then Text's Find, then Vision's Locate, then
// the recorded coordinate — and compiles + runs.
func TestGateTextVisionClickRoute(t *testing.T) {
	dir := t.TempDir()
	osSrc, _ := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 640, Y: 480, Button: "left", Template: tinyPNG(t), AnchorText: "Play", AnchorVision: "button"},
	}
	src, assets, err := Route("start game", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"use vision.",                             // the detector surface is imported
		"this's Label is a text.",                 // the Anchor type carries the class locator
		`the v1 is an Anchor with Label "button"`, // vision locator declared, with the class
		", X 640, Y 480",                          // ...the recorded point as a hint
		"do OS's Find with a1...",                 // 1) image gate
		"do Text's Find with t1...",               // 2) text locator
		"do Vision's Locate with v1...",           // 3) vision locator
	} {
		if !strings.Contains(src, want) {
			t.Errorf("gate+text+vision route missing %q\n--- source ---\n%s", want, src)
		}
	}
	// Ordering: Text's Find must come before Vision's Locate (exact text beats class).
	if ti, vi := strings.Index(src, "Text's Find"), strings.Index(src, "Vision's Locate"); ti < 0 || vi < 0 || ti > vi {
		t.Errorf("expected Text's Find before Vision's Locate (text %d, vision %d)", ti, vi)
	}
	for path, data := range assets {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("gate+text+vision route failed to compile: %v\n--- source ---\n%s", err, src)
	}
	if err := driver.RunFileWithHosts(routePath, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("gate+text+vision route failed to run: %v\n--- source ---\n%s", err, src)
	}
}

// TestWindowRelativeClick: a click that captured a window offset becomes a
// window-relative Point (absolute X,Y fallback + RelX,RelY), and the route still
// compiles and runs under the dryrun host.
func TestWindowRelativeClick(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{
		{Kind: macroir.StepClick, X: 500, Y: 300, Button: "left", WinRel: true, RelX: 40, RelY: 30},
	}
	src, _, err := Route("tap", "notepad", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "the p1 is a Point with X 500, Y 300, RelX 40, RelY 30.") {
		t.Fatalf("expected a window-relative point\n--- source ---\n%s", src)
	}
	routePath := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(routePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(routePath, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("window-relative route failed to compile: %v\n--- source ---\n%s", err, src)
	}
}

// TestArgPlaceholders: a typed step with numeric {{N}} placeholders keeps them in
// the source (positional args, not secrets), and routes.ApplyArgs fills them at run
// time so the route types the passed values.
func TestArgPlaceholders(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{{Kind: macroir.StepType, Text: "{{1}} and {{2}}"}}
	src, _, err := Route("greet", "", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "{{1}}") || !strings.Contains(src, "{{2}}") {
		t.Fatalf("numeric placeholders should survive in source:\n%s", src)
	}
	filled := routes.ApplyArgs(src, nil, []string{"hello", "world"})
	var out bytes.Buffer
	if err := driver.RunSourceWithHostsCtx(context.Background(), filled, dir, "greet.marco", &out, nil); err != nil {
		t.Fatalf("run: %v\n--- filled ---\n%s", err, filled)
	}
	if !strings.Contains(out.String(), "hello and world") {
		t.Fatalf("args not substituted at run; output:\n%s\n--- filled ---\n%s", out.String(), filled)
	}
}

// TestNamedArgPlaceholders: a DECLARED {{name}} stays in the route (filled at run by
// name), while an UNDECLARED {{secret}} still becomes a Secret lookup.
func TestNamedArgPlaceholders(t *testing.T) {
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	steps := []macroir.Step{{Kind: macroir.StepType, Text: "hi {{name}} {{secret-pin}}"}}
	src, _, err := Route("greet", "", steps, dir, "name") // declare "name" only
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "{{name}}") {
		t.Fatalf("declared arg {{name}} should survive:\n%s", src)
	}
	if !strings.Contains(src, `do OS's Secret with "secret-pin".`) {
		t.Fatalf("undeclared {{secret-pin}} should become a Secret:\n%s", src)
	}
	filled := routes.ApplyArgs(src, map[string]string{"name": "chris"}, nil)
	var out bytes.Buffer
	if err := driver.RunSourceWithHostsCtx(context.Background(), filled, dir, "greet.marco", &out, nil); err != nil {
		t.Fatalf("run: %v\n--- filled ---\n%s", err, filled)
	}
	if !strings.Contains(out.String(), "hi chris") {
		t.Fatalf("named arg not filled; output:\n%s", out.String())
	}
}

// TestSecretArg: a declared arg whose name is secret-typed (password) becomes a
// route-qualified Secret lookup (never written into the route), while a plain
// declared arg (username) stays a {{name}} placeholder.
func TestSecretArg(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepType, Text: "{{username}}"},
		{Kind: macroir.StepType, Text: "{{password}}"},
	}
	src, _, err := Route("login to facebook", "facebook", steps, "", "username", "password")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "{{username}}") {
		t.Errorf("plain declared arg username should stay a placeholder:\n%s", src)
	}
	if !strings.Contains(src, `do OS's Secret with "login-to-facebook:password".`) {
		t.Errorf("password should be a route-qualified Secret:\n%s", src)
	}
	if strings.Contains(src, "{{password}}") {
		t.Errorf("password must NOT be left in the route source:\n%s", src)
	}
}

var sample = []macroir.Step{
	{Kind: macroir.StepClick, X: 100, Y: 200, Button: "left"},
	{Kind: macroir.StepWait, Ms: 350},
	{Kind: macroir.StepClick, X: 400, Y: 500, Button: "left"},
	{Kind: macroir.StepKey, Key: "e", Count: 3},
	{Kind: macroir.StepType, Text: `say "hi"`},
}

func TestRouteStructure(t *testing.T) {
	src, _, err := Route("open chest", "", sample, "")
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

func TestSecretPlaceholders(t *testing.T) {
	steps := []macroir.Step{
		{Kind: macroir.StepType, Text: "{{fb-password}}"},
		{Kind: macroir.StepType, Text: "user{{token}}!"},
	}
	src, _, err := Route("login", "", steps, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`do OS's Secret with "fb-password".`,
		`do OS's Type with "user".`,
		`do OS's Secret with "token".`,
		`do OS's Type with "!".`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q\n%s", want, src)
		}
	}
	// The literal password must never appear, and the placeholder braces must be
	// gone (converted to a Secret lookup).
	if strings.Contains(src, "{{") {
		t.Errorf("placeholder braces leaked into route:\n%s", src)
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
		"copy paste": {
			{Kind: macroir.StepKey, Key: "ctrl+c", Count: 1},
			{Kind: macroir.StepKey, Key: "ctrl+v", Count: 1},
		},
		"switch apps": {
			{Kind: macroir.StepActivate, Text: "notepad"},
			{Kind: macroir.StepType, Text: "hello"},
			{Kind: macroir.StepActivate, Text: "chrome"},
			{Kind: macroir.StepClick, X: -1920, Y: -5, Button: "left"}, // negative coord on a secondary monitor
		},
		"secret login": {
			{Kind: macroir.StepClick, X: 5, Y: 5, Button: "left"},
			{Kind: macroir.StepType, Text: "myuser"},
			{Kind: macroir.StepKey, Key: "tab"},
			{Kind: macroir.StepType, Text: "{{login-pw}}"},
			{Kind: macroir.StepKey, Key: "enter"},
		},
	}
	osSrc, err := os.ReadFile(filepath.Join("..", "osmod", "os.marco"))
	if err != nil {
		t.Fatalf("read os.marco: %v", err)
	}
	for name, steps := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
				t.Fatal(err)
			}
			src, assets, err := Route(name, "", steps, dir)
			if err != nil {
				t.Fatal(err)
			}
			for path, data := range assets {
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
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
