#!/usr/bin/env bash

set -euo pipefail

if [ ! -d /nix/store ]; then
    mkdir -p /nix/store
fi
mount --bind /host-nix/store /nix/store

"${HOST_NIX_PATH}/nix" --version
export PATH="${HOST_NIX_PATH}:/bin:${PATH}"
export NIX_STATE_DIR="/host-nix/var/nix"

if [ ! -f /etc/ssl/certs/ca-certificates.crt ]; then
    export NIX_SSL_CERT_FILE="/host-nix/var/nix/profiles/default/etc/ssl/certs/ca-bundle.crt"
fi

if ! command -v nix >/dev/null; then
    echo "❌ Error: Nix binary not found. Check if /nix/store is mounted correctly."
    exit 1
fi

# 3. Environment Compatibility
# Some distros (like Fedora/Ubuntu) might need Nss for networking in Go
export GODEBUG=netdns=go

# 4. Execute from the mapped source
cd /src

# We use --offline because we've shared the store from the host.
# This prevents the container from trying to reach the internet for stuff 
# that should already be on your SSD.
RUN_NIX_FLAKE=(nix --extra-experimental-features 'nix-command flakes' develop path:. --offline)

if [ $# -eq 0 ] || [ -z "$1" ]; then
    echo "--- Entering Nix Develop Shell ---"
    "${RUN_NIX_FLAKE[@]}"
else
    echo "--- Running command: ${*} ---"
    "${RUN_NIX_FLAKE[@]}" -c "$@"
fi

