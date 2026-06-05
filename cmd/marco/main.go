package main

import (
	"fmt"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
)

func main() {
	if len(os.Args) < 3 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if err := driver.RunFile(os.Args[2], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "test":
		if err := driver.RunTests(os.Args[2], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "contracts":
		if err := driver.PrintContracts(os.Args[2], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "check":
		jsonMode := false
		path := os.Args[2]
		if path == "--json" {
			if len(os.Args) < 4 {
				usage()
				os.Exit(2)
			}
			jsonMode = true
			path = os.Args[3]
		}
		if err := driver.Check(path, os.Stdout, jsonMode); err != nil {
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: marco run <file.marco>")
	fmt.Fprintln(os.Stderr, "       marco test <file.marco>")
	fmt.Fprintln(os.Stderr, "       marco contracts <file.marco>")
	fmt.Fprintln(os.Stderr, "       marco check [--json] <file.marco>")
}
