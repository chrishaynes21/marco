package observe

// Whether a region matched again is the control it was matched to.
//
// # What re-acquisition establishes, and what it does not
//
// `sameControl` is mutual centre containment: two rectangles are two readings of one control
// when each contains the other's centre. That is symmetric, threshold-free, and robust to the
// thing that actually varies — a proposer that found the same row slightly larger on a later
// frame. It is also the whole of the test. It consults no pixels and no text.
//
// [[Experiment-044-two-live-runs-that-pressed-nothing-and-pressed-grass]] measured what that
// costs. Rocket League's main menu is bottom-anchored, and signed out it loses two entries, so
// PLAY sat 95px below the rectangle the demonstration recorded. A proposer boxed a 128x24 patch
// of pitch inside that old rectangle, re-acquisition reported MATCHED, and the step pressed
// grass — while the word PLAY was read, on the same look, on a different candidate.
//
// So a geometric match establishes **overlap**, not identity. It may nominate a control. It is
// not by itself an answer to "is this the control", when something better is on the look.
//
// # The asymmetry, which is the one from ADR-194 a level down
//
// Text may REFUSE a match. It may never propose one. Nothing here mints a target, moves a press,
// or reaches the artifact — the strongest thing this file does is say "not that one", after
// which the step is refused and the repair that CAN propose is asked.
//
// And the contradiction is a competing candidate, not a silence:
//
//   - a region with no words in it is an icon, or a label drawn outside the clickable rectangle,
//     or an OCR that failed. Missing evidence is not contradictory evidence.
//   - a region with no words in it, while a DIFFERENT region on the same look positively says
//     what this control is supposed to say, is a contradiction.
//
// That distinction is the whole safety argument. Without it, every unlabelled control in every
// application becomes unpressable the moment OCR has a bad frame.

import (
	"sort"
	"strings"
)

// ReadWord is one word a source read, with where it read it, normalised to the window.
type ReadWord struct {
	Text  string
	Where Region
}

// Named is a candidate region and whatever was read inside it.
type Named struct {
	Region Region
	Text   string
}

// IdentityResult is what the words on a look say about a geometric match.
type IdentityResult string

const (
	// IdentityUnexamined: there was nothing to ask with, or nothing to ask about.
	// NOT a weak pass — it is the absence of the question, and the report says so.
	IdentityUnexamined IdentityResult = "not examined"
	// IdentityCorroborated: the matched region says what this control should say.
	IdentityCorroborated IdentityResult = "corroborated by what it says"
	// IdentityContradicted: it does not, and something else on this look does.
	IdentityContradicted IdentityResult = "contradicted by a better-named candidate"
)

// ControlIdentity is what a look's words make of one geometric match.
type ControlIdentity struct {
	Result IdentityResult
	// Said is what was read inside the matched region, empty when nothing was.
	Said string
	// Elsewhere is every OTHER candidate on this look that is named as this control
	// should be. More than one is reported and NOT chosen between.
	Elsewhere []Named
	Because   string
}

// Contradicted reports whether this match may not be pressed.
func (i ControlIdentity) Contradicted() bool { return i.Result == IdentityContradicted }

// ControlIdentityOf examines one geometric match against the words read on the same look.
//
// `where` is the region re-acquisition matched, `candidates` every region the sources proposed,
// `read` the words, and `wanted` what this control is supposed to say — the control's own saved
// text where a demonstration recorded it, and the workflow's name only as a weaker fallback.
// Where that came from is the caller's to report; this function is told the words and not the
// provenance, because the rule is the same either way and a function that took both would invite
// one of them to change the answer.
func ControlIdentityOf(where Region, candidates []Region, read []ReadWord, wanted []string,
	demonstrated bool) ControlIdentity {
	switch {
	case len(wanted) == 0:
		return ControlIdentity{Result: IdentityUnexamined,
			Because: "nothing recorded says what this control should say, so its " +
				"identity cannot be checked against what is on the screen"}
	case len(read) == 0:
		return ControlIdentity{Result: IdentityUnexamined,
			Because: "no words were read on this look, and missing evidence is " +
				"not contradictory evidence"}
	}

	named := NameCandidates(candidates, read)

	// WHAT THE MATCHED REGION ITSELF SAYS, asked first. A match that is positively
	// corroborated is finished with; nothing else on the look can take it away, which
	// is what keeps two rows of one menu from arguing about each other.
	said := ""
	for _, n := range named {
		if sameRect(n.Region, where) && n.Text != "" {
			said = n.Text
			break
		}
	}
	if said != "" && NamesControl(Words(said), wanted, demonstrated) {
		return ControlIdentity{Result: IdentityCorroborated, Said: said,
			Because: "the region matched on this look reads " + quoted(said) +
				", which is what this control is recorded as"}
	}

	// AND WHETHER SOMETHING ELSE POSITIVELY IS IT.
	var elsewhere []Named
	for _, n := range named {
		if n.Text == "" || sameRect(n.Region, where) {
			continue
		}
		if NamesControl(Words(n.Text), wanted, demonstrated) {
			elsewhere = append(elsewhere, n)
		}
	}
	if len(elsewhere) == 0 {
		return ControlIdentity{Result: IdentityUnexamined, Said: said,
			Because: "nothing on this look is named as this control is, here or " +
				"anywhere else, so there is no competing candidate and " +
				"nothing contradicts the match"}
	}

	what := "nothing was read inside it"
	if said != "" {
		what = "it reads " + quoted(said)
	}
	at := quoted(elsewhere[0].Text)
	if len(elsewhere) > 1 {
		at = howMany(len(elsewhere)) + " other candidates, nothing chooses between them"
	}
	return ControlIdentity{Result: IdentityContradicted, Said: said, Elsewhere: elsewhere,
		Because: "the region matched on this look is not this control: " + what +
			", and " + at + " on the same look is what this control is " +
			"recorded as"}
}

// NameCandidates pairs each candidate region with the words read inside it.
//
// A word belongs to a candidate when its box lies wholly within that candidate's, and the
// SMALLEST containing candidate takes it — so a tile and the panel behind it do not both claim
// the same label, and a panel does not inherit every word on the screen.
//
// A band across the window is not a candidate, whatever is written on it. It locates nothing, so
// a word inside it says nothing about where anything is.
func NameCandidates(candidates []Region, read []ReadWord) []Named {
	out := make([]Named, 0, len(candidates))
	for _, r := range candidates {
		if ok, _ := r.Locates(); !ok {
			continue
		}
		out = append(out, Named{Region: r})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Region.Area() < out[j].Region.Area()
	})
	for _, w := range read {
		if w.Text == "" {
			continue
		}
		for i := range out {
			if within(w.Where, out[i].Region) {
				out[i].Text = strings.TrimSpace(out[i].Text + " " + w.Text)
				break
			}
		}
	}
	return out
}

// within reports whether all of `a` lies inside `b`.
func within(a, b Region) bool {
	return a.X >= b.X && a.Y >= b.Y &&
		a.X+a.Width <= b.X+b.Width && a.Y+a.Height <= b.Y+b.Height
}

// Words splits a phrase into lowercase words, dropping anything that is not a letter or digit.
//
// A single character is not a word. Two readings of one Rocket League menu disagreed about forty
// tokens of the shape `ae an bs ee fe om on oo pe ro si we ww`, and a rule that counted them made
// every screen agree with every other.
func Words(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(f) > 1 {
			out = append(out, f)
		}
	}
	return out
}

// Covered reports whether every word of `some` appears in `all`.
//
// Deliberately one-directional and whole-word. "FREE PLAY" is covered by "enter free play";
// "CUSTOM TRAINING" is not. A substring rule would make "PLAY" match "PLAYLIST", and a
// bidirectional one would make any short label match any long name.
// NamesControl reports whether a candidate reading `said` is named as a control recorded as
// saying `wanted`, and the direction depends on where `wanted` came from.
//
// # Why one rule cannot serve both sources
//
// The fallback is the workflow's own NAME -- "enter free play" -- and what it can say is that a
// candidate's label is made of the task's words. It runs `said ⊆ wanted`, because a title is a
// handful of words and a control that says something outside them is a different control. That
// is [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] and it is unchanged.
//
// A DEMONSTRATED label is the opposite shape. It is what somebody actually clicked, and the
// question is whether this candidate says it -- `wanted ⊆ said`. The direction matters on the
// measured case: the control was learned as `training`, and a later look reads `CUSTOM TRAINING`
// on the tile. Under the title rule that is a miss, because `custom` is not in the saved label;
// under this one it is the control, because everything the control is recorded as saying is
// there.
//
// Both directions keep the same safety property, which is that nothing here PROPOSES on its own:
// a caller still refuses when more than one candidate matches, and `NameCandidates` has already
// given each word to exactly one candidate.
func NamesControl(said, wanted []string, demonstrated bool) bool {
	if demonstrated {
		return Covered(wanted, said)
	}
	return Covered(said, wanted)
}

func Covered(some, all []string) bool {
	if len(some) == 0 {
		return false
	}
	has := make(map[string]bool, len(all))
	for _, a := range all {
		has[a] = true
	}
	for _, s := range some {
		if !has[s] {
			return false
		}
	}
	return true
}

func quoted(s string) string { return `"` + s + `"` }

// howMany renders a small count without pulling fmt into a file about words.
func howMany(n int) string {
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	if len(d) == 0 {
		return "0"
	}
	return string(d)
}
