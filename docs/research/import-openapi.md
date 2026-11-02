# OpenAPI importer research and design

Research date: 2026-09-30. Implementation: `internal/importing/openapi`.

OpenAPI 3.0 specifies operation overrides for path parameters, identified by `(name, in)`, and server overrides at operation/path/root scope. The importer follows those rules and treats explicit empty server lists as the default relative server. Media-type and parameter examples override schema examples. Local pointer traversal preserves `~0`, `~1`, URI fragment percent-encoding, and literal plus signs. [OpenAPI 3.0.4](https://spec.openapis.org/oas/v3.0.4.html)

Security requirement entries are alternatives; schemes within one entry must all apply. An empty requirement permits anonymous access, and an empty operation security list removes inherited requirements. The converter chooses an anonymous alternative when available, otherwise the first fully representable alternative. Scalar query/cookie parameters use form serialization, while scalar path/header parameters use simple serialization. Complex encodings have materially different delimiter behavior and cannot all be represented by Posting's editable string fields. [OpenAPI 3.1.1](https://spec.openapis.org/oas/v3.1.1.html)

The upstream OAI passing-schema Petstore example was imported during development (3 operations). Swagger's current Petstore document also imported (19 operations); its alternative JSON/XML/form bodies exercised media selection and its OAuth security exercised diagnostics. Network-independent tests use an original compact fixture; `POSTING_OPENAPI_OFFICIAL_FIXTURES` enables rechecking downloaded upstream documents. [OAI passing fixture](https://github.com/OAI/OpenAPI-Specification/blob/main/_archive_/schemas/v3.0/pass/petstore.yaml), [Swagger Petstore](https://github.com/swagger-api/swagger-petstore/blob/master/src/main/resources/openapi.yaml)

A report against Swagger Parser documents failures around `$id`, `$dynamicRef`, and dereferencing. Its reproduction examples informed explicit diagnostics for unsupported dynamic schema scope; this importer does not attempt full JSON Schema evaluation. The report is evidence of a practical interoperability failure, not a replacement for the OpenAPI specification's reference rules. [Swagger Parser issue 2331](https://github.com/swagger-api/swagger-parser/issues/2331)

## Conversion policy

`Parse([]byte) (importing.Result, error)` reads a single JSON or YAML document without filesystem writes or network fetches. It accepts 3.0.x and 3.1.x, rejecting Swagger 2 and newer minor versions. Sorted paths and a fixed method order make requests and diagnostics reproducible. Operation summary, operation ID, then method/path determine names; first tags suggest folders. The shared writer owns safe filenames and collisions.

The first server is selected, server variables are expanded from required string defaults, and relative servers use an empty `BASE_URL` variable with instructions to configure the API origin. Defaults remain local to the relevant operation. Segment-sized path templates become Posting path parameters. Embedded templates are retained with manual-edit warnings. Examples containing dollars are escaped for Posting substitution; bodies disable substitution so JSON and raw examples remain literal.

Query scalars, exploded arrays, flat exploded objects, and flat deepObject values are represented. Scalar headers and cookies are represented. Missing optional scalar parameters start disabled; required parameters remain editable. JSON bodies use explicit examples/defaults or a bounded scaffold of properties carrying examples/defaults, excluding readOnly fields. Raw string examples and URL-encoded forms are supported. Basic, Digest, Bearer and API-key security produce stable, collision-resistant, empty credential variables.

## Limits and diagnostics

External references are diagnosed and never fetched; a required external or broken local reference fails the conversion. Local reference chains and document nesting are bounded. Recursive schema expansion has depth and work budgets. Reference siblings, anchors, dynamic refs, resource scopes, schema composition without examples, unsupported serializers and allowReserved, multipart, callbacks, and unsupported security are diagnosed. TRACE operations are skipped because Posting's persisted method model does not support them. Webhook/response definitions are not converted into outgoing requests. Multiple servers, body media types, named examples and security alternatives produce selection warnings.

Schema scaffolds are editable starting points, not generated instances guaranteed to satisfy arbitrary JSON Schema constraints. This is an importer, not a complete specification validator. Scope-relative server URLs need an explicit origin; server URLs containing query/fragment components or lacking an explicit scheme for a network-path reference are rejected instead of guessed.

## Verification

Tests exercise server/security inheritance, explicit empty overrides, AND/OR security, parameter replacement, dates, boolean schemas, large exact integers, escaped JSON pointers, cycles, external references, malformed types, deterministic output, Posting serialization roundtrips, and actual HTTP requests against a loopback server. A Go fuzz target checks parser stability, deterministic outputs and readable generated request files. Initial 30-second fuzzing completed 3,538 executions without a failure; further adversarial probing follows integration.
