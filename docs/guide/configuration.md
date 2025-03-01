## Overview

Posting can be configured using a configuration file, environment variables, and/or `.env` files.

Configuration values are loaded in the following order of precedence (highest to lowest):

1. Configuration file
2. Environment variables
3. `.env` files

## Configuration file

You can write configuration for Posting using YAML.

The location of the config file can be checked using the command `posting locate config`.

Here's an example configuration file:

```yaml
theme: galaxy
layout: horizontal
response:
  prettify_json: false
heading:
  visible: true
  show_host: false
```

## Environment variables

All configuration values can also be set as environment variables.

Simply prefix the name of the config with `POSTING_` and set it as an environment variable.

For nested configuration values, use `__` as the delimiter. So to set `heading.visible` to `false`, you can set the environment variable `POSTING_HEADING__VISIBLE=false`.

For example, to set the theme to `galaxy`, you can set the environment variable `POSTING_THEME=galaxy`.

### dotenv (`.env`) files

Posting also supports `.env` (dotenv) files, which are useful if you want to swap out environment variable values depending on the environment you're working in (for example, "dev" vs "prod").

You can tell Posting to use a `.env` file using the `--env` option.
This option can be supplied multiple times to load multiple `.env` files.

Here's an example `.env` file:

```bash
POSTING_THEME="cobalt"
POSTING_LAYOUT="vertical"
POSTING_HEADING__VISIBLE="false"
```

Dotenv files are separate from collections, although you may wish to include them inside a collection to make it easy to version and share with others.

## Configuring SSL

Posting verifies SSL certificates by default using the CA bundle provided by the `certifi` package.

### SSL certificate configuration

Posting can load custom CA bundles from a `.pem` file.

The easiest way to do this is in your `config.yaml` file:

```yaml
ssl:
  ca_bundle: 'absolute/path/to/certificate.pem'
```

### Environment-specific certificates

If the required CA bundle differs per environment, you can again use the principle that all configuration can be set as environment variables which can optionally be set and loaded using `--env` and `.env` files:

```bash
# dev.env
POSTING_SSL__CA_BUNDLE='/path/to/certificate.pem'
```

Now load the `dev.env` file when working in the `dev` environment to ensure the dev environment CA bundle is used:

```bash
posting --env dev.env
```

### Disabling SSL verification

SSL verification can be disabled on a per-request basis in the "Options" tab.

### Client-side certificates

You can specify local certificates to use as a client-side certificate:

```yaml
ssl:
  certificate_path: /path/to/certificate.pem
  key_file: /path/to/key.key  # optional
  password: '***********'  # optional password for key_file
```

## Full configuration reference

The table below lists all available configuration options and their environment variable equivalents, their default values, and descriptions.

| Config Key (Env Var) | Values (Default) | Description |
|----------------------|------------------|-------------|
| `theme` (`POSTING_THEME`) | See the list of themes in the command palette (Default: `"galaxy"`) | Sets the theme of the application. |
| `load_user_themes` (`POSTING_LOAD_USER_THEMES`) | `true`, `false` (Default: `true`) | If enabled, load user themes from the theme directory, allowing them to be specified in config and selected via the command palette. |
| `load_builtin_themes` (`POSTING_LOAD_BUILTIN_THEMES`) | `true`, `false` (Default: `true`) | If enabled, load builtin themes, allowing them to be specified in config and selected via the command palette. |
| `theme_directory` (`POSTING_THEME_DIRECTORY`) | (Default: `${XDG_DATA_HOME}/posting/themes`) | The directory containing user themes. |
| `layout` (`POSTING_LAYOUT`) | `"vertical"`, `"horizontal"` (Default: `"horizontal"`) | Sets the layout of the application. |
| `use_host_environment` (`POSTING_USE_HOST_ENVIRONMENT`) | `true`, `false` (Default: `false`) | Allow/deny using environment variables from the host machine in requests via `$env:` syntax. When disabled, only variables defined explicitly in `.env` files will be available for use. |
| `watch_env_files` (`POSTING_WATCH_ENV_FILES`) | `true`, `false` (Default: `true`) | If enabled, automatically reload environment files when they change. |
| `watch_themes` (`POSTING_WATCH_THEMES`) | `true`, `false` (Default: `true`) | If enabled, automatically reload themes in the theme directory when they change on disk. |
| `watch_collection_files` (`POSTING_WATCH_COLLECTION_FILES`) | `true`, `false` (Default: `true`) | If enabled, automatically reload collection files when they change on disk. Right now, this is limited to reloading Python scripts in the collection. |
| `animation` (`POSTING_ANIMATION`) | `"none"`, `"basic"`, `"full"` (Default: `"none"`) | Controls the animation level. |
| `response.prettify_json` (`POSTING_RESPONSE__PRETTIFY_JSON`) | `true`, `false` (Default: `true`) | If enabled, JSON responses will be pretty-formatted. |
| `response.show_size_and_time` (`POSTING_RESPONSE__SHOW_SIZE_AND_TIME`) | `true`, `false` (Default: `true`) | If enabled, the size and time taken for the response will be displayed in the response area border subtitle. |
| `heading.visible` (`POSTING_HEADING__VISIBLE`) | `true`, `false` (Default: `true`) | Show/hide the app header. |
| `heading.show_host` (`POSTING_HEADING__SHOW_HOST`) | `true`, `false` (Default: `true`) | Show/hide the hostname in the app header. |
| `heading.show_version` (`POSTING_HEADING__SHOW_VERSION`) | `true`, `false` (Default: `true`) | Show/hide the version in the app header. |
| `heading.hostname` (`POSTING_HEADING__HOSTNAME`) | (Default: `unset`) | The hostname to display in the app header. You may use Rich markup here. If unset, the hostname provided via `socket.gethostname()` will be used. |
| `url_bar.show_value_preview` (`POSTING_URL_BAR__SHOW_VALUE_PREVIEW`) | `true`, `false` (Default: `true`) | Show/hide the variable value preview below the URL bar. |
| `collection_browser.position` (`POSTING_COLLECTION_BROWSER__POSITION`) | `"left"`, `"right"` (Default: `"left"`) | The position of the collection browser on screen. |
| `collection_browser.show_on_startup` (`POSTING_COLLECTION_BROWSER__SHOW_ON_STARTUP`) | `true`, `false` (Default: `true`) | Show/hide the collection browser on startup. Can always be toggled using the command palette. |
| `pager` (`POSTING_PAGER`) | (Default: `$PAGER`) | Command to use for paging text. |
| `pager_json` (`POSTING_PAGER_JSON`) | (Default: `$PAGER`) | Command to use for paging JSON. |
| `editor` (`POSTING_EDITOR`) | (Default: `$EDITOR`) | Command to use for opening files in an external editor. |
| `ssl.ca_bundle` (`POSTING_SSL__CA_BUNDLE`) | Absolute path (Default: `unset`) | Absolute path to a CA bundle file/dir. If not set, the [Certifi](https://pypi.org/project/certifi/) CA bundle will be used. |
| `ssl.certificate_path` (`POSTING_SSL__CERTIFICATE_PATH`) | Absolute path (Default: `unset`) | Absolute path to a client SSL certificate file or directory. |
| `ssl.key_file` (`POSTING_SSL__KEY_FILE`) | Absolute path (Default: `unset`) | Absolute path to a client SSL key file. |
| `ssl.password` (`POSTING_SSL__PASSWORD`) | Password for the key file. (Default: `unset`) | Password to decrypt the key file if it's encrypted. |
| `focus.on_startup` (`POSTING_FOCUS__ON_STARTUP`) | `"url"`, `"method", "collection"` (Default: `"url"`) | Automatically focus the URL bar, method, or collection browser when the app starts. |
| `focus.on_response` (`POSTING_FOCUS__ON_RESPONSE`) | `"body"`, `"tabs"` (Default: `unset`)| Automatically focus the response tabs or response body text area when a response is received. |
| `focus.on_request_open` (`POSTING_FOCUS__ON_REQUEST_OPEN`) | `"headers"`, `"body"`, `"query"`, `"info"`, `"url"`, `"method"` (Default: `unset`) | Automatically focus the specified target when a request is opened from the collection browser. |
| `text_input.blinking_cursor` (`POSTING_TEXT_INPUT__BLINKING_CURSOR`) | `true`, `false` (Default: `true`) | If enabled, the cursor will blink in input widgets and text area widgets. |
| `command_palette.theme_preview` (`POSTING_COMMAND_PALETTE__THEME_PREVIEW`) | `true`, `false` (Default: `false`) | If enabled, the command palette will display a preview of the selected theme when the cursor is over it. This will slow down cursor movement and so is disabled by default. |
| `use_xresources` (`POSTING_USE_XRESOURCES`) | `true`, `false` (Default: `false`) | Try to create themes called `xresources-dark` and `xresources-light` (see the section below) |
| `curl_export_extra_args` (`POSTING_CURL_EXPORT_EXTRA_ARGS`) | (Default: `""`) | Extra arguments to pass to curl when exporting a request as a curl command. This string will be inserted directly into the command that gets copied to your clipboard, immediately after `curl `. |