#!/usr/bin/env python3
"""Generate the shioriai2api logo and social assets.

Design: a rounded squircle tile with a deep indigo-to-black gradient and a
stylised "shiori" mark -- a bookmark (shiori, 栞) whose ribbon doubles as a
chat bubble tail -- plus a clean wordmark. Everything is deterministic so the
checked-in PNGs can be regenerated.

    python3 scripts/make_assets.py
"""
from __future__ import annotations

import os
from PIL import Image, ImageDraw, ImageFont, ImageFilter

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
ASSETS = os.path.join(ROOT, "assets")
os.makedirs(ASSETS, exist_ok=True)

BG_TOP = (26, 16, 48)
BG_BOTTOM = (8, 6, 20)
VIOLET = (150, 108, 255)
PINK = (255, 110, 175)
WHITE = (246, 245, 255)
MUTED = (162, 158, 196)
PALE = (206, 200, 236)


def find_font(size: int, bold: bool = True) -> ImageFont.FreeTypeFont:
    candidates = [
        ("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf" if bold
         else "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"),
        ("/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf" if bold
         else "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"),
    ]
    for path in candidates:
        if os.path.exists(path):
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def lerp(a, b, t):
    return tuple(int(a[i] + (b[i] - a[i]) * t) for i in range(3))


def vertical_gradient(size, top, bottom):
    w, h = size
    strip = Image.new("RGB", (1, h))
    for y in range(h):
        strip.putpixel((0, y), lerp(top, bottom, y / max(1, h - 1)))
    return strip.resize((w, h))


def rounded_mask(size, radius):
    w, h = size
    mask = Image.new("L", (w, h), 0)
    ImageDraw.Draw(mask).rounded_rectangle([0, 0, w - 1, h - 1], radius=radius, fill=255)
    return mask


def draw_glow(canvas, center, radius, color, alpha=90, blur=60):
    glow = Image.new("RGBA", canvas.size, (0, 0, 0, 0))
    ImageDraw.Draw(glow).ellipse(
        [center[0] - radius, center[1] - radius, center[0] + radius, center[1] + radius],
        fill=color + (alpha,),
    )
    canvas.alpha_composite(glow.filter(ImageFilter.GaussianBlur(blur)))


def draw_mark(canvas, box, lw):
    """A bookmark (shiori) that reads as a chat bubble with a ribbon tail."""
    d = ImageDraw.Draw(canvas)
    x0, y0, x1, y1 = box
    w, h = x1 - x0, y1 - y0
    violet, pink = VIOLET + (255,), PINK + (255,)

    # Chat bubble body (rounded rect) with a small tail.
    bx0, by0, bx1, by1 = x0 + w * 0.08, y0 + h * 0.14, x1 - w * 0.08, y0 + h * 0.74
    d.rounded_rectangle([bx0, by0, bx1, by1], radius=int(min(w, h) * 0.16),
                        outline=violet, width=lw)
    # Tail (bottom-left pointing down).
    d.polygon([(x0 + w * 0.26, by1 - lw * 0.2),
               (x0 + w * 0.46, by1 - lw * 0.2),
               (x0 + w * 0.24, y1 - h * 0.06)], fill=violet)

    # Bookmark ribbon inside the bubble: a vertical band with a notched foot.
    rx0, rx1 = x0 + w * 0.42, x0 + w * 0.58
    ry0, ry1 = y0 + h * 0.26, y0 + h * 0.62
    d.rectangle([rx0, ry0, rx1, ry1], fill=pink)
    d.polygon([(rx0, ry1), (rx1, ry1), ((rx0 + rx1) / 2, ry1 + h * 0.10)], fill=pink)

    # Two dots to suggest a conversation.
    r = lw * 1.1
    for cx in (x0 + w * 0.30, x0 + w * 0.70):
        cy = y0 + h * 0.40
        d.ellipse([cx - r, cy - r, cx + r, cy + r], fill=WHITE + (255,))


def make_logo(size=1024):
    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    img.paste(vertical_gradient((size, size), BG_TOP, BG_BOTTOM).convert("RGBA"),
              (0, 0), rounded_mask((size, size), int(size * 0.22)))
    draw_glow(img, (size * 0.28, size * 0.26), size * 0.36, VIOLET, alpha=130, blur=110)
    draw_glow(img, (size * 0.74, size * 0.76), size * 0.30, PINK, alpha=95, blur=110)

    d = ImageDraw.Draw(img)
    d.rounded_rectangle([size * 0.012] * 2 + [size * 0.988] * 2,
                        radius=int(size * 0.21), outline=(150, 120, 230, 70),
                        width=max(2, size // 320))
    draw_mark(img, (size * 0.18, size * 0.20, size * 0.82, size * 0.82), lw=max(6, int(size * 0.026)))
    return img


def fit_text(draw, text, bold, start, max_width):
    size = start
    while size > 10:
        f = find_font(size, bold=bold)
        if draw.textlength(text, font=f) <= max_width:
            return f
        size -= 2
    return find_font(10, bold=bold)


def make_wordmark(width=1600, height=480):
    img = Image.new("RGBA", (width, height), (0, 0, 0, 0))
    draw_glow(img, (width * 0.14, height * 0.5), height * 0.5, VIOLET, alpha=80, blur=90)
    draw_glow(img, (width * 0.86, height * 0.5), height * 0.5, PINK, alpha=60, blur=90)
    draw_mark(img, (width * 0.03, height * 0.18, width * 0.25, height * 0.82), lw=max(5, height // 48))

    d = ImageDraw.Draw(img)
    tx = int(width * 0.30)
    avail = width - tx - 40
    font = fit_text(d, "shioriai2api", True, int(height * 0.40), avail)
    sub = find_font(int(height * 0.105), bold=False)
    d.text((tx, height * 0.22), "shioriai2api", font=font, fill=WHITE)
    d.text((tx + 4, height * 0.70), "OpenAI-compatible API for shiori.ai", font=sub, fill=MUTED)
    return img


def make_social(width=1280, height=640):
    img = vertical_gradient((width, height), BG_TOP, BG_BOTTOM).convert("RGBA")
    draw_glow(img, (width * 0.15, height * 0.28), 340, VIOLET, alpha=125, blur=130)
    draw_glow(img, (width * 0.88, height * 0.76), 340, PINK, alpha=100, blur=130)

    d = ImageDraw.Draw(img)
    d.rounded_rectangle([12, 12, width - 12, height - 12], radius=36,
                        outline=(150, 120, 230, 65), width=3)
    draw_mark(img, (width * 0.06, height * 0.24, width * 0.31, height * 0.80), lw=max(7, height // 58))

    tx = int(width * 0.36)
    avail = width - tx - 60
    title = fit_text(d, "shioriai2api", True, int(height * 0.185), avail)
    sub = fit_text(d, "Turn shiori.ai into an OpenAI-compatible API.", False, int(height * 0.055), avail)
    sub2 = fit_text(d, "Pure Go - zero deps - streaming - reasoning - multi-account", False, int(height * 0.055), avail)
    d.text((tx, height * 0.19), "shioriai2api", font=title, fill=WHITE)
    d.text((tx, height * 0.45), "Turn shiori.ai into an OpenAI-compatible API.", font=sub, fill=MUTED)
    d.text((tx, height * 0.45 + int(height * 0.075)), "Pure Go - zero deps - streaming - reasoning - multi-account",
           font=sub2, fill=PALE)

    pills = ["OpenAI compatible", "90+ models", "CF-aware"]
    px, py, pad = tx, int(height * 0.71), 20
    pf = find_font(int(height * 0.036), bold=True)
    for p in pills:
        tw = d.textlength(p, font=pf)
        d.rounded_rectangle([px, py, px + tw + pad * 2, py + 56], radius=28,
                            fill=(42, 26, 78, 215), outline=(150, 120, 230, 165), width=2)
        d.text((px + pad, py + 11), p, font=pf, fill=(220, 210, 250))
        px += int(tw + pad * 2 + 16)
    return img


def main():
    logo = make_logo(1024)
    for name, size in [("logo.png", 1024), ("logo-512.png", 512),
                       ("logo-256.png", 256), ("favicon.png", 128)]:
        out = logo if size == 1024 else logo.resize((size, size), Image.LANCZOS)
        out.save(os.path.join(ASSETS, name))

    make_wordmark().save(os.path.join(ASSETS, "wordmark.png"))
    make_social().convert("RGB").save(os.path.join(ASSETS, "social.png"), quality=95)
    print("assets written to", ASSETS)


if __name__ == "__main__":
    main()
