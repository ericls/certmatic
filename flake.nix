{
  description = "Certmatic development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = [
            # Go
            pkgs.go

            # Go tools
            pkgs.air
            pkgs.gofumpt
            pkgs.xcaddy

            pkgs.nodejs_22
            pkgs.corepack_22

            pkgs.pre-commit
          ];

          shellHook = ''
          '';
        };
      }
    );
}
