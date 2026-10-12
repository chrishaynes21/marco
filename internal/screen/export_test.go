package screen

import "time"

// AddAcquireSampleForTest feeds one latency to a stage.
//
// `add` is unexported because nothing outside this package produces a stage's samples — the
// benchmark does, and it is in here. The tests for the percentile arithmetic are in the external
// test package, where they cannot reach it, and the arithmetic is worth testing: a cadence has
// to survive its p95, and a percentile taken without sorting reports whatever arrived last.
//
// Test-only, in a _test.go file, so it exists in no build of the product.
func AddAcquireSampleForTest(s *AcquireStage, d time.Duration) { s.add(d) }
