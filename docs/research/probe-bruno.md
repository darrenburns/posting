# Bruno importer independent probing

Probed in an isolated Posting 3 worktree based on `a4f8d49b`, with the shared YAML persistence fix `8c589b4b` cherry-picked for final verification. No Posting 2 code was used.

## Reproduced and fixed

Every issue below had a minimized failing regression before its fix. Deterministic assertions isolated the cause; additional hypothesis ranking and runtime instrumentation were unnecessary for these small parser/conversion defects.

- Context-free counting of triple apostrophes rejected ordinary values and quoted keys containing them. Conversely, a multiline value starting on the block-opening line was not tracked, so an embedded column-zero brace terminated the block early. Delimiters now open only in syntactically appropriate value/annotation positions, including the opening line.
- A quoted annotation argument containing three apostrophes was misread as a multiline annotation and rejected. Annotation skipping now uses the same contextual delimiter check.
- Disabled `~"~key"` lost both leading tildes. Only the disabled marker is removed, leaving the actual `~key` name.
- Inline raw-body text lost trailing spaces and gained a newline. It now preserves its bytes after Bruno's indentation removal.
- Multiline indentation removal sliced UTF-8 bytes rather than JavaScript UTF-16 units, corrupting non-ASCII input. Tests cover accented and astral characters; the conversion now follows UTF-16 slicing, replacing a split surrogate when producing UTF-8.
- JSON interpolation followed the selected body editor mode rather than the effective Content-Type. Text bodies with JSON headers produced invalid JSON, while JSON-mode bodies with text headers acquired unwanted escaping. Effective headers are now expanded before body interpolation. An unresolved Content-Type produces an explicit warning because its future value cannot determine import-time escaping.
- Per-field expansion limits allowed aggregate amplification: a 1 MiB variable repeated in 17 headers passed. A request-wide 16 MiB expansion-work bound now also counts intermediate recursive results.
- Collection input-byte limits did not constrain inherited values copied into many requests: a roughly 1 MiB collection expanded to more than 128 MiB. The loader now caps cumulative materialized requests at 128 MiB, including conservative request/row overhead.

## Evidence and verification

Primary sources checked on 2026-09-30:

- [Official Bruno v2 grammar and semantic actions](https://github.com/usebruno/bruno/blob/main/packages/bruno-lang/v2/src/bruToJson.js): dictionary literals, disabled keys, annotations, inline bodies, and multiline slicing.
- [Official text outdent utility](https://github.com/usebruno/bruno/blob/main/packages/bruno-lang/v2/src/utils.js): raw-text indentation.
- [Official request preparation](https://github.com/usebruno/bruno/blob/main/packages/bruno-cli/src/runner/prepare-request.js) and [runtime interpolation](https://github.com/usebruno/bruno/blob/main/packages/bruno-cli/src/runner/interpolate-vars.js): explicit headers override body defaults, and the effective Content-Type controls JSON escaping.
- [Upstream request fixture](https://github.com/usebruno/bruno/blob/main/packages/bruno-lang/v2/tests/fixtures/request.bru): quoted/disabled rows, separate path/query parameters, auth, and body selectors.

Regression commands (all with `GOWORK=off`):

- `go test ./internal/importing/bruno -run TestProbe -count=1`: initially reproduced the grammar, whitespace and disabled-key failures; subsequent focused tests reproduced content-type and aggregate-size failures. All pass after fixes.
- `go test ./internal/importing/bruno -count=1`: pass.
- `go test -race ./internal/importing/bruno -count=1`: pass.
- `go vet ./internal/importing/bruno`: pass.
- `git diff --check`: pass.

Two semantic wire tests use real `collection.Dir.Save`, `Load`, and the HTTP client against an `httptest` server. They check Content-Type-sensitive escaping and a nested collection containing inherited bearer auth, literal dollars, folder headers, disabled overrides, repeated query parameters, path values, and enabled/disabled URL-encoded fields. Filesystem probes cover the 64-level nesting limit, oversized files, symlink metadata, directory cycles, and row limits. Expansion tests cover both existing recursive bombs and aggregate amplification.

Native fuzz campaigns used two workers and no remote services:

| Target | Duration | Executions | Result |
|---|---:|---:|---|
| `FuzzParse` before fixes | 60 seconds | 4,641,440 | pass |
| `FuzzProbeJSONSubstitution` | 60 seconds | 1,525,490 | pass |
| `FuzzParse` after initial fixes | 60 seconds | 2,009,016 | pass |
| `FuzzParse` after all fixes + shared YAML fix | 60 seconds | 2,166,837 | pass |

Commands use `go test ./internal/importing/bruno -run '^$' -fuzz=TARGET -fuzztime=60s -parallel=2`. `FuzzProbeJSONSubstitution` builds valid requests around arbitrary UTF-8 single-line scalar values, imports and serializes them, reloads them, calls `model.Resolve`, then asserts the actual decoded JSON value equals the original. It exercises escaping and substitution semantics, beyond parser survival. Inputs that introduce multiline grammar, nested references, or surrounding whitespace are excluded from that property; dedicated examples cover multiline grammar.

## Limits

The documented unsupported features remain unsupported. Runtime Content-Type selection and unbound JSON variables cannot be made fully context-aware by this importer. No execution-based differential comparison against Bruno's Node runtime was run because Node/npm were not on the probe shell's PATH; grammar/runtime source and upstream fixtures supplied the semantic oracle. Fuzzing is bounded evidence, not an exhaustive proof. Symlink probes cover stable filesystem state rather than adversarial concurrent filesystem mutation. The directory-entry and total source-byte limits were inspected but not driven to their full thresholds; expanded output, depth, per-file and row limits were exercised.
