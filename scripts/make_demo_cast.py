#!/usr/bin/env python3
"""Render assets/demo.cast (asciinema v2 format) from assets/demo.tape.

The tape lists the demo commands (lines starting with "$ "). Each command is
executed from the repository root with a controlled environment against the
shipped offline fixtures, so the recording is deterministic. Build the
binary first:

    go build -o agentmanifest ./cmd/agentmanifest
    python3 scripts/make_demo_cast.py

All credentials in the fixtures are fake, vendor-shaped placeholders; the
cast therefore contains fingerprints and masked hints only.
"""

import json
import pathlib
import shutil
import subprocess
import sys
import time

ROOT = pathlib.Path(__file__).resolve().parent.parent
TAPE = ROOT / "assets" / "demo.tape"
CAST = ROOT / "assets" / "demo.cast"
DEMO_OUT = ROOT / ".demo-out"

WIDTH, HEIGHT = 100, 30
FIXTURE_ENV = {
    "HOME": str(ROOT / "testdata" / "fixtures" / "home"),
    "PATH": f"{ROOT}:/usr/bin:/bin",
    "GROQ_API_KEY": "gsk_FAKE0123456789abcdefghijklmnop",
    "SHELL": "/bin/zsh",
    "TERM": "xterm-256color",
}


def tape_commands():
    commands = []
    for line in TAPE.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line.startswith("$ "):
            commands.append(line[2:])
    if not commands:
        sys.exit(f"no commands found in {TAPE}")
    return commands


def main():
    if shutil.which("agentmanifest") is None and not (ROOT / "agentmanifest").exists():
        sys.exit("build the binary first: go build -o agentmanifest ./cmd/agentmanifest")

    events = []
    clock = 0.0

    def emit(chunk, delay):
        nonlocal clock
        if not chunk:
            return
        events.append([round(clock, 3), "o", chunk])
        clock += delay

    for command in tape_commands():
        emit(f"$ {command}\n", 0.45)
        proc = subprocess.run(
            command, shell=True, cwd=ROOT, env=FIXTURE_ENV,
            capture_output=True, text=True,
        )
        if proc.returncode != 0:
            sys.exit(f"command failed ({proc.returncode}): {command}\n{proc.stderr}")
        for line in proc.stdout.splitlines(keepends=True):
            emit(line, 0.12)
        for line in proc.stderr.splitlines(keepends=True):
            emit(line, 0.12)
        emit("\n", 0.35)

    header = {
        "version": 2,
        "width": WIDTH,
        "height": HEIGHT,
        "timestamp": int(time.time()),
        "duration": round(clock, 3),
        "command": "agentmanifest scan",
        "title": "agentmanifest — offline fixture scan demo",
        "env": {"SHELL": "/bin/zsh", "TERM": "xterm-256color"},
    }
    CAST.write_text(
        json.dumps(header, ensure_ascii=False) + "\n"
        + "".join(json.dumps(event, ensure_ascii=False) + "\n" for event in events),
        encoding="utf-8",
    )
    print(f"wrote {CAST} ({clock:.1f}s, {len(events)} events)")


if __name__ == "__main__":
    main()
