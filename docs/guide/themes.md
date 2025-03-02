## Overview

Posting ships with several built-in themes, and also supports custom, user-made themes.

When editing a theme on disk, Posting can show a live preview of the theme in effect, making it easy to design and test themes.

### Creating a theme

You can check where Posting will look for user-defined themes by running `posting locate themes` in your terminal.
Place custom themes in this directory and Posting will load them on startup.
Theme files must be suffixed with `.yaml`, but the rest of the filename is unused by Posting.
Built-in themes are *not* in this directory, but are part of the Posting code itself.

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

# Optional metadata
author: Darren Burns
description: A dark theme with a blue primary color.
homepage: https://github.com/darrenburns/posting
```

After adding a theme, you'll need to restart Posting for it to take effect.

To use the theme, you can specify it in your `config.yaml` file:

```yaml
theme: example
```

Note that the theme name is *not* defined by the filename, but by the `name` field in the theme file.

!!! tip

    If you edit a theme on disk while Posting is using it, the UI will automatically
    refresh to reflect the changes you've made. This is enabled by default, but if you'd
    like to disable it, you can set `watch_themes` to `false` in your `config.yaml`.

#### Syntax highlighting

Syntax highlighted elements such as the URL bar, text areas, and fields which contain variables will be colored based on the semantic colors defined in the theme (`primary`, `secondary`, etc) by default.

If you'd like more control over the syntax highlighting, you can specify a custom syntax highlighting colors inside the theme file.

The example below illustrates some of the options available when it comes to customizing syntax highlighting.

```yaml
text_area:
  cursor: 'reverse'  # style the block cursor
  cursor_line: 'underline'  # style the line the cursor is on
  selection: 'reverse'  # style the selected text
  gutter: 'bold #50e3c2'  # style the gutter
  matched_bracket: 'black on green'  # style the matched bracket
url:
  base: 'italic #50e3c2'  # style the 'base' of the url
  protocol: 'bold #b8e986'  # style the protocol
syntax:
  json_key: 'italic #4a90e2'  # style json keys
  json_number: '#50e3c2'  # style json numbers
  json_string: '#b8e986'  # style json strings
  json_boolean: '#b8e986'  # style json booleans
  json_null: 'underline #b8e986'  # style json null values
```

#### Method styles

You can also specify custom styles for methods in the collection tree.

Here's an example:

```yaml
method:
  get: 'underline #50e3c2'
  post: 'italic #b8e986'
  put: 'bold #b8e986'
  delete: 'strikethrough #b8e986'
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
