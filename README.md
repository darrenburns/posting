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

## Learn More

Learn more about Posting at [https://posting.sh](https://posting.sh).

Posting was built with [Textual](https://github.com/textualize/textual).
