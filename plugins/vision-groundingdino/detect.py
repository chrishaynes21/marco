"""Grounding DINO as a Marco vision benchmark backend.

One frame in, normalised detections out. It is a DETECTOR and nothing else: it is handed a
closed vocabulary built by the Go side, it returns boxes with labels drawn from that
vocabulary, and it never describes, explains, or suggests. An open-vocabulary model asked a
loose question answers in prose, and prose has no place in a semantic pipeline.

Protocol — one JSON object on stdin, one on stdout:

    in   {"image": "<base64 png>", "prompt": "button . menu . panel .",
          "threshold": 0.3, "max_detections": 120, "model": "IDEA-Research/grounding-dino-tiny"}
    out  {"detections": [{"label": "button", "class_id": "button",
                          "confidence": 0.86,
                          "bounds": {"x": 0.25, "y": 0.18, "width": 0.5, "height": 0.62}}],
          "load_seconds": 4.2}
    err  {"error": "...", "error_kind": "model_missing|load_failed|runtime|inference"}

Coordinates are NORMALISED to 0..1. The plugin never learns the frame's pixel size in the
caller's coordinate space and so cannot get placement wrong on its behalf; the Go side owns
that conversion, as it does for every other detector.

Errors are reported as JSON with a KIND rather than raised as a traceback, because "the
weights are missing" and "inference crashed" send a reader to completely different places
and the benchmark reports them separately.
"""

import base64
import io
import json
import sys
import time

DEFAULT_MODEL = "IDEA-Research/grounding-dino-tiny"


def fail(message, kind="inference"):
    """Report a failure as data, then stop."""
    json.dump({"error": str(message), "error_kind": kind}, sys.stdout)
    sys.stdout.flush()
    sys.exit(0)


def main():
    try:
        request = json.load(sys.stdin)
    except Exception as exc:  # noqa: BLE001 - a malformed request is a protocol fault
        fail(f"unreadable request: {exc}", "runtime")

    encoded = request.get("image") or ""
    prompt = (request.get("prompt") or "").strip()
    threshold = float(request.get("threshold") or 0.3)
    max_detections = int(request.get("max_detections") or 120)
    model_id = request.get("model") or DEFAULT_MODEL

    if not encoded:
        fail("no image was supplied", "runtime")
    if not prompt:
        # Refused rather than defaulted. A prompt the Go side did not build is a prompt
        # nobody versioned, and results under it would not be comparable with anything.
        fail("no vocabulary prompt was supplied", "runtime")

    try:
        from PIL import Image
    except Exception as exc:  # noqa: BLE001
        fail(f"Pillow is not installed: {exc}", "runtime")

    try:
        import torch
        from transformers import AutoProcessor, AutoModelForZeroShotObjectDetection
    except Exception as exc:  # noqa: BLE001
        fail(f"the Python environment is missing a required package: {exc}", "runtime")

    try:
        raw = base64.b64decode(encoded)
        image = Image.open(io.BytesIO(raw)).convert("RGB")
    except Exception as exc:  # noqa: BLE001
        fail(f"the image could not be decoded: {exc}", "runtime")

    # Model load is TIMED and reported separately. Folding startup into the first frame's
    # latency would make a slow-loading model look like a slow-inferring one, and the
    # benchmark scores those differently — startup is paid once, inference every frame.
    load_start = time.perf_counter()
    try:
        processor = AutoProcessor.from_pretrained(model_id)
        model = AutoModelForZeroShotObjectDetection.from_pretrained(model_id)
    except Exception as exc:  # noqa: BLE001
        text = str(exc)
        kind = "model_missing" if (
            "not a local folder" in text
            or "Repository Not Found" in text
            or "Can't load" in text
            or "does not appear to have" in text
            or "Connection" in text
        ) else "load_failed"
        fail(f"the model could not be loaded: {text}", kind)
    load_seconds = time.perf_counter() - load_start

    device = "cuda" if torch.cuda.is_available() else "cpu"
    try:
        model = model.to(device)
    except Exception:  # noqa: BLE001 - a device move failure is not fatal; stay on CPU
        device = "cpu"

    try:
        inputs = processor(images=image, text=prompt, return_tensors="pt").to(device)
        with torch.no_grad():
            outputs = model(**inputs)
        results = processor.post_process_grounded_object_detection(
            outputs,
            inputs.input_ids,
            threshold=threshold,
            text_threshold=threshold,
            target_sizes=[image.size[::-1]],
        )
    except Exception as exc:  # noqa: BLE001
        fail(f"inference failed: {exc}", "inference")

    width, height = image.size
    detections = []
    if results:
        first = results[0]
        boxes = first.get("boxes", [])
        scores = first.get("scores", [])
        labels = first.get("text_labels", first.get("labels", []))

        for box, score, label in zip(boxes, scores, labels):
            if len(detections) >= max_detections:
                break
            x0, y0, x1, y1 = (float(v) for v in box.tolist())
            if x1 <= x0 or y1 <= y0 or width <= 0 or height <= 0:
                continue
            # Normalised, and clamped to the frame. A box the model placed slightly
            # outside is a rounding artefact; one placed far outside is rejected by the
            # Go side, which is where that judgement belongs.
            detections.append({
                "label": str(label),
                "class_id": str(label),
                "confidence": float(score),
                "bounds": {
                    "x": max(0.0, min(1.0, x0 / width)),
                    "y": max(0.0, min(1.0, y0 / height)),
                    "width": max(0.0, min(1.0, (x1 - x0) / width)),
                    "height": max(0.0, min(1.0, (y1 - y0) / height)),
                },
            })

    json.dump({
        "detections": detections,
        "load_seconds": load_seconds,
        "device": device,
    }, sys.stdout)
    sys.stdout.flush()


if __name__ == "__main__":
    main()
