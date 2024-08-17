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
- user-defined themes
- configuration
- "open in $EDITOR"
- a command palette for quickly accessing functionality

Visit the [website](https://posting.sh) for more information, the roadmap, and the user guide.

## Installation

Posting can be installed via [`pipx`](https://pipx.pypa.io/stable/) or Homebrew:

```bash
pipx install posting
# OR
brew install darrenburns/homebrew/posting
```

## Learn More

Learn more about Posting at [https://posting.sh](https://posting.sh).

Posting was built with [Textual](https://github.com/textualize/textual).
