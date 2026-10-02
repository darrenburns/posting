## Overview

Posting supports importing from external sources.

## Importing from curl

!!! example "This feature is experimental."

You can import a curl command by pasting it into the URL bar.

This will fill out the request details in the UI based on the curl command you pasted, overwriting any existing values.

## Importing from OpenAPI

!!! example "This feature is experimental."

Posting can convert OpenAPI 3.x specs into collections.

To import an OpenAPI Specification, use the `posting import path/to/openapi.yaml` command.

You can optionally supply an output directory.

If no output directory is supplied, the default collection directory will be used.

Posting will attempt to build a file structure in the collection that aligns with the URL structure of the imported API.

## Importing from Postman

!!! example "This feature is experimental."

Collections can be imported from Postman.

To import a Postman collection, use the `posting import --type postman path/to/postman_collection.json` command.

You can optionally supply an output directory with the `-o` option.
If no output directory is supplied, the default collection directory will be used (check where this is using `posting locate collection`).

Variables will also be imported from the Postman collection and placed in a `.env` file inside the collection directory.


### Importing a Postman environment

Export the environment as JSON, then run:

```bash
posting import --type postman-env path/to/environment.json -o environments
```

Posting creates the output directory if needed and writes `<environment name>.env` inside it.
Without `-o`, the file is written to the current directory.
Load it with `posting --env "environments/<environment name>.env"`.

Only enabled variables are imported. Variable names use the same normalization as imported Postman collections, so `baseUrl` becomes `BASE_URL`.
Empty and null values become empty strings.

The import fails before writing if names are unsafe, variable names collide after normalization, or dotenv cannot preserve the values.
This includes values changed by `${...}` interpolation and some quoted values ending in a backslash.
