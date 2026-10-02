package main

import (
	"regexp"
	"strings"
	"testing"
)

// `marco help` is a PRODUCT LISTING, and the developer surfaces are not in it.
//
// # What went wrong before, and what it looked like
//
// `assistant` and `dispatch` are the pre-Director halves of Marco: an interactive loop and a
// phrase classifier that pick among the plays you already have, neither of which can see the
// screen or plan anything. They kept working, and they were listed beside `marco do` as though
// they were the same kind of thing — so the first surface a new person reads offered them three
// front doors and said nothing about which one is the product.
//
// Phase 5 moved them under a heading that says plainly what they are. Nothing held that: an
// independent mutation run put them back in the ordinary Assistant list and the whole tree stayed
// green, in both modules.
//
// # Why this parses the text rather than diffing it
//
// A golden copy of the help output fails for every wording change, which trains people to
// re-bless it, which is how it stops being read. What is asserted here are the two properties the
// listing has to have, over WHATEVER verbs the sections happen to hold: a verb is developer or it
// is product, never both; and the surfaces this phase demoted stay demoted.

// developerHeading is the section whose whole point is that what follows is not the product.
const developerHeading = "not part of the normal product"

// usageSections renders `marco help` and returns section heading → the verbs it offers.
//
// A section is a heading line at column 0 ending in a colon; its verbs are the indented
// `marco <verb> …` lines under it. Prose paragraphs at column 0 are not sections and the
// sentences inside them that happen to mention a verb are not offers.
func usageSections(t *testing.T) map[string][]string {
	t.Helper()
	var b strings.Builder
	usage(&b)

	// A bare list of alternatives — `diag | games | args` — is several verbs on one line.
	// Anything else (a quoted argument, a placeholder, a description) is one verb plus prose.
	alternatives := regexp.MustCompile(`^[a-z]+( \| [a-z]+)+$`)

	out := map[string][]string{}
	heading := ""
	for _, line := range strings.Split(b.String(), "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			continue
		case !strings.HasPrefix(line, " "):
			if strings.HasSuffix(strings.TrimRight(line, " "), ":") {
				heading = strings.TrimSpace(line)
				if _, seen := out[heading]; !seen {
					out[heading] = nil
				}
			}
			continue
		}
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "marco ")
		if !ok || heading == "" {
			continue
		}
		if alternatives.MatchString(strings.TrimSpace(rest)) {
			for _, v := range strings.Split(rest, "|") {
				out[heading] = append(out[heading], strings.TrimSpace(v))
			}
			continue
		}
		out[heading] = append(out[heading], strings.Fields(rest)[0])
	}
	return out
}

// A verb is offered as the product or as a developer surface, and never as both.
func TestNoDeveloperVerbIsAlsoOfferedAsTheProduct(t *testing.T) {
	sections := usageSections(t)

	var developer []string
	product := map[string]string{}
	found := false
	for heading, verbs := range sections {
		if strings.Contains(heading, developerHeading) {
			found = true
			developer = append(developer, verbs...)
			continue
		}
		for _, v := range verbs {
			product[v] = heading
		}
	}
	if !found {
		t.Fatalf("`marco help` no longer separates the developer surfaces from the product; "+
			"the headings are %v", headings(sections))
	}
	if len(developer) == 0 {
		t.Fatal("the developer section is empty, so the separation it exists to make is " +
			"vacuous and every verb in the help text now reads as the product")
	}
	for _, v := range developer {
		if where, dup := product[v]; dup {
			t.Errorf("`marco %s` is listed under %q AND under the developer heading.\n"+
				"The first surface a new person reads then offers it as the product, "+
				"which is what the developer heading exists to stop.", v, where)
		}
	}
}

// The two surfaces Phase 5 demoted stay demoted.
//
// Named, because the structural rule above cannot see a PROMOTION: a verb lifted out of the
// developer section and into the Assistant list appears in exactly one place, which is all that
// rule asks for. These two are the pre-Director loop and the pre-Director classifier — the halves
// the Director replaced — and putting either back beside `marco do` is a product decision that has
// to be made on purpose rather than by a tidy-up of the help text.
func TestThePreDirectorSurfacesStayUnderTheDeveloperHeading(t *testing.T) {
	sections := usageSections(t)
	for _, verb := range []string{"assistant", "dispatch"} {
		var listedUnder []string
		for heading, verbs := range sections {
			for _, v := range verbs {
				if v == verb {
					listedUnder = append(listedUnder, heading)
				}
			}
		}
		if len(listedUnder) == 0 {
			t.Errorf("`marco %s` vanished from the help text entirely. It still answers in "+
				"main.go, and a working verb nobody documents is worse than a demoted "+
				"one.", verb)
			continue
		}
		for _, heading := range listedUnder {
			if !strings.Contains(heading, developerHeading) {
				t.Errorf("`marco %s` is offered under %q.\nIt is the pre-Director "+
					"surface the Director replaced; listing it beside `marco do` "+
					"tells a new person it is one of the product's front doors.",
					verb, heading)
			}
		}
	}
}

// headings is the section list, for a failure message that says what was actually there.
func headings(sections map[string][]string) []string {
	var out []string
	for h := range sections {
		out = append(out, h)
	}
	return out
}

// THE FIRST THING `marco help` SAYS IS WHAT MARCO IS FOR.
//
// # What was wrong, and why it was the top market blocker
//
// An audit of the product read the whole surface a new user meets and found that `marco help` did
// not contain the word "observe" at all, and that its opening line described a macro recorder:
// `marco learn "log into facebook"`. Meanwhile the Director could already perform thirty-three
// verified semantic actions against whatever was on screen, needing no play, no demonstration and
// nothing learned first — and nothing anywhere told anybody so.
//
// A capability a person cannot discover has no value to them. This holds the two properties the
// listing now has to have, over whatever wording it carries:
//
//	saying what you want comes FIRST, and names no prerequisite
//	watching is offered at all
//
// It deliberately does not assert the prose. What it asserts is order and presence, because those
// are the two things that were wrong and the two a rewording could silently undo.
func TestHelpLeadsWithSayingWhatYouWant(t *testing.T) {
	var b strings.Builder
	usage(&b)
	text := b.String()

	// `do` IS THE FIRST VERB OFFERED. Not merely present — first. It was present before, on
	// the fourth line of a section headed by the recorder, which is how it read as a way to
	// replay something you had already taught.
	first := regexp.MustCompile(`(?m)^\s+marco (\S+)`).FindStringSubmatch(text)
	if first == nil {
		t.Fatal("the help offers no verbs at all")
	}
	if first[1] != "do" {
		t.Errorf("the first verb `marco help` offers is %q.\n\n"+
			"Saying what you want is the product and it needs no setup; a listing that "+
			"leads with anything else tells a new person the first thing they must do is "+
			"teach Marco something.", first[1])
	}

	// AND WATCHING IS MENTIONED. It was absent entirely — the headline capability of the
	// learning half, undiscoverable from the only surface a new person reads.
	if !strings.Contains(text, "marco observe") {
		t.Error("`marco help` does not mention `marco observe`. Watching is a real " +
			"capability and a person who cannot find it does not have it.")
	}

	// AND THE LEAD DOES NOT REQUIRE ANYTHING FIRST. The first section must not tell somebody
	// to record, learn or register before they can ask for something.
	lead := text
	if i := strings.Index(text, "\nPlays"); i > 0 {
		lead = text[:i]
	}
	for _, prerequisite := range []string{"record it once", "teach it first", "learn it first"} {
		if strings.Contains(lead, prerequisite) {
			t.Errorf("the opening section says %q. `marco do \"click Save\"` works on a "+
				"screen Marco has never seen, with nothing learned and nothing "+
				"remembered.", prerequisite)
		}
	}
}
