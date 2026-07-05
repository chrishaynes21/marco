module github.com/chaynes-simpleclouds/marco/plugins/ocr

go 1.26.2

// The OCR resolver reuses the engine's cross-platform screen capture
// (internal/screen) but lives in its own module so its OCR dependency (the
// tesseract CLI / a future native backend) never becomes a dependency of the
// zero-dep engine. The engine itself imports only the stdlib, so requiring it
// here pulls in no external packages.
require github.com/chaynes-simpleclouds/marco v0.0.0

replace github.com/chaynes-simpleclouds/marco => ../..
