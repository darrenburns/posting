# Response history verification

Verified on macOS with Python 3.11.7 and Textual 6.1.0.

## Automated checks

```sh
PATH="$PWD/.venv/bin:$PATH" env -u NO_COLOR make test ARGS='-q'
```

Result: **212 passed, 4 skipped** across the Makefile's parallel and serial phases
(211 passed / 4 skipped in parallel; 1 passed in serial). `NO_COLOR` was unset so
snapshots use the application's normal theme.

- Six storage tests cover restart persistence, byte-for-byte headers (including
  repeated and non-ASCII headers), cookies, elapsed time, binary and compressed
  bodies, private file permissions, collection isolation, entry and byte limits,
  deletion, and corrupt databases.
- Twelve new UI snapshots cover empty and replay states in both spacing modes,
  persistence across startup, editor and session-cookie preservation, no duplicate
  records on replay, returning to live responses, disabling recording, deletion
  and clear confirmation, narrow/right sidebar layout, storage failure, and entries
  removed by another process.
- Existing sidebar snapshots were updated for the new tabs and stable sidebar width.
- Rendered snapshots were visually inspected for normal, compact, narrow, and
  collection-tree layouts.

## Computer-use verification

Ran the real Posting application through `textual serve`, using computer-use mouse
and keyboard actions in the local browser terminal. All HTTP traffic used a disposable
local test server on `127.0.0.1:8766`. Its `/items` endpoint returned a JSON body and
response cookie; `/missing` returned HTTP 404.

1. Opened History and checked the explanatory empty state.
2. Sent real requests to `/items` and `/missing`; verified newest-first ordering.
3. Entered an unsent draft URL, then clicked the older response. Its original body
   appeared and the draft remained unchanged.
4. Used `k` and `Enter` to load the 404 response from history.
5. Restarted Posting, selected a persisted entry, and inspected its saved headers.
6. Opened a second Posting instance in compact mode and replayed the saved response.

The four PNGs below are actual computer-use screenshots. They contain synthetic
local test data only. Deletion and confirmation behavior are covered by automated
UI tests.

### Empty history

![Empty history](empty.png)

### Replay while preserving an unsent draft

![Replay preserves the request editor](replay.png)

### Saved headers after restarting Posting

![Persisted headers](headers-after-restart.png)

### Compact layout

![Compact history](compact.png)
