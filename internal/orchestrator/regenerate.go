package orchestrator

// PREPARE, REVIEW, THEN REPLACE OR CANCEL — AND NOTHING IS REWRITTEN BEHIND ANYBODY'S BACK.
//
// A saved routine keeps the rules it was generated with, and the way to give it today's is to
// render its stored demonstration again. That is a replacement of somebody's working file, so it
// is four steps and not one:
//
//	SELECT    one exact routine, by its own identity.
//	PREPARE   render the recording again. NOTHING IS WRITTEN. The result is held in memory,
//	          along with a digest of each of the two things it was built from.
//	REVIEW    both sources, the differences the change table names, and any refusal — because a
//	          rebuild under today's rules can legitimately refuse where the old save did not.
//	REPLACE   or cancel. Cancelling writes nothing, because nothing was written.
//
// # Why `Resave` is not this flow
//
// `Resave` repairs a routine that cannot press anything at all, and it is immediate on purpose:
// there is nothing to weigh up, because the file as it stands refuses every press. It also
// RESCOPES — `ScopeAppAnywhere` is the repair — and it saves through the ordinary save path,
// which chooses where the routine lives. Neither is allowed here. This replaces a routine that
// works today with one that works differently, which is a decision only its owner can make, and
// it writes the routine back exactly where and as it was.
//
// # What the two digests are for
//
// A review is of a particular pair of files, and both can change while somebody reads it: a
// person can edit the routine in `marco edit`, or record the same name again, in another window
// while the review is open. Replacing then would write a rendering of a recording that is no
// longer there over a source nobody reviewed — and silently, because the new file would look
// exactly like a successful replacement. So a replacement re-reads both and refuses if either
// moved, naming which.
//
// This is also the whole of the protection for a HAND-EDITED routine. A person who edited their
// generated file sees their own text as the left-hand side of the review: what replacing it costs
// them is on the screen, in their own words, before they decide. What is not allowed is for that
// to happen without the review.
//
// See [[ADR-262-a-routine-states-which-generation-wrote-it]].

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Prepared is one routine rendered again from its own stored demonstration, for review.
//
// NOTHING ON DISK HAS CHANGED. Every field here is a reading or a rendering held in memory, and
// the routine is exactly as it was until `Replace` is called with this and succeeds.
type Prepared struct {
	// Route is the exact routine this is about — the identity a replacement targets, and
	// the one a caller selected. Not a name, which two routines can share.
	Route routes.Route
	// Name is what the routine calls itself, as its own header records it.
	Name string
	// Was is the saved source as it stands, and Now the source a replacement would write.
	//
	// BOTH IN FULL, because the review is of the whole of each. A list of differences is
	// an interpretation, and this product's own history says the interpretation is the
	// part that goes wrong; the file is the thing that will run.
	Was, Now string
	// Refused is why a rebuild of this recording produces no routine at all, in the
	// generator's own words, and empty when it produced one.
	//
	// A REVIEWABLE ANSWER AND NOT AN ERROR. Today's rules refuse a recording containing a
	// press that cannot be stopped at, which an older save wrote happily — so "what would
	// I get if I rebuilt this" has three answers, not two, and a person is owed the third.
	// A `Prepared` carrying one cannot be replaced with.
	Refused string
	// WasGeneration and NowGeneration are what each source states about the generation
	// that wrote it.
	WasGeneration, NowGeneration StatedGeneration
	// Differences is what the saved routine was written without, resolved against its own
	// contents — empty unless it states a generation this build can walk from. See
	// `GenerationDifferencesOf`, and `Bears` for the only thing that licenses saying a
	// routine needs replacing.
	Differences []Difference
	// sourceAt and recordingAt are digests of the two files this was built from, as they
	// were at the moment of preparation. A replacement re-reads both and refuses if
	// either moved. See the note at the top of this file.
	sourceAt, recordingAt string
	// assets are the files the rendering produced — a routine's anchor images — held here
	// rather than written, because preparation writes nothing.
	assets map[string][]byte
}

// Replaceable is whether this review has something a replacement could write.
func (p Prepared) Replaceable() bool { return p.Refused == "" && p.Now != "" }

// Changes is whether a replacement would change the file at all.
//
// A routine already rendered by today's generator from its own recording rebuilds byte for byte,
// and offering to replace a file with itself is an offer to do nothing — worth saying so plainly
// rather than leaving somebody to compare two identical panes.
func (p Prepared) Changes() bool { return p.Replaceable() && p.Now != p.Was }

// Prepare renders a saved routine again from its own stored demonstration, for review.
//
// IT WRITES NOTHING. Not the source, not the assets, not the recording — a failure anywhere in
// here leaves the library exactly as it was, which is why every exit is a returned error rather
// than a partial state.
func (d Deps) Prepare(rt routes.Route) (Prepared, error) {
	path := d.Reg.Path(rt)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Prepared{}, fmt.Errorf("%s could not be read, so there is nothing to "+
			"compare a rebuilt version against: %w", rt.Slug, err)
	}
	src := string(raw)
	p := Prepared{Route: rt, Name: NameIn(src, rt.Slug), Was: src,
		sourceAt:      routes.DigestOf(src),
		WasGeneration: GenerationOf(src), Differences: GenerationDifferencesOf(src)}

	rec, ok := d.Reg.LoadRecording(rt)
	if !ok {
		// THE EXPLANATION IS STILL OWED, AND IT IS NOT THIS FUNCTION'S. A routine
		// whose demonstration is gone can be read and described — see
		// `GenerationSentence`, which says so without offering anything. What
		// cannot happen is an offer that fails after somebody takes it.
		return Prepared{}, fmt.Errorf("%q no longer has the demonstration it was made "+
			"from, so there is nothing to rebuild a version from — recording it "+
			"again is the way to get one written by today's rules", p.Name)
	}
	p.recordingAt = routes.DigestOf(string(rec))
	var events []recorder.RecordedEvent
	if err := json.Unmarshal(rec, &events); err != nil {
		return Prepared{}, fmt.Errorf("%q: its demonstration could not be read back, "+
			"so no version can be rebuilt from it: %w", p.Name, err)
	}
	// ── WHAT THE ROUTINE SAYS IT WAS RECORDED AS, READ OFF THE FILE BEING REPLACED ──
	//
	// Not from this `Deps` and not from the caller. The stated mode decides the
	// SIMPLIFICATION as well as the line it writes, so taking it from the environment
	// would rebuild somebody's routine with every recorded pause capped and no anchor to
	// wait on in its place — a routine that then presses a menu before it has drawn,
	// having been repaired. That is a defect this had, in `Resave`, and the reason
	// `ModeOf` is read here too.
	dr := d.DraftOf(p.Name, events, rt.App, "a review", ModeOf(src))
	switch {
	case dr == nil || len(dr.Steps) == 0:
		p.Refused = "its demonstration contains nothing that can be performed"
		return p, nil
	case dr.Unrepresentable != "":
		// A GESTURE TODAY'S RULES WILL NOT WRITE DOWN. The same refusal a fresh
		// save would give, surfaced in the review rather than at the moment of
		// replacement.
		p.Refused = fmt.Sprintf("a version rebuilt from its recording cannot be "+
			"saved as it stands: %s", dr.Unrepresentable)
		return p, nil
	}
	// AND THE SCOPE IT ALREADY HAS. `Save` chooses one; this one is not choosing. A
	// routine that came back from a review living somewhere else, or bringing an
	// application forward when it used not to, would be a replacement nobody reviewed.
	scope := ScopeAlreadyOf(rt)
	now, assets, err := d.render(dr, scope.mode(), rt.App, rt)
	if err != nil {
		// THE GENERATOR'S OWN WORDS. It refuses a recording whose press cannot be
		// stopped at, which is a thing a person needs to read rather than a thing
		// this should translate.
		p.Refused = err.Error()
		return p, nil
	}
	p.Now, p.assets, p.NowGeneration = now, assets, GenerationOf(now)
	return p, nil
}

// Replace writes the reviewed source over the routine it was prepared from.
//
// # It refuses unless the world is still the one that was reviewed
//
// Both of the files this was built from are re-read and re-digested. Either having moved means
// the review was of something else, and writing anyway would replace a source nobody looked at
// with a rendering of a recording that is no longer there — invisibly, because the result would
// look like an ordinary successful replacement.
//
// On any refusal, and on a failed write, the routine is exactly as it was: the source lands whole
// or not at all (`routes.Registry.Replace`), and the assets go first, because they are additive
// and a failure to write them must not leave a new source beside the old pictures.
func (d Deps) Replace(p Prepared) error {
	if p.Route.Slug == "" {
		return fmt.Errorf("a replacement needs the routine it was prepared from")
	}
	if !p.Replaceable() {
		if p.Refused != "" {
			return fmt.Errorf("%q cannot be replaced from its recording: %s", p.Name,
				p.Refused)
		}
		return fmt.Errorf("%q has no rebuilt version to be replaced with", p.Name)
	}
	raw, err := os.ReadFile(d.Reg.Path(p.Route))
	if err != nil {
		return fmt.Errorf("%q could not be read back, so it is left as it is: %w",
			p.Name, err)
	}
	if now := routes.DigestOf(string(raw)); now != p.sourceAt {
		return fmt.Errorf("%q has been saved again since the version you reviewed was "+
			"built, so it is left as it is — review it again and the comparison "+
			"will be against the routine as it now stands", p.Name)
	}
	rec, ok := d.Reg.LoadRecording(p.Route)
	if !ok {
		return fmt.Errorf("%q no longer has the demonstration the version you reviewed "+
			"was built from, so it is left as it is", p.Name)
	}
	if now := routes.DigestOf(string(rec)); now != p.recordingAt {
		return fmt.Errorf("%q has been recorded again since the version you reviewed "+
			"was built, so it is left as it is — review it again and the "+
			"comparison will be against the new recording", p.Name)
	}
	// THE PICTURES FIRST. They are additive — each one keyed by the path the new source
	// names — so a failure here leaves the old source beside the old pictures, which is
	// the state that was there a moment ago. The reverse order would leave a new routine
	// looking for an image nobody wrote.
	if err := d.Reg.WriteAssets(p.assets); err != nil {
		return fmt.Errorf("%q is left as it is: the images the rebuilt version needs "+
			"could not be written: %w", p.Name, err)
	}
	if err := d.Reg.Replace(p.Route, p.Now); err != nil {
		return fmt.Errorf("%q is left as it is: %w", p.Name, err)
	}
	mlog.Info("review: replaced from its own recording", "route", p.Route.Slug,
		"was", string(p.WasGeneration.Reading), "now", string(p.NowGeneration.Reading),
		"scope", string(ScopeAlreadyOf(p.Route)))
	return nil
}
