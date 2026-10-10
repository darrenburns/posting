## Overview

Posting 3 is a rewrite of Posting in Go. It starts quickly, installs as a single binary, and
brings request tabs, layered environments, a variables screen, request timing and a new set of
themes.

It's designed to pick up where Posting 2 left off: it reads the same files, from the same places,
so you can point it at your existing collections and carry on.

## What carries over

| | |
|-|-|
| **Collections** | Request files use the same `.posting.yaml` format. Posting 3 can open collections made by Posting 2, and Posting 2 can open requests saved by Posting 3. |
| **Configuration** | Posting 3 reads the same `config.yaml` and `POSTING_*` environment variables. A few settings aren't supported yet; see [Configuration](./configuration.md#settings-not-yet-supported). |
| **Environment files** | `.env` files work as before, and `--env` still accepts file paths. |
| **The default collection** | Posting 3 uses the same default collection directory. |
| **Keymaps** | Action IDs are the same where the action still exists. See [Keymaps](./keymap.md#coming-from-posting-2) for the differences. |
| **Themes** | Posting 2 theme files with hex colours are loaded, using their main colours. See [Themes from Posting 2](./themes.md#themes-from-posting-2). |

## What's new

- **Request tabs.** Open as many requests as you like, each in its own tab, and send them at the
  same time. Browsing the collection uses a single [preview tab](./navigation.md#preview-tabs), so
  tabs don't pile up.
- **Layered environments.** A shared `posting.env` base, per-environment files like `staging.env`, and
  `.local.env` files for secrets that stay out of version control. Start in one by name with
  `--env staging`. Posting remembers the environment you [switch](./environments.md#switching-environments)
  to in each collection, and starts in it next time.
- **Request timing.** The [Trace](./responses.md#trace) tab shows how long each stage of an exchange
  took, and the URL bar shows its progress while it's in flight. Timing is kept in history too.
- **Collection search.** Press ++ctrl+g++ to [filter the collection](./navigation.md#searching-for-requests)
  as you type, by name, folder or method.
- **Curl export with or without variables.** Export a request as a curl command that's ready to run, or
  one that keeps its `${VARIABLES}` for sharing.
- **Bruno import.** `posting import` now reads Bruno requests and collections, and detects the format
  of any source by itself.
- **37 themes**, previewed live as you browse them.
- **Resizable panels.** Drag the dividers between the sidebar, request and response.

## What's changed

- **Installation.** Posting 3 is installed with Homebrew, from a release download or with
  `go install`, not with `uv` or `pipx`. See [Installation](./index.md#installation).
- **Some shortcuts moved.** Searching requests is now ++ctrl+g++, and expanding a panel is ++alt+z++.
  ++ctrl+n++ opens a new request tab.
- **Help.** ++f1++ shows all the keyboard shortcuts in one place, rather than help for the focused widget.
- **History** is stored in a new format, so history from Posting 2 isn't shown.
- **The theme picker** previews themes as you move through them. Posting 2 themes that no longer exist
  fall back to `galaxy`.

## Not yet supported

These Posting 2 features aren't in Posting 3 yet:

- **Scripts.** Pre-request and post-response Python scripts aren't run. They're kept in your request files,
  so nothing is lost. See [Scripting](./scripting.md) for alternatives.
- **X resources themes** (`use_xresources`), and live reloading of theme files (`watch_themes`).
- **Custom syntax highlighting and method colours** in theme files. Colours come from the theme's
  main colours instead.

Posting 2 continues to work with the same files, so you can keep it installed alongside Posting 3 if you
need any of these. Both install a command called `posting`, so whichever comes first on your `PATH` wins:
run the other by its full path, or give it an alias in your shell.
