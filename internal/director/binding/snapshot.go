package binding

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The durable form.
//
//	Binding metadata is audit and targeting information, not authorization.
//	Avoid persisting ephemeral pointers or implementation objects.
//
// A Store holds one MUTABLE binding per request. The action graph holds an IMMUTABLE
// Snapshot per node. They are different things on purpose: the store's copy is refreshed
// when the world moves under it, and the graph's copy is what was true when the action
// ran and must never change afterwards.
//
// The conversion is explicit in both directions, and lossy in one. A snapshot drops the
// raw stability token in favour of a DIGEST — the token is a summary of everything
// focusable at the time, which is both large and a description of the user's screen, and
// neither belongs in durable history. What a replay needs from it is only "is this still
// the same world?", which a digest answers exactly as well.

// Snapshot is a binding as durable history.
//
// Every field is identity or evidence, and there are no coordinates. The zero value means
// "this node was not produced from a bound target", which is what every node written
// before this existed decodes to.
type Snapshot struct {
	// Phrase is the originating deictic phrase ("this file").
	Phrase string `json:"phrase,omitempty"`
	// Expected and Resolved are the kind asked for and the kind found.
	Expected ObjectKind `json:"expected_kind,omitempty"`
	Resolved ObjectKind `json:"resolved_kind,omitempty"`

	// NativeID is the accessibility source's own name for the object — the identity
	// that survives a re-observation. ElementID is the world's, kept as evidence only.
	NativeID  string `json:"native_id,omitempty"`
	ElementID string `json:"element_id,omitempty"`
	// Resource is the backing file path, the identity that survives a rebuild.
	Resource string `json:"resource,omitempty"`

	Application string `json:"application,omitempty"`
	WindowID    string `json:"window_id,omitempty"`
	WindowTitle string `json:"window_title,omitempty"`
	Label       string `json:"label,omitempty"`

	// Evidence is why the object was chosen.
	Evidence []Evidence `json:"evidence,omitempty"`
	// StabilityDigest is a short, non-reversible summary of the stability token. See
	// the package comment for why the token itself is not stored.
	StabilityDigest string `json:"stability_digest,omitempty"`
	// Refreshed is the history of harmless changes revalidation absorbed.
	Refreshed []string `json:"refreshed,omitempty"`

	Confidence float64   `json:"confidence"`
	ResolvedAt time.Time `json:"resolved_at,omitempty"`
	Origin     Origin    `json:"origin,omitempty"`

	// Identity is the source.s account of the object, when one established it. Durable
	// and coordinate-free like the rest of this struct — a path, how it was obtained,
	// and why.
	Identity *directorapi.ResourceIdentity `json:"identity,omitempty"`
}

// Bound reports whether this snapshot describes a real binding.
func (s *Snapshot) Bound() bool { return s != nil && s.Resolved != "" }

// Identified reports whether the snapshot carries enough identity to be revalidated.
//
//	An old graph representing a deictic action without sufficient target identity must
//	be refused at execution rather than guessed.
//
// A resource or a native id can be looked for again. A LABEL cannot: two files in
// different folders share a name, and re-finding by caption is how a replay renames the
// wrong one. So a snapshot with only a label is bound and unidentified, and the replay
// path refuses it.
func (s *Snapshot) Identified() bool {
	return s.Bound() && (s.Resource != "" || s.NativeID != "")
}

// Describe renders a snapshot in one line.
func (s *Snapshot) Describe() string {
	if !s.Bound() {
		return "(unbound)"
	}
	out := fmt.Sprintf("%q → %s", s.Phrase, s.Resolved.Describe())
	switch {
	case s.Resource != "":
		out += " " + s.Resource
	case s.Label != "":
		out += " " + fmt.Sprintf("%q", s.Label)
	}
	return out
}

// Snapshot converts a live binding into durable history.
func (b *Binding) Snapshot() *Snapshot {
	if !b.Bound() {
		return nil
	}
	s := &Snapshot{
		Phrase: b.Phrase, Expected: b.Expected, Resolved: b.Resolved,
		NativeID: b.NativeID, ElementID: b.ElementID, Resource: b.Resource,
		Application: b.Application, WindowID: b.WindowID, WindowTitle: b.WindowTitle,
		Label: b.Label, Confidence: b.Confidence, ResolvedAt: b.ResolvedAt,
		Origin: b.Origin, StabilityDigest: Digest(b.Stability),
		Identity: b.Identity.Clone(),
	}
	// Copied, not aliased. A snapshot that shared the live binding's slices would change
	// when the binding was refreshed, which is the one thing durable history may not do.
	s.Evidence = append([]Evidence{}, b.Evidence...)
	s.Refreshed = append([]string{}, b.Refreshed...)
	return s
}

// Restore turns durable history back into a live binding for revalidation.
//
//	Replay uses the stored semantic target and binding identity.
//	Replay does not re-resolve the original deictic phrase.
//
// The restored binding carries the stored identity and NOT the stored stability token —
// the digest cannot be turned back into one, so a restored binding always looks like the
// world has moved and is always re-identified by resource or native id. That is the safe
// direction: a replay never gets to skip the check.
func (s *Snapshot) Restore() *Binding {
	if !s.Bound() {
		return nil
	}
	b := &Binding{
		Phrase: s.Phrase, Expected: s.Expected, Resolved: s.Resolved,
		NativeID: s.NativeID, ElementID: s.ElementID, Resource: s.Resource,
		Application: s.Application, WindowID: s.WindowID, WindowTitle: s.WindowTitle,
		Label: s.Label, Confidence: s.Confidence, ResolvedAt: s.ResolvedAt,
		Origin: s.Origin, Identity: s.Identity.Clone(),
	}
	b.Evidence = append([]Evidence{}, s.Evidence...)
	b.Refreshed = append([]string{}, s.Refreshed...)
	return b
}

// Digest summarises a stability token for storage and display.
//
// Empty in, empty out — so a binding made against a world that could not be summarised
// does not acquire a digest that suggests it was.
func Digest(token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])[:16]
}

// SameResource reports whether a snapshot and a live binding name the same backing
// object.
//
//	A replayed node whose current resource identity differs from the stored resource
//	must be refused before confirmation.
//
// Compared case-insensitively because Windows paths are, and answered as UNKNOWN
// (false, false) when either side has no resource — an absent path is not evidence of
// sameness, and treating it as such is how a replay acts on a lookalike.
func SameResource(s *Snapshot, b *Binding) (same bool, known bool) {
	if s == nil || b == nil || s.Resource == "" || b.Resource == "" {
		return false, false
	}
	return strings.EqualFold(s.Resource, b.Resource), true
}
