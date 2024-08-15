## Overview

Posting currently supports importing from OpenAPI specs.

Support for other API formats will be added in future updates.

## Importing from OpenAPI

!!! example "This feature is experimental."

Posting can convert OpenAPI 3.x specs into collections.

To import an OpenAPI Specification, use the `posting import path/to/openapi.yaml` command.

You can optionally supply an output directory.

If no output directory is supplied, the default collection directory will be used.

Posting will attempt to build a file structure in the collection that aligns with the URL structure of the imported API.
