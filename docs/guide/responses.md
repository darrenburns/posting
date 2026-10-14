## Overview

When a response arrives, it's shown in the **Response** panel. The panel's title shows the status
code and reason, coloured green for success, yellow for redirects and red for errors, along with
the size of the response and how long it took.

<figure class="screen">
--8<-- "response.html"
<figcaption>A JSON response, formatted and highlighted.</figcaption>
</figure>

While a request is in flight, the URL bar shows its progress through each stage of the exchange, and
you can press ++escape++ to cancel it. If a request fails, for example because the host can't be
found or the connection times out, the Response panel explains what went wrong.

The response has four tabs. Jump to them with ++ctrl+o++ followed by ++a++ ++s++ ++d++ ++f++.
A gRPC response has a **Trailers** tab in place of **Cookies**. See
[gRPC responses](./requests.md#grpc-responses).

## Server-sent events (SSE)

Send an ordinary HTTP request to an SSE endpoint. Posting detects `Content-Type: text/event-stream`
and replaces **Body** with a live **Events** log. GET and POST both work, including POST requests
with a JSON body. Headers, authentication, cookies, TLS settings and variables work as usual;
add `Accept: text/event-stream` if the endpoint requires it.

Events appear as they arrive, with timestamps and event names. Select an event to inspect its
data and last event ID; JSON data supports highlighting, search and the JSON tree. Multiline
`data` fields are joined with newlines. Comments are ignored, and an unfinished event isn't
dispatched when the connection ends. Gzip and deflate responses are decoded while streaming.

The log follows new events until you move through it or inspect a payload. Arrivals then leave
your selection alone and increment the unread count. Press ++f++ in the log to follow again.
On narrow panels, ++enter++ opens the full-width inspector and ++escape++ returns to the log.

Press ++escape++ from the log to stop the stream. Received events remain available and are saved
in history when the stream stops or ends, subject to the usual history size limit. A connection
failure keeps the received data and shows **Stream interrupted** with an error in the log.

In the UI, the request timeout applies until response headers arrive; an established SSE stream
stays open until you stop it or the server closes it. Posting does **not** automatically reconnect
or replay a request, and ignores `retry` fields. Send again to reconnect; add a `Last-Event-ID`
request header if you want the server to resume from a particular event.

For bounded memory use, Posting stops with an explanation if a stream exceeds 10,000 events,
64 MiB of decoded response data, or 1 MiB in a single event or line. The retained raw SSE body is
available to exports and [`posting run --json`](./running.md); headless runs keep their timeout
for the entire response and report interruptions as failures.

## Body

The response body, syntax highlighted based on its `Content-Type`. JSON, HTML, XML, CSS, JavaScript
and YAML are highlighted. JSON is also indented to make it easier to read; to see JSON exactly as the
server sent it, set `response.prettify_json: false` in your [configuration](./configuration.md).

Move into the body with ++down++ (or jump to it with ++ctrl+o++ then ++a++, then ++down++) to move
through it and select text. The body is read-only, and supports the same Vim-style keys as Posting 2.
Below the body, Posting shows its content type, number of lines and, while the body has focus, the
cursor's line and column.

| Key | Action |
|-----|--------|
| ++up++ ++down++ ++left++ ++right++ / ++k++ ++j++ ++h++ ++l++ | Move the cursor |
| ++w++ / ++b++, ++ctrl+right++ / ++ctrl+left++ | Next / previous word |
| ++0++ `^` ++home++ ++ctrl+a++ / `$` ++end++ ++ctrl+e++ | Start / end of the line |
| ++g++ / ++shift+g++ | Top / bottom of the body |
| `%` | The bracket matching the one under the cursor |
| ++page-up++ ++ctrl+b++ / ++page-down++ | Up / down a page |
| ++ctrl+u++ / ++ctrl+d++ | Up / down half a page |
| ++shift++ + movement, or ++shift+k++ ++shift+j++ ++shift+h++ ++shift+l++ ++shift+w++ ++shift+b++ | Select while moving |
| ++v++ | Visual mode: moving the cursor selects, as if you were holding ++shift++. ++escape++ leaves it |
| ++shift+v++ / ++f6++ | Select the line |
| ++f7++ | Select the whole body |
| ++y++ / ++c++ | Copy the selection, or the whole body if nothing is selected |
| ++ctrl+f++ / `/` | Find text in the body |
| ++n++ / ++shift+n++ | Next / previous match while search is open |
| ++f3++ | Open the body in your [pager](./external_tools.md) |
| ++f4++ | Open the body in your [editor](./external_tools.md) |

You can also click and drag to select, double-click to select a word, and triple-click to select a
line. In visual mode, the character under the cursor is part of the selection, as it is in Vim.

Click **copy** or **wrap on**/**wrap off** below the body to copy it or turn line wrapping on or off,
or choose **Copy response body** or **Toggle response wrap** in the command palette.

### Searching a response

With the body focused, press ++ctrl+f++ or `/` to open Find. Matches highlight as you type, and the
search bar shows the current match and total, or **No matches**. Search is literal (not a regular
expression) and ignores case.

Press ++enter++ for the next match or ++shift+enter++ for the previous one; both wrap around at the
ends of the body. Press ++up++ to return to the body while keeping search open, then use ++n++ and
++shift+n++ to navigate matches, or ++y++ to copy the selected match.

++escape++ closes search and clears its highlights, leaving the result selected for copying.
Reopening Find selects the previous query so you can replace it. Each request tab keeps its own search.

Unlike Posting 2, ++ctrl+f++ opens Find rather than paging down. Use ++page-down++ for a full page or
++ctrl+d++ for half a page (++ctrl+u++ moves up half a page).

### Images

Responses with an `image/*` content type open as a preview. Posting supports PNG, JPEG, GIF
(the first frame) and static WebP. It also recognises images when the server omits the content type
or uses `application/octet-stream`; explicit text content types stay in the text viewer.

The image fits inside the Body tab without stretching. Terma uses Kitty or Sixel graphics when
the terminal supports them, and coloured character cells otherwise. Press ++alt+z++ to expand the
Response panel for a larger preview. The footer shows the decoded format and pixel dimensions.

With the preview focused, press ++f2++ (or click **text**) to inspect the response as text, and
++f2++ (or **preview**) to return. Press ++s++ or click **save** to save the original response body
bytes, not a screenshot or a re-encoded image. The save dialog accepts absolute or relative paths
and `~/`; it never overwrites an existing file. These actions also appear in the command palette.

Previews are limited to 32 MiB of encoded data, 16,777,216 pixels and 16,384 pixels per side.
Unsupported formats, corrupt images and larger images show an explanation, with **text** and
**save** still available. Preview errors do not change the HTTP status or response bytes.
Images saved in [history](./collections.md#history) can be previewed again when you reopen them.

### JSON tree

For a valid JSON body, click **JSON tree** below the body or press ++t++ while the body has focus.
The tree uses Terma's JSON viewer, with collapsible objects and arrays and coloured values.
Posting detects JSON from the body even if the server omits or mislabels its content type. The
tree also works with responses loaded from history and decoded JSON messages in the gRPC inspector.
Empty, malformed and non-JSON bodies keep the text view.

Type an RFC 9535 JSONPath in the field below the tree to filter it, for example `$.users[*].name`
or `$.users[?@.admin == true]`. Matches keep their ancestors for context and include complete
subtrees; array indices refer to the original response. The bar shows the match count. An invalid
query displays an error and keeps the last valid results; ++escape++ or **×** clears the filter.

| Key | Action |
|-----|--------|
| ++up++ / ++down++, ++k++ / ++j++ | Move between nodes |
| ++left++ / ++right++, ++h++ / ++l++ | Collapse / expand a node |
| ++home++ / ++end++ | First / last visible node |
| ++e++ / ++shift+e++ | Expand / collapse all nodes |
| ++ctrl+f++ / `/` | Focus JSONPath, selecting the previous query |
| ++enter++ in JSONPath | Return to the tree |
| ++y++ / ++c++ | Copy the current node's value as JSON, including its complete subtree |
| ++shift+y++ | Copy the current node's JSONPath |
| ++t++ | Return to the text view |

Click **Text view** or press ++t++ to return to text search, selection and wrapping; the text
cursor, selection and search are preserved. Each request tab keeps its own view and filter.
A new response or selected message clears the filter so it cannot hide the new data. Object keys
are sorted in the tree; the text view retains their original order. **Copy response body** in the
command palette and the ++f3++/++f4++ external tools still use the complete, unfiltered body.
Tree rows do not wrap; use the text view to read long values with wrapping enabled.

## Headers

The response headers, one per row.

Move into the table with ++down++, then move between rows with ++up++ / ++k++ and ++down++ / ++j++.
These keys copy the row under the cursor:

| Key | Copies |
|-----|--------|
| ++y++ / ++c++ | The value |
| ++n++ | The name |
| ++b++ | Both, as `Name: value` |

In **Headers** and **Trailers**, select several rows with ++shift+up++ / ++shift+down++,
++shift++-click, or dragging; ++ctrl+a++ selects all. With an explicit selection, ++y++ / ++c++
copies the selected rows as `Name: value` lines, preserving their order and repeated names.
Press ++escape++ to clear the selection and return to the single-row shortcuts above.

The same keys work in the **Cookies** and **Trailers** tables. ++b++ copies a cookie as
`name=value`, the way a `Cookie` header carries it. The command palette also has **Copy header
value**, **Copy header name** and **Copy header name and value** (or **cookie** or **trailer**) for
the row under the cursor on the tab that's showing.

## Cookies

The cookies the response set, with each cookie's name, value, path, and whether it's `HttpOnly`
or `Secure`. Copy a cookie's value with ++y++, its name with ++n++, or `name=value` with ++b++. See
[Headers](#headers).

Cookies from responses are remembered until you quit Posting, and sent with later requests to the
same site, unless you turn off **Attach cookies** in the request's [Options](./requests.md#options).

## Trace

A timeline showing how long each stage of the exchange took:

| Stage | |
|-------|-|
| connect | Opening a connection to the server |
| tls handshake | Setting up an encrypted connection (skipped for `http://` URLs) |
| send headers | Sending the request line and headers |
| send body | Sending the request body |
| receive headers | Waiting for the server, until the response headers arrive |
| receive body | Reading the response body |
| response closed | Finishing the exchange |

The trace is a quick way to see whether a slow request is slow to connect, slow on the server, or
slow to download. The same stages are shown as small squares in the URL bar while the request is in
flight.

Traces are kept in [history](./collections.md#history), so you can look back at the timing of
earlier responses too.

## Making room

Press ++alt+z++ in the Response panel to expand it to fill the screen, and again to restore it.
**Layout: side by side** in the command palette puts the response beside the request, which suits
wide terminals.

## Responses in other tabs

Each [request tab](./navigation.md#request-tabs) keeps its own response. If a response arrives for
a tab you're not looking at, it waits in that tab until you go back to it, and every response is
added to [history](./collections.md#history).
