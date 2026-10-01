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
3. **Session values**, set on the [variables screen](#the-variables-screen)

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
