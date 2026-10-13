package observe

// FINDING A DEMONSTRATED TARGET AGAIN, FROM THE PIXELS AND NOT FROM THE CLICK.
//
// # The question this answers
//
// [[ADR-172-a-click-transition-is-a-source-a-target-and-a-destination]] made the learning unit:
// source screen, visual target, destination. [[ADR-179-the-first-live-observation]] showed the
// first half working live — screens recognised on return, presses correlated with the changes
// that followed. The owner named what comes next:
//
//	Given a previously observed screen, can Marco find the same clickable target again without
//	relying on the original click coordinates?
//
// Without the coordinates is the whole point. A stored pointer position re-presses a place on the
// screen; it does not find a control. The first time the menu scrolls, the window resizes or the
// row order changes, a coordinate is a confident wrong answer — and the owner's standard for that
// is unambiguous:
//
//	A system that confidently identifies the wrong button is worse than one that says it cannot
//	find the button.
//
// So reacquisition is a match between what the target has been SEEN AS and what the proposer
// found on the look in front of us, and it has three outcomes rather than two.
//
// # Three outcomes, and only one of them is a candidate for anything
//
//	matched       one control, agreed on by every supporting proposal
//	ambiguous     more than one control could be it
//	unobservable  nothing the proposer found supports it
//
// `matched` is not permission. It is a geometric claim about one look, made by a layer that
// cannot act, and the actuation boundary is unchanged: nothing here produces a press, and nothing
// here may become durable.
//
// # Why there is no new threshold
//
// Everything below is decided by `sameRect` — mutual centre containment — which is the same
// relation `identify` uses to decide that two readings are one control. Reusing it is deliberate:
// this subsystem has a measured history of invented ratios being wrong, and a reacquisition rule
// with its own tolerance would be a second opinion about what "the same control" means, free to
// drift from the first.

import (
	"fmt"
	"sort"
)

// ReacquireResult is what came of looking for a demonstrated target again.
type ReacquireResult string

const (
	// ReacquireMatched is one control supported by the current proposals.
	//
	// "One control" rather than "one proposal": a proposer that read the same row twice,
	// as a row and as a span of it, offers two rectangles that agree with each other, and
	// calling that ambiguous would report a disagreement nobody has.
	ReacquireMatched ReacquireResult = "matched"
	// ReacquireAmbiguous is more than one control that could be the demonstrated one.
	//
	// Reported, never resolved. The smallest is not the answer here for the same reason it
	// is not the answer in identify: it is a hypothesis, and this is the layer that was
	// built to stop a hypothesis becoming a verified identity on its own.
	ReacquireAmbiguous ReacquireResult = "ambiguous"
	// ReacquireUnobservable is nothing in the current proposals that supports the target.
	//
	// The honest answer when a control has moved, gone, or was never visible on this look.
	// Also the answer when the ONLY support is a region that merely contained the original
	// press: a container is not the control, and pressing its centre is a different act.
	ReacquireUnobservable ReacquireResult = "unobservable"
)

// TargetDestination is where a target's clicks were observed to lead.
type TargetDestination struct {
	To    VisualScreenID  `json:"to"`
	Cause TransitionCause `json:"cause"`
	Count int             `json:"count"`
}

// TargetRecord is everything one session knows about one demonstrated target.
//
// The registry entry the owner specified, assembled rather than stored twice: the readings come
// from the identity machinery, the destinations are derived from the recorded steps, and the
// uncertainty is derived from the attributions. Nothing here is a second copy of evidence that
// could drift from the first.
type TargetRecord struct {
	ID     VisualTargetID `json:"id"`
	Screen VisualScreenID `json:"screen"`
	// Where is every reading of the CONTROL — the smallest region containing each press
	// that was identified as this target. What a later look has to find again.
	Where []Region `json:"where"`
	// Containers is every LARGER region that also contained one of those presses.
	//
	// Kept because the control/container ambiguity is the thing that makes a click
	// target hard, and discarded evidence cannot later contradict a match. NOT used to
	// match: a container that is still on screen when the control is gone would otherwise
	// reacquire the panel as the button.
	Containers []Region `json:"containers,omitempty"`
	// Presses is how often this target was clicked, and AmbiguousPresses how many of
	// those landed inside more than one region.
	//
	// The repeated-evidence count. One press is a demonstration; two on separate
	// occasions is the beginning of a claim that survives a second look.
	Presses          int `json:"presses"`
	AmbiguousPresses int `json:"ambiguous_presses,omitempty"`
	// FirstLook and LastLook are when it was clicked, so a reacquisition can be scored
	// against looks that are not the demonstration itself.
	FirstLook int `json:"first_look"`
	LastLook  int `json:"last_look"`
	// Destinations is where clicking it was observed to lead, with the cause.
	Destinations []TargetDestination `json:"destinations,omitempty"`
	// MaybeOneControlWith is the other identities on this screen that led to the same
	// destination, so a match cannot claim to know which of two the session could not
	// tell apart. See ClickEvidence.MaybeOneControl.
	MaybeOneControlWith []VisualTargetID `json:"maybe_one_control_with,omitempty"`
}

// Repeated reports whether this target was clicked more than once.
//
// The distinction the owner drew: a click's own observation is not a reacquisition, and a target
// clicked once has no independent evidence that it can be found again at all.
func (t TargetRecord) Repeated() bool { return t.Presses > 1 }

// Reacquisition is one attempt to find one target on one look.
type Reacquisition struct {
	Target VisualTargetID  `json:"target"`
	Screen VisualScreenID  `json:"screen"`
	Result ReacquireResult `json:"result"`
	// Where is the matched control's smallest current reading, and only set when matched.
	// A region on an ambiguous or unobservable result would be a quiet decision.
	Where Region `json:"where,omitempty"`
	// Supporting is every current proposal that agreed with a stored reading, and Controls
	// how many distinct controls those proposals describe.
	Supporting []Region `json:"supporting,omitempty"`
	Controls   int      `json:"controls"`
	// Indistinct is proposals that agreed with the control AND with a region that merely
	// contained the original press, so they could not tell the two apart.
	//
	// A different refusal from finding nothing, carried as a number rather than only in
	// words: "the control is gone" sends a reader to the screen and "the control cannot
	// be told from its panel" sends them here.
	Indistinct int `json:"indistinct,omitempty"`
	// Uncertain says the session itself could not distinguish this identity from another
	// on the same screen. A match is then a match to a pair, not to a button.
	Uncertain bool `json:"uncertain,omitempty"`
	// Because says why, in words, so a reader does not re-derive it from the counts.
	Because string `json:"because"`
}

// Actionable reports whether this outcome could, with authorisation, ground a press.
//
// Matched and certain. Nothing else — and this says COULD: the authorisation is somebody else's
// to give, the legal Marco boundary is unchanged, and no caller of this may inject anything.
func (r Reacquisition) Actionable() bool {
	return r.Result == ReacquireMatched && !r.Uncertain
}

// Registry is every demonstrated target, with its provenance, alternatives and destinations.
//
// Derived on demand from the same state the evidence is derived from. A registry kept as its own
// structure would be a second account of the session, and this subsystem's recorded failure mode
// is two statements of one fact drifting apart.
func (c *ClickTargets) Registry() []TargetRecord {
	pairs := c.Evidence().MaybeOneControl
	out := make([]TargetRecord, 0, len(c.targets))
	for _, t := range c.targets {
		rec := TargetRecord{
			ID: t.id, Screen: t.screen,
			Where:            append([]Region{}, t.where...),
			Containers:       append([]Region{}, t.containers...),
			Presses:          t.seen,
			AmbiguousPresses: t.ambiguous,
			FirstLook:        t.firstLook,
			LastLook:         t.lastLook,
		}
		for _, st := range c.steps {
			if st.Target != t.id {
				continue
			}
			rec.Destinations = append(rec.Destinations, TargetDestination{
				To: st.To, Cause: st.Cause, Count: st.Count,
			})
		}
		for _, p := range pairs {
			switch t.id {
			case p.A:
				rec.MaybeOneControlWith = append(rec.MaybeOneControlWith, p.B)
			case p.B:
				rec.MaybeOneControlWith = append(rec.MaybeOneControlWith, p.A)
			}
		}
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Reacquire looks for every target demonstrated on one screen among that look's proposals.
//
// `screen` is the screen standing NOW and `regions` what the proposer found NOW. The original
// click position is not consulted and is not reachable from here: the inputs are a screen
// identity and a set of rectangles.
//
// Returns one outcome per demonstrated target on that screen, in identity order, including the
// ones it could not find — because a registry of five targets that reports two matches and says
// nothing about the other three is the shape of a measurement that flatters itself.
func (c *ClickTargets) Reacquire(screen VisualScreenID, regions []Region) []Reacquisition {
	if screen == "" || screen == VisualScreenUnknown || screen == VisualScreenUnobserved {
		// NO SCREEN IS NOT A SCREEN. Matching a target against an arrangement nobody
		// could identify would attribute a control to a look that failed, and the
		// unobserved look is exactly where a confident wrong answer is cheapest to
		// produce.
		return nil
	}
	var out []Reacquisition
	for _, t := range c.targets {
		if t.screen != screen {
			continue
		}
		out = append(out, c.reacquireOne(t, regions))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

// StoredTarget is everything finding a control again actually needs.
//
// # Why this type exists
//
// Reacquisition used to be reachable only through a session's own registry, which meant a
// control could be found again only in the session that learned it. A workflow read back from a
// file has the same evidence — the readings, the containers that are not the control, and
// whether the session could tell this identity from another — and no registry at all.
//
// So the matcher takes evidence rather than a registry, and the registry became one of its two
// callers. There is one matcher: a target re-acquired from a file and a target re-acquired from
// the session that learned it are judged by the same rules, which is the only way the offline
// measurement means anything about the live path.
//
// The ID and Screen are carried through to the outcome and are not consulted. They are labels
// for a reader — for a reloaded workflow they are the artifact's own, which name nothing in this
// session, and the matching is pure geometry either way.
type StoredTarget struct {
	ID     VisualTargetID
	Screen VisualScreenID
	// Readings is every reading of the CONTROL, Containers every larger region that also
	// contained one of its presses.
	Readings   []Region
	Containers []Region
	// Uncertain says the session that learned this could not tell it from another
	// identity on the same screen. Carried because a match against an uncertain identity
	// matches a PAIR, and a reader has to be told that.
	Uncertain bool
}

// ReacquireStored matches one stored control against one look's proposals.
//
// `regions` is what the proposer found NOW. The original click position is not consulted and is
// not reachable from here: the inputs are rectangles and rectangles.
func ReacquireStored(t StoredTarget, regions []Region) Reacquisition {
	r := Reacquisition{Target: t.ID, Screen: t.Screen, Uncertain: t.Uncertain}
	// EVERY PROPOSAL THAT AGREES WITH A READING OF THIS CONTROL AND IS NOT A KNOWN
	// CONTAINER OF IT.
	//
	// `sameControl` is the relation identify already uses, so "found again" and "the same
	// control" are one statement rather than two. The container exclusion is not a second
	// opinion either, and it is not a ratio: it tests the rectangles this target's own
	// presses were recorded as being inside.
	//
	// It is needed because mutual centre containment is not a size test. A thin row that
	// straddles its panel's centre contains the panel's centre, and the panel contains
	// the row's — so the panel is "the same rectangle" as the row, and without this a
	// reacquisition would match the menu and offer its middle as the button. Found by a
	// test whose row happened to sit at the panel's vertical centre; the row at a third
	// of the way down had hidden it.
	//
	// Any container that could do this was necessarily recorded: the press was inside the
	// control, the control is inside the container, so the container was one of the
	// press's own candidates.
	indistinct := 0
	for _, p := range regions {
		if !sameControl(t.Readings, []Region{p}) {
			continue
		}
		if sameControl(t.Containers, []Region{p}) {
			// INDISTINGUISHABLE FROM A REGION THAT CONTAINED THE PRESS, so it
			// cannot be support. Counted, because it is a different refusal from
			// finding nothing and it sends a reader somewhere else entirely.
			indistinct++
			r.Indistinct++
			continue
		}
		r.Supporting = append(r.Supporting, p)
	}
	if len(r.Supporting) == 0 {
		r.Result = ReacquireUnobservable
		switch {
		case indistinct > 0:
			r.Because = fmt.Sprintf("%d proposal(s) agree with this control and are "+
				"also the same rectangle as a region that merely CONTAINED the "+
				"original press, so none of them distinguishes the control from "+
				"its container. Mutual centre containment is not a size test: a "+
				"row that straddles its panel's centre agrees with the panel",
				indistinct)
		default:
			r.Because = "no proposal on this look agrees with any reading of this " +
				"control"
			// AND WHETHER SOMETHING THAT MERELY CONTAINED IT IS STILL THERE: a
			// reader who sees `unobservable` while the panel is plainly on screen
			// should be told so rather than left to suspect a capture fault.
			for _, p := range regions {
				if sameControl(t.Containers, []Region{p}) {
					r.Because += ", though a region that contained the " +
						"original press is still here: a container is not " +
						"the control and its centre is not the same press"
					break
				}
			}
		}
		return r
	}
	// HOW MANY CONTROLS THOSE PROPOSALS DESCRIBE, which is not how many proposals there
	// are. A row and a span of that row agree with each other and are one control read
	// twice; two different rows do not agree and are two.
	groups := controlGroups(r.Supporting)
	r.Controls = len(groups)
	switch {
	case len(groups) == 1:
		r.Result = ReacquireMatched
		r.Where = smallest(groups[0])
		r.Because = fmt.Sprintf("%d proposal(s) agreeing with each other and with a "+
			"reading of this control", len(r.Supporting))
		if r.Uncertain {
			r.Because += "; the session could not tell this identity from another " +
				"on the same screen, so this matches a pair"
		}
	default:
		r.Result = ReacquireAmbiguous
		r.Because = fmt.Sprintf("%d proposal(s) support this control but describe %d "+
			"different ones; nothing here chooses between them",
			len(r.Supporting), len(groups))
	}
	return r
}

// reacquireOne is one target against one look's proposals, from the registry.
//
// The registry's own entry point into the matcher above. It is the only thing that knows about
// `MaybeOneControl`, which is session evidence: whether two identities led to the same place is
// a fact about what was observed, not about the rectangles.
func (c *ClickTargets) reacquireOne(t visualTarget, regions []Region) Reacquisition {
	st := StoredTarget{
		ID: t.id, Screen: t.screen,
		Readings: t.where, Containers: t.containers,
	}
	for _, p := range c.Evidence().MaybeOneControl {
		if p.A == t.id || p.B == t.id {
			st.Uncertain = true
			break
		}
	}
	return ReacquireStored(st, regions)
}

// controlGroups partitions regions into sets that are readings of one control.
//
// Transitively, by `sameRect`: a region joins a group if it is the same rectangle as anything
// already in it. Transitive rather than all-pairs because "the same control" is what the relation
// is for, and demanding that every member agree with every other would split a control the
// proposer read three times at slightly growing sizes into two.
//
// That transitivity is also the honest weakness, and it is the one worth stating: a dense ladder
// of overlapping proposals could chain two genuinely different controls into one group. The
// consequence is a MATCH where there should have been an ambiguity, which is the direction that
// costs something — so it is measured on the recording rather than argued about here.
func controlGroups(regions []Region) [][]Region {
	var groups [][]Region
	for _, r := range regions {
		placed := false
		for i := range groups {
			if sameControl(groups[i], []Region{r}) {
				groups[i] = append(groups[i], r)
				placed = true
				break
			}
		}
		if !placed {
			groups = append(groups, []Region{r})
		}
	}
	return groups
}

// smallest is the least-area region of a set, ties broken by position.
//
// The same ordering Press applies to a press's candidates, for the same reason: two runs of one
// recording must name the same rectangle.
func smallest(regions []Region) Region {
	best := regions[0]
	for _, r := range regions[1:] {
		a, b := r.Width*r.Height, best.Width*best.Height
		switch {
		case a != b:
			if a < b {
				best = r
			}
		case r.Y != best.Y:
			if r.Y < best.Y {
				best = r
			}
		default:
			if r.X < best.X {
				best = r
			}
		}
	}
	return best
}
