package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Where a play came from, kept beside it and never inside it.
//
// # Why a sidecar
//
// Because a `.marco` file is a play somebody reads, and provenance is backstage. A route that
// carried candidate ids and evidence digests in its comments would be less readable for the sake
// of an audit almost nobody performs — and the comments would rot the moment the file was edited.
//
// The convention already exists: a taught route keeps its recording at `<slug>.rec.json` beside
// the source, and Delete and Rename already carry it along. This is the same shape for the same
// reason.
//
// # Fail-closed, in one direction
//
// A `.marco` with no sidecar is an ORDINARY authored play. A sidecar with no `.marco` is
// meaningless and is refused. A sidecar whose digest disagrees with the source is a sidecar
// describing a play that no longer exists, and reading it says so.
//
// That direction is deliberate. A crash between the two writes leaves an untrusted ordinary route,
// never a trusted learned one — which is the only asymmetry that is safe.
//
// See [[ADR-028-a-learned-play-is-a-file-with-a-past]].

// originExt is the suffix for a play's provenance, beside its source.
const originExt = ".origin.json"

// OriginVersion is the sidecar's format version. Present so an unreadable future format is
// refused rather than half-understood.
const OriginVersion = 1

// Kind is where a play came from.
//
// Three words, and the whole point of the type is that they are not interchangeable. An authored
// play is somebody's writing; a taught one is a recording turned into source; a learned one is
// something Director watched, rehearsed and verified. Only the last has provenance worth auditing,
// and only the last can lose it.
type Kind string

const (
	// KindAuthored is a play a person wrote. It has no sidecar, and needs none.
	KindAuthored Kind = "authored"
	// KindTaught is a play generated from a recorded demonstration.
	KindTaught Kind = "taught"
	// KindLearned is a play Director watched, rehearsed, verified and wrote down.
	KindLearned Kind = "learned"
)

// Origin is the minimum durable provenance for one saved play.
//
// # What is here, and why each field earns its place
//
// Only what a lifecycle question actually needs: what kind of play this is, which application it
// was learned against, which durable relationship produced it, which demonstration of that
// relationship, which verified evidence authorized it, and whether the source still matches.
//
// # What is deliberately absent
//
// Session ids, proposal ids, screen states, shadow tracks, subject fingerprints, window
// generations, process ids, confidence, and every other ephemeral identity. None of them survives
// a restart, so none of them can be part of how a play is found again — and a field that exists
// gets written.
type Origin struct {
	Version int  `json:"version"`
	Kind    Kind `json:"kind"`
	// Application is the program this was learned against, empty for an app-less play.
	Application string `json:"application,omitempty"`
	// From and To are the DURABLE remembered subjects the route runs between.
	//
	// Durable identities, not session ordinals: the same ids a fresh Director resolves from
	// semantic memory, which is what makes an audit possible after a restart.
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// Sequence is which demonstration of that route was rehearsed.
	Sequence int `json:"sequence,omitempty"`
	// Evidence is the rehearsal digest the attempt was authorized against.
	//
	// Provenance, not a score. It answers "which version of the demonstration is this play a
	// recording of", and nothing else.
	Evidence string `json:"evidence,omitempty"`
	// Source is the digest of the `.marco` this describes, at the moment it was written.
	//
	// INTEGRITY, not confidence. It is the difference between "this is the play Director
	// verified" and "this is a file that happens to sit where that play was" — and a user is
	// free to edit the play, which is exactly the case this field exists to notice.
	Source string `json:"source"`
	// SavedAt is when it was written.
	SavedAt string `json:"saved_at,omitempty"`
}

// DigestOf is the content digest of a play's source.
//
// Short because it is an equality check between two things this repository wrote, not a defence
// against anybody. Sixteen hex characters is the same width every other digest here uses.
func DigestOf(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])[:16]
}

// OriginPath is where a play's provenance lives.
func (r Registry) OriginPath(rt Route) string {
	return filepath.Join(r.locDir(rt), rt.Slug+originExt)
}

// SaveWithOrigin writes a play and its provenance, source first.
//
// # The two-file problem, and the direction it fails in
//
// There is no filesystem primitive that writes two files atomically, so one of them lands first
// and a crash between them is possible. The order is chosen so the survivable state is the SAFE
// one: source without provenance reads as an ordinary authored play, which claims nothing.
//
// The reverse order would leave provenance describing a file that does not exist — and worse, a
// later unrelated file saved under that slug would inherit it. `Origin` refuses that case
// separately, but the write order means it is not reachable by a crash in the first place.
func (r Registry) SaveWithOrigin(rt Route, source string, o Origin) error {
	if rt.Slug == "" {
		return fmt.Errorf("routes: a play needs a name")
	}
	o.Version = OriginVersion
	o.Source = DigestOf(source)
	if err := r.Save(rt, source); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(r.OriginPath(rt), append(raw, '\n')); err != nil {
		// The source is on disk without provenance. That is an ordinary authored play, which
		// is the honest reading — and the error says so rather than leaving a caller to
		// assume the pair landed.
		return fmt.Errorf("routes: %s was saved but its provenance was not: %w", rt.Slug, err)
	}
	return nil
}

// writeAtomic writes a file through a temporary name in the same directory.
//
// The same pattern semantic memory uses: a reader never sees a half-written file, and a crash
// leaves either the old bytes or the new ones.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".origin-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// OriginState is what reading a play's provenance found.
type OriginState string

const (
	// OriginNone is a play with no sidecar: an ordinary authored play.
	OriginNone OriginState = "authored"
	// OriginIntact is provenance whose digest matches the source it sits beside.
	OriginIntact OriginState = "intact"
	// OriginEdited is provenance for a source that has since been changed.
	//
	// NOT an error and NOT a corruption. The user was invited to edit the play, and they did.
	// What it means is that the file is no longer the artifact Director verified — for trust
	// purposes it is now an ordinary authored play that happens to remember where it started.
	OriginEdited OriginState = "edited"
	// OriginOrphaned is provenance with no source beside it. Refused: a later unrelated file
	// under the same slug must never inherit it.
	OriginOrphaned OriginState = "orphaned"
	// OriginUnreadable is a sidecar this version cannot understand.
	OriginUnreadable OriginState = "unreadable"
)

// Verified reports provenance that still describes the file it sits beside.
//
// The only state in which a play may be called learned. Everything else is an ordinary play, which
// is not a failure — it is what a readable, editable file is allowed to become.
func (s OriginState) Verified() bool { return s == OriginIntact }

// Origin reads a play's provenance and says whether it still describes the source.
//
// Recomputed on every read rather than trusted: the same discipline this repository applies to
// every other judgement, and the reason a user may edit a learned play without anybody having to
// remember to invalidate anything.
func (r Registry) Origin(rt Route) (Origin, OriginState) {
	raw, err := os.ReadFile(r.OriginPath(rt))
	if err != nil {
		return Origin{}, OriginNone
	}
	var o Origin
	if err := json.Unmarshal(raw, &o); err != nil || o.Version != OriginVersion {
		return Origin{}, OriginUnreadable
	}
	source, err := os.ReadFile(r.Path(rt))
	if err != nil {
		// Provenance describing nothing. Refused rather than carried, because the next file
		// written under this slug would otherwise inherit a past it never had.
		return o, OriginOrphaned
	}
	if DigestOf(string(source)) != o.Source {
		return o, OriginEdited
	}
	return o, OriginIntact
}

// MoveOrigin carries a play's provenance to another scope, byte for byte.
//
// # Why verbatim, and why not SaveWithOrigin
//
// Because `SaveWithOrigin` recomputes the digest from the source it is given, which is exactly
// right when it is WRITING the play Director verified — and exactly wrong here. Moving a play
// between scopes does not change its bytes, so re-digesting it would recompute a match and quietly
// re-verify a play whose file the person had edited: `edited` would become `intact`, and a surface
// that had honestly said "you have changed this since Marco wrote it down" would go back to
// calling it the artifact Director verified.
//
// So the sidecar is copied, not rebuilt. An unreadable one stays unreadable for the same reason:
// its state is a fact about this play, not damage to be tidied away by a move.
//
// A play with no sidecar is an ordinary authored one; there is nothing to carry and that is not an
// error. Called while the source `.marco` is still in place, so `locDir`'s legacy fallback still
// resolves the old location.
//
// Deleting this — or moving a play with SaveWithOrigin instead — must fail
// TestChangingScopeDoesNotReVerifyAnEditedPlay.
func (r Registry) MoveOrigin(from, to Route) error {
	raw, err := os.ReadFile(r.OriginPath(from))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := writeAtomic(r.OriginPath(to), raw); err != nil {
		return err
	}
	if err := os.Remove(r.OriginPath(from)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("routes: %s kept its past at the old location too: %w", from.Slug, err)
	}
	return nil
}

// DeleteOrigin removes a play's provenance, if it has any.
func (r Registry) DeleteOrigin(rt Route) error {
	err := os.Remove(r.OriginPath(rt))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// LearnedDir is where a saved-but-unregistered learned play waits.
//
// # Why saved and registered are different places rather than a flag
//
// Route discovery is a directory scan: `global/`, an app's loose files, `context/` and `focus/`.
// Nothing else is looked at. So a play in `<app>/learned/` is on disk, readable, editable and
// auditable — and structurally invisible to the resolver.
//
// That makes "saved" and "registered" impossible to confuse, because they are not two values of a
// field somebody could forget to check. Registering is moving the file somewhere the resolver
// looks, and a `"registered": true` that disagreed with the filesystem could not happen.
const LearnedDir = "learned"

// LearnedFocus is the scope a staged play registers into.
//
// NOT a fact on disk, and saying so is the point: `stagedDir` puts every staged play in
// `<app>/learned/` whatever `Route.Focus` says, so the bit cannot be recovered by reading the
// filesystem. It is a decision, taken once — a learned play is asked for from anywhere and brings
// its own application forward. See [[ADR-080-a-learned-play-is-asked-for-from-anywhere]].
//
// It is a constant so that the code which STAGES a play and the code which LISTS one read the
// same fact. A listing that decided it independently could promise an activation that
// registration would not perform.
//
// Deleting this and hard-coding the value in either place must fail
// TestAStagedPlayListsAsBringingItsApplicationForward.
const LearnedFocus = true

// StagedPath is where a saved-but-unregistered play lives.
func (r Registry) StagedPath(rt Route) string {
	return filepath.Join(r.stagedDir(rt), rt.Slug+".marco")
}

func (r Registry) stagedDir(rt Route) string {
	if rt.App == "" {
		return filepath.Join(r.Dir, GlobalDir, LearnedDir)
	}
	return filepath.Join(r.Dir, rt.App, LearnedDir)
}

// SaveStaged writes a learned play and its provenance where the resolver cannot see them.
//
// The first durable step, and deliberately not the last. A play here is a file somebody can read,
// edit and delete; it is not something a later request can find.
func (r Registry) SaveStaged(rt Route, source string, o Origin) error {
	if rt.Slug == "" {
		return fmt.Errorf("routes: a play needs a name")
	}
	o.Version, o.Source = OriginVersion, DigestOf(source)
	if err := os.MkdirAll(r.stagedDir(rt), 0o755); err != nil {
		return err
	}
	if err := writeAtomic(r.StagedPath(rt), []byte(source)); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(r.stagedOriginPath(rt), append(raw, '\n')); err != nil {
		// Source on disk without provenance. That reads as an ordinary authored play, which
		// is the honest answer — and the error says so rather than letting a caller assume
		// the pair landed.
		return fmt.Errorf("routes: %s was saved but its provenance was not: %w", rt.Slug, err)
	}
	return nil
}

func (r Registry) stagedOriginPath(rt Route) string {
	return filepath.Join(r.stagedDir(rt), rt.Slug+originExt)
}

// StagedOrigin reads a staged play's provenance and whether it still describes the source.
func (r Registry) StagedOrigin(rt Route) (Origin, OriginState) {
	raw, err := os.ReadFile(r.stagedOriginPath(rt))
	if err != nil {
		return Origin{}, OriginNone
	}
	var o Origin
	if err := json.Unmarshal(raw, &o); err != nil || o.Version != OriginVersion {
		return Origin{}, OriginUnreadable
	}
	source, err := os.ReadFile(r.StagedPath(rt))
	if err != nil {
		return o, OriginOrphaned
	}
	if DigestOf(string(source)) != o.Source {
		return o, OriginEdited
	}
	return o, OriginIntact
}

// ListStaged returns every saved-but-unregistered play, from both staging directories:
// `global/learned/` for an app-less play and `<app>/learned/` for an application's.
//
// # Why this is a second function and not a wider List
//
// Because `Resolve` step 3 walks `List`, so widening `List` by one directory would make every
// staged play answerable from every application — which is exactly the thing staging exists to
// prevent (see LearnedDir). Two enumerations with two names means a caller has to say which set
// it means, and no caller can reach the staged one by accident.
//
// # Read-only, and that is load-bearing
//
// `os.ReadDir` and nothing else. An application that has never saved a learned play has no
// `learned/` directory, and LISTING plays may not create the directory SAVING one would need.
//
// The Focus bit is `LearnedFocus` — an intention, not something read from disk.
//
// Deleting this must fail TestStagedPlaysAreListedAndStayUnresolvable.
func (r Registry) ListStaged() []Route {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil
	}
	var out []Route
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if name := e.Name(); name == GlobalDir {
			out = append(out, r.listDir(filepath.Join(r.Dir, name, LearnedDir), "", false)...)
		} else {
			out = append(out,
				r.listDir(filepath.Join(r.Dir, name, LearnedDir), name, LearnedFocus)...)
		}
	}
	sort.Slice(out, func(i, j int) bool { // the order List uses, for the same reason
		if out[i].Slug != out[j].Slug {
			return out[i].Slug < out[j].Slug
		}
		return out[i].App < out[j].App
	})
	return out
}

// KindOf is what kind of play this is, and whether its provenance still describes it.
//
// # Why the answer is not simply Origin.Kind
//
// Because a sidecar that is orphaned or unreadable still carries a `kind` field, and believing it
// would call a file learned on the strength of provenance that describes nothing. The state and
// the kind have to be read together or not at all.
//
// This is THE decider, and there is one so that there is one. `orchestrator.Classify` — which
// feeds the authority seam — and the product listing both call it, so a surface cannot come to a
// different conclusion about the same file than the door that guards it.
//
// Read-only: `Origin` and nothing else.
//
// Deleting this and re-deriving the switch in either caller must fail
// TestAnOrphanedSidecarDoesNotMakeAPlayLearned.
func (r Registry) KindOf(rt Route) (Kind, OriginState) {
	return kindFrom(r.Origin(rt))
}

// KindOfStaged is KindOf for a play that has not been registered yet.
//
// Separate because a staged play's sidecar is in a different directory: calling `KindOf` on a
// staged route reads `<app>/focus/<slug>.origin.json`, finds nothing, and reports the learned play
// Director just wrote as somebody's own writing.
//
// Deleting this and calling KindOf must fail TestAStagedPlayIsNotReportedAsAuthored.
func (r Registry) KindOfStaged(rt Route) (Kind, OriginState) {
	return kindFrom(r.StagedOrigin(rt))
}

// kindFrom is the one switch both readers share.
func kindFrom(o Origin, state OriginState) (Kind, OriginState) {
	switch state {
	case OriginNone, OriginOrphaned, OriginUnreadable:
		// No record, a record of nothing, or a record this version cannot read. The file is
		// still a play; it simply has no past worth believing.
		return KindAuthored, state
	default:
		return o.Kind, state
	}
}

// StagedSource returns a staged play's source.
func (r Registry) StagedSource(rt Route) (string, bool) {
	data, err := os.ReadFile(r.StagedPath(rt))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// DeleteStaged removes a saved-but-unregistered play and its provenance, and nothing else.
//
// # Why this is not Unregister
//
// `Unregister` forgets EVERYTHING that answers to a name in a scope — the registered play, its
// provenance, and any staged copy waiting behind it. That is right for "Marco no longer has this
// play", which is what a person means when they say it once about a name.
//
// It is wrong for a surface that lists the two SEPARATELY. The Plays list shows a registered play
// and a staged play of the same name as two rows, because they are two files with two different
// standings — and a staged play is normally in that position precisely BECAUSE registration was
// refused for colliding with the registered one. Forgetting one row through `Unregister` deleted
// the other, so following the registry's own advice ("rename the learned play or remove the other
// one first") destroyed the learned play it was telling you to keep.
//
// So each row gets a door onto its own files. This one opens on `<app>/learned/` and cannot reach
// anywhere else.
//
// Deleting this — and forgetting a staged play through Unregister — must fail
// TestForgettingAStagedPlayLeavesTheRegisteredOneAlone.
func (r Registry) DeleteStaged(rt Route) error {
	if err := os.Remove(r.stagedOriginPath(rt)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(r.StagedPath(rt)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Register moves a staged play into the scope the resolver reads.
//
// # What this does and does not do
//
// It makes the play DISCOVERABLE. It does not run it, does not create any authority to run it, and
// does not attach it to any phrase beyond the name a person chose — resolution afterwards is the
// ordinary route resolver, doing to this play exactly what it does to a taught one.
//
// A collision is refused rather than resolved. Overwriting somebody's authored play with something
// Director wrote is not a decision this code gets to make quietly.
// nameTaken reports whether this name is already answerable, in ANY scope that could answer it.
//
// # Why not just Has
//
// Because `Has` is scope-exact — it asks whether THIS file exists — and a collision is not about
// a file, it is about a name. A learned play registers as a focus route, so `Has` looks in
// `<app>/focus/`; somebody's authored play of the same name sits in `<app>/context/` or loose
// beside it. Scope-exact, they do not collide. To the person who asks for the name, they very
// much do, and the one that answers is whichever the resolver reaches first.
//
// So every scope that `Resolve` can reach for this name is checked: both of the application's
// scopes, the legacy loose location, and global. Registering may not put a second play behind a
// name that already means something.
//
// Deleting any arm must fail TestCollisionsAreRefusedRatherThanResolved.
func (r Registry) nameTaken(rt Route) (bool, string) {
	for _, c := range []struct {
		rt    Route
		where string
	}{
		{Route{App: rt.App, Slug: rt.Slug}, ""},
		{Route{App: rt.App, Focus: true, Slug: rt.Slug}, " as a focus play"},
		{Route{Slug: rt.Slug}, " as a global play"},
	} {
		if c.rt.App == "" && rt.App == "" && c.where != "" {
			continue // the app-less route IS rt; do not report it against itself
		}
		if r.Has(c.rt) {
			return true, c.where
		}
	}
	return false, ""
}

func (r Registry) Register(rt Route) error {
	if rt.Slug == "" {
		return fmt.Errorf("routes: a play needs a name")
	}
	if taken, where := r.nameTaken(rt); taken {
		return fmt.Errorf(
			"routes: %s already exists%s; rename the learned play or remove the other one first",
			rt.Slug, where)
	}
	source, ok := r.StagedSource(rt)
	if !ok {
		return fmt.Errorf("routes: there is no saved play called %s", rt.Slug)
	}
	o, state := r.StagedOrigin(rt)
	if !state.Verified() {
		// Fail-closed. A play whose provenance no longer describes it may still be
		// registered — by hand, as an ordinary play — but not BY THIS, which would be
		// registering it as something Director verified.
		return fmt.Errorf("routes: %s is %s, so it cannot be registered as a learned play",
			rt.Slug, state)
	}
	// REGISTERED **OR** STAGED, NEVER BOTH.
	//
	// Two directories hold one play, and the whole point of that is that a person can look at
	// the filesystem and know where it stands. A failed registration that left a half-written
	// copy in `context/` next to the original in `learned/` would make the same play answer
	// two different questions — and the retry would then refuse on a collision with itself.
	//
	// So a partial write is undone, and the staged copy — untouched, complete, and still the
	// only one — is what the caller comes back to.
	//
	// Deleting the rollback must fail TestARefusedRegistrationLeavesOneCopy.
	if err := r.SaveWithOrigin(rt, source, o); err != nil {
		_ = os.Remove(r.OriginPath(rt))
		_ = os.Remove(r.Path(rt))
		return err
	}
	if err := os.Remove(r.StagedPath(rt)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("routes: %s is registered and its staged copy could not be "+
			"removed, so the play is now in two places: %w", rt.Slug, err)
	}
	_ = os.Remove(r.stagedOriginPath(rt))
	return nil
}

// Unregister removes a play and its provenance together.
//
// It removes the PLAY. It does not touch anything Director observed: "Marco no longer has this
// play" and "Marco must forget what it saw" are different operations, and only one of them was
// asked for.
func (r Registry) Unregister(rt Route) error {
	if err := r.DeleteOrigin(rt); err != nil {
		return err
	}
	// THE STAGED COPY GOES TOO, and this is the only door it can leave by.
	//
	// A play that was saved and whose registration was refused — a name already taken is the
	// ordinary way — sits in `<app>/learned/` where nothing scans. Forgetting reached only the
	// registered scope, so that copy could never be removed by any command a person has: it
	// was unreachable, unforgettable, and still enough to make the next `Register` of the same
	// slug behave strangely. Removing it here costs nothing when there is none.
	//
	// Deleting this must fail TestForgettingRemovesAStagedPlayNothingCouldReach.
	if err := os.Remove(r.stagedOriginPath(rt)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Remove(r.StagedPath(rt)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return r.Delete(rt)
}
