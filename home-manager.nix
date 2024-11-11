{
  config,
  lib,
  pkgs,
  ...
}: let
  inherit (lib) mkOption mkIf mkEnableOption;
  cfg = config.programs.posting;
  posting = pkgs.callPackage ./flake.nix {inherit pkgs;};
in {
  options.programs.posting = {
    enable = mkEnableOption "Posting API client";
    # package = mkPackageOption pkgs "posting" {};
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
    home.packages = [posting];
    home.file.".config/posting/config.yaml".text = lib.genrators.toYAML cfg.settings;
  };
}
