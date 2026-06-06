//go:build windows

package secrets

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

func New() Store { return winStore{} }

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procCredWriteW     = advapi32.NewProc("CredWriteW")
	procCredReadW      = advapi32.NewProc("CredReadW")
	procCredDeleteW    = advapi32.NewProc("CredDeleteW")
	procCredEnumerateW = advapi32.NewProc("CredEnumerateW")
	procCredFree       = advapi32.NewProc("CredFree")
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
)

// credentialW mirrors the Win32 CREDENTIALW struct.
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        struct{ Low, High uint32 } // FILETIME
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type winStore struct{}

func target(name string) string { return namespace + name }

func (winStore) Set(name, value string) error {
	tgt, err := syscall.UTF16PtrFromString(target(name))
	if err != nil {
		return err
	}
	blob := []byte(value)
	var blobPtr *byte
	if len(blob) > 0 {
		blobPtr = &blob[0]
	}
	user, _ := syscall.UTF16PtrFromString(name)
	cred := credentialW{
		Type:               credTypeGeneric,
		TargetName:         tgt,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     blobPtr,
		Persist:            credPersistLocalMachine,
		UserName:           user,
	}
	r, _, e := procCredWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r == 0 {
		return fmt.Errorf("CredWrite %q failed: %v", name, e)
	}
	return nil
}

func (winStore) Get(name string) (string, bool, error) {
	tgt, err := syscall.UTF16PtrFromString(target(name))
	if err != nil {
		return "", false, err
	}
	var pcred *credentialW
	r, _, e := procCredReadW.Call(uintptr(unsafe.Pointer(tgt)), credTypeGeneric, 0,
		uintptr(unsafe.Pointer(&pcred)))
	if r == 0 {
		// ERROR_NOT_FOUND (1168) → not found, not an error.
		if en, ok := e.(syscall.Errno); ok && en == 1168 {
			return "", false, nil
		}
		return "", false, fmt.Errorf("CredRead %q failed: %v", name, e)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(pcred)))
	if pcred.CredentialBlobSize == 0 || pcred.CredentialBlob == nil {
		return "", true, nil
	}
	b := unsafe.Slice(pcred.CredentialBlob, pcred.CredentialBlobSize)
	return string(b), true, nil
}

func (winStore) Delete(name string) error {
	tgt, err := syscall.UTF16PtrFromString(target(name))
	if err != nil {
		return err
	}
	r, _, e := procCredDeleteW.Call(uintptr(unsafe.Pointer(tgt)), credTypeGeneric, 0)
	if r == 0 {
		if en, ok := e.(syscall.Errno); ok && en == 1168 { // not found
			return nil
		}
		return fmt.Errorf("CredDelete %q failed: %v", name, e)
	}
	return nil
}

func (winStore) List() ([]string, error) {
	filter, _ := syscall.UTF16PtrFromString(namespace + "*")
	var count uint32
	var creds **credentialW
	r, _, e := procCredEnumerateW.Call(uintptr(unsafe.Pointer(filter)), 0,
		uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&creds)))
	if r == 0 {
		if en, ok := e.(syscall.Errno); ok && en == 1168 { // none found
			return nil, nil
		}
		return nil, fmt.Errorf("CredEnumerate failed: %v", e)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(creds)))
	arr := unsafe.Slice(creds, count)
	var out []string
	for _, c := range arr {
		name := utf16PtrToString(c.TargetName)
		out = append(out, strings.TrimPrefix(name, namespace))
	}
	return out, nil
}

func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var u []uint16
	for ptr := unsafe.Pointer(p); ; ptr = unsafe.Add(ptr, 2) {
		c := *(*uint16)(ptr)
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return syscall.UTF16ToString(u)
}
