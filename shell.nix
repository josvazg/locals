let
  pkgs = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/nixos-23.11.tar.gz";
    sha256 = "sha256:1f5d2g1p6nfwycpmrnnmc2xmcszp804adp16knjvdkj8nz36y1fg";
  }) {};

  unstable = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz";
    # sha256 = "sha256:15ypswq0yiwk5rsmkp2zkprs1gb2va5gj2nvwqai3d4d5l5vp79h";
  }) {};
in
pkgs.mkShell {
  name = "locals";

  buildInputs = [
    pkgs.curl
    pkgs.git
    unstable.go
    pkgs.vscodium
    pkgs.shellcheck
    pkgs.neovim
    pkgs.ripgrep
    pkgs.wl-clipboard
    pkgs.jq
    pkgs.gotools
    pkgs.gopls
    pkgs.mkcert
    pkgs.traefik
  ];

  shellHook = ''
    export PATH=$PATH:$(go env GOPATH)/bin
    export EDITOR=vim
    export CGO_CFLAGS="-O2" # makes delve work in vscodium
    alias mage='go tool mage'
  '';
}
