## Overview

As explained in the [Help System](./help_system.md) section, you can view the keybindings for any widget by pressing ++f1++ or ++ctrl+question-mark++ when that widget has focus.

If you wish to use different keybindings, you can do so by editing the `keymap` section of your `config.yaml` file.
Check the location of that file on your system by running `posting locate config` on the command line.

### Changing the keymap

Actions in Posting have unique IDs which map to a keybinding (listed at the bottom of this page).
For any of these IDs, you can change the keybinding by adding an entry to the `keymap` section of your `config.yaml` file:

```yaml
keymap:
  <ID>: <key-combination>
```

Here's an example of changing the keybinding for the "Send Request" action:

```yaml
keymap:
  send-request: ctrl+r
```

After adding the above entry to `config.yaml` and restarting Posting, you'll notice that that the footer of the app now shows `^r` to send a request rather than the default `^j`.

Now you can press `^r` to send a request *instead of* `^j`.

You can also have multiple keys map to the same action by separating them with commas:

```yaml
keymap:
  send-request: ctrl+r,ctrl+i
```

Note that by adding an entry to the `keymap` you are overriding the default keybinding for that action, so if you wish to keep the default keybinding, you'll need to specify it again:

```yaml
keymap:
  send-request: ctrl+r,ctrl+i,ctrl+j
```

### Key format

Support for keys in the terminal varies between terminals, multiplexers and operating systems.
It's a complex topic, and one that may involve some trial and error.
Some keys might be intercepted before reaching Posting, and your emulator might not support certain keys.

- To specify ++ctrl+x++, use `ctrl+x`.
- To specify ++ctrl+shift+x++, use `ctrl+X` (control plus uppercase "X").
- To specify multiple keys, separate them with commas: `ctrl+shift+left,ctrl+y`.
- To specify a function key, use `f<number>`. For example, ++f1++ would be `f1`.
- To specify `@` (at) use `at` (*not* e.g. ++shift+2++ as this only applies to some keyboard layouts).
- Arrow keys can be specified as `left`, `right`, `up` and `down`.
- `shift` works as a modifier non-printable keys e.g. `shift+backspace`, `shift+enter`, `shift+right` are all acceptable. Support may vary depending on your emulator.
- `alt` also works as a modifier e.g. `alt+enter`.
- `ctrl+enter`, `alt+enter`,`ctrl+backspace`, `ctrl+shift+enter`, `ctrl+shift+space` etc. are supported if your terminal supports the Kitty keyboard protocol.
- Other keys include (but are not limited to) `comma`, `full_stop`, `colon`, `semicolon`, `quotation_mark`, `apostrophe`, `left_bracket`, `right_square_bracket`, `left_square_bracket`, `backslash`, `vertical_line` (pipe |), `plus`, `minus`, `equals_sign`, `slash`, `asterisk`,`tilde`, `percent_sign`.

The only way to know for sure which keys are supported in your particular terminal emulator is to install Textual, run `textual keys`, press the key you want to use, and look at the `key` field of the printed output.

!!! example "Work in progress"
    In the future, I hope to make it easier to discover which keys are supported and when key presses they correspond to for a particular environment directly within Posting. This will likely take the form of a CLI command that outputs key names and their corresponding key presses. For now, if you need assistance, please open a discussion on [GitHub](https://github.com/darrenburns/posting/discussions).

### Binding IDs

These are the IDs of the actions that you can change the keybinding for:

- `send-request` - Send the current request. Default: `ctrl+j,alt+enter`.
- `focus-method` - Focus the method selector. Default: `ctrl+t`.
- `focus-url` - Focus the URL input. Default: `ctrl+l`.
- `save-request` - Save the current request. Default: `ctrl+s`.
- `expand-section` - Expand or shrink the section which has focus. Default: `ctrl+m`.
- `toggle-collection` - Toggle the collection browser. Default: `ctrl+h`.
- `new-request` - Create a new request. Default: `ctrl+n`.
- `commands` - Open the command palette. Default: `ctrl+p`.
- `help` - Open the help dialog for the currently focused widget. Default: `f1,ctrl+question_mark`.
- `quit` - Quit the application. Default: `ctrl+c`.
- `jump` - Enter jump mode. Default: `ctrl+o`.
- `open-in-pager` - Open the content of the focused text area in your $PAGER/$POSTING_PAGER/$POSTING_PAGER_JSON. Default: `f3`.
- `open-in-editor` - Open the content of the focused text area in your $EDITOR/$POSTING_EDITOR. Default: `f4`.
- `search-requests` - Go to a request by name. Default: `ctrl+shift+p`.
