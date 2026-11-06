package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// THE RAN LINE IS WHAT THE PERSON SEES IN THE WIDGET'S HISTORY: the routine and the values it was
// run with, in the routine's order, positional values on their names — and never a value for an
// argument the credential store answers, whatever was supplied for it.
func TestTheRanLineNamesTheValuesAndNoSecret(t *testing.T) {
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{App: "arkascended", Focus: true, Slug: "cloud-pull"}
	src := "use os.\n\n" +
		"do OS's Type with \"{{resource}} {{count}}\".\n" +
		"do OS's Secret with \"cloud-pull:pin\".\n"
	if err := reg.Save(rt, src); err != nil {
		t.Fatal(err)
	}
	d := newDeps()
	d.Reg = reg
	for _, c := range []struct {
		named      map[string]string
		positional []string
		want       string
	}{
		{map[string]string{"resource": "stone"}, nil, "cloud pull resource: stone"},
		{map[string]string{"Count": "3", "resource": "black pearl"}, nil, "cloud pull resource: black pearl count: 3"},
		{nil, []string{"metal", "2"}, "cloud pull resource: metal count: 2"},
		{map[string]string{"resource": "stone", "pin": "1234"}, nil, "cloud pull resource: stone"},
		{nil, nil, "cloud pull"},
	} {
		if got := ranLine(d, rt, c.named, c.positional); got != c.want {
			t.Errorf("ranLine(%v, %v) = %q, want %q", c.named, c.positional, got, c.want)
		}
	}
}
