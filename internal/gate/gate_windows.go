//go:build windows

package gate

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess          = kernel32.NewProc("OpenProcess")
	procQueryFullProcessName = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetProcessTimes      = kernel32.NewProc("GetProcessTimes")
	procTerminateProcess     = kernel32.NewProc("TerminateProcess")
)

const (
	queryLimitedInformation = 0x1000
	processTerminate        = 0x0001
)

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

// StartedAt is when a running process started, which is what makes its pid an identity.
//
// ASKED AT THE MOMENT THIS RUN STARTS SOMETHING, not when the answer is wanted: a pid that has
// exited cannot be identified at all afterwards, and the pid itself may by then belong to
// somebody else. A pid this account cannot open, or that is already gone, gets an error — and the
// caller's answer to that is to establish nothing and end nothing, never to fall back to the bare
// pid. See Identity.
func StartedAt(pid uint32) (time.Time, error) {
	if pid == 0 {
		return time.Time{}, fmt.Errorf("pid 0 is not a process")
	}
	_, started := imageAndStart(pid)
	if started.IsZero() {
		return time.Time{}, fmt.Errorf("pid %d could not be asked when it started, "+
			"so it cannot be identified", pid)
	}
	return started, nil
}

// End stops one process instance, and refuses to stop anything else.
//
// # Why the identity is checked on the handle
//
// Because the pid was read out of a snapshot, and a snapshot is a photograph. Between the
// photograph and this call the process can exit and Windows can hand its pid to something else —
// which, during a gate that spawns hundreds of processes, is not a remote possibility. A kill by
// pid alone would then end whatever now holds the number, and the report would say it ended the
// survivor it had written down.
//
// So the handle is opened ONCE, the start time is read from THAT handle, and the terminate is
// issued on THAT handle. A handle names a kernel object, not a number, so nothing can be swapped
// underneath it: either the object is the instance this run established, or nothing is done. A
// mismatch is an error and says what it found, because a survivor that got away is a finding too.
func End(id Identity) error {
	if !id.Known() {
		return fmt.Errorf("%s, so it was not ended", id)
	}
	h, _, err := procOpenProcess.Call(processTerminate|queryLimitedInformation, 0,
		uintptr(id.PID))
	if h == 0 {
		return fmt.Errorf("pid %d could not be opened, so it was not ended: %v",
			id.PID, err)
	}
	defer syscall.CloseHandle(syscall.Handle(h))

	var created, exited, kernel, user syscall.Filetime
	if r, _, err := procGetProcessTimes.Call(h,
		uintptr(unsafe.Pointer(&created)), uintptr(unsafe.Pointer(&exited)),
		uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user))); r == 0 {
		return fmt.Errorf("pid %d would not say when it started, so it was not "+
			"ended: %v", id.PID, err)
	}
	if at := time.Unix(0, created.Nanoseconds()); !at.Equal(id.Since) {
		return fmt.Errorf("pid %d is no longer the process this run established: it "+
			"started %s, and the one written down started %s, so the pid has "+
			"been reused and NOTHING HAS BEEN DONE TO IT",
			id.PID, at.Format("2006-01-02 15:04:05.000"),
			id.Since.Format("2006-01-02 15:04:05.000"))
	}
	if r, _, err := procTerminateProcess.Call(h, 1); r == 0 {
		return fmt.Errorf("ending %s: %v", id, err)
	}
	return nil
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
