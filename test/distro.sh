#!/usr/bin/env bash

set -euo pipefail

if command -v dnf >/dev/null; then
	dnf install -y --skip-unavailable golang git sudo nss3-tools
elif command -v apt-get >/dev/null; then
        apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y golang git sudo libnss3-tools
elif command -v pacman >/dev/null; then
        pacman -Syu --noconfirm go git sudo nss
elif command -v nix-env >/dev/null; then
	nix-channel --add https://nixos.org/channels/nixpkgs-unstable nixpkgs
    	nix-channel --update
    	nix-env -iA nixpkgs.go nixpkgs.git nixpkgs.nss
	export PATH="/root/.nix-profile/bin:/nix/var/nix/profiles/default/bin:$PATH"
fi

cd /src

export PATH="$PATH:$(go env GOPATH)/bin"
export GOTOOLCHAIN=auto
export GOSUMDB=sum.golang.org
go version
go install github.com/magefile/mage@latest
go install filippo.io/mkcert@latest

mage -v test
