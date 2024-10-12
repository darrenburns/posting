# Posting

**A powerful HTTP client that lives in your terminal.**

Posting is an HTTP client, not unlike Postman and Insomnia. As a TUI application, it can be used over SSH and enables efficient keyboard-centric workflows. Your requests are stored locally in simple YAML files, so they're easy to read and version control.

<img width="1337" alt="image" src="./docs/assets/home-image-ad-15aug24.svg">

Some notable features include:

- "jump mode" navigation
- environments/variables
- autocompletion
- syntax highlighting using tree-sitter
- Vim keys
- user-defined themes
- configuration
- "open in $EDITOR"
- a command palette for quickly accessing functionality

Visit the [website](https://posting.sh) for more information, the roadmap, and the user guide.

## Installation

Posting can be installed via [`pipx`](https://pipx.pypa.io/stable/) or [Rye](https://rye-up.com/guide/installation) on MacOS, Linux, and Windows:

```bash
pipx install posting
# or
rye install posting
```

### Rye is recommended

Rye is recommended, as it is significantly faster than Homebrew and `pipx`, and can install Posting in under a second.

```bash
# quick install on MacOS/Linux
curl -sSf https://rye.astral.sh/get | bash

# install Posting
rye install posting
```

Windows users should follow the guide [Rye](https://rye-up.com/guide/installation) to learn how to install Rye.


## Learn More

Learn more about Posting at [https://posting.sh](https://posting.sh).

Posting was built with [Textual](https://github.com/textualize/textual).
