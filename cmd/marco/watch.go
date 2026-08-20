package main

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// The thin client's visibility surface.
//
// # Why the CLI renders the same value the overlay does
//
// Because the point of the playbill is that there is ONE account. A person testing
// Marco will have the overlay's Watch panel open and a terminal beside it, and the two
// disagreeing about whether Marco recognises a screen would waste an afternoon before
// anybody suspected the surfaces rather than the Director.
//
// So neither of them writes a sentence. `playbill.View.Watch()` produces the lines and
// both render them; the only thing this file decides is how an indent looks in a
// terminal.
//
// # Three readings, one request
//
//	marco director watch      [--json] [--cursor=N]   what Marco sees and believes
//	marco director diagnose   [--json]                the evidence underneath it
//	marco director normal     [--json]                the one-line consumer reduction
//
// `normal` exists as PROOF rather than as product: it demonstrates that a consumer
// surface reduces from the same value, so the two can never disagree about whether
// Marco has a question.

// directorWatch prints what the Director currently sees, believes and needs.
//
// Read-only, and cheap enough to poll: the service copies state it already holds. This
// starts no observation, takes no sample and forms no interpretation — a person running
// it in a loop beside a live session cannot change what that session sees.
func directorWatch(jsonMode, deep bool, cursor uint64) int {
	view, code := fetchPlaybill(deep, cursor)
	if jsonMode {
		directorPrintJSON(view)
		return code
	}
	lines := view.Watch()
	if deep {
		lines = view.Deep()
	}
	printPlaybill(lines)
	return code
}

// directorNormal prints the consumer reduction: one word and one sentence.
//
// It renders the SAME account through the SAME reduction whether or not the Director is
// running — which is the point. "Marco is asleep" is a headline like any other, and a
// consumer surface that had to special-case it would be a second reduction.
func directorNormal(jsonMode bool) int {
	view, code := fetchPlaybill(false, 0)
	h := view.Normal()
	if jsonMode {
		directorPrintJSON(h)
		return code
	}
	fmt.Println(h.Word)
	if h.Detail != "" {
		fmt.Println("  " + h.Detail)
	}
	return code
}

// fetchPlaybill gets the account, rendering "not running" as a normal condition.
//
// A front-end polling this needs "the Director is asleep" to be DATA rather than a parse
// error on empty output — the same rule `perception --json` already follows. The exit code
// still says 1, because the payload is for a parser and the code is for a script, and they
// should not disagree.
//
// Nothing is printed here. Every caller renders the account its own way, and a helper that
// printed on the way past would mean `normal` showing the Watch panel's wording — which is
// exactly what it did before this comment existed.
func fetchPlaybill(deep bool, cursor uint64) (playbill.View, int) {
	c, err := directorConnect(false)
	if err != nil {
		return playbill.Unavailable(playbill.Absent,
			"the Director service is not running — start it with: director serve"), 1
	}
	defer c.Close()

	res, err := c.Playbill(service.PlaybillPayload{Cursor: cursor, Diagnostics: deep})
	if err != nil {
		return playbill.Unavailable(playbill.Unreachable, err.Error()), 1
	}
	return res.View, 0
}

// printPlaybill lays the lines out for a terminal.
//
// Presentation only — indentation and a blank line before a heading. Nothing here
// decides what a line SAYS, which is what keeps this surface and the overlay's honest
// about being the same account.
func printPlaybill(lines []playbill.Line) {
	for _, l := range lines {
		switch {
		case l.Head:
			fmt.Println(strings.ToLower(l.Text))
		case l.Text == "":
			fmt.Println()
		default:
			fmt.Println(strings.Repeat("  ", l.Indent+1) + l.Text)
		}
	}
}
