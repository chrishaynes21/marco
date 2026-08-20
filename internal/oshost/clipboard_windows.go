//go:build windows

package oshost

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// Clipboard access, added to Marco because the Director needs an editing primitive
// rather than a keystroke sequence.
//
// The rule this exists to satisfy: if Marco lacks an editing primitive, add it to
// Marco rather than expanding the Director into a keyboard macro engine. Reading and
// writing the clipboard is an OS capability, so it belongs beside the other OS acts —
// not reimplemented in the planner as Ctrl+C and a guess.
//
// stdlib only. user32 and kernel32 through syscall, like the rest of this package.
//
// go vet reports "possible misuse of unsafe.Pointer" on the two uintptr→pointer
// conversions below, as it does for internal/recorder. The conversions are correct
// here for a reason vet cannot see: the memory came from GlobalAlloc, so it lives in
// the OS heap rather than the Go heap, and GlobalLock pins it for exactly the span in
// which it is dereferenced. Go's collector neither moves nor frees it. This is the
// standard Win32 clipboard idiom and there is no safe-pointer alternative — the API
// hands back a HANDLE, not a Go value.

// The DLL handles are already declared in backend_windows.go — reused rather than
// re-opened, so both files talk to the same loaded library.
var (
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	// CountClipboardFormats is what separates an EMPTY clipboard from one holding
	// something that is not text. Both read as ("", false) through the text format
	// alone, and the difference decides whether the clipboard can be borrowed safely.
	procCountClipboardFormats = user32.NewProc("CountClipboardFormats")

	procGlobalAlloc  = kernel32.NewProc("GlobalAlloc")
	procGlobalFree   = kernel32.NewProc("GlobalFree")
	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

// clipboardText reads the clipboard's text, if it holds any.
//
// An empty string with no error means the clipboard holds something that is not text —
// an image, a file list. That is a real state and NOT an error: a caller saving the
// clipboard to restore it later needs to know the difference between "empty" and
// "could not read", because restoring an empty string over an image would destroy it.
func clipboardText() (text string, isText bool, empty bool, err error) {
	if err := openClipboard(); err != nil {
		return "", false, false, err
	}
	defer procCloseClipboard.Call()

	n, _, _ := procCountClipboardFormats.Call()
	if n == 0 {
		return "", false, true, nil // genuinely empty
	}

	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", false, false, nil // holds something, and it is not text
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return "", false, false, fmt.Errorf("clipboard: could not lock the clipboard memory")
	}
	defer procGlobalUnlock.Call(h)

	return utf16PtrToString(p), true, false, nil
}

// setClipboardText replaces the clipboard's contents with text.
func setClipboardText(s string) error {
	utf16, err := syscall.UTF16FromString(s)
	if err != nil {
		return fmt.Errorf("clipboard: the text cannot be represented: %w", err)
	}
	size := uintptr(len(utf16) * 2)

	if err := openClipboard(); err != nil {
		return err
	}
	defer procCloseClipboard.Call()

	if r, _, e := procEmptyClipboard.Call(); r == 0 {
		return fmt.Errorf("clipboard: could not empty it: %v", e)
	}

	h, _, e := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("clipboard: could not allocate %d bytes: %v", size, e)
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("clipboard: could not lock the new buffer")
	}
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(p)), len(utf16))
	copy(dst, utf16)
	procGlobalUnlock.Call(h)

	if r, _, e := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		// Ownership only transfers on success. Freeing on failure is what keeps a
		// failed clipboard write from leaking a buffer on every retry.
		procGlobalFree.Call(h)
		return fmt.Errorf("clipboard: could not set the data: %v", e)
	}
	return nil
}

// openClipboard takes the clipboard with a few retries.
//
// The clipboard is a single global resource and another process may hold it for a few
// milliseconds at a time — a browser finishing a copy, an editor updating its own view.
// Failing on the first refusal would make clipboard operations flaky for no reason;
// retrying forever would hang. A handful of attempts is the honest middle.
func openClipboard() error {
	var lastErr error
	for i := 0; i < 8; i++ {
		if r, _, e := procOpenClipboard.Call(0); r != 0 {
			return nil
		} else {
			lastErr = e
		}
		sleepMs(20)
	}
	return fmt.Errorf("clipboard: another process is holding it: %v", lastErr)
}

func utf16PtrToString(p uintptr) string {
	// Bounded scan: a clipboard entry is a NUL-terminated UTF-16 string, and a missing
	// terminator would otherwise walk out of the mapping.
	const maxChars = 1 << 22 // 4M characters
	buf := unsafe.Slice((*uint16)(unsafe.Pointer(p)), maxChars)
	for i := 0; i < maxChars; i++ {
		if buf[i] == 0 {
			return syscall.UTF16ToString(buf[:i])
		}
	}
	return syscall.UTF16ToString(buf[:maxChars])
}

func sleepMs(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
