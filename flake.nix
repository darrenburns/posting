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
    flake-parts.lib.mkFlake {inherit inputs;} {
      imports = [flake-parts.flakeModules.modules flake-parts.flakeModules.easyOverlay];
      systems = ["x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin"];
      perSystem = {
        pkgs,
        system,
        ...
      }: rec {
        overlayAttrs = {
          posting = packages.default;
        };
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
