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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Prepared is one routine rendered again from its own stored demonstration, for review.
//
// NOTHING ON DISK HAS CHANGED. Every field here is a reading or a rendering held in memory, and
// the routine is exactly as it was until `Replace` is called with this and succeeds.
type Prepared struct {
	// Token identifies THIS preparation, and a replacement must carry it back.
	//
	// # Why an identity for the review and not just for the routine
	//
	// Because a window can have two preparations of one routine in flight. Studio held
	// them in a slot keyed by the routine, so the second overwrote the first — and the two
	// calls are asynchronous, so the page can be displaying A's sources while the slot
	// holds B. Press Replace and B is written: every check passes, the success names the
	// routine the person was looking at, and the bytes are the ones they never saw.
	//
	// # Why it is derived from the content rather than counted
	//
	// Because what has to be identified is the review — the two sources on the screen and
	// the recording they came from — and the content IS that. Two preparations that share
	// a token are byte-for-byte identical in what was displayed and in what a replacement
	// would write, so resolving one to the other cannot write anything nobody saw. A
	// counter would be unique and would need somewhere to live, and the place it would
	// live is the shared state this defect came out of.
	//
	// It is immutable: a `Prepared` is built once and never edited, and every field this
	// is derived from is fixed at the moment the two files were read.
	Token string
	// Route is the exact routine this is about — the identity a replacement targets, and
	// the one a caller selected. Not a name, which two routines can share.
	Route routes.Route
	// Rebuild is which of the two sets of rules this version was rendered under.
	//
	// PART OF THE REVIEW'S IDENTITY, so it is in the token: two reviews of one routine
	// differing only in the choice that produced them are two different things to replace
	// it with, and the person picked one of them. See `sealed`.
	Rebuild Rebuild
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
	// library is the store this was prepared against, which is part of the target and
	// therefore part of the token. A `Deps` holding another library may not replace with
	// it: the same routine exists in a dogfood store, in a test store and in somebody's
	// real one, and "the right bytes in the wrong library" is a replacement nobody asked
	// for. See `sealed`.
	library string
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
func (d Deps) Prepare(rt routes.Route, kind Rebuild) (Prepared, error) {
	if !kind.known() {
		// A WORD THIS DOES NOT KNOW IS NOT A CHOICE. Not defaulted to the gentler
		// one either: a caller asking for rules nobody here can name is a caller
		// about to show somebody a review labelled as something it is not.
		return Prepared{}, fmt.Errorf("%q is not a way of rebuilding a routine", kind)
	}
	path := d.Reg.Path(rt)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Prepared{}, fmt.Errorf("%s could not be read, so there is nothing to "+
			"compare a rebuilt version against: %w", rt.Slug, err)
	}
	src := string(raw)
	p := Prepared{Route: rt, Rebuild: kind, Name: NameIn(src, rt.Slug), Was: src,
		library:       d.Reg.Dir,
		sourceAt:      routes.DigestOf(src),
		WasGeneration: GenerationOf(src), Differences: GenerationDifferencesOf(src)}

	// ── ELIGIBILITY, AGAINST THE SOURCE THIS ACTUALLY READ ──
	//
	// Not against whatever a caller looked at earlier. Studio greys its button from a
	// LISTING, which is one read of the library; this is a second read, and between the
	// two the file can be replaced — by a hand edit, by another window, by a sync — with a
	// version stating a generation this build cannot explain. Checking only the listing
	// would let exactly that case through the restriction, and the restriction is the one
	// place where this build refuses to say what a replacement would change.
	//
	// So the rule is applied where the bytes are: the generation read off THIS snapshot
	// decides, and the sentence is the same one the card shows.
	if !p.WasGeneration.Offerable() {
		return Prepared{}, fmt.Errorf("%q cannot be rebuilt: %s", p.Name,
			GenerationSentence(p.WasGeneration, d.Reg.HasRecording(rt)))
	}

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
	dr := d.draftFor(kind, p.Name, events, rt.App, "a review", ModeOf(src))
	switch {
	case dr == nil || len(dr.Steps) == 0:
		p.Refused = "its demonstration contains nothing that can be performed"
		return p.sealed(), nil
	case dr.Unrepresentable != "":
		// A GESTURE TODAY'S RULES WILL NOT WRITE DOWN. The same refusal a fresh
		// save would give, surfaced in the review rather than at the moment of
		// replacement.
		p.Refused = fmt.Sprintf("a version rebuilt from its recording cannot be "+
			"saved as it stands: %s", dr.Unrepresentable)
		return p.sealed(), nil
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
		return p.sealed(), nil
	}
	p.Now, p.assets, p.NowGeneration = now, assets, GenerationOf(now)
	return p.sealed(), nil
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
	// AND THE LIBRARY IS THIS ONE. Said separately from the token, which would also
	// catch it and would say the wrong thing about it: "not the one that was prepared"
	// sends somebody to look at the review, and the review is fine — it is this `Deps`
	// that is pointed somewhere else. The same routine exists in a dogfood store, in a
	// test store and in somebody's real one.
	if p.library != "" && p.library != d.Reg.Dir {
		return fmt.Errorf("%q was reviewed in the library at %s and this is %s, so "+
			"nothing was written — a routine of that name here is a different "+
			"routine", p.Name, p.library, d.Reg.Dir)
	}
	// AND IT IS THE PREPARATION IT SAYS IT IS. The token is derived from the fields
	// below it, so a `Prepared` edited after it was prepared — by a caller, by a bug, by a
	// struct assembled from parts of two — no longer matches its own content. Cheap, and
	// it means the one thing a caller hands back cannot be quietly wrong.
	if p.Token == "" || p.Token != p.sealed().Token {
		return fmt.Errorf("the version offered for %q is not the one that was "+
			"prepared, so nothing was written — review it again", p.Name)
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
	// ── THE PICTURES ARE NOT ADDITIVE, WHICH THIS USED TO ASSUME ──
	//
	// A routine's anchor images are named after the routine — `<base>-anchor-1.png` and so
	// on — so a rebuild produces the SAME paths. Writing them and calling that additive
	// was wrong in the one direction that matters: write the new images, fail to write the
	// source, and the OLD source is still on disk pointing at paths that now hold
	// different pictures. A routine that presses what a different image matched, having
	// been left alone.
	//
	// So `imagesToWrite` decides every path before anything is written, and only the
	// MISSING ones are written: one that already holds the right picture is skipped, and
	// one that holds a different picture or cannot be read at all is a refusal with
	// nothing written. See there for why a refusal rather than a way round.
	//
	// What that leaves if the SOURCE write then fails: image files that did not exist
	// before, beside an unchanged old source that does not reference them. Unreferenced
	// files, and the routine behaves exactly as it did.
	images, why := imagesToWrite(p.assets)
	if why != "" {
		return fmt.Errorf("%q is left as it is: %s", p.Name, why)
	}
	// AND THE CREATION IS ITSELF THE CHECK, because classifying and then writing leaves a
	// gap. Between deciding a path was missing and writing it, another save — another
	// Studio, a `marco learn`, a sync — can create it, and `os.WriteFile` would truncate
	// what they had just written on the strength of a reading taken beforehand. So these
	// go through `CreateAssets`, which creates exclusively: the path existing IS the
	// failure, with nothing written to it.
	if d.afterClassifying != nil {
		d.afterClassifying()
	}
	if err := d.Reg.CreateAssets(images); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%q is left as it is: an image the rebuilt version "+
				"needs appeared while this was being prepared to write, so "+
				"something else is writing beside this routine — review it "+
				"again and the comparison will be against what is there now "+
				"(%v)", p.Name, err)
		}
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

// imagesToWrite is which of a rebuild's anchor images may be written, or the reason none may.
//
// Four states per path, and the first version of this collapsed two of them wrongly.
//
//	MISSING      write it. A file nobody references yet is not a change to anything that runs.
//	THE SAME     SKIP it. Writing a file the bytes it already holds changes nothing that runs
//	             and is still a write: it touches the file, it can fail, and it is one more
//	             thing between a person's decision and the one file this is meant to replace.
//	DIFFERENT    REFUSE. These paths are reusable — `<base>-anchor-N.png` — so this is not an
//	             addition, it is an overwrite, and if the source write then failed the OLD
//	             source would be left pointing at a picture it was never written for.
//	UNREADABLE   REFUSE. Locked, in use, a directory in the way, permissions. The first
//	             version treated this as "nothing there", which is the one reading that
//	             licenses writing over it — and a path this cannot read is precisely a path
//	             it cannot say is safe to replace.
//
// ALL OR NOTHING, which is why this decides everything before anything is written. A refusal
// found on the third image after two were written would leave a routine beside pictures from two
// different rebuilds.
//
// # Why a refusal rather than a way round
//
// The alternative is content-addressed image names, which makes the paths immutable and the
// question disappear. That changes what every newly generated routine's files are CALLED,
// reaching every asset writer and every saved file's embedded paths, and would arrive inside a
// provenance slice unreviewed. A refusal is also the honest answer for the case it catches: if
// the images beside a routine have changed since it was written, the person has two things that
// disagree, and quietly overwriting one of them on the way past is not a decision this flow may
// make for them.
func imagesToWrite(assets map[string][]byte) (map[string][]byte, string) {
	write := map[string][]byte{}
	for path, want := range assets {
		got, err := os.ReadFile(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			write[path] = want
		case err != nil:
			return nil, fmt.Sprintf("the image at %s cannot be read, so nothing "+
				"here can say whether replacing it would change what the "+
				"routine you have looks for — so nothing was written (%v)",
				path, err)
		case bytes.Equal(got, want):
			// SKIPPED, not written. See above.
		default:
			return nil, fmt.Sprintf("the image at %s is not the one the rebuilt "+
				"version needs, and replacing it would change what the "+
				"routine you have looks for as well — so nothing was "+
				"written. Record it again if the pictures beside it are out "+
				"of date", path)
		}
	}
	return write, ""
}

// sealed is this preparation with its token, which is the last thing done to it.
//
// EVERY EXIT GOES THROUGH HERE, including the three refusals — a refused review is still shown on
// a screen and still has to be told apart from another refused review of the same routine, which
// is the whole of what the token is for. A `Prepared` returned without one cannot be replaced
// with, which is safe, and would be a silent dead end rather than a refusal with a reason.
//
// # The TARGET is part of it, and leaving it out was a defect
//
// The first version of this digested only the content — the two files, what would be written, and
// why nothing would be — on the argument that two preparations sharing a token are identical in
// what was displayed and in what a replacement would write. That argument was incomplete: it said
// nothing about WHERE the replacement would write. Two routines with identical sources and
// identical demonstrations are a thing this product produces readily — the same routine saved
// under two scopes or for two applications, which is the `settings` case everything about
// `Routine.ID` exists for — and they would have shared a token. A window holding one and handed
// the other's token would then write the right bytes to the wrong file.
//
// So the identity of the routine and the library it lives in are digested too. Two preparations
// sharing a token now agree about all four questions: what was shown, what would be written,
// where, and in which library.
func (p Prepared) sealed() Prepared {
	p.Token = routes.DigestOf(strings.Join([]string{
		// WHAT THE REVIEW IS: the two files it was built from, what it would
		// write, and why it would write nothing.
		p.sourceAt, p.recordingAt, routes.DigestOf(p.Now), p.Refused,
		// AND WHERE IT WOULD GO: the route's whole identity, each part on its
		// own so no two routes can run together into one string, and the
		// library. `Focus` is in because it is the difference between a routine
		// that brings its application forward and one that does not, and those
		// are two files.
		p.Route.Slug, p.Route.App, fmt.Sprint(p.Route.Focus), p.library,
		// AND WHICH RULES IT WAS RENDERED UNDER. Two reviews of one routine
		// differing only in the choice that produced them are two different
		// things to replace it with, and the person picked one of them.
		string(p.Rebuild),
	}, "\x00"))
	return p
}
