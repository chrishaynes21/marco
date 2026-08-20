"""Run the ONNX artifact over Corpus V2 and emit normalised detections as JSON.

The Go benchmark computes every metric. This only performs inference and normalises
coordinates, which is precisely what the Go plugin will do once wired.
"""
import glob, json, os, sys, time
from ultralytics import YOLO

MODEL = "tools/vision-export/weights/screenparser-1280.onnx"
CONF = float(sys.argv[1]) if len(sys.argv) > 1 else 0.25
OUT = sys.argv[2] if len(sys.argv) > 2 else f"tools/vision-export/dets-{CONF:.2f}.json"

m = YOLO(MODEL, task="detect")
frames = sorted(glob.glob("fixtures/vision/v2/rocketleague/*/*.png"))
out, lat = {}, []
for f in frames:
    t0 = time.perf_counter()
    r = m.predict(f, imgsz=1280, conf=CONF, verbose=False)[0]
    lat.append(time.perf_counter() - t0)
    h, w = r.orig_shape
    dets = []
    if r.boxes is not None:
        for xyxy, c, k in zip(r.boxes.xyxy.tolist(), r.boxes.conf.tolist(), r.boxes.cls.tolist()):
            x1, y1, x2, y2 = xyxy
            dets.append({"class": r.names[int(k)], "confidence": round(float(c), 4),
                         "bounds": {"x": x1/w, "y": y1/h,
                                    "width": (x2-x1)/w, "height": (y2-y1)/h}})
    out[os.path.basename(f)[:-4]] = dets
lat.sort()
json.dump({"model": os.path.basename(MODEL), "conf": CONF,
           "median_ms": round(lat[len(lat)//2]*1000, 1),
           "p95_ms": round(lat[int(len(lat)*0.95)]*1000, 1),
           "max_ms": round(lat[-1]*1000, 1),
           "frames": out}, open(OUT, "w"), indent=1)
print(f"conf={CONF} frames={len(frames)} dets={sum(len(v) for v in out.values())} "
      f"median={lat[len(lat)//2]*1000:.0f}ms p95={lat[int(len(lat)*.95)]*1000:.0f}ms -> {OUT}")
