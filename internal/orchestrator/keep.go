package orchestrator

// KEEPING A RECORDING IS ONE SET OF RULES, AND EVERY SURFACE OBEYS IT.
//
// # Why this exists
//
// Because it did not, and the three surfaces disagreed about what saving means.
//
// Studio held the rules: a scope word nobody recognises is refused rather than defaulted; a scope
// whose presses cannot be performed under it is refused with the remedy; and a collision is
// refused and offers Replace, re-checked against the identity somebody consented to at the
// instant of writing. `marco learn` and the widget held none of them. Both were measured:
//
//   - Learning a name twice REPLACED the first routine. The library kept one routine, the file
//     held the second recording, and the output said `Learned …` as though it were a fresh save.
//     Re-recording is what somebody testing a routine does all afternoon, which makes the
//     likeliest casualty the routine they were comparing against.
//   - Answering "only here" or "anywhere" for a recording whose presses check their own window
//     SAVED A ROUTINE THAT REFUSES EVERY PRESS — `Deps.Unrunnable`, the same reader Studio greys
//     the Play button with, reported it immediately.
//
// The old note on this said the overwrite was the other surfaces' contract "rather than an
// oversight", and that Studio refused so the layer underneath could keep the behaviour every
// other caller was written against. That was a decision about a window; it is not a decision
// anybody made about somebody's library. The rules are here now, and the surfaces supply the
// INTERACTION — a card to press, a prompt to answer, or nothing at all.
//
// # What is shared and what is not
//
// Shared: whether a scope word means anything, whether this draft can be saved under it, what is
// already where it would go, and whether a replacement is the one that was agreed to.
//
// NOT shared, deliberately: what a surface SAYS, and WHAT IT CAPTURES. Studio states coordinate
// mode to the recorder and `marco learn` states nothing, which leaves the hooks doing what the
// environment says — the behaviour every caller before Studio had. This is shared orchestration
// and shared save rules, and it is not an identical capture configuration. See `Deps.recordedAs`.
//
// See [[ADR-266-one-set-of-rules-for-keeping-a-recording]].

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// WantedScope reads a surface's word for a scope, or says what the choices are.
//
// REFUSED RATHER THAN DEFAULTED. A scope nobody recognised becoming "only in this application"
// is the surface choosing where somebody's routine works and not telling them.
func WantedScope(scope string) (Scope, string) {
	switch Scope(strings.TrimSpace(scope)) {
	case ScopeApp:
		return ScopeApp, ""
	case ScopeAppAnywhere:
		return ScopeAppAnywhere, ""
	case ScopeEverywhere:
		return ScopeEverywhere, ""
	}
	return "", fmt.Sprintf("say where it should work: %q, %q or %q",
		ScopeApp, ScopeAppAnywhere, ScopeEverywhere)
}

// RouteID is a route's whole identity, as a person reads it and as a surface hands it back.
//
// `global/<slug>`, `<app>/context/<slug>`, `<app>/focus/<slug>` — because a route IS its scope
// and its slug, and a name is not unique. One derivation, so a collision's identity and the
// identity a replacement is checked against cannot be two different strings.
func RouteID(rt routes.Route) string {
	if rt.App == "" {
		return routes.GlobalDir + "/" + rt.Slug
	}
	scope := routes.ContextDir
	if rt.Focus {
		scope = routes.FocusDir
	}
	return rt.App + "/" + scope + "/" + rt.Slug
}

// Collision is what is already saved where a recording would go.
type Collision struct {
	Exists bool `json:"exists"`
	// Route is the identity a save would write. Not on the wire: a surface gets `ID`,
	// which is the readable form and the term it hands back.
	Route   routes.Route `json:"-"`
	ID      string       `json:"id,omitempty"`
	Name    string       `json:"name,omitempty"`
	SavedAt string       `json:"saved_at,omitempty"`
	// Says is the sentence a person reads. Why is why the question could not be asked at
	// all — a scope nobody recognised, or nothing recorded to ask about.
	Says string `json:"says,omitempty"`
	Why  string `json:"why,omitempty"`
}

// Collision answers whether saving this draft under this scope would replace something.
//
// A QUESTION. It writes nothing and decides nothing, and it is NOT what protects the library:
// every save asks again, because between the question and the answer somebody can save that name
// from a terminal, and a surface trusting its earlier look would overwrite at exactly that moment.
func (d Deps) Collision(dr *Draft, scope Scope, app string) Collision {
	// THE IDENTITY A SAVE WOULD WRITE, from the generator's own derivation rather than
	// from a second one here. See Deps.Target.
	rt, err := d.Target(dr, scope, strings.TrimSpace(app))
	if err != nil {
		return Collision{Why: err.Error()}
	}
	st, serr := os.Stat(d.Reg.Path(rt))
	if serr != nil {
		return Collision{Route: rt}
	}
	c := Collision{Exists: true, Route: rt, ID: RouteID(rt), Name: dr.Name,
		SavedAt: st.ModTime().Local().Format("2006-01-02 15:04")}
	if raw, rerr := os.ReadFile(d.Reg.Path(rt)); rerr == nil {
		c.Name = NameIn(string(raw), rt.Slug)
	}
	c.Says = fmt.Sprintf("there is already a routine saved there: %q, kept on %s. "+
		"Saving this one over it would replace its steps and the demonstration "+
		"beside it, and neither can be got back. Give this one another name, or "+
		"replace that routine on purpose.", c.Name, c.SavedAt)
	return c
}

// WhyNotThisScope says why this draft cannot be saved under this scope, or "" when it can.
//
// REFUSED, NOT REWRITTEN. Quietly saving it as something else would be the surface deciding
// where somebody's routine works, and saving it as asked would write a routine that refuses
// every press — which was measured on the `marco learn` path for two of its three answers.
func (d Deps) WhyNotThisScope(dr *Draft, want Scope) string {
	only := ScopeFor(dr)
	if only == "" || want == only {
		return ""
	}
	where := "the application it was recorded in"
	if dr != nil && dr.App != "" {
		where = dr.App
	}
	return fmt.Sprintf("every press in this recording checks the window it was made in, "+
		"and %q brings no window forward — so there would be nothing for those "+
		"presses to be checked against and every one of them would refuse. Save it "+
		"to run in %s with Marco bringing it forward.", want, where)
}

// Keep is the save, and the only enforcement of what a save may do.
//
// `replacing` empty means a collision is REFUSED. Named, it means this save may replace that one
// identity and no other — re-derived and compared here, at the instant of writing, because
// somebody agreed to replace ONE routine and whatever is at the target by now may not be it.
//
// Every surface goes through this. What each one does with the refusal is its own: Studio shows a
// card offering Replace, `marco learn` asks, and the widget's unattended path refuses and keeps
// what is there.
func (d Deps) Keep(dr *Draft, scope Scope, app, replacing string) (routes.Route, error) {
	if dr == nil {
		return routes.Route{}, fmt.Errorf("there is no recording to save")
	}
	if why := d.WhyNotThisScope(dr, scope); why != "" {
		return routes.Route{}, fmt.Errorf("%s", why)
	}
	at := d.Collision(dr, scope, app)
	switch {
	case at.Why != "":
		return routes.Route{}, fmt.Errorf("%s", at.Why)
	case replacing == "" && at.Exists:
		return routes.Route{}, fmt.Errorf("%s", at.Says)
	case replacing != "" && !at.Exists:
		return routes.Route{}, fmt.Errorf("there is no routine %q to replace any "+
			"more — it has gone since you chose it. Save this recording as it is.",
			replacing)
	case replacing != "" && at.ID != replacing:
		return routes.Route{}, fmt.Errorf("this would not replace the routine you "+
			"chose. You chose %q, and this recording would be saved as %q — "+
			"choose once more.", replacing, at.ID)
	}
	return d.Save(dr, scope, strings.TrimSpace(app))
}
