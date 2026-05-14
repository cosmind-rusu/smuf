#!/usr/bin/env python3
"""Generate an asciinema v2 cast file showing a smuf demo."""

import json
import sys
import time

# ─── helpers ─────────────────────────────────────────────────────────────

def header(width=80, height=18, title="smuf — self-hosted HTTP tunnel"):
    return {
        "version": 2,
        "width": width,
        "height": height,
        "timestamp": int(time.time()),
        "title": title,
        "env": {
            "TERM": "xterm-256color",
            "SHELL": "/bin/bash",
            "USER": "cosmin",
            "PWD": "~/smuf"
        }
    }

def delay(t):
    """Insert a delay event (blank output that represents waiting)."""
    return [t, "o", ""]

def prompt(t, text, typing_speed=0.008):
    """Input event: user types text."""
    return [t, "i", text]

def output(t, text):
    """Output event: terminal prints text."""
    return [t, "o", text]

def line(t, text="", typing_delay=0.02):
    """A line of output with optional typing delay."""
    return [t, "o", text + "\r\n"]

# ─── scene builder ────────────────────────────────────────────────────────

def build_cast():
    events = []
    t = 0.0

    # Use ANSI codes for colours to make it look real
    GREEN = "\x1b[32m"
    CYAN = "\x1b[36m"
    BOLD = "\x1b[1m"
    RESET = "\x1b[0m"
    DIM = "\x1b[2m"
    YELLOW = "\x1b[33m"

    # ── 1. Clear screen ──
    events.append(output(t, "\x1b[2J\x1b[H"))
    events.append(line(t, f"{DIM}smuf v0.3.0 — self-hosted HTTP tunnel (Apache 2.0){RESET}"))
    events.append(delay(t := t + 0.3))
    events.append(line(t, ""))

    # ── 2. Start server ──
    events.append(output(t, "$ "))
    events.append(prompt(t, "./smuf-server", typing_speed=0.01))
    events.append(line(t := t + 0.6))
    events.append(delay(t := t + 0.8))

    # Server output
    events.append(line(t := t + 0.1, f"  {BOLD}smuf-server{RESET} v0.3.0"))
    events.append(line(t := t + 0.05, f"  {DIM}━━━━━━━━━━━━━━━━━━━━{RESET}"))
    events.append(line(t := t + 0.05, f"  Control  {CYAN}:7000{RESET}"))
    events.append(line(t := t + 0.05, f"  HTTP     {CYAN}:8080{RESET}"))
    events.append(line(t := t + 0.05, f"  HTTPS    {YELLOW}disabled{RESET}"))
    events.append(line(t := t + 0.05, f"  Dashboard {CYAN}http://smuf.cdrusu.com:8080/{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.1, f"  {GREEN}✓ Ready.{RESET}  Waiting for clients..."))
    events.append(delay(t := t + 0.5))
    events.append(line(t := t + 0.1, ""))

    # ── 3. First tunnel: single port ──
    events.append(output(t, "$ "))
    events.append(prompt(t, "./smuf 3000", typing_speed=0.01))
    events.append(line(t := t + 0.6))
    events.append(delay(t := t + 1.0))

    events.append(line(t := t + 0.1, f"  {GREEN}{BOLD}Tunnel ready!{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.05, f"    {DIM}Local   →{RESET} {CYAN}http://localhost:3000{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Public  →{RESET} {BOLD}https://a3f1c9.smuf.cdrusu.com{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.1, f"    {DIM}Press Ctrl+C to stop{RESET}"))
    events.append(delay(t := t + 0.8))
    events.append(line(t := t + 0.1, ""))

    # ── 4. Multiple ports ──
    events.append(output(t, "$ "))
    events.append(prompt(t, "./smuf 3000 4000 5000", typing_speed=0.01))
    events.append(line(t := t + 0.7))
    events.append(delay(t := t + 1.0))

    events.append(line(t := t + 0.1, f"  {GREEN}{BOLD}Tunnel ready!{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.05, f"    {DIM}Local   →{RESET} {CYAN}http://localhost:3000{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Public  →{RESET} {BOLD}https://a3f1c9.smuf.cdrusu.com{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Local   →{RESET} {CYAN}http://localhost:4000{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Public  →{RESET} {BOLD}https://b8d2e4.smuf.cdrusu.com{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Local   →{RESET} {CYAN}http://localhost:5000{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Public  →{RESET} {BOLD}https://f7c1b2.smuf.cdrusu.com{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.1, f"    {DIM}Press Ctrl+C to stop{RESET}"))
    events.append(delay(t := t + 0.8))
    events.append(line(t := t + 0.1, ""))

    # ── 5. Custom subdomain ──
    events.append(output(t, "$ "))
    events.append(prompt(t, "./smuf --sub myapp 3000", typing_speed=0.01))
    events.append(line(t := t + 0.7))
    events.append(delay(t := t + 1.0))

    events.append(line(t := t + 0.1, f"  {GREEN}{BOLD}Tunnel ready!{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.05, f"    {DIM}Local   →{RESET} {CYAN}http://localhost:3000{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Public  →{RESET} {BOLD}https://myapp.smuf.cdrusu.com{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.1, f"    {DIM}Press Ctrl+C to stop{RESET}"))
    events.append(delay(t := t + 0.5))
    events.append(line(t := t + 0.1, ""))

    # ── 6. TCP tunnel ──
    events.append(output(t, "$ "))
    events.append(prompt(t, "./smuf --tcp 22", typing_speed=0.01))
    events.append(line(t := t + 0.6))
    events.append(delay(t := t + 1.0))

    events.append(line(t := t + 0.1, f"  {GREEN}{BOLD}Tunnel ready!{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.05, f"    {DIM}Local   →{RESET} {CYAN}tcp://localhost:22{RESET}"))
    events.append(line(t := t + 0.05, f"    {DIM}Public  →{RESET} {BOLD}tcp://smuf.cdrusu.com:21034{RESET}"))
    events.append(line(t := t + 0.05))
    events.append(line(t := t + 0.1, f"    {DIM}Press Ctrl+C to stop{RESET}"))
    events.append(delay(t := t + 0.5))
    events.append(line(t := t + 0.1, ""))

    # ── 7. Dashboard link ──
    events.append(output(t, "$ "))
    events.append(prompt(t, "# open http://smuf.cdrusu.com:8080/ for the dashboard", typing_speed=0.005))
    events.append(line(t := t + 0.8))
    events.append(delay(t := t + 0.3))
    events.append(line(t := t + 0.1, ""))
    events.append(output(t, "$ "))

    return header(), events


# ─── main ──────────────────────────────────────────────────────────────────

def main():
    hdr, events = build_cast()

    # Write asciinema v2 file
    with open(sys.argv[1], "w") as f:
        f.write(json.dumps(hdr) + "\n")
        for ev in events:
            f.write(json.dumps(ev) + "\n")

    print(f"✓ Cast written to {sys.argv[1]}")
    print(f"  Duration: ~{events[-1][0]:.1f}s")
    print(f"  Events: {len(events)}")
    print(f"  Convert to SVG: svg-term --cast={sys.argv[1]} --out demo.svg --window")


if __name__ == "__main__":
    main()
