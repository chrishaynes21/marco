package localizetrial

// THE PROTOCOL, DECLARED BEFORE ANYTHING WAS EVALUATED.
//
// Two rules make the rest of this worth reading:
//
//   - The parameters below were chosen from the SETUP examples and frozen. If one of them is
//     changed after an evaluation run, that run was tuning and the evaluation starts again with
//     fresh cases. `TestTheFrozenRuleIsTheOneTheSetupSupports` is what keeps that honest: it
//     re-derives each number from the setup examples and fails if the frozen value is not the
//     one those numbers justify.
//   - Setup and evaluation use DISJOINT SEED RANGES, listed here. No case the parameters were
//     chosen against is scored.
//
// The seeds are the reproduction: another person runs the same test and gets the same hundred
// images, the same scores, and the same table.

import "image"

// Seed ranges. Disjoint by construction, and `TestTheSeedRangesDoNotOverlap` says so.
const (
	// demoSeed renders the one look a target is demonstrated from.
	demoSeed = 500
	// setupFrom is where the examples the parameters were chosen against begin.
	setupFrom, setupCount = 1000, 60
	// positiveFrom is the supported cases: the control is there and the window is
	// the one it was demonstrated in.
	positiveFrom = 2000
	// negativeFrom is the cases a correct method refuses.
	negativeFrom = 3000
	// movedFrom is the window-movement cases, which are supported and are counted
	// separately because they answer a different question.
	movedFrom = 4000
)

// Frozen is every parameter of the method.
//
// # Region — 176x52
//
// The button is 160x36 and this is it plus eight pixels of its own surroundings. That is the
// smallest region that holds the control and its edge, and it deliberately stops short of the
// panel heading twenty-two pixels further up — the one thing on screen that tells the two panels
// apart. A region grown until it swallowed that heading would make appearance sufficient on its
// own, and the experiment would be measuring its own fixture rather than the question.
//
// # Tolerance — 6px
//
// The setup examples move a control by at most two pixels (a focus ring, a scrollbar, the
// ordinary wobble of a layout that is fixed but not frozen). Six is three times the largest
// variation observed, which is a bound meant not to be tight. It was NOT widened afterwards:
// the reflow case displaces by forty, which is unsupported by design.
//
// # Accept — 0.90, Margin — 0.03, Apart — 12
//
// Measured on the setup examples, where the correct match scores at worst about 0.98 and the
// best thing at a wrong place scores well under 0.9 — except the decoy, which scores as well as
// the target does, because it is the same pixels. So 0.90 separates "this is the control" from
// "this is some other part of the window", and NOTHING separates the target from its decoy by
// score — which is the experiment's whole point, and why the margin exists to call that case
// ambiguous instead of guessing at it.
var Frozen = Rule{
	Region:    image.Pt(176, 52),
	Tolerance: 6,
	Accept:    0.90,
	Margin:    0.03,
	Apart:     12,
}

// Kind is what a case is testing, for the table in the report.
type Kind string

const (
	QuietCase    Kind = "quiet background"
	BannerCase   Kind = "changing banner outside the region"
	AnimatedCase Kind = "animated content outside the region"
	AbsentCase   Kind = "intended control absent, decoy present"
	CoveredCase  Kind = "intended control covered"
	CrowdedCase  Kind = "partly covered, locally ambiguous"
	ReflowCase   Kind = "reflow beyond the neighbourhood"
	ResizedCase  Kind = "unsupported client size or scale"
	MovedCase    Kind = "window moved on screen"
)

// Case is one evaluation case: what to render, what was meant, and what a correct method does.
type Case struct {
	Kind  Kind
	Seed  int64
	Scene Scene
	// Want is the instance the demonstration was of.
	Want Instance
	// Supported says a correct method answers. When false it must REFUSE: there is no
	// right point to press, and choosing one anyway is the failure this measures.
	Supported bool
}

// Demonstration is the look a target is taught from, and the two targets taught in it.
//
// One look for both, because a person demonstrating two controls in the same window does it in
// the same window. The click is the centre of the control, which is where a person clicks.
func Demonstration() (Look, map[Instance]Target) {
	l := Render(Scene{Seed: demoSeed, Banner: Quiet})
	out := map[Instance]Target{}
	for _, in := range []Instance{LeftApply, RightApply} {
		at, ok := l.Centre(in)
		if !ok {
			continue
		}
		out[in] = Demonstrate(l, at, in, Frozen.Region)
	}
	return l, out
}

// SetupExamples is the examples the parameters were chosen against. Never scored.
func SetupExamples() []Case {
	var out []Case
	for i := 0; i < setupCount; i++ {
		seed := int64(setupFrom + i)
		banner := []BannerKind{Quiet, Changing, Animated}[i%3]
		want := LeftApply
		if i%2 == 1 {
			want = RightApply
		}
		out = append(out, Case{Kind: QuietCase, Seed: seed, Want: want,
			Supported: true,
			Scene:     Scene{Seed: seed, Banner: banner}})
	}
	return out
}

// Cases is the evaluation set: sixty supported, forty unsupported, ten moved.
//
// Ten per intended instance per background, with the other instance present as a decoy
// throughout — which it always is, because the fixture always draws both panels.
func Cases() []Case {
	var out []Case
	n := 0
	for _, want := range []Instance{LeftApply, RightApply} {
		for _, banner := range []BannerKind{Quiet, Changing, Animated} {
			kind := QuietCase
			switch banner {
			case Changing:
				kind = BannerCase
			case Animated:
				kind = AnimatedCase
			}
			for i := 0; i < 10; i++ {
				seed := int64(positiveFrom + n)
				n++
				out = append(out, Case{Kind: kind, Seed: seed,
					Want: want, Supported: true,
					Scene: Scene{Seed: seed, Banner: banner}})
			}
		}
	}

	// ── AND THE FOUR WAYS A CASE IS NOT SUPPORTED ──
	//
	// Ten each, all of them about the LEFT instance, with the right one present
	// and untouched. That is the shape that matters: the thing demonstrated is
	// unusable and something that looks exactly like it is sitting there.
	m := 0
	add := func(k Kind, make func(seed int64) Scene) {
		for i := 0; i < 10; i++ {
			seed := int64(negativeFrom + m)
			m++
			out = append(out, Case{Kind: k, Seed: seed, Want: LeftApply,
				Supported: false, Scene: make(seed)})
		}
	}
	left := LeftApply
	add(AbsentCase, func(seed int64) Scene {
		return Scene{Seed: seed, Banner: bannerFor(seed), Drop: &left}
	})
	add(CoveredCase, func(seed int64) Scene {
		return Scene{Seed: seed, Banner: bannerFor(seed), Cover: &left}
	})
	add(CrowdedCase, func(seed int64) Scene {
		return Scene{Seed: seed, Banner: bannerFor(seed), Cover: &left,
			Half: true}
	})
	add(ReflowCase, func(seed int64) Scene {
		return Scene{Seed: seed, Banner: bannerFor(seed), Reflow: 40}
	})
	// AND THE UNSUPPORTED GEOMETRY, which is five of each: a client area of
	// another size, and the same size at another display scale. Both are refused
	// for the same reason and by different fields, so both are exercised.
	for i := 0; i < 10; i++ {
		seed := int64(negativeFrom + m)
		m++
		s := Scene{Seed: seed, Banner: bannerFor(seed)}
		if i < 5 {
			s.Size = image.Pt(ClientW*5/4, ClientH*5/4)
		} else {
			// A DISPLAY SCALE CHANGE RE-RENDERS EVERYTHING BIGGER,
			// so it is a size change that also says what it was.
			// The scale field is checked on its own by
			// TestADifferentDisplayScaleIsRefused, where the pixels
			// can be held identical and only the claim differs.
			s.Size = image.Pt(ClientW*5/4, ClientH*5/4)
			s.Scale = 1.25
		}
		out = append(out, Case{Kind: ResizedCase, Seed: seed, Want: LeftApply,
			Supported: false, Scene: s})
	}

	// ── AND THE WINDOW SOMEWHERE ELSE ON THE SCREEN ──
	//
	// Supported, and counted apart from the sixty: every stored coordinate is
	// relative to the client area, so moving the window must change nothing at
	// all. A method that stored screen coordinates would fail all ten.
	for i := 0; i < 10; i++ {
		seed := int64(movedFrom + i)
		out = append(out, Case{Kind: MovedCase, Seed: seed, Want: LeftApply,
			Supported: true,
			Scene: Scene{Seed: seed, Banner: bannerFor(seed),
				Origin: image.Pt(37+i*53, 11+i*29)}})
	}
	return out
}

// bannerFor spreads the three backgrounds across the cases that are not about the background.
func bannerFor(seed int64) BannerKind {
	return []BannerKind{Quiet, Changing, Animated}[seed%3]
}
