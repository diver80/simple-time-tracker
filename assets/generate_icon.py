#!/usr/bin/env python3
"""
Generate macOS AppIcon.icns, master PNG icon (1024x1024), and Windows ICO for Simple Time Tracker.
Uses high-resolution supersampling (2048x2048 -> 1024x1024 Lanczos) and Apple macOS
Human Interface Guidelines (HIG) squircle geometry with ambient drop shadow.

Theme:
- Deep obsidian / slate base (#0b0f19)
- Luminous emerald (#10B981) and cyan (#06B6D4) precision stopwatch dial
- Glowing circular progress ring with multi-layer bloom
- Precision tick markers and chronograph pushers
- Minimalist center indicator with crosshair instrument lines
"""

import math
import os
import shutil
import subprocess
from PIL import Image, ImageDraw, ImageFilter

def create_superellipse_mask(size, radius):
    """Generates an accurate macOS squircle mask with smooth antialiasing."""
    w, h = size
    mask = Image.new("L", (w, h), 0)
    draw = ImageDraw.Draw(mask)
    draw.rounded_rectangle([(0, 0), (w - 1, h - 1)], radius=radius, fill=255)
    return mask

def interpolate_color(c1, c2, t):
    """Linearly interpolates between two RGB tuples."""
    return (
        int(c1[0] + (c2[0] - c1[0]) * t),
        int(c1[1] + (c2[1] - c1[1]) * t),
        int(c1[2] + (c2[2] - c1[2]) * t),
    )

def generate_master_icon(size=1024):
    """Draws a 1024x1024 macOS app icon with 2x supersampling (2048x2048 downscaled)."""
    scale = 2
    canvas_size = size * scale  # 2048x2048

    # Base transparent canvas
    img = Image.new("RGBA", (canvas_size, canvas_size), (0, 0, 0, 0))

    # Squircle dimensions inside canvas (Apple HIG standard: ~824x824 at 1024 canvas, 100px inset)
    inset = 100 * scale  # 200px
    squircle_size = canvas_size - (2 * inset)  # 1648x1648
    squircle_radius = int(185 * scale * (squircle_size / (824 * scale)))  # 370px

    # 1. Smooth ambient drop shadow beneath squircle
    shadow_img = Image.new("RGBA", (canvas_size, canvas_size), (0, 0, 0, 0))
    shadow_draw = ImageDraw.Draw(shadow_img)
    shadow_offset_y = int(24 * scale)  # 48px
    shadow_box = [
        inset,
        inset + shadow_offset_y,
        canvas_size - inset,
        canvas_size - inset + shadow_offset_y
    ]
    shadow_draw.rounded_rectangle(shadow_box, radius=squircle_radius, fill=(0, 0, 0, 150))
    shadow_img = shadow_img.filter(ImageFilter.GaussianBlur(radius=24 * scale))
    img.alpha_composite(shadow_img)

    # 2. Squircle Background Tile
    tile = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))

    # Deep obsidian / slate gradient (#141e30 -> #0f172a -> #0b0f19)
    c_top = (20, 30, 48)     # #141e30
    c_mid = (15, 23, 42)     # #0f172a
    c_bot = (11, 15, 25)     # #0b0f19
    for y in range(squircle_size):
        t = y / (squircle_size - 1)
        if t < 0.5:
            st = t / 0.5
            col = interpolate_color(c_top, c_mid, st)
        else:
            st = (t - 0.5) / 0.5
            col = interpolate_color(c_mid, c_bot, st)
        line_img = Image.new("RGBA", (squircle_size, 1), (*col, 255))
        tile.paste(line_img, (0, y))

    # Subtle ambient radial glow in upper-center
    glow = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    glow_draw = ImageDraw.Draw(glow)
    gcx, gcy = squircle_size // 2, int(squircle_size * 0.44)
    max_rad = int(squircle_size * 0.65)
    for rad in range(max_rad, 0, -12):
        t_glow = 1.0 - (rad / max_rad)
        alpha = int(35 * (t_glow ** 1.6))
        glow_draw.ellipse([gcx - rad, gcy - rad, gcx + rad, gcy + rad], fill=(6, 182, 212, alpha))
    tile = Image.alpha_composite(tile, glow)

    # Subtle diagonal glass sheen across top-left
    sheen = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    sheen_draw = ImageDraw.Draw(sheen)
    # 45-degree soft highlight band
    for i in range(int(squircle_size * 0.7)):
        alpha = int(14 * (1.0 - i / (squircle_size * 0.7)))
        sheen_draw.line([(0, i), (i, 0)], fill=(255, 255, 255, alpha), width=3)
    tile = Image.alpha_composite(tile, sheen)

    # 3. Vector Elements Layer
    elements = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    draw_elem = ImageDraw.Draw(elements)

    # Dial center coordinates
    dcx = squircle_size // 2  # 824
    dcy = int(squircle_size * 0.52)  # 856 (centered vertically considering top pusher)

    # --- (A) Stopwatch Chronograph Pushers / Crown ---
    # Top Crown / Main Pusher at 12 o'clock
    r_case_outer = 545
    crown_w = int(120 * scale)  # 240
    crown_h = int(36 * scale)   # 72
    crown_x = dcx - crown_w // 2
    crown_y = dcy - r_case_outer - crown_h + int(14 * scale)

    # Stem connecting crown to case
    stem_w = int(52 * scale)
    stem_top = crown_y + crown_h - int(8 * scale)
    stem_bot = dcy - r_case_outer + int(10 * scale)
    draw_elem.rectangle(
        [dcx - stem_w // 2, stem_top, dcx + stem_w // 2, stem_bot],
        fill=(30, 41, 59, 255)
    )
    # Stem edge highlights
    draw_elem.line([(dcx - stem_w // 2, stem_top), (dcx - stem_w // 2, stem_bot)], fill=(255, 255, 255, 60), width=int(2 * scale))
    draw_elem.line([(dcx + stem_w // 2, stem_top), (dcx + stem_w // 2, stem_bot)], fill=(0, 0, 0, 100), width=int(2 * scale))

    # Top crown cap
    draw_elem.rounded_rectangle(
        [crown_x, crown_y, crown_x + crown_w, crown_y + crown_h],
        radius=int(12 * scale),
        fill=(30, 41, 59, 255),
        outline=(255, 255, 255, 80),
        width=int(2 * scale)
    )
    # Crown knurling grooves
    for ki in range(5):
        kx = crown_x + int(24 * scale) + ki * int(18 * scale)
        draw_elem.line([(kx, crown_y + int(6 * scale)), (kx, crown_y + crown_h - int(6 * scale))], fill=(15, 23, 42, 220), width=int(2.5 * scale))
        draw_elem.line([(kx + int(2 * scale), crown_y + int(6 * scale)), (kx + int(2 * scale), crown_y + crown_h - int(6 * scale))], fill=(255, 255, 255, 40), width=int(1.5 * scale))

    # Crown glowing accent stripe
    draw_elem.line(
        [dcx - int(24 * scale), crown_y + int(5 * scale), dcx + int(24 * scale), crown_y + int(5 * scale)],
        fill=(6, 182, 212, 240),
        width=int(2 * scale)
    )

    # Angled Chronograph Pushers at 10:30 and 1:30 (angles -38 deg and +38 deg from 12 o'clock)
    for angle_deg in (-38, 38):
        rad = math.radians(angle_deg - 90)
        p_dist = r_case_outer - int(8 * scale)
        px = dcx + p_dist * math.cos(rad)
        py = dcy + p_dist * math.sin(rad)
        tx = -math.sin(rad)
        ty = math.cos(rad)
        nx = math.cos(rad)
        ny = math.sin(rad)

        pw = int(22 * scale)
        ph = int(18 * scale)
        p_pts = [
            (px - tx * pw + nx * ph, py - ty * pw + ny * ph),
            (px + tx * pw + nx * ph, py + ty * pw + ny * ph),
            (px + tx * pw, py + ty * pw),
            (px - tx * pw, py - ty * pw),
        ]
        draw_elem.polygon(p_pts, fill=(30, 41, 59, 255), outline=(255, 255, 255, 70))
        draw_elem.line([p_pts[0], p_pts[1]], fill=(255, 255, 255, 100), width=int(2 * scale))

    # --- (B) Stopwatch Outer Bezel & Case ---
    r_bezel = 540
    # Outer dark titanium case bezel
    draw_elem.ellipse(
        [dcx - r_bezel, dcy - r_bezel, dcx + r_bezel, dcy + r_bezel],
        fill=(15, 23, 42, 255),
        outline=(30, 41, 59, 255),
        width=int(6 * scale)
    )
    # Bezel inner rim highlight
    draw_elem.ellipse(
        [dcx - (r_bezel - int(4 * scale)), dcy - (r_bezel - int(4 * scale)),
         dcx + (r_bezel - int(4 * scale)), dcy + (r_bezel - int(4 * scale))],
        outline=(255, 255, 255, 35),
        width=int(1.5 * scale)
    )

    # Dial Face Recessed Plate
    r_dial = 518
    draw_elem.ellipse(
        [dcx - r_dial, dcy - r_dial, dcx + r_dial, dcy + r_dial],
        fill=(8, 12, 21, 255)
    )

    # Concentric precision dial grooves
    for r_groove in (462, 345, 220):
        draw_elem.ellipse(
            [dcx - r_groove, dcy - r_groove, dcx + r_groove, dcy + r_groove],
            outline=(255, 255, 255, 10),
            width=int(1.5 * scale)
        )

    # Precision Crosshair instrument lines (broken at center and edge)
    ch_col = (255, 255, 255, 16)
    # Vertical crosshair
    draw_elem.line([(dcx, dcy - 340), (dcx, dcy - 90)], fill=ch_col, width=int(1.5 * scale))
    draw_elem.line([(dcx, dcy + 90), (dcx, dcy + 340)], fill=ch_col, width=int(1.5 * scale))
    # Horizontal crosshair
    draw_elem.line([(dcx - 340, dcy), (dcx - 90, dcy)], fill=ch_col, width=int(1.5 * scale))
    draw_elem.line([(dcx + 90, dcy), (dcx + 340, dcy)], fill=ch_col, width=int(1.5 * scale))

    # --- (C) Precision Tick Markers (60 ticks across 360 degrees) ---
    r_tick_outer = 498
    for i in range(60):
        angle = i * 6 - 90
        rad = math.radians(angle)
        cos_a = math.cos(rad)
        sin_a = math.sin(rad)

        if i % 15 == 0:
            # Cardinal ticks (12, 3, 6, 9 o'clock)
            tick_len = int(24 * scale)  # 48px
            tick_w = int(4.5 * scale)   # 9px
            color = (16, 185, 129, 255)  # Luminous emerald

            if i == 0:
                # 12 o'clock double-tick chronograph signature
                offset_tan = int(5.5 * scale)
                tan_x = -sin_a
                tan_y = cos_a
                for s in (-1, 1):
                    x1 = dcx + (r_tick_outer - tick_len) * cos_a + s * offset_tan * tan_x
                    y1 = dcy + (r_tick_outer - tick_len) * sin_a + s * offset_tan * tan_y
                    x2 = dcx + r_tick_outer * cos_a + s * offset_tan * tan_x
                    y2 = dcy + r_tick_outer * sin_a + s * offset_tan * tan_y
                    draw_elem.line([(x1, y1), (x2, y2)], fill=color, width=int(3 * scale))
                continue
        elif i % 5 == 0:
            # Major 5-minute ticks
            tick_len = int(17 * scale)  # 34px
            tick_w = int(3.5 * scale)   # 7px
            color = (6, 182, 212, 230)  # Luminous cyan
        else:
            # Minor 1-minute ticks
            tick_len = int(9 * scale)   # 18px
            tick_w = int(2 * scale)     # 4px
            color = (148, 163, 184, 110)  # Subtle slate

        x1 = dcx + (r_tick_outer - tick_len) * cos_a
        y1 = dcy + (r_tick_outer - tick_len) * sin_a
        x2 = dcx + r_tick_outer * cos_a
        y2 = dcy + r_tick_outer * sin_a
        draw_elem.line([(x1, y1), (x2, y2)], fill=color, width=tick_w)

    # --- (D) Inactive Recessed Progress Ring Track (full 360 channel) ---
    r_ring = 412
    ring_w = int(26 * scale)  # 52px
    ring_box = [dcx - r_ring, dcy - r_ring, dcx + r_ring, dcy + r_ring]
    draw_elem.ellipse(ring_box, outline=(14, 21, 37, 240), width=ring_w)
    # Inner and outer channel edge grooves
    draw_elem.ellipse(
        [dcx - (r_ring + ring_w // 2), dcy - (r_ring + ring_w // 2),
         dcx + (r_ring + ring_w // 2), dcy + (r_ring + ring_w // 2)],
        outline=(255, 255, 255, 14),
        width=int(1.5 * scale)
    )
    draw_elem.ellipse(
        [dcx - (r_ring - ring_w // 2), dcy - (r_ring - ring_w // 2),
         dcx + (r_ring - ring_w // 2), dcy + (r_ring - ring_w // 2)],
        outline=(255, 255, 255, 14),
        width=int(1.5 * scale)
    )

    # --- (E) Luminous Active Progress Arc with Multi-layer Bloom Glow ---
    # Arc spans from 12 o'clock (-90 deg / 270 deg) clockwise by 225 deg (to 135 deg / ~4:30 position / 225 deg clock)
    start_deg = -90
    arc_span = 225
    end_deg = start_deg + arc_span  # 135

    c_emerald = (16, 185, 129)  # #10B981
    c_cyan = (6, 182, 212)      # #06B6D4

    # Glow layer 1: wide soft bloom
    glow_wide = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    draw_glow_wide = ImageDraw.Draw(glow_wide)

    # Glow layer 2: medium neon aura
    glow_med = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    draw_glow_med = ImageDraw.Draw(glow_med)

    # Sharp arc layer
    arc_layer = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    draw_arc = ImageDraw.Draw(arc_layer)

    # Render arc slices with smooth color gradient interpolation
    num_slices = 320
    for s in range(num_slices):
        t = s / (num_slices - 1)
        cur_ang = start_deg + t * arc_span
        col = interpolate_color(c_emerald, c_cyan, t)

        # Wide bloom arc
        draw_glow_wide.arc(
            ring_box,
            start=cur_ang,
            end=cur_ang + 1.8,
            fill=(*col, 75),
            width=ring_w + int(28 * scale)
        )
        # Medium neon arc
        draw_glow_med.arc(
            ring_box,
            start=cur_ang,
            end=cur_ang + 1.8,
            fill=(*col, 160),
            width=ring_w + int(12 * scale)
        )
        # Sharp core arc
        draw_arc.arc(
            ring_box,
            start=cur_ang,
            end=cur_ang + 1.2,
            fill=(*col, 255),
            width=ring_w
        )

    # Rounded start cap at 12 o'clock (-90 deg)
    cap_start_rad = math.radians(start_deg)
    sx = dcx + r_ring * math.cos(cap_start_rad)
    sy = dcy + r_ring * math.sin(cap_start_rad)
    draw_arc.ellipse(
        [sx - ring_w // 2, sy - ring_w // 2, sx + ring_w // 2, sy + ring_w // 2],
        fill=(*c_emerald, 255)
    )
    draw_glow_med.ellipse(
        [sx - ring_w // 2 - int(6 * scale), sy - ring_w // 2 - int(6 * scale),
         sx + ring_w // 2 + int(6 * scale), sy + ring_w // 2 + int(6 * scale)],
        fill=(*c_emerald, 160)
    )

    # Rounded end cap & Active Leading Node at 135 deg
    cap_end_rad = math.radians(end_deg)
    ex = dcx + r_ring * math.cos(cap_end_rad)
    ey = dcy + r_ring * math.sin(cap_end_rad)
    draw_arc.ellipse(
        [ex - ring_w // 2, ey - ring_w // 2, ex + ring_w // 2, ey + ring_w // 2],
        fill=(*c_cyan, 255)
    )

    # Luminous puck node at the leading head
    node_r = int(17 * scale)
    draw_arc.ellipse(
        [ex - node_r, ey - node_r, ex + node_r, ey + node_r],
        fill=(255, 255, 255, 255),
        outline=(*c_cyan, 255),
        width=int(3.5 * scale)
    )

    # Node bloom
    draw_glow_med.ellipse(
        [ex - node_r - int(10 * scale), ey - node_r - int(10 * scale),
         ex + node_r + int(10 * scale), ey + node_r + int(10 * scale)],
        fill=(6, 182, 212, 230)
    )
    draw_glow_wide.ellipse(
        [ex - node_r - int(24 * scale), ey - node_r - int(24 * scale),
         ex + node_r + int(24 * scale), ey + node_r + int(24 * scale)],
        fill=(6, 182, 212, 130)
    )

    # Apply Gaussian blurs to glows
    glow_wide = glow_wide.filter(ImageFilter.GaussianBlur(radius=28 * scale))
    glow_med = glow_med.filter(ImageFilter.GaussianBlur(radius=10 * scale))

    # Composite glows & arc into elements
    elements = Image.alpha_composite(elements, glow_wide)
    elements = Image.alpha_composite(elements, glow_med)
    elements = Image.alpha_composite(elements, arc_layer)

    # --- (F) Precision Stopwatch Hand & Center Indicator ---
    # Hand originates at center hub and points precisely to active elapsed time position (135 deg)
    hand_angle_rad = math.radians(end_deg)
    hand_len = 384  # Reaches right up to the glowing puck indicator
    hx_tip = dcx + hand_len * math.cos(hand_angle_rad)
    hy_tip = dcy + hand_len * math.sin(hand_angle_rad)

    # Perpendicular unit vector for hand tapering
    perp_x = -math.sin(hand_angle_rad)
    perp_y = math.cos(hand_angle_rad)

    base_w = int(6.5 * scale)  # 13px width at hub base
    tip_w = int(1.5 * scale)   # 3px width at tip

    # Minimalist needle originating cleanly from center pivot (covered by hub)
    hand_poly = [
        (dcx - perp_x * base_w, dcy - perp_y * base_w),
        (hx_tip - perp_x * tip_w, hy_tip - perp_y * tip_w),
        (hx_tip + perp_x * tip_w, hy_tip + perp_y * tip_w),
        (dcx + perp_x * base_w, dcy + perp_y * base_w),
    ]

    # Hand subtle drop shadow
    hand_shadow = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    draw_hs = ImageDraw.Draw(hand_shadow)
    hs_offset_x = int(6 * scale)
    hs_offset_y = int(8 * scale)
    draw_hs.polygon(
        [(x + hs_offset_x, y + hs_offset_y) for x, y in hand_poly],
        fill=(0, 0, 0, 160)
    )
    hand_shadow = hand_shadow.filter(ImageFilter.GaussianBlur(radius=5 * scale))
    elements = Image.alpha_composite(elements, hand_shadow)

    # Draw Hand Core & Luminous Tip
    draw_elem = ImageDraw.Draw(elements)
    draw_elem.polygon(hand_poly, fill=(245, 250, 255, 255))
    # Glowing cyan accent stripe along center of the pointer
    draw_elem.line(
        [(dcx, dcy), (hx_tip, hy_tip)],
        fill=(6, 182, 212, 255),
        width=int(2.5 * scale)
    )
    # Luminous cyan tip accent
    draw_elem.line(
        [(hx_tip - 30 * math.cos(hand_angle_rad), hy_tip - 30 * math.sin(hand_angle_rad)), (hx_tip, hy_tip)],
        fill=(6, 182, 212, 255),
        width=int(3.5 * scale)
    )

    # --- (G) Center Pivot Hub ---
    hub_shadow = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    draw_hub_s = ImageDraw.Draw(hub_shadow)
    draw_hub_s.ellipse(
        [dcx - int(36 * scale), dcy - int(36 * scale) + int(6 * scale),
         dcx + int(36 * scale), dcy + int(36 * scale) + int(6 * scale)],
        fill=(0, 0, 0, 160)
    )
    hub_shadow = hub_shadow.filter(ImageFilter.GaussianBlur(radius=4 * scale))
    elements = Image.alpha_composite(elements, hub_shadow)

    draw_elem = ImageDraw.Draw(elements)
    # Outer hub ring (metallic slate)
    r_hub_outer = int(32 * scale)
    draw_elem.ellipse(
        [dcx - r_hub_outer, dcy - r_hub_outer, dcx + r_hub_outer, dcy + r_hub_outer],
        fill=(30, 41, 59, 255),
        outline=(255, 255, 255, 80),
        width=int(2 * scale)
    )
    # Inner hub disc (deep titanium)
    r_hub_inner = int(22 * scale)
    draw_elem.ellipse(
        [dcx - r_hub_inner, dcy - r_hub_inner, dcx + r_hub_inner, dcy + r_hub_inner],
        fill=(15, 23, 42, 255)
    )
    # Center jewel (luminous emerald with specular glint)
    r_hub_jewel = int(12 * scale)
    draw_elem.ellipse(
        [dcx - r_hub_jewel, dcy - r_hub_jewel, dcx + r_hub_jewel, dcy + r_hub_jewel],
        fill=(16, 185, 129, 255)
    )
    # Specular white dot
    r_glint = int(3.5 * scale)
    draw_elem.ellipse(
        [dcx - int(4 * scale) - r_glint, dcy - int(4 * scale) - r_glint,
         dcx - int(4 * scale) + r_glint, dcy - int(4 * scale) + r_glint],
        fill=(255, 255, 255, 240)
    )

    # Composite elements onto squircle tile
    tile = Image.alpha_composite(tile, elements)

    # 4. Glass Edge Bevel / Inner Highlight Border (macOS HIG signature)
    border = Image.new("RGBA", (squircle_size, squircle_size), (0, 0, 0, 0))
    border_draw = ImageDraw.Draw(border)
    # Crisp upper highlight
    border_draw.rounded_rectangle(
        [int(2 * scale), int(2 * scale), squircle_size - int(2 * scale), squircle_size - int(2 * scale)],
        radius=squircle_radius,
        outline=(255, 255, 255, 55),
        width=int(2.5 * scale)
    )
    # Subtle dark rim on inner bottom edge
    border_draw.rounded_rectangle(
        [int(4 * scale), int(4 * scale), squircle_size - int(4 * scale), squircle_size - int(4 * scale)],
        radius=squircle_radius - int(2 * scale),
        outline=(0, 0, 0, 50),
        width=int(1.5 * scale)
    )
    tile = Image.alpha_composite(tile, border)

    # 5. Mask Tile to Squircle
    mask = create_superellipse_mask((squircle_size, squircle_size), radius=squircle_radius)

    # Composite squircle into main 2048x2048 canvas
    img.paste(tile, (inset, inset), mask=mask)

    # 6. Downsample from 2048 to 1024 with high-quality Lanczos resampling
    final_img = img.resize((size, size), Image.Resampling.LANCZOS)
    return final_img

def build_iconset_and_icns(master_img, assets_dir):
    """Generates an .iconset directory and converts to AppIcon.icns using iconutil."""
    iconset_dir = os.path.join(assets_dir, "AppIcon.iconset")
    if os.path.exists(iconset_dir):
        shutil.rmtree(iconset_dir)
    os.makedirs(iconset_dir, exist_ok=True)

    icon_sizes = [
        (16, "icon_16x16.png"),
        (32, "icon_16x16@2x.png"),
        (32, "icon_32x32.png"),
        (64, "icon_32x32@2x.png"),
        (128, "icon_128x128.png"),
        (256, "icon_128x128@2x.png"),
        (256, "icon_256x256.png"),
        (512, "icon_256x256@2x.png"),
        (512, "icon_512x512.png"),
        (1024, "icon_512x512@2x.png"),
    ]

    for sz, filename in icon_sizes:
        resized = master_img.resize((sz, sz), Image.Resampling.LANCZOS)
        resized.save(os.path.join(iconset_dir, filename), "PNG")

    icns_path = os.path.join(assets_dir, "AppIcon.icns")
    subprocess.run(["iconutil", "-c", "icns", iconset_dir, "-o", icns_path], check=True)

    # Also save Windows .ico file with multi-size resolutions
    ico_path = os.path.join(assets_dir, "icon.ico")
    master_img.save(
        ico_path,
        format="ICO",
        sizes=[(16, 16), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
    )

    # Clean up temporary iconset directory
    shutil.rmtree(iconset_dir)
    print(f"✓ Successfully compiled {icns_path} and {ico_path}")

def main():
    assets_dir = os.path.dirname(os.path.abspath(__file__))
    master_png_path = os.path.join(assets_dir, "icon.png")

    print("🎨 Rendering high-resolution macOS master icon (1024x1024) with 2x supersampling...")
    icon = generate_master_icon(1024)
    icon.save(master_png_path, "PNG")
    print(f"✓ Master icon generated: {master_png_path}")

    print("📦 Building Apple ICNS and Windows ICO bundles...")
    build_iconset_and_icns(icon, assets_dir)
    print("✨ Icon asset creation complete!")

if __name__ == "__main__":
    main()
