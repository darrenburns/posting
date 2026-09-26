# Posting

**A powerful HTTP client that lives in your terminal.**

Posting is an HTTP client, not unlike Postman and Insomnia. As a TUI application, it can be used over SSH and enables efficient keyboard-centric workflows. Your requests are stored locally in simple YAML files, so they're easy to read and version control.

<img width="968" alt="image" src="https://github.com/user-attachments/assets/78359ab0-5e0c-4c0b-a60b-dce06b11bbf5" />

Some notable features include:

- "jump mode" navigation
- environments/variables
- autocompletion
- syntax highlighting using tree-sitter
- Vim keys
- customizable keybindings
- user-defined themes
- run Python code before and after requests
- extensive configuration
- open in $EDITOR/$PAGER
- import curl commands by pasting them into the URL bar
- export requests as cURL commands
- import from Postman and OpenAPI specs
- a command palette for quickly accessing functionality

Visit the [website](https://posting.sh) for more information, the roadmap, and the user guide.

## Installation

Posting can be installed via [uv](https://docs.astral.sh/uv/getting-started/installation/) on MacOS, Linux, and Windows.

```bash
# quickly install uv on MacOS/Linux
curl -LsSf https://astral.sh/uv/install.sh | sh

# install Posting (will also quickly install Python 3.13 if needed)
uv tool install --python 3.13 posting
```

 Now you can run Posting via the command line:

```bash
posting
```

Homebrew and NixOS are not officially supported at the moment.

### Prefer `pipx`?

If you'd prefer to use `pipx`, that works too: `pipx install posting`.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines, information on setting up your development environment, and running tests.

### Nix

This repository contains a Nix flake that can be used to install Posting on NixOS. The package is exposed as `outputs.packages.${system}.default`. The flake also provides an overlay at `outputs.overlays.default`. Lastly, if you'd like to configure Posting with Nix, the flake exposes a Home Manager module that allows for this to be done. Adding `outputs.modules.homeManager.default` to your Home Manager imports will add the `programs.posting` option. You can find documentation for this option [here](https://posting.sh/guide/home_manager).

Example flake using Posting:

```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

    home-manager = {
      url = "github:nix-community/home-manager";
      inputs.nixpkgs.follows = "nixpkgs";
    };

    posting = {
      url = "github:darrenburns/posting";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = {nixpkgs, posting, home-manager, ...}: let
    system = "x86_64-linux";
  in {
    nixosConfigurations.example = nixpkgs.lib.nixosSystem {
      modules = [
      home-manager.nixosModules.default
      ({pkgs, ...}: {
        # Use the overlay:
        nixpkgs.overlays = [posting.overlays.default];
        environment.systemPackages = [pkgs.posting];

        # Or, use the package directly:
        environment.systemPackages = [posting.packages.${system}.default];

        # Or, use the Home Manager options:
        home-manager.users.example = {
          programs.posting = {
            enable = true;
            # settings...
            # See Home Manager in the guide for more information
          };
        };
      })];
    };
  };
}

```

**Note on XResources**: if you wish to use the XResources themes, the home-manager module will take care of installing xrdb for you. However, if you're adding posting to your packages list manually, you will either have to add xrdb as well or use an override on Posting:

```nix
environment.systemPackages = [pkgs.posting.override {use_xresources = true;}];
```

## Learn More

Learn more about Posting at [https://posting.sh](https://posting.sh).

Posting was built with [Textual](https://github.com/textualize/textual).
