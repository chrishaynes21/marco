//go:build windows

package gate

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procQueryFullProcessName = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetProcessTimes      = kernel32.NewProc("GetProcessTimes")
)

const queryLimitedInformation = 0x1000

// Snapshot is every process this account can see, with its parent and its executable.
//
// Toolhelp for the list, because it is the one enumeration that answers the PARENT — which is how
// a survivor's ownership is decided and the reason a plain name scan would not do.
//
// The full path is asked for per process and is allowed to fail: a process this account may not
// open answers nothing, and nothing is the honest answer rather than a guess from its name. See
// Watching.has, which refuses to match a process whose path could not be read.
func Snapshot() ([]Process, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snap)

	var e syscall.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []Process
	for err = syscall.Process32First(snap, &e); err == nil; err = syscall.Process32Next(snap, &e) {
		p := Process{
			PID:    e.ProcessID,
			Parent: e.ParentProcessID,
			Name:   lower(syscall.UTF16ToString(e.ExeFile[:])),
		}
		p.Path, p.Since = imageAndStart(e.ProcessID)
		out = append(out, p)
	}
	return out, nil
}

// imageAndStart is a process's executable path and when it started, or zero values.
func imageAndStart(pid uint32) (string, time.Time) {
	if pid == 0 {
		return "", time.Time{}
	}
	h, _, _ := procOpenProcess.Call(queryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return "", time.Time{}
	}
	defer syscall.CloseHandle(syscall.Handle(h))

	path := ""
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	if r, _, _ := procQueryFullProcessName.Call(h, 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 &&
		size > 0 {
		path = syscall.UTF16ToString(buf[:size])
	}

	var created, exited, kernel, user syscall.Filetime
	started := time.Time{}
	if r, _, _ := procGetProcessTimes.Call(h,
		uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r != 0 {
		started = time.Unix(0, created.Nanoseconds())
	}
	return path, started
}

func lower(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}
