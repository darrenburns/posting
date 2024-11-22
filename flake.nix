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
        inherit (lib) mkOption mkIf mkEnableOption mkPackageOption types;
        cfg = config.programs.posting;
        hexColor = lib.mkOptionType {
          name = "Hex Color";
          description = "hex-encoded color string";
          check = x:
            builtins.isString x && (builtins.match "#[0-9a-fA-F]{6}" x) != null;
          merge = lib.mergeOneOption;
        };
        mkColorOption = desc:
          mkOption {
            type = hexColor;
            description = desc;
          };
        mkStringOption = desc:
          mkOption {
            type = types.nullOr types.str;
            default = null;
            description = desc;
          };
      in {
        options.programs.posting = {
          enable = mkEnableOption "Posting API client";
          package = mkPackageOption pkgs "posting" {};
          settings = {
            theme = mkOption {
              type = types.enum ([
                  "textual-dark"
                  "textual-light"
                  "nord"
                  "gruvbox"
                  "catppuccin-mocha"
                  "dracula"
                  "tokyo-night"
                  "monokai"
                  "flexoki"
                  "catppuccin-latte"
                  "solarized-light"
                  "galaxy"
                  "nebula"
                  "sunset"
                  "aurora"
                  "nautilus"
                  "cobalt"
                  "twilight"
                  "hacker"
                  "manuscript"
                ]
                ++ lib.optionals cfg.settings.load_user_themes (builtins.attrNames cfg.themes)
                ++ lib.optionals cfg.settings.use_xresources ["xresources-dark" "xresources-light"]);
              default = "galaxy";
              description = "Sets the theme of the application.";
            };
            load_user_themes = mkOption {
              type = types.bool;
              default = true;
              description = "If enabled, load user themes from the theme directory, allowing them to be specified in config and selected via the command palette.";
            };
            load_builtin_themes = mkOption {
              type = types.bool;
              default = true;
              description = "If enabled, load builtin themes, allowing them to be specified in config and selected via the command palette.";
            };
            theme_directory = mkOption {
              type = types.str;
              default = "${config.xdg.dataHome}/posting/themes";
              description = "The directory containing user themes.";
            };
            layout = mkOption {
              type = types.enum ["horizontal" "vertical"];
              default = "horizontal";
              description = "Sets the layout of the application.";
            };
            use_host_environment = mkOption {
              type = types.bool;
              default = true;
              description = "Allow/deny using environment variables from the host machine in requests via `$env:` syntax. When disabled, only variables defined explicitly in `.env` files will be available for use.";
            };
            watch_env_files = mkOption {
              type = types.bool;
              default = true;
              description = "If enabled, automatically reload environment files when they change.";
            };
            watch_themes = mkOption {
              type = types.bool;
              default = true;
              description = "If enabled, automatically reload themes in the theme directory when they change on disk.";
            };
            animation = mkOption {
              type = types.enum ["none" "basic" "full"];
              default = "none";
              description = "Controls the animation level.";
            };
            response = {
              prettify_json = mkOption {
                type = types.bool;
                default = true;
                description = "If enabled, JSON responses will be pretty-formatted.";
              };
              show_size_and_time = mkOption {
                type = types.bool;
                default = true;
                description = "If enabled, the size and time taken for the response will be displayed in the response area border subtitle.";
              };
            };
            heading = {
              visible = mkOption {
                type = types.bool;
                default = true;
                description = "Show/hide the app header.";
              };
              show_host = mkOption {
                type = types.bool;
                default = true;
                description = "Show/hide the version in the app header.";
              };
              show_version = mkOption {
                type = types.bool;
                default = true;
                description = "Show/hide the version in the app header.";
              };
              hostname = mkStringOption "The hostname to display in the app header. You may use Rich markup here. If unset, the hostname provided via `socket.gethostname()` will be used.";
            };
            url_bar.show_value_preview = mkOption {
              type = types.bool;
              default = true;
              description = "Show/hide the variable value preview below the URL bar.";
            };
            collection_browser = {
              position = mkOption {
                type = types.enum ["left" "right"];
                default = "left";
                description = "The position of the collection browser on the screen.";
              };
              show_on_startup = mkOption {
                type = types.bool;
                default = true;
                description = "Show/hide the collection browser on startup Can always be toggled using the command palette.";
              };
            };
            pager = mkStringOption "Command to use for paging text.";
            pager_json = mkStringOption "Command to use for paging JSON.";
            editor = mkStringOption "Command to use for opening files in an external editor.";
            ssl = {
              ca_bundle = mkStringOption "Absolute path to a CA bundle file/dir. If not set, the Certifi CA bundle will be used.";
              certificate_path = mkStringOption "Absolute path to a client SSL certificate file or directory.";
              key_file = mkStringOption "Absolute path to a client SSL key file.";
              password = mkStringOption "Password to decrypt the key file if it's encrypted.";
            };
            focus = {
              on_startup = mkOption {
                type = types.enum ["url" "method" "collection"];
                default = "url";
                description = "Automatically focus the URL bar, method, or collection browser when the app starts.";
              };
              on_response = mkOption {
                type = types.nullOr (types.enum ["body" "tabs"]);
                default = null;
                description = "Automatically focus the URL bar, method, or collection browser when the app starts.";
              };
              on_request_open = mkOption {
                type = types.nullOr (types.enum ["headers" "body" "query" "info" "url" "method"]);
                default = null;
                description = "Automatically focus the specified target when a request is opened from the collection browser.";
              };
            };
            text_input.blinking_cursor = mkOption {
              type = types.bool;
              default = true;
              description = "If enabled, the cursor will blink in input widgets and text area widgets.";
            };
            command_palette.theme_preview = mkOption {
              type = types.bool;
              default = true;
              description = "If enabled, the command palette will display a preview of the selected theme when the cursor is over it. This will slow down cursor movement and so is disabled by default.";
            };
            use_xresources = mkOption {
              type = types.bool;
              default = false;
              description = "Try to create themes called `xresources-dark` and `xresources-light`";
            };
          };
          themes = mkOption {
            type = types.attrsOf (types.submodule {
              options = {
                author = mkStringOption "Author of the theme.";
                description = mkStringOption "Description of the theme.";
                homepage = mkStringOption "Homepage of the theme.";

                primary = mkColorOption "Buttons, fixed table columns";
                secondary = mkColorOption "Method selector, some minor labels";
                accent = mkColorOption "Header text, scrollbars, cursors, focus highlights";
                background = mkColorOption "Background colors";
                surface = mkColorOption "Panels, etc";
                error = mkColorOption "Error messages";
                success = mkColorOption "Success messages";
                warning = mkColorOption "Warning messages";

                text_area = {
                  cursor = mkStringOption "The block cursor";
                  cursor_line = mkStringOption "The line the cursor is on";
                  selection = mkStringOption "The selected text";
                  gutter = mkStringOption "The gutter";
                  matched_bracket = mkStringOption "The matched bracket";
                };

                url = {
                  base = mkStringOption "The 'base' of the URL";
                  protocol = mkStringOption "The protocol";
                };

                syntax = {
                  json_key = mkStringOption "JSON keys";
                  json_number = mkStringOption "JSON numbers";
                  json_string = mkStringOption "JSON strings";
                  json_boolean = mkStringOption "JSON booleans";
                  json_null = mkStringOption "JSON null values";
                };

                method = {
                  get = mkStringOption "GET method";
                  post = mkStringOption "POST method";
                  put = mkStringOption "PUT method";
                  delete = mkStringOption "PATCH method";
                  patch = mkStringOption "PATCH method";
                  options = mkStringOption "OPTIONS method";
                  head = mkStringOption "HEAD method";
                };
              };
            });
            default = {};
            description = "List of user-defined themes. See <https://github.com/darrenburns/posting/blob/main/docs/guide/themes.md>";
          };
        };

        config = mkIf cfg.enable {
          home.packages = [
            (
              if cfg.settings.use_xresources
              then cfg.package.override {inherit (cfg.settings) use_xresources;}
              else cfg.package
            )
          ];
          home.file =
            {".config/posting/config.yaml".text = builtins.toJSON cfg.settings;}
            // (lib.mapAttrs' (name: value: {
                name = "${cfg.settings.theme_directory}/${name}.yaml";
                value = {text = builtins.toJSON (value // {inherit name;});};
              })
              cfg.themes);

          nixpkgs.overlays = [flake.overlays.default];
        };
      };

      perSystem = {
        pkgs,
        system,
        ...
      }: rec {
        _module.args.pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [
            inputs.textual-autocomplete.overlays.default
          ];
        };
        packages.default = pkgs.callPackage ./package.nix {};
        devShells.default = pkgs.mkShell {
          inputsFrom = [packages.default];
          packages = [pkgs.python312Packages.textual-dev];
        };
      };
    };
}
