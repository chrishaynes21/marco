package main

// COMPOSING AN ACT AND THEN SELECTING IT, THE WAY A PERSON WOULD.
//
// `--play` selects a saved Act and nothing created one: `Compose` and `Save` had only test
// callers, so the path approved for live acceptance could not be reached by anybody. These drive
// the two commands in the order a person uses them — compose, then run — and the second one only
// gets as far as it can without a window, which is the point: what is being established is that
// the Act a person composes is an Act the runner accepts.

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// aDemonstrationToComposeFrom saves a two-interaction routine in an isolated store.
func aDemonstrationToComposeFrom(t *testing.T) workflowstore.Artifact {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", filepath.Join(home, "routes"))

	control := func(label string, y float64) workflowstore.Control {
		return workflowstore.Control{Label: label,
			Readings: []observe.Region{{X: 0.41, Y: y, Width: 0.07, Height: 0.03}}}
	}
	art := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics live",
		SavedAt:     time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Application: "ArkAscended.exe",
		Screens: []workflowstore.Screen{
			{Label: "screen_a"}, {Label: "screen_b"}},
		Controls: []workflowstore.Control{
			control("control_1", 0.24), control("control_2", 0.29)},
		Steps: []workflowstore.Step{
			{Position: 0, From: "screen_a", To: "screen_b", Control: "control_1",
				Cause: observe.CauseClick, Seen: 2, Outcome: []string{"bloom"}},
			{Position: 1, From: "screen_b", To: "screen_b", Control: "control_2",
				Cause: observe.CauseClick, Seen: 2, Outcome: []string{"master"}},
		},
	}
	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	return art
}

// AN ACT COMPOSED BY THE COMMAND IS AN ACT THE RUNNER ACCEPTS.
func TestAnActComposedByTheCommandPreflights(t *testing.T) {
	art := aDemonstrationToComposeFrom(t)
	if code := runComposeAct([]string{
		"--from", art.Name, "--name", "graphics preset",
		"--interaction", "0=the graphics tab",
		"--interaction", "1=apply changes"}); code != 0 {
		t.Fatalf("composing exited %d", code)
	}

	// IT IS ON DISK AS A PLAY WITH BINDINGS, and the pair preflights — which is the
	// claim: a person's Act is one the runner will take.
	rt := routes.Route{Slug: routes.Slug("graphics preset")}
	src, binds, err := demonstrated.Load(routesHere(), rt)
	if err != nil {
		t.Fatalf("the Act did not come back off disk: %v", err)
	}
	store, serr := demo.Open(configDir())
	if serr != nil {
		t.Fatal(serr)
	}
	plan, perr := demonstrated.Preflight(src, binds, store, marcoChecks{})
	if perr != nil {
		t.Fatalf("the composed Act does not preflight: %v", perr)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("the plan binds %d interaction(s)", len(plan.Steps))
	}
	// AND THE PLAY SAYS WHAT THE PERSON CALLED THINGS, not control_1.
	for _, want := range []string{`"the graphics tab"`, `"apply changes"`} {
		if !strings.Contains(src, want) {
			t.Errorf("the play does not name %s:\n%s", want, src)
		}
	}
	if strings.Contains(src, "control_1") {
		t.Errorf("the play names a perception label:\n%s", src)
	}
	// AND THE PLAY IS NAMED FOR THE ACT, not for the demonstration it came from.
	//
	// Three places say a name: the header comment, the actor, and the line logged when
	// every interaction verified. All three must be the ACT's — an Act called "graphics
	// preset" that ends by logging "ark graphics live: every action verified" is telling
	// a person about a file they are not looking at. The demonstration is named once, as
	// where the interactions came from.
	if !strings.Contains(src, "// graphics preset — an Act") {
		t.Errorf("the play's header is not about the Act:\n%s", src)
	}
	if !strings.Contains(src, "the GraphicsPreset is an actor") {
		t.Errorf("the actor is not named for the Act:\n%s", src)
	}
	if !strings.Contains(src, `log "graphics preset: every action verified"`) {
		t.Errorf("the success line is not about the Act:\n%s", src)
	}
	if !strings.Contains(src, `demonstrated as "ark graphics live"`) {
		t.Errorf("the play does not say which demonstration it came from:\n%s", src)
	}
	if strings.Contains(src, `log "ark graphics live: every`) {
		t.Errorf("the success line still names the demonstration:\n%s", src)
	}
	// AND THE PROVENANCE IS TRUTHFUL.
	if o, _ := routesHere().Origin(rt); o.Kind != routes.KindDemonstrated {
		t.Errorf("the provenance says %q", o.Kind)
	}
}

// A SUBSET AND A REORDERING ARE BOTH EXPRESSIBLE, which is what the flag order is for.
func TestAnActMayBeASubsetOrReordered(t *testing.T) {
	art := aDemonstrationToComposeFrom(t)
	if code := runComposeAct([]string{
		"--from", art.Name, "--name", "just apply",
		"--interaction", "1=apply changes"}); code != 0 {
		t.Fatalf("composing a subset exited %d", code)
	}
	if code := runComposeAct([]string{
		"--from", art.Name, "--name", "backwards",
		"--interaction", "1=apply changes",
		"--interaction", "0=the graphics tab"}); code != 0 {
		t.Fatalf("composing a reordering exited %d", code)
	}
	src, _, err := demonstrated.Load(routesHere(),
		routes.Route{Slug: routes.Slug("backwards")})
	if err != nil {
		t.Fatal(err)
	}
	// THE ORDER IS THE FLAG ORDER: `apply changes` is asked for first.
	first := strings.Index(src, `"apply changes"`)
	second := strings.Index(src, `"the graphics tab"`)
	if first < 0 || second < 0 || first > second {
		t.Fatalf("the Act does not perform the interactions in the order asked "+
			"for:\n%s", src)
	}
}

// AND IT REFUSES THE THINGS IT SHOULD.
func TestComposingAnActRefusesWhatItCannotDo(t *testing.T) {
	art := aDemonstrationToComposeFrom(t)
	for _, c := range []struct {
		name string
		args []string
		want int
	}{
		{"no demonstration named", []string{"--name", "x",
			"--interaction", "0=a"}, 2},
		{"no name for the Act", []string{"--from", art.Name,
			"--interaction", "0=a"}, 2},
		{"no interactions", []string{"--from", art.Name, "--name", "x"}, 2},
		{"an interaction with no name", []string{"--from", art.Name,
			"--name", "x", "--interaction", "0="}, 2},
		{"an interaction written without a name at all", []string{
			"--from", art.Name, "--name", "x", "--interaction", "0"}, 2},
		{"a demonstration nobody saved", []string{"--from", "nothing",
			"--name", "x", "--interaction", "0=a"}, 1},
		{"an interaction the demonstration does not have", []string{
			"--from", art.Name, "--name", "x", "--interaction", "7=a"}, 1},
		{"two interactions of one name", []string{"--from", art.Name,
			"--name", "x", "--interaction", "0=a", "--interaction", "1=a"}, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			if code := runComposeAct(c.args); code != c.want {
				t.Fatalf("exited %d, want %d", code, c.want)
			}
		})
	}
	// AND IT NEVER REPLACES AN ACT SOMEBODY HAS.
	if code := runComposeAct([]string{"--from", art.Name, "--name", "mine",
		"--interaction", "0=the graphics tab"}); code != 0 {
		t.Fatal("the first save failed")
	}
	if code := runComposeAct([]string{"--from", art.Name, "--name", "mine",
		"--interaction", "1=apply changes"}); code == 0 {
		t.Fatal("composing over an Act that already exists returned 0")
	}
	// THEIRS IS UNTOUCHED.
	src, _, err := demonstrated.Load(routesHere(), routes.Route{Slug: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, `"the graphics tab"`) {
		t.Fatalf("the Act that was already there was changed:\n%s", src)
	}
}
