# Posting.

**A powerful HTTP client that lives in your terminal.**

Posting is an HTTP client, not unlike Postman and Insomnia. As a TUI application, it can be used over SSH and enables efficient keyboard-centric workflows. Your requests are stored locally in simple YAML files, meaning they're easy to read and version control.

<img width="1337" alt="image" src="https://github.com/darrenburns/posting/assets/5740731/77f50aa0-bc93-4e42-b06b-c209ec233fe8">

Some notable features include: "jump mode" navigation, environments/variables system with autocompletion, syntax highlighting powered by tree-sitter, Vim keys, various builtin themes, a configuration system, "open in $EDITOR", and a command palette for quickly accessing functionality.

Posting was built with [Textual](https://github.com/textualize/textual).

## Installation

Posting can be installed via [`pipx`](https://pipx.pypa.io/stable/):

```bash
pipx install posting
```

Python 3.11 or later is required.

More installation methods (`brew`, etc) will be added soon.

## Collections

Requests are stored directly on your file system as simple YAML files, suffixed with `.posting.yaml` - easy to read, understand, and version control!

Here's what they look like:

```yaml
name: Create user
description: Adds a new user to the system.
method: POST
url: https://jsonplaceholder.typicode.com/users
body: 
  content: |-
    {
      "firstName": "John",
      "email": "john.doe@example.com"
    }
headers:
- name: Content-Type
  value: application/json
params:
- name: sendWelcomeEmail
  value: 'true'
```

To open a collection (a directory containing requests), use the `--collection` option:

```bash
posting --collection path/to/collection
```

This will recursively find and display requests in the sidebar.
If you don't supply a directory, Posting will use the default collection directory.
You can check where this is by running `posting locate collection`.

## Navigation

Posting can be navigated using either mouse or keyboard.

### Jump mode

Jump mode is the fastest way to get around.

Press <kbd>ctrl</kbd>+<kbd>o</kbd> to enter jump mode.

A key overlay will appear on the screen, allowing you to jump to any widget by pressing the corresponding key.

<p align="center">
  <img src="https://github.com/darrenburns/posting/assets/5740731/c4f09ca8-a228-449c-97b1-573f86d4ae6a" alt="url-bar-environments-short">
</p>

### Tab navigation

<kbd>tab</kbd> and <kbd>shift+tab</kbd> will move focus between widgets,
and <kbd>j</kbd>/<kbd>k</kbd>/<kbd>up</kbd>/<kbd>down</kbd> will move around within a widget.

Where it makes sense, <kbd>up</kbd> and <kbd>down</kbd> will move between widgets.

### Contextual help

Many widgets have additional bindings beyond those displayed in the footer. You can view the full list of keybindings for the currently
focused widget, as well as additional usage information and tips, by pressing <kbd>f1</kbd> or <kbd>ctrl</kbd>+<kbd>?</kbd> (or <kbd>ctrl</kbd>+<kbd>shift</kbd>+<kbd>/</kbd>).

<img width="1229" alt="image" src="https://github.com/user-attachments/assets/707be55f-6dfc-4faf-b9f3-fe7bc5422008">

### Exiting

Press <kbd>ctrl</kbd>+<kbd>c</kbd> to quit Posting.

## Environments

You can use variables in the UI using the `${VARIABLE_NAME}` or `$VARIABLE_NAME` syntax.
These variables will be substituted into outgoing requests.

<p align="center">
  <img src="https://github.com/darrenburns/posting/assets/5740731/24b64f58-747b-409e-9672-e354eb8994d8" alt="url-bar-environments-short">
</p>

`.env` files can be loaded using the `--env` option.
Variables from these files can then be used in the UI.

### Example

Imagine you're testing an API which exists in both `dev` and `prod` environments.

The `dev` and `prod` environments share some common variables, but differ in many ways too.
We can model this by having a single `shared.env` file which contains variables which are shared between environments, and then a `dev.env` and `prod.env` file which contain environment specific variables.

```bash
# file: shared.env
API_PATH="/api/v1"
ENV_NAME="shared"

# file: dev.env
API_KEY="dev-api-key"
ENV_NAME="dev"
BASE_URL="https://${ENV_NAME}.example.com"

# file: prod.env
API_KEY="prod-api-key"
ENV_NAME="prod"
BASE_URL="https://${ENV_NAME}.example.com"
```

When working in the `dev` environment, you can then load all of the shared variables and all of the development environment specific variables using the `--env` option:

```bash
posting --env shared.env --env dev.env
```

This will load all of the shared variables from `shared.env`, and then load the variables from `dev.env`. Since `ENV_NAME` appears in both files, the value from the `dev.env` file will be used since that was the last one specified.

Note that you do *not* need to restart to load changes made to these files,
so you can open and edit your env files in an editor of your choice alongside Posting.
However, autocompletion and variable highlighting will not update until Posting is restarted.

If you want to permit using environment variables that exist on the host machine (i.e. those which are not defined in any `.env` files), you must set the `use_host_environment` config option to `true` (or set the environment variable `POSTING_USE_HOST_ENVIRONMENT=true`).

#### Environment specific config

Since all Posting configuration options can also be specified as environment variables, we can also put environment specific config inside `.env` files. There's a dedicated "Configuration" section in this document which covers this in more detail.

For example, if you wanted to use a light theme in the prod environment (as a subtle reminder that you're in production!), you could set the environment variable `POSTING_THEME=solarized-light` inside the `prod.env` file.

Note that configuration files take precedence over environment variables, so if you set a value in both a `.env` file and a `config.yaml`, the value from the `config.yaml` file will be used.

## Command palette

Some functionality in Posting doesn't warrant a dedicated keyboard shortcut (for example, switching to a specific theme), and can instead be accessed via the _command palette_.

To open the command palette, press <kbd>ctrl</kbd>+<kbd>p</kbd>.

<p align="center">
  <img src="https://github.com/darrenburns/posting/assets/5740731/945b585c-dcb8-48cd-8458-24ceed0f5efa" alt="command-palette-demo">
</p>

## Configuration

Posting can be configured using a configuration file, environment variables, and/or `.env` files.

Configuration values are loaded in the following order of precedence (highest to lowest):

1. Configuration file
2. Environment variables
3. `.env` files

The range of available configuration will be greatly expanded in the future.

### Configuration file

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

### Environment variables

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

### Available configuration options

| Config Key (Env Var) | Values (Default) | Description |
|----------------------|------------------|-------------|
| `theme` (`POSTING_THEME`) | `"posting"`, `"galaxy"`, `"monokai"`, `"solarized-light"`, `"nautilus"`, `"nebula"`, `"alpine"`, `"cobalt"`, `"twilight"`, `"hacker"` (Default: `"posting"`) | Sets the theme of the application. |
| `load_user_themes` (`POSTING_LOAD_USER_THEMES`) | `true`, `false` (Default: `true`) | If enabled, load user themes from the theme directory, allowing them to be specified in config and selected via the command palette. |
| `load_builtin_themes` (`POSTING_LOAD_BUILTIN_THEMES`) | `true`, `false` (Default: `true`) | If enabled, load builtin themes, allowing them to be specified in config and selected via the command palette. |
| `theme_directory` (`POSTING_THEME_DIRECTORY`) | (Default: `${XDG_DATA_HOME}/posting/themes`) | The directory containing user themes. |
| `layout` (`POSTING_LAYOUT`) | `"vertical"`, `"horizontal"` (Default: `"horizontal"`) | Sets the layout of the application. |
| `use_host_environment` (`POSTING_USE_HOST_ENVIRONMENT`) | `true`, `false` (Default: `false`) | Allow/deny using environment variables from the host machine in requests via `$env:` syntax. When disabled, only variables defined explicitly in `.env` files will be available for use. |
| `animation` (`POSTING_ANIMATION`) | `"none"`, `"basic"`, `"full"` (Default: `"none"`) | Controls the animation level. |
| `response.prettify_json` (`POSTING_RESPONSE__PRETTIFY_JSON`) | `true`, `false` (Default: `true`) | If enabled, JSON responses will be pretty-formatted. |
| `response.show_size_and_time` (`POSTING_RESPONSE__SHOW_SIZE_AND_TIME`) | `true`, `false` (Default: `true`) | If enabled, the size and time taken for the response will be displayed in the response area border subtitle. |
| `heading.visible` (`POSTING_HEADING__VISIBLE`) | `true`, `false` (Default: `true`) | Show/hide the app header. |
| `heading.show_host` (`POSTING_HEADING__SHOW_HOST`) | `true`, `false` (Default: `true`) | Show/hide the hostname in the app header. |
| `heading.show_version` (`POSTING_HEADING__SHOW_VERSION`) | `true`, `false` (Default: `true`) | Show/hide the version in the app header. |
| `url_bar.show_value_preview` (`POSTING_URL_BAR__SHOW_VALUE_PREVIEW`) | `true`, `false` (Default: `true`) | Show/hide the variable value preview below the URL bar. |
| `pager` (`POSTING_PAGER`) | (Default: `$PAGER`) | Command to use for paging text. |
| `pager_json` (`POSTING_PAGER_JSON`) | (Default: `$PAGER`) | Command to use for paging JSON. |
| `editor` (`POSTING_EDITOR`) | (Default: `$EDITOR`) | Command to use for opening files in an external editor. |
| `ssl.ca_bundle` (`POSTING_SSL__CA_BUNDLE`) | Absolute path (Default: `unset`) | Absolute path to a CA bundle file/dir. If not set, the [Certifi](https://pypi.org/project/certifi/) CA bundle will be used. |
| `ssl.verify` (`POSTING_SSL__VERIFY`) | `true`, `false` (Default: `true`) | Verify server identity. |
| `ssl.certificate_path` (`POSTING_SSL__CERTIFICATE_PATH`) | Absolute path (Default: `unset`) | Absolute path to a client SSL certificate file or directory. |
| `ssl.key_file` (`POSTING_SSL__KEY_FILE`) | Absolute path (Default: `unset`) | Absolute path to a client SSL key file. |
| `ssl.password` (`POSTING_SSL__PASSWORD`) | Password for the key file. (Default: `unset`) | Password to decrypt the key file if it's encrypted. |
| `focus.on_startup` (`POSTING_FOCUS__ON_STARTUP`) | `"url"`, `"method", "collection"` (Default: `"url"`) | Automatically focus the URL bar, method, or collection browser when the app starts. |
| `focus.on_response` (`POSTING_FOCUS__ON_RESPONSE`) | `"body"`, `"tabs"` (Default: `unset`)| Automatically focus the response tabs or response body text area when a response is received. |
| `text_input.blinking_cursor` (`POSTING_TEXT_INPUT__BLINKING_CURSOR`) | `true`, `false` (Default: `true`) | If enabled, the cursor will blink in input widgets and text area widgets. |
| `command_palette.theme_preview` (`POSTING_COMMAND_PALETTE__THEME_PREVIEW`) | `true`, `false` (Default: `false`) | If enabled, the command palette will display a preview of the selected theme when the cursor is over it. This will slow down cursor movement and so is disabled by default. |
| `use_xresources` (`POSTING_USE_XRESOURCES`) | `true`, `false` (Default: `false`) | Try to create themes called `xresources-dark` and `xresources-light` (see the section below) |

## SSL certificate configuration

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

### Client-side certificates

You can specify local certificates to use as a client-side certificate:

```yaml
ssl:
  certificate_path: /path/to/certificate.pem
  key_file: /path/to/key.key  # optional
  password: '***********'  # optional password for key_file
```

## Theming

Place custom themes in the themes directory and Posting will load them on startup. Theme files must be suffixed with `.yaml`, but the rest of the filename is unused by Posting.

You can check where Posting will look for themes by running `posting locate themes` in your terminal.

Here's an example theme file:

```yaml
name: example  # use this name in your config file
primary: '#4e78c4'  # buttons, fixed table columns
secondary: '#f39c12'  # method selector, some minor labels
accent: '#e74c3c'  # header text, scrollbars, cursors, focus highlights
background: '#0e1726' # background colors
surface: '#17202a'  # panels, etc
error: '#e74c3c'  # error messages
success: '#2ecc71'  # success messages
warning: '#f1c40f'  # warning messages
syntax: 'dracula'  # auto-switch syntax highlighting theme

# Optional metadata
author: Darren Burns
description: A dark theme with a blue primary color.
homepage: https://github.com/darrenburns/posting
```

### X resources themes

Posting supports using X resources for theming. To use this, enable the `use_xresources` option (see above).

It requires the `xrdb` executable on your `PATH` and `xrdb -query` must return the following variables:

| Xresources  | Description |
|-------------|-----------|
| *color0     | primary color: used for button backgrounds and fixed table columns |
| *color8     | secondary color: used in method selector and some minor labels |
| *color1     | error color: used for error messages |
| *color2     | success color: used for success messages |
| *color3     | warning color: used for warning messages |
| *color4     | accent color: used for header text, scrollbars, cursors, focus highlights |
| *background | background color |
| *color7     | surface/panel color |

If these conditions are met, themes called `xresources-dark` and `xresources-light` will be available for use.

## Importing OpenAPI Specifications

Note: this feature is highly experimental.

Posting can convert OpenAPI 3.x specs into collections.

To import an OpenAPI Specification, use the `posting import path/to/openapi.yaml` command.

You can optionally supply an output directory.

If no output directory is supplied, the default collection directory will be used.

Posting will attempt to build a file structure in the collection that aligns with the URL structure of the imported API.
