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
## Response history

The **History** tab in the collections sidebar keeps recent responses, newest first.
Each entry shows the method, status, URL, and local time. Click an entry or select it
with the arrow keys (or `j`/`k`) and press `Enter` to view its body, headers, and cookies
in the Response pane. Your current request stays in the editor, and viewing history
does not send a request or change the session's cookies.

A source line identifies the saved response and its timestamp, including in compact
mode. Scripts and Trace are unavailable for saved responses because their output is
not retained. Sending another request returns the Response pane to its normal live view.

Press `Backspace` in the history list to delete an entry. `Ctrl+Backspace` clears the
current collection's history after confirmation. You can also reach the History tab
using jump mode (`Ctrl+O`, then `3`).

History is stored in SQLite files under `$XDG_DATA_HOME/posting/history/` (normally
`~/.local/share/posting/history/`), separate from your collection files. Each collection's
absolute, resolved path identifies its history, so moving a collection starts a new
history. Posting retains at most 100 responses and 50 MiB of response data per collection,
pruning the oldest entries automatically. A response larger than 50 MiB is still displayed
but is not saved, and Posting shows a notification.

The database contains response bodies, headers (including cookies), method, URL, status,
and timing. It does not retain request headers or bodies. URLs and responses can contain
secrets: history is local, unencrypted, and created with owner-only file permissions.
To stop saving new responses, set this in your configuration:

```yaml
history:
  enabled: false
```

Or set `POSTING_HISTORY__ENABLED=false`. Existing history remains available for viewing
and deletion. Responses are recorded for all HTTP statuses, including errors such as
404 and 500; connection failures without a response are not recorded.
