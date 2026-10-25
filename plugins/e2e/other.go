//go:build !windows

package main

// THE HARNESS DOES NOT RUN HERE, AND SAYS SO AS NOT RUN.
//
// Not a build failure and not a pass. Everything this drives — the input hooks it must not
// trigger, the fixture it points Marco at, the UI Automation tree it reads Studio through — is
// Windows, and a harness that could not be compiled elsewhere would make `go build ./...` on
// another machine fail for a reason that has nothing to do with the change somebody made.
//
// Exit 2 is the same code an unavailable desktop gets, because it is the same kind of answer:
// nothing was established, and the reason is a fact about the machine.

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	fmt.Printf("NOT RUN: the desktop harness drives Windows windows, and this is %s. "+
		"Nothing was established.\n", runtime.GOOS)
	os.Exit(2)
}
