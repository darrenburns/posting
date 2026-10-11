---
name: using-posting
description: Sends HTTP, GraphQL and gRPC requests through the Posting API client the user has open in another terminal, with `posting remote`, so they watch each request and response. Use instead of curl when calling, testing or debugging an API while Posting is running, or when the user mentions Posting or the request they have open.
---

# Using Posting

The user may have Posting, a terminal API client, open beside you. Requests you send with
`posting remote` open and send in their Posting window, using its active environment, so they see
what you sent and what came back. The response is printed for you too.

## Check whether Posting is open

Before calling an API, run:

```bash
posting remote list
```

- A Posting is listed: send this project's API requests through it, as below.
- "No Posting is running", or `posting` isn't installed: use curl as usual. Don't start Posting
  yourself; it's a full-screen app for the user's terminal.
- "Postings are running; choose one": pick the one whose collection or directory matches this
  project, and pass `--instance PID` to every command.

## Send requests

Prefer the user's saved requests, which carry their headers, auth and variables:

```bash
posting remote requests                     # METHOD, FILE, NAME and URL of each saved request
posting remote send users/get               # by file, without .posting.yaml
posting remote send "Get user"              # or by name
```

For a request that isn't saved, give a curl command, or YAML on standard input. YAML avoids shell
quoting problems with JSON bodies:

```bash
posting remote send --curl 'curl -X POST https://api.example.com/users -H "Content-Type: application/json" -d "{}"'

posting remote send --yaml - <<'EOF'
method: POST
url: ${BASE_URL}/users
headers:
- name: Authorization
  value: Bearer ${API_TOKEN}
body:
  content: '{"name": "Ada"}'
  content_type: application/json
EOF
```

`${NAME}` is filled in from the user's active environment. Use the variables they have rather than
pasting values: `posting remote env` lists the environments and their variable names (never their
values).

Add `--json` to get structured output: `outcome`, `error`, and `response` with `status`, `headers`
and `body`. For example, `posting remote send users/get --json | jq -r .response.body`.

`send` exits 0 whenever a response arrives, including 4xx and 5xx; read the status. It exits 1 when
no response arrives (connection refused, timeout, undefined variable), with the reason on stderr.

## See what the user sees

```bash
posting remote show        # the request in the user's active tab, as YAML
posting remote response    # the latest response in that tab
```

Use these when the user says "this request", "the one I have open" or "look at the response".

## Rules

- **Don't send the user's tab unasked.** `posting remote send` with no request sends whatever is in
  their active tab. Only do that when they ask.
- **Ask before risky requests.** Before a POST, PUT, PATCH or DELETE against anything other than a
  local or test environment, use `posting remote open` instead of `send`, and ask the user to send
  it themselves.
- **Say before switching environments.** `posting remote env NAME` changes the environment the user
  is working in. Tell them, and switch back when you're done if they were using another.
- **Keep bulk work out of Posting.** Loops, polling, load tests and large downloads would flood the
  user's window. Use curl for those, and send one representative request through Posting.
- **Save only when asked.** `posting remote save FILE --name NAME` saves the request in the active
  tab into the collection. Open the request first, and never use it to overwrite the user's work.

## Other commands

| Command | Does |
|---|---|
| `posting remote open REQUEST` | Opens a request (same forms as `send`) without sending it |
| `posting remote env NAME` | Switches environment; `--none` switches to none |
| `posting remote save [FILE] --name NAME` | Saves the active tab's request |
| `posting remote --help` | Full reference |
