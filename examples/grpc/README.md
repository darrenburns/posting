# Local gRPC playground

A real, loopback-only gRPC server and a Posting 3 collection covering unary,
server-streaming, client-streaming and bidirectional calls. No account, API key,
Docker, `protoc` or generated code is needed. The server compiles the embedded
[`demo.proto`](demo.proto) at startup using the repository's Go dependencies.

## Run

Requires Go 1.25.5 or newer. Run these commands from the repository root, in two
terminals. The first run downloads Go dependencies.

Start the server:

```sh
go run ./examples/grpc
```

Start **Posting 3 from this checkout**, not a separately installed Posting 2:

```sh
go run ./cmd/posting --collection examples/grpc/collection
```

The server listens on `127.0.0.1:50051`, with plaintext HTTP/2 and gRPC reflection.
The collection uses an explicit `grpc://` address to select plaintext. This is a
local development fixture, not an authenticated or TLS-enabled production server.
Stop each process with `Ctrl+C` when finished.

If the port is busy, start with `go run ./examples/grpc -addr 127.0.0.1:50052`
and change the address in Posting's URL bar (or the collection files) to match.

## Walkthrough

Select a request in the collection and press `Enter` to open it.

| Request | Action | Expected result |
| --- | --- | --- |
| **01 · Say hello** | `Ctrl+J` | `OK`, `Hello, Ada!`, sequence 1. |
| **02 · Server stream** | `Ctrl+J` | `Hello, Grace!`, `Bonjour, Grace!`, then `Hola, Grace!`, one second apart. |
| **03 · Client stream** | `Ctrl+J` | Three names sent in order; one reply with `names: ["Ada", "Grace", "Linus"]` and `count: 3`. |
| **04 · Bidi chat** | Follow the steps below | Replies arrive while the same call stays open. |

Streaming calls use the **Messages** tab. Once a call finishes, the inspector
follows its final status event. Click a **↓ received** row (or select it with
the arrow keys) to inspect that reply's full JSON.

### Bidirectional streaming, one message at a time

1. Open **04 · Bidi chat** and press `Alt+J`. Posting sends the initial message
   and shows **Streaming**. The server immediately replies with
   `{"text":"HELLO FROM POSTING!","sequence":1}`.
2. Click the message editor and replace its JSON with:

   ```json
   {"text": "Still on the same stream"}
   ```

3. Press `Ctrl+J`. The log gains a sent message and a received reply:
   `{"text":"STILL ON THE SAME STREAM","sequence":2}`. **Streaming** remains
   visible: neither side had to finish before receiving a reply.
4. Repeat with another message if desired. The server uppercases each message
   and increments `sequence` within that call; it is an echo service, not a chatbot.
5. Press `Alt+J` again to half-close the client side. The server ends normally;
   Posting shows **Call ended: OK**. The **Trailers** tab contains
   `x-messages-sent` with the number of replies.

If your terminal intercepts `Alt+J`, use `Ctrl+P` and choose **Open gRPC stream**
or **End gRPC stream** instead. `Escape` cancels an active call (leave any open
menu or message detail view first). In the response message list, arrow keys
select a payload to inspect; `f` resumes following the latest message.

For comparison, sending the bidi request with `Ctrl+J` before opening a stream
sends the current message and immediately half-closes. To try client streaming
interactively, use `Alt+J` on **03 · Client stream**: the server returns its
summary only after you press `Alt+J` to finish sending.

### Schema discovery and metadata

The requests use server reflection, so the collection needs no proto paths.
Focus the RPC method field to browse all four methods and their stream types.
To try local schema files instead, set **Proto → Files** to `../demo.proto`
(paths are relative to the collection directory).

The unary response includes the header `x-demo-server: posting-grpc`. Responses
also include message-count trailers. The server logs each request, reply, and
bidi half-close, making it easy to correlate the UI with real network activity.

## Check the demo

```sh
go test ./examples/grpc -v
```

The tests load the saved collection and call the real server over a loopback
socket with Posting's gRPC client. The bidi check waits for each reply before
sending the next message and verifies the stream completes after half-closing.
