## Overview

Posting 3 imports OpenAPI specifications, Postman collections, and Bruno requests
or collections from the command line. Imported requests become ordinary
`.posting.yaml` files that you can edit and version control.

```sh
posting import api.yaml -o ./my-api
posting import collection.postman_collection.json -o ./my-api
posting import ./bruno-collection -o ./my-api
posting import request.bru -o ./my-api
```

Posting detects the format from the document or the `.bru` extension. You can
select it explicitly with `--type` (or `-t`):

```sh
posting import --type openapi api.json -o ./my-api
posting import --type postman collection.json -o ./my-api
posting import --type bruno ./bruno-collection -o ./my-api
```

Options work before or after the source path. Without `--output` (`-o`), Posting
creates a named folder inside the default collection directory. Find that
location with `posting locate collection`.

Existing files are never overwritten: filename collisions receive a numeric
suffix. Folder and request names are made safe for use as filenames. Conversion
errors produce a nonzero exit status. Warnings explain source features that
Posting cannot represent; review them before sending imported requests.

If an import contains collection variables, Posting writes a separate
`imported.env` file (also with a numeric suffix if that name is already taken).
The command prints the exact invocation to open the collection with its variables:

```sh
posting -c ./my-api -e ./my-api/imported.env
```

Imports read local files. They do not execute source scripts or send requests.

## Importing from curl

Paste a curl command into the URL bar, or choose **Import curl command…** from
the command palette to use the multiline dialog. **Import curl from clipboard**
is also available when your terminal allows applications to read its clipboard.

Importing curl replaces the current tab's request details. The request keeps its
name and saved file association, and becomes unsaved until you save it.

## Importing from OpenAPI

The importer accepts OpenAPI 3.0 and 3.1 specifications in JSON or YAML. It maps
HTTP operations to requests, including servers, parameters, request examples,
and supported authentication. Local references are resolved within the document.
External references are diagnosed rather than fetched from the network.

Swagger/OpenAPI 2.0 is not supported. Complex parameter serialization, multipart
uploads, and security schemes that Posting cannot send are reported as warnings.
See the [OpenAPI research and support notes](../research/import-openapi.md) for
selection rules and detailed limitations.

## Importing from Postman

The importer accepts Postman Collection v2 and v2.1 JSON. It preserves nested
folders and converts common URL, header, query, body, authentication, and variable
settings. Collection defaults are exported as environment variables; narrower
scopes are handled per request so values do not leak between sibling requests.

Scripts, dynamic variables, file uploads, and unsupported body or authentication
modes are diagnosed. See the [Postman research and support notes](../research/import-postman.md)
for the supported mappings and differences from Postman's runtime.

## Importing from Bruno

The importer accepts a single `.bru` HTTP request or a Bruno collection directory.
Directory imports include nested request files and supported collection/folder
inheritance. Common request bodies, headers, parameters, authentication, and
static variables are converted to Posting requests.

Bruno environments and executable behavior are not automatically activated.
Unsupported source features are reported as warnings. See the
[Bruno research and support notes](../research/import-bruno.md) for supported
Bru syntax, inheritance, and limitations.
