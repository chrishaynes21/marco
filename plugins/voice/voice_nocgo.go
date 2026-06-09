//go:build !cgo

package main

import "errors"

// runVosk is unavailable without cgo. Build with CGO_ENABLED=1 and libvosk on the
// link path (see README); `--demo` still works for pipe testing.
func runVosk(_ func(ev, data string), _ string) error {
	return errors.New("built without cgo — rebuild with CGO_ENABLED=1 + libvosk, or run with --demo (see README)")
}
