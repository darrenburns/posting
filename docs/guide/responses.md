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
| ++page-up++ / ++page-down++ | Up / down a page |
| ++shift++ + movement, or ++shift+k++ ++shift+j++ ++shift+h++ ++shift+l++ ++shift+w++ ++shift+b++ | Select while moving |
| ++v++ | Visual mode: moving the cursor selects, as if you were holding ++shift++. ++escape++ leaves it |
| ++shift+v++ / ++f6++ | Select the line |
| ++f7++ | Select the whole body |
| ++y++ / ++c++ | Copy the selection, or the whole body if nothing is selected |
| ++f3++ | Open the body in your [pager](./external_tools.md) |
| ++f4++ | Open the body in your [editor](./external_tools.md) |

You can also click and drag to select, double-click to select a word, and triple-click to select a
line. In visual mode, the character under the cursor is part of the selection, as it is in Vim.

Click **copy** or **wrap on**/**wrap off** below the body to copy it or turn line wrapping on or off,
or choose **Copy response body** or **Toggle response wrap** in the command palette.

!!! tip "Searching a response"

    Posting doesn't have a search in the response body yet. Open the body in a pager such as `less`,
    or a JSON viewer such as [`fx`](https://fx.wtf), with ++f3++. See [External Tools](./external_tools.md).

## Headers

The response headers, one per row.

## Cookies

The cookies the response set, with each cookie's name, value, path, and whether it's `HttpOnly`
or `Secure`.

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
