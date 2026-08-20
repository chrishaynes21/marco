package observe

import (
	"fmt"
	"math"
	"sort"
)

// Elements that come and go together.
//
// # Why this is justified NOW and was not before
//
// Co-occurrence grouping was deferred twice, correctly: without screen states there was nothing
// to co-occur WITHIN, so "these five tracks appear together" was indistinguishable from "these
// five tracks are all common". A group would have been a restatement of presence.
//
// State-local persistence gives it a denominator. Five controls that are each persistent within
// state_2 and absent everywhere else are not five coincidences — they are one structure that
// the session saw arrive, leave and return as a unit. That is a claim with content, and it is
// the first thing in this subsystem that looks like an interface element rather than a box.
//
// # What it is still not
//
// Not a menu. Not a dialog, a panel or a list. A group is the observation that some tracks share
// a state and an arrival pattern; naming the thing they form is interpretation, and interpreting
// it here would put a guess underneath a measurement — the mistake the numbered state IDs exist
// to prevent.

// MaxStructuralGroups and MaxGroupMembers bound the derived structure.
const (
	MaxStructuralGroups = 16
	MaxGroupMembers     = 24
	// GroupMemberPresence is the state-local presence a track needs to count as part of its
	// state's structure. Below it the track comes and goes WITHIN the state, which is the
	// opposite of the property a group is claiming.
	GroupMemberPresence = 0.80
	// GroupGapBreak is the multiple of the median vertical gap at which a run of members
	// stops being one arrangement.
	GroupGapBreak = 3.0
)

// StructuralGroup is a set of tracks that behave as one structure inside one screen state.
type StructuralGroup struct {
	ID    string        `json:"id"`
	State ScreenStateID `json:"state"`
	// Members are track IDs, ordered top-to-bottom by reference geometry.
	Members []string       `json:"members"`
	Roles   map[string]int `json:"roles,omitempty"`
	// Episodes is how many times the group's state was entered.
	Episodes int `json:"episodes"`
	// Envelope is the bounding box of the members' reference geometry — window-relative and
	// normalised, like everything else here. Never absolute desktop coordinates: a panel
	// that moves with its window must stay the same structure.
	Envelope Region `json:"envelope"`
	// MeanSpacing is the average vertical gap between consecutive members, and Uniformity
	// how regular that spacing is (1.0 for a perfectly even stack). Together they describe
	// the arrangement RELATIVELY, so a panel that shifts slightly is still itself.
	MeanSpacing float64 `json:"mean_spacing"`
	Uniformity  float64 `json:"uniformity"`
	Nameable    int     `json:"nameable"`
}

// Groups derives the co-occurring structures from tracks and states.
//
// Derived on demand from evidence already retained, so the group model costs no memory of its
// own — which is what keeps a five-minute session flat.
func Groups(tracks []ShadowTrack, states []ScreenState) []StructuralGroup {
	byState := map[ScreenStateID][]ShadowTrack{}
	for _, t := range tracks {
		// Persistent in exactly ONE state is the membership rule, and the "exactly" is
		// load-bearing. A HUD element that is equally reliable on the menu screen and
		// during play is ambient: it does not arrive or leave with either structure, and
		// folding it into both groups would describe a panel that includes half the screen.
		// What makes a group a group is that its members come and go TOGETHER.
		var in []ScreenStateID
		for _, s := range t.States {
			if s.Seen >= 2 && s.PresenceRatio() >= GroupMemberPresence {
				in = append(in, s.State)
			}
		}
		if len(in) == 1 {
			byState[in[0]] = append(byState[in[0]], t)
		}
	}

	episodes := map[ScreenStateID]int{}
	order := make([]ScreenStateID, 0, len(states))
	for _, s := range states {
		episodes[s.ID] = s.Episodes
		order = append(order, s.ID)
	}
	// Any state carrying members but missing from the table still gets a stable slot.
	for id := range byState {
		if _, ok := episodes[id]; !ok {
			order = append(order, id)
		}
	}
	sort.Slice(order, func(a, b int) bool { return order[a] < order[b] })

	var out []StructuralGroup
	for _, id := range order {
		members := byState[id]
		// One member is not a structure; it is a track that already reports itself.
		if len(members) < 2 || len(out) >= MaxStructuralGroups {
			continue
		}
		sort.SliceStable(members, func(a, b int) bool {
			if members[a].Reference.Y != members[b].Reference.Y {
				return members[a].Reference.Y < members[b].Reference.Y
			}
			return members[a].ID < members[b].ID
		})
		if len(members) > MaxGroupMembers {
			members = members[:MaxGroupMembers]
		}

		// Sharing a state is necessary but not sufficient. Everything detected only while
		// the menu is up shares state_3 — the four rows and a pair of corner boxes 0.3
		// normalised units below them — and calling that one structure describes a panel
		// covering half the screen. Arrangement is the second condition, so the run is cut
		// where the vertical gap stops being of a piece with the others.
		for _, run := range splitByArrangement(members) {
			if len(run) < 2 || len(out) >= MaxStructuralGroups {
				continue
			}
			out = append(out, describeGroup(len(out)+1, id, episodes[id], run))
		}
	}
	return out
}

// splitByArrangement cuts a vertically sorted run where the spacing changes character.
//
// A gap several times the typical one is not a wider row of the same list; it is the end of the
// list. The multiple is compared against the MEDIAN gap rather than the mean so that the one
// large gap being looked for cannot drag the threshold up to hide itself.
func splitByArrangement(members []ShadowTrack) [][]ShadowTrack {
	if len(members) < 3 {
		return [][]ShadowTrack{members}
	}
	gaps := make([]float64, 0, len(members)-1)
	for i := 1; i < len(members); i++ {
		gaps = append(gaps, members[i].Reference.Y-members[i-1].Reference.Y)
	}
	sorted := append([]float64(nil), gaps...)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	if median <= 0 {
		return [][]ShadowTrack{members}
	}

	var out [][]ShadowTrack
	run := []ShadowTrack{members[0]}
	for i, g := range gaps {
		if g > GroupGapBreak*median {
			out = append(out, run)
			run = nil
		}
		run = append(run, members[i+1])
	}
	return append(out, run)
}

func describeGroup(n int, id ScreenStateID, episodes int, members []ShadowTrack) StructuralGroup {
	{
		g := StructuralGroup{
			ID:       fmt.Sprintf("group_%d", n),
			State:    id,
			Roles:    map[string]int{},
			Episodes: episodes,
		}
		x0, y0 := math.Inf(1), math.Inf(1)
		x1, y1 := math.Inf(-1), math.Inf(-1)
		var gaps []float64
		for i, m := range members {
			g.Members = append(g.Members, m.ID)
			g.Roles[m.Role]++
			if m.Nameable {
				g.Nameable++
			}
			r := m.Reference
			x0, y0 = math.Min(x0, r.X), math.Min(y0, r.Y)
			x1, y1 = math.Max(x1, r.X+r.Width), math.Max(y1, r.Y+r.Height)
			if i > 0 {
				gaps = append(gaps, r.Y-members[i-1].Reference.Y)
			}
		}
		g.Envelope = Region{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
		g.MeanSpacing, g.Uniformity = spacing(gaps)
		return g
	}
}

// spacing summarises a run of gaps as a mean and a regularity in [0,1].
func spacing(gaps []float64) (float64, float64) {
	if len(gaps) == 0 {
		return 0, 0
	}
	var sum float64
	for _, g := range gaps {
		sum += g
	}
	mean := sum / float64(len(gaps))
	if mean <= 0 {
		return mean, 0
	}
	var dev float64
	for _, g := range gaps {
		dev += math.Abs(g - mean)
	}
	// Mean absolute deviation as a fraction of the spacing, inverted: an evenly stacked
	// column scores 1.0 and a scatter scores near 0.
	return mean, math.Max(0, 1-(dev/float64(len(gaps)))/mean)
}
