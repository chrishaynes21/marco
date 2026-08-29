package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/uiaclient"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WHAT DOES THIS SCREEN SAY IT IS, AND WHICH OF ITS WORDS IS THE PLACE?
//
// # Why this exists
//
// A screen carries several TRUE names at once. Measured on Windows Settings' Printers page:
// `Settings` is the application, `Bluetooth & devices` is the section it sits under and the
// selected item in the rail, and `Printers & scanners` is where the person actually is. Only the
// last may be a Place's name, and 37J caught the rule admitting the section instead — at one
// window width and not the others, which is the hardest kind of wrong to find by reading code.
//
// So this prints every claim the production producer found, what the production rule made of
// each, and why. It calls `placeNameEvidence` and `observe.ExplainPlaceName` — the same two
// functions a live sample calls, in the same order. A probe with its own parser would answer a
// question about itself, and would agree with production right up until the moment somebody
// needed it not to.
//
// It performs no input, starts no session, writes nothing and carries no authority.

// runNameProbe is `director name-probe --application <app>`.
func runNameProbe(args []string) int {
	fs := flag.NewFlagSet("name-probe", flag.ExitOnError)
	app := fs.String("application", "", "the application to read")
	title := fs.String("title", "", "or the window whose title contains this")
	deep := fs.Bool("deep", false, "also show what contains every claim and every sibling group")
	bridgeFlag := fs.String("accessibility", defaultBridge(), "path to the accessibility bridge")
	_ = fs.Parse(flagsFirst(args))

	if *app == "" && *title == "" {
		fmt.Fprintln(os.Stderr,
			"director: one of --application or --title is required\n"+
				"  example: director name-probe --application applicationframehost")
		return 2
	}

	ctx := context.Background()
	windows := winprovider.New()
	tracker := windowref.NewTracker(windows)
	selector := windowref.Selector{Application: *app}
	if *title != "" {
		selector = windowref.Selector{Title: *title}
	}
	ref := tracker.AcquireBy(ctx, windows, windowref.NewDirectory(), selector)
	if !ref.State.OK() {
		fmt.Fprintf(os.Stderr, "director: %s (%s)\n", ref.Reason, ref.State)
		return 1
	}

	host := bridgehost.New(*bridgeFlag)
	defer host.Close()
	uia := uiaclient.New(host)
	if !uia.Available(ctx) {
		fmt.Fprintln(os.Stderr, "director: the accessibility provider reports it is unavailable")
		return 1
	}
	collector := providers.NewCollector(
		providers.NewAccessibility(uia).WithTargetResolver(newTargetResolver(tracker)),
		providers.NewWindowSystem(windows),
	)
	window := ref.Ref.ID
	cycle := collector.Collect(ctx, observation.Request{
		Window: &window, Target: expectedTarget(ref.Ref),
	})
	world, _, err := fusion.NewEngine().Fuse(cycle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: fusing: %v\n", err)
		return 1
	}

	fmt.Printf("\n%s at %dx%d — %d elements\n\n",
		ref.Ref.Application, ref.Ref.Bounds.Width, ref.Ref.Bounds.Height, len(world.Elements))

	// THE PRESENTATION, from the interface's own evidence rather than from the width.
	//
	// Windows moves its breakpoints with DPI and font scaling, so a pixel figure describes one
	// machine. What describes the STATE is the affordance the collapsed layout adds: a control
	// for opening the navigation exists exactly when the navigation is not there.
	fmt.Printf("  navigation    %s\n", navigationState(world))

	// THE EVIDENCE THE RULE WORKS FROM, before the rule reads it. A claim rejected for want
	// of a trail and a claim rejected because the trail said something else are different
	// findings, and only this line tells them apart.
	for _, group := range siblingButtonGroups(world) {
		fmt.Printf("  siblings      %v\n", group)
	}

	if *deep {
		reportStructure(world)
	}

	naming := observe.ExplainPlaceName(placeNameEvidence(world))
	if len(naming.Claims) == 0 {
		fmt.Printf("  claims        none — nothing in this observation claimed a destination\n")
	}
	for _, c := range naming.Claims {
		mark := " "
		if c.Admitted {
			mark = "*"
		}
		fmt.Printf("  %s %-24q %-12s %-16s %s\n", mark, c.Value, c.Level, c.Function, c.Why)
	}
	fmt.Println()
	if naming.Name == "" {
		fmt.Printf("  NO NAME       %s\n", naming.Why)
	} else {
		fmt.Printf("  DESTINATION   %q\n", naming.Name)
	}
	return 0
}

// navigationState says whether anything on screen reports itself as the selected destination.
//
// # Why this and not a width, and not a label
//
// Windows moves its breakpoints with DPI and font scaling, so a pixel figure describes one
// machine. And a control called `Open Navigation` describes one operating system in one
// language — matching it here would put a Windows string into a diagnostic that is supposed to
// generalise.
//
// A SELECTED NAVIGABLE ELEMENT is neither. It is the thing an application produces when it is
// showing you where you are among places you could be, it is what `placeNameEvidence` collects,
// and it is exactly what disappears when a responsive layout puts the navigation away. So the
// presentation is reported in the same terms the naming rule reasons in, which is what makes the
// two lines below explain each other.
func navigationState(world directorapi.WorldState) string {
	var selected []string
	for _, el := range world.Elements {
		if el == nil || el.Offscreen || !el.Visible || !el.Selected {
			continue
		}
		if el.Role.Navigable() {
			selected = append(selected, fmt.Sprintf("%s %q", el.Role, el.Label))
		}
	}
	sort.Strings(selected)
	if len(selected) == 0 {
		return "no element reports itself as the selected destination"
	}
	return fmt.Sprintf("%v", selected)
}

// siblingButtonGroups is the raw material `trailContaining` searches.
//
// Reported rather than summarised because the question this probe was built for is why a trail
// was not found, and "there was no group holding that word" and "the group held something else"
// send a reader to different places.
func siblingButtonGroups(world directorapi.WorldState) [][]string {
	groups := map[directorapi.ElementID][]string{}
	for _, el := range world.Elements {
		if el == nil || el.Role != directorapi.RoleButton || el.ParentID == nil {
			continue
		}
		if el.Offscreen || !el.Visible {
			continue
		}
		if label := el.Label; label != "" {
			groups[*el.ParentID] = append(groups[*el.ParentID], label)
		}
	}
	out := make([][]string, 0, len(groups))
	for _, labels := range groups {
		if len(labels) < 2 {
			continue
		}
		sort.Strings(labels)
		out = append(out, labels)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

// ancestryOf is the chain of roles above an element, nearest first.
//
// The question 37L opens with: can the accessibility hierarchy ABOVE a claim say whether it names
// where you are or the region you are in? A selected rail item and a selected breadcrumb step are
// both `selected` and both navigable; if anything separates them it is what contains them.
//
// Bounded, because a tree can be deep and a diagnostic that printed forty ancestors would be read
// by nobody.
func ancestryOf(world directorapi.WorldState, el *directorapi.Element) []string {
	var out []string
	cur := el
	for depth := 0; depth < 8; depth++ {
		if cur.ParentID == nil {
			break
		}
		parent, ok := world.Elements[*cur.ParentID]
		if !ok || parent == nil {
			break
		}
		label := parent.Label
		if label == "" {
			label = "-"
		}
		out = append(out, fmt.Sprintf("%s(%s)", parent.Role, label))
		cur = parent
	}
	return out
}

// reportStructure prints, for every selected element and every sibling-button group, what contains
// it. This is the Stage A measurement: whether local structure separates a destination from a
// section without asking memory anything.
func reportStructure(world directorapi.WorldState) {
	type row struct {
		what      string
		ancestors []string
	}
	var rows []row
	for _, el := range world.Elements {
		if el == nil || el.Offscreen || !el.Visible {
			continue
		}
		if el.Selected {
			rows = append(rows, row{
				what:      fmt.Sprintf("selected %s %q", el.Role, el.Label),
				ancestors: ancestryOf(world, el),
			})
			continue
		}
		if el.Role == directorapi.RoleButton && el.Label != "" && el.ParentID != nil {
			if siblingCount(world, *el.ParentID) < 2 {
				continue
			}
			rows = append(rows, row{
				what:      fmt.Sprintf("button   %q", el.Label),
				ancestors: ancestryOf(world, el),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].what < rows[j].what })
	for _, r := range rows {
		fmt.Printf("  where         %-46s in %v\n", r.what, r.ancestors)
	}
}

func siblingCount(world directorapi.WorldState, parent directorapi.ElementID) int {
	n := 0
	for _, el := range world.Elements {
		if el != nil && el.ParentID != nil && *el.ParentID == parent &&
			el.Role == directorapi.RoleButton && el.Label != "" && !el.Offscreen && el.Visible {
			n++
		}
	}
	return n
}
