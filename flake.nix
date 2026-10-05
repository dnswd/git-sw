{
  description = "git-sw Go environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "git-sw";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-xTPH5pwh4Reqnj6u4GuVyHDVQh5uoPeW+hUwY7aVdzA=";
          meta = {
            description = "Fuzzy git branch switcher";
            mainProgram = "git-sw";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            pkgs.go
            pkgs.git
          ];
        };
      }
    );
}
