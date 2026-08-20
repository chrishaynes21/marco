package demo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Extraction: turning a demonstration into a procedure a person can read.
//
//	Extract intent, not clicks.
//	The learned result must be explainable.
//
// Every rule below is a stated rule over the recorded semantics, and every one records a
// Decision saying which rule fired and why. That is not decoration: a learned procedure
// runs on the user's own files, and "why is this a parameter?" has to have an answer that
// is not "the extractor thought so".
//
// The extractor PROPOSES. Nothing here installs anything — see Approve.

// Decision is one extraction choice, with the rule that made it.
type Decision struct {
	// Subject is what was decided about: "goal", "parameter new_name", "step 3".
	Subject string `json:"subject"`
	// Verdict is what was decided, in a small closed vocabulary so a reader can scan.
	Verdict Verdict `json:"verdict"`
	// Reason is the rule that fired, in a sentence.
	Reason string `json:"reason"`
}

// Verdict is what an extraction decision concluded.
type Verdict string

const (
	// VerdictRecovered: a goal was recovered from the semantic execution.
	VerdictRecovered Verdict = "recovered"
	// VerdictParameter: a demonstrated value became a parameter.
	VerdictParameter Verdict = "parameter"
	// VerdictConstant: something stayed as it was demonstrated.
	VerdictConstant Verdict = "constant"
	// VerdictKept: a step was carried into the procedure unchanged.
	VerdictKept Verdict = "kept"
	// VerdictRefused: the extraction stopped here.
	VerdictRefused Verdict = "refused"
)

// Parameter is a value the user supplies when the learned procedure runs.
type Parameter struct {
	// Name is what the procedure calls it — new_name, folder_name, customer_name.
	Name string `json:"name"`
	// Role is the goal parameter this fills, when it fills one. That is what lets a
	// learned procedure be driven by the ordinary goal parser: "rename this file to
	// Budget" already puts "Budget" in goal.ParamName.
	Role string `json:"role,omitempty"`
	// Step is the 1-based step that consumes it.
	Step int `json:"step"`
	// Example is what the user typed while demonstrating.
	//
	// Shown during review so the proposal can be recognised, and NEVER used at run time:
	// a procedure that fell back to the demonstrated value would silently rename a second
	// file to the first one's name. A demonstration that handled anything private is
	// refused whole (see Unsafe), so nothing here is a secret.
	Example string `json:"example,omitempty"`
	// Prompt is the question to ask when the parameter is missing.
	Prompt string `json:"prompt,omitempty"`
}

// CandidateStep is one step of a proposed procedure.
//
// The generalised form: what to DO and what to do it TO, semantically, with the
// demonstrated literal replaced by a parameter where one was discovered.
type CandidateStep struct {
	Index int      `json:"index"`
	Kind  StepKind `json:"kind"`
	// Semantic is the verb, for a semantic step.
	Semantic directorapi.SemanticActionKind `json:"semantic,omitempty"`
	// Role names the control by MEANING, which is what makes the procedure portable.
	Role goal.ControlRole `json:"role,omitempty"`
	// Deictic marks the step that acts on the object the user points at when they ask.
	Deictic bool `json:"deictic,omitempty"`
	// DerivedEditor marks a step acting on the editor a previous step opened.
	DerivedEditor bool `json:"derived_editor,omitempty"`
	// Parameter is the parameter whose value this step writes, empty for a constant.
	Parameter string `json:"parameter,omitempty"`
	// Text is a literal that stayed constant.
	Text string `json:"text,omitempty"`
	// Preconditions are the semantic waits this step runs under.
	Preconditions []directorapi.Condition `json:"preconditions,omitempty"`
	// Phrase is the human description.
	Phrase string `json:"phrase"`
}

// Describe renders a candidate step for review.
func (s CandidateStep) Describe() string {
	verb := string(s.Kind)
	if s.Semantic != "" {
		verb = string(s.Semantic)
	}
	what := "the focused control"
	switch {
	case s.DerivedEditor:
		what = "the editor the previous step opened"
	case s.Deictic:
		what = "the object the user points at"
	case s.Role != "":
		what = s.Role.Describe()
	}
	out := fmt.Sprintf("%s %s", verb, what)
	if s.Parameter != "" {
		out = fmt.Sprintf("%s %s to ${%s}", verb, what, s.Parameter)
	} else if s.Text != "" {
		out = fmt.Sprintf("%s %s to %q", verb, what, s.Text)
	}
	return out
}

// Candidate is a procedure proposed from a demonstration.
//
// Data, not code. That is what makes it reviewable before it is installed, storable
// afterwards, and identical in kind to the built-in procedures once it is registered — see
// AsProcedure.
type Candidate struct {
	// Name identifies it in the registry: "learned explorer rename".
	Name string `json:"name"`
	// Goal is the outcome it achieves, recovered from the actions.
	Goal goal.Kind `json:"goal"`
	// Application is what it was demonstrated in, and what it will be registered for.
	Application string `json:"application,omitempty"`
	// Expect is the kind of object its deictic step points at.
	Expect string `json:"expects,omitempty"`

	Parameters []Parameter     `json:"parameters,omitempty"`
	Steps      []CandidateStep `json:"steps"`
	Safety     goal.Safety     `json:"safety"`
	// Why explains the shape of the expansion, for `director procedures`.
	Why string `json:"why,omitempty"`

	// Source is the demonstration it came from, and Nodes the action-graph lineage.
	//
	//	Demonstrations reference Action Graph nodes. They do not duplicate them.
	//
	// So does a candidate: the provenance of a learned procedure is a list of node ids
	// and the id of a demonstration, both of which point at history rather than copying
	// any of it.
	Source ID                   `json:"source"`
	Nodes  []actiongraph.NodeID `json:"nodes,omitempty"`
}

// Extraction is everything reading one demonstration produced.
type Extraction struct {
	Demonstration ID `json:"demonstration"`
	// Candidate is the proposal, nil when the demonstration produced none.
	Candidate *Candidate `json:"candidate,omitempty"`
	// Decisions are every choice made, in order.
	Decisions []Decision `json:"decisions,omitempty"`
	// Refusal is why there is no candidate.
	Refusal string `json:"refusal,omitempty"`
}

// OK reports whether a procedure was proposed.
func (e Extraction) OK() bool { return e.Candidate != nil }

// Extract reads a demonstration and proposes a procedure.
//
// The order is the argument:
//
//	safety → validation → goal recovery → parameters → generalisation → naming
//
// Safety first, because a demonstration that may never be learned should not have its
// contents analysed at all. Validation second, because a demonstration with an unverified
// step describes no outcome. Goal recovery third, because everything after it — which
// value is the name, what the procedure is called, what it declares about itself — depends
// on what the user was DOING.
func Extract(d *Demonstration) Extraction {
	out := Extraction{}
	if d != nil {
		out.Demonstration = d.ID
	}

	if learnable, why := d.Learnable(); !learnable {
		return out.refuse("demonstration", why)
	}
	if why, unsafe := Unsafe(d); unsafe {
		return out.refuse("safety", why)
	}
	if why, ok := Validate(d); !ok {
		return out.refuse("validation", why)
	}

	recovered, gdec, ok := RecoverGoal(d)
	out.Decisions = append(out.Decisions, gdec...)
	if !ok {
		return out.refuse("goal", "the actions do not match any outcome the Director "+
			"knows how to name, so there is nothing to call this procedure")
	}

	params, steps, pdec := generalize(d, recovered)
	out.Decisions = append(out.Decisions, pdec...)

	c := &Candidate{
		Name:        candidateName(recovered, d.Application),
		Goal:        recovered,
		Application: d.Application,
		Parameters:  params,
		Steps:       steps,
		Safety:      safetyFor(recovered, steps),
		Source:      d.ID,
		Nodes:       append([]actiongraph.NodeID{}, d.Nodes...),
		Expect:      expectFor(d),
		Why: fmt.Sprintf("demonstrated once in %s: %s",
			describeApp(d.Application), strings.Join(stepPhrases(steps), ", then ")),
	}
	out.Candidate = c
	return out
}

// refuse records the refusal and returns the extraction with no candidate.
func (e Extraction) refuse(subject, why string) Extraction {
	e.Candidate = nil
	e.Refusal = why
	e.Decisions = append(e.Decisions, Decision{
		Subject: subject, Verdict: VerdictRefused, Reason: why,
	})
	return e
}

// ── goal recovery ─────────────────────────────────────────────────────────────

// signature is the semantic shape of one outcome.
//
// Matched against what the demonstration DID. A signature names roles and verbs — the
// durable, language-independent parts — and never the phrase the user spoke.
type signature struct {
	Goal goal.Kind
	// Roles are the semantic control roles that must have been invoked. ALL of them.
	Roles []goal.ControlRole
	// Verbs are semantic verbs that must appear, in this relative order, as a
	// subsequence. Steps between them are allowed: a demonstration is a real session and
	// contains scrolling, selecting and looking around.
	Verbs []directorapi.SemanticActionKind
	// Text requires at least one step that typed something.
	Text bool
	// TextAfter requires the typing to have happened AFTER this role was invoked.
	//
	// The ordering IS the claim. "The rename command was invoked" and "something was
	// typed" are both true of a user who renamed a file and of one who invoked Rename,
	// gave up, and typed into the search box. Only the order tells them apart, and it is
	// decidable from what was recorded.
	//
	// Deliberately not "the typing went into the inline editor". That is the truer
	// statement and it is not recoverable: whether a control is an inline editor is
	// known from its CLASS, which lives in the world and not in the verified action
	// record a demonstration is built from. Requiring it would refuse every rename a
	// user demonstrated by asking for the steps rather than by asking for the goal.
	TextAfter goal.ControlRole
	// Why explains the recovery in the extraction's own words.
	Why string
}

// weight is how much of a claim a signature makes.
//
// The tie-break between two matching signatures, and the reason "copy then paste" is
// recovered as DUPLICATE rather than as COPY: duplicate claims more, and every claim it
// makes was observed. A signature that claims less is not more likely to be right — it is
// less specific, in exactly the sense the procedure registry already uses.
func (s signature) weight() int {
	w := len(s.Roles)*2 + len(s.Verbs)
	if s.Text {
		w++
	}
	if s.TextAfter != "" {
		w += 2
	}
	return w
}

// signatures is the recovery table.
//
// Hand-written, like the procedures themselves, and for the same reason: what a sequence
// of semantic actions MEANS is a claim about the desktop, and a claim nobody wrote down is
// a claim nobody can check. Adding an outcome is adding a row here.
var signatures = []signature{
	{
		Goal: goal.Rename, Roles: []goal.ControlRole{goal.RoleRenameCommand},
		Text: true, TextAfter: goal.RoleRenameCommand,
		Why: "the rename command was invoked and the item's name was replaced after it",
	},
	{
		Goal: goal.CreateFolder, Roles: []goal.ControlRole{goal.RoleNewFolderCommand},
		Text: true,
		Why:  "the new-folder command was invoked and the new folder was named",
	},
	{
		Goal: goal.CreateFolder,
		Roles: []goal.ControlRole{
			goal.RoleNewSubmenu, goal.RoleFolderItem,
		},
		Text: true,
		Why: "the New submenu was opened, Folder was chosen from it, and the new folder " +
			"was named",
	},
	{
		Goal: goal.Duplicate,
		Verbs: []directorapi.SemanticActionKind{
			directorapi.SemanticCopy, directorapi.SemanticPaste,
		},
		Why: "the object was copied and then pasted, which is what duplicating is",
	},
	{
		Goal: goal.Move,
		Verbs: []directorapi.SemanticActionKind{
			directorapi.SemanticCut, directorapi.SemanticPaste,
		},
		Why: "the object was cut and then pasted, which is what moving is",
	},
	{
		Goal: goal.SaveAs, Roles: []goal.ControlRole{goal.RoleSaveAsCommand}, Text: true,
		Why: "the save-as command was invoked and a name was typed",
	},
	{
		Goal: goal.Save, Roles: []goal.ControlRole{goal.RoleSaveCommand},
		Why: "the save command was invoked",
	},
	{
		Goal: goal.Print, Roles: []goal.ControlRole{goal.RolePrintCommand},
		Why: "the print command was invoked",
	},
	{
		Goal: goal.OpenSettings, Roles: []goal.ControlRole{goal.RoleSettingsCommand},
		Why: "the settings command was invoked",
	},
	{
		Goal: goal.CreateTab, Roles: []goal.ControlRole{goal.RoleNewTabCommand},
		Why: "the new-tab command was invoked",
	},
	{
		Goal: goal.Download, Roles: []goal.ControlRole{goal.RoleDownloadCommand},
		Why: "the download command was invoked",
	},
	{
		Goal: goal.Craft, Roles: []goal.ControlRole{goal.RoleCraftCommand},
		Verbs: []directorapi.SemanticActionKind{
			directorapi.SemanticChoose, directorapi.SemanticInvoke,
		},
		Why: "something was chosen from what can be made and the make command was invoked",
	},
	{
		Goal: goal.Sort, Roles: []goal.ControlRole{goal.RoleSortCommand},
		Why: "the sort command was invoked",
	},
	{
		Goal: goal.OpenFile, Verbs: []directorapi.SemanticActionKind{directorapi.SemanticOpen},
		Why: "the object was opened",
	},
	{
		Goal: goal.Copy, Verbs: []directorapi.SemanticActionKind{directorapi.SemanticCopy},
		Why: "the selection was copied",
	},
	{
		Goal: goal.Paste, Verbs: []directorapi.SemanticActionKind{directorapi.SemanticPaste},
		Why: "the clipboard was pasted",
	},
}

// RecoverGoal infers the outcome from the semantic execution.
//
//	Do not rely on the spoken phrase. Recover the goal from the semantic execution.
//
// The phrase is what the user said they were doing; the actions are what they did. Those
// differ more often than one would like — a user says "rename" and demonstrates a
// save-as — and the actions are the half that the learned procedure will actually repeat.
//
// The recorded goal provenance IS consulted, but only to report agreement: a recovery that
// matches what the user asked for is worth saying, and one that does not is worth saying
// louder.
func RecoverGoal(d *Demonstration) (goal.Kind, []Decision, bool) {
	var matched []signature
	for _, s := range signatures {
		if s.matches(d) {
			matched = append(matched, s)
		}
	}
	if len(matched) == 0 {
		return "", []Decision{{
			Subject: "goal", Verdict: VerdictRefused,
			Reason: fmt.Sprintf(
				"nothing in the demonstration matches a known outcome. It invoked %s and "+
					"performed %s.", describeRoles(d.Roles()), describeVerbs(d.Verbs())),
		}}, false
	}

	// Most specific wins, and a tie REFUSES. The same rule the procedure registry uses,
	// for the same reason: letting table order decide between two equally good readings
	// is the kind of bug that surfaces once, in front of a user, as the wrong procedure.
	sort.SliceStable(matched, func(i, j int) bool {
		if matched[i].weight() != matched[j].weight() {
			return matched[i].weight() > matched[j].weight()
		}
		return matched[i].Goal < matched[j].Goal
	})
	best := matched[0]
	if len(matched) > 1 && matched[1].weight() == best.weight() &&
		matched[1].Goal != best.Goal {
		return "", []Decision{{
			Subject: "goal", Verdict: VerdictRefused,
			Reason: fmt.Sprintf(
				"the demonstration reads equally well as %s and as %s, and nothing "+
					"distinguishes them. Refusing rather than guessing.",
				best.Goal.Describe(), matched[1].Goal.Describe()),
		}}, false
	}

	decisions := []Decision{{
		Subject: "goal", Verdict: VerdictRecovered,
		Reason: fmt.Sprintf("%s: %s", best.Goal.Describe(), best.Why),
	}}
	if asked := askedFor(d); asked != "" {
		if asked == string(best.Goal) {
			decisions = append(decisions, Decision{
				Subject: "goal", Verdict: VerdictRecovered,
				Reason: fmt.Sprintf("the user also asked for %s, which agrees — but the "+
					"recovery is from the actions either way", best.Goal.Describe()),
			})
		} else {
			decisions = append(decisions, Decision{
				Subject: "goal", Verdict: VerdictRecovered,
				Reason: fmt.Sprintf("the user asked for %q and the actions are %s. The "+
					"ACTIONS decide: they are what a learned procedure repeats.",
					asked, best.Goal.Describe()),
			})
		}
	}
	return best.Goal, decisions, true
}

// matches reports whether a demonstration satisfies this signature.
func (s signature) matches(d *Demonstration) bool {
	for _, r := range s.Roles {
		if !d.HasRole(r) {
			return false
		}
	}
	text := d.TextSteps()
	if s.Text && len(text) == 0 {
		return false
	}
	if s.TextAfter != "" {
		at, ok := stepInvoking(d, s.TextAfter)
		if !ok || !anyTextAfter(text, at) {
			return false
		}
	}
	return isSubsequence(s.Verbs, d.Verbs())
}

// stepInvoking is the position of the step that acted on a control of this role.
func stepInvoking(d *Demonstration, role goal.ControlRole) (int, bool) {
	for _, st := range d.Steps {
		if st.Target.Role == role {
			return st.Index, true
		}
	}
	return 0, false
}

// anyTextAfter reports whether SOMETHING was typed at or after a position.
//
// Any, not the first. A user who typed "rename" into a command palette to reach the
// command and then typed the new name has two text steps, and the one that matters is the
// second — refusing on the first would refuse the demonstration for having used a palette.
func anyTextAfter(text []Step, at int) bool {
	for _, s := range text {
		if s.Index >= at {
			return true
		}
	}
	return false
}

// isSubsequence reports whether want appears in got, in order, with gaps allowed.
func isSubsequence(want, got []directorapi.SemanticActionKind) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && g == want[i] {
			i++
		}
	}
	return i == len(want)
}

// askedFor is the goal the user's own request was expanded as, empty when none was.
func askedFor(d *Demonstration) string {
	for _, s := range d.Steps {
		if s.Goal != "" {
			return s.Goal
		}
	}
	return ""
}

// ── parameters, constants and generalisation ──────────────────────────────────

// goalParameters maps an outcome to the parameter it takes, and what that parameter is
// called in a learned procedure.
//
// Two names, deliberately. The ROLE is what the goal layer already calls it (goal.ParamName),
// so the ordinary parser fills it from "rename this file to Budget" with no new parsing.
// The NAME is what a person reads in the proposal, and "new_name" says more than "name".
var goalParameters = map[goal.Kind]struct{ Name, Role, Prompt string }{
	goal.Rename:       {"new_name", goal.ParamName, "what should it be called?"},
	goal.CreateFolder: {"folder_name", goal.ParamName, "what should the folder be called?"},
	goal.SaveAs:       {"file_name", goal.ParamName, "what should the file be called?"},
	goal.Move:         {"destination", goal.ParamDestination, "where should it go?"},
}

// generalize turns recorded steps into candidate steps, discovering parameters.
//
//	Only parameterize user-provided data.
//	Err toward fewer parameters.
//
// The rule, and it is one rule: a value becomes a parameter when the user TYPED it. Not
// when they chose it, not when they clicked it, not when the procedure named it. That is
// what keeps "Move to Downloads" constant when Downloads was a folder the user clicked and
// makes it a parameter when it was a path they typed — and it is decidable from what was
// recorded rather than from what the extractor imagines the user meant.
func generalize(d *Demonstration, kind goal.Kind) ([]Parameter, []CandidateStep, []Decision) {
	var params []Parameter
	var steps []CandidateStep
	var decisions []Decision

	used := map[string]bool{}
	textSeen := 0
	subject := subjectStep(d, kind)

	for _, s := range d.Steps {
		cs := CandidateStep{
			Index:         len(steps) + 1,
			Kind:          s.Kind,
			Semantic:      s.Semantic,
			Role:          s.Target.Role,
			Deictic:       s.Target.Deictic,
			DerivedEditor: s.Target.DerivedEditor,
			Preconditions: append([]directorapi.Condition{}, s.Preconditions...),
			Phrase:        s.Phrase,
		}
		// The OBJECT the procedure is about becomes "the thing the user points at".
		//
		// A rename demonstrated on Alpha.txt is not a procedure that renames Alpha.txt;
		// it is a procedure that renames the file you mean, and the file you mean is
		// supplied when you ask. This is the same generalisation the typed value gets,
		// applied to the subject rather than to the data — and it is what makes the
		// learned procedure answer "rename this file to Q4" at all.
		if s.Index == subject {
			cs.Deictic, cs.Role = true, ""
			decisions = append(decisions, Decision{
				Subject: fmt.Sprintf("step %d", cs.Index), Verdict: VerdictParameter,
				Reason: fmt.Sprintf(
					"it acted on %q, which is the object the procedure is ABOUT rather than "+
						"a control it uses. It becomes the thing the user points at when they "+
						"ask, so the procedure works on any %s.",
					s.Target.Label, describeKindObject(kind)),
			})
			steps = append(steps, cs)
			continue
		}

		if s.Kind != StepEdit || s.Text == "" {
			// Semantic STRUCTURE. The verb, the role it was aimed at, and the waits it
			// ran under are what the procedure IS; turning any of them into a parameter
			// would produce a procedure whose meaning the caller supplies.
			decisions = append(decisions, Decision{
				Subject: fmt.Sprintf("step %d", cs.Index), Verdict: VerdictConstant,
				Reason: fmt.Sprintf("%s is semantic structure, not data: it is what the "+
					"procedure does", cs.Describe()),
			})
			steps = append(steps, cs)
			continue
		}

		textSeen++
		if why, keep := constantText(s, d); keep {
			cs.Text = s.Text
			decisions = append(decisions, Decision{
				Subject: fmt.Sprintf("step %d", cs.Index), Verdict: VerdictConstant,
				Reason: why,
			})
			steps = append(steps, cs)
			continue
		}

		p := Parameter{
			Name:    parameterName(s, kind, textSeen, used),
			Step:    cs.Index,
			Example: s.Text,
		}
		if textSeen == 1 {
			if gp, ok := goalParameters[kind]; ok {
				p.Role, p.Prompt = gp.Role, gp.Prompt
			}
		}
		used[p.Name] = true
		params = append(params, p)
		cs.Parameter = p.Name
		decisions = append(decisions, Decision{
			Subject: "parameter " + p.Name, Verdict: VerdictParameter,
			Reason: fmt.Sprintf(
				"step %d typed %q into %s. Typed text is data the user supplied, so it "+
					"becomes a parameter rather than being repeated verbatim.",
				cs.Index, s.Text, s.Target.Describe()),
		})
		steps = append(steps, cs)
	}
	return params, steps, decisions
}

// subjectStep is the step that acted on the OBJECT the goal is about, 0 when none did.
//
// Identified structurally rather than by name: the first step aimed at a CONTENT element
// — a list item, a row, a tree item, an image — that carries no semantic control role and
// was not already pointed at. Those are the things a file manager, a mail client and a
// browser tab strip present as objects; a button, a menu item and a tab are controls the
// procedure USES.
//
// Only for a goal that takes a target. "Open settings" acts on no object, and turning its
// first step into a deictic reference would make it demand one.
func subjectStep(d *Demonstration, kind goal.Kind) int {
	if !takesTarget(kind) {
		return 0
	}
	for _, s := range d.Steps {
		t := s.Target
		switch {
		case t.Deictic:
			// Already the thing the user points at, which is the shape this produces.
			return 0
		case t.Role != "" || t.DerivedEditor || t.Anaphoric:
			continue
		case isObject(t.ElementRole):
			return s.Index
		}
	}
	return 0
}

// objectRoles are the element roles that name a THING rather than a control.
//
// A closed list. Everything absent from it is treated as a control the procedure uses,
// which is the safe direction: a control wrongly generalised into "the object you point
// at" would make the procedure demand a target it does not need and act on the wrong
// thing when given one.
var objectRoles = map[directorapi.ElementRole]bool{
	directorapi.RoleListItem: true,
	directorapi.RoleTreeItem: true,
	directorapi.RoleRow:      true,
	directorapi.RoleCell:     true,
	directorapi.RoleImage:    true,
}

func isObject(r directorapi.ElementRole) bool { return objectRoles[r] }

// takesTarget reports whether a goal acts on an object the user names or points at.
func takesTarget(kind goal.Kind) bool {
	switch kind {
	case goal.Rename, goal.Duplicate, goal.Delete, goal.Move, goal.Copy, goal.OpenFile,
		goal.Download:
		return true
	}
	return false
}

// describeKindObject names what a goal acts on, for an extraction decision.
func describeKindObject(kind goal.Kind) string {
	if kind == goal.CreateFolder {
		return "folder"
	}
	return "one"
}

// constantText reports whether typed text must stay as it was demonstrated.
//
// The exceptions to "typed text is a parameter", each one a case where the text is part of
// the procedure's MEANING rather than the user's data.
func constantText(s Step, d *Demonstration) (string, bool) {
	text := strings.TrimSpace(s.Text)
	switch {
	case text == "":
		return "there is nothing to parameterise", true
	case strings.EqualFold(text, d.Application):
		return fmt.Sprintf("%q is the application's own name, which is not something a "+
			"caller supplies", text), true
	case goal.RoleForLabel(text) != "":
		return fmt.Sprintf("%q is the name of %s — a semantic control rather than data",
			text, goal.RoleForLabel(text).Describe()), true
	case isProceduralVerb(text):
		return fmt.Sprintf("%q is a procedural verb, which names the step rather than "+
			"supplying data to it", text), true
	}
	return "", false
}

// proceduralVerbs are words that name an operation rather than data.
//
// A short, closed list. It exists for the case where a user types a command into a command
// palette or a search box — "rename", "new folder" — where the text IS the step.
var proceduralVerbs = []string{
	"rename", "delete", "save", "save as", "print", "copy", "cut", "paste",
	"duplicate", "move", "new folder", "new tab", "settings", "open", "close",
	"undo", "redo", "download",
}

func isProceduralVerb(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, v := range proceduralVerbs {
		if t == v {
			return true
		}
	}
	return false
}

// parameterName names a discovered parameter.
//
// In order:
//
//  1. The GOAL's own parameter name, for the first typed value. A rename's first typed
//     value is the new name, whatever field it went into — and naming it new_name is what
//     lets the ordinary goal parser fill it from "rename this to Budget".
//  2. The FIELD's label, slugified. "Customer name" becomes customer_name, which is a name
//     the user can predict from the form they filled in.
//  3. value_N, when the field said nothing useful.
//
// A derived editor's label is never used: it is the object's OLD value, so naming a
// parameter after it would produce alpha_txt for a rename.
func parameterName(s Step, kind goal.Kind, ordinal int, used map[string]bool) string {
	if ordinal == 1 {
		if gp, ok := goalParameters[kind]; ok && !used[gp.Name] {
			return gp.Name
		}
	}
	if !s.Target.DerivedEditor && s.Target.Label != "" &&
		!strings.EqualFold(s.Target.Label, s.Text) {
		if name := slug(s.Target.Label); name != "" && !used[name] {
			return name
		}
	}
	for i := ordinal; ; i++ {
		name := fmt.Sprintf("value_%d", i)
		if !used[name] {
			return name
		}
	}
}

// ── the shape of the proposal ─────────────────────────────────────────────────

// candidateName is what a learned procedure is called.
//
// "learned" is in the name on purpose. It appears in `director procedures`, in the plan of
// every request it serves, and in the action graph — and a user looking at a procedure that
// is about to act on their files is entitled to see where it came from without asking.
func candidateName(kind goal.Kind, app string) string {
	if app == "" {
		return fmt.Sprintf("learned %s", kind)
	}
	return fmt.Sprintf("learned %s %s", strings.ToLower(app), kind)
}

// safetyFor declares what a learned procedure will do.
//
// Conservative by construction, and it can afford to be: everything genuinely dangerous was
// refused before extraction began (see Unsafe), so what reaches here mutates at most what
// its typed steps write. The mutation count is the number of steps that change something —
// which for every learned procedure so far is the number of typed values plus the commit.
func safetyFor(kind goal.Kind, steps []CandidateStep) goal.Safety {
	mutations := 0
	for _, s := range steps {
		if s.Kind == StepEdit || s.Semantic == directorapi.SemanticConfirm ||
			s.Semantic == directorapi.SemanticSubmit {
			mutations++
		}
	}
	if mutations == 0 {
		mutations = 1
	}
	risk := directorapi.RiskMedium
	if kind == goal.OpenSettings || kind == goal.CreateTab || kind == goal.Copy {
		risk = directorapi.RiskLow
	}
	return goal.Safety{Mutations: mutations, Risk: risk}
}

// expectFor is the kind of object the procedure's deictic step points at.
//
// Read from what the demonstration actually bound rather than assumed from the goal: a
// procedure that declares the wrong kind is refused at expansion, which is the right
// failure but a confusing one to hit because of a guess made here.
func expectFor(d *Demonstration) string {
	for _, s := range d.Steps {
		if s.Target.Deictic {
			// The binding layer's own vocabulary. A demonstration in a file manager binds
			// a file; nothing else is claimed, and a procedure that needs a different kind
			// is a different demonstration.
			return "file"
		}
	}
	return ""
}

func stepPhrases(steps []CandidateStep) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Describe())
	}
	return out
}

func describeApp(app string) string {
	if app == "" {
		return "no single application"
	}
	return app
}

func describeRoles(roles []goal.ControlRole) string {
	if len(roles) == 0 {
		return "no control this build recognises"
	}
	out := make([]string, 0, len(roles))
	for _, r := range roles {
		out = append(out, r.Describe())
	}
	return strings.Join(out, ", ")
}

func describeVerbs(verbs []directorapi.SemanticActionKind) string {
	if len(verbs) == 0 {
		return "no semantic verbs"
	}
	out := make([]string, 0, len(verbs))
	for _, v := range verbs {
		out = append(out, string(v))
	}
	return strings.Join(out, " → ")
}

// SignedOutcomes is the set of goals recovery can name.
//
// Exported for the completeness test rather than for use: a caller that branched on this
// would be asking "could this have been learned?" of a thing that either was or was not.
func SignedOutcomes() map[goal.Kind]bool {
	out := map[goal.Kind]bool{}
	for _, s := range signatures {
		out[s.Goal] = true
	}
	return out
}
