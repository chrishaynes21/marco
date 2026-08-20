package main

// view.go — the VIEW in the overlay's Model/View/Controller split.
//
// The view is an ebiten game that does one thing: render a snapshot of the model
// (model.go). It owns presentation only — window, fonts, theme/colours, layout,
// the wake/idle fade, and the clock. It never decides behaviour; that's the
// controller (controller_*.go) and Marco (the brain).

import (
	"bytes"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
)

// ---- palette (mirrors MacroMarco views/themes.ahk; $MARCO_OVERLAY_THEME picks) ----

func hex(s string) color.RGBA {
	hv := func(c byte) uint8 {
		switch {
		case c >= '0' && c <= '9':
			return c - '0'
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10
		case c >= 'A' && c <= 'F':
			return c - 'A' + 10
		}
		return 0
	}
	b := func(i int) uint8 { return hv(s[i])<<4 | hv(s[i+1]) }
	return color.RGBA{b(0), b(2), b(4), 0xff}
}

// lerp blends a→b by t (0..1), matching View.LerpColor.
func lerp(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}

type theme struct {
	bg, name, cmd, lastCmd, sep, desc color.RGBA
	idle, listen, run, errc           color.RGBA
	logInfo                           color.RGBA
}

var themes = map[string]theme{
	"default": {
		bg: hex("0A0A0A"), name: hex("00E5CC"), cmd: hex("DDDDDD"), lastCmd: hex("4A8A8A"),
		sep: hex("2A4A4A"), desc: hex("6A9A98"),
		idle: hex("A0A8A5"), listen: hex("5AB4F5"), run: hex("4DC94D"), errc: hex("F56565"),
		logInfo: hex("8AC8DE"),
	},
	"dracula": {
		bg: hex("282A36"), name: hex("BD93F9"), cmd: hex("F8F8F2"), lastCmd: hex("6272A4"),
		sep: hex("44475A"), desc: hex("6272A4"),
		idle: hex("9291CD"), listen: hex("8BE9FD"), run: hex("50FA7B"), errc: hex("FF5555"),
		logInfo: hex("8BE9FD"),
	},
	"solarized-dark": {
		bg: hex("002B36"), name: hex("2AA198"), cmd: hex("EEE8D5"), lastCmd: hex("2AA198"),
		sep: hex("0F4A5A"), desc: hex("586E75"),
		idle: hex("6F979B"), listen: hex("268BD2"), run: hex("859900"), errc: hex("DC322F"),
		logInfo: hex("93A1A1"),
	},
	"monokai": {
		bg: hex("272822"), name: hex("A6E22E"), cmd: hex("F8F8F2"), lastCmd: hex("75715E"),
		sep: hex("3E3D32"), desc: hex("75715E"),
		idle: hex("9AA470"), listen: hex("66D9E8"), run: hex("A6E22E"), errc: hex("F92672"),
		logInfo: hex("F8F8F2"),
	},
	"nord": {
		bg: hex("2E3440"), name: hex("88C0D0"), cmd: hex("ECEFF4"), lastCmd: hex("4C566A"),
		sep: hex("434C5E"), desc: hex("4C566A"),
		idle: hex("7287A0"), listen: hex("81A1C1"), run: hex("A3BE8C"), errc: hex("BF616A"),
		logInfo: hex("88C0D0"),
	},
	"tokyo-night": {
		bg: hex("1A1B26"), name: hex("BB9AF7"), cmd: hex("C0CAF5"), lastCmd: hex("565F89"),
		sep: hex("292E42"), desc: hex("565F89"),
		idle: hex("6A6A97"), listen: hex("7AA2F7"), run: hex("9ECE6A"), errc: hex("F7768E"),
		logInfo: hex("7AA2F7"),
	},
	"catppuccin-mocha": {
		bg: hex("1E1E2E"), name: hex("CBA6F7"), cmd: hex("CDD6F4"), lastCmd: hex("6C7086"),
		sep: hex("313244"), desc: hex("7F849C"),
		idle: hex("8E8BAA"), listen: hex("89B4FA"), run: hex("A6E3A1"), errc: hex("F38BA8"),
		logInfo: hex("89DCEB"),
	},
	"gruvbox-dark": {
		bg: hex("282828"), name: hex("FE8019"), cmd: hex("EBDBB2"), lastCmd: hex("7C6F64"),
		sep: hex("3C3836"), desc: hex("928374"),
		idle: hex("A78461"), listen: hex("83A598"), run: hex("B8BB26"), errc: hex("FB4934"),
		logInfo: hex("83A598"),
	},
	"rose-pine": {
		bg: hex("191724"), name: hex("C4A7E7"), cmd: hex("E0DEF4"), lastCmd: hex("6E6A86"),
		sep: hex("26233A"), desc: hex("908CAA"),
		idle: hex("7F76A0"), listen: hex("9CCFD8"), run: hex("31748F"), errc: hex("EB6F92"),
		logInfo: hex("9CCFD8"),
	},
	"light": {
		bg: hex("FAFAFA"), name: hex("6F42C1"), cmd: hex("1A1A2E"), lastCmd: hex("606060"),
		sep: hex("A0A8B0"), desc: hex("3A4A54"),
		idle: hex("505050"), listen: hex("0969DA"), run: hex("1A7F37"), errc: hex("CF222E"),
		logInfo: hex("0969DA"),
	},
}

// th is the active theme. It starts from the saved config and is reassigned live
// by applyTheme when the config editor changes the theme.
var th = themeByName(cfg.Theme)

// stateInfo maps a state to its status word and colour (View.BuildStateLine /
// StatusColor). The symbol is drawn as a vector dot, not a glyph.
func stateInfo(state string) (word string, c color.RGBA) {
	switch state {
	case "listen":
		return "listening", th.listen
	case "run":
		return "running", th.run
	case "ok":
		return "done", th.run
	case "error":
		return "error", th.errc
	default:
		return "ready", th.idle
	}
}

// ---- fonts (Consolas if present, else the embedded Go mono font) ----

var (
	faceClock  *text.GoTextFace // bold, the crown clock
	faceStatus *text.GoTextFace // bold, the state line
	faceBody   *text.GoTextFace // command line / last run
	faceSmall  *text.GoTextFace // day, name, hint, logs
	fontOnce   sync.Once
)

// fontSource loads a Windows terminal font (so the glyphs render like the AHK
// overlay), falling back to the embedded Go mono font off-Windows.
// $MARCO_OVERLAY_FONT overrides the file.
func fontSource(winFile string, fallback []byte) *text.GoTextFaceSource {
	try := func(p string) *text.GoTextFaceSource {
		if p == "" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		s, err := text.NewGoTextFaceSource(bytes.NewReader(b))
		if err != nil {
			return nil
		}
		return s
	}
	if s := try(envStr("MARCO_OVERLAY_FONT", "")); s != nil {
		return s
	}
	if dir := os.Getenv("WINDIR"); dir != "" {
		if s := try(filepath.Join(dir, "Fonts", winFile)); s != nil {
			return s
		}
	}
	s, err := text.NewGoTextFaceSource(bytes.NewReader(fallback))
	if err != nil {
		panic(err)
	}
	return s
}

func initFonts() {
	mono := fontSource("consola.ttf", gomono.TTF)
	bold := fontSource("consolab.ttf", gomonobold.TTF)
	faceClock = &text.GoTextFace{Source: bold, Size: 15}
	faceStatus = &text.GoTextFace{Source: bold, Size: 13}
	faceBody = &text.GoTextFace{Source: mono, Size: 13}
	faceSmall = &text.GoTextFace{Source: mono, Size: 11}
}

// ---- the ebiten view ----

type view struct {
	st                 *model
	tick               int
	alphaBg, alphaText float64
	positioned         bool
}

func (g *view) Update() error {
	if g.st.shouldQuit() {
		return ebiten.Termination
	}
	g.tick++
	if !g.positioned {
		g.positioned = true
		applyWindow() // place on the configured monitor/corner at the configured size
	}

	// Clear the last-command echo + log tail after a minute idle, so the HUD
	// returns to a clean clock/ready state.
	if g.st.sinceActive() > time.Minute {
		g.st.clearText()
	}

	s := g.st.snapshot()
	// Auto-revert a finished run to idle after a few seconds (green "done" → "ready").
	if (s.state == "ok" || s.state == "error") && g.st.sinceActive() > idleAfter {
		g.st.idle()
	}
	// Ease opacity: solid on focus, opaque while active, the configured floor at idle.
	tb, tt := cfgIdle(), textIdle
	switch {
	case s.editing:
		tb, tt = 1.0, textWake // focus → fully solid
	case s.configOn:
		// Live-preview the opacity setting while the editor is open (floored so it
		// stays readable) so the slider visibly does something.
		tb, tt = clampF(cfgIdle(), 0.6, 1.0), textWake
	case s.helpOn || s.teaching || g.st.sinceActive() < idleAfter:
		tb, tt = bgWake, textWake
	}
	g.alphaBg += (tb - g.alphaBg) * 0.18
	g.alphaText += (tt - g.alphaText) * 0.18

	// Mouse passthrough follows the mode.
	//
	// The overlay is click-through by default and must stay that way: it floats over a
	// game, and an overlay that swallowed a click meant for the game would be worse
	// than useless. Inspector is the ONE mode that captures the mouse, it is entered by
	// name, and Esc leaves it.
	//
	// Applied here rather than at the toggle because ebiten window calls belong on the
	// loop that owns the window, and only when the value actually changes — this runs
	// every frame.
	setPassthrough(!s.inspectorOn)

	// Auto-fit the window height to the content (grow for menus/logs, shrink when idle).
	if nh := layoutHeight(s); nh != int(winH.Load()) {
		winH.Store(int32(nh))
		if g.positioned {
			placeWindow(false) // resize + re-anchor to the corner (no monitor switch)
		}
	}
	return nil
}

// layoutHeight mirrors Draw's vertical layout to compute the content height, so
// the window can size to fit. Keep in sync with Draw.
func layoutHeight(s snapshot) int {
	fontOnce.Do(initFonts) // layoutHeight runs in Update, possibly before Draw inits fonts
	mini := cfgMini()
	cw, _ := cfgSize()
	innerW := float64(cw) - 16 // 2*pad (pad=8)
	var y float64
	if mini {
		y = 16 + terminalHeight(s, innerW) // just the (wrapping) command line
		if cfgCoords() && s.curHasPos {
			y += 15 // coords tooltip line
		}
	} else {
		y = 20 + 18 + 15 + 16 // title crown
		if cfgMetrics() {
			y += 15 // cpu/ram widget line
		}
		if cfgCoords() && s.curHasPos {
			y += 15 // coords tooltip line
		}
		y += 12                        // separator
		y += descHeight(s, innerW)     // description / hint line (wraps if long)
		y += 18                        // state line
		y += 12                        // separator
		y += terminalHeight(s, innerW) // terminal line (wraps if long)
	}
	y += teachHeight(s, innerW) // teach prompt + selection, below the command line
	switch {
	case s.configOn:
		y += 8 + menuLineH*float64(configRowCount(s, innerW)) + 18 // sep + rows + footer
	case s.wmode != watchOff:
		y += 8 + watchLineH*float64(watchRowCount(s, innerW))
	case s.insightOn:
		y += 8 + menuLineH*float64(insightRowCount(s, innerW))
		if s.inspectorOn {
			y += menuLineH // the captured-mouse banner
		}
	case s.helpOn:
		y += 8 + menuLineH*float64(helpRowCount(s, innerW))
	case !mini && len(s.history) > 0:
		y += 8 + histLineH*float64(historyRowCount(s, innerW))
	}
	return int(y) + 8 // bottom padding
}

// passthroughOn mirrors the window's current click-through setting.
//
// Tracked so the per-frame call is a comparison rather than a window-system call sixty
// times a second. Starts true because runView installs passthrough before the first frame.
var passthroughOn = true

// setPassthrough changes click-through only when it actually differs.
func setPassthrough(want bool) {
	if want == passthroughOn {
		return
	}
	passthroughOn = want
	ebiten.SetWindowMousePassthrough(want)
}

func (g *view) Layout(int, int) (int, int) { return cfgSize() }

func (g *view) Draw(screen *ebiten.Image) {
	fontOnce.Do(initFonts)
	s := g.st.snapshot()
	if !s.visible {
		return
	}
	bgDrawAlpha = g.alphaBg
	textDrawAlpha = g.alphaText

	cw, chh := cfgSize()
	w := float32(cw)
	h := float32(chh)
	pad := 8.0
	innerW := float64(w) - 2*pad
	cx := float64(w) / 2

	c := cfgSnapshot()
	drawPanel(screen, w, h, c.Border)

	word, scol := stateInfo(s.state)
	blink := ""
	if g.tick/30%2 == 0 {
		blink = "_"
	}

	var y float64
	if c.Mini {
		// Mini mode — just the command line.
		y = 16
		drawTerminalLine(screen, pad, y, innerW, s, scol, blink)
		y += terminalHeight(s, innerW)
		if c.Coords && s.curHasPos {
			drawText(screen, fmt.Sprintf("x %d   y %d", s.curX, s.curY), pad, y, faceSmall, th.listen)
			y += 15
		}
	} else {
		// --- Title crown: clock / date / name, centred. ---
		now := time.Now()
		y = 20
		centerText(screen, now.Format("3:04 PM"), cx, y, faceClock, th.name)
		y += 18
		centerText(screen, now.Format("Monday, Jan 2"), cx, y, faceSmall, lerp(th.bg, th.idle, 0.55))
		y += 15
		centerText(screen, "Marco", cx, y, faceSmall, lerp(th.bg, th.idle, 0.38))
		y += 16

		// --- Optional system widget: CPU / RAM. ---
		if c.Metrics {
			centerText(screen, fmt.Sprintf("cpu %.0f%%   ram %.0f%%", s.cpu, s.ram), cx, y, faceSmall, th.desc)
			y += 15
		}

		// --- Optional live cursor-coords tooltip (handy for teaching, esp. multi-monitor). ---
		if c.Coords && s.curHasPos {
			centerText(screen, fmt.Sprintf("x %d   y %d", s.curX, s.curY), cx, y, faceSmall, th.listen)
			y += 15
		}

		sep(screen, pad, y, innerW)
		y += 12

		// --- Description / idle hint line. Wraps to the panel width so a long teach
		// prompt (e.g. `Save this as "…"? [y]/[n]/[s]`) is shown in full, not cut off. ---
		isHint := !s.editing && (s.state == "idle" || s.state == "listen")
		dy := y
		for _, ln := range descRows(s, innerW) {
			col := th.desc
			if isHint {
				col = lerp(th.bg, th.idle, 0.4)
			}
			drawText(screen, ln, pad, dy, faceSmall, col)
			dy += descLineH
		}
		y += descHeight(s, innerW)

		// --- State line: dot + "app  word". ---
		stateDot(screen, pad+4, y+5, scol)
		drawText(screen, s.app+"  "+word, pad+16, y, faceStatus, scol)
		y += 18

		sep(screen, pad, y, innerW)
		y += 12

		// --- Terminal line: live typing / leader echo / last command + result. ---
		drawTerminalLine(screen, pad, y, innerW, s, scol, blink)
		y += terminalHeight(s, innerW)
	}

	// --- Teach transcript, terminal-style: directly BELOW the command. The recording
	// timer, then each answered question + your selection, then the active prompt and
	// what you've typed so far (type y/n/s, then Enter). ---
	if rows := teachRows(s, innerW); len(rows) > 0 {
		y += 6
		for _, ln := range rows {
			col := th.desc
			if strings.HasPrefix(ln, "› ") || strings.HasPrefix(ln, "● ") {
				col = th.cmd // selection / timer lines, brighter
			}
			drawText(screen, ln, pad, y, faceSmall, col)
			y += menuLineH
		}
	}

	// --- Config editor / help menu / log tail (logs only in full mode). ---
	switch {
	case s.configOn:
		sep(screen, pad, y-4, innerW)
		y += 8
		for i, line := range s.config {
			col := th.desc
			prefix := "  "
			if i == s.configSel {
				col, prefix = th.cmd, "› " // selected row
			}
			for _, ln := range wrapText(prefix+line, faceSmall, innerW) {
				drawText(screen, ln, pad, y, faceSmall, col)
				y += menuLineH
			}
		}
		drawText(screen, "↑↓ pick  ←→ change  S save  Esc", pad, y+2, faceSmall, lerp(th.bg, th.idle, 0.4))
	case s.wmode != watchOff:
		// The Director's playbill: WATCH, or DIAGNOSTICS. One account, two readings —
		// and the status chip at its top is the same one word a consumer surface would
		// show, from the same value. See watchview.go.
		sep(screen, pad, y-4, innerW)
		y += 8
		drawWatch(screen, pad, y, innerW, s)
	case s.insightOn:
		// The Director insight panel. Colour carries the reading: a warning row is the
		// point of the panel and must not look like the rest of the table.
		sep(screen, pad, y-4, innerW)
		y += 8
		if s.inspectorOn {
			// Capturing the mouse must never be a silent state. If the overlay is
			// eating clicks, the reason has to be on screen — and so does the way out.
			drawText(screen, "INSPECTOR — mouse captured — Esc to release",
				pad, y, faceSmall, th.listen)
			y += menuLineH
		}
		for _, line := range s.insight {
			col := th.desc
			switch {
			case strings.HasPrefix(line, "director"):
				col = th.idle
			case strings.Contains(line, "<- none"),
				strings.Contains(line, "PROVENANCE INCOMPLETE"),
				strings.Contains(line, "degraded:"),
				strings.Contains(line, "failed:"),
				strings.Contains(line, "not running"),
				strings.Contains(line, "unreachable"):
				col = th.errc
			}
			for _, ln := range wrapText(line, faceSmall, innerW) {
				drawText(screen, ln, pad, y, faceSmall, col)
				y += menuLineH
			}
		}
	case s.helpOn:
		sep(screen, pad, y-4, innerW)
		y += 8
		for _, line := range s.help {
			col := th.desc
			if strings.HasSuffix(line, ":") || strings.HasPrefix(line, "`") {
				col = th.idle
			}
			for _, ln := range wrapText(line, faceSmall, innerW) {
				drawText(screen, ln, pad, y, faceSmall, col)
				y += menuLineH
			}
		}
	case !c.Mini && len(s.history) > 0:
		// Command history: each past command with its outcome icon + time,
		// right-justified (✓ green / ■ grey / ✗ red), fading with the text. The
		// command wraps; the time + icon sit on its first row.
		sep(screen, pad, y-4, innerW)
		y += 8
		right := pad + innerW
		for _, e := range slices.Backward(s.history) { // newest on top (a stack)

			col := resultColor(e.outcome)
			t := dur(e.dur)
			tw, _ := text.Measure(t, faceSmall, 0)
			rows := wrapText(e.cmd, faceSmall, innerW-histReserve)
			for j, ln := range rows {
				drawText(screen, ln, pad, y, faceSmall, th.lastCmd)
				if j == 0 {
					drawText(screen, t, right-tw, y, faceSmall, col)         // time, right-justified
					drawResultIcon(screen, right-tw-10, y+5, e.outcome, col) // vector icon left of it
				}
				y += histLineH
			}
		}
	}
}

// marcoVerbs are the command words highlighted in the command line (the same
// accent as the `m leader badge), so a marco command reads distinctly from a
// plain route name.
var marcoVerbs = map[string]bool{
	"teach": true, "simplify": true, "forget": true, "rename": true,
	"bind": true, "unbind": true, "config": true, "help": true,
	"exit": true, "quit": true,
}

// drawTerminalLine renders the "> input_" / "> `h" / "> lastcmd" line (live
// typing, leader echo, voice preview, or the last command). The per-command
// outcome + time lives in the history list below, not here.
func drawTerminalLine(dst *ebiten.Image, pad, y, innerW float64, s snapshot, scol color.RGBA, blink string) {
	// In command mode the leader shows as a tight `m badge in the accent colour.
	// The input renders flush against it — no auto-inserted gap; the separating
	// space is one you type (`m + space + command), shown verbatim.
	if s.editing {
		bw := drawText(dst, "`m", pad, y, faceBody, scol) // accent badge
		// Render the typed command with the verb accented and the arg labels woven into
		// the "with" clause, wrapping across rows under the badge.
		drawEditLine(dst, s.input, blink, s.argHints, pad+bw, y, innerW-bw, scol)
		return
	}
	bw := drawText(dst, "> ", pad, y, faceBody, scol)
	ix := pad + bw
	avail := innerW - bw // width left of the "> " prefix for the (wrapping) text
	wrapped := func(str string, col color.RGBA) {
		for i, ln := range wrapText(str, faceBody, avail) {
			drawText(dst, ln, ix, y+float64(i)*termContH, faceBody, col)
		}
	}
	switch {
	case s.leaderEcho != "":
		drawText(dst, s.leaderEcho+blink, ix, y, faceBody, scol)
	case s.heard != "":
		wrapped(s.heard, th.listen) // live voice
	case s.lastRun != "":
		wrapped(s.lastRun, th.lastCmd)
	}
}

// terminalRows is how many rows the terminal line needs once wrapped. Editing,
// the leader echo, and the empty state are always one row; voice/last-command
// echoes wrap to the panel width.
func terminalRows(s snapshot, innerW float64) int {
	if s.leaderEcho != "" {
		return 1
	}
	if s.editing {
		bw, _ := text.Measure("`m", faceBody, 0)
		// Count rows the way drawEditLine lays them out (the labels widen the line, so a
		// plain wrapText of the raw input would under-count).
		_, row := layoutRuns(editRuns(s.input, s.argHints, th.name), 0, innerW-bw, nil)
		return row + 1
	}
	bw, _ := text.Measure("> ", faceBody, 0)
	avail := innerW - bw
	switch {
	case s.heard != "":
		return len(wrapText(s.heard, faceBody, avail))
	case s.lastRun != "":
		return len(wrapText(s.lastRun, faceBody, avail))
	}
	return 1
}

// terminalHeight is the terminal line's vertical space: the original single-row
// 20px, plus termContH for each extra wrapped row. Shared by Draw + layoutHeight.
func terminalHeight(s snapshot, innerW float64) float64 {
	return 20 + float64(terminalRows(s, innerW)-1)*termContH
}

// editRun is one colored stretch of the command line (verb, plain text, or an
// arg-name label). editRuns flattens the typed command into these; layoutRuns then
// wraps and draws them so a long "with" clause flows across rows like plain text.
type editRun struct {
	s   string
	col color.RGBA
}

// editRuns flattens the command being typed into colored runs: leading spaces and
// non-verb words in th.cmd, a recognized verb in the accent colour, and — once a
// "with" clause is present — each positional value behind its colored arg-name label
// ("say hello with name:chris"). The labels are a visual guide only; the command
// sent to marco stays "say hello with chris".
func editRuns(in string, names []string, accent color.RGBA) []editRun {
	var rs []editRun
	addCmd := func(part string) { // verb-accent the first word, rest in th.cmd
		trimmed := strings.TrimLeft(part, " ")
		if lead := part[:len(part)-len(trimmed)]; lead != "" {
			rs = append(rs, editRun{lead, th.cmd})
		}
		first, rest, hasSpace := strings.Cut(trimmed, " ")
		if marcoVerbs[strings.ToLower(first)] {
			rs = append(rs, editRun{first, accent})
			if hasSpace {
				rs = append(rs, editRun{" " + rest, th.cmd})
			}
		} else if trimmed != "" {
			rs = append(rs, editRun{trimmed, th.cmd})
		}
	}
	idx := strings.Index(strings.ToLower(in), " with ")
	if idx < 0 {
		addCmd(in)
		return rs
	}
	addCmd(in[:idx])
	rs = append(rs, editRun{" with ", th.cmd})
	for i, seg := range strings.Split(in[idx+len(" with "):], ",") {
		if i > 0 {
			rs = append(rs, editRun{", ", th.cmd})
		}
		if i < len(names) { // colored label in front of the value the user typed
			rs = append(rs, editRun{names[i] + ":", th.listen})
		}
		rs = append(rs, editRun{strings.TrimSpace(seg), th.cmd})
	}
	return rs
}

// layoutRuns places styled runs left-to-right in faceBody, word-wrapping on spaces
// within avail (a leading space after a wrap is dropped). emit draws each placed atom
// at its (x, row); pass nil to only measure. Returns the end x and final row index,
// so the caller can place the cursor / a trailing hint.
func layoutRuns(rs []editRun, x0, avail float64, emit func(s string, col color.RGBA, x float64, row int)) (endX float64, row int) {
	cur := x0
	for _, r := range rs {
		for i := 0; i < len(r.s); {
			isSpace := r.s[i] == ' '
			j := i
			for j < len(r.s) && (r.s[j] == ' ') == isSpace {
				j++
			}
			seg := r.s[i:j]
			i = j
			w, _ := text.Measure(seg, faceBody, 0)
			if !isSpace && cur > x0 && cur-x0+w > avail { // word overflows → next row
				row++
				cur = x0
			}
			if isSpace && cur == x0 && row > 0 { // drop a leading space only after a wrap
				continue
			}
			if emit != nil {
				emit(seg, r.col, cur, row)
			}
			cur += w
		}
	}
	return cur, row
}

// drawEditLine renders the command being typed, wrapping across rows from (x, y).
// Before a "with" clause exists, the arg labels trail as a "name:  (tab)" suggestion;
// once it does, they sit in front of each value (see editRuns).
func drawEditLine(dst *ebiten.Image, in, blink string, names []string, x, y, avail float64, accent color.RGBA) {
	rs := editRuns(in, names, accent)
	endX, row := layoutRuns(rs, x, avail, func(s string, col color.RGBA, ax float64, r int) {
		drawText(dst, s, ax, y+float64(r)*termContH, faceBody, col)
	})
	ry := y + float64(row)*termContH
	drawText(dst, blink, endX, ry, faceBody, th.cmd)
	if !strings.Contains(strings.ToLower(in), " with ") {
		drawArgSuggestion(dst, names, endX, ry) // prompt to start the args clause
	}
}

// drawArgSuggestion renders the arg labels as a trailing "name:  (tab)" suggestion,
// shown before a "with" clause exists to prompt adding arguments.
func drawArgSuggestion(dst *ebiten.Image, names []string, x, y float64) {
	if len(names) == 0 {
		return
	}
	drawText(dst, "  "+strings.Join(names, ": ")+":  (tab)", x, y, faceSmall, th.listen)
}

// drawResultIcon draws the outcome glyph as a VECTOR (no font dependency — the
// ✓/■/✗ glyphs aren't in every mono font and render as tofu): a check, a square,
// or a cross, centred at (cx, cy).
func drawResultIcon(dst *ebiten.Image, cx, cy float64, outcome string, col color.RGBA) {
	c := aText(col)
	x, y := float32(cx), float32(cy)
	switch outcome {
	case "ok": // check
		vector.StrokeLine(dst, x-4, y, x-1, y+3, 1.6, c, true)
		vector.StrokeLine(dst, x-1, y+3, x+4, y-4, 1.6, c, true)
	case "failed": // cross
		vector.StrokeLine(dst, x-3, y-3, x+3, y+3, 1.6, c, true)
		vector.StrokeLine(dst, x-3, y+3, x+3, y-3, 1.6, c, true)
	case "canceled": // square
		vector.FillRect(dst, x-3, y-3, 6, 6, c, true)
	}
}

func resultColor(outcome string) color.RGBA {
	switch outcome {
	case "ok":
		return th.run
	case "canceled":
		return th.idle
	case "failed":
		return th.errc
	}
	return th.lastCmd
}

// dur formats a route's run time ("4.2s", "1:05").
func dur(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	s := int(d.Seconds())
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// Two draw alphas per frame: the panel fill/seps fade to nothing at idle (no box,
// no border, no background bleed), while the text fades only partway so the
// clock/status keep floating faintly.
var (
	bgDrawAlpha   = 1.0
	textDrawAlpha = 1.0
)

func aBg(c color.RGBA) color.RGBA   { c.A = uint8(float64(c.A) * bgDrawAlpha); return c }
func aText(c color.RGBA) color.RGBA { c.A = uint8(float64(c.A) * textDrawAlpha); return c }

// drawText draws s at (x,y) and returns its advance width, so callers can place
// the next run flush against it.
func drawText(dst *ebiten.Image, s string, x, y float64, face *text.GoTextFace, c color.RGBA) float64 {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(aText(c))
	text.Draw(dst, s, face, op)
	w, _ := text.Measure(s, face, 0)
	return w
}

func centerText(dst *ebiten.Image, s string, cx, y float64, face *text.GoTextFace, c color.RGBA) {
	tw, _ := text.Measure(s, face, 0)
	drawText(dst, s, cx-tw/2, y, face, c)
}

// stateDot draws the status LED (a small vector circle — no font glyph needed),
// sized/placed to sit on the text line.
func stateDot(dst *ebiten.Image, x, y float64, c color.RGBA) {
	vector.FillCircle(dst, float32(x), float32(y), 3, aText(c), true)
}

func sep(dst *ebiten.Image, x, y, w float64) {
	vector.StrokeLine(dst, float32(x), float32(y), float32(x+w), float32(y), 1, aBg(th.sep), false)
}

// roundRectPath appends a rounded-rectangle subpath.
func roundRectPath(p *vector.Path, x, y, w, h, r float32) {
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.ArcTo(x+w, y, x+w, y+r, r)
	p.LineTo(x+w, y+h-r)
	p.ArcTo(x+w, y+h, x+w-r, y+h, r)
	p.LineTo(x+r, y+h)
	p.ArcTo(x, y+h, x, y+h-r, r)
	p.LineTo(x, y+r)
	p.ArcTo(x, y, x+r, y, r)
	p.Close()
}

// drawPanel fills the rounded panel (one path, so translucent fills don't seam)
// and, when the border config is on, strokes an accent outline over it in a
// single pass (a stroke, not an inset fill, so it doesn't compound at low alpha).
func drawPanel(dst *ebiten.Image, w, h float32, border bool) {
	var p vector.Path
	roundRectPath(&p, 0, 0, w, h, 10)
	fop := &vector.DrawPathOptions{AntiAlias: true}
	fop.ColorScale.ScaleWithColor(aBg(th.bg))
	vector.FillPath(dst, &p, &vector.FillOptions{}, fop)
	if border {
		var s vector.Path
		s.AddStroke(&p, &vector.AddStrokeOptions{StrokeOptions: vector.StrokeOptions{Width: 1.5}})
		bop := &vector.DrawPathOptions{AntiAlias: true}
		bop.ColorScale.ScaleWithColor(aBg(lerp(th.name, th.bg, 0.6)))
		vector.FillPath(dst, &s, &vector.FillOptions{}, bop)
	}
}

// Per-row advances for the (now wrapping) panels, plus the history time/icon
// reserve. descLineH/menuLineH match the old single-line spacing so unwrapped
// content sits exactly where it did; termContH is the gap between extra terminal
// rows (faceBody is larger than the menu's faceSmall).
const (
	descLineH   = 14.0 // description / log line (faceSmall)
	menuLineH   = 14.0 // help / config rows (faceSmall)
	histLineH   = 15.0 // command-history rows (faceSmall)
	termContH   = 18.0 // extra wrapped rows of the terminal line (faceBody)
	histReserve = 54.0 // right-side width kept for a history row's run time + icon
)

// configRowCount / helpRowCount / historyRowCount return the total wrapped row
// count for each panel, so layoutHeight grows the window to fit. They mirror the
// wrapping Draw does — keep them in sync.
func configRowCount(s snapshot, innerW float64) int {
	n := 0
	for i, line := range s.config {
		prefix := "  "
		if i == s.configSel {
			prefix = "› "
		}
		n += len(wrapText(prefix+line, faceSmall, innerW))
	}
	return n
}

func helpRowCount(s snapshot, innerW float64) int {
	n := 0
	for _, line := range s.help {
		n += len(wrapText(line, faceSmall, innerW))
	}
	return n
}

func insightRowCount(s snapshot, innerW float64) int {
	n := 0
	for _, line := range s.insight {
		n += len(wrapText(line, faceSmall, innerW))
	}
	return n
}

func historyRowCount(s snapshot, innerW float64) int {
	n := 0
	for _, e := range s.history {
		n += len(wrapText(e.cmd, faceSmall, innerW-histReserve))
	}
	return n
}

// descRows returns the rows to draw on the description/log line: the idle hint, or
// the latest log word-wrapped to innerW (so a long teach prompt isn't truncated).
func descRows(s snapshot, innerW float64) []string {
	if !s.editing && (s.state == "idle" || s.state == "listen") {
		// The NORMAL reading, on the always-visible line. This is the consumer surface:
		// one word, and one sentence when there is one. It is a reduction of the SAME
		// account the Watch panel expands — see model.setWatch — so the two cannot
		// disagree about whether Marco has a question.
		//
		// Only when there is something to say. "Ready" is not news, and a hint line that
		// restated it forever would train a person to stop reading the one line that
		// eventually tells them Marco is waiting on them.
		if row := normalRow(s); row != "" {
			return wrapText(row, faceSmall, innerW)
		}
		return []string{"`m watch  ·  `m ui  ·  `m help  ·  `m config"}
	}
	if len(s.logs) > 0 {
		return wrapText(s.logs[len(s.logs)-1], faceSmall, innerW)
	}
	return nil
}

// teachRows are the lines of the teach session, rendered BELOW the command line
// like a terminal: a live recording timer (while demonstrating, before any prompt),
// then the answered question + selection transcript, then the active prompt and the
// answer you've typed so far (type y/n/s, then Enter). Empty when not teaching.
func teachRows(s snapshot, innerW float64) []string {
	if !s.teaching && len(s.teachLog) == 0 && s.teachPrompt == "" {
		return nil
	}
	var rows []string
	// While recording (no prompt up yet), show how long you've been demonstrating.
	if s.teaching && s.teachPrompt == "" && len(s.teachLog) == 0 {
		el := time.Since(s.teachStart)
		rows = append(rows, fmt.Sprintf("● rec  %d:%02d", int(el.Minutes()), int(el.Seconds())%60))
	}
	for _, line := range s.teachLog { // answered Q + "› answer" pairs
		rows = append(rows, wrapText(line, faceSmall, innerW)...)
	}
	if s.teachPrompt != "" {
		rows = append(rows, wrapText(s.teachPrompt, faceSmall, innerW)...)
		rows = append(rows, "› "+s.teachPending+"_") // what you've typed; press Enter to send
	}
	return rows
}

// teachHeight is the vertical space the teach prompt block occupies below the
// command line (a small gap + one menuLineH per row); 0 when no prompt is up.
func teachHeight(s snapshot, innerW float64) float64 {
	if n := len(teachRows(s, innerW)); n > 0 {
		return 6 + float64(n)*menuLineH
	}
	return 0
}

// descHeight is the vertical space the description/log line occupies — the original
// single-line 16px, plus one descLineH for each extra wrapped row. Shared by Draw
// and layoutHeight so the window grows to fit a wrapped prompt.
func descHeight(s snapshot, innerW float64) float64 {
	if n := len(descRows(s, innerW)); n > 1 {
		return 16 + float64(n-1)*descLineH
	}
	return 16
}

// wrapText breaks s into lines that each fit within maxWidth for face, splitting on
// spaces; a single word wider than the line is hard-broken by runes. Existing
// newlines start new lines. Returns at least one (possibly empty) line.
func wrapText(s string, face *text.GoTextFace, maxWidth float64) []string {
	fits := func(str string) bool {
		w, _ := text.Measure(str, face, 0)
		return w <= maxWidth
	}
	var lines []string
	for para := range strings.SplitSeq(s, "\n") {
		cur := ""
		for word := range strings.FieldsSeq(para) {
			switch {
			case cur == "":
				cur = word
			case fits(cur + " " + word):
				cur += " " + word
			default:
				lines = append(lines, cur)
				cur = word
			}
			// Hard-break a single token that overflows the line on its own.
			for !fits(cur) && len([]rune(cur)) > 1 {
				r := []rune(cur)
				i := len(r)
				for i > 1 && !fits(string(r[:i])) {
					i--
				}
				lines = append(lines, string(r[:i]))
				cur = string(r[i:])
			}
		}
		if cur != "" {
			lines = append(lines, cur)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
