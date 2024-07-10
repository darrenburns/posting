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

### Keyboard shortcuts

Important keyboard shortcuts are displayed at the bottom of the screen.
Some additional keyboard shortcuts are shown in the table below.

<details>
  <summary>Click here to see keybinds</summary>
  
| Action | Shortcut | Context |
|--------|----------|---------|
| Focus the URL bar | <kbd>ctrl</kbd>+<kbd>l</kbd> | Global |
| Toggle collection browser sidebar | <kbd>ctrl</kbd>+<kbd>h</kbd> | Global |
| Expand request section | <kbd>ctrl</kbd>+<kbd>m</kbd> | When request section is focused |
| Expand response section | <kbd>ctrl</kbd>+<kbd>m</kbd> | When response section is focused |
| Next directory | <kbd>shift</kbd>+<kbd>j</kbd> | When collection browser is focused |
| Previous directory | <kbd>shift</kbd>+<kbd>k</kbd> | When collection browser is focused |
| Undo in request body | <kbd>ctrl</kbd>+<kbd>z</kbd> | When request body text area is focused |
| Redo in request body | <kbd>ctrl</kbd>+<kbd>y</kbd> | When request body text area is focused |
| Copy selection to clipboard | <kbd>y</kbd> or <kbd>c</kbd> | When response body text area is focused |
| Open in pager | <kbd>f3</kbd> | When a text area is focused |
| Open in external editor | <kbd>f4</kbd> | When a text area is focused |

</details>

> [!TIP]
> Many parts of the UI support Vim keys for navigation.

### Exiting Posting TUI

Press `Ctrl`+`c` to quit the TUI.

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

Posting also supports `.env` (dotenv) files, which are useful if you want to keep your configuration in a file rather than in your shell's environment variables.

You can tell Posting to use a `.env` file using the `--env-file` option.

Here's an example `.env` file:

```bash
POSTING_THEME="cobalt"
POSTING_LAYOUT="vertical"
POSTING_HEADING__VISIBLE="false"
```

### Available configuration options

| Config Key (Env Var) | Values (Default) | Description |
|----------------------|------------------|-------------|
| `theme` (`POSTING_THEME`) | `"posting"`, `"galaxy"`, `"monokai"`, `"solarized-light"`, `"nautilus"`, `"nebula"`, `"alpine"`, `"cobalt"`, `"twilight"`, `"hacker"` (Default: `"posting"`) | Sets the theme of the application. |
| `layout` (`POSTING_LAYOUT`) | `"vertical"`, `"horizontal"` (Default: `"horizontal"`) | Sets the layout of the application. |
| `use_host_environment` (`POSTING_USE_HOST_ENVIRONMENT`) | `true`, `false` (Default: `false`) | Allow/deny using environment variables from the host machine in requests via `$env:` syntax. When disabled, only variables defined explicitly in `.env` files will be available for use. |
| `animation` (`POSTING_ANIMATION`) | `"none"`, `"basic"`, `"full"` (Default: `"none"`) | Controls the animation level. |
| `response.prettify_json` (`POSTING_RESPONSE__PRETTIFY_JSON`) | `true`, `false` (Default: `true`) | If enabled, JSON responses will be pretty-formatted. |
| `heading.visible` (`POSTING_HEADING__VISIBLE`) | `true`, `false` (Default: `true`) | Show/hide the app header. |
| `heading.show_host` (`POSTING_HEADING__SHOW_HOST`) | `true`, `false` (Default: `true`) | Show/hide the hostname in the app header. |
| `url_bar.show_value_preview` (`POSTING_URL_BAR__SHOW_VALUE_PREVIEW`) | `true`, `false` (Default: `true`) | Show/hide the variable value preview below the URL bar. |


## Importing OpenAPI Specifications

Note: this feature is highly experimental.

Posting can convert OpenAPI 3.x specs into collections.

To import an OpenAPI Specification, use the `posting import path/to/openapi.yaml` command.

You can optionally supply an output directory.

If no output directory is supplied, the default collection directory will be used.

Posting will attempt to build a file structure in the collection that aligns with the URL structure of the imported API.
