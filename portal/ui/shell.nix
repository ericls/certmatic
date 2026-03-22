# shell.nix
{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.nodejs_22
    pkgs.corepack_22
  ];

  shellHook = ''
    echo "Node.js version $(node -v) is available."
  '';
}
