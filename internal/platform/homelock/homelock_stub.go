//go:build !windows

package homelock

// The stub, and it ENFORCES NOTHING. That is stated rather than hidden.
//
// Every claim succeeds. A build on a platform with no backend can therefore run two Directors
// against one home, and this file is the record of that gap rather than a pretence that it is
// closed.
//
// The alternative — refusing to claim — would be worse: it would make the Director unstartable
// on a platform where it otherwise works, to protect against a race that platform's users would
// have to create deliberately. And a file-based stand-in would be the very thing the package doc
// rejects, with the added harm of looking authoritative.
//
// The real backend is Windows, which is where Marco drives a desktop. When another platform grows
// one, it needs a primitive with the same property: released by the operating system when the
// process ends, however it ends.

// Unix-like filesystems are case-sensitive, so two spellings that differ only in case are two
// different directories and must be two different homes. Folding them would merge them.
const caseInsensitiveFilesystem = false

type openClaim struct{ name string }

func (c *openClaim) Name() string { return c.name }
func (c *openClaim) Release()     {}

func claim(name string) (Claim, error) { return &openClaim{name: name}, nil }
