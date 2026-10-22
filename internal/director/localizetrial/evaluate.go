package localizetrial

// SCORING, WHICH IS DELIBERATELY NOT AN ACCURACY NUMBER.
//
// Four outcomes, kept apart on purpose:
//
//	right      — it answered, and the point it chose is on the instance that was demonstrated
//	WRONG      — it answered, and the point it chose is on a DIFFERENT instance of the control
//	nowhere    — it answered, and the point it chose is on no control at all
//	refused    — it did not answer
//
// A wrong-instance selection and a refusal are not the same failure and must never be averaged:
// one presses something nobody asked for, the other presses nothing. The report prints all four
// per condition per kind, and the pass criterion is stated over them individually.
//
// Which instance a chosen point lands on comes from the FIXTURE'S OWN GROUND TRUTH — the
// rectangle it drew the control in — and not from anything the method believes. Nothing under
// test can reach `Look.Where`.

import (
	"fmt"
	"image"
	"sort"
	"strings"
)

// Outcome is what one method did with one case.
type Outcome string

const (
	Right   Outcome = "right"
	Wrong   Outcome = "WRONG INSTANCE"
	Nowhere Outcome = "chose a point on no control"
	Refused Outcome = "refused"
)

// Result is one case under one method.
type Result struct {
	Case    Case
	Method  string
	Outcome Outcome
	// Chose is the instance the chosen point landed on, when it landed on one.
	Chose Instance
	// Verdict is the method's own answer, kept whole so the report can quote the
	// refusal and the scores that produced it.
	Verdict Verdict
	// Correct says this is what a correct method does with this case: the right
	// instance when the case is supported, a refusal when it is not.
	Correct bool
}

// Method is one of the two things being compared.
type Method struct {
	Name string
	Run  func(Target, Look, Rule) Verdict
}

// Methods is the comparison: the same matching, differing only in where it looks.
func Methods() []Method {
	return []Method{
		{Name: "whole-window", Run: Whole},
		{Name: "local region", Run: Local},
	}
}

// Evaluate runs every case under both methods.
//
// The target is demonstrated ONCE, from one look, and every case is scored against that one
// record — which is what a person demonstrating something once and running it later has.
func Evaluate(cases []Case, rule Rule) []Result {
	_, targets := Demonstration()
	var out []Result
	for _, c := range cases {
		look := Render(c.Scene)
		target := targets[c.Want]
		for _, m := range Methods() {
			out = append(out, score(c, m, target, look, rule))
		}
	}
	return out
}

func score(c Case, m Method, t Target, l Look, rule Rule) Result {
	v := m.Run(t, l, rule)
	r := Result{Case: c, Method: m.Name, Verdict: v}
	if !v.Found {
		r.Outcome = Refused
		// A REFUSAL IS CORRECT EXACTLY WHEN THERE WAS NOTHING TO PRESS.
		r.Correct = !c.Supported
		return r
	}
	on, hit := l.Hit(v.Chose)
	switch {
	case !hit:
		r.Outcome = Nowhere
	case on == c.Want:
		r.Outcome, r.Chose = Right, on
		r.Correct = c.Supported
	default:
		r.Outcome, r.Chose = Wrong, on
	}
	return r
}

// Tally is the counts for one method and one kind of case.
type Tally struct {
	Method string
	Kind   Kind
	Cases  int
	Counts map[Outcome]int
	// Correct is how many of them a correct method would have got right, which for a
	// supported case is a selection of the demonstrated instance and for an
	// unsupported one is a refusal.
	Correct int
}

// Count groups results by method and kind, in a stable order.
func Count(results []Result) []Tally {
	index := map[string]*Tally{}
	var order []string
	for _, r := range results {
		key := r.Method + "\u0000" + string(r.Case.Kind)
		t, had := index[key]
		if !had {
			t = &Tally{Method: r.Method, Kind: r.Case.Kind,
				Counts: map[Outcome]int{}}
			index[key] = t
			order = append(order, key)
		}
		t.Cases++
		t.Counts[r.Outcome]++
		if r.Correct {
			t.Correct++
		}
	}
	sort.Strings(order)
	out := make([]Tally, 0, len(order))
	for _, k := range order {
		out = append(out, *index[k])
	}
	return out
}

// Failures is every result a correct method would not have produced, as sentences.
//
// Split by what went wrong rather than summed, because "chose the wrong instance" and "refused
// something it should have pressed" are the two different things this experiment is for.
func Failures(results []Result) (wrong, refusedButShould, choseWhenShouldNot []string) {
	for _, r := range results {
		if r.Correct {
			continue
		}
		where := fmt.Sprintf("%s · %s · seed %d", r.Method, r.Case.Kind,
			r.Case.Seed)
		switch {
		case r.Outcome == Wrong:
			wrong = append(wrong, fmt.Sprintf("%s — meant %s, chose %s",
				where, r.Case.Want, r.Chose))
		case r.Outcome == Nowhere:
			wrong = append(wrong, where+" — chose a point on no control")
		case r.Outcome == Refused:
			refusedButShould = append(refusedButShould,
				fmt.Sprintf("%s — %s", where, r.Verdict.Why))
		default:
			choseWhenShouldNot = append(choseWhenShouldNot,
				fmt.Sprintf("%s — chose %s where there was nothing "+
					"to press", where, r.Chose))
		}
	}
	return wrong, refusedButShould, choseWhenShouldNot
}

// Report is the table, written for the end-of-day report.
func Report(results []Result) string {
	var b []byte
	add := func(f string, a ...any) { b = append(b, fmt.Sprintf(f, a...)...) }
	add("%-14s %-42s %5s %7s %7s %9s %9s\n", "METHOD", "CASE", "n", "right",
		"WRONG", "nowhere", "refused")
	for _, t := range Count(results) {
		add("%-14s %-42s %5d %7d %7d %9d %9d\n", t.Method, t.Kind, t.Cases,
			t.Counts[Right], t.Counts[Wrong], t.Counts[Nowhere],
			t.Counts[Refused])
	}
	wrong, missed, pressed := Failures(results)
	add("\nWRONG-INSTANCE SELECTIONS  %d\n", len(wrong))
	for _, w := range wrong {
		add("  %s\n", w)
	}
	add("\nFALSE REFUSALS  %d\n", len(missed))
	for _, m := range missed {
		add("  %s\n", m)
	}
	add("\nSELECTED WHERE IT SHOULD HAVE REFUSED  %d\n", len(pressed))
	for _, p := range pressed {
		add("  %s\n", p)
	}
	return string(b)
}

// Supported is how many of a method's supported cases it got right, and how many there were.
func Supported(results []Result, method string) (right, of int) {
	for _, r := range results {
		if r.Method != method || !r.Case.Supported {
			continue
		}
		of++
		if r.Outcome == Right {
			right++
		}
	}
	return right, of
}

// Unsupported is how many of a method's unsupported cases it refused, and how many there were.
func Unsupported(results []Result, method string) (refused, of int) {
	for _, r := range results {
		if r.Method != method || r.Case.Supported {
			continue
		}
		of++
		if r.Outcome == Refused {
			refused++
		}
	}
	return refused, of
}

// WrongInstances is how many times a method pressed a different instance of the control.
func WrongInstances(results []Result, method string) int {
	n := 0
	for _, r := range results {
		if r.Method == method && r.Outcome == Wrong {
			n++
		}
	}
	return n
}

// at converts a client point to where it is on the virtual screen, which is the only place the
// window's position on screen is ever used.
func onScreen(l Look, client image.Point) image.Point { return client.Add(l.On.Min) }

// Reasons is how often each method refused for each reason.
//
// Printed beside the table because a refusal count on its own hides a question worth asking: an
// ambiguity rule that never fires is a rule nobody has tested, whatever the totals say.
func Reasons(results []Result) string {
	count := map[string]int{}
	var order []string
	for _, r := range results {
		if r.Outcome != Refused {
			continue
		}
		key := r.Method + " · " + why(r.Verdict.Why)
		if _, had := count[key]; !had {
			order = append(order, key)
		}
		count[key]++
	}
	sort.Strings(order)
	var b []byte
	b = append(b, "WHY EACH REFUSAL HAPPENED\n"...)
	for _, k := range order {
		b = append(b, fmt.Sprintf("  %4d  %s\n", count[k], k)...)
	}
	return string(b)
}

// why shortens a refusal to the rule that produced it.
func why(s string) string {
	switch {
	case strings.Contains(s, "more than one thing"):
		return "ambiguous: two things that look alike"
	case strings.Contains(s, "looks enough like"):
		return "below the acceptance threshold"
	case strings.Contains(s, "different layout"):
		return "the client area is not the demonstrated one"
	case strings.Contains(s, "scale"):
		return "the display scale is not the demonstrated one"
	}
	return s
}
