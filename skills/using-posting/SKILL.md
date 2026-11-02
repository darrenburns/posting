---
name: using-posting
description: Sends HTTP, GraphQL and gRPC requests through Posting with `posting remote`, or runs saved requests and collections without a running instance using `posting run`. Use when calling, testing or debugging an API with Posting, when the user mentions Posting, or when an agent needs to execute saved Posting requests headlessly.
---

# Using Posting

The user may have Posting, a terminal API client, open beside you. Requests you send with
`posting remote` open and send in their Posting window, using its active environment, so they see
what you sent and what came back. The response is printed for you too.

For explicitly headless work, use `posting run` on saved files instead; it doesn't need a running
Posting or an instance-discovery step. See **Run without the UI** below.

## Check whether Posting is open

Unless the user asked for headless execution, check whether Posting is open before calling an API:

```bash
posting remote list
```

- A Posting is listed: send this project's API requests through it, as below.
- "No Posting is running": use `posting run` for saved requests, or curl for ad hoc calls. Don't
  start the full-screen UI yourself.
- `posting` isn't installed: use curl as usual.
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

## Run without the UI

```bash
posting run api/health.posting.yaml --json
posting run api --env dev --json > results.json
posting run -c api users/get.posting.yaml --env staging --json
jq '.results[] | {file, outcome, error, status: .response.status}' results.json
```

Give a saved `.posting.yaml` file or a directory, not a request display name. Directories run
recursively in lexical file-path order; numbered filenames order dependent requests. With `-c`,
the path is relative to that collection root; omit it to run the whole collection. Without `-c`,
the root is the directory given, or the file's parent directory. Upload and proto paths use this root.

The default environment is `posting.env` + `posting.local.env` in the root, never the UI's last
selection. `--env NAME` selects a named environment there; an explicit file path is relative to
the working directory. Repeat `--env` to layer. Shell secrets can be referenced from these files.
Cookies and successful response captures carry forward within one run, and nothing is written
to UI history. `--fail-fast` stops after the first failure; otherwise later requests still run.

Unlike `remote send`, `run` exits 1 for HTTP 4xx/5xx, GraphQL errors, non-OK gRPC statuses, transport
errors, or failed captures. It exits 0 only when all requests pass, and 2 for invalid arguments.
Empty collections, malformed files and legacy Python scripts are rejected. There are no custom
response assertions: inspect JSON output to decide whether the response meets the task's expectations.
An exit code of 1 can still include a useful response, such as an expected 404. Read the report
even when the command exits nonzero; don't assume every failed result is a transport failure.

`--json` includes response headers and bodies, which may contain secrets. Don't publish the report
without checking its contents. Inspect the selected requests and environment before running a whole
collection: it may contain destructive requests, and headless mode doesn't make those safe.

## Rules

- **Don't send the user's tab unasked.** `posting remote send` with no request sends whatever is in
  their active tab. Only do that when they ask.
- **Ask before risky requests.** Before a POST, PUT, PATCH or DELETE against anything other than a
  local or test environment, use `posting remote open` instead of `send`, and ask the user to send
  it themselves. For headless runs, get approval before sending risky requests to a shared environment.
- **Say before switching environments.** `posting remote env NAME` changes the environment the user
  is working in. Tell them, and switch back when you're done if they were using another.
- **Keep bulk work out of Posting.** Loops, polling, load tests and large downloads would flood the
  user's window. Use `posting run` for saved collections, or curl for other bulk work, and send one
  representative request through the UI when useful.
- **Save only when asked.** `posting remote save FILE --name NAME` saves the request in the active
  tab into the collection. Open the request first, and never use it to overwrite the user's work.

## Other commands

| Command | Does |
|---|---|
| `posting remote open REQUEST` | Opens a request (same forms as `send`) without sending it |
| `posting remote env NAME` | Switches environment; `--none` switches to none |
| `posting remote save [FILE] --name NAME` | Saves the active tab's request |
| `posting remote --help` | Full reference |
| `posting run --help` | Headless runner reference |
