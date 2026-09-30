## Overview

Posting can hand text over to your favourite editor and pager. Edit a request body in `vim`, then
browse the JSON response in `less` or [`fx`](https://fx.wtf), and come straight back to Posting.

| Key | Action |
|-----|--------|
| ++f4++ | Open the text in your editor |
| ++f3++ | Open the text in your pager |

These keys work in the request body and the response body. Posting writes the text to a temporary
file, with an extension for JSON, HTML, XML, CSS or YAML content (such as `.json`), so your editor can highlight
it. The editor or pager takes over the terminal until you quit it, then you're back in Posting and
the temporary file is deleted.

## Editors

Press ++f4++ in the request body to edit it in your editor. When you save and quit, your changes
replace the body in Posting.

Press ++f4++ in the response body to open a copy of it in your editor. Changes you make there
aren't brought back into Posting, since the response can't be edited.

Posting uses the `editor` setting from your [configuration](./configuration.md), which defaults to
your `$EDITOR` environment variable:

```yaml
editor: nvim
```

You can also set `POSTING_EDITOR`, which takes priority over the config file.

!!! tip "Graphical editors"

    The editor needs to keep running until you've finished editing. For editors that open a window
    and return straight away, pass their "wait" option. For VS Code, use `code --wait`, or for
    Cursor, `cursor --wait`.

## Pagers

Press ++f3++ to view a request or response body in your pager. Posting uses the `pager` setting,
which defaults to your `$PAGER` environment variable:

```yaml
pager: less -R
```

You can also set `POSTING_PAGER`.

### A pager for JSON

To use a different pager for JSON, such as [`fx`](https://fx.wtf) or [`jless`](https://jless.io),
set `pager_json`. It's used instead of `pager` whenever the body is JSON:

```yaml
pager_json: fx
```

You can also set `POSTING_PAGER_JSON`.

The response body is passed to your pager as it's shown in Posting, so JSON is already formatted,
unless you've set `response.prettify_json: false`.

## How commands are run

The setting is split into words like a shell command, so it can include options (quote any
argument that contains spaces), and the temporary file's path is added to the end. It isn't run
through a shell, so shell features such as pipes and aliases aren't available. To use them, put
them in a small script and set the script as your editor or pager.

If no editor or pager is set, Posting tells you so when you press the key.
