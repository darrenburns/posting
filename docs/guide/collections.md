## Overview

A *collection* is a directory of requests. Each request is a YAML file ending in `.posting.yaml`
(see [Requests](./requests.md#the-request-file-format)), and folders inside the directory group
requests together.

There's nothing special about a collection: no hidden files or metadata, just a directory.
It could even be empty. That makes collections easy to share, review and keep in version control
alongside the code they're for.

Collections made with Posting 2 work in Posting 3, and the other way around.

## Opening a collection

Pass the collection's directory to Posting with `--collection` (or `-c`):

```bash
posting --collection path/to/my-api
```

Posting reads every `.posting.yaml` file in the directory and its subdirectories, and shows them
in the sidebar. Hidden directories (those starting with `.`) and `node_modules` are skipped.

To start a new collection, create an empty directory and open it:

```bash
mkdir my-api
posting -c my-api
```

If a request file can't be read, Posting tells you which one in a notification when it starts,
and loads the rest of the collection as normal.

## The default collection

If you start Posting without `--collection`, it opens the *default collection*: a directory that
Posting keeps for you, whichever directory you start Posting from. It's a good home for requests you
want to have to hand from anywhere.

Run `posting locate collection` to see where it is:

```bash
$ posting locate collection
Default collection directory:
/home/you/.local/share/posting/default
```

## The collection browser

The **Requests** tab of the sidebar shows the collection you've opened, with its name at the top.

Folders are listed first, sorted by name, followed by requests, which are grouped by method
(`GET`, `POST`, `PUT`, `PATCH`, `DELETE`, then `HEAD` and `OPTIONS`) and then sorted by name.
Sorting is case-sensitive, so capitalised names come first. Each request shows its
colour-coded method and its name. The request in the current tab
is shown in bold, other open requests are marked with a dot, and the [preview tab](./navigation.md#preview-tabs)'s
request is shown in italics.

- Press ++enter++, or click, to open a request or to expand or collapse a folder.
- Double-click a request to open it in a tab that stays open.
- Hover over a request (or move the cursor to it) to see its description, if it has one.
- Press `/`, or ++ctrl+g++ from anywhere, to [search the collection](./navigation.md#searching-for-requests).
- Press ++d++ to [duplicate](./requests.md#duplicating-a-request) the request under the cursor, and
  ++backspace++ to [delete](./requests.md#deleting-a-request) it.

Press ++ctrl+h++ to hide or show the sidebar. To put it on the right of the screen, or have it
hidden when Posting starts, see the `collection_browser` settings in [Configuration](./configuration.md).

### Selecting several requests

Hold ++shift++ while you move the cursor with the arrow keys, ++home++ or ++end++ to select a range
of rows. You can also shift-click a row to select everything up to it, or drag across rows with the
mouse. Selected rows are highlighted.

With a selection, these keys act on every selected request:

- ++d++ duplicates each request. The copies become the new selection.
- ++backspace++ deletes them all, after one confirmation.
- ++enter++ opens each request in a tab of its own.

Selected folders are skipped. Press ++escape++, move the cursor without ++shift++, or click a row
to clear the selection.

### Folders

Folders in the collection are ordinary directories. To put a request in a folder, type the
folder's path in the **Folder** field when you [save](./requests.md#saving-a-request) the request,
for example `users` or `admin/users`. Posting creates the folders for you.

When you save a new request, the **Folder** field starts out as the folder the cursor is on in the
collection browser, so moving the cursor to the right place first saves you some typing.

Folders only appear in the collection browser once they contain a request.

## Changes on disk

Posting checks the collection for changes every second. If you add, edit or remove request files
outside Posting (in your editor, or by switching branches in git, say), the collection browser
updates to match.

Open tabs whose files changed are reloaded too, unless you've made changes in them that you haven't
saved. Posting never overwrites unsaved work.

To turn this off, set `watch_collection_files: false` in your [configuration](./configuration.md).
You can still reload the collection by hand with **Reload collection** in the command palette.

## History

The **History** tab in the sidebar lists the responses you've received, newest first.
Each entry shows the method, the status code, the time the request was sent, and the URL.

<figure class="screen">
--8<-- "history.html"
<figcaption>A response reopened from History, in the side by side layout, showing its timing.</figcaption>
</figure>

Press ++enter++ on an entry, or click it, to open it. The request opens in the
[preview tab](./navigation.md#preview-tabs) as you wrote it when you sent it, and the response is
shown as it was received, including its headers, cookies and [timing](./responses.md#trace). Opening an entry
doesn't send anything.

The request is opened as a new, unsaved request, so you can change it and send it again, or save it
into the collection, without affecting the request it came from. It's stored as you wrote it,
before variables were filled in, so sending it again uses your current variables.

To remove entries:

- press ++backspace++ on an entry to delete it
- select several entries, as you [select requests](#selecting-several-requests), and press
  ++backspace++ to delete them together
- choose **Clear history** from the command palette to delete them all

In jump mode, ++4++ takes you straight to the history list.

### Where history is kept

History is saved separately for each collection, so it's there the next time you open the collection.
Posting keeps the 100 most recent exchanges, up to a total of 50 MiB of request and response bodies,
and deletes the oldest as new ones arrive. Requests that fail without getting a response aren't
recorded.

History is stored in `$XDG_DATA_HOME/posting/history/` (normally `~/.local/share/posting/history/`),
outside your collection, so it never ends up in version control.

!!! warning "History can contain secrets"

    Requests and responses often contain credentials: tokens in headers, passwords in bodies, cookies
    in responses. History files are only readable by your user, but they aren't encrypted.

To stop saving history to disk, set:

```yaml
history:
  enabled: false
```

or set `POSTING_HISTORY__ENABLED=false`. The History tab then only shows responses from the current
session, and forgets them when you quit.

History isn't shared with Posting 2, which stored it differently.
