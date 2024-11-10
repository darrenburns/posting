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
