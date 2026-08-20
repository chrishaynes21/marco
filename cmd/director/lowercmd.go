package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
)

// `director lower` — what Marco would the Director actually run?
//
//	director lower "click the save button"
//	director lower "click the save button" --json
//
// It NEVER executes. That is the whole value: the question it answers is "what is
// about to happen to my computer", and a command that answered it by doing the thing
// would be useless for the one moment you most want to ask.
//
// It also does not observe or resolve, which is why it takes an operation description
// rather than driving the planner: resolving a target means walking the accessibility
// tree of whatever is in front, and a preview that quietly did that would be a
// different, heavier command than it looks.
func runLower(args []string) int {
	fs := flag.NewFlagSet("lower", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the lowering as JSON")
	recent := fs.Bool("recent", false, "show the operations that were actually lowered and run")
	limit := fs.Int("n", 10, "how many recent lowerings to show")
	start := fs.Bool("start", false, "start the service if it is not running")
	_ = fs.Parse(flagsFirst(args))

	if *recent {
		return lowerRecent(*jsonOut, *limit, *start)
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "director lower: needs an operation, e.g.")
		fmt.Fprintln(os.Stderr, `  director lower key ctrl+s`)
		fmt.Fprintln(os.Stderr, `  director lower click 912 704`)
		fmt.Fprintln(os.Stderr, `  director lower type "hello world"`)
		fmt.Fprintln(os.Stderr, `  director lower secret my-login`)
		fmt.Fprintln(os.Stderr, `  director lower --recent`)
		return 2
	}

	op, err := operationFromArgs(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director lower: %v\n", err)
		return 2
	}

	// Preview lowers and stops. No compile, no runtime, no host.
	res, err := marcoexec.Preview(op)
	if *jsonOut {
		return printJSON(res)
	}
	fmt.Print(renderLowering(res))
	if err != nil {
		return 1
	}
	return 0
}

// operationFromArgs builds an Operation from the command line.
//
// A small, explicit vocabulary rather than the intent parser: this command is for
// inspecting the LOWERING, and routing it through target resolution would make what
// you see depend on what happened to be on screen.
func operationFromArgs(args []string) (marcoexec.Operation, error) {
	switch strings.ToLower(args[0]) {
	case "key":
		if len(args) < 2 {
			return marcoexec.Operation{}, fmt.Errorf("key needs a chord")
		}
		return marcoexec.Operation{Kind: marcoexec.KindKey, Chord: args[1]}, nil
	case "type":
		if len(args) < 2 {
			return marcoexec.Operation{}, fmt.Errorf("type needs some text")
		}
		return marcoexec.Operation{Kind: marcoexec.KindType, Text: strings.Join(args[1:], " ")}, nil
	case "secret":
		if len(args) < 2 {
			return marcoexec.Operation{}, fmt.Errorf("secret needs a credential name")
		}
		return marcoexec.Operation{Kind: marcoexec.KindTypeSecret, SecretRef: args[1]}, nil
	case "click":
		x, y, err := twoInts(args[1:], "click")
		if err != nil {
			return marcoexec.Operation{}, err
		}
		op := marcoexec.Operation{Kind: marcoexec.KindClick, At: marcoexec.Point{X: x, Y: y}}
		if len(args) > 3 {
			op.Button = args[3]
		}
		return op, nil
	case "move":
		x, y, err := twoInts(args[1:], "move")
		if err != nil {
			return marcoexec.Operation{}, err
		}
		return marcoexec.Operation{Kind: marcoexec.KindMove, At: marcoexec.Point{X: x, Y: y}}, nil
	case "activate":
		if len(args) < 2 {
			return marcoexec.Operation{}, fmt.Errorf("activate needs an application")
		}
		return marcoexec.Operation{Kind: marcoexec.KindActivate, App: args[1]}, nil
	case "launch":
		if len(args) < 2 {
			return marcoexec.Operation{}, fmt.Errorf("launch needs something to launch")
		}
		return marcoexec.Operation{Kind: marcoexec.KindLaunch, Target: strings.Join(args[1:], " ")}, nil
	case "windowstate":
		if len(args) < 3 {
			return marcoexec.Operation{}, fmt.Errorf("windowstate needs a window and a state")
		}
		return marcoexec.Operation{Kind: marcoexec.KindWindowState, Window: args[1], State: args[2]}, nil
	case "movewindow":
		if len(args) < 6 {
			return marcoexec.Operation{}, fmt.Errorf("movewindow needs a window, x, y, w and h")
		}
		x, y, err := twoInts(args[2:], "movewindow")
		if err != nil {
			return marcoexec.Operation{}, err
		}
		w, h, err := twoInts(args[4:], "movewindow")
		if err != nil {
			return marcoexec.Operation{}, err
		}
		return marcoexec.Operation{Kind: marcoexec.KindMoveWindow, Window: args[1],
			Bounds: marcoexec.Rect{X: x, Y: y, W: w, H: h}}, nil
	case "clipboard":
		return marcoexec.Operation{Kind: marcoexec.KindClipboardGet}, nil
	}
	return marcoexec.Operation{}, fmt.Errorf("%q is not an operation this command knows", args[0])
}

func twoInts(args []string, what string) (int, int, error) {
	if len(args) < 2 {
		return 0, 0, fmt.Errorf("%s needs an x and a y", what)
	}
	var x, y int
	if _, err := fmt.Sscanf(args[0], "%d", &x); err != nil {
		return 0, 0, fmt.Errorf("%q is not a coordinate", args[0])
	}
	if _, err := fmt.Sscanf(args[1], "%d", &y); err != nil {
		return 0, 0, fmt.Errorf("%q is not a coordinate", args[1])
	}
	return x, y, nil
}

// lowerRecent shows what was actually lowered and run.
func lowerRecent(jsonOut bool, limit int, start bool) int {
	c, err := connect(start)
	if err != nil {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Println("Director: not running")
		}
		return 1
	}
	defer c.Close()

	results, err := c.Lowerings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if limit > 0 && len(results) > limit {
		results = results[len(results)-limit:]
	}
	if jsonOut {
		return printJSON(results)
	}
	if len(results) == 0 {
		fmt.Println("Nothing has been lowered yet.")
		return 0
	}
	for i, r := range results {
		if i > 0 {
			fmt.Println()
		}
		fmt.Print(renderLowering(r))
	}
	return 0
}

// renderLowering describes one operation, its Marco, and what became of it.
func renderLowering(r marcoexec.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", r.Operation.Kind, r.Operation.Describe())
	if len(r.Capabilities) > 0 {
		fmt.Fprintf(&b, "  capability   %s\n", strings.Join(r.Capabilities, ", "))
	}
	if r.Status != "" {
		fmt.Fprintf(&b, "  status       %s\n", r.Status)
	}
	if r.Diagnostic != "" {
		fmt.Fprintf(&b, "  diagnostic   %s\n", r.Diagnostic)
	}
	if r.Source != "" {
		// Already redacted by marcoexec before it was stored — this renderer never
		// sees a credential name, so it cannot print one.
		b.WriteString("  marco\n")
		for _, line := range strings.Split(strings.TrimRight(r.Source, "\n"), "\n") {
			b.WriteString("    " + line + "\n")
		}
	}
	return b.String()
}

// `director op` — execute ONE operation through the ordinary path.
//
//	director op launch notepad
//	director op activate notepad
//
// It exists because several operations have no spoken phrase — launching, activating,
// setting a window state — and "validate with the shipped binary" needs a way to
// reach them. It is NOT a bypass: it submits a typed Operation, and the service runs
// it through the same executor, the same foreground guard and the same compiler a
// planned action uses. A client cannot submit Marco source, only an Operation.
func runOp(args []string) int {
	fs := flag.NewFlagSet("op", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print the result as JSON")
	start := fs.Bool("start", false, "start the service if it is not running")
	// --window names the INTENDED target, and supplying it is what arms the foreground
	// guard. An operation with no window means "whatever has focus", which is a real
	// instruction rather than something to refuse.
	window := fs.String("window", "", "the intended window, hwnd:<handle>")
	_ = fs.Parse(flagsFirst(args))

	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(os.Stderr, "director op: needs an operation, e.g. `director op launch notepad`")
		return 2
	}
	op, err := operationFromArgs(rest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director op: %v\n", err)
		return 2
	}
	if *window != "" {
		op.Window = *window
	}

	c, err := connect(*start)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Director: not running")
		return 3
	}
	defer c.Close()

	res, err := c.RunOperation(op)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(res)
	}
	fmt.Print(renderLowering(res))
	if res.Failed() {
		return 1
	}
	return 0
}
