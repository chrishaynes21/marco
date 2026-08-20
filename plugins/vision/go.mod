module github.com/chaynes-simpleclouds/marco/plugins/vision

go 1.26.2

// The vision resolver reuses the engine's cross-platform screen capture
// (internal/screen) but lives in its own module so its heavy dependency — an ONNX
// Runtime UI-element detector — never becomes a dependency of the zero-dep engine.
// The engine imports only the stdlib, so requiring it here pulls in no external
// packages. The ONNX backend itself is behind the `onnxvision` build tag and is the
// ONLY thing that needs an external module (github.com/yalue/onnxruntime_go, cgo — it
// loads onnxruntime.dll dynamically); the default build uses a null detector and stays
// dependency-free, so the plugin builds and tests everywhere without the runtime or a model.
require github.com/chaynes-simpleclouds/marco v0.0.0

require github.com/yalue/onnxruntime_go v1.32.0 // indirect

replace github.com/chaynes-simpleclouds/marco => ../..
