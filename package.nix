{
  pkgs,
  lib,
  ...
}: let
  package = builtins.fromTOML (builtins.readFile ./pyproject.toml);
in
  pkgs.python312Packages.buildPythonPackage {
    pname = package.project.name;
    version = package.project.version;
    pyproject = true;
    src = ./.;
    build-system = [pkgs.python312Packages.hatchling];
    dependencies = with pkgs.python312Packages; [
      click
      xdg-base-dirs
      click-default-group
      httpx
      pyperclip
      pydantic
      pyyaml
      pydantic-settings
      python-dotenv
      (pkgs.textual-autocomplete.overridePythonAttrs
        (old: rec {
          version = "3.0.0a12";
          src = pkgs.fetchPypi {
            pname = "textual_autocomplete";
            inherit version;
            hash = "sha256-HSyeTSTH9XWryMYSy2q//0cG9qqrm5OVBrldroRUkwk=";
          };

          postPatch = ''
            sed -i "/^requires-python =.*/a version = '${version}'" pyproject.toml
          '';
        }))
      (textual.overridePythonAttrs (old: rec {
        version = "0.85.0";
        src = pkgs.fetchFromGitHub {
          owner = "Textualize";
          repo = "textual";
          rev = "refs/tags/v${version}";
          hash = "sha256-ROq/Pjq6XRgi9iqMlCzpLmgzJzLl21MI7148cOxHS3o=";
        };

        postPatch = ''
          sed -i "/^requires-python =.*/a version = '${version}'" pyproject.toml
        '';
      }))
      (watchfiles.overridePythonAttrs (old: rec {
        version = "0.24.0";

        src = pkgs.fetchFromGitHub {
          owner = "samuelcolvin";
          repo = "watchfiles";
          rev = "refs/tags/v${version}";
          hash = "sha256-uc4CfczpNkS4NMevtRxhUOj9zTt59cxoC0BXnuHFzys=";
        };

        cargoDeps = pkgs.rustPlatform.importCargoLock {
          lockFile = pkgs.fetchurl {
            url = "https://raw.githubusercontent.com/samuelcolvin/watchfiles/refs/tags/v${version}/Cargo.lock";
            hash = "sha256-rA6K0bjivOGhoGUYUk5OubFaMh3duEMaDgGtCqbY26g=";
          };
          outputHashes = {
            "notify-6.1.1" = "sha256-lT3R5ZQpjx52NVMEKTTQI90EWT16YnbqphqvZmNpw/I=";
          };
        };

        postPatch = ''
          sed -i "/^requires-python =.*/a version = '${version}'" pyproject.toml
          substituteInPlace Cargo.toml \
            --replace-fail 'version = "0.0.0"' 'version = "${version}"'
        '';
      }))
    ];
    meta = {
      description = package.project.description;
      homepage = package.project.urls.homepage;
      license = lib.licenses.asl20;
    };
  }
