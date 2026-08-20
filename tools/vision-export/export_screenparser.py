"""One-time ScreenParser .pt -> .onnx conversion.

DEVELOPMENT TOOLING. Python, PyTorch and Ultralytics exist to produce the .onnx artifact
Marco runs; none of them is a Marco runtime dependency and none is ever shipped. See
docs/director-vision-ui-detector-decision.md for why the export step exists at all: no
surveyed UI detector publishes an ONNX artifact.

The checkpoint is a pickle and is deserialised ONLY here, never inside Director, the
benchmark process, or the overlay.
"""
import json, os, sys
from ultralytics import YOLO

WEIGHTS = "tools/vision-export/weights/best.pt"
IMGSZ = 1280          # ScreenParser's documented training resolution. Not shrunk for speed
                      # before a correct baseline exists.
OUT = "tools/vision-export/weights/screenparser-1280.onnx"

model = YOLO(WEIGHTS)
names = model.names
print("classes:", len(names))

path = model.export(format="onnx", imgsz=IMGSZ, batch=1, half=False,
                    dynamic=False, simplify=False, nms=False, opset=17)
print("exported:", path)
if os.path.abspath(path) != os.path.abspath(OUT):
    os.replace(path, OUT)

json.dump({str(k): v for k, v in names.items()},
          open("tools/vision-export/screenparser-classes.json", "w"), indent=2)
print("wrote class map")
