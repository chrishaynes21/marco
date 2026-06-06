//go:build !windows

package main

// readPassword has no no-echo console reader on non-Windows yet, so it returns
// handled=false and the caller does a plain line read (piping stdin avoids the
// echo). macOS/Linux no-echo via termios is a small follow-on.
func readPassword(string) (string, bool, error) { return "", false, nil }
