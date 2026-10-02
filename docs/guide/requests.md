## Overview

A request is everything Posting needs to make an HTTP call: the method and URL, headers, query and
path parameters, a body, authentication and a few options. You edit a request in the **Request**
panel, send it with ++ctrl+j++, and save it into your [collection](./collections.md) with ++ctrl+s++.

## Creating a request

Press ++ctrl+n++ to open a new, empty request in a tab of its own. Choose a method with ++ctrl+t++
(or press a method's letter while the method selector has focus: ++g++ `GET`, ++p++ `POST`,
++u++ `PUT`, ++a++ `PATCH`, ++d++ `DELETE`, ++h++ `HEAD`, ++o++ `OPTIONS`), and type a URL.

You don't need to type `https://` or `http://`: a URL without a scheme is sent over `http://`.

A new request isn't written to disk until you [save it](#saving-a-request).

## Editing a request

The **Request** panel has seven tabs. Jump to any of them with ++ctrl+o++ followed by
++q++ ++w++ ++e++ ++r++ ++t++ ++y++ ++u++, or move along the tabs with ++left++ and ++right++ and
press ++down++ to move into one.

Type `$` in the URL bar, a header, query or form value, the auth fields, the proxy URL or the body
to see the variables you can use.
See [Environments](./environments.md) for how variables work.

### Headers

Headers are edited in a table. Type into the empty row at the bottom to add a header, and another
empty row appears beneath it. As you type a header's name, Posting suggests common header names.

| Key | Action |
|-----|--------|
| ++up++ / ++down++ | Move between rows |
| ++ctrl+space++ | Enable or disable the row |
| ++ctrl+x++ | Delete the row |

You can also click the checkbox at the start of a row to enable or disable it, or the `✕` at its
end to delete it. A disabled row stays in the request, and is saved with it, but isn't sent.
The Query and form tables work the same way. The Path table only has values to fill in: its rows
come from the URL (see [Path](#path)).

### Body

Choose the kind of body with the selector at the top of the tab (++left++ and ++right++ change it):

- **None** sends no body.
- **Raw** sends the text you type. Choose its content type beside it: **JSON**, **Text**, **XML** or
  **HTML**. The text is syntax highlighted, and Posting sends the matching `Content-Type` header,
  unless you've set a `Content-Type` header yourself.
- **Form** sends URL-encoded form fields (`application/x-www-form-urlencoded`), edited in a table.

Press ++f4++ in the body to edit it in your own editor, or ++f3++ to view it in your pager.
See [External Tools](./external_tools.md).

!!! note

    Posting 3 can't send multipart form data or upload files yet.

### Path

Path parameters are placeholders in the URL's path, written as `:name`:

```
https://api.example.com/users/:id/comments/:commentId
```

Each placeholder you type in the URL gets a row in the **Path** tab, where you fill in its value.
The rows come from the URL, so you add or remove them by editing the URL. Values are inserted into
the URL when the request is sent. A placeholder with an empty value is sent as it is.

Values can contain variables, such as `$USER_ID`.

To send a literal `:name` in a path, double the colon: `::name` is sent as `:name`.

### Query

The **Query** tab lists the query string's parameters, and it's kept in step with the URL.
Type `?page=2&per_page=50` at the end of the URL and two rows appear in the Query tab; add, change
or remove rows and the URL updates to match.

A disabled row is kept in the Query tab but removed from the URL, so it isn't sent.

### Auth

Choose how the request authenticates:

- **None** sends no credentials.
- **Basic** sends a username and password with HTTP Basic authentication.
- **Digest** answers the server's Digest challenge with a username and password.
- **Bearer token** sends `Authorization: Bearer <token>`.

The `Authorization` header is created when the request is sent, and replaces any `Authorization`
header you've added yourself (for Digest auth, once the server has sent its challenge). Credentials can use variables, for example `${API_TOKEN}`, so you can
keep secrets out of your request files.

### Info

The request's **Name** and **Description**, and the **File** it's saved in.

The name is shown in the request tabs and the collection browser. The description is shown when you
hover over the request in the collection browser, and supports `code` in backticks.

### Options

| Option | Default | Description |
|--------|---------|-------------|
| **Follow redirects** | On | Follow `3xx` responses to their destination, stopping with an error after 10 requests in a chain. When it's off, you see the redirect response itself. |
| **Verify SSL certificates** | On | Reject servers whose certificates can't be verified. Turn it off for servers with self-signed certificates. See also [Configuring SSL](./configuration.md#configuring-ssl). |
| **Attach cookies** | On | Send cookies set by earlier responses, and remember cookies from this one. Cookies are kept until you quit Posting, and are shared by all tabs. |
| **Substitute body variables** | On | Replace variables in the body. Turn this off to send a body that contains literal `$` signs, such as a JSON schema or a GraphQL query. Variables in the URL, headers and auth are still replaced. |
| **Proxy URL** | Empty | Send the request through this proxy, e.g. `http://proxy.example.com:8080`. When it's empty, the standard `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` environment variables apply. |
| **Timeout** | 5 | How many seconds to wait for the whole request, including redirects and reading the response. |

## GraphQL requests

To make a GraphQL request, open the method selector with ++ctrl+t++ and choose **GraphQL**, or press
++q++ while the method selector has focus. The selector shows `GraphQL`, and the collection browser,
request tabs and history show a `GQL` badge where an HTTP request shows its method. Choose a method
to turn it back into an HTTP request. The URL, headers, auth and options stay as they are when you
switch, and so does the query, so you can switch back and forth without losing anything.

A GraphQL request has two tabs of its own, before the usual ones:

- **Query** holds the GraphQL document, with syntax highlighting. If the document defines more than
  one operation, type the name of the one to run in **Operation**, above the document.
- **Variables** holds the operation's variables as a JSON object, such as `{"id": "${USER_ID}"}`.

The other tabs work as they do for an HTTP request. There's no **Body** tab, and the URL's query
parameters are in the **Params** tab, because **Query** is the document. Press ++f4++ in the
document or the variables to edit them in your own editor.

Posting sends a GraphQL request as a `POST` with `Content-Type: application/json` and a JSON body
of `query`, `variables` and `operationName`. Empty variables and an empty operation name are left
out. Unless you've set an `Accept` header, Posting also sends
`Accept: application/graphql-response+json, application/json`.

### Variables in GraphQL requests

In the **Variables** tab, Posting variables work as they do in a raw body: `$NAME` and `${NAME}` are
both replaced, and typing `$` suggests the variables you can use.

In the query, only `${NAME}` is replaced. A bare `$name` there is a GraphQL variable, so
`query User($id: ID!)` is sent as written, even if your environment defines `id`.

Turn off **Substitute body variables** in the **Options** tab to send the query and the variables
exactly as written.

### GraphQL responses

GraphQL servers often answer `200 OK` when an operation fails, and put the failure in an `errors`
list in the response body. When a `2xx` response has errors, Posting shows its status in the
warning colour, in the URL bar, the response panel and the history, and the response panel counts
the errors, for example `200 1 error`.

### GraphQL request files

A saved GraphQL request has `kind: graphql` and a `graphql` section instead of a method and a body:

```yaml
name: Get user
kind: graphql
url: ${BASE_URL}/graphql
graphql:
  query: |
    query User($id: ID!) {
      user(id: $id) {
        name
        email
      }
    }
  variables: |
    {"id": "${USER_ID}"}
  operation_name: User
auth:
  type: bearer_token
  bearer_token:
    token: ${API_TOKEN}
```

Posting 2 doesn't know about GraphQL requests. It opens them as a `GET` of the URL with no body, and
saving one in Posting 2 removes `kind` and the `graphql` section, which loses the query. Edit
GraphQL requests only in Posting 3.

## gRPC requests

To make a gRPC request, open the method selector with ++ctrl+t++ and choose **gRPC**, or press
++r++ while the method selector has focus. The selector shows `gRPC`, and the collection browser,
request tabs and history show an `RPC` badge where an HTTP request shows its method. Choose a
method to turn it back into an HTTP request. The address, metadata, auth and options stay as they
are when you switch, and so do the gRPC method and message.

### The server's address

Type the server's address in the URL bar, for example `localhost:50051`. The address is a host and
a port, with no path. The method goes in the **Method** field. The address's scheme decides whether
Posting connects with TLS:

| Address | Connection |
|---------|------------|
| `grpc://host:port` or `http://host:port` | Plaintext |
| `grpcs://host:port` or `https://host:port` | TLS |
| `localhost:50051`, `127.0.0.1:50051` or `[::1]:50051` | Plaintext, because the host is a loopback address |
| Any other address without a scheme, such as `api.example.com:8443` | TLS |

Without a port, TLS uses port 443 and plaintext uses port 80. If a TLS connection fails because the
server only speaks plaintext, the error suggests `grpc://`. If a plaintext connection fails because
the server wants TLS, the error suggests `grpcs://`.

**Verify SSL certificates** in the **Options** tab, and the [SSL settings](./configuration.md#configuring-ssl)
in your configuration, apply as they do to HTTP requests.

### The Message and Proto tabs

A gRPC request has two tabs of its own, before the usual ones:

- **Message** has the **Method** field, and the message to send below it.
- **Proto** lists the `.proto` files that describe the server, for servers that don't offer
  reflection. Leave it empty to ask the server.

The other tabs work as they do for an HTTP request, with these differences:

- There's no **Body**, **Path** or **Query** tab.
- The **Headers** tab is called **Metadata**. Each enabled row is sent as gRPC metadata, with its
  name in lower case. A name that ends in `-bin` takes a base64 value, which Posting decodes before
  sending. Names that gRPC sets itself (`content-type`, `te`, and names that start with `grpc-` or
  `:`) are errors, and so are HTTP/1 headers that HTTP/2 forbids (`connection`, `keep-alive`,
  `proxy-connection`, `transfer-encoding`, `upgrade` and `host`). A `user-agent` row replaces
  Posting's own user agent, and gRPC adds its version after it.
- Basic and bearer auth are sent as `authorization` metadata. Digest auth needs an HTTP challenge,
  so the **Auth** tab doesn't offer it, and a request file with `type: digest` doesn't load.
- The **Options** tab has **Verify SSL certificates**, **Substitute body variables** and
  **Timeout**. The timeout is the deadline for the whole call, including connecting.

### Finding methods

Posting finds the server's methods when the **Message** tab opens, and when the **Method** field
takes focus, if the address or the proto files have changed since it last looked. Press ++ctrl+r++
in the **Method** field, or choose **Refresh gRPC methods** in the command palette, to look again.

When the **Proto** tab is empty, Posting asks the server's reflection service. When the server has
no reflection service, add its `.proto` files on the **Proto** tab instead:

- **Files** lists the files, one per line, relative to the collection directory. A file ending in
  `.protoset`, `.binpb` or `.pb` is a compiled descriptor set, such as `protoc --descriptor_set_out`
  or `buf build -o` writes. Any other file is `.proto` source, which Posting compiles itself, so you
  don't need `protoc`.
- **Import paths** lists the directories that `import` statements are relative to. With none, the
  collection directory is the only one. Each `.proto` file must be inside one of them.

With proto files, Posting lists the methods without connecting to the server.

The line under the **Method** field shows how many methods Posting found, and where, for example
`4 methods via reflection · plaintext`. Once you choose a method, it shows the method's kind and its
message types, such as `server stream · ListBooksRequest → Book`. If Posting couldn't find the
methods, the line says why.

The **Method** field lists the methods while it has focus. Type to filter them, and press
++enter++ to choose one. You can also type a method yourself as `package.Service/Method` or
`package.Service.Method`.

### Messages and streams

Write the message as JSON, in protobuf's JSON format: field names in `lowerCamelCase` or as they're
written in the `.proto` file, enum values by name, and well-known types such as `Timestamp` in their
JSON form. When you choose a method and the message is empty, or still holds the previous method's
template, Posting fills it in with a template that has every field of the method's input. Choose
**Insert gRPC message template** in the command palette to replace the message with a template at
any time.

What you write depends on the method:

- A unary or server-streaming method takes one JSON object.
- A client-streaming or bidirectional method takes a JSON array of objects, and Posting sends each
  one as a message, in order. A single object is sent as a stream of one message.
- An empty message is sent as one empty message, for any method.

Variables work as they do in a raw body: `$NAME` and `${NAME}` are both replaced, and typing `$`
suggests the variables you can use. Turn off **Substitute body variables** to send the message
exactly as written. Press ++f4++ in the message to edit it in your own editor.

Posting sends every message and closes its side of the call while it reads the server's replies, so
a server that answers each message as it arrives works too. The response shows the replies when
the call ends. Posting doesn't show messages as they arrive, and you can't send more messages while
a call is running.

### gRPC responses

The response panel shows the call's status by name, such as `OK` or `NOT_FOUND`, with the server's
status message beside it. Only `OK` is shown in the success colour.

The **Body** tab shows the server's messages as JSON, with fields at their default values included:

- An object for a unary or client-streaming method.
- An array for a server-streaming or bidirectional method, even when the server sent one message or
  none.
- When the call fails and the server sent no messages, an object with the status's `code`,
  `message` and `details`. Posting decodes the details whose types it knows, such as the
  `google.rpc` error details. It shows the others as their type URL and base64 value.

The **Headers** tab shows the response metadata. A gRPC response has a **Trailers** tab instead of
**Cookies**. Trailers arrive when the server ends the call, and they start with `grpc-status` and,
when the server sent one, `grpc-message`. A call that ends without the server's trailers, such as
one that times out, has none.

When a call runs past its timeout, the response shows the messages that arrived before the deadline,
with the status `DEADLINE_EXCEEDED`. When Posting can't reach the server, or the server sends
nothing before the deadline, the response panel shows an error, as it does for an HTTP request.

Posting keeps up to 64 MB of messages from a call, the same limit as an HTTP response body. When a
stream sends more, Posting cancels the call. The response shows the messages that arrived before
the limit, with the status `CANCELLED` and the message `response truncated at 64.00 MB`.

Each send opens a new connection to the server. Without proto files, it also asks the server for
the method's schema again, so a changed server is picked up straight away.

### Exporting as grpcurl

For a gRPC request, the command palette offers **Export as grpcurl** in place of **Export as curl**.
The command uses `-plaintext` or `-insecure` to match the connection, `-H` for metadata and auth,
`-d` for the message, `-max-time` for the timeout, and `-import-path`, `-proto` and `-protoset` for
the proto files, with absolute paths so the command runs from any directory. grpcurl reads a stream
as JSON objects one after another, so an array message is written that way.

### gRPC request files

A saved gRPC request has `kind: grpc` and a `grpc` section instead of a method and a body:

```yaml
name: Get user
kind: grpc
url: localhost:50051
grpc:
  method: acme.users.v1.UserService/GetUser
  message: |
    {"id": "${USER_ID}"}
  proto:
    files:
      - protos/acme/users/v1/users.proto
    import_paths:
      - protos
auth:
  type: bearer_token
  bearer_token:
    token: ${API_TOKEN}
headers:
  - name: x-tenant
    value: core
options:
  timeout: 10
```

Leave out the `proto` section to use reflection. A gRPC request file can't have `method`, `body`,
`params` or `path_params`, and its `options` can't have `follow_redirects`, `attach_cookies` or
`proxy_url`. Any other key in the `grpc` or `proto` sections is an error.

Posting 2 doesn't know about gRPC requests. Edit them only in Posting 3.

### What gRPC requests don't do yet

- Show a stream's messages as they arrive, or send messages during a bidirectional call.
- gRPC-Web and Connect.
- Compressed messages.
- Proxies.
- Client certificates for a single request. The client certificate in your
  [SSL settings](./configuration.md#configuring-ssl) applies to every request.
- Importing gRPC requests from Postman, Bruno or grpcurl.
- Completing proto file paths.
- Reusing a connection from one send to the next.

## Sending a request

Press ++ctrl+j++ (or ++alt+enter++) from anywhere to send the request in the current tab, or press
++enter++ in the URL bar. Press ++escape++ to cancel it while it's in flight.

Posting sends a `User-Agent` header identifying itself unless you've set your own.
If the URL's address or path uses a variable that isn't defined, Posting tells you which one
instead of sending the request. An undefined variable anywhere else, including the query string,
is sent as written.

See [Responses](./responses.md) for what you can do with the response.

## Saving a request

Press ++ctrl+s++ to save the request in the current tab.

The first time you save a request, Posting asks where to save it:

| Field | Description |
|-------|-------------|
| **Name** | The request's name. Required. |
| **File name** | The name of the file, without `.posting.yaml`. Leave it empty to use one made from the name. |
| **Folder** | The folder in the collection to save it in, such as `users/admin`. Leave it empty to save at the top of the collection. Missing folders are created. |
| **Description** | An optional description. |

Press ++enter++ to save. After that, ++ctrl+s++ saves changes to the same file.

### Renaming a request

Change the **Name** in the **Info** tab and save. The request keeps its file name; to rename or move
the file itself, rename or move it on disk and Posting will notice the change.

### Duplicating a request

Press ++d++ with the cursor on a request in the collection browser, or choose **Duplicate request**
in the command palette, to make a copy. The copy is saved beside the original, with `(copy)` added
to its name, and opened in a new tab.

To duplicate [several requests](./collections.md#selecting-several-requests) at once, select them
first. Their copies aren't opened. They become the selection instead.

### Deleting a request

Press ++backspace++ with the cursor on a request in the collection browser, or choose **Delete request**
in the command palette to delete the request in the current tab. Posting asks you to confirm,
then deletes the file. With [several requests](./collections.md#selecting-several-requests) selected,
Posting asks once and deletes them all.

If the request is open in a tab, the tab stays open with its contents intact, as an unsaved request,
so you can save it again if you change your mind.

## Importing curl commands

Paste a `curl` command into the URL bar and Posting turns it into a request, filling in the method,
URL, headers, query parameters, body, authentication and options.

<figure class="screen">
--8<-- "curlhint.html"
<figcaption>A curl command in the URL bar, ready to import.</figcaption>
</figure>

If your terminal doesn't paste the command in one go, type or paste it and press ++enter++.
A command split over several lines with `\` at the end of each line is supported.

You can also:

- choose **Import curl command…** in the command palette, and paste the command into the dialog;
  press ++ctrl+j++ to import it
- choose **Import curl from clipboard** to import the command on your clipboard directly (this
  needs a terminal that lets applications read the clipboard)

Importing replaces everything in the current tab's request except its name and description.
The request keeps its file too, so importing into a saved request updates it once you save.

Posting understands the options people most often copy from browsers and API docs, including
`-X`, `-H`, `-d` and its variants, `--json`, `-F`, `-u`, `--digest`, `-b`, `-A`, `-e`, `-k`, `-L`,
`-x`, `-m`, `-G`, `-I` and `--oauth2-bearer`. Options that don't affect the request, such as `-s`,
`-v` and `-o`, are ignored. A few things to be aware of:

- `-F` fields are sent as a URL-encoded form, not multipart, and `@file` values aren't read from disk.
- `$NAME` in a pasted command isn't expanded: it becomes a Posting variable.

## Exporting as curl

Choose **Export as curl** in the command palette to copy the request as a `curl` command.
The command is copied to your clipboard straight away, and shown in a dialog where you can check it.

<figure class="screen">
--8<-- "curlexport.html"
<figcaption>A request exported as a curl command.</figcaption>
</figure>

The **Values** / **Variables** switch at the top of the dialog chooses between a command with
variables replaced by their values (the default), ready to run, or one with the variables left
in, ready to share. Press ++y++ to copy the command again.

To add your own options to every exported command, set `curl_export_extra_args` in your
[configuration](./configuration.md):

```yaml
curl_export_extra_args: "--silent --show-error"
```

This is inserted straight after `curl`.

## Exporting as YAML

Choose **Export as YAML** in the command palette to copy the request in Posting's file format,
a quick way to share a request with another Posting user.

## The request file format

Requests are saved as YAML files ending in `.posting.yaml`. They're designed to be easy to read,
review and edit by hand. Here's an example:

```yaml
name: Create user
description: Adds a new user to the system.
method: POST
url: https://${API_HOST}/users/:team
body:
  content: |-
    {
      "firstName": "John",
      "email": "john.doe@example.com"
    }
  content_type: application/json
auth:
  type: bearer_token
  bearer_token:
    token: ${API_TOKEN}
headers:
- name: X-Request-ID
  value: docs-example
- name: X-Debug
  value: 'true'
  enabled: false
params:
- name: sendWelcomeEmail
  value: 'true'
path_params:
- name: team
  value: engineering
options:
  timeout: 30
```

Only settings that differ from the defaults are written, so most files are short:

| Key | Description |
|-----|-------------|
| `name`, `description` | Shown in the collection browser and the **Info** tab. |
| `kind` | `graphql` for a [GraphQL request](#graphql-requests), `grpc` for a [gRPC request](#grpc-requests). Left out for an HTTP request. |
| `method` | The HTTP method. Left out for `GET`. |
| `url` | The URL, without its query string. |
| `headers`, `params` | Lists of `name` and `value`. `enabled: false` marks a disabled row. |
| `path_params` | The values of the URL's `:name` placeholders. |
| `body` | Either `content` (with `content_type`) for a raw body, or `form_data` (a list of `name` and `value`) for a form. |
| `graphql` | A GraphQL request's `query`, `variables` and `operation_name`. Any other key in it is an error. |
| `grpc` | A gRPC request's `method`, `message` and `proto` (`files` and `import_paths`). Any other key in it is an error. |
| `auth` | `type` is `basic`, `digest` or `bearer_token`, with credentials under a key of the same name (`basic: {username, password}`, `bearer_token: {token}`). |
| `options` | Any of `follow_redirects`, `verify_ssl`, `attach_cookies`, `substitute_body_variables`, `proxy_url` and `timeout`. |
| `scripts` | Posting 2 scripts. Posting 3 keeps them but doesn't run them; see [Scripting](./scripting.md). |

The format is the same as Posting 2's, so HTTP request files can be shared between the two.
