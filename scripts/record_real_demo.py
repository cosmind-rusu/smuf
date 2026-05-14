#!/usr/bin/env python3
"""Run smuf demo script and create a real asciinema v2 cast from its output."""

import json
import subprocess
import sys
import time
import os

def main():
    script_path = os.path.expanduser("~/Desktop/smuf/scripts/real_demo.sh")
    output_path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/smuf-demo-cast.json"

    # Run the demo and capture output line by line with timestamps
    start = time.time()
    events = []

    # Track lines for dedup
    last_line = ""

    proc = subprocess.Popen(
        ["bash", script_path],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1,
    )

    lines = []
    for line in proc.stdout:
        # Deduplicate consecutive identical lines (from pkill retries etc.)
        clean_line = line.rstrip("\n")
        # Don't dedup, just record everything
        ts = round(time.time() - start, 3)
        events.append([ts, "o", line])
        lines.append(clean_line)

    proc.wait()

    total_duration = events[-1][0] if events else 0

    # Header
    header = {
        "version": 2,
        "width": 80,
        "height": 28,
        "timestamp": int(start),
        "title": "smuf — self-hosted HTTP tunnel (real demo)",
        "env": {
            "TERM": "xterm-256color",
            "SHELL": "/bin/bash",
        },
    }

    with open(output_path, "w") as f:
        f.write(json.dumps(header) + "\n")
        for ev in events:
            f.write(json.dumps(ev) + "\n")

    print(f"✓ Real cast written to {output_path}")
    print(f"  Duration: {total_duration:.1f}s")
    print(f"  Lines: {len(lines)}")
    print(f"  Events: {len(events)}")

    # Convert to SVG
    svg_path = output_path.replace(".json", ".svg")
    subprocess.run(
        ["npx", "svg-term", "--out", svg_path, "--window", "--height", "28"],
        stdin=open(output_path),
        capture_output=False,
    )
    print(f"✓ SVG written to {svg_path}")


if __name__ == "__main__":
    main()
