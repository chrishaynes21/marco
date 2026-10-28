package main

// SAVING A RECORDING DOES NOT REPLACE ONE UNLESS SOMEBODY SAYS SO.
//
// `Registry.Save` is a plain file write, so keeping a recording under a name already in the
// library replaces what is there — the source, and a moment later the demonstration beside it.
// Nothing asked, nothing said, nothing kept. Re-recording is what somebody testing a routine does
// all afternoon, which makes the likeliest casualty the very routine they were comparing against.
//
// # The policy USED to be here and nowhere lower
//
// This note said the overlay and `marco learn` overwrite on purpose — that teaching a name
// again is how you correct it there, their contract rather than an oversight — and that this
// window refused so the layer underneath could keep the behaviour every other caller was written
// against. It was a decision about a window, and nobody ever made it about somebody's library:
// learning a name twice replaced the first routine, with nothing asked and nothing said.
//
// So the rules moved DOWN, to `internal/orchestrator`'s `Keep`, and every surface obeys them.
// What is left here is this window's interaction: a card that offers Replace, the three ways in,
// and the trial journal. See [[ADR-266-one-set-of-rules-for-keeping-a-recording]].
//
// # And the check a save trusts is its own
//
// `Collision` exists so the page can offer the choice before anybody presses anything. It is not
// what protects the library. Every save asks again — inside `Keep` — because between the
// question and the answer somebody can save that name from a terminal, and a surface trusting its
// earlier look would overwrite at exactly that moment.
//
// Three ways in, one place that writes: as recorded, under another name, or over a routine named
// out loud.

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Collision is what is already saved where this recording would go.
//
// THE SHARED ONE, named here so the page's binding is unchanged. It was a copy of the same six
// fields, and a copy of a thing two surfaces must agree about is a thing they can stop agreeing
// about. See orchestrator.Collision.
type Collision = orchestrator.Collision

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

// collisionFor is that question asked of one draft under one scope, through the shared backend.
func (a *Acts) collisionFor(dr *orchestrator.Draft, scope, app string) Collision {
	want, why := orchestrator.WantedScope(scope)
	if why != "" {
		return Collision{Why: why}
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	return d.Collision(dr, want, app)
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
	want, why := orchestrator.WantedScope(scope)
	if why != "" {
		return why
	}
	// ── THE RULES ARE THE SHARED ONES AND THE INTERACTION IS THIS WINDOW'S ──
	//
	// A scope word nobody recognises, a scope these presses cannot be performed under, and
	// a collision that was not consented to: all three used to be written out here, and
	// `marco learn` had none of them. `Keep` is where they are now, so this window and
	// that terminal cannot come to different conclusions about what saving means. What a
	// refusal LOOKS like is still this window's: a sentence on a card, and a Replace
	// button that names one routine.
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	rt, err := d.Keep(dr, want, app, replacing)
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
