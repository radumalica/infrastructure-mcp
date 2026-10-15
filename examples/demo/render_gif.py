#!/usr/bin/env python3
"""Render assets/demo.gif from a live run of the demo.

Runs the real tool calls against the running demo (./run-demo.sh first) and draws
their actual output as a terminal animation. Long results are shortened with a
visible "..." marker. Requires Pillow: pip install pillow

  python3 render_gif.py ../../assets/demo.gif
"""
import json
import os
import subprocess
import sys
import textwrap

from PIL import Image, ImageDraw, ImageFont

HERE = os.path.dirname(os.path.abspath(__file__))
FONT = '/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf'
SIZE = 15
COLS = 96
ROWS = 24
PAD = 18
BG, FG, DIM = (22, 24, 29), (220, 223, 228), (120, 128, 140)
GREEN, BLUE, YELLOW, RED = (126, 211, 125), (110, 170, 255), (240, 200, 110), (240, 110, 110)

font = ImageFont.truetype(FONT, SIZE)
CW = int(font.getlength('M'))
LH = SIZE + 6
WIDTH = PAD * 2 + CW * COLS
HEIGHT = PAD * 2 + LH * ROWS


def call(*args):
    env = dict(os.environ, MCP_COMPACT='1')
    proc = subprocess.run(
        [sys.executable, os.path.join(HERE, 'mcp_call.py'), *args],
        capture_output=True, text=True, env=env, timeout=60, check=False,
    )
    return (proc.stdout or proc.stderr).strip()


def shorten(text, limit):
    """Wrap to the terminal width, keeping at most `limit` lines."""
    lines = textwrap.wrap(text, COLS - 2) or ['']
    if len(lines) > limit:
        lines = lines[: limit - 1] + ['...']
    return lines


class Screen:
    def __init__(self):
        self.lines = []  # list of (text, color)
        self.frames = []

    def snap(self, ms, cursor=False):
        img = Image.new('RGB', (WIDTH, HEIGHT), BG)
        draw = ImageDraw.Draw(img)
        visible = self.lines[-ROWS:]
        for i, (text, color) in enumerate(visible):
            draw.text((PAD, PAD + i * LH), text, font=font, fill=color)
        if cursor and visible:
            x = PAD + font.getlength(visible[-1][0])
            y = PAD + (len(visible) - 1) * LH
            draw.rectangle([x + 2, y + 2, x + CW, y + LH - 3], fill=FG)
        self.frames.append((img.convert('P', palette=Image.ADAPTIVE, colors=32), ms))

    def comment(self, text, ms=900):
        self.lines.append((text, DIM))
        self.snap(ms)

    def type_command(self, command):
        self.lines.append(('$ ', GREEN))
        self.snap(250, cursor=True)
        for i in range(0, len(command), 3):
            self.lines[-1] = ('$ ' + command[: i + 3], GREEN)
            self.snap(45, cursor=True)
        self.lines[-1] = ('$ ' + command, GREEN)
        self.snap(350)

    def output(self, lines, color=FG, hold=1700):
        for text in lines:
            self.lines.append((text, color))
        self.snap(hold)

    def blank(self):
        self.lines.append(('', FG))


def main(out_path):
    tools = call('--list').split()
    scr = Screen()
    scr.comment('# Infrastructure MCP: safe, named tools for AI agents. No raw SSH.', 1400)
    scr.comment('# Two throwaway SSH targets + the server, started with ./run-demo.sh', 1400)
    scr.blank()

    scr.type_command('mcp_call.py --list | wc -l')
    scr.output([str(len(tools)) + '  tools'], hold=700)
    dangerous = [t for t in ('run_command', 'docker_exec', 'kubectl_exec') if t in tools]
    scr.type_command("mcp_call.py --list | grep -E 'run_command|docker_exec|kubectl_exec'")
    scr.output(dangerous or ['(none) - arbitrary command execution is opt-in'], YELLOW, 1800)
    scr.blank()

    for cmd, args, limit in (
        ('list_servers', '{}', 5),
        ('uptime', '{"server":"demo-web"}', 3),
        ('memory_usage', '{"server":"demo-db"}', 4),
    ):
        scr.type_command("mcp_call.py %s '%s'" % (cmd, args))
        scr.output(shorten(call(cmd, args), limit), BLUE, 2100)
        scr.blank()

    scr.type_command("mcp_call.py uptime '{\"server\":\"10.0.0.99\"}'")
    scr.output(shorten(call('uptime', '{"server":"10.0.0.99"}'), 4), RED, 2600)
    scr.blank()
    scr.comment('# Structured JSON, inventory-only targets, every call logged.', 2600)

    frames = [f for f, _ in scr.frames]
    durations = [d for _, d in scr.frames]
    frames[0].save(out_path, save_all=True, append_images=frames[1:], duration=durations,
                   loop=0, optimize=True, disposal=1)
    print('wrote %s (%d frames, %d KB)' % (out_path, len(frames), os.path.getsize(out_path) // 1024))
    json.dumps(tools)  # tools list was real; fail loudly if it was not JSON-safe


if __name__ == '__main__':
    main(sys.argv[1] if len(sys.argv) > 1 else 'demo.gif')
