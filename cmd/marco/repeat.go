package main

// REPEAT: ONE SAVED ROUTINE, A FIXED NUMBER OF TIMES, CONFIRMED FIRST.
//
// "repeat open social five times" is read on the widget's command line (`marco line`) as the
// routine `open social` and the count 5, and read back as "Open social ×5". Enter on that
// read-back runs `marco do --play=… --expect=… --times=5`, and that is the only way a repeat
// runs: words arriving at the intake in this form are PROPOSED, never performed, so a terminal,
// Studio's ask box or a widget whose reading timed out gets the same review a typed line does.
//
// Each run is the ordinary one (performOnePlay): the scope, the authority door, the version
// confirmed and the values are checked every time, and the first run that does not perform ends
// the repeat with that run's outcome. A Stop raised at any point ends it too, including between
// two runs, where no run is watching for it. What ran is said on `[ran] `: "open social ×5", or
// "open social ×2 of 5" when it stopped early.
//
// Only the explicit form is read here, and only a routine Marco can run by name: a description
// ("open the social menu five times") has its routine named by Director's resolver, and the
// widget then puts the explicit form on the command line for the same review. "repeat that" and
// "repeat until …" are not read at all. See [[ADR-325-repeat-a-routine-a-fixed-number-of-times]].

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/dispatch"
	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/resolveroute"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// maxRepeat is the most runs one repeat asks for. A count past it is somebody mis-speaking, and
// a routine pressing keys a thousand times is not a request to guess at.
const maxRepeat = 100

// numberWords are the counts a recogniser writes out.
var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7, "eight": 8,
	"nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14,
	"fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19,
}

var tensWords = map[string]int{
	"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70,
	"eighty": 80, "ninety": 90,
}

// repeatCount reads a count: digits, or the words a recogniser writes ("five", "twenty five",
// "a hundred"). ok is false for anything else and for zero; a count past maxRepeat is still read,
// so it can be refused as one rather than taken for words.
func repeatCount(s string) (int, bool) {
	words := strings.Fields(strings.ToLower(strings.ReplaceAll(s, "-", " ")))
	n := 0
	switch {
	case len(words) == 1:
		if v, err := strconv.Atoi(words[0]); err == nil {
			n = v
		} else if v, ok := numberWords[words[0]]; ok {
			n = v
		} else if v, ok := tensWords[words[0]]; ok {
			n = v
		}
	case len(words) == 2 && (words[0] == "a" || words[0] == "one") && words[1] == "hundred":
		n = 100
	case len(words) == 2:
		t, okT := tensWords[words[0]]
		u, okU := numberWords[words[1]]
		if okT && okU && u < 10 {
			n = t + u
		}
	}
	return n, n >= 1
}

// parseRepeat reads "repeat <routine> <count> times" (also "… twice", "… once", "… one time"),
// and the same without the leading "repeat". It returns the routine's words as typed, the count,
// and whether the line has that form at all; explicit is whether it began with "repeat".
func parseRepeat(line string) (inner string, times int, explicit bool, ok bool) {
	words := strings.Fields(line)
	if len(words) >= 1 && strings.EqualFold(words[0], "repeat") {
		explicit, words = true, words[1:]
	}
	if len(words) < 2 {
		return "", 0, false, false
	}
	last := strings.ToLower(words[len(words)-1])
	var countWords []string
	switch last {
	case "twice":
		times, words = 2, words[:len(words)-1]
	case "once":
		times, words = 1, words[:len(words)-1]
	case "times", "time":
		words = words[:len(words)-1]
		// The count is the one or two words before "times": "twenty five times", "5 times".
		for take := 2; take >= 1; take-- {
			if len(words) <= take {
				continue
			}
			if n, ok := repeatCount(strings.Join(words[len(words)-take:], " ")); ok {
				times, countWords = n, words[len(words)-take:]
				words = words[:len(words)-take]
				break
			}
		}
		if countWords == nil {
			return "", 0, false, false
		}
		if last == "time" && times != 1 {
			return "", 0, false, false
		}
	default:
		return "", 0, false, false
	}
	if len(words) == 0 {
		return "", 0, false, false
	}
	return strings.Join(words, " "), times, explicit, true
}

// timesWord is the read-back's count: "×5".
func timesWord(n int) string { return "×" + strconv.Itoa(n) }

// performRepeated runs one confirmed play times times, one after another, and stops at the first
// run that does not perform or at a Stop.
//
// Removing the Stop check between runs must fail TestAStopBetweenRunsEndsTheRepeat; running past
// a run that did not perform must fail TestARepeatStopsAtTheFirstRunThatDoesNotPerform.
func performRepeated(d orchestrator.Deps, p playInvocation, times int) (Outcome, error) {
	home := stopsignal.Home()
	// THE STOP GENERATION WHEN THE REPEAT BEGAN. A run watches for a Stop only while it runs, so
	// one raised in the moment between two runs would be adopted as the next run's baseline and
	// the repeat would carry on.
	began := stopsignal.Generation(home)
	done := 0
	out, err := OutcomePerformed, error(nil)
	for i := 1; i <= times; i++ {
		if stopsignal.Generation(home) != began {
			out = OutcomeCancelled
			break
		}
		// WHERE IT IS, for the widget's status line and anyone reading along. The routine's
		// name only, never a value.
		fmt.Printf("%s%d/%d] %s\n", repeatPrefix, i, times, capitalised(plays.Pretty(p.rt.Slug)))
		out, err = performStep(d, p)
		if out != OutcomePerformed {
			break
		}
		done++
	}
	// WHAT RAN, LAST, so it is the `[ran] ` a front end keeps: each run announced its own.
	line := ranLine(d, p.rt, p.named, p.positional) + " " + timesWord(times)
	if done < times {
		line = ranLine(d, p.rt, p.named, p.positional) + " " + timesWord(done) + " of " +
			strconv.Itoa(times)
		fmt.Printf("stopped after %d of %d\n", done, times)
	}
	fmt.Printf("%s%s\n", outcome.RanPrefix, line)
	return out, err
}

// repeatPrefix starts the progress line, "[repeat 2/5] Open social".
const repeatPrefix = "[repeat "

// performStep is one run of a repeat. A package variable so a test can count and end runs; the
// production value is the ordinary run. Production never reassigns it.
var performStep = performOnePlay

// withTimes is a proposal's JSON line with the count beside the answer, as `times`.
func withTimes(line string, times int) string {
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(line), &m) != nil {
		return line
	}
	m["times"] = json.RawMessage(strconv.Itoa(times))
	b, err := json.Marshal(m)
	if err != nil {
		return line
	}
	return string(b)
}

// localProposal is a routine named by its own words, as the JSON a resolver's proposal is: the
// identity, and the values the words carried, on the routine's names as `marco line` puts them
// (a bare value on the one value it takes).
func localProposal(d orchestrator.Deps, rt routes.Route, form string, named map[string]string,
	positional []string) string {
	a := resolveroute.Answer{Kind: dispatch.IntentRun, ID: rt.ID(), App: rt.App, Slug: rt.Slug,
		Focus: rt.Focus, Name: plays.Pretty(rt.Slug), Source: "exact", Library: marcopaths.Routes(),
		Resolver: "no resolver was asked: the words named the routine"}
	if raw, err := os.ReadFile(d.Reg.Path(rt)); err == nil {
		src := string(raw)
		a.SourceAt = routes.DigestOf(src)
		values := map[string]string{}
		for k, v := range named {
			values[strings.ToLower(k)] = v
		}
		order := routes.ArgNames(src)
		if form == "bare" {
			for _, arg := range routes.Arguments(src) {
				if !arg.Secret {
					order = []string{arg.Name}
				}
			}
		}
		for i, v := range positional {
			if i < len(order) {
				values[strings.ToLower(order[i])] = v
			}
		}
		for _, arg := range routes.Arguments(src) {
			if v, ok := values[strings.ToLower(arg.Name)]; ok && !arg.Secret {
				a.Values = append(a.Values, resolveroute.Proposal{Name: arg.Name, Value: v,
					From: routes.FromExtracted})
			}
		}
	}
	b, _ := json.Marshal(service.ResolveRouteResponse{Answer: a})
	return string(b)
}

// proposeRepeat answers words in the repeat form that reached the intake: never run, always
// proposed, with the routine and the count, for the review the widget's command line gives
// (performed only by `--times` on a confirmed identity). ok is false when the words are not that
// form, or name no routine Marco can run.
func proposeRepeat(d orchestrator.Deps, phrase, app string) (Outcome, bool) {
	inner, times, _, ok := parseRepeat(phrase)
	if !ok {
		return "", false
	}
	if times > maxRepeat {
		fmt.Printf("a repeat runs at most %d times — nothing was run\n", maxRepeat)
		return OutcomeRefused, true
	}
	if rt, form, named, positional, ok := lineRoutine(d, app, inner); ok &&
		!(form == "bare" && typedArgs(d, rt) != 1) {
		// THE SAME PROPOSAL A RESOLVER'S ANSWER IS, so the widget reviews it on its command
		// line however the words got here (a reading that timed out, a spoken line).
		fmt.Printf("%s%s\n", outcome.ProposedPrefix, withTimes(localProposal(d, rt, form, named, positional), times))
		fmt.Printf("Marco would run %s %s. Nothing has run; confirm it on the widget's command "+
			"line, or with marco do --play=%s --times=%d.\n", capitalised(plays.Pretty(rt.Slug)),
			timesWord(times), rt.Slug, times)
		return OutcomeClarify, true
	}
	// NOT A ROUTINE BY ITS NAME: Director's resolver is asked about the routine's words alone,
	// and the count travels beside its answer.
	if line, name, ok := proposeRoutine(inner); ok {
		if skillProposal(line) {
			// A BUILT-IN SKILL IS NOT REPEATED in this slice: there is nothing to say "×5" about.
			fmt.Printf("%s is one of Marco's built-in skills, and those are not repeated — nothing "+
				"was run\n", name)
			return OutcomeRefused, true
		}
		fmt.Printf("%s%s\n", outcome.ProposedPrefix, withTimes(line, times))
		fmt.Printf("Marco has a saved routine that may be what you meant: %s, %s. Nothing has run; "+
			"review it first.\n", name, timesWord(times))
		return OutcomeClarify, true
	}
	return "", false
}
