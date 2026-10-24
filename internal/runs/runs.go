// Package runs is the discipline of recording an attempt: the place, the reference, the claim
// made before anything is sent, and the finalisation that can only write where a claim already
// is.
//
// # Why this is shared and the record is not
//
// Two kinds of run end up in one directory. A supervised Act's run is an account of EVIDENCE —
// how many actions verified, how many were contradicted, how many nobody could decide — and a
// routine's run is an account of DELIVERY: every press went out, or it stopped and said which
// term disagreed. The two vocabularies overlap in three words out of seven and the rest do not
// translate: nothing may turn `performed` into `verified`, because a routine's run looks at
// nothing afterwards and `verified` is a claim about evidence it never gathered.
//
// So one record shape would have to drop one of those meanings, or carry both and let each side
// ignore half. What the two kinds genuinely share is narrower, and it is all here:
//
//   - a PLACE, so one reference retrieves one attempt whoever wrote it (see marcopaths.Runs);
//   - a CLAIM on the reference, made exclusively, before any input is sent;
//   - a provisional record left in that claim at once, so an attempt that dies is retrievable
//     as the attempt it was rather than retrieving nothing;
//   - a FINALISATION that cannot create, so a record can only be written into a claim that is
//     already there — which is what stops two attempts writing over each other.
//
// All of that is about one attempt's namespace and none of it is about what happened. The record
// itself is the caller's own type, marshalled as the caller declared it, and this package never
// looks inside one.
//
// The citation — the single line one process prints and another parses — is
// `internal/director/runref`, and stays there: it is a wire format between two binaries and has
// its own reasons.
package runs

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

// Dir is where records are kept.
//
// Through marcopaths rather than derived here: a second derivation of one directory is a second
// answer the day either side changes, and this one is read by a window in another process.
func Dir() string { return marcopaths.Runs() }

// Path is the record for one reference.
func Path(id string) string { return filepath.Join(Dir(), id+".json") }

// Candidate proposes a reference for an attempt that began at `at`.
//
// Sortable, second-resolution, and local time because the person reading it is in it. A PROPOSAL
// and not an identity: [Reserve] decides whether it is free, because randomness reduces
// collisions and does not prevent them.
func Candidate(at time.Time) string {
	// AND A SUFFIX, BECAUSE A SECOND IS NOT UNIQUE.
	//
	// The timestamp is the sortable half and a person reads it. Two attempts started in
	// the same second — a script, a retry, two terminals — produced the same id, and the
	// one thing a reference must do is retrieve the attempt it names.
	//
	// Four hex characters from the system's randomness, which is short enough to type and
	// read aloud. If it cannot be read, the nanosecond stands in: a worse suffix is
	// better than none, and this must never fail a run.
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("run-%s-%04x", at.Format("20060102-150405"),
			at.Nanosecond()&0xffff)
	}
	return fmt.Sprintf("run-%s-%02x%02x", at.Format("20060102-150405"), b[0], b[1])
}

// Tries is how many candidates are tried before giving up.
//
// Ten, which is not a probability argument. Each try is a fresh draw against a namespace nobody
// is competing for in earnest, so ten failures in a row means something other than bad luck — a
// read-only directory, a full disk, a clock that does not move — and that is worth reporting
// rather than retrying forever.
const Tries = 10

// A Claim is what one attempt asks to reserve.
type Claim struct {
	// At is when the attempt began, and what its reference is named after.
	At time.Time
	// Provisional is the record left in the reservation at once, for the id it is handed.
	//
	// THE CALLER'S OWN SHAPE, which is the whole reason this package is shared and the
	// record is not. It should say `started`, or whatever that kind's vocabulary calls an
	// attempt that has not finished, and it must name the id it was given — a provisional
	// record filed under one reference while describing another is worse than none.
	Provisional func(id string) any
	// FramesRoot, when set, is the directory this run's own frames directory is claimed
	// under. Empty means no pixel evidence was asked for and none is claimed.
	FramesRoot string
	// Propose overrides [Candidate], for a caller that needs a known reference. Nil is
	// the ordinary case.
	Propose func(time.Time) string
}

// A Reservation is one claimed namespace.
type Reservation struct {
	// ID is the reference, now this attempt's and no other's.
	ID string
	// Frames is this run's own frames directory, empty when none was asked for.
	Frames string
}

// Reserve claims a whole evidence namespace for one attempt, before anything is written into it
// and before the reference is printed.
//
// # Why reservation and not a check at the end
//
// The record used to be the only thing guarded: finalisation wrote with `O_EXCL`, so a second
// attempt with the same id was refused its JSON. By then both attempts had been printing the same
// reference for a minute and the hover probe had been writing `01-before.png` into the same frames
// directory — so the refusal protected the record and not the evidence it points at, and the
// attempt that was refused had already overwritten the other's images.
//
// So the id is claimed FIRST, and claimed across both places it means something: a provisional
// record under the runs directory, created exclusively, and this run's own frames directory,
// created exclusively when frames were asked for. A candidate that loses either is abandoned
// whole — the record it already made is removed — and the next one is tried. Two attempts that
// propose the same id therefore end up with two namespaces, and neither can reach the other's.
//
// The provisional record is a real record. An attempt that dies before finalisation is then
// retrievable as what it was, rather than retrieving nothing.
func Reserve(c Claim) (Reservation, error) {
	if c.Provisional == nil {
		return Reservation{}, fmt.Errorf("a reservation needs the record it leaves behind")
	}
	propose := c.Propose
	if propose == nil {
		propose = Candidate
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return Reservation{}, err
	}
	for try := 0; try < Tries; try++ {
		id := propose(c.At)
		p := Path(id)
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return Reservation{}, err
		}
		b, _ := json.MarshalIndent(c.Provisional(id), "", "  ")
		_, werr := f.Write(b)
		cerr := f.Close()
		if werr != nil || cerr != nil {
			_ = os.Remove(p)
			if werr != nil {
				return Reservation{}, werr
			}
			return Reservation{}, cerr
		}
		if c.FramesRoot == "" {
			return Reservation{ID: id}, nil
		}
		// AND THE PIXELS' OWN NAMESPACE, claimed by the same id.
		//
		// `MkdirAll` on the parent and a bare `Mkdir` on this run's own element:
		// the first is shared and must succeed if it is missing, the second must
		// FAIL if somebody else has it. MkdirAll on the whole path would succeed
		// on an existing directory and claim nothing, which is the hole this
		// closes.
		if err := os.MkdirAll(c.FramesRoot, 0o755); err != nil {
			_ = os.Remove(p)
			return Reservation{}, err
		}
		d := filepath.Join(c.FramesRoot, id)
		if err := os.Mkdir(d, 0o755); err != nil {
			_ = os.Remove(p)
			if os.IsExist(err) {
				continue
			}
			return Reservation{}, err
		}
		return Reservation{ID: id, Frames: d}, nil
	}
	return Reservation{}, fmt.Errorf("no free run reference after %d tries", Tries)
}

// Finalise writes a record into the reservation of that same id, and returns the path.
//
// A failure here is NOT a failure of the run. The run already happened; losing its record loses
// the ability to cite it, which is worth a line on stderr and nothing more.
//
// # It may only write where a reservation already is
//
// No O_CREATE, which is the whole guard. [Reserve] created this file exclusively, so the file
// EXISTING is the proof that this id belongs to this attempt; opening without O_CREATE means a
// record for an id nobody reserved cannot be written at all, and a record for an id somebody else
// holds was never reachable because they hold it.
//
// An earlier version of this guarded the other way round — O_CREATE|O_EXCL at save time — which
// refused the second writer after both had been printing the same reference and writing into the
// same frames directory for the length of a run.
func Finalise(id string, rec any) (string, error) {
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", err
	}
	p := Path(id)
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return p, nil
}
