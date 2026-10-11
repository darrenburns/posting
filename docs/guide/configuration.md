## Overview

Posting reads its settings from a YAML configuration file and from environment variables.
Every setting has a sensible default, so you only need to write down the ones you want to change.

Settings are applied in this order, with later sources winning:

1. Posting's defaults
2. The configuration file
3. `POSTING_*` environment variables

If a setting has a value Posting doesn't recognise (an unknown layout, spacing, sidebar position,
focus target or theme), that setting falls back to its default and a notification tells you when
the app starts. If the file isn't valid YAML, or a setting has the wrong type (for example a word
where `true` or `false` is expected), Posting ignores the whole file, uses the defaults, and tells
you why. Either way, a typo never stops Posting from opening.

!!! tip "Coming from Posting 2?"

    Posting 3 reads the same `config.yaml`, in the same place, so your existing configuration
    keeps working. A handful of Posting 2 settings are accepted but have no effect yet; they're
    marked in the [reference](#full-configuration-reference) below.

## The configuration file

Run `posting locate config` to see where Posting looks for its config file:

```bash
$ posting locate config
Config file:
/home/you/.config/posting/config.yaml
```

The file lives at `$XDG_CONFIG_HOME/posting/config.yaml`, which is `~/.config/posting/config.yaml`
unless you've set `XDG_CONFIG_HOME`. This is the same on macOS, Linux and Windows.
To use a different file, set `POSTING_CONFIG_FILE` to its path.

Here's an example:

```yaml
theme: aurora
layout: horizontal
spacing: compact
response:
  prettify_json: false
heading:
  show_host: false
focus:
  on_response: body
keymap:
  send-request: ctrl+r
```

## Environment variables

Any setting can also be set with an environment variable. Upper-case the name of the setting
and prefix it with `POSTING_`. For nested settings, join the levels with a double underscore:

| Setting            | Environment variable             |
|--------------------|----------------------------------|
| `theme`            | `POSTING_THEME`                  |
| `heading.show_host`| `POSTING_HEADING__SHOW_HOST`     |
| `history.enabled`  | `POSTING_HISTORY__ENABLED`       |

Boolean settings accept `true`/`false`, `1`/`0`, `yes`/`no` and `on`/`off`.

```bash
POSTING_THEME=lantern POSTING_LAYOUT=horizontal posting
```

Posting reads these from the environment it was started in, not from the `.env` files you use
for [environments](./environments.md). Those files hold variables for your requests, not
settings for the app.

## Configuring SSL

Posting verifies the certificates of HTTPS servers using your operating system's trust store.
You can switch verification off for a single request in its [Options tab](./requests.md#options).

### Custom CA bundle

If your servers use certificates signed by a private certificate authority, point Posting at a
PEM file containing the CA certificates:

```yaml
ssl:
  ca_bundle: /absolute/path/to/ca-bundle.pem
```

The certificates in the bundle are trusted *in addition to* the system's. `ca_bundle` must be a
file (a directory of certificates isn't supported).

### Client certificates

To present a client certificate to servers that ask for one:

```yaml
ssl:
  certificate_path: /path/to/client-cert.pem
  key_file: /path/to/client-key.pem  # optional
```

If you leave out `key_file`, Posting reads the private key from `certificate_path`, so a single
PEM containing both the certificate and the key works too.

The key must not be encrypted: `ssl.password` isn't supported in Posting 3 yet, and setting it
shows a warning at startup. Problems loading certificates are reported when you send a request.

### Per-environment certificates

Because every setting can come from an environment variable, you can pick certificates when you
start Posting:

```bash
POSTING_SSL__CA_BUNDLE=/path/to/dev-ca.pem posting --env dev
```

## Full configuration reference

The table below lists every setting, its default value, and what it does.
Each can also be set with the environment variable shown in brackets.

| Config key (env var) | Values (default) | Description |
|----------------------|------------------|-------------|
| `theme` (`POSTING_THEME`) | Any theme name (Default: `galaxy`) | The theme to use. See [Themes](./themes.md). |
| `theme_directory` (`POSTING_THEME_DIRECTORY`) | A path (Default: `$XDG_DATA_HOME/posting/themes`) | Where Posting looks for your own theme files. |
| `load_user_themes` (`POSTING_LOAD_USER_THEMES`) | `true`, `false` (Default: `true`) | Load the themes in `theme_directory`. |
| `load_builtin_themes` (`POSTING_LOAD_BUILTIN_THEMES`) | `true`, `false` (Default: `true`) | When `false` and you have at least one theme of your own, the theme picker lists only your themes. |
| `layout` (`POSTING_LAYOUT`) | `vertical`, `horizontal` (Default: `vertical`) | `vertical` stacks the response below the request; `horizontal` puts them side by side. |
| `spacing` (`POSTING_SPACING`) | `standard`, `compact` (Default: `standard`) | `compact` removes the blank rows between parts of the UI. Posting also switches to compact spacing on its own when the terminal is short. |
| `nerd_fonts` (`POSTING_NERD_FONTS`) | `true`, `false` (Default: detected) | Draw icons from a [Nerd Font](https://www.nerdfonts.com/). When unset, icons are used in terminals known to ship Nerd Font symbols (Ghostty), and plain characters elsewhere. |
| `use_host_environment` (`POSTING_USE_HOST_ENVIRONMENT`) | `true`, `false` (Default: `false`) | Make your shell's environment variables available as variables in requests. See [Environments](./environments.md#using-your-shells-environment). |
| `watch_env_files` (`POSTING_WATCH_ENV_FILES`) | `true`, `false` (Default: `true`) | Reload the active environment when its files change on disk. |
| `watch_collection_files` (`POSTING_WATCH_COLLECTION_FILES`) | `true`, `false` (Default: `true`) | Reload the collection when request files are added, changed or removed on disk. |
| `history.enabled` (`POSTING_HISTORY__ENABLED`) | `true`, `false` (Default: `true`) | Save [history](./collections.md#history) to disk so it's there next time. When `false`, history is kept only until you quit. |
| `response.prettify_json` (`POSTING_RESPONSE__PRETTIFY_JSON`) | `true`, `false` (Default: `true`) | Indent JSON responses for display. |
| `response.show_size_and_time` (`POSTING_RESPONSE__SHOW_SIZE_AND_TIME`) | `true`, `false` (Default: `true`) | Show the size of the response and how long it took. |
| `heading.visible` (`POSTING_HEADING__VISIBLE`) | `true`, `false` (Default: `true`) | Show the header row at the top of the app, which shows the environment and host. |
| `heading.show_host` (`POSTING_HEADING__SHOW_HOST`) | `true`, `false` (Default: `true`) | Show `user@host` in the header. |
| `heading.hostname` (`POSTING_HEADING__HOSTNAME`) | Text (Default: unset) | Replace `user@host` in the header with your own text. Markup such as `[b]prod[/]` is allowed. |
| `heading.show_version` (`POSTING_HEADING__SHOW_VERSION`) | `true`, `false` (Default: `true`) | Show the Posting logo and version in the bottom left corner. |
| `url_bar.show_value_preview` (`POSTING_URL_BAR__SHOW_VALUE_PREVIEW`) | `true`, `false` (Default: `true`) | Show the line below the URL bar that previews variable values and the resolved URL. |
| `url_bar.hide_secrets_in_value_preview` (`POSTING_URL_BAR__HIDE_SECRETS_IN_VALUE_PREVIEW`) | `true`, `false` (Default: `true`) | Mask the value of a variable in the preview when its name looks secret. See [Environments](./environments.md#secrets). |
| `collection_browser.position` (`POSTING_COLLECTION_BROWSER__POSITION`) | `left`, `right` (Default: `left`) | Which side of the screen the sidebar is on. |
| `collection_browser.show_on_startup` (`POSTING_COLLECTION_BROWSER__SHOW_ON_STARTUP`) | `true`, `false` (Default: `true`) | Show the sidebar when Posting starts. Toggle it any time with ++ctrl+h++. |
| `command_palette.theme_preview` (`POSTING_COMMAND_PALETTE__THEME_PREVIEW`) | `true`, `false` (Default: `true`) | Preview each theme as you move through the theme picker. |
| `text_input.blinking_cursor` (`POSTING_TEXT_INPUT__BLINKING_CURSOR`) | `true`, `false` (Default: `true`) | Blink the cursor in inputs and text areas. |
| `focus.on_startup` (`POSTING_FOCUS__ON_STARTUP`) | `url`, `method`, `collection` (Default: `url`) | What to focus when Posting starts. |
| `focus.on_response` (`POSTING_FOCUS__ON_RESPONSE`) | `body`, `tabs` (Default: unset) | Move focus to the response body, or the response tabs, when a response arrives. |
| `focus.on_request_open` (`POSTING_FOCUS__ON_REQUEST_OPEN`) | `url`, `method`, `headers`, `body`, `path`, `query`, `info` (Default: unset) | What to focus when you open a request from the collection. |
| `editor` (`POSTING_EDITOR`) | A command (Default: `$EDITOR`) | The editor to open text in. See [External Tools](./external_tools.md). |
| `pager` (`POSTING_PAGER`) | A command (Default: `$PAGER`) | The pager to view text in. |
| `pager_json` (`POSTING_PAGER_JSON`) | A command (Default: unset) | The pager to view JSON in, instead of `pager`. |
| `curl_export_extra_args` (`POSTING_CURL_EXPORT_EXTRA_ARGS`) | Text (Default: empty) | Inserted straight after `curl` in [exported curl commands](./requests.md#exporting-as-curl), e.g. `--silent --show-error`. |
| `keymap` | Action IDs and keys (Default: empty) | Change keyboard shortcuts. See [Keymaps](./keymap.md). |
| `remote_control.enabled` (`POSTING_REMOTE_CONTROL__ENABLED`) | `true`, `false` (Default: `true`) | Let `posting remote` drive Posting from another terminal. See [Remote Control](./remote_control.md). |
| `ssl.ca_bundle` (`POSTING_SSL__CA_BUNDLE`) | Path to a PEM file (Default: unset) | Extra certificate authorities to trust. |
| `ssl.certificate_path` (`POSTING_SSL__CERTIFICATE_PATH`) | Path (Default: unset) | A client certificate to present. |
| `ssl.key_file` (`POSTING_SSL__KEY_FILE`) | Path (Default: unset) | The private key for the client certificate. |

### Settings not yet supported

These Posting 2 settings are still accepted, so an old config file loads cleanly, but they don't
do anything in Posting 3 yet:

| Setting | Notes |
|---------|-------|
| `ssl.password` | Encrypted client keys aren't supported. Setting this shows a warning. |
| `watch_themes` | Theme files are read when Posting starts. Restart to pick up changes. |
| `use_xresources` | X resources themes aren't available. |
| `animation` | Posting 3 has no animation levels. |
