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

We use [uv](https://docs.astral.sh/uv/getting-started/installation/) to manage the development dependencies.

To setup an environment for development, run:

```bash
uv sync
```

This will create a virtual environment with the dependencies installed.
Activate the virtual environment with:

```bash
source .venv/bin/activate
# or, with fish shell:
source .venv/bin/activate.fish
```

Then you can run Posting with:

```bash
posting
```

If you wish to connect to the Textual dev tools (strongly recommended), you can run `posting` like this:

```bash
TEXTUAL=devtools,debug posting
```

The repo includes a test collection which is used in the automated tests, but you can also load it in for manual testing.

```bash 
TEXTUAL=devtools,debug posting --collection tests/sample-collections/ --env tests/sample-envs/sample_base.env --env tests/sample-envs/sample_extra.env
```

### Running the tests

To run the tests, you MUST use the `Makefile` commands.

```bash
make test
```

This will run the tests in parallel, but isolate a few tests which need to be run serially.
If you try to run the tests using a raw `pytest` command, you're gonna have a bad time.

### Snapshot testing

Snapshot testing is the primary way we test the UI of Posting.

It works by taking a "screenshot" of the running app at (actually an SVG!) the end of the test, and comparing it to the previous screenshot for that test.
If it matches, the test passes. If it doesn't, the test fails and you'll be able to view a report which shows the differences.

It's your job to look at this diff and consider whether the new output (on the left) is correct or not.

If the results are all as expected, you can update the snapshots by running:

```bash
make test-snapshot-update
```

This will update the snapshots saved on disk for all the tests which failed.
You should commit these changes into the repo - they're essentially the "source of truth" for what the UI of Posting should look like under different circumstances.

### Update the changelog

A changelog is maintained in the `docs/CHANGELOG.md` file, which follows the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.

When you're making a change which should be recorded in the changelog.
You should add your change to the `## Unreleased` section of the changelog.
If the `## Unreleased` section doesn't exist, you should add it at the top.

## Feeling unsure?

If you're feeling a bit stuck, just open a discussion and I'll do my best to help you out!
