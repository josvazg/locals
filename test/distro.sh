#!/usr/bin/env bash

set -euo pipefail

if command -v dnf >/dev/null; then
	dnf install -y --skip-unavailable nix
elif command -v apt-get >/dev/null; then
    apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y nix
elif command -v pacman >/dev/null; then
    pacman -Syu --noconfirm nix
elif command -v nix-env >/dev/null; then
	nix-channel --add https://nixos.org/channels/nixpkgs-unstable nixpkgs
    	nix-channel --update
    	nix-env -iA nixpkgs.go nixpkgs.git nixpkgs.nss
	export PATH="/root/.nix-profile/bin:/nix/var/nix/profiles/default/bin:$PATH"
fi

cd /src

nix --extra-experimental-features 'nix-command flakes' develop path:. -c "$@"
