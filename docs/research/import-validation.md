# Posting 3 importer implementation and validation

Work was based on `origin/worktree-posting-3` at `fe96f5c114049ac7667b6379a5b4e696539e5538`.
The integration branch is `codex/posting3-importers`. No changes were made to the
Posting 2 `main` checkout.

## Process

Three implementation agents each received an isolated worktree from Posting 3.
They researched primary specifications, official implementations and upstream
fixtures/tests, designed a pure conversion interface, implemented their format,
and added fixtures, semantic tests and initial fuzz targets. The parent agent
implemented CLI dispatch, format detection and a shared collision-safe writer.

After all three implementations finished and the integrated CLI tests passed,
three separate probing worktrees were created from the complete implementation.
Two fresh agents independently probed OpenAPI and Bruno. The agent limit prevented
a third fresh agent, so the Postman implementation agent was reused for a distinct
adversarial pass, as authorized. The parent probed persistence, paths, environment
files and CLI diagnostics. Findings were reproduced before fixes, with minimized
regressions and saved fuzz failures.

## Delivered behavior

```sh
posting import api.yaml -o ./api
posting import collection.postman_collection.json -o ./api
posting import ./bruno-collection -o ./api
posting import request.bru -o ./api
posting import --type postman collection.json -o ./api
```

Format detection and explicit `--type` work with flags before or after the source.
Without `-o`, the destination is a named folder in the default collection.
Converted requests use Posting's existing YAML collection format. Defaults are
written to a separately named environment file when appropriate, and the CLI
prints the invocation needed to load it. The writer does not replace existing
files, sanitizes source-derived filenames, confines writes using `os.Root`, and
removes newly created files if an import write fails. Source scripts are never
executed and external OpenAPI references are never fetched.

Supported subsets and remaining limitations are documented in the
[import guide](../guide/importing.md) and each format's cited research:
[Postman](import-postman.md), [OpenAPI](import-openapi.md), [Bruno](import-bruno.md).
Multipart/file bodies, executable scripts and unsupported authentication or
serialization modes require manual migration and produce diagnostics.

## Shared findings and fixes

- The original collection serializer trimmed multiline body whitespace. Imports
  now preserve payload bytes and descriptions rather than normalizing them.
- Dotenv single-quote escaping and initial CRLF trimming changed literal imported
  variable values. The writer and loader now roundtrip quotes, backslashes,
  dollar expressions, CRLF, empty strings and multiline values without host
  environment expansion.
- YAML block-scalar emission produced unreadable tab/newline values and dropped
  initial newlines. A scalar wrapper quotes the affected values across raw bodies,
  forms, query/path values, metadata and credentials. Fuzz-generated invalid UTF-8
  remains supported through YAML's binary-string representation. Three minimized
  body-fuzzer failures are committed; format-specific fuzzers independently found
  the same issue for query/header/path parameters.
- CLI auto-detection sent valid JSON through YAML's stricter character scanner.
  Detection now uses the JSON decoder first, with a regression for U+008B.
- Invalid source request names could inject terminal controls and additional lines
  into CLI errors. Both warnings and errors now render controls literally.

The shared writer has regression coverage for repeated imports, pre-existing
files, hostile filenames, symlink escapes, cleanup after a failed write, and
validation before output creation. CLI integration tests cover all formats,
Bruno directories, auto-detection, explicit selection, interspersed flags,
default destinations, variable-loading instructions, malformed input, size
limits, help and diagnostics.

## Probing reports

The reports record actual failures, fixes, upstream evidence and verification:

- [Postman probing](probe-postman.md)
- [OpenAPI probing](probe-openapi.md)
- [Bruno probing](probe-bruno.md)

Shared fuzz campaigns used Go's native fuzzer with two workers:

| Property | Successful executions | Duration |
|---|---:|---:|
| Exact environment-value roundtrip | 874,692 | 10 seconds |
| Exact environment-value roundtrip, follow-up | 1,167,306 | 20 seconds |
| Portable collection-relative output paths | 3,957,239 | 60 seconds |
| Arbitrary raw-body persistence, after minimized fixes | 657,180 | 60 seconds |

The path property checks the generated path and each component; actual filesystem
confinement and symlink behavior are tested separately. Body and environment
properties compare the saved-and-reloaded value to the original bytes. Bounded
fuzz campaigns are evidence of tested behavior, not proof of complete format
compatibility.

## Integration verification

The executable was built with `GOWORK=off go build -o /private/tmp/posting3-import-check ./cmd/posting`.
A subprocess smoke test imported a four-request Postman fixture, a three-operation
OpenAPI fixture and a Bruno collection. Importing each fixture a second time
retained the original files and doubled the request count.

After integrating every probing fix, the following checks passed:

```sh
GOWORK=off go test ./...
GOWORK=off go test -race ./...
GOWORK=off go vet ./...
GOWORK=off go build -o /private/tmp/posting3-import-check ./cmd/posting
git diff --check
```

The final rebuilt executable passed the three-format subprocess smoke check,
including a second import with explicit `--type`, byte-for-byte preservation of
every existing request file, and `posting import --help`.

The first full-repository race run exposed a pre-existing race in
`TestSessionSendLifecycle`. It reproduced in a temporary snapshot of the original
Posting 3 base: the standalone test dispatched callbacks inline from a worker
while reading them and cancelling on the test goroutine. The test now queues and
runs callbacks on its owning goroutine, matching the app and the existing
send-focus test pattern. The focused race test passed ten consecutive runs;
the final full-repository race check then passed. This required only a test
harness correction.

Across initial implementation and probing campaigns, the recorded successful
fuzz runs executed **20,778,336 iterations**. These counts include repeated and
mutated corpus entries and campaigns before and after repairs; they are not a
count of distinct inputs. See the individual reports for target-level counts
and the minimized failures discovered by semantic fuzzing.

All implementation and probing worktrees were retained for inspection. The
integration worktree contains the combined committed result; the `main` checkout
remains clean.
