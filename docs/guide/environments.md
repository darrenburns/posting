## Overview

*Variables* let you write a value once and use it in many requests, and swap values between
environments such as your local machine, staging and production.

Use a variable by writing `$NAME` or `${NAME}` in a request:

```
https://${API_HOST}/users/$USER_ID
```

When the request is sent, each variable is replaced by its value. Variables are defined in
[environment files](#environment-files), and you can override them for a session on the
[variables screen](#the-variables-screen).

## Using variables

Variables can be used in:

- the URL, including [path parameter](./requests.md#path) values
- query parameter names and values
- header names and values
- authentication usernames, passwords and tokens
- the proxy URL
- the body, including form fields (unless you [turn this off](#literal-dollar-signs))

Type `$` in the URL, a value, the auth fields, the proxy URL or the body to see the variables
you can use, with their current values. Variables are highlighted as you type: green when they have
a value, red when they don't. (Names in the header, query and form tables are substituted too, but
aren't highlighted or completed.)

With the cursor on a variable in the URL bar, the line below the URL shows its value and where
the value comes from. Otherwise, if the URL uses variables, it shows the URL as it'll be sent, with every
variable filled in.

A variable name starts with a letter or underscore, followed by letters, digits and underscores.
If a variable used in the URL's host or path isn't defined, Posting won't send the request, and
tells you which variable is missing. Elsewhere, including the query string, an undefined variable
is sent as written.

### Literal dollar signs

Write `$$` to send a single literal `$`: `price$$` is sent as `price$`.

If a request body is full of dollar signs, as JSON schemas often are, turn off
**Substitute body variables** in the request's [Options](./requests.md#options) tab instead.
(A [GraphQL request](./requests.md#variables-in-graphql-requests) doesn't need this: only `${NAME}`
is replaced in its query.)
The body is then sent exactly as written, while variables in the URL, headers and auth still work.
The setting is saved with the request.

## Generated values

Use `${$name}` for a value evaluated afresh for each send. For example, an idempotency key:

```yaml
headers:
  - name: Idempotency-Key
    value: ${$uuid}
```

| Reference | Value |
|-----------|-------|
| `${$uuid}` | A cryptographically random UUID v4. |
| `${$timestamp}` | Current Unix time in seconds. |
| `${$timestampMs}` | Current Unix time in milliseconds. |
| `${$isoTimestamp}` | Current UTC time in ISO 8601 format, e.g. `2026-10-12T14:30:00.123Z`. |
| `${$randomInt}` | A cryptographically random integer from 0 to 2147483647, inclusive. |

Each generator has **one value per send**: `${$uuid}` in a header and the body uses the same UUID.
Sending again evaluates all generators again (timestamps may stay the same within their clock
resolution). An open gRPC stream keeps the values from when the call started for subsequent messages.
Generators work wherever ordinary request variables work, including GraphQL and gRPC payloads.

Type `$` to find generators in completion. The editor shows descriptions, not values that might be
mistaken for the next send's values. Saving keeps the references; exporting a curl/grpcurl command
with **Values** evaluates them once for that export. Re-running the exported command reuses those
literal values. gRPC schema discovery also evaluates generators when it prepares its own request.

Only the braced `${$name}` form generates values. Write `$${$uuid}` to send the literal `${$uuid}`,
or turn off body substitution to leave the entire body unchanged.

## Response captures

Add a `captures` mapping to a request's `.posting.yaml` file to save values from its JSON response
as session variables:

```yaml
name: Start session
method: POST
url: https://${API_HOST}/sessions
captures:
  SESSION_ID: data.id
  API_TOKEN: data.token
```

For a response of `{"data":{"id":"session-123","token":"abc"}}`, later requests can use
`${SESSION_ID}` and `${API_TOKEN}` in their URL, headers, body or authentication fields.
Captures are edited in the file and preserved when saving in Posting.

Paths use [JSONPath](https://www.rfc-editor.org/rfc/rfc9535.html) and start at the response body's
root. You can omit the root prefix: Posting adds `$` before `[` or `$.` before a member name.
Expressions starting with `$` are used as written. Both forms use the same JSONPath parser;
there's no separate shorthand language or fallback for invalid expressions.

| Path | Selects |
|------|---------|
| `data.id` or `$.data.id` | A nested object field. |
| `data.users[1].id` | The second user's ID (indexes start at zero). |
| `[0].id` | A field in the first item of a root array. |
| `data["session.id"]` or `data['session.id']` | A key containing a dot. |
| `data.users[-1].id` | The last user's ID. |
| `data.users[?(@.role == "admin")].id` | The admin user's ID, provided exactly one matches. |
| `data.users[1:2].id` | The second user's ID, using a slice. |
| `data.users[*].id` | Each user's ID; succeeds only if exactly one matches. |
| `$..token` | A token at any depth, provided exactly one matches. |
| `$` | The entire JSON body. |

Each capture must select **exactly one value**. Selecting `data.users` captures the entire array
as JSON; selecting `data.users[*]` produces a match for each element, and fails if there are zero
or multiple elements. Posting never silently picks the first match.

Capture names follow the ordinary [variable naming rules](#using-variables). Strings are stored
without quotes; numbers, booleans, `null`, arrays and objects are stored as JSON. Captured numbers
keep their original precision. Numeric comparisons **inside filters** use 64-bit floating point,
which can round large integers (beyond ±9007199254740991) and high-precision decimals. Capture
such IDs by path, filter on another field, or compare IDs represented as JSON strings instead.

Captures run after a complete response, regardless of its HTTP status, and also work on GraphQL
and decoded gRPC JSON responses. They don't run on intermediate stream updates, when loading history,
or when a request fails without a response. A cancelled or replaced request cannot publish late captures.
Stopping a gRPC stream that has a response captures from its final retained body.

All captures in a request must succeed. If the body isn't valid JSON or any path matches zero or
multiple values, Posting shows a warning, keeps the response available, and leaves
**all existing session values unchanged**.
An explicit JSON `null` is a value, not a missing field.

Captured values override earlier session values and environment values. They appear on the
[variables screen](#the-variables-screen), where you can edit or remove them, and are forgotten when
Posting exits. As with other session values, switching environments keeps them. Captured strings
are literal: dollar signs inside them aren't expanded again.

## Environment files

Variables are defined in `.env` files:

```bash
# file: staging.env
API_HOST=staging.example.com
API_VERSION="v2"
BASE_URL="https://${API_HOST}/${API_VERSION}"
```

Each line is `NAME=value`. The format is the usual dotenv one:

- Lines starting with `#` are comments, and blank lines are ignored.
- Values can be unquoted, `"double quoted"` or `'single quoted'`. Quoted values can span several lines.
- Double-quoted values understand escapes such as `\n` and `\t`. Single-quoted values aren't expanded,
  and only `\'` and `\\` are unescaped.
- An `export` in front of a line is ignored, so you can `source` the same file in your shell.
- An unquoted value can be followed by a comment, with a space before the `#`.

### Referring to other variables

A value can use the value of a variable defined before it, with `${NAME}`, or `${NAME:-default}` to
fall back to a default when `NAME` isn't set or is empty. This works across [layers](#layered-environments)
too, so a named environment can build on the base. Inside environment files, only the `${...}` form is
expanded; a bare `$NAME` is kept as it is, and single-quoted values are never expanded.

If a name isn't defined in the file or an earlier layer, it's looked up in your shell's environment,
which makes it easy to pull in secrets you already have in your shell:

```bash
API_TOKEN="${STAGING_TOKEN}"
```

## Layered environments

An environment is built from layers: a *base* that every environment shares, and the environment
itself on top. Name the files like this, side by side in a directory:

| File                 | Layer                                                 |
|----------------------|-------------------------------------------------------|
| `posting.env`        | The base, shared by every environment                 |
| `posting.local.env`  | Your own base values, kept out of version control     |
| `<name>.env`         | The environment called `<name>`, e.g. `staging.env`   |
| `<name>.local.env`   | Your own values for `<name>`, e.g. secrets            |

Layers apply in that order, so a variable in a later layer overrides the same variable in an earlier
one, and can build on it:

```bash
# file: posting.env
API_PATH="/api/v1"
BASE_URL="https://example.com"

# file: staging.env
BASE_URL="https://staging.example.com"
API="${BASE_URL}${API_PATH}"   # https://staging.example.com/api/v1

# file: staging.local.env
API_TOKEN="my-staging-token"
```

Keep your `.local.env` files out of version control, so secrets stay on your machine:

```bash
# file: .gitignore
*.local.env
```

### Starting in an environment

Start Posting in an environment by name with `--env` (or `-e`):

```bash
posting --env staging
```

This loads `posting.env`, `posting.local.env`, `staging.env` and `staging.local.env`, skipping any that
don't exist. Posting looks for `staging.env` (or `staging.local.env`) in the directory you start Posting
from, then the collection's directory, then Posting's config directory (`~/.config/posting`), and takes all
the layers from the first of these that has it. The base layers on their own are the environment called
`posting`, after its file: use `--env posting`.

`--env` also accepts the path of a file, which is loaded on its own, without the base. Repeat `--env` to
layer names and files in any order, later ones on top. A file given again is applied again, so the last
one given wins. Named environments also reapply their own layers; shared base layers are loaded only once:

```bash
posting --env shared.env --env dev.env
posting --env staging --env ~/overrides.env
```

When you don't pass `--env`, Posting starts in the environment you last chose for this collection.
If you haven't chosen one, it uses the base environment (`posting.env` and `posting.local.env`) in the
directory you started Posting from, if there is one.

## Switching environments

The header at the top of the screen shows the active environment. To switch, click its name, or
press ++ctrl+p++ and choose **Switch environment…**.

The switcher lists every environment Posting can find in the directories above, with how many
variables each has and the files it's made from. Choose **No environment** to use only
[session values](#the-variables-screen).

Posting remembers the environment you choose for each collection, and starts in it next time.
A named environment is rebuilt from its folder when Posting starts, so a `.local.env` file you've added
since is included. Passing `--env` starts in that environment instead, without changing what's
remembered.

To use an environment file from anywhere else, choose **Load environment file…** in the command
palette and type its path. Separate several paths with commas to layer them. An environment loaded
this way is also remembered for the collection.

Switching environments keeps any session values you've set.

### Editing environment files

You don't need to restart Posting after changing an environment file. Posting checks the active
environment's files every second, and reloads them when they change. To turn this off, set
`watch_env_files: false` in your [configuration](./configuration.md).

## The variables screen

Press ++ctrl+shift+v++, or choose **Variables** in the command palette, to see every variable
available to your requests, its value, and where the value comes from.

<figure class="screen">
--8<-- "variables.html"
<figcaption>The variables screen, with secrets masked.</figcaption>
</figure>

From here you can override a variable for the rest of the session, or add a new one. These
*session values* take priority over the environment, and aren't written to any file: they're
forgotten when you quit.

| Key | Action |
|-----|--------|
| `/` | Filter the list by name or value |
| ++enter++ | Change the highlighted variable's value (++enter++ again to save, ++escape++ to cancel) |
| ++a++ | Add a variable |
| ++d++ | Remove a session value, going back to the environment's value |
| ++ctrl+r++ | Show or hide secret values |
| ++escape++ | Close the screen |

## Where a value comes from

Variables can come from three places. From lowest to highest priority:

1. **Your shell's environment**, if `use_host_environment` is on (see below)
2. **The environment's files**, in layer order
3. **Session values**, set on the [variables screen](#the-variables-screen) or by [response captures](#response-captures)

The variables screen, and the preview under the URL bar, show where each value comes from and what
it overrides, for example `staging.local.env over posting.env`, or `session over staging.env`.

### Using your shell's environment

By default, requests can only use variables from environment files and session values. To let requests
use any environment variable from the shell you started Posting in, turn on `use_host_environment` in your
[configuration](./configuration.md):

```yaml
use_host_environment: true
```

Variables in your environment files still override those from the shell.

Posting's own settings, such as `POSTING_THEME`, are read from the shell, not from environment files.

## Secrets

Posting treats a variable as secret when its name contains `secret`, `password`, `passwd`, `token`,
`api_key` or `apikey` (in any case). Secret values are masked:

- on the variables screen, until you press ++ctrl+r++
- in the list of suggestions when you type `$`
- in the preview under the URL bar, while `url_bar.hide_secrets_in_value_preview` is on (the default)

The preview of the complete URL isn't masked, so avoid putting secrets in URLs where you can; send them in
headers or [auth](./requests.md#auth) instead.
