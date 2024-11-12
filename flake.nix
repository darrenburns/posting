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
      flake.modules.homeManager.default = {
        config,
        lib,
        pkgs,
        ...
      }: let
        inherit (lib) mkOption mkIf mkEnableOption mkPackageOption;
        cfg = config.programs.posting;
      in {
        options.programs.posting = {
          enable = mkEnableOption "Posting API client";
          package = mkPackageOption pkgs "posting" {};
          settings = mkOption {
            type = (pkgs.formats.yaml {}).type;
            default = {};
            example = {
              theme = "galaxy";
              layout = "horizontal";
              response.prettify_json = false;
              heading = {
                visible = true;
                show_host = false;
              };
            };
            description = "Posting configuration settings. See <https://github.com/darrenburns/posting/blob/main/docs/guide/configuration.md>";
          };
        };

        config = mkIf cfg.enable {
          home.packages = [cfg.package];
          home.file.".config/posting/config.yaml".text = builtins.toJSON cfg.settings;
          nixpkgs.overlays = [flake.overlays.default];
        };
      };
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
