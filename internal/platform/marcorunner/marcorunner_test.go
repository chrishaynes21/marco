package marcorunner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/lexer"
	"github.com/chaynes-simpleclouds/marco/internal/parser"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// These tests enforce the boundary rule:
//
//	The Director may only compile into Marco what Marco can genuinely parse,
//	validate, and execute today.
//
// They live here rather than under internal/director because proving it needs the
// lexer, the parser, the graph builder and the compiler — every one of which the
// Director's own import guard forbids it from touching. That split is the point: the
// Director generates, and something that can see BOTH sides checks.

// everyOperation is one of each Kind, with realistic payloads including the ones most
// likely to break lowering: negative coordinates, a non-left click, a held chord, text
// full of quotes and backslashes and Unicode.
//
// Every table-driven test below runs the whole set, so a new Kind is covered the
// moment it is added — and a Kind with no Marco capability fails immediately.
func everyOperation() []marcoexec.Operation {
	return []marcoexec.Operation{
		{Kind: marcoexec.KindClick, At: marcoexec.Point{X: 912, Y: 704}},
		{Kind: marcoexec.KindClick, At: marcoexec.Point{X: -1400, Y: -200}, Button: "right"},
		{Kind: marcoexec.KindClick, At: marcoexec.Point{X: 10, Y: 20}, Count: 2},
		{Kind: marcoexec.KindMove, At: marcoexec.Point{X: -5, Y: -7}},
		{Kind: marcoexec.KindDrag, From: marcoexec.Point{X: -1400, Y: 50}, To: marcoexec.Point{X: 200, Y: -60}, Button: "middle"},
		{Kind: marcoexec.KindKey, Chord: "ctrl+a"},
		{Kind: marcoexec.KindKey, Chord: "alt+tab", Hold: 250},
		{Kind: marcoexec.KindType, Text: "café 日本 🎉 \"quoted\" back\\slash\ttab\nnewline"},
		{Kind: marcoexec.KindType, Text: ""},
		{Kind: marcoexec.KindTypeSecret, SecretRef: "prod-root-password"},
		{Kind: marcoexec.KindActivate, App: "notepad"},
		{Kind: marcoexec.KindLaunch, Target: `C:\Windows\System32\notepad.exe`},
		{Kind: marcoexec.KindMoveWindow, Window: "hwnd:12345", Bounds: marcoexec.Rect{X: -1920, Y: -100, W: 800, H: 600}},
		{Kind: marcoexec.KindWindowState, Window: "hwnd:12345", State: "maximized"},
		{Kind: marcoexec.KindFocus, Window: "hwnd:12345", Element: "42.7.3", MaxNodes: 1500},
		{Kind: marcoexec.KindSetValue, Window: "hwnd:12345", Element: "42.7.3", Value: `he said "hi"\`},
		{Kind: marcoexec.KindGetValue, Element: "42.7.3"},
		{Kind: marcoexec.KindClipboardGet},
		{Kind: marcoexec.KindClipboardSet, Text: "clipboard\r\ntext"},

		// The structural control effects. Each is here for the same reason as the rest:
		// the semantic action vocabulary is only real if every verb it offers reaches a
		// capability Marco actually declares, and this is the test that compiles them.
		{Kind: marcoexec.KindInvoke, Window: "hwnd:12345", Element: "42.7.3", MaxNodes: 1500},
		{Kind: marcoexec.KindExpand, Element: "42.7.3"},
		{Kind: marcoexec.KindCollapse, Element: "42.7.3"},
		{Kind: marcoexec.KindToggle, Element: "42.7.3"},
		{Kind: marcoexec.KindSelect, Element: "42.7.3"},
		{Kind: marcoexec.KindDeselect, Element: "42.7.3"},
		{Kind: marcoexec.KindScrollIntoView, Element: "42.7.3"},
	}
}

func name(o marcoexec.Operation) string { return o.Describe() }

// validate runs Marco's real validation — the same path `marco check` takes, through
// driver.Check, which lexes, parses, builds the graph, resolves every `use` import and
// compiles.
//
// driver.Check rather than the stages by hand, because merging an imported module's
// graph is unexported: reproducing it would test a copy of the logic instead of the
// logic. The program goes to a temp dir with NO sibling os.marco, so imports resolve to
// the embedded surfaces — a stale copy on disk would otherwise decide the result, which
// is exactly the drift osmod's own test guards against.
func validate(t *testing.T, program string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "program.marco")
	if err := os.WriteFile(path, []byte(program), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out bytes.Buffer
	if err := driver.Check(path, &out, false); err != nil {
		t.Fatalf("marco check rejected the generated program: %v\n%s\n\n%s", err, out.String(), program)
	}
}

// ── 1–4: lowers, lexes, parses, builds, compiles ──────────────────────────────

func TestEveryOperationLowersToLegalMarco(t *testing.T) {
	for _, o := range everyOperation() {
		t.Run(name(o), func(t *testing.T) {
			src, err := marcoexec.Lower(o)
			if err != nil {
				t.Fatalf("lower: %v", err)
			}
			tokens, err := lexer.Lex(src)
			if err != nil {
				t.Fatalf("the generated program does not lex: %v\n\n%s", err, src)
			}
			if _, err := parser.Parse(tokens); err != nil {
				t.Fatalf("the generated program does not parse: %v\n\n%s", err, src)
			}
			validate(t, src)
		})
	}
}

// ── 5: the host receives the expected capability and values ───────────────────

func TestTheHostReceivesWhatTheOperationMeant(t *testing.T) {
	cases := []struct {
		op    marcoexec.Operation
		calls []string
		check func(*testing.T, []runtime.HostCall)
	}{{
		op:    marcoexec.Operation{Kind: marcoexec.KindClick, At: marcoexec.Point{X: 912, Y: 704}},
		calls: []string{"OS's Click"},
		check: func(t *testing.T, c []runtime.HostCall) { wantPoint(t, c[0].Input, 912, 704) },
	}, {
		// A non-left click at a point is Move-then-Click, because OS's Click takes
		// EITHER a Point (always left) OR a button name (always at the cursor).
		op:    marcoexec.Operation{Kind: marcoexec.KindClick, At: marcoexec.Point{X: -1400, Y: -200}, Button: "right"},
		calls: []string{"OS's Move", "OS's Click"},
		check: func(t *testing.T, c []runtime.HostCall) {
			wantPoint(t, c[0].Input, -1400, -200)
			if got := c[1].Input.AsText(); got != "right" {
				t.Fatalf("button = %q, want right", got)
			}
		},
	}, {
		op:    marcoexec.Operation{Kind: marcoexec.KindClick, At: marcoexec.Point{X: 1, Y: 2}, Count: 2},
		calls: []string{"OS's Click", "OS's Click"},
	}, {
		op: marcoexec.Operation{Kind: marcoexec.KindDrag,
			From: marcoexec.Point{X: -1400, Y: 50}, To: marcoexec.Point{X: 200, Y: -60}, Button: "middle"},
		calls: []string{"OS's Drag"},
		check: func(t *testing.T, c []runtime.HostCall) {
			s := c[0].Input.AsSet()
			if s == nil {
				t.Fatalf("drag did not arrive as a Drag set: %v", c[0].Input)
			}
			for f, want := range map[string]float64{"FromX": -1400, "FromY": 50, "ToX": 200, "ToY": -60} {
				v, _ := s.Get(f)
				if num(v) != want {
					t.Fatalf("drag %s = %v, want %v — negative coordinates must survive", f, v, want)
				}
			}
			if v, _ := s.Get("Button"); v.AsText() != "middle" {
				t.Fatalf("drag button = %v, want middle", v)
			}
		},
	}, {
		op:    marcoexec.Operation{Kind: marcoexec.KindActivate, App: "notepad"},
		calls: []string{"OS's Activate"},
		check: func(t *testing.T, c []runtime.HostCall) {
			if got := c[0].Input.AsText(); got != "notepad" {
				t.Fatalf("activate = %q", got)
			}
		},
	}, {
		op:    marcoexec.Operation{Kind: marcoexec.KindLaunch, Target: `C:\Windows\notepad.exe`},
		calls: []string{"OS's Launch"},
		check: func(t *testing.T, c []runtime.HostCall) {
			// The backslashes are the point: escaped into the literal, unescaped by
			// the lexer, identical at the host.
			if got := c[0].Input.AsText(); got != `C:\Windows\notepad.exe` {
				t.Fatalf("launch = %q, want the path with its backslashes intact", got)
			}
		},
	}, {
		op: marcoexec.Operation{Kind: marcoexec.KindMoveWindow, Window: "hwnd:12345",
			Bounds: marcoexec.Rect{X: -1920, Y: -100, W: 800, H: 600}},
		calls: []string{"OS's MoveWindow"},
		check: func(t *testing.T, c []runtime.HostCall) {
			s := c[0].Input.AsSet()
			if s == nil {
				t.Fatalf("move did not arrive as a WindowMove set: %v", c[0].Input)
			}
			for f, want := range map[string]float64{"X": -1920, "Y": -100, "W": 800, "H": 600} {
				v, _ := s.Get(f)
				if num(v) != want {
					t.Fatalf("move %s = %v, want %v", f, v, want)
				}
			}
			if v, _ := s.Get("Window"); v.AsText() != "hwnd:12345" {
				t.Fatalf("move Window = %v", v)
			}
		},
	}, {
		op:    marcoexec.Operation{Kind: marcoexec.KindWindowState, Window: "hwnd:1", State: "maximized"},
		calls: []string{"OS's WindowState"},
		check: func(t *testing.T, c []runtime.HostCall) {
			s := c[0].Input.AsSet()
			if v, _ := s.Get("State"); v.AsText() != "maximized" {
				t.Fatalf("state = %v", v)
			}
		},
	}, {
		op:    marcoexec.Operation{Kind: marcoexec.KindTypeSecret, SecretRef: "my-login"},
		calls: []string{"OS's Secret"},
		check: func(t *testing.T, c []runtime.HostCall) {
			// The NAME reaches the host, which fetches the value itself. That is the
			// entire reason this capability exists rather than a Type with a password.
			if got := c[0].Input.AsText(); got != "my-login" {
				t.Fatalf("secret ref = %q", got)
			}
		},
	}}

	for _, c := range cases {
		t.Run(name(c.op), func(t *testing.T) {
			rec := &recordingHost{}
			ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec, "Accessibility": rec}))
			if _, err := ex.Do(context.Background(), c.op); err != nil {
				t.Fatalf("do: %v", err)
			}
			if got := rec.names(); !equal(got, c.calls) {
				t.Fatalf("capabilities = %v, want %v", got, c.calls)
			}
			if c.check != nil {
				c.check(t, rec.calls)
			}
		})
	}
}

// ── 6: unsupported fails before mutation ──────────────────────────────────────

func TestUnsupportedOperationsFailBeforeAnythingRuns(t *testing.T) {
	for _, c := range []struct {
		why string
		op  marcoexec.Operation
	}{
		{"an unknown kind", marcoexec.Operation{Kind: "teleport"}},
		{"a key with no chord", marcoexec.Operation{Kind: marcoexec.KindKey}},
		{"a secret with no name", marcoexec.Operation{Kind: marcoexec.KindTypeSecret}},
		{"a malformed window", marcoexec.Operation{Kind: marcoexec.KindWindowState, Window: "12345", State: "normal"}},
		{"a window state Marco does not know", marcoexec.Operation{Kind: marcoexec.KindWindowState, Window: "hwnd:1", State: "wobbly"}},
		{"a zero-sized window move", marcoexec.Operation{Kind: marcoexec.KindMoveWindow, Window: "hwnd:1"}},
		{"a mouse button Marco does not know", marcoexec.Operation{Kind: marcoexec.KindClick, Button: "thumb"}},
		{"text containing a NUL", marcoexec.Operation{Kind: marcoexec.KindType, Text: "a\x00b"}},
	} {
		t.Run(c.why, func(t *testing.T) {
			rec := &recordingHost{}
			ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec, "Accessibility": rec}))
			res, err := ex.Do(context.Background(), c.op)
			if err == nil {
				t.Fatalf("%s was accepted", c.why)
			}
			if res.Status != marcoexec.StatusUnsupported {
				t.Fatalf("status = %s, want unsupported", res.Status)
			}
			if len(rec.calls) != 0 {
				t.Fatalf("a host was called for %s: %v", c.why, rec.names())
			}
		})
	}
}

func TestACapabilityMarcoDoesNotExportFailsAtCompileTimeNotAtTheHost(t *testing.T) {
	// The compile-before-mutation guarantee, reporting itself. A program naming a
	// capability the language does not export must be rejected by the compiler, with
	// the runtime never entered.
	rec := &recordingHost{}
	r := marcorunner.New(map[string]runtime.Host{"OS": rec})

	src, err := marcoexec.Lower(marcoexec.Operation{Kind: marcoexec.KindKey, Chord: "a"})
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	bogus := strings.Replace(src, "OS's Key", "OS's Teleport", 1)
	if _, err := r.Run(context.Background(), "bogus", bogus); err == nil {
		t.Fatal("a program naming an unexported capability ran")
	}
	if len(rec.calls) != 0 {
		t.Fatalf("a host was called despite a compile error: %v", rec.names())
	}
}

func TestScrollIsRefusedRatherThanApproximated(t *testing.T) {
	// Marco has no scroll capability. Substituting keys or a drag would be a
	// different action, silently performed.
	rec := &recordingHost{}
	ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec}))
	err := ex.Scroll(context.Background(), directorapi.Point{X: 1, Y: 2}, 3, false)
	if err == nil {
		t.Fatal("scroll was approximated instead of refused")
	}
	if len(rec.calls) != 0 {
		t.Fatalf("scroll reached a host: %v", rec.names())
	}
}

// ── 7–9: round trip, escaping, negative coordinates ───────────────────────────

func TestTextSurvivesLoweringExactly(t *testing.T) {
	// The escaping test. strconv.Quote — the obvious choice — emits \u00e9, \x00 and
	// \r, and Marco's lexer accepts only \" \' \\ \n \t. A Director that typed any
	// non-ASCII text would have generated a program the lexer refused.
	for _, text := range []string{
		"", "plain", `he said "hi"`, `back\slash`, "tab\there", "line\nbreak",
		"café", "日本語", "🎉 emoji", `C:\path\to\file.exe`, `"\`, "\r carriage",
		`mixed "quotes" and \back\ and café 🎉`,
	} {
		t.Run(strings.ReplaceAll(text, "\n", "\\n"), func(t *testing.T) {
			rec := &recordingHost{}
			ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec}))
			if _, err := ex.Do(context.Background(),
				marcoexec.Operation{Kind: marcoexec.KindType, Text: text}); err != nil {
				t.Fatalf("do: %v", err)
			}
			if got := rec.calls[0].Input.AsText(); got != text {
				t.Fatalf("the host received %q, want %q — the literal did not round-trip", got, text)
			}
		})
	}
}

func TestStrconvQuoteWouldProduceInvalidMarco(t *testing.T) {
	// Guards the reason encode.go exists. If someone "simplifies" Quote back to
	// strconv.Quote, this fails and says why.
	program := "use os.\n\nthe P is an actor.\nthis can Run.\nthis's Run does...\n" +
		"    do OS's Type with \"caf\\u00e9\".\n    this is ok!\n\n" +
		"the App is a script.\ndo P's Run...\n    when ok?\n        log \"ok\".\n    or?\n        log that's error.\n"
	if _, err := lexer.Lex(program); err == nil {
		t.Fatal("the lexer accepted a \\u escape — encode.go's premise no longer holds, recheck readString")
	}
}

func TestNegativeCoordinatesSurviveLowering(t *testing.T) {
	// A monitor left of or above the primary one has negative coordinates. They must
	// not be mangled by the renderer or dropped by the omit-empty rules.
	rec := &recordingHost{}
	ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec}))
	if err := ex.Move(context.Background(), directorapi.Point{X: -1920, Y: -1080}); err != nil {
		t.Fatalf("move: %v", err)
	}
	wantPoint(t, rec.calls[0].Input, -1920, -1080)
}

func TestOperationsSerialiseWithoutLoss(t *testing.T) {
	// An Operation is a record, not a string, so it survives JSON — which is what
	// lets a planned operation be logged, replayed and diffed.
	for _, o := range everyOperation() {
		raw, err := json.Marshal(o)
		if err != nil {
			t.Fatalf("marshal %s: %v", o.Kind, err)
		}
		var back marcoexec.Operation
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("unmarshal %s: %v", o.Kind, err)
		}
		if !reflect.DeepEqual(back, o) {
			t.Fatalf("%s did not round-trip: %+v vs %+v", o.Kind, back, o)
		}
		a, _ := marcoexec.Lower(o)
		b, _ := marcoexec.Lower(back)
		if a != b {
			t.Fatalf("%s lowered differently after a round trip", o.Kind)
		}
	}
}

// ── 10: secrets never leak ────────────────────────────────────────────────────

func TestASecretsNameNeverAppearsInAnythingRetained(t *testing.T) {
	// The value never reaches this process at all — OS's Secret fetches it inside the
	// host — so what is guarded here is the credential NAME, which can itself be
	// sensitive, and the source line that carries it.
	const ref = "prod-root-password"
	rec := &recordingHost{fail: "secret"}
	var recorded []marcoexec.Result
	ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec})).
		WithRecorder(func(r marcoexec.Result) { recorded = append(recorded, r) })

	res, _ := ex.Do(context.Background(), marcoexec.Operation{Kind: marcoexec.KindTypeSecret, SecretRef: ref})

	if strings.Contains(res.Source, ref) {
		t.Fatalf("the retained source carries the credential name:\n%s", res.Source)
	}
	if !strings.Contains(res.Source, "<redacted>") {
		t.Fatalf("the retained source is not redacted:\n%s", res.Source)
	}
	if strings.Contains(res.Diagnostic, ref) {
		t.Fatalf("the diagnostic carries the credential name: %q", res.Diagnostic)
	}
	if strings.Contains(res.Summary(), ref) {
		t.Fatalf("the log line carries the credential name: %q", res.Summary())
	}
	for _, r := range recorded {
		if strings.Contains(r.Source, ref) || strings.Contains(r.Diagnostic, ref) {
			t.Fatal("the recorded result carries the credential name")
		}
	}
	// It must still be legal, executable Marco — redaction is for what is STORED, not
	// for what runs.
	src, err := marcoexec.Lower(marcoexec.Operation{Kind: marcoexec.KindTypeSecret, SecretRef: ref})
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	validate(t, src)
	if !strings.Contains(src, ref) {
		t.Fatal("the executed source does not name the credential, so the host cannot fetch it")
	}
}

func TestPreviewRedactsSecretsAndExecutesNothing(t *testing.T) {
	const ref = "my-login"
	res, err := marcoexec.Preview(marcoexec.Operation{Kind: marcoexec.KindTypeSecret, SecretRef: ref})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if strings.Contains(res.Source, ref) {
		t.Fatalf("preview leaked the credential name:\n%s", res.Source)
	}
	if !strings.Contains(res.Source, "OS's Secret with <redacted>") {
		t.Fatalf("preview did not show the redacted form:\n%s", res.Source)
	}
}

// ── 11–12: foreground protection ──────────────────────────────────────────────

// stubGuard refuses when the foreground is not the intended window.
type stubGuard struct {
	foreground string
	calls      int
}

func (g *stubGuard) Confirm(_ context.Context, window string) (bool, string, error) {
	g.calls++
	if window != "" && window != g.foreground {
		return false, "the intended window " + window + " is not in front; " + g.foreground + " is", nil
	}
	return true, "", nil
}

func TestForegroundMismatchRefusesTypingRatherThanTypingElsewhere(t *testing.T) {
	// The Discord incident, as a test. An edit resolved against one window executed
	// while another held the foreground, and the text went into a message box —
	// successfully, verifiably, and into the wrong application.
	rec := &recordingHost{}
	guard := &stubGuard{foreground: "hwnd:999"}
	ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec})).WithGuard(guard)

	for _, op := range []marcoexec.Operation{
		{Kind: marcoexec.KindType, Text: "hello", Window: "hwnd:111"},
		{Kind: marcoexec.KindTypeSecret, SecretRef: "login", Window: "hwnd:111"},
		{Kind: marcoexec.KindKey, Chord: "ctrl+v", Window: "hwnd:111"},
		{Kind: marcoexec.KindDrag, From: marcoexec.Point{X: 1}, To: marcoexec.Point{X: 2}, Window: "hwnd:111"},
	} {
		t.Run(string(op.Kind), func(t *testing.T) {
			before := len(rec.calls)
			res, err := ex.Do(context.Background(), op)
			if err == nil {
				t.Fatal("the operation ran despite the foreground having changed")
			}
			if res.Status != marcoexec.StatusContextChanged {
				t.Fatalf("status = %s, want %s", res.Status, marcoexec.StatusContextChanged)
			}
			if len(rec.calls) != before {
				t.Fatalf("input reached a host: %v", rec.names())
			}
		})
	}
}

func TestTheGuardIsConsultedAtExecutionNotOnceAtResolution(t *testing.T) {
	// "Do not assume a previous focus operation still holds." Every context-sensitive
	// operation asks again, because the foreground can change between any two of them.
	rec := &recordingHost{}
	guard := &stubGuard{foreground: "hwnd:111"}
	ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec})).WithGuard(guard)

	for i := 0; i < 3; i++ {
		if _, err := ex.Do(context.Background(),
			marcoexec.Operation{Kind: marcoexec.KindType, Text: "x", Window: "hwnd:111"}); err != nil {
			t.Fatalf("type %d: %v", i, err)
		}
	}
	if guard.calls != 3 {
		t.Fatalf("the guard was consulted %d times for 3 operations — a stale confirmation was reused", guard.calls)
	}
}

func TestWindowOperationsAreNotBlockedByTheForeground(t *testing.T) {
	// Moving a window names its target explicitly and does not deliver input, so a
	// different foreground is irrelevant. Guarding it would break "tidy my windows".
	rec := &recordingHost{}
	guard := &stubGuard{foreground: "hwnd:999"}
	ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec})).WithGuard(guard)

	if err := ex.MoveWindow(context.Background(), "hwnd:111",
		directorapi.Rect{X: 0, Y: 0, Width: 800, Height: 600}); err != nil {
		t.Fatalf("move window: %v", err)
	}
	if guard.calls != 0 {
		t.Fatal("a window move consulted the foreground guard")
	}
}

// ── 13: result mapping ────────────────────────────────────────────────────────

func TestStatusesAreNotCollapsedIntoFailed(t *testing.T) {
	base := map[string]runtime.Host{"OS": &recordingHost{}}

	t.Run("completed", func(t *testing.T) {
		ex := marcoexec.New(marcorunner.New(base))
		res, err := ex.Do(context.Background(), marcoexec.Operation{Kind: marcoexec.KindKey, Chord: "a"})
		if err != nil || res.Status != marcoexec.StatusCompleted {
			t.Fatalf("status = %s, err = %v", res.Status, err)
		}
	})

	t.Run("runtime_failed", func(t *testing.T) {
		rec := &recordingHost{fail: "key"}
		ex := marcoexec.New(marcorunner.New(map[string]runtime.Host{"OS": rec}))
		res, err := ex.Do(context.Background(), marcoexec.Operation{Kind: marcoexec.KindKey, Chord: "a"})
		if err == nil {
			t.Fatal("a failing act was reported as success")
		}
		if res.Status != marcoexec.StatusRuntimeFailed {
			t.Fatalf("status = %s, want runtime_failed — it compiled and ran", res.Status)
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		ex := marcoexec.New(marcorunner.New(base))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		res, _ := ex.Do(ctx, marcoexec.Operation{Kind: marcoexec.KindKey, Chord: "a"})
		if res.Status != marcoexec.StatusCancelled {
			t.Fatalf("status = %s, want cancelled — nothing was wrong, it was stopped", res.Status)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		ex := marcoexec.New(marcorunner.New(base))
		res, _ := ex.Do(context.Background(), marcoexec.Operation{Kind: "teleport"})
		if res.Status != marcoexec.StatusUnsupported {
			t.Fatalf("status = %s, want unsupported", res.Status)
		}
	})
}

// ── drift ─────────────────────────────────────────────────────────────────────

func TestBundledModulesMatchTheOnesCompilationUses(t *testing.T) {
	// buildGraph prefers a sibling <module>.marco over the built-in, so a stale copy
	// in a routes tree SHADOWS the real act surface for everything under it. That is
	// how a capability can be exported, tested, and still unreachable from the routes
	// people actually run. It happened: routes/os.marco was missing KeyDown, KeyUp,
	// Drag, Roll, EightBall and Restore.
	root := filepath.Join("..", "..", "..")
	canonical := map[string]string{
		"os":            mustRead(t, filepath.Join(root, "internal", "osmod", "os.marco")),
		"accessibility": mustRead(t, filepath.Join(root, "internal", "uiamod", "accessibility.marco")),
	}
	found := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		slash := filepath.ToSlash(path)
		base := strings.TrimSuffix(filepath.Base(path), ".marco")
		want, ok := canonical[base]
		if !ok || strings.Contains(slash, "/internal/"+base+"mod/") {
			return nil
		}
		// testdata fixtures are PINNED INPUTS, not bundled copies. A golden test's
		// os.marco declares only the handful of capabilities its program uses, on
		// purpose; making it track the canonical surface would churn golden files on
		// every export with nothing gained. Only copies that are meant to BE the act
		// surface can shadow it.
		if strings.Contains(slash, "/testdata/") {
			return nil
		}
		found++
		if norm(mustRead(t, path)) != norm(want) {
			t.Errorf("%s has drifted from the canonical %s.marco — it shadows the embedded "+
				"surface for every program beside it, so capabilities exported there are "+
				"silently unreachable here", path, base)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	t.Logf("checked %d bundled module copies", found)
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func norm(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n") }

// ── helpers ───────────────────────────────────────────────────────────────────

// recordingHost stands in for the desktop, recording every call in order.
type recordingHost struct {
	calls []runtime.HostCall
	// fail names an action that should resolve failed, to exercise runtime failure.
	fail string
}

func (h *recordingHost) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	h.calls = append(h.calls, c)
	action := strings.ToLower(c.Action)
	if h.fail != "" && action == h.fail {
		return "failed", runtime.ErrVal(&runtime.Err{Message: "the act refused"}), nil
	}
	switch action {
	case "clipboardget":
		set := runtime.NewSet()
		set.Put("Text", runtime.Text("saved"))
		set.Put("IsText", runtime.Bool(true))
		set.Put("Empty", runtime.Bool(false))
		return "ok", runtime.SetVal(set), nil
	case "getvalue", "setvalue":
		set := runtime.NewSet()
		set.Put("Value", runtime.Text("current"))
		return "ok", runtime.SetVal(set), nil
	}
	return "ok", runtime.Absent(), nil
}

func (h *recordingHost) names() []string {
	out := make([]string, 0, len(h.calls))
	for _, c := range h.calls {
		out = append(out, c.Act+"'s "+c.Action)
	}
	return out
}

func wantPoint(t *testing.T, v runtime.Value, x, y int) {
	t.Helper()
	s := v.AsSet()
	if s == nil {
		t.Fatalf("not a Point set: %v", v)
	}
	xv, _ := s.Get("X")
	yv, _ := s.Get("Y")
	if int(num(xv)) != x || int(num(yv)) != y {
		t.Fatalf("point = (%v,%v), want (%d,%d)", xv, yv, x, y)
	}
}

func num(v runtime.Value) float64 {
	n, ok := v.AsNumber()
	if !ok {
		return 0
	}
	return n
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

func TestNothingRenderedForASecretCarriesItsName(t *testing.T) {
	// Every string a person or a log file could see, checked together. The value never
	// reaches this process at all — OS's Secret fetches it inside the host — so what
	// is guarded is the NAME and the source line carrying it.
	const ref = "prod-root-password"
	op := marcoexec.Operation{Kind: marcoexec.KindTypeSecret, SecretRef: ref}

	res, err := marcoexec.Preview(op)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	for what, s := range map[string]string{
		"Describe":     op.Describe(),
		"Source":       res.Source,
		"Summary":      res.Summary(),
		"Diagnostic":   res.Diagnostic,
		"Capabilities": strings.Join(res.Capabilities, ","),
	} {
		if strings.Contains(s, ref) {
			t.Errorf("%s carries the credential name: %q", what, s)
		}
	}
}
