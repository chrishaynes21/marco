//go:build !windows

package main

import "fmt"

// runDiag is Windows-only (it inspects DPI awareness and the cursor); elsewhere it
// just says so.
func runDiag([]string) {
	fmt.Println("marco diag is only implemented on Windows.")
}
