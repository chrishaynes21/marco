//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle  = kernel32.NewProc("GetStdHandle")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	stdInputHandle    = uintptr(0xFFFFFFF6) // -10 as DWORD
	enableEchoInput   = 0x0004
)

// readPassword reads a line from the console with echo disabled. handled is
// false when stdin isn't an interactive console (piped/redirected), so the
// caller falls back to a plain read.
func readPassword(prompt string) (value string, handled bool, err error) {
	h, _, _ := procGetStdHandle.Call(stdInputHandle)
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return "", false, nil // not a console — let caller do a plain read
	}
	fmt.Fprint(os.Stderr, prompt)
	procSetConsoleMode.Call(h, uintptr(mode&^enableEchoInput))
	line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
	procSetConsoleMode.Call(h, uintptr(mode)) // restore echo
	fmt.Fprintln(os.Stderr)                   // echo the swallowed newline
	return strings.TrimRight(line, "\r\n"), true, rerr
}
