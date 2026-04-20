#!/usr/bin/env python3
"""Fuguebot 의존성 그래프 생성기 — make fuguebot-progress에서 호출됨."""
import glob
import os
import subprocess


def task_info(name):
    path = f"openspec/changes/{name}/tasks.md"
    if not os.path.exists(path):
        # 아카이브된 change는 `archive/<date>-<name>/tasks.md` 형태로 저장된다.
        matches = sorted(glob.glob(f"openspec/changes/archive/*-{name}/tasks.md"))
        if not matches:
            return "?", 0, 0
        path = matches[-1]
    content = open(path).read()
    done = content.count("- [x]")
    inp  = content.count("- [~]")
    todo = content.count("- [ ]")
    total = done + inp + todo
    if total == 0:
        return "?", 0, 0
    if done == total:
        return "x", done, total
    if done > 0 or inp > 0:
        return "~", done, total
    return " ", done, total


def node_fill(st):
    return {"x": "#0d2818", " ": "#161b22", "~": "#2d1a00", "?": "#161b22"}[st]


def node_stroke(st):
    return {"x": "#3fb950", " ": "#30363d", "~": "#d29922", "?": "#484f58"}[st]


def pct_label(done, total):
    if total == 0:
        return "?"
    return f"{done}/{total} ({done * 100 // total}%)"


# ── 레이아웃 상수 ──────────────────────────────────────────────
W, H   = 880, 540
NW, NH = 196, 72   # node width, height

# (x, y, w, h)
NODES = {
    "prereqs": (160, 28,  500, 78),
    "psc":     ( 20, 178, NW, NH),
    "pwb":     ( 20, 322, NW, NH),
    "hsf":     (282, 178, NW, NH),
    "hpd":     (532, 178, NW, NH),
    "hsc":     (407, 322, NW, NH),
    "hwb":     (407, 440, NW, NH),
    "hsfc":    (660, 440, NW, NH),
}


def cx(k):  x, y, w, h = NODES[k]; return x + w // 2
def top(k): x, y, w, h = NODES[k]; return y
def bot(k): x, y, w, h = NODES[k]; return y + h


def arrow(x1, y1, x2, y2):
    return (f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" '
            f'stroke="#484f58" stroke-width="1.8" marker-end="url(#ar)"/>')


def draw_node(key, info, line1, line2=None):
    st, done, total = info
    x, y, w, h = NODES[key]
    fill   = node_fill(st)
    stroke = node_stroke(st)
    pct    = pct_label(done, total)
    tcx    = x + w // 2
    out = [
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="8" '
        f'fill="{fill}" stroke="{stroke}" stroke-width="1.5"/>'
    ]
    if line2:
        out += [
            f'<text x="{tcx}" y="{y+22}" text-anchor="middle" '
            f'fill="#c9d1d9" font-size="11" font-family="monospace">{line1}</text>',
            f'<text x="{tcx}" y="{y+38}" text-anchor="middle" '
            f'fill="#c9d1d9" font-size="11" font-family="monospace">{line2}</text>',
            f'<text x="{tcx}" y="{y+58}" text-anchor="middle" '
            f'fill="{stroke}" font-size="10" font-family="monospace">{pct}</text>',
        ]
    else:
        out += [
            f'<text x="{tcx}" y="{y+28}" text-anchor="middle" '
            f'fill="#c9d1d9" font-size="11" font-family="monospace">{line1}</text>',
            f'<text x="{tcx}" y="{y+52}" text-anchor="middle" '
            f'fill="{stroke}" font-size="10" font-family="monospace">{pct}</text>',
        ]
    return "\n".join(out)


# ── SVG 조립 ──────────────────────────────────────────────────
parts = []

parts.append(f'<rect width="{W}" height="{H}" fill="#0d1117"/>')
parts.append(
    '<defs><marker id="ar" markerWidth="10" markerHeight="7" '
    'refX="9" refY="3.5" orient="auto">'
    '<polygon points="0 0,10 3.5,0 7" fill="#484f58"/>'
    '</marker></defs>'
)

# 병렬 실행 가능 박스 (dashed)
bx1 = NODES["psc"][0] - 14
by1 = NODES["psc"][1] - 18
bx2 = NODES["hpd"][0] + NODES["hpd"][2] + 14
by2 = NODES["psc"][1] + NODES["psc"][3] + 18
parts.append(
    f'<rect x="{bx1}" y="{by1}" width="{bx2-bx1}" height="{by2-by1}" rx="6" '
    f'fill="#161b22" stroke="#21262d" stroke-width="1.5" stroke-dasharray="6,3"/>'
)
parts.append(
    f'<text x="{bx1+8}" y="{by1-5}" fill="#3d444d" '
    f'font-size="10" font-family="monospace">▶ 병렬 실행 가능</text>'
)

# 전제조건 박스
px, py, pw, ph = NODES["prereqs"]
pcx = px + pw // 2
parts += [
    f'<rect x="{px}" y="{py}" width="{pw}" height="{ph}" rx="8" '
    f'fill="#0f2010" stroke="#3fb950" stroke-width="1.5"/>',
    f'<text x="{pcx}" y="{py+26}" text-anchor="middle" fill="#3fb950" '
    f'font-size="13" font-weight="bold" font-family="monospace">전제조건 완료 (9개 archived)</text>',
    f'<text x="{pcx}" y="{py+48}" text-anchor="middle" fill="#238636" '
    f'font-size="10" font-family="monospace">'
    f'frontier-table · claim-api · retry-backoff · host-token-bucket</text>',
    f'<text x="{pcx}" y="{py+64}" text-anchor="middle" fill="#238636" '
    f'font-size="10" font-family="monospace">'
    f'pioneer-snapshot · link-filter · harvester-image-cache (×2)</text>',
]

# 전제조건 → row1
pre_bot = py + ph
parts += [
    arrow(cx("prereqs") - 100, pre_bot, cx("psc"), top("psc")),
    arrow(cx("prereqs"),       pre_bot, cx("hsf"), top("hsf")),
    arrow(cx("prereqs") + 100, pre_bot, cx("hpd"), top("hpd")),
]

# row1 노드
psc = task_info("pioneer-scheduler-consumer")
hsf = task_info("harvester-snapshot-first-fetch")
hpd = task_info("harvester-pin-document")
parts += [
    draw_node("psc", psc, "pioneer-scheduler-", "consumer"),
    draw_node("hsf", hsf, "harvester-snapshot-", "first-fetch"),
    draw_node("hpd", hpd, "harvester-pin-", "document"),
]

# row1 → row2
parts += [
    arrow(cx("psc"), bot("psc"), cx("pwb"), top("pwb")),
    arrow(cx("hsf"), bot("hsf"), cx("hsc"), top("hsc")),
    arrow(cx("hpd"), bot("hpd"), cx("hsc"), top("hsc")),
]

# row2 노드
pwb = task_info("pioneer-worker-budget")
hsc = task_info("harvester-scheduler-consumer")
parts += [
    draw_node("pwb", pwb, "pioneer-worker-", "budget"),
    draw_node("hsc", hsc, "harvester-scheduler-", "consumer"),
]

# row2 → row3
hwb = task_info("harvester-worker-budget")
parts.append(arrow(cx("hsc"), bot("hsc"), cx("hwb"), top("hwb")))
parts.append(draw_node("hwb", hwb, "harvester-worker-", "budget"))

# harvester-snapshot-first-fetchconsumer (integration spec-delta)
# prereqs: hsc + hsf. hsf는 hsc를 거쳐 transitive 의존하므로 arrow는 hsc 하나만.
hsfc = task_info("harvester-snapshot-first-fetchconsumer")
parts += [
    arrow(cx("hsc"), bot("hsc"), cx("hsfc"), top("hsfc")),
    draw_node("hsfc", hsfc, "harvester-snapshot-", "first-fetchconsumer"),
]

# ── HTML ──────────────────────────────────────────────────────
svg_body = "\n  ".join(parts)

html = f"""<!DOCTYPE html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <title>Fuguebot 의존성 그래프</title>
  <style>
    *, *::before, *::after {{ box-sizing: border-box; margin: 0; padding: 0; }}
    body {{
      background: #0d1117;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      font-family: 'SF Mono', 'Fira Code', 'Menlo', monospace;
      color: #c9d1d9;
      gap: 20px;
      padding: 40px;
    }}
    h1 {{ font-size: 15px; color: #8b949e; letter-spacing: .04em; }}
    svg {{ border-radius: 12px; border: 1px solid #21262d; }}
    .legend {{
      display: flex; gap: 20px; font-size: 11px; color: #6e7681;
      align-items: center;
    }}
    .dot {{
      width: 11px; height: 11px; border-radius: 3px;
      display: inline-block; margin-right: 5px; vertical-align: middle;
    }}
  </style>
</head>
<body>
  <h1>Fuguebot Pioneer / Harvester — 의존성 그래프</h1>
  <svg width="{W}" height="{H}" xmlns="http://www.w3.org/2000/svg">
    {svg_body}
  </svg>
  <div class="legend">
    <span><span class="dot" style="background:#161b22;outline:1.5px solid #30363d"></span>미착수</span>
    <span><span class="dot" style="background:#2d1a00;outline:1.5px solid #d29922"></span>진행중</span>
    <span><span class="dot" style="background:#0d2818;outline:1.5px solid #3fb950"></span>완료</span>
    <span style="border: 1px dashed #3d444d; padding: 2px 8px; border-radius: 4px; font-size: 10px;">
      점선 박스 = 동시 착수 가능
    </span>
  </div>
</body>
</html>"""

out = "fuguebot-graph.html"
with open(out, "w") as f:
    f.write(html)

subprocess.run(["open", out])
print(f"✓ {out} 생성 — 브라우저에서 열림")
