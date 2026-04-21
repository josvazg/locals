#!/usr/bin/env bash

set -euo pipefail

if command -v dnf >/dev/null; then
	dnf install -y --skip-unavailable nix git
elif command -v apt-get >/dev/null; then
	apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y nix git
elif command -v pacman >/dev/null; then
	pacman -Syu --noconfirm nix git
elif command -v xbps-install >/dev/null; then
	xbps-install -Sy git nix
	ln -s /etc/sv/nix-daemon /var/service/
elif command -v nix-env >/dev/null; then
	nix-channel --add https://nixos.org/channels/nixpkgs-unstable nixpkgs
    	nix-channel --update
	nix-env -iA nixpkgs.git nixpkgs.nss
	export PATH="/root/.nix-profile/bin:/nix/var/nix/profiles/default/bin:$PATH"
fi

git config --global --add safe.directory /src
cd /src

export GODEBUG=netdns=go
RUN_NIX_FLAKE=(nix --extra-experimental-features 'nix-command flakes' develop path:.)
if [ $# -eq 0 ] || [ -z "$1" ]; then
    echo "run shell"
    "${RUN_NIX_FLAKE[@]}"
else
    echo "run command ${*}"
    "${RUN_NIX_FLAKE[@]}" -c "$@"
fi
