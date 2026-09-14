"""Draw a macOS-style window around a recorded terminal: gradient backdrop,
soft drop shadow, rounded window, title bar with traffic lights. Writes
chrome.png and prints the content offset as "x y" for ffmpeg's overlay."""
import sys
from PIL import Image, ImageDraw, ImageFilter, ImageFont

content_w, content_h, title, out = int(sys.argv[1]), int(sys.argv[2]), sys.argv[3], sys.argv[4]

MARGIN = 64          # backdrop around the window
BAR = 44             # title bar height
PAD = 18             # inner padding between window edge and terminal
RADIUS = 14
BASE = (30, 30, 46)          # Catppuccin Mocha base — matches the terminal
BAR_FILL = (24, 24, 37)      # Mocha mantle
win_w, win_h = content_w + 2 * PAD, content_h + BAR + PAD
W, H = win_w + 2 * MARGIN, win_h + 2 * MARGIN

# Diagonal three-stop gradient: violet -> indigo -> teal.
stops = [(0.0, (109, 40, 217)), (0.55, (49, 46, 129)), (1.0, (13, 148, 136))]
grad = Image.new("RGB", (W, H))
px = grad.load()
for y in range(H):
    for x in range(W):
        t = (x / W) * 0.6 + (y / H) * 0.4
        for (t0, c0), (t1, c1) in zip(stops, stops[1:]):
            if t <= t1:
                f = (t - t0) / (t1 - t0)
                px[x, y] = tuple(int(a + (b - a) * f) for a, b in zip(c0, c1))
                break
canvas = grad.convert("RGBA")

# Soft shadow under the window.
shadow = Image.new("RGBA", (W, H), (0, 0, 0, 0))
ImageDraw.Draw(shadow).rounded_rectangle(
    (MARGIN, MARGIN + 18, MARGIN + win_w, MARGIN + win_h + 18), RADIUS, fill=(0, 0, 0, 150))
canvas = Image.alpha_composite(canvas, shadow.filter(ImageFilter.GaussianBlur(26)))

# Window body: frosted glass — the backdrop behind the window, blurred hard and
# darkened, so the terminal (whose own background is keyed out at compositing)
# reads as a translucent macOS window rather than a flat default fill.
box = (MARGIN, MARGIN, MARGIN + win_w, MARGIN + win_h)
glass = grad.crop(box).filter(ImageFilter.GaussianBlur(60))
glass = Image.blend(glass, Image.new("RGB", glass.size, (14, 12, 28)), 0.80).convert("RGBA")
mask = Image.new("L", glass.size, 0)
ImageDraw.Draw(mask).rounded_rectangle((0, 0, win_w, win_h), RADIUS, fill=255)
canvas.paste(glass, (MARGIN, MARGIN), mask)
d = ImageDraw.Draw(canvas)
d.rounded_rectangle((MARGIN, MARGIN, MARGIN + win_w, MARGIN + BAR), RADIUS, fill=BAR_FILL)
d.rectangle((MARGIN, MARGIN + BAR - RADIUS, MARGIN + win_w, MARGIN + BAR), fill=BAR_FILL)
d.line((MARGIN, MARGIN + BAR, MARGIN + win_w, MARGIN + BAR), fill=(49, 50, 68), width=1)
d.rounded_rectangle(box, RADIUS, outline=(69, 71, 90), width=1)

# Traffic lights.
cy = MARGIN + BAR // 2
for i, color in enumerate([(255, 95, 87), (254, 188, 46), (40, 200, 64)]):
    cx = MARGIN + 22 + i * 22
    d.ellipse((cx - 7, cy - 7, cx + 7, cy + 7), fill=color)

# Centered title.
try:
    font = ImageFont.truetype("/System/Library/Fonts/SFNS.ttf", 15)
except OSError:
    font = ImageFont.truetype("/System/Library/Fonts/Menlo.ttc", 14)
tw = d.textlength(title, font=font)
d.text((MARGIN + (win_w - tw) / 2, cy - 9), title, font=font, fill=(166, 173, 200))

canvas.convert("RGB").save(out)
print(MARGIN + PAD, MARGIN + BAR)
