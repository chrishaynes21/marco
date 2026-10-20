package main

// COMPOSING AN ACT, WHICH IS THE HALF A PERSON CANNOT DO WITHOUT.
//
// `perform-workflow --play` selects a saved Act, preflights it and performs it. Nothing SAVED
// one: `demonstrated.Compose` and `demonstrated.Save` had only test callers, so the path the
// Architect approved for live acceptance was unreachable by a person — there was no way to get an
// Act onto the machine in the first place.
//
// This is that way, and it is deliberately small. It takes a demonstration somebody already gave,
// the interactions of it they want performed, in the order they want them, and the name they want
// to call the result. It writes the play, its bindings and its provenance, and then PRINTS THE
// PLAY — because the whole point of an Act being legal Marco is that a person can read what they
// just saved and recognise it.
//
// It performs nothing, presses nothing, and opens no window.

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// pickedInteractions collects repeated --interaction flags, in the order they were given.
//
// The ORDER IS THE FLAG ORDER, which is what makes a reordered Act expressible: the play performs
// its sentences in the order they are written, and they are written in the order asked for here.
type pickedInteractions []demonstrated.Named

func (p *pickedInteractions) String() string { return fmt.Sprint(*p) }

// Set parses `<position>=<name>`: which interaction of the demonstration, and what to call it.
//
// Two halves because they answer different questions and neither can be guessed. The position
// says which of somebody's demonstrated interactions is meant — a demonstration has several and
// an Act may want one of them. The name is what the play SAYS, and it is the person's: a
// generated one would be `control_1`, which is a label scoped to a file and means nothing
// tomorrow.
func (p *pickedInteractions) Set(v string) error {
	at, name, ok := strings.Cut(v, "=")
	if !ok {
		return fmt.Errorf("write it as <interaction>=<name>, for example "+
			`0="the graphics tab" (got %q)`, v)
	}
	n, err := strconv.Atoi(strings.TrimSpace(at))
	if err != nil {
		return fmt.Errorf("%q is not an interaction number", at)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("interaction %d needs a name — the words you would use for "+
			"it, which is what the play will say", n)
	}
	*p = append(*p, demonstrated.Named{Position: n, Name: strings.TrimSpace(name)})
	return nil
}

func runComposeAct(args []string) int {
	fs := flag.NewFlagSet("compose-act", flag.ContinueOnError)
	from := fs.String("from", "",
		"the demonstration to compose from, by the name you saved it under "+
			"(`director sequences` lists them)")
	name := fs.String("name", "",
		"what to call the Act. Must not be a play you already have: replacing one is "+
			"your decision, not this command's")
	var picked pickedInteractions
	fs.Var(&picked, "interaction",
		"an interaction to perform, as `<number>=<name>` — repeat it, in the order "+
			"you want them performed")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	switch {
	case *from == "":
		fmt.Fprintln(os.Stderr, "director: name the demonstration to compose from.\n"+
			"  director compose-act --from \"ark graphics live\" \\\n"+
			"      --name \"graphics preset\" \\\n"+
			"      --interaction '0=the graphics tab' \\\n"+
			"      --interaction '1=apply changes'")
		return 2
	case *name == "":
		fmt.Fprintln(os.Stderr, "director: name the Act. An Act is a play you will run "+
			"by name, so it needs one of its own.")
		return 2
	case len(picked) == 0:
		fmt.Fprintln(os.Stderr, "director: name at least one interaction to perform, "+
			"as --interaction '0=what you call it'.")
		return 2
	}

	store, err := demo.Open(configDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	art, ok := store.Sequence(*from)
	if !ok {
		if why := store.SequenceProblem(*from); why != "" {
			fmt.Fprintf(os.Stderr, "director: %q cannot be read: %s\n", *from, why)
			return 1
		}
		fmt.Fprintf(os.Stderr, "director: nothing called %q has been saved.\n"+
			"  director sequences    what you have shown it\n", *from)
		return 1
	}

	src, binds, cerr := demonstrated.Compose(art, *name, picked, time.Now(), marcoChecks{})
	if cerr != nil {
		// WHICH INTERACTIONS THERE ACTUALLY ARE, beside the refusal. A person who
		// asked for interaction 3 of a two-interaction demonstration needs to see the
		// two rather than be told a number is wrong.
		fmt.Fprintf(os.Stderr, "director: %v\n", cerr)
		printInteractionsOf(os.Stderr, art)
		return 1
	}
	rt := routes.Route{Slug: routes.Slug(*name)}
	if err := demonstrated.Save(routesHere(), rt, src, binds, time.Now()); err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	// AND THE PLAY ITSELF, PRINTED. An Act is legal Marco so that a person can read what
	// Marco will do; a command that saved one and showed a path would be hiding the one
	// thing this design exists for.
	fmt.Printf("Saved %q, composed from %q.\n\n", *name, art.Name)
	fmt.Printf("%s\n", src)
	fmt.Printf("  play       %s\n", routesHere().Path(rt))
	fmt.Printf("  bindings   %s\n", demonstrated.BindingsPath(routesHere(), rt))
	fmt.Printf("\n  Run it, one authorised press at a time:\n")
	fmt.Printf("    director perform-workflow --play %q --application %s --hotkeys\n",
		*name, orTheApplication(art))
	fmt.Printf("\n  Nothing is pressed without --live, and every press is asked about " +
		"either way.\n")
	return 0
}

// printInteractionsOf lists what a demonstration actually holds, by position.
func printInteractionsOf(w *os.File, art workflowstore.Artifact) {
	fmt.Fprintf(w, "\n  %q has %d interaction(s):\n", art.Name, len(art.Steps))
	for _, s := range art.Steps {
		what := s.Says
		if what == "" {
			what = fmt.Sprintf("press %s on %s", orSomething(s.Control),
				orSomething(s.From))
		}
		fmt.Fprintf(w, "    %d  %s\n", s.Position, what)
	}
}

func orSomething(s string) string {
	if s == "" {
		return "something"
	}
	return s
}

// orTheApplication is the scope to suggest, or a placeholder when the routine has none.
func orTheApplication(art workflowstore.Artifact) string {
	if art.Application == "" {
		return "<the application>"
	}
	return string(art.Application)
}
