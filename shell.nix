let
  pkgs = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/nixos-25.11.tar.gz";
    #sha256 = "sha256:15gvdgdqsxjjihq1r66qz1q97mlcaq1jbpkhbx287r5py2vy38b1";
  }) {};

  unstable = import (builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz";
    #sha256 = "sha256:15gvdgdqsxjjihq1r66qz1q97mlcaq1jbpkhbx287r5py2vy38b1";
  }) {};
in
pkgs.mkShell {
  name = "locals";

  buildInputs = [
    pkgs.curl
    pkgs.git
    unstable.go
    pkgs.shellcheck
    pkgs.neovim
    pkgs.mkcert
  ];

  shellHook = ''
    export PATH=$PATH:$(go env GOPATH)/bin
    export EDITOR=nvim
    export CGO_CFLAGS="-O2" # makes delve work in vscodium
  '';
}
