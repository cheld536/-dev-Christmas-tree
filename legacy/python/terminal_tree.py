#!/usr/bin/env python3
"""A rotating 3-D ASCII Christmas tree. Python 3.9+, no dependencies."""
import argparse
import colorsys
import math
import os
import random
import shutil
import sys
import time
from dataclasses import dataclass

ESC = '\033['
TAU = 2 * math.pi


@dataclass(frozen=True)
class Point:
    x: float
    y: float
    z: float
    char: str
    kind: str
    hue: float = 0.0


def rgb(h, s=0.9, v=1.0):
    return tuple(round(c * 255) for c in colorsys.hsv_to_rgb(h % 1, s, v))


def radius(y):
    # Three overlapping tiers, each widening towards the bottom.
    for low, high, r in ((0.34, 1.14, .46), (-.25, .60, .68), (-.91, .04, .92)):
        if low <= y <= high:
            return .035 + r * (high - y) / (high - low)
    return .025


def build_tree():
    rng = random.Random(1225)
    points = []
    def ring_point(y, angle, r, char, kind, hue=0):
        points.append(Point(r * math.cos(angle), y, r * math.sin(angle), char, kind, hue))

    # Evergreen surface: a fixed point cloud, not regenerated every frame.
    for i in range(103):
        y = -.91 + i * 2.05 / 102
        r = radius(y)
        count = max(12, int(115 * r))
        for j in range(count):
            a = TAU * j / count + i * .21
            ring_point(y, a, r + rng.uniform(-.018, .018), rng.choice('^+*.:#'), 'leaf', rng.random())

    # Two spirals of programmable RGB LEDs, wrapped around the tree.
    for j in range(370):
        u = j / 369
        y = -.88 + 1.99 * u
        a = TAU * 3.2 * u
        ring_point(y, a, radius(y) + .035, 'o', 'light', u)
        ring_point(y, a + math.pi, radius(y) + .028, '.', 'tinsel', u)

    # Developer-themed ornaments: braces, operators, binary digits.
    for j in range(95):
        y = rng.uniform(-.86, .97)
        a = rng.uniform(0, TAU)
        ring_point(y, a, radius(y) + .065, rng.choice('{}<>01@$%&'), 'ornament', rng.random())

    # Trunk.
    for i in range(22):
        for j in range(22):
            ring_point(-1.37 + i * .022, TAU * j / 22, .105, '|', 'trunk')
    return points


TREE = build_tree()
SNOW = [(random.Random(1000 + i).random(), random.Random(2000 + i).random(),
         .12 + random.Random(3000 + i).random() * .30) for i in range(90)]


def frame(width, height, t, speed=1.0, color=True):
    width, height = max(1, width), max(1, height)
    grid = [[(' ', None, -1e9) for _ in range(width)] for _ in range(height)]

    def put(x, y, char, col=None, depth=1e6):
        if 0 <= x < width and 0 <= y < height and depth >= grid[y][x][2]:
            grid[y][x] = (char, col, depth)

    def label(y, s, col):
        start = (width - len(s)) // 2
        for i, c in enumerate(s):
            put(start + i, y, c, col)

    if width < 28 or height < 13:
        label(height // 2, 'Resize terminal', (255, 200, 90))
    else:
        scale = min((height - 9) / 2.65, (width - 6) / 4.1)
        scale = max(1, scale)
        cx, cy = (width - 1) / 2, 3 + (height - 8) / 2
        angle = t * .85 * speed
        ca, sa = math.cos(angle), math.sin(angle)
        tilt = .12
        ct, st = math.cos(tilt), math.sin(tilt)

        def project(x, y, z):
            depth = z * ct + y * st
            vertical = y * ct - z * st
            perspective = 5.0 / (5.0 - depth)
            return (round(cx + x * scale * 2.02 * perspective),
                    round(cy - vertical * scale * perspective), depth)

        # Falling snow is behind the tree, so the depth buffer hides it.
        for i, (sx, sy, velocity) in enumerate(SNOW[:min(90, width * height // 24)]):
            x = int(((sx + .009 * math.sin(t + i)) % 1) * width)
            y = 1 + int(((sy + t * velocity / 5) % 1) * (height - 4))
            put(x, y, '.' if i % 3 else '*', (115, 158, 195), -100)

        for p in TREE:
            x, z = p.x * ca - p.z * sa, p.x * sa + p.z * ca
            px, py, depth = project(x, p.y, z)
            ch = p.char
            if p.kind == 'leaf':
                # Lighting follows the visible surface and adds 3-D shading.
                normal = z / max(.035, math.hypot(x, z))
                value = .37 + .48 * (normal + 1) / 2 + .12 * p.hue
                col = rgb(.34 + .08 * p.hue, .78, min(1, value))
            elif p.kind == 'light':
                col = rgb(p.hue * 1.9 - t * .35, .90, 1)
                ch = 'O' if math.sin(t * 6 + p.hue * 40) > .65 else 'o'
            elif p.kind == 'tinsel':
                col = rgb(.12 + .035 * math.sin(t * 2 + p.hue * 12), .66, .98)
            elif p.kind == 'ornament':
                col = rgb(p.hue + t * .12, .58, 1)
            else:
                col = (151, 91, 49)
            put(px, py, ch, col, depth)

        # Star with a pulsing golden glow (ASCII-safe on all terminals).
        sx, sy, _ = project(0, 1.14, 0)
        star = rgb(.12 + .025 * math.sin(t * 3), .55, 1)
        for dx, dy, ch in ((0,-2,'|'), (-1,-1,'\\'), (0,-1,'*'), (1,-1,'/'),
                           (-2,0,'-'), (-1,0,'-'), (0,0,'@'), (1,0,'-'), (2,0,'-'),
                           (-1,1,'/'), (1,1,'\\')):
            put(sx + dx, sy + dy, ch, star)

        title = 'M E R R Y   C H R I S T M A S'
        if width < len(title):
            title = 'MERRY CHRISTMAS'
        start = (width - len(title)) // 2
        for i, ch in enumerate(title):
            put(start + i, 0, ch, rgb(i / 18 - t * .18, .70, 1))
        label(height - 3, '<<<  HAPPY CODING  >>>', (255, 214, 110))
        messages = ('while True: spread_joy()', 'git commit -m "Christmas"',
                    'print("Hello, Christmas!")', '404: bugs not found')
        label(height - 2, messages[int(t / 3) % len(messages)], (105, 230, 255))
        label(height - 1, 'Ctrl+C to exit', (120, 135, 155))

    lines = []
    for row in grid:
        out, previous = [], None
        for ch, col, _ in row:
            if color and col != previous and ch != ' ':
                out.append(ESC + ('0m' if col is None else '38;2;%d;%d;%dm' % col))
                previous = col
            out.append(ch)
        if color:
            out.append(ESC + '0m')
        lines.append(''.join(out))
    return '\r\n'.join(lines)


def enable_ansi():
    if os.name != 'nt':
        return True
    try:
        import ctypes
        from ctypes import wintypes
        k = ctypes.WinDLL('kernel32', use_last_error=True)
        k.GetStdHandle.argtypes = [wintypes.DWORD]
        k.GetStdHandle.restype = wintypes.HANDLE
        k.GetConsoleMode.argtypes = [wintypes.HANDLE, ctypes.POINTER(wintypes.DWORD)]
        k.SetConsoleMode.argtypes = [wintypes.HANDLE, wintypes.DWORD]
        handle = k.GetStdHandle(-11 & 0xffffffff)
        mode = wintypes.DWORD()
        return bool(k.GetConsoleMode(handle, ctypes.byref(mode)) and
                    k.SetConsoleMode(handle, mode.value | 0x0004))
    except (OSError, AttributeError):
        return False


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fps', type=int, default=24, help='animation frames per second (1-60)')
    parser.add_argument('--speed', type=float, default=1.0, help='rotation speed; negative reverses direction')
    parser.add_argument('--preview', action='store_true', help='print one frame and exit')
    parser.add_argument('--no-color', action='store_true', help='disable RGB colors')
    args = parser.parse_args()
    if not 1 <= args.fps <= 60 or not math.isfinite(args.speed):
        parser.error('fps must be 1-60 and speed must be finite')
    tty = sys.stdout.isatty()
    color = tty and not args.no_color and enable_ansi()
    size = shutil.get_terminal_size((80, 30))
    if args.preview or not tty:
        print(frame(max(1, size.columns - 1), size.lines, 0, args.speed, color))
        return
    if not enable_ansi():
        print('ANSI terminal required for animation. Try Windows Terminal or VS Code.')
        print(frame(max(1, size.columns - 1), size.lines, 0, args.speed, False))
        return
    try:
        sys.stdout.write(ESC + '?1049h' + ESC + '?25l' + ESC + '2J')
        sys.stdout.flush()
        start = time.perf_counter()
        previous = None
        while True:
            tick = time.perf_counter()
            size = shutil.get_terminal_size((80, 30))
            dims = (max(1, size.columns - 1), max(1, size.lines))
            if dims != previous:
                sys.stdout.write(ESC + '2J')
                previous = dims
            sys.stdout.write(ESC + 'H' + frame(*dims, tick - start, args.speed, color))
            sys.stdout.flush()
            time.sleep(max(0, 1 / args.fps - (time.perf_counter() - tick)))
    except KeyboardInterrupt:
        pass
    finally:
        sys.stdout.write(ESC + '0m' + ESC + '?25h' + ESC + '?1049l')
        sys.stdout.flush()


if __name__ == '__main__':
    main()
