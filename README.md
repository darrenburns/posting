# Posting.

Posting is a powerful HTTP client which brings Postman-like functionality to your terminal. It works over SSH, stores collections in a Git-friendly format, and can be operated efficiently using both keyboard and mouse.

<img width="1138" alt="screenshot-01jul24-2" src="https://github.com/darrenburns/posting/assets/5740731/d0e9f640-a0ba-41af-b2ae-7bda67e18026">

## Important note

Posting is still under development and not yet feature complete.

The UI, config, and request format is subject to change while the version specifier contains a `b` suffix.

Some notable missing features include pre/post-request scripts, and the ability to send files. Also, some of the existing features are still being fleshed out.

## Installation

Posting can be installed via [`pipx`](https://pipx.pypa.io/stable/):

```bash
pipx install posting
```

Python 3.11 or later is required.

More installation methods (`brew`, etc) will be added soon.

## Collections

Requests can be stored directly on your file system.

Each request is stored as a simple YAML file, suffixed with `.posting.yaml` - easy to read, understand, and version control!

Here's a quick example of a `*.posting.yaml` file.

```yaml
name: Create user
description: Adds a new user to the system.
method: POST
url: https://jsonplaceholder.typicode.com/users
body: 
  content: |-
    {
      "firstName": "John",
      "lastName": "Doe",
      "email": "john.doe@example.com"
    }
headers:
- name: Content-Type
  value: application/json
- name: Some-Header
  value: Some value
  enabled: false
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

### Keybindings

- To create a new request, press <kbd>ctrl</kbd>+<kbd>n</kbd>.
- To save changes to the currently open request, press <kbd>ctrl</kbd>+<kbd>s</kbd>.

## Navigation

Posting can be navigated using either mouse or keyboard.

### Jump mode

Jump mode is the fastest way to get around in Posting.

Press <kbd>ctrl</kbd>+<kbd>o</kbd> to enter jump mode.

A key overlay will appear on the screen, allowing you to jump to any widget by pressing the corresponding key.

<img width="1327" alt="image" src="https://github.com/darrenburns/posting/assets/5740731/0911e3da-7500-456f-a53f-ea5038498e13">

### Tab navigation

<kbd>tab</kbd> and <kbd>shift+tab</kbd> will move focus between widgets,
and <kbd>j</kbd>/<kbd>k</kbd>/<kbd>up</kbd>/<kbd>down</kbd> will move around within a widget.

Where it makes sense, <kbd>up</kbd> and <kbd>down</kbd> will move between widgets.

### Keyboard shortcuts

Important keyboard shortcuts are displayed at the bottom of the screen.

However, there are many other shortcuts available - these will be documented soon.

<!-- TODO - document other shortcuts. -->

### Expanding/hiding the request/response sections

Press <kbd>ctrl</kbd>+<kbd>m</kbd> to expand the section which currently has focus (the request or response section).

Press it again to reset the UI.

This can also be done via the command palette options `view: expand request` and `view: expand response`.

## Environments

You can refer to variables in the UI using the `${env:VARIABLE_NAME}` or `$env:VARIABLE_NAME` syntax.

These variables will be substituted into outgoing requests.

Used in conjunction with dotenv (`.env`) files, this lets you define environment-specific variables.

You can load `.env` files into Posting from the command line using the `--env` option, and then refer to variables in these files within the UI.

### Example

Imagine you're testing an API which exists in both `dev` and `prod` environments.

In the `dev` and `prod` environments, some variables are shared, but others are different. We can model this by having a single `shared.env` file which contains variables which are shared between environments, and then a `dev.env` and `prod.env` file which contain environment specific variables.

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

This will load all of the shared variables, and then load the `dev.env` file. Since `ENV_NAME` appears in both files, the value from the `dev.env` file will be used since that was the last one specified.

Note that you do *not* need to restart to load changes made to these files,
so you can open and edit  your env files in an editor of your choice alongside Posting.

If you want to permit using environment variables that exist on the host machine (i.e. those which are not defined in any `.env` files), you must set the `use_host_environment` config option to `true` (or set the environmnet variable `POSTING_USE_HOST_ENVIRONMENT=true`).

#### Environment specific config

Since all Posting configuration options can also be specified as environment variables, we can put environment specific config inside `.env` files. There's a dedicated "Configuration" section in this document which covers this in more detail.

For example, if you wanted to use a light theme in the prod environment (as a subtle reminder that you're in production!), you could set the environment variable `POSTING_THEME=solarized-light` inside the `prod.env` file.

Note that configuration files take precedence over environment variables, so if you set a value in both a `.env` file and a `config.yaml`, the value from the `config.yaml` file will be used.

## Command palette

Some functionality in Posting doesn't warrant a dedicated keyboard shortcut (for example, switching to a specific theme), and can instead be accessed via the _command palette_.

To open the command palette, press <kbd>ctrl</kbd>+<kbd>p</kbd>.

<img width="1327" alt="image" src="https://github.com/darrenburns/posting/assets/5740731/2e9a28cb-af6e-454c-b368-6f7943a367a4">

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
