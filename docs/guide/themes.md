## Overview

Posting 3 comes with 37 built-in themes, and can load themes you make yourself, including the
themes you made for Posting 2.

## Choosing a theme

Press ++ctrl+p++ and choose **Theme…** to see every theme. As you move through the list, Posting
previews each one, so you can see how it looks before you choose it. Press ++enter++ to use the
highlighted theme, or ++escape++ to go back to the theme you had.

<figure class="screen">
--8<-- "themes.html"
<figcaption>The theme picker. Change the theme of these docs with the picker at the top of the page to see a few of them.</figcaption>
</figure>

A theme chosen in the palette lasts until you quit. To keep it, set it in your
[configuration file](./configuration.md):

```yaml
theme: lantern
```

or with the `POSTING_THEME` environment variable.

If moving through the list feels slow in your terminal, turn off the live preview with
`command_palette.theme_preview: false`.

## Built-in themes

| Dark | | | |
|------|-|-|-|
| `galaxy` (default) | `abyss` | `amber` | `amethyst` |
| `aurora` | `bonsai` | `catppuccin` | `cyberdeck` |
| `dracula` | `dwarven` | `garnet` | `gruvbox` |
| `hearthstone` | `kanagawa` | `kintsugi` | `lantern` |
| `midnight-ember` | `monokai` | `moonstone` | `neon-reef` |
| `nord` | `obsidian-tide` | `phosphor` | `rose-pine` |
| `solarized` | `tokyo-night` | `understory` | `velvet` |

| Light | | | |
|-------|-|-|-|
| `catppuccin-latte` | `dracula-light` | `gruvbox-light` | `kanagawa-lotus` |
| `monokai-light` | `nord-light` | `rose-pine-dawn` | `solarized-light` |
| `tokyo-night-day` | | | |

### Posting 2 theme names

Some of Posting 2's theme names are mapped to their closest Posting 3 theme, so an old config
keeps working:

| Posting 2 | Posting 3 |
|-----------|-----------|
| `posting`, `textual-dark` | `galaxy` |
| `catppuccin-mocha`, `catppuccin-macchiato`, `catppuccin-frappe` | `catppuccin` |
| `textual-light` | `catppuccin-latte` |

Posting 2 themes that Posting 3 doesn't have, such as `nebula`, `cobalt` or `synthwave`, fall back
to `galaxy`, and Posting lets you know when it starts.

## Creating a theme

Run `posting locate themes` to find where Posting looks for your themes:

```bash
$ posting locate themes
Themes directory:
/home/you/.local/share/posting/themes
```

Each theme is a YAML file in that directory, ending in `.yaml` or `.yml`. Here's an example:

```yaml
name: harbour           # the name you use in your config and see in the picker
dark: true              # false for a light theme
primary: '#4e78c4'      # the main colour: GET labels, JSON keys, dialog borders
secondary: '#f39c12'    # a second colour, used for smaller highlights
accent: '#e74c3c'       # a third colour, used to show what has focus
background: '#0e1726'   # the screen's background
surface: '#17202a'      # inputs, text areas and tables
panel: '#1f2b38'        # raised surfaces, a step above `surface`
text: '#e6edf3'         # the main text colour
success: '#2ecc71'      # successful responses, variables that have a value
warning: '#f1c40f'      # redirects, unsaved changes
error: '#e74c3c'        # errors, variables that have no value
```

Only `name` and `primary` are required. Colours must be hex values (`#rgb`, `#rrggbb` or `#rrggbbaa`).
Any colour you leave out is taken from a built-in theme (`galaxy` for dark themes and
`catppuccin-latte` for light ones), so you can start with just a couple of colours and add more as you go.
The file's name doesn't matter: the theme is known by its `name`, and a theme with the same name as a
built-in theme replaces it.

Themes are loaded when Posting starts, so restart Posting to see changes to a theme file.
If a theme file has a problem, Posting names the file and the problem in a notification when it starts.

Your themes are listed first in the theme picker, under **Yours**. To list only your own themes, set
`load_builtin_themes: false`. To ignore the themes directory, set `load_user_themes: false`.
To keep your themes somewhere else, set `theme_directory`.

### Themes from Posting 2

Posting 3 reads Posting 2 theme files, as long as their colours are hex values: a theme that uses
colour names or `rgb(…)` needs its colours changed to hex first. The colours listed above are used, and other settings in the
file are ignored, including `author`, `description` and `homepage`, and the custom styles under
`text_area`, `syntax`, `url`, `method` and `variable`. Syntax highlighting and method colours come
from the theme's colours instead (see below).

## Colours in the UI

Syntax highlighting, the URL bar and the method labels all use the theme's colours, so every theme
gets a consistent set of colours without any extra configuration:

| Element | Colour |
|---------|--------|
| `GET` | Primary |
| `POST` | Success |
| `PUT` | Warning |
| `PATCH` | Info |
| `DELETE` | Error |
| `HEAD`, `OPTIONS` | Muted text |
| JSON keys, XML and HTML tags | Primary |
| Strings | Success |
| Numbers | Warning |
| `true`, `false`, `null` | Accent |
| A variable with a value | Success |
| A variable with no value | Error, underlined |
| URL scheme / host | Accent / Secondary |

## Icons

Posting can draw icons in the sidebar tabs, the header and notifications using a
[Nerd Font](https://www.nerdfonts.com/). Icons are on by default in terminals that come with
Nerd Font symbols built in (currently Ghostty), and off elsewhere, since they show up as boxes
without one. If you use a Nerd Font, turn them on with:

```yaml
nerd_fonts: true
```
