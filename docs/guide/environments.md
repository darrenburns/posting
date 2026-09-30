## Overview

You can use *variables* in input fields and text areas using the `${VARIABLE_NAME}` or `$VARIABLE_NAME` syntax.
These variables will be substituted into outgoing requests.

<p align="center">
  <img src="https://github.com/darrenburns/posting/assets/5740731/24b64f58-747b-409e-9672-e354eb8994d8" alt="url-bar-environments-short">
</p>

## Environment files

Variables are stored in `.env` files. Here's what one might look like:

```bash
# file: dev.env
API_KEY="dev-api-key"
ENV_NAME="dev"
BASE_URL="https://${ENV_NAME}.example.com"
```

A value can refer to a variable defined above it with `${NAME}`, or
`${NAME:-default}` to fall back to a default when it isn't set.

## Layered environments

An environment is built from layers: a *base* that every environment shares,
and the environment itself on top. Name the files like this, side by side in
a folder:

| File                 | Layer                                                 |
|----------------------|-------------------------------------------------------|
| `posting.env`        | The base, shared by every environment                 |
| `posting.local.env`  | Your own base values, kept out of version control     |
| `<name>.env`         | The environment called `<name>`, e.g. `staging.env`   |
| `<name>.local.env`   | Your own values for `<name>`, e.g. secrets            |

Layers apply in that order, so a variable in a later layer overrides the same
variable in an earlier one. A later layer can also build on an earlier one:

```bash
# file: posting.env
API_PATH="/api/v1"
BASE_URL="https://example.com"

# file: staging.env
BASE_URL="https://staging.example.com"
API="${BASE_URL}${API_PATH}"   # https://staging.example.com/api/v1

# file: staging.local.env
API_KEY="my-staging-key"
```

Start Posting in an environment by name:

```bash
posting --env staging
```

This loads `posting.env`, `posting.local.env`, `staging.env` and
`staging.local.env`, skipping any that don't exist. Posting looks for the
files in the working directory, then the collection directory, then
Posting's config directory.

Keep your `.local.env` files out of version control so secrets stay on your
machine:

```bash
# file: .gitignore
*.local.env
```

### Layering files yourself

`--env` also accepts files. A file is loaded exactly as given, without the
base, and you can repeat `--env` to layer names and files in any order:

```bash
posting --env shared.env --env dev.env
posting --env staging --env ~/overrides.env
```

## Choosing an environment

Press ++ctrl+p++ and choose **Switch environment…**, or click the environment
name in the header. The switcher lists each environment it finds, with its
layers and how many variables it has, and **No environment** to use session
variables alone.

Posting remembers the environment you switch to in each collection, and
starts in it next time. Passing `--env` starts in that environment instead.
Without either, Posting uses the base environment (`posting.env`) in the
working directory if there is one.

You don't need to restart to load changes to these files: open and edit them
in an editor of your choice alongside Posting.

## Where a value comes from

Variables come from several places. From lowest to highest priority:

1. Variables from the host machine, if `use_host_environment` is on
2. The environment's files, in layer order
3. Session values, set in the **Variables** view

The **Variables** view (++ctrl+p++ → **Variables**) shows each variable with
its source, and the layer it overrides, for example
`staging.local.env over posting.env`. The preview under the URL bar shows the
same for the variable under the cursor.

## Using environment variables

By default, Posting will only use variables defined in `.env` files.

If you want to permit using environment variables that exist on the host machine (i.e. those which are not defined in any `.env` files), you must set the `use_host_environment` config option to `true` (or set the environment variable `POSTING_USE_HOST_ENVIRONMENT=true`).
Values in your environment files override host variables of the same name.

Posting's own settings (`POSTING_THEME` and so on) are read from the host
environment, not from `.env` files, and take precedence over `config.yaml`.

### Sending literal dollar signs in a body

For JSON schemas, query languages, or other payloads that use literal dollar
signs, turn off **Substitute body variables** in the request's **Options** tab.
This setting is saved with the request as `options.substitute_body_variables: false`.
Raw bodies and form field names/values are sent unchanged, including `$name`,
`${name}`, and `$$`. URL, header, and authentication variables continue to work.
Substitution is enabled by default for existing and new requests.
