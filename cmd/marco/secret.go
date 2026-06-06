package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/secrets"
)

// runSecret handles `marco secret set|get-status|list|rm`. Values are stored in
// the OS credential manager; routes only ever reference a secret by name.
func runSecret(args []string) {
	store := secrets.New()
	if len(args) == 0 {
		secretUsage()
		os.Exit(2)
	}
	switch args[0] {
	case "set":
		if len(args) < 2 {
			secretUsage()
			os.Exit(2)
		}
		name := args[1]
		val, err := readSecret(fmt.Sprintf("Value for %q: ", name))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if val == "" {
			fmt.Fprintln(os.Stderr, "empty value — not stored")
			os.Exit(1)
		}
		if err := store.Set(name, val); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Stored secret %q.\n", name)
	case "list":
		names, err := store.List()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, n := range names {
			fmt.Println("  " + n)
		}
	case "rm", "delete":
		if len(args) < 2 {
			secretUsage()
			os.Exit(2)
		}
		if err := store.Delete(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("Removed secret %q.\n", args[1])
	default:
		secretUsage()
		os.Exit(2)
	}
}

func secretUsage() {
	fmt.Fprintln(os.Stderr, "usage: marco secret set <name>   store a password (stdin, hidden where supported)")
	fmt.Fprintln(os.Stderr, "       marco secret list")
	fmt.Fprintln(os.Stderr, "       marco secret rm <name>")
}

// readSecret reads a secret value: from a no-echo console where supported (see
// readPassword), else a plain line. Piping stdin also works (and never echoes).
func readSecret(prompt string) (string, error) {
	if v, ok, err := readPassword(prompt); ok || err != nil {
		return v, err
	}
	fmt.Fprint(os.Stderr, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
