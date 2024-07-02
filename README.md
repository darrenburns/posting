# Posting.

Posting is an powerful HTTP client which brings Postman-like functionality to your terminal.

It's designed for those who prefer working in a terminal environment and enjoy fast, keyboard-centric user interfaces.

<img width="1138" alt="screenshot-01jul24-2" src="https://github.com/darrenburns/posting/assets/5740731/d0e9f640-a0ba-41af-b2ae-7bda67e18026">

## Important note

Posting is still under development and not yet feature complete.

Some notable missing features include variables/environments, pre/post-request scripts, and the ability to send form data and files.

## Installation

Posting can be installed via [`pipx`](https://pipx.pypa.io/stable/):

```bash
pipx install posting
```

Python 3.11 or later is required.

More installation methods (`brew`, etc) will be added soon.

## Collections

Requests can be stored inside "collections" on your file system.
A collection is simply a directory containing one or more requests.

Each request is stored as a simple YAML file, suffixed with `.posting.yaml` - easy to read, understand, and version control!

Here's a quick example of a `*.posting.yaml` file.

```yaml
name: Create user
description: Adds a new user to the system.
method: POST
url: https://jsonplaceholder.typicode.com/users
body: |-
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

To open a collection, simply pass the path to the `--collection` option when launching Posting:

```bash
posting --collection path/to/collection
```

The supplied directory will be recursively searched for files matching `**/*.posting.yaml`, and they'll appear in the sidebar.

If you don't supply a directory, Posting will use the default collection directory.
You can check where this is by running `posting locate collection`.

To save the currently open request, press <kbd>ctrl</kbd>+<kbd>s</kbd>.

## Navigation

Posting can be navigated using either mouse or keyboard.

### Jump mode

Jump mode is the fastest way to get around in Posting.

Press <kbd>ctrl</kbd>+<kbd>o</kbd> to enter jump mode.

A key overlay will appear on the screen, allowing you to jump to any widget by pressing the corresponding key.

https://github.com/darrenburns/posting/assets/5740731/5e7cdf57-90b2-4dba-b468-0057c6ef1806

### Tab navigation

<kbd>tab</kbd> and <kbd>shift+tab</kbd> will move focus between widgets,
and <kbd>j</kbd>/<kbd>k</kbd> will move around within a widget.

### Keyboard shortcuts

Important keyboard shortcuts are displayed at the bottom of the screen.

However, there are many other shortcuts available - these will be documented soon.

<!-- TODO - document other shortcuts. -->

### Expanding/hiding the request/response sections

Press <kbd>ctrl</kbd>+<kbd>m</kbd> to expand the section which currently has focus (the request or response section).
Press it again to reset the UI.

## Command palette

Some functionality in Posting doesn't warrant a dedicated keyboard shortcut (for example, switching to a specific theme), and can instead be accessed via the _command palette_.

To open the command palette, press <kbd>ctrl</kbd>+<kbd>p</kbd>.

https://github.com/darrenburns/posting/assets/5740731/a199e5f2-5621-42e6-b239-a796d1dc144a


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
```

### Environment variables

All configuration values can also be set as environment variables.

Simply prefix the name of the config with `POSTING_` and set it as an environment variable.

For example, to set the theme to `galaxy`, you can set the environment variable `POSTING_THEME=galaxy`.

### dotenv (`.env`) files

Posting also supports `.env` (dotenv) files, which are useful if you want to keep your configuration in a file rather than in your shell's environment variables.

You can tell Posting to use a `.env` file using the `--env-file` option.

Here's an example `.env` file:

```bash
POSTING_THEME="cobalt"
POSTING_LAYOUT="vertical"
```

### Available configuration options


| Config Key | Environment Variable | Possible Values       | Default | Description                                      |
|------------|----------------------|-----------------------|---------|--------------------------------------------------|
| `theme`    | `POSTING_THEME`      | `"posting"`, `"galaxy"`, `"monokai"`, `"solarized-light"`, `"nautilus"`, `"nebula"`, `"alpine"`, `"cobalt"`, `"twilight"`, `"hacker"` | `"posting"` | Sets the theme of the application.               |
| `layout`   | `POSTING_LAYOUT`     | `"vertical"`, `"horizontal"` | `"horizontal"` | Sets the layout of the application.              |

## Animation

You can turn off animations by setting the environment variable `TEXTUAL_ANIMATIONS=none`.

## Planned Features

- **Keyboard Friendly**: Navigate and iterate on your APIs using simple keyboard shortcuts.
- **File System Storage**: Your collections are saved as files, meaning you can easily sync them using version control or your favorite cloud provider.
- **Multiplatform**: Run on MacOS, Linux and Windows.
- **Template Variables**: Define variables and substitute them into your requests.
- **Powerful Text and JSON Editor**: Offers tree-sitter powered syntax highlighting, undo/redo, copy/paste, and more.
- **Runs Over SSH**: Send requests from a remote host via SSH.
- **Your Idea Here**: Please let me know if you have opinions on the features above, or any other ideas!
