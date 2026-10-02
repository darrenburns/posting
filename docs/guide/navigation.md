Posting is designed to be driven from the keyboard, but everything can be clicked too.

## The layout

From top to bottom, Posting's screen is made up of:

- **The sidebar**, on the left, running the full height of the screen. It has two tabs:
  **Requests**, the requests in your [collection](./collections.md), and **History**, the responses
  you've received.
- **The header**, a single row showing the active [environment](./environments.md) and your
  `user@host`. Click the environment's name to switch to another.
- **The request tabs**, one for each request you have open.
- **The URL bar**, with the method selector on its left.
- **The Request panel**, where you edit the request's headers, body, parameters, auth and options.
- **The Response panel**, which shows the response. It sits below the request, or beside it in the
  side by side layout.
- **The footer**, showing the most useful keys for whatever has focus.

The dividers between the sidebar and the panels, and between the request and response, can be
dragged with the mouse to resize them.

### Changing the layout

A few commands in the [command palette](./command_palette.md) (++ctrl+p++) change the layout for
the rest of the session:

- **Layout: side by side** / **Layout: stacked** puts the response beside or below the request.
- **View: hide collection** / **View: show collection** hides or shows the sidebar. ++ctrl+h++ does
  the same.
- **View: expand request** / **View: expand response** makes one panel fill the screen.
  ++alt+z++ expands the Request or Response panel, whichever has focus (the Request panel if neither
  does), and pressing it again restores both.
- **View: compact spacing** / **View: standard spacing** removes or restores the blank rows between
  parts of the screen.

Posting also switches to compact spacing on its own when your terminal is too short to spare
the blank rows.

To make any of these permanent, set `layout`, `spacing`, `collection_browser.show_on_startup` or
`collection_browser.position` in your [configuration](./configuration.md).

## Jump mode

Jump mode is the fastest way to get around.

Press ++ctrl+o++ and a label appears over every part of the screen you can move to. Type a label
to jump straight there. Press ++escape++ to leave jump mode without moving.

<figure class="screen">
--8<-- "jump.html"
<figcaption>Jump mode: every pane, tab and request gets a label.</figcaption>
</figure>

Some labels are always the same, so they soon become muscle memory:

| Label | Jumps to |
|-------|----------|
| ++1++ | The method selector |
| ++2++ | The URL bar |
| ++3++ | The requests in the collection |
| ++4++ | The history list |
| ++q++ ++w++ ++e++ ++r++ ++t++ ++y++ ++u++ | The request tabs: Headers, Body, Path, Query, Auth, Info and Options. A GraphQL request has no Body tab, and ++r++ jumps to its Params tab. A gRPC request has no Body, Path or Query tab, and ++q++ jumps to its Metadata tab. |
| ++i++ ++o++ | A GraphQL request's Query and Variables tabs, or a gRPC request's Message and Proto tabs |
| ++a++ ++s++ ++d++ ++f++ | The response tabs: Body, Headers, Cookies and Trace. A gRPC response has Trailers in place of Cookies. |

The request and response tabs follow the rows of a QWERTY keyboard, so their labels sit roughly
where the tabs are on screen.

Everything else on screen gets a label from the remaining letters: requests in the collection,
history entries, open request tabs, and the fields inside the current tab. Some of these labels
are two letters long; the labels narrow down as you type.

## Moving around with the keyboard

- ++tab++ and ++shift+tab++ move focus forward and backward through the fields on screen.
- The arrow keys (or ++h++ ++j++ ++k++ ++l++ where you're not typing) move within a part of the
  screen, and between neighbouring parts where it makes sense. For example, ++down++ in the URL bar
  moves into the request tabs, ++down++ from a tab moves into its content, and ++up++ from the top of
  a tab's content moves back to the tabs.
- ++ctrl+l++ jumps to the URL bar from anywhere, and ++ctrl+t++ opens the method menu.

## Request tabs

Every request you open gets a tab above the URL bar. Each tab shows the request's method and name.
A dot (`●`) marks a tab with unsaved changes, and `…` marks a tab whose request is being sent.

| Key | Action |
|-----|--------|
| ++ctrl+n++ | Open a new, empty request tab |
| ++alt+right++ / ++alt+left++ | Go to the next / previous tab |
| ++alt+down++ | Search your open tabs by name |
| ++alt+w++ | Close the current tab |

You can also click a tab to switch to it, click its `✕` to close it, click `+` for a new tab, or
click `▾` to search your tabs. When there are more tabs than fit, scroll the strip with the mouse
wheel, or click the `‹` and `›` arrows at its ends.

!!! warning

    Closing a tab doesn't ask for confirmation, even if it has unsaved changes.

Each tab sends its own requests. You can send a slow request, move to another tab, and carry on
working: when the response arrives it's kept in its tab, and Posting doesn't pull you away from what
you're doing. Press ++escape++ to cancel a request that's in flight.

### Preview tabs

When you open a request from the collection or from history, it opens in a *preview tab*, whose
name is shown in italics. Opening another request replaces the preview tab instead of opening
yet another tab, so you can browse through your collection without leaving a trail of tabs behind.

The preview tab becomes an ordinary tab, which stays open, as soon as you:

- edit the request, send it, or save it
- double-click its tab, or double-click the request in the collection
- choose **Keep tab open** from the command palette

If a request is already open in a tab, opening it again takes you to that tab.

## Searching for requests

Press ++ctrl+g++ from anywhere to search the requests in your collection. You can also press `/`
while the collection has focus. The collection is filtered as you type:

- Separate words with spaces. Every word must match part of a request's name or folder, or the
  start of its method, so `post users` finds `POST` requests in a `users` folder.
- Press ++enter++ or ++down++ to move into the results, and ++enter++ to open a request.
- Press ++escape++ to clear the search.

## Mouse support

Anything you can focus with the keyboard can be clicked. A few things are only a click away:

- Hovering over a request in the collection shows its description, if it has one.
- Clicking the environment name in the header opens the environment switcher.
- Clicking the Posting logo in the bottom right corner opens a menu with links to these docs,
  sponsoring the project, and Mastodon.
- Clicking **wrap on**/**wrap off** or **copy** under the response body toggles line wrapping or copies the body.

## Focus settings

Three settings choose where the cursor goes at key moments:

| Setting | Values | Default |
|---------|--------|---------|
| `focus.on_startup` | `url`, `method`, `collection` | `url` |
| `focus.on_request_open` | `url`, `method`, `headers`, `body`, `path`, `query`, `info` | Focus doesn't move |
| `focus.on_response` | `body`, `tabs` | Focus doesn't move |

For example, to jump into the response body whenever a response arrives:

```yaml
focus:
  on_response: body
```

`focus.on_response` only applies to the tab you're looking at: a response for a tab in the
background never moves the cursor.

## Getting help

Press ++f1++ at any time to see the keyboard shortcuts, including any you've changed in your
[keymap](./keymap.md). See [Help](./help_system.md) for more ways to find your way around.

## Quitting

Press ++ctrl+c++, or choose **Quit Posting** from the command palette.
