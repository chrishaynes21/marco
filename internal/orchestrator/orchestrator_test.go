package orchestrator

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// TestMain enables CV for the package — these teach tests were written for the CV-on default
// (anchors captured, waits capped to 50ms). CV is a feature flag that's OFF by default now, so
// without this the faithful-timing path would change the expected Sleep values.
func TestMain(m *testing.M) {
	os.Setenv("MARCO_CV", "on")
	os.Exit(m.Run())
}

// fakeRecorder replays a canned event stream, so the whole teach pipeline
// (record → simplify → codegen → save → run) is testable with no OS hooks.
type fakeRecorder struct {
	events []recorder.RecordedEvent
	ch     chan recorder.RecordedEvent
}

func (f *fakeRecorder) Start() error {
	f.ch = make(chan recorder.RecordedEvent, len(f.events))
	for _, e := range f.events {
		f.ch <- e
	}
	close(f.ch)
	return nil
}
func (f *fakeRecorder) Stop() []recorder.RecordedEvent        { return f.events }
func (f *fakeRecorder) Events() <-chan recorder.RecordedEvent { return f.ch }

func at(ms int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(ms) * time.Millisecond)
}

func TestTeachThenRun(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	// Demonstration: click (100,200), wait ~350ms, click (400,500), then Esc.
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 100, Y: 200, T: at(0)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 100, Y: 200, T: at(5)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 400, Y: 500, T: at(350)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 400, Y: 500, T: at(355)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(800)}, // stop gesture
	}

	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:      strings.NewReader("y\ny\nc\n"), // teach? yes; save? yes; scope? c (context: only here)
		Out:     &out,
		App:     func() string { return "sea-of-thieves" }, // context captured at teach time
		StopKey: "esc",                                     // these fixtures end with an Esc press
	}

	// First Do: unknown → teaches, saves (scoped), and runs under dryrun.
	if err := d.Do("open chest"); err != nil {
		t.Fatalf("Do(teach): %v\n%s", err, out.String())
	}
	scoped := routes.Route{App: "sea-of-thieves", Slug: "open-chest"}
	if !reg.Has(scoped) {
		t.Fatal("route was not saved under its app scope")
	}
	// The Esc stop gesture must not appear as a step in the saved route.
	saved, _ := os.ReadFile(reg.Path(scoped))
	if strings.Contains(string(saved), `Key with "esc"`) {
		t.Fatalf("stop key leaked into route:\n%s", saved)
	}
	if !strings.Contains(string(saved), "do OS's Click with p1.") ||
		!strings.Contains(string(saved), "do OS's Sleep with 50.") {
		t.Fatalf("route missing expected steps:\n%s", saved)
	}
	// Context scope is foreground-only, so it does NOT switch windows — no Activate.
	if strings.Contains(string(saved), "Activate") {
		t.Fatalf("context route should not activate:\n%s", saved)
	}
	// It also ran (dryrun host logs the calls + the done line).
	if !strings.Contains(out.String(), "[dryrun] OS's Click") ||
		!strings.Contains(out.String(), "open chest: done") {
		t.Fatalf("route did not run; output:\n%s", out.String())
	}

	// Second Do: now known → runs directly without teaching. A context route is
	// foreground-only, so the app must be in front for it to resolve.
	var out2 bytes.Buffer
	d2 := Deps{Reg: reg, Rec: &fakeRecorder{}, Out: &out2, App: func() string { return "sea-of-thieves" }}
	if err := d2.Do("open chest"); err != nil {
		t.Fatalf("Do(run): %v", err)
	}
	if strings.Contains(out2.String(), "I don't know") {
		t.Fatalf("second Do re-taught instead of running:\n%s", out2.String())
	}
	if !strings.Contains(out2.String(), "open chest: done") {
		t.Fatalf("second Do did not run:\n%s", out2.String())
	}
}

// TestTeachSimplifyOption exercises the third save-menu option: the user types
// at human speed (per-letter keys by default), chooses "s" to simplify, and the
// saved route has a single clean Type step instead of letter-by-letter presses.
func TestTeachSimplifyOption(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	events := []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "h", Down: true, T: at(0)},
		{Kind: recorder.EvKey, KeyName: "e", Down: true, T: at(140)},
		{Kind: recorder.EvKey, KeyName: "l", Down: true, T: at(220)},
		{Kind: recorder.EvKey, KeyName: "l", Down: true, T: at(310)},
		{Kind: recorder.EvKey, KeyName: "o", Down: true, T: at(400)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(900)},
	}

	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:      strings.NewReader("y\ns\ny\nc\n"), // teach? yes; simplify further; save; scope c (context)
		Out:     &out,
		App:     func() string { return "editor" },
		StopKey: "esc",
	}
	if err := d.Do("type hello"); err != nil {
		t.Fatalf("Do: %v\n%s", err, out.String())
	}
	rt := routes.Route{App: "editor", Slug: "type-hello"}
	if !reg.Has(rt) {
		t.Fatal("route was not saved")
	}
	saved, _ := os.ReadFile(reg.Path(rt))
	if !strings.Contains(string(saved), `do OS's Type with "hello".`) {
		t.Fatalf("simplify-further did not fold typing into a Type step:\n%s", saved)
	}
	if strings.Contains(string(saved), `do OS's Key with "h".`) {
		t.Fatalf("per-letter key presses remained after simplify:\n%s", saved)
	}
}

// TestTeachSimplifySaves: with SimplifySaves set (the overlay, which can't show
// the preview), choosing "s" simplifies AND saves in one step — no re-confirm. The
// input is "s" then the scope answer "y" (only-in-app); the "exactly one save
// prompt" check below is the real guard — if SimplifySaves were broken, a second
// "Save this as" re-prompt would appear and consume the "y".
func TestTeachSimplifySaves(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	events := []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "h", Down: true, T: at(0)},
		{Kind: recorder.EvKey, KeyName: "e", Down: true, T: at(140)},
		{Kind: recorder.EvKey, KeyName: "l", Down: true, T: at(220)},
		{Kind: recorder.EvKey, KeyName: "l", Down: true, T: at(310)},
		{Kind: recorder.EvKey, KeyName: "o", Down: true, T: at(400)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(900)},
	}

	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:            strings.NewReader("s\nc\n"), // simplify (auto-saves), then scope c (context)
		Out:           &out,
		App:           func() string { return "editor" },
		StopKey:       "esc",
		SimplifySaves: true,
	}
	if err := d.Teach("type hello"); err != nil {
		t.Fatalf("Teach: %v\n%s", err, out.String())
	}
	rt := routes.Route{App: "editor", Slug: "type-hello"}
	if !reg.Has(rt) {
		t.Fatalf("route was not saved — simplify did not auto-save:\n%s", out.String())
	}
	saved, _ := os.ReadFile(reg.Path(rt))
	if !strings.Contains(string(saved), `do OS's Type with "hello".`) {
		t.Fatalf("simplify did not fold typing into a Type step:\n%s", saved)
	}
	// And it must NOT have re-asked "Save this as ... [y]es / [n]o:" after simplify.
	if strings.Count(out.String(), "Save this as") != 1 {
		t.Fatalf("expected exactly one save prompt (the menu), got a re-prompt:\n%s", out.String())
	}
}

// TestSimplifyRoute teaches a verbose route (per-letter typing kept faithful),
// then re-simplifies the SAVED route from its stored recording — proving the
// demonstration is persisted beside the route and that `marco simplify` folds
// the typing into a clean Type step in place.
func TestSimplifyRoute(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	events := []recorder.RecordedEvent{
		{Kind: recorder.EvKey, KeyName: "h", Down: true, T: at(0)},
		{Kind: recorder.EvKey, KeyName: "e", Down: true, T: at(140)},
		{Kind: recorder.EvKey, KeyName: "l", Down: true, T: at(220)},
		{Kind: recorder.EvKey, KeyName: "l", Down: true, T: at(310)},
		{Kind: recorder.EvKey, KeyName: "o", Down: true, T: at(400)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(900)},
	}

	// Teach and save faithfully (no simplify at teach time): keys stay per-letter.
	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:      strings.NewReader("y\nc\n"), // save; scope c (context)
		Out:     &out,
		App:     func() string { return "editor" },
		StopKey: "esc",
	}
	if err := d.Teach("type hello"); err != nil {
		t.Fatalf("Teach: %v\n%s", err, out.String())
	}
	rt := routes.Route{App: "editor", Slug: "type-hello"}
	if saved, _ := os.ReadFile(reg.Path(rt)); !strings.Contains(string(saved), `do OS's Key with "h".`) {
		t.Fatalf("expected faithful per-letter route before simplify:\n%s", saved)
	}
	if _, ok := reg.LoadRecording(rt); !ok {
		t.Fatal("teach did not persist the recording beside the route")
	}

	// Now re-simplify the saved route from its recording.
	var out2 bytes.Buffer
	d2 := Deps{
		Reg: reg, Rec: &fakeRecorder{},
		In:  strings.NewReader("y\n"), // save the simplified version
		Out: &out2,
		App: func() string { return "editor" },
	}
	if err := d2.SimplifyRoute("type hello"); err != nil {
		t.Fatalf("SimplifyRoute: %v\n%s", err, out2.String())
	}
	saved, _ := os.ReadFile(reg.Path(rt))
	if !strings.Contains(string(saved), `do OS's Type with "hello".`) {
		t.Fatalf("simplify did not fold typing into a Type step:\n%s", saved)
	}
	if strings.Contains(string(saved), `do OS's Key with "h".`) {
		t.Fatalf("per-letter key presses remained after simplify:\n%s", saved)
	}
}

// TestSimplifyRouteNoRecording reports a clear error (not a crash) when a route
// has no stored demonstration to re-simplify from.
func TestSimplifyRouteNoRecording(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}
	if err := reg.Save(routes.Route{App: "editor", Slug: routes.Slug("hand written")}, "use os.\n"); err != nil {
		t.Fatal(err)
	}
	d := Deps{Reg: reg, Out: &bytes.Buffer{}, App: func() string { return "editor" }}
	if err := d.SimplifyRoute("hand written"); err == nil {
		t.Fatal("expected an error for a route with no recording")
	}
}

// TestTeachImageClick: teaching a click that carries a captured template saves
// an image-find route and writes the template PNG beside the route.
func TestTeachImageClick(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	patch := []byte("\x89PNG\r\n\x1a\n fake template bytes")
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 300, Y: 200, T: at(0), Image: patch},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 300, Y: 200, T: at(5)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(500)},
	}
	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:      strings.NewReader("y\nc\n"), // save; scope c (context)
		Out:     &out,
		App:     func() string { return "game" },
		StopKey: "esc",
	}
	if err := d.Teach("grab loot"); err != nil {
		t.Fatalf("Teach: %v\n%s", err, out.String())
	}
	rt := routes.Route{App: "game", Slug: "grab-loot"}
	saved, _ := os.ReadFile(reg.Path(rt))
	if !strings.Contains(string(saved), "do OS's Find with a1...") {
		t.Fatalf("route is not image-matched:\n%s", saved)
	}
	// The template file was written beside the route (in its scope folder), with the
	// captured bytes.
	asset := filepath.Join(reg.ScopeDir(rt), "grab-loot-anchor-1.png")
	got, err := os.ReadFile(asset)
	if err != nil {
		t.Fatalf("template not written: %v", err)
	}
	if string(got) != string(patch) {
		t.Fatal("written template bytes don't match the captured patch")
	}
	// The route references that absolute path.
	if !strings.Contains(string(saved), escapeForMarco(asset)) {
		t.Fatalf("route does not reference the template path %q:\n%s", asset, saved)
	}
}

// fakeTextHost stands in for the OCR bridge: any Text's Read returns a fixed label,
// so the teach-time anchor-labelling path is testable without tesseract. A nil reply
// (replyText == "") models "nothing readable" — the anchor should stay gate-only.
type fakeTextHost struct {
	replyText string
	calls     int
}

func (h *fakeTextHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	if c.Act != "Text" || c.Action != "Read" {
		return "failed", runtime.Absent(), nil
	}
	h.calls++
	if h.replyText == "" {
		return "failed", runtime.Absent(), nil
	}
	s := runtime.NewSet()
	s.Put("Text", runtime.Text(h.replyText))
	return "ok", runtime.SetVal(s), nil
}

// fakeVisionHost stands in for the vision bridge: any Vision's Identify returns a fixed
// class label and (optionally) a detected box, so the teach-time vision-labelling and
// re-cropping paths are testable without a model.
type fakeVisionHost struct {
	label string
	box   [4]int // X,Y,W,H — when W>0, returned so the template gets re-cropped to it
	calls int
}

func (h *fakeVisionHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	if c.Act != "Vision" || c.Action != "Identify" {
		return "failed", runtime.Absent(), nil
	}
	h.calls++
	if h.label == "" {
		return "failed", runtime.Absent(), nil
	}
	s := runtime.NewSet()
	s.Put("Label", runtime.Text(h.label))
	if h.box[2] > 0 {
		s.Put("X", runtime.Number(float64(h.box[0])))
		s.Put("Y", runtime.Number(float64(h.box[1])))
		s.Put("W", runtime.Number(float64(h.box[2])))
		s.Put("H", runtime.Number(float64(h.box[3])))
	}
	return "ok", runtime.SetVal(s), nil
}

// TestTeachVisionRecropsTemplate: with a Vision host that returns a box, the demonstrated
// anchor's template is re-cropped to the detected control — the saved PNG is the box size.
func TestTeachVisionRecropsTemplate(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	// A real 100x60 PNG template (so cropTemplate can decode + re-encode it).
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 100, 60))); err != nil {
		t.Fatal(err)
	}
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 300, Y: 200, T: at(0), Image: pngBuf.Bytes()},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 300, Y: 200, T: at(5)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(500)},
	}
	vis := &fakeVisionHost{label: "button", box: [4]int{20, 10, 50, 30}} // detected control
	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		Hosts:   map[string]runtime.Host{"Vision": vis},
		In:      strings.NewReader("y\nc\n"),
		Out:     &out,
		App:     func() string { return "game" },
		StopKey: "esc",
	}
	if err := d.Teach("dodge"); err != nil {
		t.Fatalf("Teach: %v\n%s", err, out.String())
	}
	rt := routes.Route{App: "game", Slug: "dodge"}
	asset := filepath.Join(reg.ScopeDir(rt), "dodge-anchor-1.png")
	data, err := os.ReadFile(asset)
	if err != nil {
		t.Fatalf("template not written: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("saved template not a PNG: %v", err)
	}
	if cfg.Width != 50 || cfg.Height != 30 {
		t.Fatalf("template not re-cropped to the detected box: got %dx%d, want 50x30", cfg.Width, cfg.Height)
	}
}

// TestTeachVisionAnchorFromDetector: with a Vision host wired, a demonstrated anchor is
// labelled with the clicked control's CLASS, and the saved route locates it via Vision.
func TestTeachVisionAnchorFromDetector(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	patch := []byte("\x89PNG\r\n\x1a\n fake template bytes")
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 300, Y: 200, T: at(0), Image: patch},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 300, Y: 200, T: at(5)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(500)},
	}
	vis := &fakeVisionHost{label: "icon"}
	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		Hosts:   map[string]runtime.Host{"Vision": vis},
		In:      strings.NewReader("y\nc\n"),
		Out:     &out,
		App:     func() string { return "game" },
		StopKey: "esc",
	}
	if err := d.Teach("use ability"); err != nil {
		t.Fatalf("Teach: %v\n%s", err, out.String())
	}
	if vis.calls == 0 {
		t.Fatal("Vision's Identify was never called to label the anchor")
	}
	rt := routes.Route{App: "game", Slug: "use-ability"}
	saved, _ := os.ReadFile(reg.Path(rt))
	if !strings.Contains(string(saved), "use vision.") ||
		!strings.Contains(string(saved), `Anchor with Label "icon"`) ||
		!strings.Contains(string(saved), "do Vision's Locate with v1...") {
		t.Fatalf("route did not gain the vision locator:\n%s", saved)
	}
}

// TestTeachTextAnchorFromOCR: a demonstrated anchor (a click carrying a template) gets
// OCR-labelled at teach time, so the saved route also locates the target by text.
func TestTeachTextAnchorFromOCR(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	patch := []byte("\x89PNG\r\n\x1a\n fake template bytes")
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 300, Y: 200, T: at(0), Image: patch},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 300, Y: 200, T: at(5)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(500)},
	}
	host := &fakeTextHost{replyText: "Start Game"}
	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		Hosts:   map[string]runtime.Host{"Text": host},
		In:      strings.NewReader("y\nc\n"), // save; scope c (context)
		Out:     &out,
		App:     func() string { return "game" },
		StopKey: "esc",
	}
	if err := d.Teach("grab loot"); err != nil {
		t.Fatalf("Teach: %v\n%s", err, out.String())
	}
	if host.calls == 0 {
		t.Fatal("Text's Read was never called to label the anchor")
	}
	rt := routes.Route{App: "game", Slug: "grab-loot"}
	saved, _ := os.ReadFile(reg.Path(rt))
	// The anchor is both an image gate AND a text locator now.
	if !strings.Contains(string(saved), "do OS's Find with a1...") {
		t.Fatalf("route lost its image gate:\n%s", saved)
	}
	if !strings.Contains(string(saved), `Anchor with Text "Start Game"`) ||
		!strings.Contains(string(saved), "do Text's Find with t1...") {
		t.Fatalf("route did not gain the OCR text locator:\n%s", saved)
	}
}

// TestTeachTextAnchorNoHost: with no Text host wired, teaching the same anchor stays
// image-only — text labelling is purely additive and must not require OCR.
func TestTeachTextAnchorNoHost(t *testing.T) {
	dir := t.TempDir()
	reg := routes.Registry{Dir: dir}

	patch := []byte("\x89PNG\r\n\x1a\n fake template bytes")
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 300, Y: 200, T: at(0), Image: patch},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 300, Y: 200, T: at(5)},
		{Kind: recorder.EvKey, KeyName: "esc", Down: true, T: at(500)},
	}
	var out bytes.Buffer
	d := Deps{
		Reg: reg, Rec: &fakeRecorder{events: events},
		In:      strings.NewReader("y\nc\n"),
		Out:     &out,
		App:     func() string { return "game" },
		StopKey: "esc",
	}
	if err := d.Teach("grab loot"); err != nil {
		t.Fatalf("Teach: %v\n%s", err, out.String())
	}
	rt := routes.Route{App: "game", Slug: "grab-loot"}
	saved, _ := os.ReadFile(reg.Path(rt))
	if strings.Contains(string(saved), "Text's Find") {
		t.Fatalf("text locator appeared with no Text host wired:\n%s", saved)
	}
}

// escapeForMarco mirrors codegen's backslash/quote escaping so the test can look
// for the embedded Windows path.
func escapeForMarco(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return s
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"login to facebook":     "login-to-facebook",
		"Start Sea of Thieves!": "start-sea-of-thieves",
		"  spaced  out  ":       "spaced-out",
	}
	for in, want := range cases {
		if got := routes.Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}
