## Overview

Posting can import requests from OpenAPI specifications, Postman collections, and Bruno requests
or collections, using the `posting import` command. Imported requests become ordinary
`.posting.yaml` files that you can edit and keep in version control.

```bash
posting import api.yaml -o ./my-api
posting import collection.postman_collection.json -o ./my-api
posting import ./bruno-collection -o ./my-api
posting import request.bru -o ./my-api
posting import collection.json staging.postman_environment.json -o ./my-api
```

To import a `curl` command, paste it into the URL bar inside Posting instead.
See [Importing curl commands](./requests.md#importing-curl-commands).

## Running an import

Posting works out the format from the file itself (or from the `.bru` extension, or the fact that
the source is a directory). To choose the format yourself, use `--type` (or `-t`):

```bash
posting import --type openapi api.json -o ./my-api
posting import --type postman collection.json -o ./my-api
posting import --type bruno ./bruno-collection -o ./my-api
```

`--output` (or `-o`) is the directory to write the requests to. Options can come before or after the
source. Without `--output`, Posting creates a folder named after the collection inside the
[default collection](./collections.md#the-default-collection), which you can find with
`posting locate collection`.

When it's finished, Posting tells you how many requests it imported, and the command to open them:

```text
$ posting import petstore.yaml -o ./petstore
warning: GET /pets/{id}: authentication credentials are empty placeholders; configure imported variables before sending
warning: PATCH /pets/{id} body: generated a body/parameter scaffold from schema; review values and constraints
Imported 3 openapi request(s) into "/home/you/petstore".
Environments: posting.env (base)
Open with: posting -c /home/you/petstore --env posting
```

A few things are worth knowing:

- **Existing files are never overwritten.** If a file already exists, the new one gets a number
  added to its name, so importing the same source twice gives you two copies.
- **Variables go in environment files.** Variables the source defines, such as a base URL or
  credentials, go in `posting.env`. Each environment goes in its own file on top of it. See
  [Environments](#environments).
- **Warnings tell you what couldn't be imported.** Anything Posting can't represent, such as a
  multipart body or an authentication scheme it doesn't support, is listed as a warning. Review
  those requests before you send them.
- **Imports are read-only.** Posting reads local files only: it doesn't fetch anything over the
  network, run scripts from the source, or send any requests.

Files larger than 32 MiB can't be imported.

## Environments

Posting writes imported variables as [layered environments](./environments.md#layered-environments).
The collection's own variables go in `posting.env`, the base that every environment shares. Each
environment goes in `<name>.env`, which Posting layers on top of the base. Select one with `--env`:

```text
$ posting import collection.json staging.postman_environment.json production.postman_environment.json -o ./my-api
Imported 12 postman request(s) into "/home/you/my-api".
Environments: posting.env (base), staging.env, production.env
Open with: posting -c /home/you/my-api --env staging
```

Variables keep their references to other variables. If the collection sets
`BASE_URL` to `{{HOST}}/api` and the staging environment sets `HOST`, then `BASE_URL` uses the staging
`HOST` when you select staging. To make that work, an environment file repeats the base variables
that depend on a variable it sets.

To add environments to a collection you've already imported, give only the environment files and the
collection's directory:

```bash
posting import staging.postman_environment.json -o ./my-api
```

Importing a collection into a directory that already has a `posting.env` writes the new base to
`posting-2.env`, with a warning. Posting loads that file as an environment called `posting-2`, not as
the base, so merge it into `posting.env` by hand.

Environments named `posting`, or ending in `.local`, would clash with the layering convention, so
they're renamed. An environment called `posting` is written to `posting-environment.env`, and one called
`staging.local` to `staging-local.env`.

Secrets that the source doesn't store on disk aren't imported. Posting names them in a warning. Put
their values in the environment's `.local.env` file, such as `staging.local.env`, and keep that file
out of version control.

## OpenAPI

Posting imports OpenAPI 3.0 and 3.1 specifications, in JSON or YAML. Swagger (OpenAPI 2.0) isn't
supported.

- Each operation becomes a request, named after its summary (or its `operationId`), in a folder
  named after its first tag.
- The URL comes from the first server. If the specification has no server, or only a relative one,
  the URL starts with `${BASE_URL}`, and `BASE_URL` is added to `posting.env` for you to fill in.
- Path parameters such as `{id}` become Posting's `:id`.
- Parameter values and request bodies are filled in from the specification's examples and defaults.
  Optional parameters without a value start out disabled.
- JSON and URL-encoded form bodies are imported. Imported bodies have
  [body variable substitution](./environments.md#literal-dollar-signs) turned off, so examples
  containing `$` are sent exactly as written.
- Basic, Digest, Bearer and API key security schemes are imported, with the credentials as variables
  in `posting.env`. OAuth 2, OpenID Connect and mutual TLS need to be set up by hand.
- Only references within the document are followed.

## Postman

Posting imports Postman Collection v2.0 and v2.1 JSON files.

- Folders become folders in the collection, and each request becomes a request.
- URLs, headers and query parameters (including disabled ones), and enabled path variables, are
  imported.
- Raw and URL-encoded bodies are imported. GraphQL bodies become
  [GraphQL requests](./requests.md#graphql-requests). Multipart form data and file bodies are
  skipped with a warning.
- Basic, Digest, Bearer and API key authentication are imported, including authentication inherited
  from folders and the collection.
- Postman's `{{variable}}` references become Posting's `${variable}`. Collection variables are written
  to `posting.env`. Folder and request variables are filled into the requests that use them.
- Postman environment exports given after the collection, or on their own with `--output`, become
  [environments](#environments). Disabled values are skipped. Globals exports aren't supported.
- Pre-request and test scripts aren't imported.
- gRPC requests can't be imported from Postman. Postman doesn't export collections that contain
  them as v2.1 JSON: the v2.1 format has no way to describe a gRPC request.

## Bruno

Posting imports a single `.bru` request file, or a whole Bruno collection directory (one containing
`bruno.json`).

- A collection's folders and requests are imported with the same layout, along with the headers,
  query parameters, variables and authentication that `collection.bru` and `folder.bru` files pass
  down to their requests.
- JSON, text and XML bodies, and URL-encoded forms, are imported. GraphQL requests become
  [GraphQL requests](./requests.md#graphql-requests), with their query and variables. Multipart and
  file bodies are skipped with a warning.
- gRPC requests become [gRPC requests](./requests.md#grpc-requests), with their server address,
  method, metadata and messages. A client-streaming or bidirectional request's messages become one
  array. A request's `protoPath`, and the enabled import paths in `bruno.json`, become its
  [proto files](./requests.md#grpc-request-files). Proto files aren't copied, so Posting warns you
  to put them at the same path relative to the imported collection. A request without a
  `protoPath` uses server reflection, as it does in Bruno.
- Basic, Digest, Bearer and API key authentication are imported. A gRPC request keeps Basic, Bearer
  and header API keys only, and warns about the rest.
- Collection variables from `collection.bru` are written to `posting.env`, so environments can
  override them. Folder and request variables are filled into each request, because Bruno ranks them
  above environments.
- Each file in the collection's `environments` directory becomes an [environment](#environments).
  Secret variables have no values on disk, so Posting names them in a warning instead.
- Scripts, tests and assertions aren't imported.

## Detailed support notes

For every mapping and limitation, including the edge cases, see the support notes for
[OpenAPI](../research/import-openapi.md), [Postman](../research/import-postman.md) and
[Bruno](../research/import-bruno.md).

## Coming from Posting 2

Posting 2's `posting import --type postman collection.json` still works. Posting 3 detects the format
by itself, adds Bruno support and environments, and writes collection variables to `posting.env`
rather than a file named after the collection.
