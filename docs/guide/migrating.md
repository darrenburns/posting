## Overview

Posting 3 is a rewrite of Posting in Go, available as a public beta. It installs as a single binary
and adds GraphQL, gRPC, request tabs, layered environments, request timing, and remote control.

You can open your existing Posting 2 HTTP collections without converting them. Configuration,
keymaps, and themes have some differences, and Python scripts aren't run.

!!! warning "Sharing a collection with Posting 2"

    Ordinary HTTP requests use the shared `.posting.yaml` format, but Posting 3 adds fields that
    Posting 2 doesn't understand. Saving a request in Posting 2 removes `kind`, `graphql`, `grpc`,
    and `variable_scope`, losing the query, RPC settings, or scoped variables stored there.
    Edit GraphQL, gRPC, and requests with scoped variables only in Posting 3.

## What carries over

| | |
|-|-|
| **Collections** | Posting 3 opens Posting 2 collections. Ordinary HTTP requests without Posting 3 fields can be edited in either version. |
| **Configuration** | Posting 3 reads the same `config.yaml` and `POSTING_*` environment variables. A few settings aren't supported yet; see [Configuration](./configuration.md#settings-not-yet-supported). |
| **Environment files** | `.env` files work as before, and `--env` still accepts file paths. |
| **The default collection** | Posting 3 uses the same default collection directory. |
| **Keymaps** | Action IDs are the same where the action still exists. See [Keymaps](./keymap.md#coming-from-posting-2) for the differences. |
| **Themes** | Posting 2 theme files with hex colours are loaded, using their main colours. See [Themes from Posting 2](./themes.md#themes-from-posting-2). |

## What's new

- **GraphQL requests.** Edit a [query and its JSON variables](./requests.md#graphql-requests), choose an
  operation, and see GraphQL errors even when the HTTP status is `200`.
- **gRPC requests.** Discover methods through reflection or local proto files, send unary and streaming
  calls, and [keep a stream open](./requests.md#open-streams) to send messages as you work.
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
- **Coding agents.** Claude Code, Codex and other agents can send requests through the Posting you have
  open with [`posting remote`](./remote_control.md), so you watch what they send and what comes back.
  Install the [`using-posting` skill](./agents.md) to teach them how.
- **Bruno import.** `posting import` now reads Bruno requests and collections, and detects the format
  of any source by itself.
- **37 themes**, previewed live as you browse them.
- **Resizable panels.** Drag the dividers between the sidebar, request and response.
- **Remote control.** Use `posting remote` from another terminal or a coding agent to open, send,
  and save requests in the running app, using its active environment. Run `posting remote --help`
  for the command reference.

## What's changed

- **Installation.** Posting 3 is installed with Homebrew, from a release download or with
  `go install`. Updating a `uv` or `pipx` installation keeps you on Posting 2.
  See [Installation](./index.md#installation).
- **Some shortcuts moved.** Searching requests is now ++ctrl+g++, and expanding a panel is ++alt+z++.
  ++ctrl+n++ opens a new request tab.
- **Help.** ++f1++ shows all the keyboard shortcuts in one place, rather than help for the focused widget.
- **History** is stored separately in a new format. Posting 2 history isn't shown or removed.
- **Configuration.** `POSTING_*` settings are read from the shell environment, not from request
  `.env` files. Custom CA bundles must be PEM files, rather than directories of certificates.
  See [Configuration](./configuration.md) for supported settings.
- **The theme picker** previews themes as you move through them. Posting 2 themes that no longer exist
  fall back to `galaxy`.

## Not yet supported

These Posting 2 features aren't in Posting 3 yet:

- **Scripts.** Pre-request and post-response Python scripts aren't run. They're kept in your request files,
  so nothing is lost. See [Scripting](./scripting.md) for alternatives.
- **X resources themes** (`use_xresources`), and live reloading of theme files (`watch_themes`).
- **Custom syntax highlighting and method colours** in theme files. Colours come from the theme's
  main colours instead.

You can keep Posting 2 installed alongside Posting 3 for HTTP requests that need these features.
Follow the sharing restrictions above when you use the same collection in both versions.
Both install a command called `posting`, so whichever comes first on your `PATH` wins.
Run the other by its full path, or give it an alias in your shell.
