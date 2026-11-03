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
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// SaveWithin is how long one save waits for another save of the same routine.
//
// Generous against what it is waiting for and short against a person: what holds the route is
// rendering a few steps and writing two files, so a wait that reaches this is not a queue but
// something wrong. Past it the save REFUSES and says so, because a window that waits out a
// problem is a window that looks wedged.
const SaveWithin = 5 * time.Second

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
// ONE SPELLING OF IT, which is `routes.Route.ID`. The phrase file binds itself to a route's
// identity, so a second way of writing that identity down is a second thing for a binding check
// to disagree with — and this name is kept because every surface in this package calls it.
func RouteID(rt routes.Route) string { return rt.ID() }

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
	// all — a scope nobody recognised, nothing recorded to ask about, or a path that could
	// not be looked at.
	Says string `json:"says,omitempty"`
	Why  string `json:"why,omitempty"`
	// Source and Recording are digests of the routine that is there and the demonstration
	// beside it, AS THEY WERE WHEN THIS QUESTION WAS ASKED.
	//
	// They are on the wire because consent travels back through the surface, and what
	// somebody consents to is a VERSION rather than a path. Handed to `Keep` as a
	// `Consent` and compared again at the instant of writing: see Consent.
	//
	// Empty Recording is a real answer and means there was no demonstration beside it,
	// which is different from one that has appeared since.
	Source    string `json:"source,omitempty"`
	Recording string `json:"recording,omitempty"`
	// HasStoredSteps is whether the routine that is there keeps its own step list — a
	// stored rendering basis that still describes it.
	//
	// SAID BECAUSE IT IS A SEPARATE THING TO LOSE. Replacing a routine replaces its
	// source and its demonstration, which the sentence has always covered; a routine
	// with a stored basis also loses that, and it is not recoverable from either of the
	// other two.
	//
	// NOT CALLED `Edited`, which it was. A basis is stored by a step edit AND by a
	// re-simplification, and nothing here can tell them apart — so a field named for one
	// of them would have put a guess in the sentence a person reads. See Says.
	HasStoredSteps bool `json:"has_stored_steps,omitempty"`
	// Answers is what the routine that is there also answers to, and those phrases are
	// KEPT rather than lost — they are bound to the identity, not to the files.
	//
	// On the wire because the sentence names them and a surface may want to list them
	// beside it. Empty when it answers only to its own name.
	Answers []string `json:"answers,omitempty"`
	// Shortcuts are the keys that run the routine that is there, each as "<key> <where>". A
	// replacement keeps them: they run the new version.
	Shortcuts []string `json:"shortcuts,omitempty"`
}

// Consent is permission to replace ONE routine, as it was when somebody was shown it.
//
// # Why an identity is not enough
//
// Because the identity of a routine is its place in the library, and a place does not change when
// its contents do. `Keep` checked that the route still existed with the same id, which catches it
// being DELETED while the question was open and misses it being REPLACED: another save landing
// there between the question and the answer left the newer routine sitting at the identity
// somebody had agreed to overwrite, and it was overwritten. The person agreed to lose the version
// they were shown; nobody agreed to lose that one.
//
// So consent carries the version. `Deps.Replace` already works this way for a regeneration — it
// refuses when the source or the stored demonstration has changed since the review was built —
// and this is the same rule for the other way a routine gets written over.
//
// The zero value is NO consent, which is what a fresh save passes.
type Consent struct {
	ID        string `json:"id,omitempty"`
	Source    string `json:"source,omitempty"`
	Recording string `json:"recording,omitempty"`
}

// Given reports whether anything was consented to at all.
//
// Exported because a surface asks it too: Studio says "Replaced X with" or "Saved" off the same
// answer this decides the write with, and a surface re-deriving "was this a replacement" from a
// field it happens to hold is a second answer to the question.
func (c Consent) Given() bool { return strings.TrimSpace(c.ID) != "" }

// versionAt is the digests of what is saved at a route right now.
//
// Absent source is "", which `Collision` reports as nothing being there at all; absent recording
// is "" too and is a real answer rather than a failure, because a hand-written routine never had
// one. Both from `routes.DigestOf`, the same function a play's provenance is kept with.
func (d Deps) versionAt(rt routes.Route) (source, recording string) {
	if raw, err := os.ReadFile(d.Reg.Path(rt)); err == nil {
		source = routes.DigestOf(string(raw))
	}
	if rec, ok := d.Reg.LoadRecording(rt); ok {
		recording = routes.DigestOf(string(rec))
	}
	return source, recording
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
	// AND ONLY "NOT THERE" MEANS NOTHING IS THERE.
	//
	// RECORDED AS UNFALSIFIABLE ON THIS PLATFORM, which is why the comment is this long.
	// Windows cannot be made to produce a `Stat` error here that is not path-not-found by
	// anything a test can arrange: a directory in the routine's place makes `Stat`
	// SUCCEED, and a file where a parent directory should be reports path-not-found, which
	// `os.IsNotExist` answers true for. Both measured. The branch stays because the rule
	// it states is right and because another platform, or a permission this account does
	// not have, will reach it — the same standing as `Watching.has`'s empty-path check.
	//
	// Every error used to read as absence, so a path that could not be looked at — a
	// permission, a name too long, a directory where a file should be — answered "free" and
	// the save went on to write over whatever it was. The honest answer to a question that
	// could not be asked is that it could not be asked.
	switch {
	case serr != nil && os.IsNotExist(serr):
		return Collision{Route: rt}
	case serr != nil:
		return Collision{Route: rt, Why: fmt.Sprintf("whether there is already a "+
			"routine at %s could not be read, so nothing has been saved: %v",
			d.Reg.Path(rt), serr)}
	}
	c := Collision{Exists: true, Route: rt, ID: RouteID(rt), Name: dr.Name,
		SavedAt: st.ModTime().Local().Format("2006-01-02 15:04")}
	if raw, rerr := os.ReadFile(d.Reg.Path(rt)); rerr == nil {
		c.Name = NameIn(string(raw), rt.Slug)
	}
	// AND WHICH VERSION THIS QUESTION IS ABOUT. See Consent.
	c.Source, c.Recording = d.versionAt(rt)
	c.Says = fmt.Sprintf("there is already a routine saved there: %q, kept on %s. "+
		"Saving this one over it would replace its steps and the demonstration "+
		"beside it, and neither can be got back. Give this one another name, or "+
		"replace that routine on purpose.", c.Name, c.SavedAt)
	// AND ITS OWN STEP LIST, WHICH IS A THIRD THING. The sentence above names the two
	// files; a routine with a stored basis has a step list in neither of them, and saving
	// over it is the one lifecycle operation where losing that is the right answer —
	// somebody has demonstrated the thing again. Right, and still owed a sentence.
	if _, st := d.Reg.Steps(rt); st.Usable() {
		c.HasStoredSteps = true
		c.Says += " It also keeps a step list of its own, including any step edits " +
			"made to it, and that goes with it."
	}
	// AND THE PHRASES IT ANSWERS TO, WHICH ARE A FOURTH THING AND SURVIVE.
	//
	// Said for the opposite reason to the step list: that is lost and this is KEPT. A
	// replacement writes the source at an identity the phrases are bound to, so whatever
	// somebody taught this routine to answer to goes on answering — for the new
	// demonstration. Nobody is going to guess that from a sentence about files being
	// replaced, and the two facts in one place is what makes the choice informed.
	if s, st := d.Reg.Aliases(rt); st.Offered() {
		c.Answers = s.Phrases
		c.Says += fmt.Sprintf(" It also answers to %s, and it still will — those "+
			"phrases belong to its place in the library, so they would reach "+
			"whatever is saved here.", joinWords(quoted(s.Phrases)))
	}
	// AND ITS SHORTCUTS, WHICH ARE KEPT. The owner's ruling (2026-11-03): a replacement somebody
	// agrees to is a new version of the same routine, so the keys that ran it run the new
	// version. That is the opposite of a forget followed by an unrelated routine saved under
	// the name, which inherits nothing. Said here, once, with no question per shortcut.
	c.Shortcuts, c.Says = d.shortcutsKeptSays(rt, c.Says)
	return c
}

// quoted puts each phrase in quotation marks, so a sentence naming two of them cannot be read as
// naming one with a comma in it.
func quoted(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, fmt.Sprintf("%q", s))
	}
	return out
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
// The zero `Consent` means a collision is REFUSED, and the write is EXCLUSIVE — a path that
// appears between the check and the write is somebody else's save and is not written over. A
// consent names one routine AND the version of it somebody was shown, both re-read here at the
// instant of writing, because between the question and the answer that routine can be deleted or
// saved again from a terminal.
//
// Every surface goes through this. What each one does with the refusal is its own: Studio shows a
// card offering Replace, `marco learn` asks, and the widget's unattended path refuses and keeps
// what is there.
func (d Deps) Keep(dr *Draft, scope Scope, app string, consent Consent) (routes.Route, error) {
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
	// AND THIS ONE IS NO LONGER WHAT PROTECTS ANYTHING, which is worth saying so that
	// nobody reads it as the thing holding the line. The exclusive create below refuses the
	// same case on its own and reports it with this same sentence, so deleting this arm
	// changes no behaviour and its gate run is recorded as redundant rather than as a
	// kill — the mutations that bite are the ones on the WRITE.
	//
	// It stays because it is where the sentence comes from in the ordinary case, one
	// syscall earlier and without a half-written anything to clean up, and because a
	// surface that wants to offer Replace needs the refusal before it has tried to write.
	case !consent.Given() && at.Exists:
		return routes.Route{}, fmt.Errorf("%s", at.Says)
	case consent.Given() && !at.Exists:
		return routes.Route{}, fmt.Errorf("there is no routine %q to replace any "+
			"more — it has gone since you chose it. Save this recording as it is.",
			consent.ID)
	case consent.Given() && at.ID != consent.ID:
		return routes.Route{}, fmt.Errorf("this would not replace the routine you "+
			"chose. You chose %q, and this recording would be saved as %q — "+
			"choose once more.", consent.ID, at.ID)
	}
	if !consent.Given() {
		// ── NOTHING WAS THERE A MOMENT AGO, AND THE WRITE SAYS SO ITSELF ──
		//
		// Inside the lock as well, because a fresh save and a replacement of the
		// same route are two writers of one file: serialising only the
		// replacements would leave the pair of them racing. The exclusive create
		// below would still refuse rather than overwrite — it is its own critical
		// section — and the lock is what makes the two of them queue instead of one
		// of them failing.
		//
		// The collision check above is two instants earlier. A route that appeared
		// in between belongs to whoever created it, and a caller that was told the
		// path was free has consented to nothing about it. The create refuses, and
		// what it refuses is reported as the collision it is.
		rt, err := d.saveWith(dr, scope, app, false)
		if os.IsExist(err) {
			if again := d.Collision(dr, scope, app); again.Says != "" {
				return rt, fmt.Errorf("%s", again.Says)
			}
			return rt, fmt.Errorf("a routine was saved at %s while this one was "+
				"being kept, so nothing has been written over it", RouteID(rt))
		}
		return rt, err
	}
	rt, err := d.Target(dr, scope, strings.TrimSpace(app))
	if err != nil {
		return routes.Route{}, err
	}
	// ── AND THE ROUTE IS HELD FOR THE WHOLE OF VALIDATE-AND-WRITE ──
	//
	// `Registry.Replace` is atomic, so nobody reads a half-written routine. That says
	// nothing about the gap BEFORE it: reading the digests, rendering, and writing are
	// three operations, and another save landing between the first and the last has its
	// version overwritten by a caller whose comparison was true when it was made.
	//
	// Making a comparison atomic is not something a file write can do. What closes it is
	// holding the route across all of it — the validation, the source and the
	// demonstration — so two saves of one routine serialise and the second revalidates
	// against what the first actually left. Every writer in this contract takes it; see
	// routes.Registry.Lock for why it is the operating system's lock and not a file of
	// our own.
	hold, err := d.Reg.Lock(rt, SaveWithin)
	if err != nil {
		return rt, err
	}
	defer hold.Release()
	// ── AND THE VERSION SOMEBODY AGREED TO LOSE IS THE ONE THAT IS STILL THERE ──
	//
	// The identity does not change when the contents do, so "same route, same id" is not
	// the same question. This used to be the whole of the check, which caught the routine
	// being DELETED and missed it being REPLACED: another save landing there left a newer
	// routine at the identity somebody had agreed to overwrite, and it was overwritten.
	// They agreed to lose the version they were shown.
	if src, rec := d.versionAt(rt); src != consent.Source || rec != consent.Recording {
		return rt, fmt.Errorf("%q has been saved again since you were shown it, so it "+
			"is left as it is — look at what is there now and choose once more",
			consent.ID)
	}
	// THE ONE INSTANT A TEST CAN STAND IN: after the comparison and before the write,
	// inside the lock. Nil in every production path, and the only way to make the
	// interleaving this whole section is about happen on purpose. See Deps.AfterValidating.
	if d.AfterValidating != nil {
		d.AfterValidating()
	}
	return d.saveWith(dr, scope, app, true)
}
