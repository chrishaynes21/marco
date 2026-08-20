package voicelearn

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// Env is the live OS surface a Session reads while narrating: where the cursor is,
// the active window's origin (so clicks record window-relative), and screen
// captures for anchors / wait-for-screen. Injectable so the Session is unit-testable
// with a fake, and real on Windows.
type Env interface {
	Cursor() (x, y int)                     // current cursor, virtual-desktop pixels
	WindowOrigin() (left, top int, ok bool) // foreground window's top-left
	AnchorAt(x, y int) []byte               // PNG template around (x,y); nil if not worth it
	Screenshot() []byte                     // PNG of a distinctive screen region; nil if unavailable
}

// Session accumulates macroir Steps as narration commands arrive.
type Session struct {
	env      Env
	steps    []macroir.Step
	argNames []string // declared args (from the learn phrase's "with name, …" clause)
}

// New starts an empty voice-learn session reading the live OS through env. argNames
// are the declared argument names, so narrating "type person" (or "type the first
// argument") drops the matching {{person}} placeholder instead of literal text.
func New(env Env, argNames ...string) *Session {
	return &Session{env: env, argNames: argNames}
}

// resolveArg maps a Type value onto a declared argument placeholder: "{{N}}" (from
// "type arg N") → the Nth declared name; a bare declared-arg name ("type person")
// → "{{person}}"; anything else is literal text.
func (s *Session) resolveArg(text string) string {
	t := strings.TrimSpace(text)
	if strings.HasPrefix(t, "{{") && strings.HasSuffix(t, "}}") {
		if n, err := strconv.Atoi(t[2 : len(t)-2]); err == nil && n >= 1 && n <= len(s.argNames) {
			return "{{" + s.argNames[n-1] + "}}"
		}
		return text
	}
	for _, a := range s.argNames {
		if strings.EqualFold(t, a) {
			return "{{" + a + "}}"
		}
	}
	return text
}

// Steps returns the macro built so far.
func (s *Session) Steps() []macroir.Step { return s.steps }

// Apply records the step(s) for one parsed command and returns a short status line
// for the HUD, plus whether the session should finish (Done) or be abandoned
// (Cancel). An unrecognized command records nothing and asks for a rephrase.
func (s *Session) Apply(c Cmd) (status string, done, canceled bool) {
	switch c.Kind {
	case Click:
		x, y := s.env.Cursor()
		s.steps = append(s.steps, s.clickStep(x, y, nil))
		return fmt.Sprintf("click (%d, %d)", x, y), false, false
	case Anchor:
		x, y := s.env.Cursor()
		s.steps = append(s.steps, s.clickStep(x, y, s.env.AnchorAt(x, y)))
		return fmt.Sprintf("anchor + click (%d, %d)", x, y), false, false
	case ClickText:
		// Locate the spoken word on screen (OCR) and click it — for a target that moves.
		// The cursor position is recorded as the fallback if the text isn't found / no
		// OCR host is wired, so position the cursor over the target while you say it.
		x, y := s.env.Cursor()
		st := s.clickStep(x, y, nil)
		st.AnchorText = c.Text
		s.steps = append(s.steps, st)
		return fmt.Sprintf("click the text %q", c.Text), false, false
	case WaitScreen:
		img := s.env.Screenshot()
		if len(img) == 0 {
			return "couldn't capture the screen — try again", false, false
		}
		s.steps = append(s.steps, macroir.Step{Kind: macroir.StepFind, Template: img})
		return "wait for this screen", false, false
	case Wait:
		s.steps = append(s.steps, macroir.Step{Kind: macroir.StepWait, Ms: c.Ms})
		return fmt.Sprintf("wait %dms", c.Ms), false, false
	case Type:
		text := s.resolveArg(c.Text)
		s.steps = append(s.steps, macroir.Step{Kind: macroir.StepType, Text: text})
		return fmt.Sprintf("type %q", text), false, false
	case Key:
		s.steps = append(s.steps, macroir.Step{Kind: macroir.StepKey, Key: c.Key, Count: 1})
		return "press " + c.Key, false, false
	case Activate:
		s.steps = append(s.steps, macroir.Step{Kind: macroir.StepActivate, Text: c.App})
		return "activate " + c.App, false, false
	case Undo:
		if len(s.steps) == 0 {
			return "nothing to undo", false, false
		}
		s.steps = s.steps[:len(s.steps)-1]
		return "undid the last step", false, false
	case Done:
		return "done", true, false
	case Cancel:
		return "canceled", false, true
	default:
		return "didn't catch that — try \"click\", \"type …\", \"wait for this screen\", \"done\"", false, false
	}
}

// clickStep builds a click at (x,y), window-relative when there's a foreground
// window (so it follows the app across monitors), optionally carrying an image
// template for an image-find click.
func (s *Session) clickStep(x, y int, tmpl []byte) macroir.Step {
	st := macroir.Step{Kind: macroir.StepClick, X: x, Y: y, Button: "left", Template: tmpl}
	if left, top, ok := s.env.WindowOrigin(); ok {
		st.WinRel, st.RelX, st.RelY = true, x-left, y-top
	}
	return st
}
