# Postman importer adversarial probe

This second phase ran against the integrated Posting 3 CLI/importer worktree, after the first importer implementation completed. It used additional upstream research, minimized semantic regressions, actual persisted requests and environments, loopback HTTP requests, structured mutations and native Go fuzzing.

## Method and evidence

Applied the diagnosing-bugs discipline: add a failing test at the public `Parse` → shared `Write` → request/environment reload → `model.Resolve` or HTTP sender boundary, run it to observe the incorrect result, then fix and rerun. Heavy hypothesis ranking and instrumentation were unnecessary for the deterministic failures here: each minimal input isolated one direct format conversion or serialization boundary, with an upstream implementation establishing the expected semantics. No production credentials or external services were used in tests.

Primary sources revisited:

- [Postman runtime API-key authorizer](https://github.com/postmanlabs/postman-runtime/blob/develop/lib/authorizer/apikey.js): removes existing matching fields before adding the helper value; headers compare without case; query keys compare exactly; empty helper does nothing; unknown placement falls back to a header. [Integration tests](https://github.com/postmanlabs/postman-runtime/blob/develop/test/integration/auth-methods/apikey.test.js) cover header/query/default/unrecognized placement and variable substitution.
- [SDK URL source](https://github.com/postmanlabs/postman-collection/blob/develop/lib/collection/url.js) and [URL tests](https://github.com/postmanlabs/postman-collection/blob/develop/test/unit/url.test.js): array path segments are joined without discarding leading empty segments; path substitutions preserve suffixes. URL parsing respects delimiters inside variable names.
- [SDK query source](https://github.com/postmanlabs/postman-collection/blob/develop/lib/collection/query-param.js) and [query tests](https://github.com/postmanlabs/postman-collection/blob/develop/test/unit/query-param.test.js): parameter strings retain existing percent escapes, while reserved delimiters are normalized.
- [SDK variable-list source](https://github.com/postmanlabs/postman-collection/blob/develop/lib/collection/variable-list.js), [variable tests](https://github.com/postmanlabs/postman-collection/blob/develop/test/unit/variable-list.test.js), and [property substitution](https://github.com/postmanlabs/postman-collection/blob/develop/lib/collection/property.js): last enabled duplicate wins; substitutions use the nearest variable source and inherited sources.

## Reproduced and repaired

The deterministic repro command was `GOWORK=off go test ./internal/importing/postman -run TestProbe -count=1`. Regressions were introduced and run before their fixes.

| Finding | Minimal symptom | Repair |
| --- | --- | --- |
| API-key override | Existing `X-Key: old` plus helper produced both old and replacement on the wire | Remove matching header/query entries before adding the helper; match case-insensitive header names |
| Empty/default API-key helper | Empty helper added a field; unknown destination dropped credentials | Match runtime no-op and header fallback behavior |
| Encoded API-key query values/names | `a%2Fb+c` became `a%252Fb%2Bc`; encoded key failed to replace existing key | Normalize query encoding before matching and storage |
| Encoded path variable | `a%20b` became `a%2520b` after resolve | Decode once into Posting's editable path value |
| Empty path extension | `:id.` with value `42` became `42` | Retain the dot even with an empty suffix |
| Leading empty array path segment | `["", "foo"]` became `/foo` instead of `//foo` | Trim a leading slash only for string paths |
| Null URL array entry | `["example", null]` silently reused the previous decoded string | Reject null host/path components explicitly |
| Inherited variable alias | Collection `URL={{BASE}}/api` ignored folder `BASE` | Materialize inherited aliases that transitively depend on a local override |
| Reserved characters inside templates | `{{#}}` and `{{&}}` were split as URL syntax, losing diagnostics | Split URL/query delimiters only outside template references |
| Percent-encoded template literals | `%7B%7BROOT%7D%7D` unexpectedly substituted ROOT | Interpret source templates before percent decoding; protect newly decoded dollars |
| Aggregate expansion amplification | A small inherited payload duplicated across many requests could allocate without a collection-wide bound | Reject imports whose cumulative variable expansion exceeds 32 MiB |
| Shared YAML serialization (reported to parent) | Structured fuzz reduced a failure to a single newline: query/header value `"\n"` reloaded as empty | Parent repaired shared YAML string emission; importer retains minimized fuzz corpus as a cross-layer regression |

Additional mutations verify duplicate and disabled variable precedence, sibling isolation, escaped dollar syntax, Unicode, quotes, backslashes, multiline strings, malformed/null collection members, explicit query arrays replacing raw queries, and persistence through imported environment files with a hostile host-variable lookup.

## Fuzzing and verification

- Native parser fuzzing: `GOWORK=off go test ./internal/importing/postman -run '^$' -fuzz '^FuzzParse$' -fuzztime=60s -parallel=2` — passed, **2,056,523 executions**.
- New structured semantic target: `FuzzProbeVariableQueryRoundtrip` always constructs valid collections, exercises scoped and disabled variables, encoded query values and actual request/environment persistence, and compares resolved behavior with independently constructed expected values.
- Initial structured run discovered the shared newline truncation after **2,916 executions**. Minimized corpus: `testdata/fuzz/FuzzProbeVariableQueryRoundtrip/a7f7a265bde6cd00`.
- Structured fuzz after the shared repair: **63,381 executions over 60 seconds**, passed. A further 60-second run passed **50,485 executions** with the final URL decoding fixes.
- Package tests, race detection and `go vet` cover the final implementation. Loopback HTTP checks confirm API-key override behavior after writing/reloading imported files.

## Remaining limits

These probes are bounded tests, not proof of complete Postman equivalence. Existing documented unsupported body/auth/script modes remain unsupported. Root variable defaults containing percent escapes or plus signs can mean encoded URL data in Postman but literal data in Posting; query uses now emit a warning requiring review. API-key names that use variables warn that field replacement uses imported defaults, because Posting has no runtime auth helper capable of removing a newly matching field after a variable is edited. Collection alias defaults remain snapshots unless local overrides require materialization. Path values intentionally containing unescaped slashes still require manual review as documented in the implementation research. URL query encoding is normalized, so differently encoded spellings of otherwise equivalent query keys do not retain their raw spelling.
