{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    textual-autocomplete = {
      # TODO: update to darrenburns once flake PRr gets merged
      url = "github:justdeeevin/textual-autocomplete/flake";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} rec {
      imports = [flake-parts.flakeModules.modules];

      systems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];

      flake.overlays.default = final: prev: (
        inputs.textual-autocomplete.overlays.default final prev
        // {
          posting = prev.callPackage ./package.nix {};
        }
      );

      flake.modules.homeManager.default = args: import ./home-manager.nix args // {overlay = flake.overlays.default;};

      perSystem = {
        pkgs,
        system,
        ...
      }: {
        _module.args.pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [
            inputs.textual-autocomplete.overlays.default
          ];
        };
        packages.default = pkgs.callPackage ./package.nix {};
      };
    };
}
