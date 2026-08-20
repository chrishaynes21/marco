package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// `marco games` — what the Director understands about games, from the engine's own CLI.
//
// A thin client, like `marco director`: the capability packs live in the Director service,
// which is the process that observes the desktop and therefore the only one that can say
// what is in front of the user. This asks it.
//
// It exists so a front-end — the overlay, and through it voice — can answer "does Marco
// know this game?" without the user learning a second command name.

// runGames is `marco games [--json]`.
func runGames(args []string) {
	jsonOut := false
	for _, a := range args {
		if a == "--json" || a == "-json" {
			jsonOut = true
		}
	}

	client, err := directorClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	out, err := client.Game(service.GamePayload{Action: service.GameCapabilities})
	if err != nil {
		fmt.Fprintf(os.Stderr, "marco: %v\n", err)
		os.Exit(1)
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out.Report); err != nil {
			fmt.Fprintf(os.Stderr, "marco: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(out.Report.Packs) == 0 {
		fmt.Println("Marco has no game capability packs registered.")
		fmt.Println("The Director still works in any application; it just has no extra")
		fmt.Println("vocabulary for one.")
		return
	}
	fmt.Print(out.Report.Describe())
}

// directorClient connects to the running Director service.
//
// It does NOT start one. `marco games` is a question, and a question that launched a
// desktop-observing daemon as a side effect would be a surprising one.
func directorClient() (*service.Client, error) {
	ep, ok := service.ReadEndpoint(directorDir())
	if !ok {
		return nil, fmt.Errorf("the Director service is not running (start it with: director serve)")
	}
	return service.Dial(ep, 2*time.Second)
}
