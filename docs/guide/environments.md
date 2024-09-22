## Overview

You can use *variables* in input fields and text areas using the `${VARIABLE_NAME}` or `$VARIABLE_NAME` syntax.
These variables will be substituted into outgoing requests.

<p align="center">
  <img src="https://github.com/darrenburns/posting/assets/5740731/24b64f58-747b-409e-9672-e354eb8994d8" alt="url-bar-environments-short">
</p>

## Loading variables

Variables are stored in `.env` files, and loaded using the `--env` option.

Here's what a `.env` file might look like:

```bash
# file: dev.env
API_KEY="dev-api-key"
ENV_NAME="dev"
BASE_URL="https://${ENV_NAME}.example.com"
```

To make these variables available in the UI, you can load them using the `--env` option:

```bash
posting --env dev.env
```

You can load multiple `.env` files by specifying the `--env` option multiple times:

```bash
posting --env dev.env --env shared.env
```

This allows you to build up a set of variables which are common to all environments, and then override them for specific environments.

## Using environment variables

By default, Posting will only use variables defined in `.env` files that have been explicitly loaded using the `--env` option.

If you want to permit using environment variables that exist on the host machine (i.e. those which are not defined in any `.env` files), you must set the `use_host_environment` config option to `true` (or set the environment variable `POSTING_USE_HOST_ENVIRONMENT=true`).

## Practical example

Imagine you're testing an API which exists in both `dev` and `prod` environments.

The `dev` and `prod` environments share some common variables, but differ in many ways too.
We can model this by having a single `shared.env` file which contains variables which are shared between environments, and then a `dev.env` and `prod.env` file which contain environment specific variables.

```bash
# file: shared.env
API_PATH="/api/v1"
ENV_NAME="shared"

# file: dev.env
API_KEY="dev-api-key"
ENV_NAME="dev"
BASE_URL="https://${ENV_NAME}.example.com"

# file: prod.env
API_KEY="prod-api-key"
ENV_NAME="prod"
BASE_URL="https://${ENV_NAME}.example.com"
```

When working in the `dev` environment, you can then load all of the shared variables and all of the development environment specific variables using the `--env` option:

```bash
posting --env shared.env --env dev.env
```

This will load all of the shared variables from `shared.env`, and then load the variables from `dev.env`. Since `ENV_NAME` appears in both files, the value from the `dev.env` file will be used since that was the last one specified.

Note that you do *not* need to restart to load changes made to these files,
so you can open and edit your env files in an editor of your choice alongside Posting.

### Environment specific config

Since all Posting configuration options can also be specified as environment variables, we can also put environment specific config inside `.env` files. There's a dedicated "Configuration" section in this document which covers this in more detail.

For example, if you wanted to use a light theme in the prod environment (as a subtle reminder that you're in production!), you could set the environment variable `POSTING_THEME=solarized-light` inside the `prod.env` file.

Note that configuration files take precedence over environment variables, so if you set a value in both a `.env` file and a `config.yaml`, the value from the `config.yaml` file will be used.
