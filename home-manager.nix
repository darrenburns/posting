{
  config,
  lib,
  pkgs,
  overlay,
  ...
}: let
  inherit (lib) mkOption mkIf mkEnableOption mkPackageOption types;
  cfg = config.programs.posting;
in {
  options.programs.posting = {
    enable = mkEnableOption "Posting API client";
    package = mkPackageOption pkgs "posting" {};
    settings = mkOption {
      type = types.submodule {
        theme = types.enum ["galaxy" "posting" "monokai" "solarized-light" "nautilus" "nebula" "alpine" "cobalt" "twilight" "hacker"] ++ builtins.map cfg.themes (theme: theme.name);
      };
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
    themes = mkOption {
      type = types.listOf ((pkgs.formats.yaml {}).type);
      default = {};
      description = "List of user-defined themes. See <https://github.com/darrenburns/posting/blob/main/docs/guide/themes.md>";
    };
  };

  config = mkIf cfg.enable {
    home.packages = [cfg.package];
    home.file =
      {".config/posting/config.yaml".text = builtins.toJSON cfg.settings;}
      // builtins.listToAttrs (map (theme: {
          name = ".local/share/posting/themes/${theme.name}.yaml";
          value = {text = builtins.toJSON theme;};
        })
        cfg.themes);
    nixpkgs.overlays = [overlay];
  };
}
