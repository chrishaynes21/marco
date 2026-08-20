package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
)

// Reading back the object a request acted on.
//
//	Verification must establish that the observed result belongs to the same bound
//	object.
//
// The Director's ordinary verification compares world states, and a world state cannot
// answer "is the file that was at A now at B, unchanged?" — the accessibility tree reports
// what a window is showing, not what is on disk, and a list item reading "Budget.txt" is
// equally consistent with the wrong file having been renamed.
//
// So the whole-goal check reads the filesystem, and this is the only thing in the Director
// that does. It is deliberately READ-ONLY by type: there is no method here that writes,
// moves or deletes anything, so no amount of wiring can turn verification into an effect.

// digestLimit bounds how much of a file is hashed.
//
// A rename must preserve the content, and hashing the first megabyte proves that as well
// as hashing a gigabyte would for any file a person renames by voice — while keeping the
// check bounded on a video or a disk image, where reading the whole thing would stall the
// request it is meant to be verifying.
const digestLimit = 1 << 20

// osResources reads what is actually on disk.
type osResources struct{}

var _ verify.Inspector = osResources{}

// Inspect reports what is at a path.
//
// The second result distinguishes "there is nothing there" from "this could not be
// answered", and they are genuinely different: a missing file is evidence about a rename,
// and a permission error is the absence of evidence. Reporting the second as the first is
// how a verification passes because it could not look.
func (osResources) Inspect(path string) (verify.Identity, bool) {
	if path == "" {
		return verify.Identity{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return verify.Identity{Resource: path, Exists: false}, true
		}
		// Permission denied, a disconnected network share, a path that is not a path.
		// Unknown, which the correlation treats as a reason to refuse rather than pass.
		return verify.Identity{}, false
	}
	id := verify.Identity{Resource: path, Exists: true}
	if info.IsDir() {
		// A directory has no content digest, and hashing its listing would make the
		// check depend on what is inside a folder nobody asked about.
		return id, true
	}
	if d, ok := digestOf(path); ok {
		id.ContentDigest = d
	}
	return id, true
}

// digestOf hashes the first digestLimit bytes of a file.
//
// A failure is reported as no digest rather than as an error: the correlation already
// treats a missing digest as "a replacement cannot be ruled out", which is the honest
// reading of a file that could be seen and not read.
func digestOf(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, digestLimit)); err != nil {
		return "", false
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), true
}
