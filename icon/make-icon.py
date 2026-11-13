"""Draws Gocast's icons: a white terminal cursor block.

    python3 icon/make-icon.py   (needs Pillow)

Writes, next to this script:
  icon.png          1024 app icon (rounded square on a transparent margin), for mygo build
  AppIcon.icon/     Icon Composer icon (macOS 26+): solid fill + one glyph layer
  tray.png          36 px (18 pt @2x) menu bar template, black on transparent
  logo-template.png 40 px (20 pt @2x) search bar logo, black on transparent
"""
import json, math, os
from PIL import Image, ImageDraw, ImageFilter

HERE = os.path.dirname(os.path.abspath(__file__))
BG_TOP, BG_BOTTOM = (0x24, 0x24, 0x27), (0x12, 0x12, 0x14)
SS = 4  # supersampling

def squircle(cx, cy, half, n=5.0, steps=720):
    pts = []
    for i in range(steps):
        t = 2 * math.pi * i / steps
        c, s = math.cos(t), math.sin(t)
        pts.append((cx + half * math.copysign(abs(c) ** (2 / n), c),
                    cy + half * math.copysign(abs(s) ** (2 / n), s)))
    return pts

def rrect(d, box, r, fill):
    d.rounded_rectangle(box, r, fill=fill)

def app_icon(size=1024):
    S = size * SS
    k = S / 1024
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    # shadow (macOS grid: y +12, blur ~28, 30%)
    sh = Image.new("L", (S, S), 0)
    ImageDraw.Draw(sh).polygon(squircle(512 * k, 524 * k, 412 * k), fill=int(255 * .30))
    sh = sh.filter(ImageFilter.GaussianBlur(14 * k))
    img.paste((0, 0, 0, 255), (0, 0), sh)
    # tile: vertical gradient clipped to squircle
    grad = Image.new("RGBA", (S, S))
    gd = ImageDraw.Draw(grad)
    for y in range(S):
        t = min(max((y - 100 * k) / (824 * k), 0), 1)
        gd.line([(0, y), (S, y)], fill=tuple(round(a + (b - a) * t) for a, b in zip(BG_TOP, BG_BOTTOM)) + (255,))
    mask = Image.new("L", (S, S), 0)
    ImageDraw.Draw(mask).polygon(squircle(512 * k, 512 * k, 412 * k), fill=255)
    img.paste(grad, (0, 0), mask)
    # hairline edge so it holds on dark docks
    edge = Image.new("L", (S, S), 0)
    ed = ImageDraw.Draw(edge)
    ed.polygon(squircle(512 * k, 512 * k, 412 * k), fill=int(255 * .14))
    ed.polygon(squircle(512 * k, 512 * k, 408 * k), fill=0)
    img.paste((255, 255, 255, 255), (0, 0), edge)
    # cursor block: 240 x 440, r 40, optically centred
    d = ImageDraw.Draw(img)
    rrect(d, (392 * k, 292 * k, 632 * k, 732 * k), 40 * k, (255, 255, 255, 255))
    return img.resize((size, size), Image.LANCZOS)

def template(pt):
    """Squircle outline with the cursor inside, black on transparent, @2x."""
    px = pt * 2
    S = px * SS
    u = S / 18  # design on an 18-unit grid
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    m = Image.new("L", (S, S), 0)
    d = ImageDraw.Draw(m)
    d.polygon(squircle(9 * u, 9 * u, 7.6 * u), fill=255)
    d.polygon(squircle(9 * u, 9 * u, 6.05 * u), fill=0)
    d.rounded_rectangle((7.25 * u, 5.1 * u, 10.75 * u, 12.9 * u), 0.8 * u, fill=255)
    img.paste((0, 0, 0, 255), (0, 0), m)
    return img.resize((px, px), Image.LANCZOS)

big = app_icon(1024)
big.save(os.path.join(HERE, "icon.png"))
template(18).save(os.path.join(HERE, "tray.png"))
template(20).save(os.path.join(HERE, "logo-template.png"))

# Icon Composer: the system draws shape, glass, shadow and dark/tinted looks.
ic = os.path.join(HERE, "AppIcon.icon")
os.makedirs(os.path.join(ic, "Assets"), exist_ok=True)
open(os.path.join(ic, "Assets", "cursor.svg"), "w").write(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">'
    '<rect x="363" y="238" width="298" height="548" rx="50" fill="#FFFFFF"/></svg>\n')
json.dump({
    "fill": {"linear-gradient": ["srgb:0.14118,0.14118,0.15294,1.00000",
                                  "srgb:0.07059,0.07059,0.07843,1.00000"]},
    "groups": [{"layers": [{"glass": False, "image-name": "cursor.svg", "name": "cursor"}],
                "shadow": {"kind": "neutral", "opacity": 0.5},
                "translucency": {"enabled": False, "value": 0}}],
    "supported-platforms": {"squares": ["macOS"]},
}, open(os.path.join(ic, "icon.json"), "w"), indent=2)
print("ok")
