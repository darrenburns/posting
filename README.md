# Posting

**A powerful HTTP client that lives in your terminal.**

Posting is an HTTP client, not unlike Postman and Insomnia. As a TUI application, it can be used over SSH and enables efficient keyboard-centric workflows. Your requests are stored locally in simple YAML files, so they're easy to read and version control.

<img width="1337" alt="image" src="https://github.com/darrenburns/posting/assets/5740731/77f50aa0-bc93-4e42-b06b-c209ec233fe8">

Some notable features include:

- "jump mode" navigation
- environments/variables
- autocompletion
- syntax highlighting using tree-sitter
- Vim keys
- customizable keybindings
- user-defined themes
- run Python code before and after requests
- configuration
- "open in $EDITOR"
- a command palette for quickly accessing functionality

Visit the [website](https://posting.sh) for more information, the roadmap, and the user guide.

## Installation

Posting can be installed via [uv](https://docs.astral.sh/uv/getting-started/installation/) on MacOS, Linux, and Windows.

`uv` is a single Rust binary that you can use to install Python packages in an isolated environment on MacOS, Linux, and Windows. It's significantly faster than alternative tools, and will have you up and running in seconds.

You don't even need to worry about installing Python yourself - `uv` will manage everything for you.

```bash
# quick install on MacOS/Linux
curl -LsSf https://astral.sh/uv/install.sh | sh

# install Posting
uv tool install --python 3.11 posting
```

`uv` can also be installed via Homebrew, Cargo, Winget, pipx, and more. See the [installation guide](https://docs.astral.sh/uv/getting-started/installation/) for more information.

## Learn More

Learn more about Posting at [https://posting.sh](https://posting.sh).

Posting was built with [Textual](https://github.com/textualize/textual).
