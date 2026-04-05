{
  description = "Multi-arch dev environment for locals";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.11";
    nixpkgs-unstable.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        unstable = import nixpkgs-unstable { inherit system; };
      in {
        devShells.default = pkgs.mkShell {
          name = "locals";

          buildInputs = with pkgs; [
            mage
            shellcheck
            git
            curl
            unstable.go
	    mkcert
	    neovim
	  ];

          shellHook = ''
            export PATH=bin:$(go env GOPATH)/bin:$PATH
            export GOTOOLCHAIN=auto
            export GOSUMDB=sum.golang.org
            export EDITOR=nvim
            export CGO_CFLAGS="-O2"
          '';
        };
      }
    );
}

