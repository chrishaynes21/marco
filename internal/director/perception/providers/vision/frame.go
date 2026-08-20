package vision

import (
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The frame.
//
//	The Director should never know where the frame came from — desktop capture, game
//	capture, browser, remote stream.
//
// So a Frame carries the picture and everything needed to place what is found in it, and
// says nothing about how it was obtained. A capture.WindowCapture produces one today; a
// video stream or a remote agent would produce the same shape, and nothing in this package
// or downstream would change.
//
// # Why the frame is identified
//
// Every observation records the frame it came from. That is what makes "why does the
// Director think there is a button there?" answerable a second later, when the screen has
// moved on: the frame id ties a belief to one picture at one instant, and a diagnostic can
// say which. Without it, vision evidence is unattributable in exactly the situation where
// attribution matters most — a detection that turned out to be wrong.

// FrameID identifies one picture.
type FrameID string

// Frame is one picture, with where and when it came from.
type Frame struct {
	// ID identifies this picture, so an observation can be traced back to it.
	ID FrameID
	// Image is the picture itself, in image-local pixels.
	Image image.Image
	// CapturedAt is when the shutter closed — not when the frame was handed on. Every
	// staleness judgement is made against this.
	CapturedAt time.Time
	// Window is what was photographed.
	Window directorapi.Window
	// Bounds is where the image came from, in canonical desktop coordinates.
	Bounds directorapi.Rect
	// Transform converts image-local coordinates to desktop ones.
	Transform capture.Transform
	// Scale is the DPI scaling of the captured surface, 1.0 at 96 DPI.
	Scale float64
	// Target is which validated window generation these pixels belong to, established
	// after the capture returned. See providers.ProvenCapture for why the capture
	// backend.s own before/after ownership check cannot supply it.
	// Nil means provenance could not be established, which is treated as unproven
	// rather than as agreement. Derivative evidence read out of this frame — a scoped
	// OCR label, say — inherits this and can never improve on it.
	Target *directorapi.TargetProvenance
}

// Size is the picture's dimensions, "0x0" when there is no picture.
func (f Frame) Size() string {
	if f.Image == nil {
		return "0x0"
	}
	b := f.Image.Bounds()
	return fmt.Sprintf("%dx%d", b.Dx(), b.Dy())
}

// Source is a thing that produces frames.
//
// Deliberately narrow, and deliberately not "a capture implementation". What the vision
// provider needs is a picture of a window; whether that comes from a desktop capture API, a
// game's own frame buffer, a browser screenshot or a stream is not its business, and an
// interface naming any of those would make it its business.
type Source interface {
	// Frame photographs a window.
	Frame(ctx contextT, window directorapi.Window) (Frame, error)
}

// contextT is context.Context, named locally so the interface above reads as one line.
type contextT = interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

// frameSeq numbers frames within this process.
var frameSeq atomic.Int64

func mintFrameID() FrameID {
	return FrameID(fmt.Sprintf("frame:%d", frameSeq.Add(1)))
}

// ── the frame log ─────────────────────────────────────────────────────────────

// FrameRecord is one frame the provider read, as a diagnostic remembers it.
//
// The PICTURE is not kept. A frame log holding images would be a rolling buffer of
// screenshots of whatever the user was doing — which is the single most sensitive thing
// this system could accumulate, and it would sit in memory for as long as the daemon runs.
// What is kept is the shape: when, how big, which window, and what was made of it.
type FrameRecord struct {
	ID         FrameID              `json:"id"`
	CapturedAt time.Time            `json:"captured_at"`
	WindowID   directorapi.WindowID `json:"window_id,omitempty"`
	Window     string               `json:"window,omitempty"`
	Size       string               `json:"size,omitempty"`
	Scale      float64              `json:"scale,omitempty"`
	Transform  string               `json:"transform,omitempty"`
	Counters   Counters             `json:"counters"`
	Timings    Timings              `json:"timings"`
	Grids      int                  `json:"grids,omitempty"`
}

// FrameLogSize is how many frames are remembered.
//
// Small. This is a diagnostic for "what did the last few passes see?", not a history, and
// a large buffer would keep window titles — which are user content — around long after
// anybody could want them.
const FrameLogSize = 20

// frameLog is a bounded ring of recent frames.
type frameLog struct {
	mu      sync.Mutex
	records []FrameRecord
}

func (l *frameLog) add(r FrameRecord) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r)
	if len(l.records) > FrameLogSize {
		l.records = l.records[len(l.records)-FrameLogSize:]
	}
}

func (l *frameLog) recent() []FrameRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]FrameRecord, len(l.records))
	copy(out, l.records)
	// Newest first: a reader asking about frames wants the one that just happened.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// recordFrame files one frame in the log.
func (p *Provider) recordFrame(f Frame, d Diagnostics) {
	p.frame.add(FrameRecord{
		ID:         f.ID,
		CapturedAt: f.CapturedAt,
		WindowID:   f.Window.ID,
		Window:     f.Window.Title,
		Size:       f.Size(),
		Scale:      f.Scale,
		Transform:  f.Transform.String(),
		Counters:   d.Counters,
		Timings:    d.Timings,
		Grids:      len(d.Grids),
	})
}

// Frames is the recent frame log, newest first.
func (p *Provider) Frames() []FrameRecord { return p.frame.recent() }
