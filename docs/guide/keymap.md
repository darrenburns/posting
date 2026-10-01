## Overview

Posting's global keyboard shortcuts can be changed in the `keymap` section of your
[configuration file](./configuration.md). Run `posting locate config` to find it.

Press ++f1++ inside Posting to see the shortcuts currently in effect, including any you've changed.
The footer at the bottom of the screen also shows the most useful keys for whatever is focused.

## Changing a shortcut

Each global action has an ID (listed [below](#action-ids)). To change the key for an action, map
its ID to a key in the `keymap` section:

```yaml
keymap:
  send-request: ctrl+r
```

Restart Posting, and ++ctrl+r++ now sends the request. The footer, the help screen and the command
palette all show the new key.

To bind several keys to the same action, separate them with commas (`jump` only uses the first):

```yaml
keymap:
  send-request: ctrl+r,f5
```

A keymap entry *replaces* the default keys for that action. To keep a default key as well as
adding a new one, list both:

```yaml
keymap:
  send-request: ctrl+r,ctrl+j
```

## Key format

A key is written as a key name, optionally preceded by modifiers joined with `+`:

- Modifiers are `ctrl`, `alt` and `shift` (and `meta` and `super`, where your terminal sends them),
  for example `ctrl+r`, `alt+enter` or `ctrl+shift+v`.
- Letters, digits and symbols are written as themselves: `a`, `5`, `/`, `@`.
- Named keys are `enter`, `tab`, `space`, `backspace`, `delete`, `insert`, `escape`, `up`, `down`,
  `left`, `right`, `home`, `end`, `pgup`, `pgdown`, and the function keys `f1` to `f12`.

Keys are case-insensitive, so write ++ctrl+shift+x++ as `ctrl+shift+x` rather than Posting 2's
`ctrl+X`. A comma can't be bound, since commas separate keys.

!!! note "Terminal support"

    Not every terminal can send every key combination, and some are intercepted by your terminal,
    multiplexer or operating system before they reach Posting. Combinations with `shift` on
    non-printing keys, and keys such as `ctrl+enter`, generally need a terminal that supports the
    [Kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/). If a shortcut
    doesn't seem to work, try a simpler combination.

## Action IDs

These are the actions you can rebind, with their default keys:

| ID | Default | Action |
|----|---------|--------|
| `send-request` | ++ctrl+j++, ++alt+enter++ | Send the request |
| `jump` | ++ctrl+o++ | Enter [jump mode](./navigation.md#jump-mode) |
| `commands` | ++ctrl+p++ | Open the [command palette](./command_palette.md) |
| `save-request` | ++ctrl+s++ | Save the request to the collection |
| `new-request` | ++ctrl+n++ | Open a new request tab |
| `close-tab` | ++alt+w++ | Close the request tab |
| `keep-tab` | *(none)* | Keep the [preview tab](./navigation.md#preview-tabs) open |
| `next-tab` | ++alt+right++ | Go to the next request tab |
| `previous-tab` | ++alt+left++ | Go to the previous request tab |
| `search-tabs` | ++alt+down++ | Search the open request tabs |
| `search-requests` | ++ctrl+g++ | Search the requests in the collection |
| `focus-url` | ++ctrl+l++ | Focus the URL bar |
| `focus-method` | ++ctrl+t++ | Open the method menu |
| `toggle-collection` | ++ctrl+h++ | Show or hide the sidebar |
| `expand-section` | ++alt+z++ | Expand the focused panel to fill the screen, or restore it |
| `variables` | ++ctrl+shift+v++ | Open the [variables](./environments.md#the-variables-screen) screen |
| `help` | ++f1++ | Show the keyboard shortcuts |

`expand-section` can't be fully rebound yet: ++alt+z++ keeps working inside the Request and
Response panels, and a new key always expands the Request panel.

A few keys are fixed: ++escape++ cancels a request that's in flight (or closes whatever dialog is
open), ++ctrl+c++ quits, ++ctrl+z++ suspends Posting, and ++ctrl+shift+s++ saves a text screenshot
of the screen to the current directory.

Many terminals send ++ctrl+h++ as ++backspace++. If ++ctrl+h++ doesn't toggle the sidebar for you,
rebind `toggle-collection` or use **View: hide collection** in the command palette.

## Default shortcuts

Beyond the global actions above, the parts of Posting have keys of their own. These can't be
rebound yet.

### Everywhere

| Key | Action |
|-----|--------|
| ++tab++ / ++shift+tab++ | Move focus to the next / previous field |
| ++ctrl+z++ | Suspend Posting (resume it with `fg` in your shell) |

### Method selector

| Key | Action |
|-----|--------|
| ++enter++ / ++space++ | Open the method menu |
| ++g++ ++p++ ++u++ ++a++ ++d++ ++h++ ++o++ | Choose `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD` or `OPTIONS` (these also work in the open menu) |

### URL bar

| Key | Action |
|-----|--------|
| ++enter++ | Send the request, or import the URL bar's contents if it's a curl command |
| ++down++ | Move down to the request tabs |
| ++ctrl+y++ | Copy the URL |
| Type `$` | Suggest variables |

### Tabs

These work on the request tabs, the response tabs and the sidebar tabs.

| Key | Action |
|-----|--------|
| ++left++ / ++h++, ++right++ / ++l++ | Previous / next tab |
| ++down++ / ++j++ / ++enter++ | Move into the tab's content |
| ++up++ / ++k++ | Move back out (from the request tabs, to the URL bar) |

### Headers, query and form tables

| Key | Action |
|-----|--------|
| Type in the last row | Add a new row |
| ++up++ / ++down++ | Move between rows (++up++ from the first row goes back to the tabs, or to the body type in a form) |
| ++ctrl+space++ | Enable or disable the row |
| ++ctrl+x++ | Delete the row |

The Path table's names come from the URL, so only its values can be edited.

### Selectors and checkboxes

| Key | Action |
|-----|--------|
| ++left++ / ++h++, ++right++ / ++l++ | Change a choice such as the body type or auth type |
| ++enter++ / ++space++ | Toggle a checkbox |

### Text areas and inputs

| Key | Action |
|-----|--------|
| ++ctrl+left++ / ++alt+b++, ++ctrl+right++ / ++alt+f++ | Move by word |
| ++home++ / ++end++, ++ctrl+e++ | Start / end of the line |
| ++shift++ + movement keys | Select text |
| ++ctrl+a++ | Select all |
| ++ctrl+w++ / ++alt+backspace++ | Delete the word before the cursor |
| ++ctrl+u++ / ++ctrl+k++ | Delete to the start / end of the line |
| ++ctrl+d++ / ++delete++ | Delete the character after the cursor |
| ++f3++ | Open the text in your [pager](./external_tools.md) (request and response bodies) |
| ++f4++ | Open the text in your [editor](./external_tools.md) (request and response bodies) |

In a list of suggestions, ++up++ and ++down++ move, ++enter++ accepts, and ++escape++ dismisses it.

### Response body

| Key | Action |
|-----|--------|
| ++y++ | Copy the whole body |
| ++w++ | Turn line wrapping on or off |
| ++f3++ / ++f4++ | Open the body in your pager / editor |

### Collection

| Key | Action |
|-----|--------|
| ++enter++ | Open a request, or expand/collapse a folder. With a selection, open every selected request |
| ++up++ / ++k++, ++down++ / ++j++ | Move the cursor |
| ++shift+up++ / ++shift+k++, ++shift+down++ / ++shift+j++ | Extend the selection up / down |
| ++left++ / ++h++, ++right++ / ++l++ | Collapse / expand, or move to the parent / child |
| ++home++ / ++g++, ++end++ / ++shift+g++ | First / last row |
| ++shift+home++ / ++shift+end++ | Extend the selection to the first / last row |
| ++space++ | Expand or collapse a folder |
| `/` | Search the collection |
| ++escape++ | Clear the selection, then the search |
| ++d++ | Duplicate the request, or every selected request |
| ++backspace++ / ++delete++ | Delete the request, or every selected request |

### History

| Key | Action |
|-----|--------|
| ++enter++ | Open the entry |
| ++up++ / ++k++, ++down++ / ++j++ | Move the cursor |
| ++shift+up++ / ++shift+k++, ++shift+down++ / ++shift+j++ | Extend the selection up / down |
| ++home++ / ++g++, ++end++ / ++shift+g++ | First / last entry |
| ++shift+home++ / ++shift+end++ | Extend the selection to the first / last entry |
| ++page-up++ / ++ctrl+u++, ++page-down++ / ++ctrl+d++ | Page up / down |
| ++escape++ | Clear the selection |
| ++backspace++ | Delete the entry, or every selected entry |

### Command palette and menus

| Key | Action |
|-----|--------|
| ++up++ / ++down++ (or ++ctrl+p++ / ++ctrl+n++) | Move the cursor |
| ++enter++ | Run the command, or open a submenu |
| ++escape++ | Go back a level, or close the palette |
| ++backspace++ on an empty search | Go back a level |

## Coming from Posting 2

Posting 3 uses the same action IDs as Posting 2 where the action still exists, so most keymap
entries keep working. A few things changed:

| Posting 2 | Posting 3 |
|-----------|-----------|
| `search-requests` defaulted to ++ctrl+shift+p++ | Now ++ctrl+g++ |
| `expand-section` defaulted to ++ctrl+m++ | Now ++alt+z++ |
| `help` also accepted `ctrl+?` | ++f1++ only |
| `quit` could be rebound | ++ctrl+c++ is fixed |
| `open-in-pager` and `open-in-editor` could be rebound | ++f3++ and ++f4++ are fixed |
| Keys like `ctrl+X` meant ++ctrl+shift+x++ | Write `ctrl+shift+x` |

Keymap entries for IDs Posting 3 doesn't have are ignored.
