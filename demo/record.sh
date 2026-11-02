#!/bin/sh
# Records docs/assets/readme.gif from demo/readme.tape. Posting runs against
# demo/server.py with its config and data in a temporary directory, so your
# own settings, history and environments stay out of the recording.
#
# Needs vhs, python3 and the BlexMono Nerd Font Mono font.
set -eu

cd "$(dirname "$0")/.."
DEMO_HOME=$(mktemp -d)
export DEMO_HOME
trap 'pkill -f "demo/server.py" || true; rm -rf "$DEMO_HOME"' EXIT

version=$(git describe --tags --abbrev=0 --match 'v3*' 2>/dev/null | sed 's/^v//')
go build -ldflags "-X main.version=${version:-3.0.0-beta.0}" -o "$DEMO_HOME/bin/posting" ./cmd/posting

mkdir -p "$DEMO_HOME/config/posting" "$DEMO_HOME/data"
cat > "$DEMO_HOME/config/posting/config.yaml" <<'EOF'
nerd_fonts: true
heading:
  hostname: you@devbox
EOF

vhs demo/readme.tape
