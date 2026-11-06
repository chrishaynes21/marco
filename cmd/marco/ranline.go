package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"github.com/chaynes-simpleclouds/marco/internal/plays"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// ranLine is what a routine was run with, as a person reads it: its name, then each value it was
// given as `name: value`, in the routine's own order (positional values land on the names in the
// order the intake's grammar puts them). An argument the credential store answers is never
// written, whatever was supplied for it.
//
// Writing a secret argument's value must fail TestTheRanLineNamesTheValuesAndNoSecret.
func ranLine(d orchestrator.Deps, rt routes.Route, named map[string]string, positional []string) string {
	line := plays.Pretty(rt.Slug)
	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return line
	}
	values := map[string]string{}
	for k, v := range named {
		values[strings.ToLower(k)] = v
	}
	for i, name := range routes.ArgNames(string(src)) {
		if i < len(positional) {
			if _, given := values[strings.ToLower(name)]; !given {
				values[strings.ToLower(name)] = positional[i]
			}
		}
	}
	for _, a := range routes.Arguments(string(src)) {
		v, ok := values[strings.ToLower(a.Name)]
		if !ok || a.Secret || strings.TrimSpace(v) == "" {
			continue
		}
		line += " " + a.Name + ": " + v
	}
	return line
}

// sayRan announces what a routine is about to run with on `[ran] ` (outcome.RanPrefix), for a
// front end to show the person. Called once nothing is left that could refuse the run.
func sayRan(d orchestrator.Deps, rt routes.Route, named map[string]string, positional []string) {
	fmt.Printf("%s%s\n", outcome.RanPrefix, ranLine(d, rt, named, positional))
}
