# Postman collection importer

Implemented for Posting 3 in `internal/importing/postman`, researched September 2026.

## Sources and semantic decisions

- The official [v2.1 JSON schema](https://schema.postman.com/collection/json/v2.1.0/draft-07/collection.json) describes nested item groups, URL strings/objects, disabled entries, descriptions, request-string shorthand, and request-body modes. [v2.0](https://schema.postman.com/collection/json/v2.0.0/draft-07/collection.json) uses object auth attributes; v2.1 uses key/value arrays. Both are accepted. Inputs require `info` and an `item` array and at least one request. A provided schema must identify v2.0.0 or v2.1.0.
- [Postman's auth documentation](https://learning.postman.com/docs/use/send-requests/authorization/specifying-authorization-details/) describes parent inheritance. Collection/folder auth is carried down the item tree; absent or null auth inherits, while `noauth` stops inheritance. Supported helpers are Basic, Digest, Bearer and API key (header or query). Unsupported auth produces a request-specific warning.
- The [upstream URL implementation](https://github.com/postmanlabs/postman-collection/blob/develop/lib/collection/url.js) builds URLs from components, excluding disabled query fields. Components take precedence over an inconsistent `raw`. For compatibility with third-party exports, a raw-only URL object is also accepted. An explicit query array replaces the raw query, even when empty.
- [Upstream URL tests](https://github.com/postmanlabs/postman-collection/blob/develop/test/unit/url.test.js) cover string/array hosts and paths, path variables, and suffixes such as `:id.json`. The importer preserves suffixes through an equivalent Posting path parameter. [Query tests](https://github.com/postmanlabs/postman-collection/blob/develop/test/unit/query-param.test.js) show that null/missing values omit `=` and that existing percent encoding is retained. Posting stores decoded editable parameter values and re-encodes on send. Repeated parameters and disabled entries are retained. A warning explains that bare parameters become empty values with `=` because the Posting parameter model cannot distinguish them.
- [Variable documentation](https://learning.postman.com/docs/use/send-requests/variables/variables) establishes double-brace references and precedence of narrower scopes. Collection defaults become environment variables, with folder/request overrides materialized per request. Sibling requests therefore retain their own values. Posting substitution is one-pass: references between exported collection defaults are flattened at import, with warnings for unresolved references. Local overrides can still reference collection variables through the resulting Posting placeholders.

## Design

`Parse([]byte) (importing.Result, error)` is independent of the filesystem, CLI, and network. It returns suggested nested request paths for the shared writer to sanitize and disambiguate. Parsing is all-or-nothing for structural errors and unrepresentable HTTP methods; unsupported optional features generate stable, deduplicated warnings. Unknown unrelated JSON fields are tolerated for exporter compatibility.

Names, descriptions, order, repeated parameters, disabled headers/query/form fields, raw bodies and their MIME type, and URL-encoded bodies are preserved. Explicit enabled Content-Type headers override inferred raw-body types. An entirely disabled body is omitted. Literal dollar signs are escaped before `{{NAME}}` becomes `${NAME}`, preserving shell-looking strings such as `$HOME` and passwords containing dollar signs. Returned environment values are literal, without Posting escapes, for the common writer to quote.

Folder nesting is limited to 128 levels. Variable substitution limits depth, recursive work, and expanded value size to avoid cyclic or exponentially growing substitutions. The importer never executes scripts, evaluates expressions, reads file bodies/certificates, or fetches URLs.

## Limitations reported to users

- Multipart/form-data, file and GraphQL body modes are omitted with warnings; they are not incorrectly relabeled as URL-encoded forms.
- Postman scripts/tests, custom protocol settings, proxies and client certificates require manual migration.
- OAuth, AWS signing, NTLM and other unsupported auth helpers require manual configuration. Digest helper overrides are dropped; Posting negotiates the challenge with the server.
- Dynamic/vault variables and non-identifier variable names cannot become Posting variables. Their references remain literal and produce warnings. Only values present in the collection export are available; separate environments and runtime script values are not imported.
- Flattened collection variable aliases represent their imported default values. Later changes to an upstream variable do not recompute a flattened alias.
- Bare query flags normalize to `flag=`. URL query escaping normalizes on send. Posting path parameters escape slashes in values, so values intended to inject multiple path segments need manual adjustment.
- Only Posting's supported HTTP methods are accepted; unsupported methods fail explicitly rather than writing unloadable request files.

## Verification

The fixture covers nesting, auth inheritance/noauth, disabled values, string/object URLs, encoded and repeated query values, path suffixes, raw JSON, form bodies, and literal dollar escaping. Tests serialize/reload all imported fixture requests and compare their resolved behavior. A local HTTP server verifies actual method, path, query, body, MIME type, auth credentials and literal dollars after serialization. Additional tests cover v2 object auth, v2.1 array auth, API keys, scope isolation, invalid documents, unsupported-feature warnings and bounded variable expansion. `FuzzParse` checks deterministic output and that every successful imported request can be serialized and reloaded.
