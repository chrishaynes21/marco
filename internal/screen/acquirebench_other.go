//go:build !windows

package screen

import (
	"fmt"
	"time"
)

// BenchmarkAcquire is unavailable without a capture backend, and says so.
//
// Inert rather than faked, for the reason the whole stub side of this package is: a benchmark
// that returned plausible timings on a platform with no capture would be a measurement of
// nothing, and it would be indistinguishable in a report from one taken on a machine that can
// actually take a picture.
func BenchmarkAcquire(x, y, w, h int, cfgs []AcquireConfig, frames int,
	maxWall time.Duration) ([]AcquireResult, error) {

	return nil, fmt.Errorf("screen: measuring acquisition needs a capture backend, " +
		"which this platform build does not provide")
}
