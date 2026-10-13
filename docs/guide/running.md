## Overview

`posting run` sends saved requests without opening the terminal UI or connecting to a running
Posting. Agents and scripts can use your existing requests, authentication and environments,
then inspect the responses:

```bash
posting run api/health.posting.yaml --json
posting run api --env dev --json > results.json
jq -r '.results[0].response.body' results.json
```

It uses the same HTTP, GraphQL and gRPC clients as the UI, including authentication, TLS settings,
uploads, request options and variable substitution. It doesn't open tabs, save history, or change
the UI's environment. To send through a Posting someone is watching, use
[`posting remote`](./remote_control.md) instead.

## Working with agents

The bundled [`using-posting` skill](./agents.md) teaches agents both commands: use `remote` when
Posting is open beside you, and `run` for saved requests when no instance is running or when you
ask for headless execution. Installing the skill with `posting remote skill` doesn't require a
running instance either.

Use `--json` so the agent gets response statuses, headers and bodies to work with. Error responses
are included too: an exit code of `1` doesn't necessarily mean no response arrived. An agent can
inspect a `404`, for example, and decide whether it was expected for the task.

This is a request-execution command, not an assertion-based test runner. It can provide basic CI
smoke checks, but doesn't verify that a successful response contains the right application data.
Custom response assertions are outside its current scope.

## Selecting requests

Give a `.posting.yaml` file to run one request, or a directory to run every request inside it,
recursively. Hidden directories, `node_modules` and `__pycache__` are skipped, just as in the UI.
An empty collection is an error, rather than a successful run with no checks.

The collection root is the directory you give, or the parent directory of a single request file.
Use `--collection` (`-c`) to select a different root. With `-c`, a relative request or subdirectory
path is relative to that root; with no path, the whole collection runs:

```bash
posting run -c api users/get.posting.yaml
posting run -c api users
posting run -c api
```

Upload paths and gRPC schema paths are relative to the collection root, not each request's folder.
Use `-c` when running a nested request that uses files from its parent collection.

Options may appear before or after the path. Unlike `remote send`, `run` takes file paths, not
request display names, and the `.posting.yaml` extension is required for a single file.

## Ordering and dependent requests

Requests run sequentially in case-sensitive lexical order of their relative file paths. This is
different from the sidebar, which groups requests by method and display name. Use numbered filenames
when order matters, for example:

```text
01-login.posting.yaml
02-create-user.posting.yaml
03-get-user.posting.yaml
04-delete-user.posting.yaml
```

Cookies carry forward within the run, subject to each request's `attach_cookies` option.
[Response captures](./environments.md#response-captures) from successful requests become session
variables for subsequent requests, taking precedence over environment and request-scoped values.
Failed requests don't update captured values. Cookies and captures are discarded when the run ends.

By default, the runner continues after a failed request and reports every result. Use `--fail-fast`
for dependent sequences so later requests aren't sent after a failure:

```bash
posting run api --env dev --fail-fast
```

Request files are loaded before any request is sent. Unreadable or malformed files, missing URLs,
and references to legacy Python scripts abort the run. Posting 3 doesn't execute Python scripts,
so `run` refuses them rather than silently skipping setup or assertions.

## Environments

Without `--env`, the runner loads `posting.env` and `posting.local.env` from the collection root.
It never uses the UI's remembered environment or implicitly loads environment files from the current
working directory or Posting's config directory.

`--env NAME` (`-e NAME`) looks up a named environment in the collection root and applies the usual
layers: `posting.env`, `posting.local.env`, `NAME.env`, `NAME.local.env`, skipping missing files.
An explicit file path is relative to the current working directory and loads just that file.
Repeat `--env` to layer names or files, with later values winning:

```bash
posting run api --env dev --env ./overrides.env
```

Use environment files to refer to secrets supplied by your shell, rather than putting credentials on the
command line or committing them:

```bash
# api/dev.env
BASE_URL="${DEV_API_URL}"
API_TOKEN="${DEV_API_TOKEN}"
```

Posting's `config.yaml` and `POSTING_*` shell settings still apply, including custom CA bundles,
client certificates and `use_host_environment`. With `POSTING_USE_HOST_ENVIRONMENT=true`, shell
variables are available directly to requests, beneath environment-file values. Invalid settings
abort the run instead of silently falling back to defaults. Settings aren't loaded from `.env` files.

## Results and exit codes

Text output gives one line per request and a summary. It doesn't print response headers or bodies:

```text
PASSED 01-health.posting.yaml: 200 OK (3.42 ms)
FAILED 02-users.posting.yaml: 503 Service Unavailable
1 passed, 1 failed, 0 skipped
```

| Exit code | Meaning |
|-----------|---------|
| `0` | Every request passed. |
| `1` | A request failed, the run was interrupted, or its files/configuration couldn't be loaded. |
| `2` | Invalid command-line arguments. |

HTTP 4xx/5xx responses, GraphQL responses with errors, non-OK gRPC statuses, transport errors,
timeouts and failed response captures count as failures. Redirects aren't failures; requests follow
them unless their `follow_redirects` option is disabled. Request timeouts also bound gRPC streams;
there's no interactive stream mode in a headless run.

These are status checks, not custom assertions about your API's response. A `200` with the wrong
application data still passes unless a declared capture fails. Inspect the JSON response to determine
whether it meets the task's expectations. `remote send` keeps its different exit behavior: receiving
any response is a success, even an error status.

`--json` prints one JSON document with `results`, `passed`, `failed` and `skipped`. Results are in
execution order, each with `file`, `name`, `outcome` (`passed` or `failed`), an `error` on failure,
and a `response` if one arrived. The response fields match `remote send --json`, including status,
headers, trailers, elapsed time, and a textual `body` or `bodyBase64` for non-UTF-8 data.
Setup errors also appear in the report's top-level `error` field. Argument errors print usage
diagnostics on stderr instead of a JSON report.

```bash
posting run api --env dev --json > results.json
jq '.results[] | {file, outcome, error, status: .response.status}' results.json
jq -r '.results[0].response.body' results.json
```

!!! warning "Reports may contain secrets"

    JSON reports contain response headers and bodies, which can include tokens, cookies or private
    data. Error messages can also include request URLs. Don't publish them as unrestricted artifacts
    or logs. Text output omits response bodies, but is not a secret-redaction mechanism.

Only run collections you trust, against an appropriate test environment: running a collection sends
every selected request, including requests that modify data.
