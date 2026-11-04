package orchestrator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// SHORTCUTS: ONE OWNER FOR ASSIGNING, CHANGING, REMOVING, LISTING AND CARRYING LEADER KEYS.
//
// Every surface — `marco bind`/`unbind`/`bindings`, the widget's `bind` word (which spawns `marco
// bind`), the control centre's Bindings tab, and Studio when it gains a control — comes here. What
// differs between them is only how they COLLECT the answer to "listened for where": the terminal
// and the widget read the foreground at the moment somebody types, the control centre and Studio
// state it. None of them decides anything else.
//
// The rules, each in one place:
//
//   - A shortcut names ROUTINE IDENTITIES, resolved once, here. A press performs those or refuses;
//     it never looks a name up again.
//   - A shortcut may be NARROWER than its routine and never wider (see `wider`).
//   - Replacing or removing one needs consent to the BINDING VERSION the person was shown,
//     re-checked under the bindings lock at the write.
//   - Reserved keys cannot be bound.
//   - Forgetting a routine removes its shortcuts; moving one carries them.
//   - A legacy, name-only entry does not run until it is reassigned.
//
// See docs/Surfaces-alignment-proposal-2026-11-02.md §0 (R4, R6) and §5.1, §5.8.

// reservedKeys can never be shortcuts: the widget's own command line and help, and the key that
// dismisses an armed leader. The configured leader is reserved too, and is passed in by the
// caller because only the caller knows it.
var reservedKeys = map[string]string{
	"m":   "the widget's command line",
	"h":   "the widget's help",
	"esc": "dismissing the leader",
}

// Assignment is one request to put a shortcut on a key.
type Assignment struct {
	// Key is the key pressed after the leader.
	Key string
	// ListenIn is the application the shortcut is listened for in, or "" for everywhere.
	ListenIn string
	// Steps are the routines it runs, in order — one, or a chain — and Cmd is the text the
	// person sees, one step per routine, possibly carrying configured values
	// (`enter freeplay with 3`). Cmd is the existing text when a legacy entry is reassigned.
	Steps []routes.Route
	Cmd   string
	// Leader is the configured leader key, which cannot itself be bound.
	Leader string
	// Consent is the VERSION of the binding on ListenIn+Key that the person was shown, or ""
	// for "it was free when I looked". Not a routine ID: the same routine can stay on a key
	// while its binding changes.
	Consent string
	// Changing, when set, is the shortcut this one REPLACES: a change of key or of where it is
	// listened for. It is taken off in the same write the new one is put on, and only if it
	// is still the version shown, so a change can never leave both keys, or neither.
	Changing *Slot
}

// Slot is one shortcut as somebody was shown it: where it is listened for, its key, and the
// version of the entry.
type Slot struct {
	ListenIn string
	Key      string
	Version  string
}

// ShortcutState is what is on one app+key now, for a surface to show before anything is written.
type ShortcutState struct {
	Exists  bool
	Binding routes.Binding
	// Version is what a replacement or removal must name as its consent.
	Version string
	// Says describes the entry in a sentence.
	Says string
}

// ShortcutAt reads one app+key as it stands.
func (d Deps) ShortcutAt(listenIn, key string) ShortcutState {
	b, ok := d.Reg.BindingAt(listenIn, strings.ToLower(key))
	if !ok {
		return ShortcutState{}
	}
	return ShortcutState{Exists: true, Binding: b, Version: b.Version(),
		Says: fmt.Sprintf("`%s %s runs %s", b.Key, listenedFor(b.App), describe(b))}
}

// WhyNotShortcut is why this assignment can never be made, before any collision is considered.
func (d Deps) WhyNotShortcut(a Assignment) string {
	key := strings.ToLower(strings.TrimSpace(a.Key))
	switch {
	case key == "":
		return "say which key"
	case len(a.Steps) == 0:
		return "say which routine the shortcut runs"
	case strings.EqualFold(key, strings.TrimSpace(a.Leader)) && a.Leader != "":
		return fmt.Sprintf("`%s is the leader key itself", key)
	}
	if what, ok := reservedKeys[key]; ok {
		return fmt.Sprintf("`%s is %s, so a shortcut on it could never run", key, what)
	}
	for _, rt := range a.Steps {
		if !d.Reg.Has(rt) {
			return fmt.Sprintf("there is no routine %s", rt.ID())
		}
		if why := wider(rt, a.ListenIn); why != "" {
			return why
		}
	}
	return ""
}

// wider is why a shortcut listened for in listenIn would be wider than routine rt allows.
//
// An Only-in routine works only while its application is in front, so its shortcut may only be
// listened for there. The other two scopes work from anywhere, so their shortcuts may be listened
// for anywhere or in any one application.
func wider(rt routes.Route, listenIn string) string {
	if rt.App == "" || rt.Focus {
		return ""
	}
	if sameApplication(listenIn, rt.App) {
		return ""
	}
	where := "everywhere"
	if listenIn != "" {
		where = "in " + listenIn
	}
	return fmt.Sprintf("%q is %q, so a shortcut for it cannot be listened for %s — "+
		"only in %s", plays.Pretty(rt.Slug), ScopeLabel(ScopeApp, rt.App), where, rt.App)
}

// AssignShortcut puts a shortcut on a key, or says why it did not.
//
// THE COLLISION IS DECIDED UNDER THE LOCK, against the consent the caller carries: an empty
// consent means "it was free when I looked" and is refused if anything is there now; a version
// means "replace this one" and is refused if what is there now is any other version, or nothing.
func (d Deps) AssignShortcut(a Assignment) (routes.Binding, error) {
	// ── THE ROUTINES ARE HELD WHILE THEY ARE CHECKED AND WHILE THE SHORTCUT IS WRITTEN ──
	//
	// Route locks first, in identity order, then the bindings lock: the one documented order.
	// A check made without them is a check of an instant that is over by the write — a forget
	// or a move completing in between would leave this shortcut naming an identity with
	// nothing there, which the next routine saved there would inherit. Held, a forget or a
	// move of these routines waits for this write, and then removes or carries it.
	//
	// Deleting the locks must fail TestAnAssignmentInterleavedWithAForgetLeavesNoOrphan.
	release, err := d.LockRoutes(a.Steps)
	if err != nil {
		return routes.Binding{}, fmt.Errorf("not bound: %w", err)
	}
	defer release()
	if why := d.WhyNotShortcut(a); why != "" {
		return routes.Binding{}, fmt.Errorf("not bound: %s", why)
	}
	if d.AfterShortcutChecked != nil {
		d.AfterShortcutChecked()
	}
	key := strings.ToLower(strings.TrimSpace(a.Key))
	ids := make([]string, len(a.Steps))
	for i, rt := range a.Steps {
		ids[i] = rt.ID()
	}
	cmd := strings.TrimSpace(a.Cmd)
	if cmd == "" {
		names := make([]string, len(a.Steps))
		for i, rt := range a.Steps {
			names[i] = plays.Pretty(rt.Slug)
		}
		cmd = strings.Join(names, " then ")
	}
	if n := len(routes.SplitChain(cmd)); n != len(ids) {
		return routes.Binding{}, fmt.Errorf("not bound: %q has %d steps and %d routines "+
			"were named", cmd, n, len(ids))
	}
	nb := routes.Binding{App: a.ListenIn, Key: key, Cmd: cmd, Routes: ids}
	var from *Slot
	if a.Changing != nil {
		c := *a.Changing
		c.Key = strings.ToLower(strings.TrimSpace(c.Key))
		from = &c
	}
	err = d.Reg.UpdateBindings(SaveWithin, func(bs []routes.Binding) ([]routes.Binding, error) {
		var out []routes.Binding
		var there *routes.Binding
		changed := false
		for _, b := range bs {
			if from != nil && b.App == from.ListenIn && b.Key == from.Key {
				// THE ONE BEING CHANGED goes in this write, and only as it was shown.
				if b.Version() != from.Version {
					return nil, fmt.Errorf("not changed: `%s %s has changed since "+
						"you were shown it — it now runs %s; look again", b.Key,
						listenedFor(b.App), describe(b))
				}
				// When it is also the slot being written, its own version is the
				// consent, so nothing further is asked about it below.
				changed = true
				continue
			}
			if b.App == a.ListenIn && b.Key == key {
				bb := b
				there = &bb
				continue
			}
			out = append(out, b)
		}
		if from != nil && !changed {
			return nil, fmt.Errorf("not changed: the shortcut on `%s %s that you were "+
				"changing has gone since you were shown it; look again", from.Key,
				listenedFor(from.ListenIn))
		}
		switch {
		case there != nil && a.Consent == "":
			return nil, fmt.Errorf("not bound: `%s %s already runs %s — say to "+
				"replace it", key, listenedFor(a.ListenIn), describe(*there))
		case there != nil && there.Version() != a.Consent:
			return nil, fmt.Errorf("not bound: `%s %s has changed since you were "+
				"shown it — it now runs %s; look again", key,
				listenedFor(a.ListenIn), describe(*there))
		case there == nil && a.Consent != "":
			return nil, fmt.Errorf("not bound: the shortcut on `%s %s that you "+
				"agreed to replace has gone since you were shown it; look again",
				key, listenedFor(a.ListenIn))
		}
		return append(out, nb), nil
	})
	if err != nil {
		return routes.Binding{}, err
	}
	return nb, nil
}

// RemoveShortcut takes a shortcut off a key, with consent to the version that was shown.
func (d Deps) RemoveShortcut(listenIn, key, consent string) (routes.Binding, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	var gone routes.Binding
	err := d.Reg.UpdateBindings(SaveWithin, func(bs []routes.Binding) ([]routes.Binding, error) {
		var out []routes.Binding
		found := false
		for _, b := range bs {
			if b.App == listenIn && b.Key == key {
				if b.Version() != consent {
					return nil, fmt.Errorf("not removed: `%s %s has changed since "+
						"you were shown it — it now runs %s; look again", key,
						listenedFor(listenIn), describe(b))
				}
				gone, found = b, true
				continue
			}
			out = append(out, b)
		}
		if !found {
			return nil, fmt.Errorf("not removed: there is no shortcut on `%s %s",
				key, listenedFor(listenIn))
		}
		return out, nil
	})
	return gone, err
}

// ForgetShortcutsOf removes every shortcut that runs rt, because rt is being forgotten (R6).
//
// A chain that includes rt goes as a whole: what is left of it is not what anybody bound. Legacy
// entries are matched by name, because they name nothing else — and removing one whose words
// would have reached this routine is the safe direction. Returns what was removed.
func (d Deps) ForgetShortcutsOf(rt routes.Route) ([]routes.Binding, error) {
	id := rt.ID()
	var gone []routes.Binding
	err := d.Reg.UpdateBindings(SaveWithin, func(bs []routes.Binding) ([]routes.Binding, error) {
		var out []routes.Binding
		for _, b := range bs {
			if runs(b, id) || (b.Legacy() && legacyNames(b, rt)) {
				gone = append(gone, b)
				continue
			}
			out = append(out, b)
		}
		return out, nil
	})
	return gone, err
}

// dropShortcutsNaming removes the identity shortcuts that run rt, and only those — no legacy
// entry, because a legacy entry names words, and the words still name whatever holds that name.
func (d Deps) dropShortcutsNaming(rt routes.Route) ([]routes.Binding, error) {
	id := rt.ID()
	var gone []routes.Binding
	err := d.Reg.UpdateBindings(SaveWithin, func(bs []routes.Binding) ([]routes.Binding, error) {
		var out []routes.Binding
		for _, b := range bs {
			if runs(b, id) {
				gone = append(gone, b)
				continue
			}
			out = append(out, b)
		}
		return out, nil
	})
	return gone, err
}

// CarryShortcuts points every shortcut that runs `from` at `to`, because the routine moved.
//
// Called with the route locks of the move HELD and taking only the bindings lock: the documented
// order is route then bindings. A shortcut carried to a routine whose scope it is now wider than
// is kept — removing it would lose somebody's binding silently — and refuses at the press, where
// the door says why; `ListShortcuts` marks it.
func (d Deps) CarryShortcuts(from, to routes.Route) (int, error) {
	fromID, toID := from.ID(), to.ID()
	n := 0
	err := d.Reg.UpdateBindings(SaveWithin, func(bs []routes.Binding) ([]routes.Binding, error) {
		for i := range bs {
			for j, id := range bs[i].Routes {
				if id == fromID {
					bs[i].Routes = append([]string(nil), bs[i].Routes...)
					bs[i].Routes[j] = toID
					n++
				}
			}
		}
		return bs, nil
	})
	return n, err
}

func runs(b routes.Binding, id string) bool {
	for _, r := range b.Routes {
		if r == id {
			return true
		}
	}
	return false
}

// legacyNames is whether a legacy entry's words name rt in any step.
func legacyNames(b routes.Binding, rt routes.Route) bool {
	for _, step := range routes.SplitChain(b.Command()) {
		base, _, _ := routes.ParseInvocation(step)
		if routes.Slug(base) == rt.Slug {
			return true
		}
	}
	return false
}

// LockRoutes takes the route locks for rts, each once, in identity order — the order every caller
// that holds more than one uses, so two of them cannot each hold what the other waits for.
func (d Deps) LockRoutes(rts []routes.Route) (func(), error) {
	byID := map[string]routes.Route{}
	for _, rt := range rts {
		byID[rt.ID()] = rt
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var holds []*routes.Hold
	release := func() {
		for i := len(holds) - 1; i >= 0; i-- {
			holds[i].Release()
		}
	}
	for _, id := range ids {
		h, err := d.Reg.Lock(byID[id], SaveWithin)
		if err != nil {
			release()
			return func() {}, err
		}
		holds = append(holds, h)
	}
	return release, nil
}

// ── FORGETTING A ROUTINE ──

// Forgotten is what a forget removed, part by part, so a partial one can be said honestly.
type Forgotten struct {
	Route     routes.Route
	Shortcuts []routes.Binding
	// Removed is whether the routine itself is gone.
	Removed bool
}

// ForgetRoutine forgets one routine: its shortcuts FIRST, then its provenance, then the routine.
//
// # The order is the guarantee
//
// Under the routine's lock, so nothing can save at its identity or assign a shortcut to it while
// this runs; then the bindings lock (route then bindings). The shortcuts go BEFORE the routine:
// a forget whose shortcut cleanup came second, and failed, would leave keys naming an identity
// with nothing there — and the next routine saved under that name would start answering them.
// So a cleanup that fails forgets NOTHING; and a routine removal that fails after its shortcuts
// went leaves a routine with no shortcuts, which can redirect nothing, and is said.
//
// Reordering must fail TestAForgetWhoseShortcutsCannotBeRemovedForgetsNothing.
func (d Deps) ForgetRoutine(rt routes.Route) (Forgotten, error) { return d.forget(rt, "") }

// ForgetAsPlanned is ForgetRoutine for a forget somebody was SHOWN: it refuses, having removed
// nothing, if the routine, its demonstration or its shortcuts are not what the plan listed.
// Checked under the routine's lock, so nothing saved or assigned after the review is removed
// without having been shown.
//
// Skipping the check must fail TestAForgetRemovesNothingItDidNotShow.
func (d Deps) ForgetAsPlanned(p ForgetPlan) (Forgotten, error) {
	if p.Token == "" {
		return Forgotten{Route: p.Route}, fmt.Errorf("this forget was never reviewed, so nothing was removed")
	}
	return d.forget(p.Route, p.Token)
}

func (d Deps) forget(rt routes.Route, shown string) (Forgotten, error) {
	out := Forgotten{Route: rt}
	hold, err := d.Reg.Lock(rt, SaveWithin)
	if err != nil {
		return out, fmt.Errorf("%s was not forgotten: %w", rt.ID(), err)
	}
	defer hold.Release()
	if !d.Reg.Has(rt) {
		return out, fmt.Errorf("there is no routine %s to forget", rt.ID())
	}
	if shown != "" && d.planOf(rt).Token != shown {
		return out, fmt.Errorf("%s has changed since you looked at what forgetting it would "+
			"remove, so nothing was removed — look again", rt.ID())
	}
	if d.BeforeForgetWrites != nil {
		d.BeforeForgetWrites()
	}
	gone, err := d.ForgetShortcutsOf(rt)
	if err != nil {
		return out, fmt.Errorf("%s was not forgotten: its shortcuts could not be removed "+
			"first (%v), and forgetting it without them would leave them for the next "+
			"routine saved under its name", rt.ID(), err)
	}
	out.Shortcuts = gone
	if err := d.Reg.DeleteOrigin(rt); err != nil {
		return out, fmt.Errorf("%s was not forgotten: %d shortcut(s) were removed, and its "+
			"provenance could not be (%v), so the routine is still there", rt.ID(),
			len(gone), err)
	}
	if err := d.Reg.Delete(rt); err != nil {
		return out, fmt.Errorf("%s was not forgotten: %d shortcut(s) and its provenance "+
			"were removed, and the routine itself could not be (%v)", rt.ID(), len(gone), err)
	}
	out.Removed = true
	return out, nil
}

// ── PRESSING ──

// PressedStep is one routine a press will perform, with the words its values come from.
type PressedStep struct {
	Route routes.Route
	Text  string
	// Pin is the digest of the routine's source as this press read it, under its lock, while
	// the shortcut still named it. The run refuses bytes that differ: a routine published at
	// this identity after the press resolved it is not the routine the key was bound to.
	Pin string
}

// Pressed is what a press of a key should perform here, or why it performs nothing.
type Pressed struct {
	Found   bool
	Binding routes.Binding
	Steps   []PressedStep
	// Refused is the sentence when the binding exists and must not run.
	Refused string
}

// ShortcutPressed resolves a press of key with app in front, WITHOUT LOOKING ANY NAME UP.
//
// An app binding first, then a global one. A legacy entry refuses with the reassign sentence. A
// step whose routine no longer exists refuses naming it. Each step's text is handed back only so
// its configured values can be read; its words never choose the routine.
func (d Deps) ShortcutPressed(app, key string) Pressed {
	b, ok := d.Reg.BindingFor(app, key)
	if !ok {
		return Pressed{}
	}
	p := Pressed{Found: true, Binding: b}
	if b.Legacy() {
		p.Refused = fmt.Sprintf("`%s %s was set up by name (%q) before shortcuts named a "+
			"routine, so it does not run until it is reassigned — `marco bind --reassign "+
			"%s` shows which routine it would run and asks", b.Key, listenedFor(b.App),
			b.Command(), b.Key)
		return p
	}
	texts := routes.SplitChain(b.Command())
	if len(texts) != len(b.Routes) {
		p.Refused = fmt.Sprintf("`%s %s names %d routines for %d steps, so it is not run",
			b.Key, listenedFor(b.App), len(b.Routes), len(texts))
		return p
	}
	rts := make([]routes.Route, len(b.Routes))
	for i, id := range b.Routes {
		rt, err := routes.ParseID(id)
		if err != nil {
			p.Refused = fmt.Sprintf("`%s %s: %v", b.Key, listenedFor(b.App), err)
			return p
		}
		rts[i] = rt
	}
	// ── UNDER THE ROUTINES' LOCKS, THE SHORTCUT IS READ AGAIN AND EACH ROUTINE IS PINNED ──
	//
	// A move that replaces a routine invalidates its shortcuts under the same locks before it
	// publishes anything, so here either the shortcut still names the routine that is there —
	// and its bytes are pinned — or it has gone. A press that read the shortcut a moment
	// before such a move would otherwise run whatever the move then published. The run refuses
	// bytes other than the pinned ones.
	if d.BetweenPressReads != nil {
		d.BetweenPressReads()
	}
	release, err := d.LockRoutes(rts)
	if err != nil {
		p.Refused = fmt.Sprintf("`%s %s was not run: %v", b.Key, listenedFor(b.App), err)
		return p
	}
	defer release()
	if again, ok := d.Reg.BindingFor(app, key); !ok || again.Version() != b.Version() {
		p.Refused = fmt.Sprintf("`%s %s changed while it was being pressed, so nothing was "+
			"performed — press it again", b.Key, listenedFor(b.App))
		return p
	}
	for i, rt := range rts {
		src, err := os.ReadFile(d.Reg.Path(rt))
		if err != nil || !d.Reg.Has(rt) {
			p.Refused = fmt.Sprintf("`%s %s ran %s, which is no longer in the library — "+
				"nothing was performed. Change or remove the shortcut", b.Key,
				listenedFor(b.App), rt.ID())
			return p
		}
		p.Steps = append(p.Steps, PressedStep{Route: rt, Text: texts[i],
			Pin: routes.DigestOf(string(src))})
	}
	return p
}

// ── LISTING ──

// ListedShortcut is one row of what is bound.
type ListedShortcut struct {
	Key      string   `json:"key"`
	ListenIn string   `json:"listen_in"` // "" = everywhere
	Cmd      string   `json:"cmd"`
	Routes   []string `json:"routes,omitempty"`
	Version  string   `json:"version"`
	// State is "ok", "needs-reassigning" (legacy), "stale" (a routine is gone) or "wider"
	// (it is listened for where its routine cannot run).
	State string `json:"state"`
	Says  string `json:"says"`
	// Shadowed names the app binding that hides this global one, when listing for an app.
	Shadowed bool `json:"shadowed,omitempty"`
}

// ListShortcuts is every shortcut, or — with here non-empty — what each key fires with that app in
// front, app bindings first and globals they do not hide after them.
func (d Deps) ListShortcuts(here string, onlyHere bool) []ListedShortcut {
	all := d.Reg.Bindings()
	var out []ListedShortcut
	appKeys := map[string]bool{}
	for _, b := range all {
		if b.App == here && here != "" {
			appKeys[b.Key] = true
		}
	}
	for _, b := range all {
		if onlyHere && b.App != "" && b.App != here {
			continue
		}
		row := ListedShortcut{Key: b.Key, ListenIn: b.App, Cmd: b.Command(),
			Routes: b.Routes, Version: b.Version(), State: "ok"}
		row.Says = fmt.Sprintf("`%s %s runs %s", b.Key, listenedFor(b.App), describe(b))
		switch p := d.ShortcutPressed(b.App, b.Key); {
		case b.Legacy():
			row.State = "needs-reassigning"
			row.Says = p.Refused
		case p.Refused != "":
			row.State = "stale"
			row.Says = p.Refused
		default:
			for _, s := range p.Steps {
				if why := wider(s.Route, b.App); why != "" {
					row.State, row.Says = "wider", why
				}
			}
		}
		if b.App == "" && appKeys[b.Key] {
			row.Shadowed = true
		}
		if onlyHere && row.Shadowed {
			continue
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].ListenIn == "") != (out[j].ListenIn == "") {
			return out[i].ListenIn != ""
		}
		if out[i].ListenIn != out[j].ListenIn {
			return out[i].ListenIn < out[j].ListenIn
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// shortcutsKeptSays adds to a replacement's sentence the shortcuts that run rt and will run what
// replaces it. Unreadable shortcuts are said as that, never as none.
func (d Deps) shortcutsKeptSays(rt routes.Route, says string) ([]string, string, bool) {
	bs, err := d.Reg.ReadBindings()
	if err != nil {
		return nil, says + " Whether it has shortcuts could not be read; any it has are kept.", true
	}
	var keys []string
	for _, b := range bs {
		if runs(b, rt.ID()) {
			keys = append(keys, fmt.Sprintf("%s %s", b.Key, listenedFor(b.App)))
		}
	}
	switch len(keys) {
	case 0:
		return nil, says, false
	case 1:
		return keys, says + fmt.Sprintf(" Its shortcut (%s) is kept and will run the new "+
			"version.", keys[0]), false
	default:
		return keys, says + fmt.Sprintf(" Its shortcuts (%s) are kept and will run the new "+
			"version.", strings.Join(keys, "; ")), false
	}
}

func listenedFor(app string) string {
	if app == "" {
		return "everywhere"
	}
	return "in " + app
}

func describe(b routes.Binding) string {
	if b.Legacy() {
		return fmt.Sprintf("%q (by name)", b.Command())
	}
	return fmt.Sprintf("%q (%s)", b.Command(), strings.Join(b.Routes, ", "))
}

// ── RENAMING A FILE IN PLACE ──

// RenameInPlace renames a routine's files, carrying its shortcuts, for the callers that rename
// without a review (`marco rename`; the reviewed move is `Rescope`).
//
// # The original stays until the new name holds it AND its shortcuts
//
// Under both routes' locks, in identity order, then the bindings lock. The steps are:
//
//  1. CREATE the destination as an exact copy of the source, exclusively. The shortcuts still
//     name the original, which still exists.
//  2. CARRY the shortcuts to the destination, which now holds the same routine.
//  3. FINISH: move the original's files over the copy, which removes the original.
//
// So at no instant does a shortcut name an identity that does not hold the routine it was bound
// to, and nothing has to be rolled back to make that true. A failure at 1 or 2 leaves the
// shortcuts on the original and DELETES NOTHING: whatever is at the new name is left and reported
// (see failedRename), because its bytes cannot show who wrote it. A later routine saved there
// answers nobody's key. A failure at 3 is a finished rename with the original left over, and the
// error says so.
//
// The previous order carried first and rolled back on failure: a rename and a rollback that both
// failed left shortcuts naming an empty identity, which the next routine saved there would have
// answered. Restoring it must fail TestAFailedRenameLeavesNoShortcutForTheNextRoutineThere.
func (d Deps) RenameInPlace(from, to routes.Route) error {
	release, err := d.LockRoutes([]routes.Route{from, to})
	if err != nil {
		return err
	}
	defer release()
	switch {
	case !d.Reg.Has(from):
		return fmt.Errorf("there is no routine %s", from.ID())
	case d.Reg.Has(to):
		return fmt.Errorf("a routine %s already exists", to.ID())
	}
	fault := func(step string) error {
		if d.RenameFault != nil {
			return d.RenameFault(step)
		}
		return nil
	}
	raw, err := os.ReadFile(d.Reg.Path(from))
	if err != nil {
		return fmt.Errorf("%s was not renamed: it could not be read (%v)", from.ID(), err)
	}
	src := string(raw)

	// 1. CREATE, exclusively. A file that appeared there is somebody else's and is left alone.
	err = fault("create")
	if err == nil {
		err = d.Reg.Create(to, src)
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%s was not renamed: a routine %s appeared while it was being "+
				"renamed", from.ID(), to.ID())
		}
	}
	if err != nil {
		return d.failedRename(from, to, err)
	}

	// 2. CARRY, while the routine is in both places.
	err = fault("carry")
	if err == nil {
		_, err = d.CarryShortcuts(from, to)
	}
	if err != nil {
		return d.failedRename(from, to,
			fmt.Errorf("its shortcuts could not be moved (%v)", err))
	}

	// 3. FINISH. Moving the original's files over the copy is what removes the original.
	err = fault("finish")
	if err == nil {
		err = d.Reg.Rename(from, to)
	}
	if err != nil {
		return fmt.Errorf("%s was renamed to %s and its shortcuts now run it there, but the "+
			"original could not be removed (%v): it is still at %s with no shortcuts, and "+
			"its recording and history stayed with it", from.ID(), to.ID(), err, from.ID())
	}
	return nil
}

// failedRename is the sentence for a rename that stopped at step 1 or 2. It changes nothing.
//
// Whatever is at the new name is LEFT THERE. This rename may have written it, or another writer
// may have: nothing about a file's bytes says who wrote it (an empty file, or any prefix of the
// source, is what a partial copy looks like and also what somebody else's file can be), so it is
// not deleted on that evidence. It is reported instead: a routine no shortcut names, which runs
// only when somebody asks for it by name. The shortcuts stay on the original.
func (d Deps) failedRename(from, to routes.Route, why error) error {
	switch _, err := os.Lstat(d.Reg.Path(to)); {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s was not renamed: %v", from.ID(), why)
	case err != nil:
		return fmt.Errorf("%s was not renamed (%v), and whether anything was left at %s "+
			"could not be read (%v). No shortcut names it; the shortcuts still run %s",
			from.ID(), why, to.ID(), err, from.ID())
	default:
		return fmt.Errorf("%s was not renamed (%v). A file was left at %s and has not been "+
			"touched, because it cannot be shown to be this rename's copy; check it and "+
			"remove it if it is not wanted. No shortcut names it; the shortcuts still run %s",
			from.ID(), why, to.ID(), from.ID())
	}
}

// WiderThanItsRoutine is why a shortcut for rt listened for in listenIn ("" everywhere) would be
// wider than rt allows, or "" when it would not. The owner's own rule, for a surface that offers
// the choice before anybody makes it.
func WiderThanItsRoutine(rt routes.Route, listenIn string) string { return wider(rt, listenIn) }

// ForgetPlan is what forgetting one routine would remove, read before anything is, and sealed.
type ForgetPlan struct {
	Route routes.Route
	// Shortcuts are the keys that run it, which go first.
	Shortcuts []routes.Binding
	// Demonstration is whether its recording is kept beside it, and Steps whether an edited
	// step account is.
	Demonstration, Steps bool
	// Provenance is whether a provenance record is kept beside it.
	Provenance bool
	// Phrases are what else it answers to.
	Phrases []string
	// Token seals all of the above with the routine's source and demonstration as they are.
	Token string
}

// PlanForget reads what forgetting rt would remove. It writes nothing.
func (d Deps) PlanForget(rt routes.Route) (ForgetPlan, error) {
	if !d.Reg.Has(rt) {
		return ForgetPlan{Route: rt}, fmt.Errorf("there is no routine %s to forget", rt.ID())
	}
	return d.planOf(rt), nil
}

func (d Deps) planOf(rt routes.Route) ForgetPlan {
	p := ForgetPlan{Route: rt, Demonstration: d.Reg.HasRecording(rt), Steps: d.Reg.HasSteps(rt)}
	if _, err := os.Stat(d.Reg.OriginPath(rt)); err == nil {
		p.Provenance = true
	}
	for _, b := range d.Reg.Bindings() {
		for _, id := range b.Routes {
			if id == rt.ID() {
				p.Shortcuts = append(p.Shortcuts, b)
				break
			}
		}
	}
	if a, st := d.Reg.Aliases(rt); st == routes.AliasesIntact {
		p.Phrases = a.Phrases
	}
	src, _ := os.ReadFile(d.Reg.Path(rt))
	rec, _ := d.Reg.LoadRecording(rt)
	parts := []string{rt.ID(), routes.DigestOf(string(src)), routes.DigestOf(string(rec))}
	for _, b := range p.Shortcuts {
		parts = append(parts, b.App+"\x01"+b.Key+"\x01"+strings.Join(b.Routes, ","))
	}
	parts = append(parts, strings.Join(p.Phrases, "\x01"))
	p.Token = routes.DigestOf(strings.Join(parts, "\x00"))
	return p
}
