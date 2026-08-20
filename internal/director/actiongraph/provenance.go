package actiongraph

// Goal provenance: which goal and procedure produced this node.
//
//	Persist provenance as diagnostic metadata rather than executable semantics.
//	Replay must not re-expand the original goal.
//
// The distinction is the whole design. A node records a SEMANTIC ACTION, and replay
// re-resolves and re-runs exactly that action — it does not look up the procedure that
// produced it, does not consult the registry, and would behave identically if the
// procedure were deleted tomorrow. Provenance is what lets a person ask "where did this
// come from?"; it is not an instruction for reproducing it.
//
// That matters because a procedure is a living thing. If replay re-expanded the goal, a
// node recorded today would replay differently after the library changed — the history
// would stop being a record of what happened and become a request to do it again the
// current way.

// GoalProvenance is the diagnostic account of a node's origin.
//
// Every field is optional and every field is a stable IDENTIFIER rather than a display
// string where one exists: `goal` and `procedure` are the registry's own names, and a
// renamed procedure leaves old nodes readable rather than silently re-pointed.
type GoalProvenance struct {
	// Goal is the goal kind that matched ("rename"). A stable identifier.
	Goal string `json:"goal"`
	// Procedure is the procedure's registered name ("explorer rename").
	Procedure string `json:"procedure"`
	// Request is the user's original words, which is the thing they will recognise
	// when nothing else in the node looks familiar.
	Request string `json:"request,omitempty"`
	// StepID and StepIndex locate this action within the expansion. The ID is stable;
	// the index is for reading.
	StepID    string `json:"step_id,omitempty"`
	StepIndex int    `json:"step_index,omitempty"`
	StepCount int    `json:"step_count,omitempty"`
	// Verification is what the procedure asked of this step: "required" or
	// "best_effort". Recorded because a step that was allowed to be inconclusive and a
	// step that had to prove itself are different claims about the same outcome.
	Verification string `json:"verification,omitempty"`
	// Guarded reports whether the step waited on a precondition before running.
	Guarded bool `json:"guarded,omitempty"`
	// ProcedureGeneric distinguishes the generic procedure from an application
	// override, which is the first thing to check when an expansion looks wrong for
	// the application it ran in.
	ProcedureGeneric bool `json:"procedure_generic,omitempty"`
	// Application is the app the procedure was selected for, empty when none was named.
	Application string `json:"application,omitempty"`
}

// Empty reports whether there is no provenance to show.
//
// An absent GoalProvenance is the ordinary case for every node recorded before this
// existed and for every request that was not a goal. Old graphs stay valid because the
// field is omitted entirely rather than written as a zero value that a reader would have
// to distinguish from a real one.
func (p *GoalProvenance) Empty() bool { return p == nil || (p.Goal == "" && p.Procedure == "") }

// Describe renders provenance in one line.
func (p *GoalProvenance) Describe() string {
	if p.Empty() {
		return ""
	}
	out := p.Goal
	if p.Procedure != "" {
		out += " via " + p.Procedure
	}
	if p.StepCount > 0 {
		out += " (step " + itoa(p.StepIndex) + " of " + itoa(p.StepCount) + ")"
	}
	return out
}

// itoa avoids pulling strconv in for two call sites.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
