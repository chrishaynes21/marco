package main

// SAVING A RECORDING DOES NOT REPLACE ONE UNLESS SOMEBODY SAYS SO.
//
// `Registry.Save` is a plain file write, so keeping a recording under a name already in the
// library replaces what is there — the source, and a moment later the demonstration beside it.
// Nothing asked, nothing said, nothing kept. Re-recording is what somebody testing a routine does
// all afternoon, which makes the likeliest casualty the very routine they were comparing against.
//
// # The policy is HERE and nowhere lower
//
// The orchestrator, the overlay and `marco learn` overwrite on purpose: teaching a name again is
// how you correct it there, and that is their contract rather than an oversight. So this window
// refuses, and the layer underneath keeps the behaviour every other caller was written against.
//
// # And the check a save trusts is its own
//
// `Collision` exists so the page can offer the choice before anybody presses anything. It is not
// what protects the library. Every save asks again, because between the question and the answer
// somebody can save that name from a terminal — and a window trusting its earlier look would
// overwrite at exactly that moment.
//
// Three ways in, one place that writes: as recorded, under another name, or over a routine named
// out loud.

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Collision is what is already saved where this recording would go.
type Collision struct {
	Exists  bool   `json:"exists"`
	ID      string `json:"id,omitempty"`   // <app>/<scope>/<slug> — the one that is there
	Name    string `json:"name,omitempty"` // what the saved routine calls itself
	SavedAt string `json:"saved_at,omitempty"`
	// Says is the sentence the person reads. Why is why the question could not be asked
	// at all — a scope nobody recognised, or nothing recorded to ask about.
	Says string `json:"says,omitempty"`
	Why  string `json:"why,omitempty"`
}

// Collision answers whether saving this recording under this scope would replace something.
//
// A question. It writes nothing and decides nothing.
func (a *Acts) Collision(scope, app string) Collision {
	dr, _, why := a.draftToSave()
	if why != "" {
		return Collision{Why: why}
	}
	return a.collisionFor(dr, scope, app)
}

// collisionFor is that question asked of one draft under one scope.
func (a *Acts) collisionFor(dr *orchestrator.Draft, scope, app string) Collision {
	want, why := wantedScope(scope)
	if why != "" {
		return Collision{Why: why}
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	// THE IDENTITY A SAVE WOULD WRITE, from the generator's own derivation rather than
	// from a second one here. See orchestrator.Deps.Target.
	rt, err := d.Target(dr, want, strings.TrimSpace(app))
	if err != nil {
		return Collision{Why: err.Error()}
	}
	st, serr := os.Stat(a.reg().Path(rt))
	if serr != nil {
		return Collision{}
	}
	c := Collision{Exists: true, ID: routeID(rt), Name: dr.Name,
		SavedAt: st.ModTime().Local().Format("2006-01-02 15:04")}
	if raw, rerr := os.ReadFile(a.reg().Path(rt)); rerr == nil {
		c.Name = orchestrator.NameIn(string(raw), rt.Slug)
	}
	c.Says = fmt.Sprintf("there is already a routine saved there: %q, kept on %s. "+
		"Saving this one over it would replace its steps and the demonstration "+
		"beside it, and neither can be got back. Give this one another name, or "+
		"replace that routine on purpose.", c.Name, c.SavedAt)
	return c
}

// draftToSave is the recording waiting to be kept, or why there is nothing to keep.
func (a *Acts) draftToSave() (*orchestrator.Draft, *capture, string) {
	a.mu.Lock()
	cp := a.capture
	a.mu.Unlock()
	if cp == nil {
		return nil, nil, "there is no recording to save"
	}
	select {
	case <-cp.done:
	default:
		return nil, nil, "that recording is still running — press Stop first"
	}
	a.mu.Lock()
	draft, already := cp.draft, cp.saved
	a.mu.Unlock()
	if draft == nil {
		return nil, nil, "there is no recording to save"
	}
	if already != "" {
		// SAVED ONCE. A second Save would write the same routine over itself under
		// whatever scope was selected the second time, which is a routine quietly
		// rescoped by a double-click.
		return nil, nil, "that recording has already been saved — " + already
	}
	return draft, cp, ""
}

// SaveDraftAs keeps this recording under a different name — the way out of a collision that keeps
// both routines.
//
// It refuses a collision of its own, because the other name can be taken too and saving over THAT
// routine would be the same loss by a longer route.
func (a *Acts) SaveDraftAs(name, scope, app string) string {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "say what to call it first — the name is what you will look for later"
	case routes.Slug(name) == "":
		return fmt.Sprintf("%q has nothing in it a file can be named after — give it "+
			"some letters or numbers", name)
	}
	dr, cp, why := a.draftToSave()
	if why != "" {
		return why
	}
	// THE SAME NAME RULE AS A RECORDING STARTS UNDER, for the same reason: both become
	// declarations in the play that composes them.
	for _, c := range a.Saved() {
		if strings.EqualFold(c.Name, name) {
			return fmt.Sprintf("there is already an Act called %q, and a routine "+
				"cannot share its name — call this something else", c.Name)
		}
	}
	// THE DRAFT IS NOT RENAMED; A COPY IS. The recording in hand belongs to the name it
	// was made under until something is actually written, so a refusal below leaves the
	// window exactly as it was.
	under := *dr
	under.Name = name
	return a.keep(cp, &under, scope, app, "")
}

// ReplaceDraft saves this recording over one named routine, on purpose.
//
// The identity is PASSED IN rather than worked out here, and it is checked against the identity
// this save would write. Somebody agreed to replace ONE routine; a window that replaced whatever
// happened to be at the target by then would be doing something they did not agree to.
func (a *Acts) ReplaceDraft(id, scope, app string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "say which routine is being replaced"
	}
	dr, cp, why := a.draftToSave()
	if why != "" {
		return why
	}
	return a.keep(cp, dr, scope, app, id)
}

// keep is the save itself, and the only place that writes.
//
// `replacing` empty means a collision is refused. Named, it means this save may replace that one
// identity and no other.
func (a *Acts) keep(cp *capture, dr *orchestrator.Draft, scope, app, replacing string) string {
	want, why := wantedScope(scope)
	if why != "" {
		return why
	}
	// ── AND A SCOPE ITS PRESSES CANNOT BE PERFORMED UNDER IS REFUSED, NOT REWRITTEN ──
	//
	// Quietly saving it as something else would be this window deciding where somebody's
	// routine works — the thing `wantedScope` exists to prevent — and saving it as asked
	// would write a routine that refuses every press. So it says which scope it needs and
	// why, and the person asks again.
	if only := orchestrator.ScopeFor(dr); only != "" && want != only {
		where := dr.App
		if where == "" {
			where = "the application it was recorded in"
		}
		return fmt.Sprintf("every press in this recording checks the window it was "+
			"made in, and %q brings no window forward — so there would be "+
			"nothing for those presses to be checked against and every one of "+
			"them would refuse. Save it to run in %s with Marco bringing it "+
			"forward.", want, where)
	}
	// ── WHAT IS THERE RIGHT NOW, asked again at the save ──
	at := a.collisionFor(dr, scope, app)
	switch {
	case at.Why != "":
		return at.Why
	case replacing == "" && at.Exists:
		return at.Says
	case replacing != "" && !at.Exists:
		return fmt.Sprintf("there is no routine %q to replace any more — it has "+
			"gone since you chose it. Save this recording as it is.", replacing)
	case replacing != "" && at.ID != replacing:
		return fmt.Sprintf("this would not replace the routine you chose. You chose "+
			"%q, and this recording would be saved as %q — press Save again "+
			"and choose once more.", replacing, at.ID)
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	rt, err := d.Save(dr, want, strings.TrimSpace(app))
	if err != nil {
		return err.Error()
	}
	a.mu.Lock()
	cp.saved = a.reg().Path(rt)
	where := cp.saved
	a.mu.Unlock()
	kept := "Saved"
	if replacing != "" {
		kept = fmt.Sprintf("Replaced %q with", replacing)
	}
	note := fmt.Sprintf("%s %q from the recorder-first path (%s): %d step(s), %d of "+
		"which press something — %s", kept, dr.Name, want, len(dr.Steps),
		dr.Pressable(), where)
	next := "Run it, or build an Act over it."
	// AND THE JOURNAL SAYS WHEN WHAT WAS SAVED CANNOT RUN. A trial note reading
	// "Saved, run it" about a routine that will refuse every press is a note that will be
	// believed later by somebody reading back what happened.
	if why := whyItWillNotRun(dr.Steps); why != "" {
		note += ". " + why
		next = "Record it again — this one will refuse to press."
	}
	a.trial.recorded(dr.Name, note, next, false, Pictured{})
	return ""
}
