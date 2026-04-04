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
	testDeps = with pkgs; [
          mage
          shellcheck
          git
          curl
          unstable.go
	  mkcert
        ];
      in {
        apps.test = {
	        type = "app";
          program = "${pkgs.writeShellScriptBin "mage-test" ''
            export PATH=${pkgs.lib.makeBinPath testDeps}:$PATH
            # Run the actual mage command with sudo here
            sudo -E ${pkgs.mage}/bin/mage "$@"
          ''}/bin/mage-test";
        };

        devShells.default = pkgs.mkShell {
          name = "locals";

          buildInputs = testDeps ++ [ pkgs.neovim ] ;

          shellHook = ''
            export PATH=bin:$(go env GOPATH)/bin:$PATH
            export EDITOR=nvim
            export CGO_CFLAGS="-O2"
          '';
        };
      }
    );
}

