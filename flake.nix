{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };
  outputs =
    { self, nixpkgs }:
    {
      devShells = builtins.mapAttrs (system: pkgs: {
        default = import ./shell.nix { inherit pkgs; };
      }) nixpkgs.legacyPackages;
      formatter = builtins.mapAttrs (system: pkgs: pkgs.nixfmt) nixpkgs.legacyPackages;
    };
}
