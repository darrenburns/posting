# OpenAPI importer adversarial probe report

2026-09-30. Independent probe worktree based on Posting 3 integration `a4f8d49b`; the Posting 2/main implementation was not used. The parent's shared YAML persistence fix (`8c589b4b`, locally cherry-picked as `272599ac`) is required by one retained fuzz regression.

## Method and results

Used the diagnosing-bugs skill's failing-first loop. Each finding below was reproduced with a minimized Go regression before changing its implementation. These were direct deterministic conversion failures; additional ranked-hypothesis and instrumentation phases were unnecessary because the failing assertions isolated the precise conversion boundary. No debug instrumentation was left behind.

Reviewed primary OpenAPI, JSON Pointer, HTTP media-type, and YAML sources to settle ambiguities. In particular, current OpenAPI 3.0.4 explicitly clarifies form encoding behavior that older 3.0 revisions left ambiguous. Added general-document mutation fuzzing and a separate structured fuzz target that keeps every mutation inside a valid OpenAPI request envelope. The latter compares imported values through Posting YAML persistence and `model.Resolve`, including path escaping, repeated query values, dollar literals, control characters, Unicode, and JSON body contents.

Confirmed and fixed:

1. **Bodies disappeared for parameterized/mixed-case media types.** `application/json; charset=utf-8`, `Application/JSON`, and parameterized URL-encoded forms were treated as unrecognized raw formats. Media classification now parses the media type; a form's original parameterized Content-Type survives save/load and the actual HTTP request.
2. **Numeric examples changed silently.** `18446744073709551617` became `18446744073709552000`; `1.00000000000000000001` became `1`; YAML `1e400` became a string. JSON uses exact numeric tokens. YAML numbers are restored from scalar nodes with alias/merge precedence, explicit string tags, quoted strings, alternate-base integers, and decimal notation covered. The YAML restoration walk has an additional 250,000-node work limit.
3. **Invalid JSON Pointer indexes were accepted.** `+0`, `-0`, and `+1` previously indexed arrays. Array indexes now require the RFC's unsigned decimal syntax; existing leading-zero checks remain.
4. **Form objects and ignored encoding metadata changed requests.** A default `address: {zip: ...}` emitted `zip=...` instead of an `address` field containing JSON. Form object values now follow default JSON content serialization. Explicit style/explode/allowReserved select parameter serialization; contentType is then ignored. Form headers are ignored as required instead of causing the field to disappear. Persisted loopback HTTP tests verify both default and explicit style behavior for OpenAPI 3.0.4 and 3.1.1.
5. **Literal server path segments were substituted.** Server `https://example.test/:id` with operation `/{id}` and value `42` sent `/42/42`. Literal server colons are escaped for Posting so resolution produces `/:id/42`.
6. **Valid boolean-schema references failed conversion.** Referencing a component whose schema is `true` or `false` raised “expected an object.” Schema-specific reference resolution now handles them like inline boolean schemas; ordinary parameter/reference objects still require objects.
7. **Valid JSON Unicode was rejected by the YAML scanner.** Structured fuzzing minimized this to a U+008B example. JSON now uses its own decoder, retaining duplicate-key rejection, exact numbers, and depth limits. Corpus `55d2cd8a97a89c6b` records the failure.
8. **One operation's recursive schema erased another's explicit default.** A binary recursive schema exhausted the document-wide synthesis budget, after which a later unrelated schema default `"keep"` produced an empty body. The budget now bounds synthesis while explicit examples/defaults remain available.
9. **References amplified output without a total bound.** A roughly 4 MiB shared example reused across nine operations exceeded 32 MiB of materialized data, and repeated schema defaults could create one oversized body. Added a 32 MiB aggregate request/variable-data budget, incremental checks, immediate failure on overflow, and a bounded traversal before JSON encoding. Large JSON examples stay compact to prevent indentation amplification. Small bounded fixtures exercise both aggregate and single-body limits.

Structured fuzzing also independently reproduced the parent's shared serializer bug: a newline-only path value saved as empty and left `:id` unresolved. Corpus `295b3d32146579ad` now passes with the shared serializer fix; no shared module was modified by this probe.

## Verification

All commands used `GOWORK=off`, `login:false`, and the isolated Go worktree.

- `go test ./internal/importing/openapi -count=1`: passed after all deterministic fixes.
- `go test -race ./internal/importing/openapi -count=1`: passed, including saved/reloaded files and loopback HTTP requests.
- `go vet ./internal/importing/openapi`: passed.
- `git diff --check`: passed.
- `go test ./internal/importing/openapi -run '^$' -fuzz '^FuzzParse$' -fuzztime=60s -parallel=2`: initial campaign passed with 34,728 executions; post-decoder campaign passed with 27,462 executions. The final post-resource-limit campaign also passed with 27,712 executions and 32 new interesting inputs.
- `go test ./internal/importing/openapi -run '^$' -fuzz '^FuzzParameterRoundtrip$' -fuzztime=60s -parallel=2`: after fixing both minimized failures, passed with 326,098 executions and 426 new interesting inputs.
- Explicit adversarial cases cover alias expansion bombs, cyclic aliases, reference cycles, binary recursive schemas, pointer syntax, large precise numbers, persisted bodies, encoding/default precedence, literal server paths, and reused-example amplification.

The general parser fuzz oracle now checks that saving and loading does not change `model.Resolve` results or errors, rather than checking only whether the generated YAML parses.

## Sources and remaining scope

- [OpenAPI 3.1.1 media types, schemas, and encoding](https://spec.openapis.org/oas/v3.1.1.html), especially sections 3.6, 4.8.15, and 4.8.24.
- [OpenAPI 3.0.4 encoding clarification](https://spec.openapis.org/oas/v3.0.4.html#encoding-object).
- [RFC 6901 array-index grammar](https://www.rfc-editor.org/rfc/rfc6901.html#section-4).
- [RFC 9110 media-type casing and parameters](https://www.rfc-editor.org/rfc/rfc9110.html#name-media-type).
- [YAML 1.2.2 scalar/number definitions](https://yaml.org/spec/1.2.2/).

External references remain unfetched, and the existing documented limits on multipart, OAuth/OpenID/mTLS, embedded path templates, complex unsupported serializers, composition without examples, and dynamic JSON Schema scope remain. Generated scaffolds are editable examples, not validation-complete schema instances. Resource limits intentionally reject very large expanded imports. Fuzzing is bounded testing, not a proof of full OpenAPI conformance.
