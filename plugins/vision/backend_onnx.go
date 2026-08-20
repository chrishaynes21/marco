//go:build onnxvision

// This file is compiled only with `-tags onnxvision`. It is the ONLY thing in the plugin
// that depends on an external module — the ONNX Runtime binding — so the default build
// (backend_null.go) stays dependency-free. The binding needs cgo (like the voice plugin),
// but loads the onnxruntime shared library DYNAMICALLY, so no onnxruntime source/headers
// are needed at build time and it works on Windows with Mingw. Enable it with:
//
//	go -C plugins/vision get github.com/yalue/onnxruntime_go
//	CGO_ENABLED=1 go -C plugins/vision build -tags onnxvision -o vision.exe .
//
// At run time it needs the ONNX Runtime shared library ($MARCO_ONNXRUNTIME → onnxruntime
// .dll / .so / .dylib) and a YOLOv8-style detection model ($MARCO_VISION_MODEL → .onnx).
// $MARCO_VISION_INPUT / $MARCO_VISION_OUTPUT name the model's tensors (YOLOv8 exports use
// "images" / "output0", the defaults).

package main

import (
	"fmt"
	"image"
	"os"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var initOnce sync.Once
var initErr error

// onnxDetector runs a YOLOv8-style UI-element detector through ONNX Runtime. The session
// isn't goroutine-safe, so Detect is serialised; the bridge processes one request at a
// time anyway.
type onnxDetector struct {
	mu       sync.Mutex
	sess     *ort.DynamicAdvancedSession
	inName   string
	outName  string
	side     int
	conf     float32
	iou      float32
	labels   []string
	labelsBy labelSource
	ready    bool
	loadErr  error
}

func newDetector() detector {
	labels, from := loadLabels()
	d := &onnxDetector{
		side:     envInt("MARCO_VISION_SIZE", 640),
		conf:     float32(envFloat("MARCO_VISION_CONF", 0.25)),
		iou:      float32(envFloat("MARCO_VISION_IOU", 0.45)),
		labels:   labels,
		labelsBy: from,
		inName:   envStr("MARCO_VISION_INPUT", "images"),
		outName:  envStr("MARCO_VISION_OUTPUT", "output0"),
	}
	if err := d.load(); err != nil {
		d.loadErr = err // surfaced on the first Detect; Ready() stays false
	}
	return d
}

// load points the binding at the runtime shared library, initialises the (process-global)
// environment once, and opens a dynamic session on the model.
func (d *onnxDetector) load() error {
	model := strings.TrimSpace(os.Getenv("MARCO_VISION_MODEL"))
	if model == "" {
		return fmt.Errorf("MARCO_VISION_MODEL not set (path to the .onnx detector)")
	}
	initOnce.Do(func() {
		if lib := strings.TrimSpace(os.Getenv("MARCO_ONNXRUNTIME")); lib != "" {
			ort.SetSharedLibraryPath(lib)
		}
		initErr = ort.InitializeEnvironment()
	})
	if initErr != nil {
		return fmt.Errorf("onnxruntime init (set $MARCO_ONNXRUNTIME to the shared lib): %w", initErr)
	}
	sess, err := ort.NewDynamicAdvancedSession(model, []string{d.inName}, []string{d.outName}, nil)
	if err != nil {
		return fmt.Errorf("open model %s: %w", model, err)
	}
	d.sess = sess
	d.adoptModelNames(model)
	d.ready = true
	return nil
}

// adoptModelNames replaces GUESSED labels with the ones the model carries.
//
// Ultralytics exports embed `names` — the authoritative class list, in class-index order.
// Using it closes the gap that made a one-class icon detector announce 56 desktop icons as
// buttons: the built-in default list starts with "button", and nothing had ever asked the
// model what its single class actually was.
//
// An EXPLICIT list still wins. Someone who set $MARCO_VISION_LABELS or wrote a labels.txt
// is correcting the model on purpose, and silently overriding them would make that
// impossible.
func (d *onnxDetector) adoptModelNames(model string) {
	if d.labelsBy != labelsFromDefault {
		return
	}
	md, err := ort.GetModelMetadata(model)
	if err != nil {
		return
	}
	defer md.Destroy()

	raw, _, err := md.LookupCustomMetadataMap("names")
	if err != nil {
		return
	}
	if names := parseNames(raw); len(names) > 0 {
		d.labels = names
		d.labelsBy = labelsFromModel
	}
}

func (d *onnxDetector) Ready() bool { return d.ready }

// Detect letterboxes the image, runs the model, and decodes + NMS-filters the output into
// labelled boxes in the ORIGINAL image's coordinates.
func (d *onnxDetector) Detect(img *image.RGBA) ([]Element, error) {
	if !d.ready {
		if d.loadErr != nil {
			return nil, d.loadErr
		}
		return nil, fmt.Errorf("vision detector not ready")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	lb, scale, padX, padY := letterbox(img, d.side)
	input := toCHW(lb)
	inT, err := ort.NewTensor(ort.NewShape(1, 3, int64(d.side), int64(d.side)), input)
	if err != nil {
		return nil, fmt.Errorf("vision input tensor: %w", err)
	}
	defer inT.Destroy()

	outputs := []ort.Value{nil} // nil → the binding allocates the output tensor
	if err := d.sess.Run([]ort.Value{inT}, outputs); err != nil {
		return nil, fmt.Errorf("vision inference: %w", err)
	}
	defer outputs[0].Destroy()

	out, ok := outputs[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("vision output is not float32 (got %T)", outputs[0])
	}
	boxes := nms(decode(out.GetData(), []int64(out.GetShape()), d.conf), d.iou)
	b := img.Bounds()
	return toElements(boxes, scale, padX, padY, d.labels, b.Dx(), b.Dy()), nil
}
