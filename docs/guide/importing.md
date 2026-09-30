## Overview

Posting can import requests from OpenAPI specifications, Postman collections, and Bruno requests
or collections, using the `posting import` command. Imported requests become ordinary
`.posting.yaml` files that you can edit and keep in version control.

```bash
posting import api.yaml -o ./my-api
posting import collection.postman_collection.json -o ./my-api
posting import ./bruno-collection -o ./my-api
posting import request.bru -o ./my-api
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
Collection variables: "/home/you/petstore/imported.env"
Open with: posting -c /home/you/petstore -e /home/you/petstore/imported.env
```

A few things are worth knowing:

- **Existing files are never overwritten.** If a file already exists, the new one gets a number
  added to its name, so importing the same source twice gives you two copies.
- **Variables go in an environment file.** If the source defines variables, such as a base URL or
  credentials, Posting writes them to `imported.env` in the output directory. Pass it with `-e`, or
  pick **imported** in the [environment switcher](./environments.md#switching-environments).
- **Warnings tell you what couldn't be imported.** Anything Posting can't represent, such as a
  multipart body or an authentication scheme it doesn't support, is listed as a warning. Review
  those requests before you send them.
- **Imports are read-only.** Posting reads local files only: it doesn't fetch anything over the
  network, run scripts from the source, or send any requests.

Files larger than 32 MiB can't be imported.

## OpenAPI

Posting imports OpenAPI 3.0 and 3.1 specifications, in JSON or YAML. Swagger (OpenAPI 2.0) isn't
supported.

- Each operation becomes a request, named after its summary (or its `operationId`), in a folder
  named after its first tag.
- The URL comes from the first server. If the specification has no server, or only a relative one,
  the URL starts with `${BASE_URL}`, and `BASE_URL` is added to `imported.env` for you to fill in.
- Path parameters such as `{id}` become Posting's `:id`.
- Parameter values and request bodies are filled in from the specification's examples and defaults.
  Optional parameters without a value start out disabled.
- JSON and URL-encoded form bodies are imported. Imported bodies have
  [body variable substitution](./environments.md#literal-dollar-signs) turned off, so examples
  containing `$` are sent exactly as written.
- Basic, Digest, Bearer and API key security schemes are imported, with the credentials as variables
  in `imported.env`. OAuth 2, OpenID Connect and mutual TLS need to be set up by hand.
- Only references within the document are followed.

## Postman

Posting imports Postman Collection v2.0 and v2.1 JSON files.

- Folders become folders in the collection, and each request becomes a request.
- URLs, headers and query parameters (including disabled ones), and enabled path variables, are
  imported.
- Raw and URL-encoded bodies are imported. Multipart form data, file and GraphQL bodies are skipped
  with a warning.
- Basic, Digest, Bearer and API key authentication are imported, including authentication inherited
  from folders and the collection.
- Postman's `{{variable}}` references become Posting's `${variable}`. Collection variables are written
  to `imported.env`; folder and request variables are filled into the requests that use them.
- Pre-request and test scripts aren't imported. Postman environment files aren't imported either:
  recreate them as [environment files](./environments.md#environment-files).

## Bruno

Posting imports a single `.bru` request file, or a whole Bruno collection directory (one containing
`bruno.json`).

- A collection's folders and requests are imported with the same layout, along with the headers,
  query parameters, variables and authentication that `collection.bru` and `folder.bru` files pass
  down to their requests.
- JSON, text and XML bodies, and URL-encoded forms, are imported. Multipart, file and GraphQL bodies
  are skipped with a warning.
- Basic, Digest, Bearer and API key authentication are imported.
- Bruno variables are filled into each request. References that can't be resolved when importing
  become Posting variables.
- Bruno's `environments` directory, scripts, tests and assertions aren't imported.

## Detailed support notes

For every mapping and limitation, including the edge cases, see the support notes for
[OpenAPI](../research/import-openapi.md), [Postman](../research/import-postman.md) and
[Bruno](../research/import-bruno.md).

## Coming from Posting 2

Posting 2's `posting import --type postman collection.json` still works. Posting 3 detects the format
by itself, adds Bruno support, and writes collection variables to `imported.env` rather than a file
named after the collection.
