package directorapi

import (
	"fmt"
	"strings"
)

// The object BEHIND a control.
//
//	A selected filesystem item in Windows Explorer exposes a canonical backing path or
//	an equivalent stable shell identity.
//
// Most controls have nothing behind them. A button is a button; there is no file, no
// document and no record it stands for, and the honest representation of that is an absent
// resource rather than an empty one.
//
// Some do. A File Explorer list item stands for a file on disk, and until this existed the
// Director could see only its CAPTION — "Alpha.txt" — which is not identity: two folders
// hold files with that name, a hidden extension makes the caption differ from the name, and
// "rename this file" aimed at a caption renames whichever thing happens to be captioned so.
// The binding layer refused it, correctly, and the live rename scenario stopped there.
//
// So a source that genuinely KNOWS what a control stands for says so here, and one that
// does not leaves the field absent. There is deliberately no way to express "probably a
// file called Alpha.txt": a resource is established or it is not.
//
// # What may not go in here
//
// No coordinates, no handles, no COM pointers, no PIDLs, no process-local identifiers of
// any kind. Everything here is durable enough to be written to the action graph and acted
// on by a later process — that is the whole point of it — and a field that meant something
// only inside one observation would be a trap for exactly the code that trusts this most.
type ResourceIdentity struct {
	// Kind is what sort of thing it is: "file", "folder". A CLOSED vocabulary at the
	// point of use — see binding.ObjectKind, which refuses anything it cannot check.
	Kind string `json:"kind"`
	// Path is the canonical filesystem path. The field that makes "this file" checkable
	// rather than hopeful, and the reason everything else here is corroboration.
	Path string `json:"path,omitempty"`
	// ParsingName is the source's own round-trippable name for the object. Equal to Path
	// for a filesystem item; recorded as evidence for anything else, never acted on.
	ParsingName string `json:"parsing_name,omitempty"`
	// DisplayName is what the source calls it, which is what a caption should match.
	// Kept so a correlation failure can name the two strings that disagreed.
	DisplayName string `json:"display_name,omitempty"`
	// Source names how this was established ("shell_folder_view"), so a reader can weigh
	// it and a future second mechanism can be told apart from this one.
	Source string `json:"source,omitempty"`
	// Confidence is the source's certainty, 0..1.
	Confidence float64 `json:"confidence,omitempty"`
	// Link marks a shortcut. Reported and deliberately not followed: binding to what a
	// shortcut points at would act on a file in another folder that the user is not
	// looking at.
	Link bool `json:"link,omitempty"`
	// Evidence explains how the identity was obtained, one clause each.
	Evidence []string `json:"evidence,omitempty"`
}

// Resource kinds. Strings rather than a Go enum for the same reason the rest of the
// Director's vocabularies are: they cross a process boundary as JSON.
const (
	ResourceFile   = "file"
	ResourceFolder = "folder"
)

// Known reports whether this is a resource anything can act on.
//
// A path AND a kind. Either alone is not a resource: a kind with no path names nothing,
// and a path with no kind cannot answer "is this a file?" — which is the question the
// binding layer exists to ask.
func (r *ResourceIdentity) Known() bool {
	if r == nil {
		return false
	}
	if strings.TrimSpace(r.Path) == "" {
		return false
	}
	return r.Kind == ResourceFile || r.Kind == ResourceFolder
}

// IsFile reports whether this names a file rather than a folder.
func (r *ResourceIdentity) IsFile() bool { return r.Known() && r.Kind == ResourceFile }

// IsFolder reports whether this names a folder.
func (r *ResourceIdentity) IsFolder() bool { return r.Known() && r.Kind == ResourceFolder }

// Describe renders a resource in one line.
func (r *ResourceIdentity) Describe() string {
	if !r.Known() {
		return "(no backing resource)"
	}
	out := fmt.Sprintf("%s %s", r.Kind, r.Path)
	if r.Link {
		out += " (a shortcut; the identity is the shortcut itself)"
	}
	if r.Source != "" {
		out += " via " + r.Source
	}
	return out
}

// Clone returns a copy that shares no slice with the original.
//
// A resource travels from an observation into a fused element, into a binding and into the
// action graph. Sharing the evidence slice across those would let a later append change
// what history says was known at the time.
func (r *ResourceIdentity) Clone() *ResourceIdentity {
	if r == nil {
		return nil
	}
	out := *r
	out.Evidence = append([]string{}, r.Evidence...)
	return &out
}
