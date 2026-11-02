# Feature demos

These are recordings and screenshots of Posting's real UI, using the Galaxy
theme and local demo servers. They contain no live credentials or external API
data. The theme slideshow in `../themes/` is unchanged.

## Capture setup

Build Posting from the repository root:

```sh
GOWORK=off go build -o /tmp/posting-readme-bin ./cmd/posting
```

Run `python3 demo/server.py` and `go run ./examples/grpc` in separate terminals.
Use a disposable copy of `demo/sample-api` for the HTTP demos, with the `local`
environment selected. The GraphQL endpoint returns a canned `GetUser` response;
it is not a general-purpose GraphQL server. The gRPC demos use the real service
and collection in `examples/grpc`.

The captures use Terma v0.26.0's `cmd/terma-browser` launcher, which runs Posting
in a PTY inside an xterm.js terminal:

```sh
GOWORK=off go install github.com/darrenburns/terma/cmd/terma-browser@v0.26.0
POSTING_THEME=galaxy POSTING_HEADING__HOSTNAME=you@devbox \
  TERMA_DISABLE_PIXEL_MOUSE=1 terma-browser -port 8750 -- \
  /tmp/posting-readme-bin --collection /path/to/sample-api-copy -e local
```

Open the URL printed by the launcher. Set `cols` and `rows` query parameters
before its authentication fragment. Use **100 × 32 cells** for the recordings
and **64 × 36 cells**, with the sidebar hidden (`Ctrl+H`), for both stills.
For gRPC, launch a second instance with `--collection examples/grpc/collection`
and no environment flag.

Load JetBrains Mono 2.304 regular, bold, italic and bold italic explicitly into
the browser, disable ligatures, and set the terminal font to 16px. Wait for all
four font faces with `document.fonts.load` and `document.fonts.ready`; verify
that `iiii`, `WWWW` and `0000` each measure 38.4px. Hide the launcher's web
header and terminal padding, and capture just the terminal at **2× scale**.
The resulting recordings are 1920 × 1344px; the stills are 1228 × 1512px.

## Scenes

- **Jump mode:** start with List users and Get order open and sent. Search for
  `post users` with `Ctrl+G`, open Create user, clear the search and send it.
  Use `Ctrl+O` and the displayed label to focus a new header name. Type `Acce`,
  accept `Accept`, then enter `application/json`. Jump to the response body,
  pausing after the first character of its two-character label.
- **Mouse:** switch between Get order and List users, drag the horizontal
  divider upwards, scroll the response, then drag to select Katherine Johnson's
  name. Record the real moving pointer, not an added animation.
- **Responses:** expand List users' response with `Alt+Z`, find `role` with
  `Ctrl+F` and advance a match. Press `Escape`, then `t` for the JSON tree.
  Collapse the root with `Home`, `Left`, then expand with `Right`. Focus
  JSONPath with `Ctrl+F`, paste `$.users[?@.role=="admin"]`, and press `Enter`.
  The result contains Ada's complete object and shows **1 match**.
- **gRPC:** open Bidi chat, focus Method to discover the schema, then open the
  stream with `Alt+J`. Send `Hello from Posting!`, `Still streaming`, and
  `One more message`, editing the message between sends. Capture the three
  replies while the status is still **Streaming**.
- **GraphQL:** open Get user in `graphql/`, send it, and keep both the query
  and successful JSON response visible.

## GIF encoding

Record at 2× scale, then encode at 12 fps without resizing. Use two passes to
avoid buffering every full-resolution frame in memory:

```sh
ffmpeg -i scene.mp4 -vf 'fps=12,palettegen=stats_mode=diff' \
  -frames:v 1 palette.png
ffmpeg -i scene.mp4 -i palette.png \
  -lavfi 'fps=12[x];[x][1:v]paletteuse=dither=none:diff_mode=rectangle' \
  -loop 0 scene.gif
gifsicle -O3 --lossy=30 scene.gif -o scene-optimized.gif
```

Inspect the encoded animations, including intermediate jump labels and tree
states, before replacing the README assets. Check the Markdown at desktop and
narrow widths; GitHub makes each image a link to its full-resolution version.
