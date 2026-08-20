package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The game diagnostics.
//
//	director game               what is detected, and what the pack permits
//	director capabilities       what each registered pack contributes
//	director explain game       why this pack was chosen and the others were not
//	director explain inventory  what the Director can see of what the player holds
//
// All four go through the daemon, because detection is a property of what the daemon is
// LOOKING AT: it is recomputed on every observation, and a client that answered from its
// own registry would report which packs exist rather than which one is serving the window
// in front of the user.
//
// None of them touches the desktop, and none takes the command lock — so `director game`
// answers while a command is running, which is when someone asks.

// runGame is `director game`.
func runGame(args []string) int {
	fs := flag.NewFlagSet("game", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.Game(service.GamePayload{Action: service.GameDetected})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(out)
	}
	fmt.Print(out.Active.Describe())
	if !out.Active.Detected() && out.Packs == 0 {
		fmt.Println("\nNo capability packs are registered in this build.")
	}
	return 0
}

// runCapabilities is `director capabilities`.
func runCapabilities(args []string) int {
	fs := flag.NewFlagSet("capabilities", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.Game(service.GamePayload{Action: service.GameCapabilities})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(out.Report)
	}
	fmt.Print(out.Report.Describe())
	return 0
}

// runExplainGame is `director explain game`.
//
// The same detection `director game` shows, plus every pack that was ASKED and what it
// said — which is the difference between "no game detected" and an answer to "why not?".
func runExplainGame(args []string) int {
	fs := flag.NewFlagSet("explain game", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.Game(service.GamePayload{Action: service.GameDetected})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(out.Active)
	}
	fmt.Print(out.Active.Describe())
	if len(out.Active.Considered) > 0 && out.Active.Detected() {
		fmt.Println("\nConsidered")
		for _, c := range out.Active.Considered {
			mark := " "
			if c.Pack == out.Active.Pack {
				mark = "*"
			}
			fmt.Printf("  %s %-16s %.0f%%\n", mark, c.Pack, c.Confidence*100)
			for _, e := range c.Evidence {
				fmt.Printf("      %s\n", e)
			}
		}
	}
	return 0
}

// runExplainInventory is `director explain inventory`.
func runExplainInventory(args []string) int {
	fs := flag.NewFlagSet("explain inventory", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "print as JSON")
	_ = fs.Parse(flagsFirst(args))

	container := strings.TrimSpace(strings.Join(fs.Args(), " "))

	client, err := connect(false)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer client.Close()

	out, err := client.Game(service.GamePayload{
		Action: service.GameInventory, Container: container,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}
	if *jsonOut {
		return printJSON(out.Inventory)
	}
	fmt.Print(out.Inventory.Describe())
	return 0
}
