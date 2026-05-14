#!/bin/bash
# Real smuf demo for recording
# Run: bash ~/Desktop/smuf/scripts/real_demo.sh

set -e

SMUF_DIR="${SMUF_DIR:-$HOME/Desktop/smuf}"
DEMO_DIR=/tmp/smuf-demo

# Clean slate
pkill -f "smuf-server" 2>/dev/null || true
pkill -f "python3 -m http.server 3" 2>/dev/null || true
pkill -f "nc -l -k -p 4" 2>/dev/null || true
sleep 0.3
rm -rf "$DEMO_DIR"
mkdir -p "$DEMO_DIR"

# Copy binaries
cp "$SMUF_DIR/smuf" "$SMUF_DIR/smuf-server" "$DEMO_DIR/"
chmod +x "$DEMO_DIR/smuf" "$DEMO_DIR/smuf-server"

cd "$DEMO_DIR"

# Start fake services
python3 -m http.server 3000 --bind 127.0.0.1 &>/dev/null &
echo "  ✓ Demo service on port 3000"
python3 -m http.server 4000 --bind 127.0.0.1 &>/dev/null &
echo "  ✓ Demo service on port 4000"
python3 -m http.server 5000 --bind 127.0.0.1 &>/dev/null &
echo "  ✓ Demo service on port 5000"

sleep 0.5
echo ""

# ── Step 1: Start server ──
echo "  $ ./smuf-server"
echo ""

SMUF_DOMAIN=smuf.cdrusu.com \
SMUF_CONTROL_PORT=17003 \
SMUF_HTTP_PORT=18083 \
SMUF_AUTH_TOKEN=demo-token \
SMUF_TCP_PORT_RANGE=21000-21005 \
./smuf-server &>/tmp/smuf-server.log &
SERVER_PID=$!
sleep 1
echo "    Control port  :7000"
echo "    HTTP port     :8080"
echo "    Dashboard     http://smuf.cdrusu.com:8080/"
echo "    TCP tunnels   :21000-21005"
echo ""
echo "    ✓ Ready. Waiting for clients..."
echo ""

# ── Step 2: Single tunnel ──
echo "  $ ./smuf 3000"
echo ""
SMUF_SERVER=localhost:17003 SMUF_AUTH_TOKEN=demo-token timeout 5 ./smuf 3000 2>&1 || true
echo ""

# ── Step 3: Multiple ports ──
echo "  $ ./smuf 3000 4000 5000"
echo ""
SMUF_SERVER=localhost:17003 SMUF_AUTH_TOKEN=demo-token timeout 5 ./smuf 3000 4000 5000 2>&1 || true
echo ""

# ── Step 4: Custom subdomain ──
echo "  $ ./smuf --sub myapp 3000"
echo ""
SMUF_SERVER=localhost:17003 SMUF_AUTH_TOKEN=demo-token timeout 5 ./smuf --sub myapp 3000 2>&1 || true
echo ""

# ── Step 5: TCP tunnel ──
echo "  $ ./smuf --tcp 22"
echo ""
SMUF_SERVER=localhost:17003 SMUF_AUTH_TOKEN=demo-token timeout 5 ./smuf --tcp 22 2>&1 || true
echo ""

# Cleanup
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
pkill -f "python3 -m http.server 3" 2>/dev/null || true
pkill -f "python3 -m http.server 4" 2>/dev/null || true
pkill -f "python3 -m http.server 5" 2>/dev/null || true
echo ""
echo "  ✓ Demo complete"
