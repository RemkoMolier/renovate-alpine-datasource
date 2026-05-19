#!/bin/bash
# Setup script run by OpenHands at the start of every session.
# Docs: https://docs.openhands.dev/usage/prompting/repository

set -euo pipefail

# The default OpenHands runtime image (python-nodejs) does NOT include Go.
# Install the latest stable Go if it's missing or older than the floor in
# go.mod (currently 1.25). Latest version is queried from go.dev so no
# version pin lives in this script.
GO_MIN_MINOR=25

needs_install=true
if command -v go >/dev/null 2>&1; then
    current_minor=$(go env GOVERSION 2>/dev/null | sed 's/^go//' | awk -F. '{print $2}')
    if [[ -n "$current_minor" && "$current_minor" -ge "$GO_MIN_MINOR" ]]; then
        needs_install=false
    fi
fi

if $needs_install; then
    go_version=$(curl -fsSL https://go.dev/VERSION?m=text | head -1 | sed 's/^go//')
    echo "[setup] installing Go ${go_version}"
    curl -fsSL "https://go.dev/dl/go${go_version}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
    if command -v sudo >/dev/null 2>&1; then
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf /tmp/go.tar.gz
        go_bin=/usr/local/go/bin
    else
        rm -rf "$HOME/.local/go"
        mkdir -p "$HOME/.local"
        tar -C "$HOME/.local" -xzf /tmp/go.tar.gz
        go_bin="$HOME/.local/go/bin"
    fi
    rm -f /tmp/go.tar.gz
    export PATH="$go_bin:$PATH"
    # Persist for the agent's subsequent shells.
    if ! grep -qF "$go_bin" "$HOME/.bashrc" 2>/dev/null; then
        echo "export PATH=\"$go_bin:\$PATH\"" >> "$HOME/.bashrc"
    fi
fi

echo "[setup] $(go version)"

echo "[setup] downloading Go module dependencies"
go mod download

echo "[setup] done. Run 'make verify' before claiming any task complete."
echo "[setup] (first 'make lint' / 'make coverage' call fetches the pinned tools — ~30s cold, then cached)"
