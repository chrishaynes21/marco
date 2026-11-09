package main

// A ROUTINE'S SHORTCUTS, FROM STUDIO (S3 of docs/Surfaces-alignment-proposal-2026-11-02.md).
//
// Every rule is S2's owner, `internal/orchestrator` shortcuts.go: which keys can be bound, how
// narrow or wide a shortcut may be, the consent a replacement or a removal needs, the locks. This
// file decides nothing of that. It reads what is there for one routine, hands the person's choice
// to the owner, and returns what the owner said, read back off disk.
//
// Three things are Studio's own, and each is said where it is decided:
//
//   - WHERE A NEW ONE IS LISTENED FOR IS STATED, NEVER INFERRED. Studio owns the foreground while
//     somebody is using it, so "the application in front" would always be Studio (ruling R4). A
//     routine about an application offers that application first; "everywhere" is always an
//     explicit choice; a routine about no application has no default at all.
//   - A KEY IN USE IS SHOWN, NOT REPLACED. A Set onto a taken key writes nothing and comes back
//     with what is there and its VERSION; only a second Set carrying that version replaces it,
//     and the owner checks the version again under its lock.
//   - A SHORTCUT SET UP BY NAME by an older build is shown on the routine its words name, with
//     exactly the routines it would run, and is reassigned only when the person says so for
//     exactly those (`plan`). It is never migrated by being looked at.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// ShortcutKey is one shortcut that runs, or by its words would run, the routine on screen.
type ShortcutKey struct {
	Key      string `json:"key"`
	ListenIn string `json:"listen_in"`
	// Pressed is the key as somebody presses it, leader first ("`e"), and Where is where it
	// is listened for, in words.
	Pressed string `json:"pressed"`
	Where   string `json:"where"`
	// Version is what a change, a removal or a reassignment must carry as its consent.
	Version string `json:"version"`
	// Runs is what it runs, in words, and Cmd the text it was set up with.
	Runs string `json:"runs"`
	Cmd  string `json:"cmd"`
	// State is the owner's: "ok", "needs-reassigning", "stale" or "wider"; Says is its sentence
	// when it is not ok.
	State string `json:"state"`
	Says  string `json:"says,omitempty"`
	// Shadows is, for a shortcut listened for everywhere, the applications where the same key
	// runs something else instead. Said, not refused.
	Shadows string `json:"shadows,omitempty"`
	// Plan, for a shortcut set up by name, is the identities a reassignment would make it run,
	// joined by " then ", and Reassignable whether it can be; Why is why not.
	Plan         string `json:"plan,omitempty"`
	PlanSays     string `json:"plan_says,omitempty"`
	Reassignable bool   `json:"reassignable,omitempty"`
	Why          string `json:"why,omitempty"`
}

// ShortcutPlace is one answer to "listened for where?".
type ShortcutPlace struct {
	// ListenIn is the application, or "" for everywhere. Named means the person types the
	// application.
	ListenIn string `json:"listen_in"`
	Named    bool   `json:"named,omitempty"`
	Label    string `json:"label"`
	Allowed  bool   `json:"allowed"`
	Why      string `json:"why,omitempty"`
	// Default is the place offered first. At most one, and none for a routine about no
	// application, where everywhere has to be chosen.
	Default bool `json:"default,omitempty"`
}

// ShortcutTaken is a key a Set found in use: what is there, and the version a replacement must
// name.
type ShortcutTaken struct {
	Key      string `json:"key"`
	ListenIn string `json:"listen_in"`
	Pressed  string `json:"pressed"`
	Where    string `json:"where"`
	Version  string `json:"version"`
	Runs     string `json:"runs"`
}

// Shortcuts is everything the Shortcut tab shows for one routine.
type Shortcuts struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Leader is the widget's configured leader, and LeaderSays it in words.
	Leader     string `json:"leader"`
	LeaderSays string `json:"leader_says"`
	// Keys run this routine; Legacy were set up by name and their words name it.
	Keys   []ShortcutKey   `json:"keys"`
	Legacy []ShortcutKey   `json:"legacy"`
	Places []ShortcutPlace `json:"places"`
	// Askable is whether anything may be changed; Why is why not.
	Askable bool   `json:"askable"`
	Why     string `json:"why,omitempty"`
	// Said is what the last change came to, and Failed whether it was a refusal.
	Said   string `json:"said,omitempty"`
	Failed bool   `json:"failed,omitempty"`
	// Taken is a key in use that the last Set found, to be agreed to or avoided.
	Taken *ShortcutTaken `json:"taken,omitempty"`
}

// RoutineShortcuts is the routine's shortcuts as they are now.
func (a *Acts) RoutineShortcuts(id string) Shortcuts {
	out, _, _ := a.shortcutsFor(id)
	return out
}

// SetShortcut puts a new shortcut on key, listened for in listenIn ("" everywhere).
//
// consent is "" for "it was free when I looked", or the version of the shortcut on that key the
// person was shown and agreed to replace.
func (a *Acts) SetShortcut(id, key, listenIn, consent string) Shortcuts {
	return a.assign(id, key, listenIn, consent, nil)
}

// ChangeShortcut moves the shortcut on fromKey (listened for in fromListenIn, as the version
// shown) to key and listenIn, in one write: the old one comes off only as the new one goes on.
func (a *Acts) ChangeShortcut(id, fromListenIn, fromKey, fromVersion, key, listenIn,
	consent string) Shortcuts {

	return a.assign(id, key, listenIn, consent, &orchestrator.Slot{ListenIn: fromListenIn,
		Key: fromKey, Version: fromVersion})
}

func (a *Acts) assign(id, key, listenIn, consent string, from *orchestrator.Slot) Shortcuts {
	out, d, ok := a.shortcutsFor(id)
	if !ok {
		return out
	}
	chosen, _ := a.routineWithID(id)
	rt := chosen.route()
	key = strings.ToLower(strings.TrimSpace(key))
	listenIn = strings.TrimSpace(listenIn)
	asked := orchestrator.Assignment{Key: key, ListenIn: listenIn, Steps: []routes.Route{rt},
		Leader: out.Leader, Consent: consent, Changing: from}
	if why := d.WhyNotShortcut(asked); why != "" {
		out.Failed, out.Said = true, "Not set: "+why+"."
		return out
	}
	// A KEY IN USE IS SHOWN FIRST. Unless it is the very shortcut being changed, in which case
	// its own version is the consent and the owner takes it off in the same write.
	same := from != nil && from.ListenIn == listenIn && strings.EqualFold(from.Key, key)
	if consent == "" && !same {
		if st := d.ShortcutAt(listenIn, key); st.Exists {
			out.Taken = &ShortcutTaken{Key: key, ListenIn: listenIn,
				Pressed: widgetconfig.Pressed(out.Leader, key), Where: placeWords(listenIn),
				Version: st.Version, Runs: runsWords(st.Binding)}
			return out
		}
	}
	// AND A BINDING THAT ALREADY RUNS THIS ROUTINE, WHEN CHANGED, KEEPS ITS WORDS: they can
	// carry configured values (`enter freeplay with 3`), which are the person's.
	if from != nil {
		if b, ok := d.Reg.BindingAt(from.ListenIn, strings.ToLower(from.Key)); ok &&
			b.Version() == from.Version {
			asked.Cmd = b.Command()
			asked.Steps = stepsOf(b, rt)
		}
	}
	b, err := d.AssignShortcut(asked)
	again, _, _ := a.shortcutsFor(id)
	if err != nil {
		again.Failed, again.Said = true, sentence(err.Error())
		return again
	}
	verb := "now runs"
	if from != nil {
		verb = "now runs, instead of " + widgetconfig.Pressed(out.Leader, from.Key) + " " +
			placeWords(from.ListenIn) + ","
	}
	again.Said = fmt.Sprintf("%s %s %s %q.", widgetconfig.Pressed(out.Leader, b.Key),
		placeWords(b.App), verb, chosen.Name)
	return again
}

// RemoveShortcut takes the shortcut on key, listened for in listenIn, off — only as the version
// shown.
func (a *Acts) RemoveShortcut(id, listenIn, key, version string) Shortcuts {
	out, d, ok := a.shortcutsFor(id)
	if !ok {
		return out
	}
	gone, err := d.RemoveShortcut(strings.TrimSpace(listenIn), key, version)
	again, _, _ := a.shortcutsFor(id)
	if err != nil {
		again.Failed, again.Said = true, sentence(err.Error())
		return again
	}
	again.Said = fmt.Sprintf("%s %s no longer runs anything.",
		widgetconfig.Pressed(out.Leader, gone.Key), placeWords(gone.App))
	return again
}

// ReassignShortcut makes a shortcut set up by name run exactly the routines in plan — the plan the
// person was shown, which is checked against what its words reach now — keeping its words.
func (a *Acts) ReassignShortcut(id, listenIn, key, version, plan string) Shortcuts {
	out, d, ok := a.shortcutsFor(id)
	if !ok {
		return out
	}
	chosen, _ := a.routineWithID(id)
	listenIn = strings.TrimSpace(listenIn)
	st := d.ShortcutAt(listenIn, key)
	if !st.Exists || !st.Binding.Legacy() || st.Version != version {
		out.Failed, out.Said = true, fmt.Sprintf("Not reassigned: %s %s has changed since "+
			"you were shown it. Look again.", widgetconfig.Pressed(out.Leader, key),
			placeWords(listenIn))
		return out
	}
	steps, nowPlan, why := planFor(d, st.Binding, chosen.route())
	if why != "" || nowPlan != plan {
		if why == "" {
			why = fmt.Sprintf("its words now reach %s, not what you were shown", nowPlan)
		}
		out.Failed, out.Said = true, "Not reassigned: "+why+"."
		return out
	}
	b, err := d.AssignShortcut(orchestrator.Assignment{Key: st.Binding.Key, ListenIn: listenIn,
		Steps: steps, Cmd: st.Binding.Command(), Leader: out.Leader, Consent: version})
	again, _, _ := a.shortcutsFor(id)
	if err != nil {
		again.Failed, again.Said = true, sentence(err.Error())
		return again
	}
	again.Said = fmt.Sprintf("%s %s now runs exactly %s.",
		widgetconfig.Pressed(out.Leader, b.Key), placeWords(b.App),
		strings.Join(b.Routes, " then "))
	return again
}

// shortcutsFor reads one routine's shortcuts, or says why it cannot.
func (a *Acts) shortcutsFor(id string) (Shortcuts, orchestrator.Deps, bool) {
	id = strings.TrimSpace(id)
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	leader := widgetconfig.Leader()
	out := Shortcuts{ID: id, Leader: leader, LeaderSays: leaderSays(leader),
		Keys: []ShortcutKey{}, Legacy: []ShortcutKey{}, Places: []ShortcutPlace{}}
	if id == "" {
		out.Why = "choose a play to see its shortcuts"
		return out, d, false
	}
	chosen, found := a.routineWithID(id)
	if !found {
		out.Why = "that play is not in the library any more"
		return out, d, false
	}
	out.Name = chosen.Name
	rt := chosen.route()
	bs, err := d.Reg.ReadBindings()
	if err != nil {
		// NEVER "NO SHORTCUTS". A file that cannot be read is said as that, and nothing is
		// offered over it: the owner refuses to write over a file it cannot read.
		out.Why = "the shortcuts could not be read, so none are shown and none can be " +
			"set: " + err.Error()
		return out, d, false
	}
	listed := map[string]orchestrator.ListedShortcut{}
	for _, l := range d.ListShortcuts("", false) {
		listed[l.ListenIn+"\x00"+l.Key] = l
	}
	for _, b := range bs {
		k := ShortcutKey{Key: b.Key, ListenIn: b.App, Pressed: widgetconfig.Pressed(leader, b.Key),
			Where: placeWords(b.App), Version: b.Version(), Runs: runsWords(b), Cmd: b.Command(),
			State: "ok"}
		if l, ok := listed[b.App+"\x00"+b.Key]; ok && l.State != "ok" {
			k.State, k.Says = l.State, sentence(l.Says)
		}
		switch {
		case !b.Legacy() && runsRoutine(b, rt):
			if b.App == "" {
				k.Shadows = shadowsOf(bs, b, leader)
			}
			out.Keys = append(out.Keys, k)
		case b.Legacy() && wordsName(b, rt):
			_, k.Plan, k.Why = planFor(d, b, rt)
			k.Reassignable = k.Why == ""
			if k.Reassignable {
				k.PlanSays = fmt.Sprintf("Set up by name as %q. Reassigning makes it run "+
					"exactly %s, keeping those words.", b.Command(), k.Plan)
			} else {
				k.Why = sentence(k.Why)
			}
			out.Legacy = append(out.Legacy, k)
		}
	}
	out.Places = placesFor(rt)
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		// THE SAME HOLD-OFF THE OTHER PANELS TAKE: one thing at a time.
		out.Why = "Marco is recording a demonstration; one thing at a time"
		return out, d, false
	}
	out.Askable = true
	return out, d, true
}

// placesFor is where a new shortcut for rt may be listened for, with the owner's reason for any
// place it may not.
func placesFor(rt routes.Route) []ShortcutPlace {
	var out []ShortcutPlace
	if rt.App != "" {
		out = append(out, ShortcutPlace{ListenIn: rt.App, Allowed: true, Default: true,
			Label: "while " + rt.App + " is in front"})
	}
	// WHETHER EVERYWHERE IS ALLOWED IS THE OWNER'S ANSWER, not this file's.
	wide := orchestrator.WiderThanItsRoutine(rt, "")
	// AND ONE REASON, SAID ONCE: an Only-in routine refuses both other places for the same
	// reason, which is where it works.
	onlyIn := fmt.Sprintf("%q works only while %s is in front, so its shortcut can only be "+
		"listened for there.", plays.Pretty(rt.Slug), rt.App)
	every := ShortcutPlace{ListenIn: "", Label: "everywhere", Allowed: wide == ""}
	if !every.Allowed {
		every.Why = onlyIn
	}
	other := ShortcutPlace{Named: true, Allowed: true, Label: "while an app you name is in front"}
	if rt.App != "" {
		other.Label = "while another app you name is in front"
		if !rt.Focus {
			// AN ONLY-IN ROUTINE, which the owner allows to be listened for in its own
			// application and nowhere else. Any other name would be refused by it.
			other.Allowed, other.Why = false, onlyIn
		}
	}
	return append(out, every, other)
}

// planFor is what a shortcut set up by name would run if reassigned from rt's panel: the step its
// words name as rt is rt itself, and any other step is what its words reach in the application it
// is listened for in — exactly what a press there would have looked up.
func planFor(d orchestrator.Deps, b routes.Binding, rt routes.Route) ([]routes.Route, string,
	string) {

	var steps []routes.Route
	var ids []string
	for _, step := range routes.SplitChain(b.Command()) {
		base, _, _ := routes.ParseInvocation(step)
		if routes.Slug(base) == rt.Slug {
			steps, ids = append(steps, rt), append(ids, rt.ID())
			continue
		}
		other, ok := d.Reg.Resolve(b.App, base)
		if !ok {
			return nil, "", fmt.Sprintf("its step %q reaches no play here, so it can "+
				"only be removed or set again", step)
		}
		steps, ids = append(steps, other), append(ids, other.ID())
	}
	return steps, strings.Join(ids, " then "), ""
}

// stepsOf is the routines a binding runs, with rt standing for itself.
func stepsOf(b routes.Binding, rt routes.Route) []routes.Route {
	var out []routes.Route
	for _, id := range b.Routes {
		if id == rt.ID() {
			out = append(out, rt)
			continue
		}
		if other, err := routes.ParseID(id); err == nil {
			out = append(out, other)
		}
	}
	if len(out) == 0 {
		out = []routes.Route{rt}
	}
	return out
}

func runsRoutine(b routes.Binding, rt routes.Route) bool {
	for _, id := range b.Routes {
		if id == rt.ID() {
			return true
		}
	}
	return false
}

// wordsName is whether a shortcut set up by name names rt in any of its steps.
func wordsName(b routes.Binding, rt routes.Route) bool {
	for _, step := range routes.SplitChain(b.Command()) {
		base, _, _ := routes.ParseInvocation(step)
		if routes.Slug(base) == rt.Slug {
			return true
		}
	}
	return false
}

// shadowsOf is where else the same key runs something else, for a shortcut listened for
// everywhere: an application's own shortcut on that key wins while it is in front.
func shadowsOf(bs []routes.Binding, global routes.Binding, leader string) string {
	var apps []string
	for _, b := range bs {
		if b.App != "" && b.Key == global.Key {
			apps = append(apps, fmt.Sprintf("in %s, %s runs %s instead", b.App,
				widgetconfig.Pressed(leader, b.Key), runsWords(b)))
		}
	}
	if len(apps) == 0 {
		return ""
	}
	sort.Strings(apps)
	s := strings.Join(apps, "; ")
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

// runsWords is what a binding runs, in words.
func runsWords(b routes.Binding) string {
	if b.Legacy() {
		return fmt.Sprintf("%q (set up by name)", b.Command())
	}
	names := make([]string, 0, len(b.Routes))
	for _, id := range b.Routes {
		if rt, err := routes.ParseID(id); err == nil {
			names = append(names, fmt.Sprintf("%q", plays.Pretty(rt.Slug)))
			continue
		}
		names = append(names, id)
	}
	return strings.Join(names, " then ")
}

func placeWords(listenIn string) string {
	if listenIn == "" {
		return "everywhere"
	}
	return "in " + listenIn
}

func leaderSays(leader string) string {
	if strings.TrimSpace(leader) == "" {
		return "The widget has no leader key configured, so no shortcut can be pressed."
	}
	return fmt.Sprintf("Press the widget's leader (%s), then the key.", widgetconfig.Named(leader))
}

// sentence makes an owner's message read as a sentence: first letter up, one full stop.
func sentence(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "not bound: "))
	if s == "" {
		return ""
	}
	s = strings.ToUpper(s[:1]) + s[1:]
	if !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

// shortcutLabels is, for the library list, the keys that run each routine and the ones set up by
// name that would, as chips: "`e Notepad", "`c everywhere".
func shortcutLabels(bs []routes.Binding, leader string) map[string][]string {
	out := map[string][]string{}
	for _, b := range bs {
		if b.Legacy() {
			continue // shown on the routine's own Shortcut tab, to be reassigned
		}
		label := widgetconfig.Pressed(leader, b.Key) + " " +
			strings.TrimPrefix(placeWords(b.App), "in ")
		for _, id := range b.Routes {
			out[id] = append(out[id], label)
		}
	}
	return out
}
