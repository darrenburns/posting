## Overview

`posting remote` drives the Posting you have open from another terminal. A request sent with
`posting remote send` opens and sends in Posting's window, using its active environment, so you
watch the request and its response arrive. The response is also printed in the terminal the command
ran in.

It's made for working beside a coding agent such as Claude Code or Codex: run the agent in one
pane and Posting in another, and the agent can call your API through Posting while you watch. It
can use the requests and environments you already have, and everything it sends lands in your
History.

```bash
posting remote requests                  # what's in the collection
posting remote send users/list           # send a saved request
posting remote send --curl 'curl -X POST https://api.example.com/users -d "{}"'
posting remote response --json           # the active tab's latest response
```

Run `posting remote --help` for the full reference. It is written so an agent can learn the command
from it.

## Commands

| Command | What it does |
|---------|--------------|
| `list` | List the running Postings. |
| `requests` | List the requests saved in the collection. |
| `show` | Print the request in the active tab, in Posting's [request file format](./collections.md). |
| `open [REQUEST]` | Open a request in a tab, without sending it. |
| `send [REQUEST]` | Send a request, wait for the response, and print it. |
| `response` | Print the active tab's latest response. |
| `env [NAME\|FILE...]` | List the environments, after switching to the one named, or to the files given. `--none` switches to no environment. |
| `save [FILE]` | Save the request in the active tab. `--name NAME` names it. |

## Choosing a request

`open` and `send` take a request saved in the collection: its file (`users/get.posting.yaml`, or
just `users/get`), a path to the file, or the request's name. A saved request opens just as it would
from the sidebar: in its own tab if it's already open, unsaved edits and all, and otherwise in the
preview tab.

Instead of a saved request, give one of:

- `--curl COMMAND`: a curl command, imported as if pasted into the URL bar.
- `--yaml FILE`: a request in Posting's YAML format. `--yaml -` reads it from standard input. A path
  to a request file outside the collection works the same way.

Requests that aren't saved open in a tab of their own, which later ones reuse, so a string of them
doesn't fill the tab strip. Once you edit or save that tab it's yours, and the next request gets a
new tab.

With no request given, `open` and `send` use the active tab, so `posting remote send` sends whatever
you have open.

A handy loop for changing a request is to print it, edit it, and send the result:

```bash
posting remote show > draft.posting.yaml
# edit draft.posting.yaml
posting remote send --yaml draft.posting.yaml
```

## Environments

`posting remote env` lists the environments Posting can switch to, marking the active one with `*`,
along with the names of their variables. Values aren't shown, since they're often secrets.

```bash
posting remote env              # list them
posting remote env staging      # switch to staging, then list them
posting remote env --none       # switch to no environment
```

Give a name from the list, or one or more `.env` files to layer. Switching from the command line is
shown in Posting, but isn't remembered for the next launch: which environment Posting starts in stays
your choice.

## Saving requests

`posting remote save` saves the request in the active tab, so a request built from the command line
can join the collection:

```bash
posting remote open --curl 'curl https://api.example.com/health'
posting remote save ops/health --name "Health check"
```

`FILE` is relative to the collection unless it starts with `/`, `./` or `../`, and `.posting.yaml` is
added if it's missing. Without `FILE`, a request that's already saved is saved in place, and an
unsaved one is saved in the collection's top folder under its name. `save` never replaces a different
request that's already saved.

`save` acts on the active tab, so if you have unsaved edits open there, they're what gets saved.

## Output

`send` and `response` print the response the way `curl -i` does: the status line, the headers, a
blank line, then the body. JSON bodies are indented. With `--json`, every command prints its result
as JSON instead, which suits scripts and agents:

```bash
posting remote send users/get --json | jq -r .response.body
```

An error status such as `404` is still a response: `send` exits `0` when a response arrives, and `1`
when none does, for example when the server can't be reached or a variable isn't defined. The error
is printed on standard error.

## Several Postings at once

Each Posting listens for commands while it runs. When more than one is running, `posting remote`
picks the one started in the current directory, or else the one whose collection contains the
current directory or is inside it. To choose one yourself, give its process ID (from
`posting remote list`) with `--instance`, or set `POSTING_INSTANCE`.

## Security

Posting listens on a Unix socket in `$XDG_RUNTIME_DIR/posting`, or in a directory of your own in the
system's temporary directory. The directory is accessible only to you, and Posting refuses to use
one that belongs to anyone else. Nothing listens on the network.

To turn remote control off, set `remote_control.enabled` to `false` in your
[configuration](./configuration.md).
