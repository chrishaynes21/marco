//go:build windows

package main

// WHAT MARCO WROTE DOWN ABOUT A RUN, READ BACK OFF DISK.
//
// This is MARCO's account, and it is kept rigidly apart from the fixture's. The fixture says what
// it received; this says what Marco believes it did. The whole value of the harness is in
// comparing them, and a comparison is worth nothing if either side has been helped by the other:
//
//	both agree               — the result, and the only one that may be called green
//	Marco failed and said so — an honest failure, and reportable as one
//	Marco believed it won    — the dangerous case, and the reason there are two accounts
//
// The record is read from the store THIS RUN owns, which is a temporary directory nothing else
// writes to — so a record found here was written by this run's `marco`, and the owner's own store
// is never opened.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

// runRecord is the part of a routine's run record this harness asks about.
//
// The field names are the record's own; a harness with its own names for them would be a second
// vocabulary for one fact. Pointers where the record uses them, because ABSENT and ZERO are
// different answers there and flattening them would be undoing the thing that was built.
type runRecord struct {
	Version     int    `json:"version"`
	Kind        string `json:"kind"`
	Run         string `json:"run"`
	Build       string `json:"build"`
	Routine     string `json:"routine"`
	Application string `json:"application"`
	Scope       string `json:"scope"`
	Source      string `json:"source_digest"`
	Library     string `json:"library"`
	Mode        string `json:"mode"`
	Contract    string `json:"contract"`
	Started     string `json:"started"`
	Ended       string `json:"ended"`
	Outcome     string `json:"outcome"`
	Because     string `json:"because"`
	Asked       *int   `json:"input_actions"`
	file        string
}

func (r runRecord) String() string {
	asked := "absent"
	if r.Asked != nil {
		asked = fmt.Sprintf("%d", *r.Asked)
	}
	return fmt.Sprintf("%s: %s/%s %s outcome=%q mode=%s contract=%s input_actions=%s "+
		"build=%s", filepath.Base(r.file), r.Application, r.Routine, r.Scope, r.Outcome,
		r.Mode, r.Contract, asked, r.Build)
}

// records is every run record in this run's store, oldest first.
func records(store string) ([]runRecord, error) {
	dir := filepath.Join(store, "runs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []runRecord
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var r runRecord
		if err := json.Unmarshal(data, &r); err != nil {
			return out, fmt.Errorf("%s is not a readable run record: %w", e.Name(), err)
		}
		r.file = p
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Run < out[j].Run })
	return out, nil
}

// waitForRecord waits until a run record exists that has ENDED, and hands back the last one.
//
// Ended, not merely present: the reference is claimed and a provisional record written before
// anything is sent, so a record that exists says only that the attempt began. A harness that read
// the first thing it found would be reporting the outcome of a run still in flight — which for
// this record shape is the word `unfinished`, and it would look exactly like a real verdict.
func waitForRecord(bound time.Duration, store string, after int) (runRecord, error) {
	var got runRecord
	ok, saw := until(bound, func() (bool, string) {
		rs, err := records(store)
		if err != nil {
			return false, err.Error()
		}
		if len(rs) <= after {
			return false, fmt.Sprintf("%d record(s) so far", len(rs))
		}
		last := rs[len(rs)-1]
		if last.Ended == "" {
			return false, "the record is there and the run has not finished"
		}
		got = last
		return true, got.String()
	})
	if !ok {
		return got, fmt.Errorf("no finished run record appeared in %s: %s", store, saw)
	}
	return got, nil
}

// countRecords is how many are there now, so a scenario can ask about what happens next.
func countRecords(store string) int {
	rs, _ := records(store)
	return len(rs)
}

// routesDir is this run's library.
//
// Through the production rule rather than by joining paths here. `main` puts this run's store in
// `$MARCO_HOME` before anything reads a route, so the harness, Studio, and the `marco` Studio
// spawns all resolve the library the same way — which is the whole of what `internal/marcopaths`
// exists to guarantee, and the thing whose absence once made a routine saved in Studio invisible
// to `marco do`.
func routesDir() string { return marcopaths.Routes() }

func readFileString(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
