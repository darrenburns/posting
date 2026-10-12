# Contributing to Posting

Posting is open to contributions! 🚀

Contributions of any type, no matter the "size" and form, are welcome!
Bug reports, typo fixes, ideas, questions, and code are all valuable ways of contributing.
If you're new to open source and would like a bit of extra guidance, don't be afraid to ask - we all start somewhere 😌

## How do I contribute?

You can suggest ideas by creating a discussion, or report bugs by creating an issue.

If you wish to contribute code, and it's a change which is not an "objective improvement", please open a discussion first.
An "objective improvement" is a change which _indisputably_ improves Posting.
Some examples include adding test coverage, performance improvements, bug fixes, and fixing clear inconsistencies.

By opening a discussion, we can make sure:

- You're not working on something that someone else has already started.
- The feature is within Posting's scope.
- We can iron out the details before you go and commit a bunch of time to it!
- Maintainers can give you tips, and the discussion will be a place you can ask for help and guidance.

## Development

Posting 3 is written in Go. Development takes place on `main`. The `2.x` branch is for maintenance
of the Python version.

Install [Go](https://go.dev/dl/) 1.25.5 or newer, as specified in `go.mod`, then run Posting from the
repository root:

```bash
go run ./cmd/posting
```

To build a binary:

```bash
go build -o posting ./cmd/posting
./posting
```

The repository includes a collection and environment files for manual testing:

```bash
go run ./cmd/posting --collection tests/sample-collections/ --env tests/sample-envs/sample_base.env --env tests/sample-envs/sample_extra.env
```

### Check Go changes

Format changed Go files with `gofmt`. Run the same build, vet, and tests that CI runs:

```bash
go build ./...
go vet ./...
go test ./...
```

### Build the documentation

The website uses MkDocs Material. Install its dependencies with
[uv](https://docs.astral.sh/uv/getting-started/installation/):

```bash
uv sync --group docs
uv run mkdocs serve
```

Before submitting documentation changes, check the build:

```bash
uv run mkdocs build --strict
```

`make docs-screens` regenerates the Posting 3 screens embedded in the website.

### Maintain Posting 2

For Python fixes, work from `2.x` and target pull requests at that branch.
Use `uv` to install the development dependencies and run Posting 2:

```bash
uv sync
uv run posting
```

To connect Posting 2 to the Textual development tools, run:

```bash
TEXTUAL=devtools,debug uv run posting
```

Run Python tests through the Makefile, which separates tests that must run serially from the
parallel tests:

```bash
uv run make test
```

Posting 2 UI tests compare SVG snapshots. Inspect the report for each changed snapshot before
accepting the new output. To update snapshots after checking the differences, run:

```bash
uv run make test-snapshot-update
```

Commit intentional snapshot changes with the code that caused them.

### Update the changelog

A changelog is maintained in the `docs/CHANGELOG.md` file, which follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.

Add user-visible changes under the release heading marked `[Unreleased]`.
If there isn't an unreleased section, add one at the top.

## Releasing Posting 3

Releases are built by [GoReleaser](https://goreleaser.com) when a tag starting with `v` is pushed.
The version comes from the tag, so nothing in the code needs changing.

1. Replace `[Unreleased]` in the changelog heading with the release date.
2. Tag the release commit and push the tag, for example `git tag v3.0.0-beta.0 && git push origin v3.0.0-beta.0`.
   Go only accepts tags in [semantic version](https://semver.org) form, so write a beta as `-beta.0`, not `b0`.

The Release workflow publishes a GitHub release with archives for macOS, Linux and Windows, and updates the
`posting@beta` cask in [darrenburns/homebrew-homebrew](https://github.com/darrenburns/homebrew-homebrew).
It needs a `HOMEBREW_TAP_GITHUB_TOKEN` secret, with write access to that repository, in the repository's
`goreleaser` environment.

To try a release locally without publishing it, run `goreleaser release --snapshot --clean` and look in `dist/`.

## Feeling unsure?

If you're feeling a bit stuck, just open a discussion and I'll do my best to help you out!
