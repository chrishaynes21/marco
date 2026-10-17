package resolver

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Asking a plugin WHICH OF THESE CONTROLS the person meant.
//
// # Why this is a different switch from the other two questions
//
// The two questions above are never told anything about the screen, and the note on Interpret
// says why in as many words: the target travels on as the user's PHRASE, and Marco's own
// resolver decides what it refers to against a world it observed itself. That is a deliberate
// boundary, and this question crosses it — it cannot be answered without being told what is on
// screen.
//
// So it is the same protocol, the same stdio shape and the same package, because one boundary
// between Marco and a model is worth keeping. It is NOT the same variable. A plugin somebody
// configured to pick between their saved play names has not agreed to be sent the labels of
// whatever is in front of them, and quietly widening what an existing `$MARCO_RESOLVER` setting
// means would be making that decision for them. `$MARCO_INTERPRETER` is unset by default, and
// unset means this question is never asked.
//
//	→ {"input":"dark mode","which":{"application":"msedge","controls":[
//	     {"id":"e12","role":"combo_box","label":"Theme"},
//	     {"id":"e18","role":"checkbox","label":"Send notifications"}]}}
//	← {"control":"e12","because":"a theme setting is where a dark appearance is chosen"}
//	← {"control":""}                                       (nothing here fits — an abstention)
//
// # What it may say, which is one id and one sentence
//
// There is no field for a coordinate, a keystroke, a window, a role, a confidence, a permission
// or an action — so the narrowest statement of what this boundary permits is not a rule written
// down here, it is the shape of [Choice]. An id that was not in the list sent is discarded, so
// a plugin cannot name a control that was not observed even by trying.
//
// Everything it cannot do, it cannot do because it was never given anywhere to say it. It
// cannot make a control operable, because operability is decided afterwards by the code that
// decided it before. It cannot authorise anything, because risk and confirmation never see
// this. And it cannot act, because in this experiment nothing in the resolution or execution
// path asks the question at all — see internal/director/meaning.

// Control is one candidate control, as a plugin is told about it.
//
// Three fields. Not the value — a text field's contents are the person's own writing. Not the
// bounds, which a plugin has no use for and could only use to point somewhere. Not the native
// id, which is the application's own handle on the control.
type Control struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Label string `json:"label"`
}

// Choice is what a plugin made of a screenful of controls.
//
// An empty Control is an ABSTENTION and is a first-class answer rather than a failure: "I do
// not know which of these you meant" is the right reply to most requests, and a plugin that
// never abstains is one whose answers cannot be trusted when it does.
type Choice struct {
	Control string `json:"control"`
	Because string `json:"because,omitempty"`
}

// Abstained reports whether the plugin declined to choose.
func (c Choice) Abstained() bool { return c.Control == "" }

// whichRequest is the question, carried on the existing request object.
type whichRequest struct {
	// Application is the process the window belongs to — "msedge", "explorer". Context a
	// reader of the labels would have anyway, and the one piece of it that is not the
	// person's own content.
	Application string    `json:"application,omitempty"`
	Controls    []Control `json:"controls"`
}

type whichAsked struct {
	Input string        `json:"input"`
	Which *whichRequest `json:"which"`
}

// InterpreterPath returns the configured interpreter plugin ($MARCO_INTERPRETER).
func InterpreterPath() string { return os.Getenv("MARCO_INTERPRETER") }

// InterpreterConfigured reports whether a plugin may be asked which control was meant.
func InterpreterConfigured() bool { return InterpreterPath() != "" }

// howLongToWaitForAChoice bounds the question.
//
// A plugin that is slow is a plugin that is not there: an interpretation is advisory, and a
// person waiting on one has been made to wait for something that was never going to decide
// anything on its own.
const howLongToWaitForAChoice = 20 * time.Second

// WhichControl asks the plugin which of these controls the request meant.
//
// Returns an abstention — never an error — when no plugin is configured, when there is nothing
// to choose between, or when anything at all goes wrong: a missing executable, a timeout, a
// garbled reply, an id that was not offered. Every one of those is the same thing to the
// caller, which is that nothing interpreted the request, and the caller's own deterministic
// answer stands.
func WhichControl(ctx context.Context, input, application string,
	controls []Control) Choice {

	path := InterpreterPath()
	if path == "" || input == "" || len(controls) == 0 {
		return Choice{}
	}
	asked := whichAsked{Input: input, Which: &whichRequest{
		Application: application, Controls: controls,
	}}
	body, err := json.Marshal(asked)
	if err != nil {
		return Choice{}
	}
	ctx, cancel := context.WithTimeout(ctx, howLongToWaitForAChoice)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(append(body, '\n'))
	out, err := cmd.Output()
	if err != nil {
		return Choice{}
	}
	var got Choice
	if json.Unmarshal(bytes.TrimSpace(out), &got) != nil {
		return Choice{}
	}
	// ONLY AN ID THAT WAS OFFERED. The whole of "it cannot name a control that was not
	// observed", and it is a check rather than a hope: a plugin that returns anything else
	// has abstained, whatever it thinks it said.
	for _, c := range controls {
		if c.ID == got.Control {
			return Choice{Control: got.Control, Because: trimReason(got.Because)}
		}
	}
	return Choice{}
}

// trimReason bounds the sentence a plugin sends back, which is going into a person's account of
// why something was chosen and is not a place for an essay.
func trimReason(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	const most = 200
	if len(s) > most {
		s = strings.TrimSpace(s[:most]) + "…"
	}
	return s
}
