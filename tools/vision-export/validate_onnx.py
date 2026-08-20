"""Prove the ONNX artifact agrees with the PyTorch source before anything is benchmarked.

A conversion that silently changed behaviour would make every downstream number a measurement
of the exporter rather than of the model.
"""
import glob, numpy as np
from ultralytics import YOLO

FRAMES = sorted(glob.glob("fixtures/vision/v2/rocketleague/*/*.png"))[:6]
pt = YOLO("tools/vision-export/weights/best.pt")
ox = YOLO("tools/vision-export/weights/screenparser-1280.onnx", task="detect")

def boxes(res):
    b = res[0].boxes
    if b is None or len(b) == 0:
        return np.zeros((0, 6))
    return np.concatenate([b.xyxy.cpu().numpy(),
                           b.conf.cpu().numpy()[:, None],
                           b.cls.cpu().numpy()[:, None]], axis=1)

def iou(a, b):
    x1 = max(a[0], b[0]); y1 = max(a[1], b[1])
    x2 = min(a[2], b[2]); y2 = min(a[3], b[3])
    if x2 <= x1 or y2 <= y1: return 0.0
    i = (x2-x1)*(y2-y1)
    ua = (a[2]-a[0])*(a[3]-a[1]) + (b[2]-b[0])*(b[3]-b[1]) - i
    return i/ua if ua > 0 else 0.0

tot_p = tot_o = matched = cls_agree = 0
ious, dconf = [], []
for f in FRAMES:
    P = boxes(pt.predict(f, imgsz=1280, conf=0.25, verbose=False))
    O = boxes(ox.predict(f, imgsz=1280, conf=0.25, verbose=False))
    tot_p += len(P); tot_o += len(O)
    used = set()
    for p in P:
        best, bj = 0.0, -1
        for j, o in enumerate(O):
            if j in used: continue
            v = iou(p[:4], o[:4])
            if v > best: best, bj = v, j
        if bj >= 0 and best >= 0.5:
            used.add(bj); matched += 1; ious.append(best)
            dconf.append(abs(p[4] - O[bj][4]))
            if int(p[5]) == int(O[bj][5]): cls_agree += 1
    print(f"  {f.split(chr(92))[-1][:38]:40s} pt={len(P):3d} onnx={len(O):3d}")

print(f"\nframes            {len(FRAMES)}")
print(f"pytorch dets      {tot_p}")
print(f"onnx dets         {tot_o}")
print(f"matched (IoU>=.5) {matched}")
if matched:
    print(f"class agreement   {cls_agree}/{matched} = {100*cls_agree/matched:.1f}%")
    print(f"median IoU        {np.median(ious):.4f}")
    print(f"max conf delta    {max(dconf):.5f}")
    print(f"mean conf delta   {np.mean(dconf):.5f}")
