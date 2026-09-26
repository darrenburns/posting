## Overview

A *collection* is just a directory on your file system which may or may not contain requests in the `.posting.yaml` format.

There's absolutely nothing special about a collection.
It contains no "special files" or metadata -- it's just a directory.
It could even be empty.
"Collection" is simply the name we give to the directory which we've loaded into Posting.

## The collection browser

Posting displays the currently open collection in the sidebar.
This is called the *collection browser*.

![Posting collection tree](../assets/collection-tree.png){ height=300px }

The name of the currently open collection is displayed in the bottom right corner of the collection browser.
In the example above, the collection is named "sample-collection".

You can navigate this sidebar using the keyboard or mouse.
Open a request by clicking on it or pressing ++enter++ while it has focus,
and it'll be loaded into the main body of the UI.
A marker will also appear to the left of the request's title, indicating that the request is open.
A save operation will overwrite the currently open request.

!!! example "Keyboard shortcuts"

    The collection browser supports various keyboard shortcuts for quick navigation. For example ++shift+j++ and ++shift+k++ can be used to jump through sub-collections.
    Press ++f1++ while the browser has focus to view the full list of shortcuts.


The collection browser can be moved to the left or right side of the screen by setting the `collection_browser.position` configuration option
to either `"left"` or `"right"`.

## The default collection

If you launch Posting without a `--collection` argument, it will load the *default collection*, which is stored in Posting's reserved data directory on your file system.

The default collection can be thought of as a *system wide collection*.
It's a place to keep useful requests that you can easily access from anywhere, without having to manually specify a `--collection` argument.

You can check where this is by running `posting locate collection`.
The default collection is named "default", that name will be displayed in the bottom right corner of the collection browser.

![Posting default collection](../assets/default-collection.png)

This is useful to get started quickly, but you'll probably want to create your own collection directory and load it instead.
This makes it easier to organize your requests and check them into version control.

## Creating a collection

A collection is just a directory, so you can create a collection by simply creating an empty directory anywhere on your file system.

With the directory created, it's time to load it into Posting...

## Loading a collection

If you want to load a collection, you can do so by passing the path to the collection directory to Posting:

```bash
posting --collection path/to/collection
```

### Example

To open a collection (a directory containing requests), use the `--collection` option:

```bash
posting --collection path/to/collection
```

This will recursively find and display requests in the sidebar.
If you don't supply a directory, Posting will use the default collection directory.
You can check where the default collection is by running `posting locate collection`.
## Request and response history

The **History** tab in the collections sidebar keeps recent exchanges, newest first.
Each entry shows the sent method, status, URL, and local time. Click an entry or
highlight it and press `Enter` to restore its saved request configuration and response.
This replaces the current editor contents. Moving the highlight alone does not load
anything. Restoring never sends a request, runs scripts, or changes the session's cookies.

Request snapshots include the method, URL, headers (including disabled rows), body,
query/path parameters, authentication, metadata, options, and script paths. They are
captured before variables are resolved and scripts run, so variable expressions and
script references remain editable. Scripts are references to files relative to the
collection, not archived copies. Missing scripts do not prevent restoration; resending
reports the existing script-loading error. Changed variables or scripts can change what
gets sent next time.

Restored requests are detached from collection files. `Ctrl+S` opens the save dialog
instead of overwriting whichever collection request was open previously. Older history
entries that were recorded without a request snapshot still load their response and
show an explanatory notification; their request configuration cannot be recovered.

A source line identifies the saved response and its timestamp, including in compact
mode. Response Scripts and Trace are unavailable because their output is not retained.
The request's Scripts tab still contains its saved script references. Sending another
request returns the Response pane to its normal live view.

### Keyboard and jump mode

If the sidebar is hidden, show it with `Ctrl+H` first.

1. Press `Ctrl+O`, then `3` to activate the **History tab**.
2. Press `Down` or `j` to move from the tabs into the **history list**.
3. Use `Up`/`Down` or `k`/`j` to highlight an entry; `g`/`G` selects the first/last.
4. Press `Enter` or `l` to restore that request and response. Focus stays in the list.
5. Edit the request as needed and press `Ctrl+J` to send it explicitly.

With History already visible, `Ctrl+O`, then `h` jumps directly to the list. The `h`
target appears only when the list is visible and nonempty. `Ctrl+O`, then `4` activates
Collections; `Down` enters its tree (or use `Ctrl+O`, then `Tab` when that tree is visible).
When the sidebar tabs have focus, `Left`/`Right` or `h`/`l` switches between them. These
keys are context-specific: `h` in jump mode targets history, while `l` in the list restores
an entry. `Escape` cancels jump mode and returns focus to where it was.

Press `Backspace` in the history list to delete an entry. `Ctrl+Backspace` clears the
current collection's history after confirmation. Press `F1` in the list for help.

### Local storage

History is stored in SQLite files under `$XDG_DATA_HOME/posting/history/` (normally
`~/.local/share/posting/history/`), separate from your collection files. Each collection's
absolute, resolved path identifies its history, so moving a collection starts a new
history. Posting retains at most 100 exchanges and 50 MiB of request/response data per
collection, pruning the oldest entries automatically. An exchange larger than 50 MiB
is still displayed but is not saved, and Posting shows a notification.

The database contains request configuration, response bodies and headers (including
cookies), status, and timing. Auth values, headers, bodies, and URLs can contain secrets:
history is local, unencrypted, and created with owner-only file permissions. Resending
uses the current environment, script files, and session cookies; those dependencies are
not archived. To stop saving new entries, set this in your configuration:

```yaml
history:
  enabled: false
```

Or set `POSTING_HISTORY__ENABLED=false`. Existing history remains available for viewing
and deletion. Exchanges are recorded for all HTTP statuses, including errors such as
404 and 500; connection failures without a response are not recorded.
