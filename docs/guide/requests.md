## Overview

Requests are stored directly on your file system as simple YAML files, suffixed with `.posting.yaml` - easy to read, understand, and version control!

## Example

Here's an example of what a request file looks like:

```yaml
name: Create user
description: Adds a new user to the system.
method: POST
url: https://jsonplaceholder.typicode.com/users
body: 
  content: |-
    {
      "firstName": "John",
      "email": "john.doe@example.com"
    }
headers:
- name: Content-Type
  value: application/json
params:
- name: sendWelcomeEmail
  value: 'true'
```

## Creating a new request

Press ++ctrl+n++ to create a new request.

You'll be prompted to supply a name for the request.
By default, this name is used to generate the filename, but you can also choose your own filename if you wish.

!!! tip
    If you already have a collection loaded, the path in the "New Request" dialog will be pre-filled based on the position of the cursor in the collection tree, so moving the cursor to the correct location *before* pressing ++ctrl+n++ will save you from needing to type out the path.

Within the "Path in collection" field of this dialog, it's important to note that `.` refers to the currently loaded *collection* directory (that is, the directory that was loaded using the `--collection` option), and *not* necessarily the current working directory.

### Duplicating a request

With a the cursor over a request in the collection tree, press ++d++ to create a duplicate of that request. This will bring up a dialog allowing you to change the name and description of the request, or move it to another location.

To skip the dialog and quickly duplicate the request, press ++shift+d++, creating it as a sibling of the original request. The file name of the new request will be generated automatically. You can always modify the name and description after it's created in the `Info` tab.

## Saving a request

Press ++ctrl+s++ to save the currently open request.

If you haven't saved the request yet, a dialog will appear, prompting you to give the request a name, and to select a directory to save it in.

!!! tip "Folders"

    Requests can be saved to folders - simply include a `/` in the `Path in collection` field when you save the request, and Posting will create the required directory structure for you.

If the request is already saved on disk, ++ctrl+s++ will overwrite the previous version with your new changes.

## Loading requests

Requests are stored on your file system as simple YAML files, suffixed with `.posting.yaml`.

A directory can be loaded into Posting using the `--collection` option, and all `.posting.yaml` files in that directory will be displayed in the sidebar.

## GraphQL

Posting has a dedicated editor for GraphQL queries and mutations.

In the `Body` tab, change the body type to `GraphQL`, and you'll be given three fields:

- **Operation** - the name of the operation to run. Only required if your query contains more than one named operation. It's sent as `operationName`.
- **Query** - your query or mutation, written as plain, *unescaped* text - exactly as you'd write it in any other GraphQL editor.
- **Variables (JSON)** - the variables that accompany the query, written as a JSON object.

When the request is sent, Posting wraps these up into the JSON payload that GraphQL servers expect, escaping the query for you:

```json
{"query": "query GetUser($id: ID!) {\n  user(id: $id) {\n    name\n  }\n}", "variables": {"id": "1"}, "operationName": "GetUser"}
```

The `content-type` header is set to `application/json` automatically (unless you've set it yourself), and selecting the GraphQL body type switches the method to `POST`, which is how GraphQL is sent. Loading a saved request never changes the method it was saved with, so a `GET` GraphQL request stays a `GET`.

If the variables field doesn't contain a valid JSON object, Posting will tell you rather than sending the request.

The query is syntax highlighted as you type - keywords, types, fields, arguments, variables, enum values, strings and comments each get their own colour from your theme.

### GraphQL in the request file

GraphQL requests are stored in the request file under `body.graphql`, so the query stays readable and diffable:

```yaml
name: Get user
method: POST
url: https://example.com/graphql
body:
  graphql:
    query: |-
      query GetUser($id: ID!) {
        user(id: $id) {
          name
        }
      }
    variables: '{"id": "1"}'
    operation_name: GetUser
```

### Choosing the operation

A document can define more than one operation:

```graphql
query Articles {
  articles { items { id } }
}

query Podcasts {
  podcasts { items { id } }
}
```

A server can't guess which of them to run, so when you send a request like this Posting asks:

```
╭ Send which operation? ───────────────────────╮
│ This query defines more than one operation,  │
│ so the request must say which one to run.    │
│                                              │
│  query Articles                              │
│  query Podcasts                              │
╰──────────────────────────────────────────────╯
```

Picking one writes it into the `Operation` field, so it's sent as `operationName`, saved with the request, and you're only asked once. Change it there (or clear it to be asked again) whenever you want to run the other one. Dismiss the prompt with ++escape++ and the request isn't sent.

Requests with a single operation are sent without any of this - the server can work out what to run.

### Fetching the schema

Press ++f5++ while the GraphQL editor has focus (or run `graphql: Fetch schema` from the command palette) and Posting will send an introspection query to the endpoint in the URL bar.

The introspection request reuses the URL, headers, auth and options of the open request, so endpoints behind authentication work exactly like sending the request itself does.

The label at the right of the `Operation` row tells you whether a schema is available for the current endpoint - it shows `no schema` until you fetch one, then the number of types in the schema.

Schemas are cached per endpoint URL, both in memory and inside Posting's data directory (run `posting locate data` to find it), so autocompletion keeps working when you restart Posting. Press ++f5++ again whenever the API changes to fetch a fresh copy.

### Autocompletion

With a schema fetched, the `Query` field suggests what can appear at the cursor as you type:

- fields of the type you're selecting from, including inside nested selection sets, aliases, inline fragments (`... on User`) and fragment definitions
- argument names for a field, and the values of enum arguments
- the fields of input objects, including nested ones
- the variables declared by the operation, after you type `$`
- type names after `... on`

| Key | Action |
| --- | --- |
| ++ctrl+space++ | Ask for suggestions at the cursor |
| ++up++ / ++down++ | Move through the suggestions |
| ++enter++ / ++tab++ | Insert the highlighted suggestion |
| ++escape++ | Dismiss the suggestions |

### Browsing the schema

Press ++f2++ (or run `graphql: Browse schema` from the command palette) to open the schema browser.

The left pane is a tree of everything the endpoint supports:

- the root fields of `query`, `mutation` and `subscription`
- every type in the schema, under `types`

Expand a field to walk into the type it returns, and keep going as deep as you like. The right pane describes whatever the cursor is on - its type, description, arguments and their defaults, enum values, input fields, or the concrete types of a union.

| Key | Action |
| --- | --- |
| ++enter++ | Insert the field into the query editor |
| ++space++ | Expand or collapse the entry |
| ++slash++ | Filter fields and types |
| ++escape++ | Close the browser |

Vim-style navigation works too: ++j++ and ++k++ move, ++l++ selects, ++h++ jumps to the parent, and ++g++/++shift+g++ go to the top and bottom.

#### Searching the schema

The filter searches the whole schema, not just what's on screen. Typing into it groups the results:

- the root fields of each operation whose name matches
- under `fields`, the matching fields of *any* type, qualified with the type that owns them (`User.posts`, `Post.title`)
- under `types`, the matching type names

Fields also match on their qualified name, so `user.na` finds `User.name`. Matches on the start of a name come first, and if a search matches a very large number of fields, only the first 200 are listed - narrow the filter to see the rest.

#### Inserting from the browser

Pressing ++enter++ on a field writes it into the `Query` field:

- A **root field**, with an empty query, becomes a complete operation. Required arguments become variables, and the leaf fields of the return type are selected for you:

    ```graphql
    query GetUser($id: ID!) {
      user(id: $id) {
        id
        name
      }
    }
    ```

    The `Variables (JSON)` field is filled with a template (`{"id": null}`) when it's empty, ready for you to fill in.

- A **root field, with a query already written**, is appended as a second operation. Since a document with more than one operation has to say which one to run, Posting sets `Operation` to the new one.

- **Any other field** - or a root field while your cursor is inside a selection set - is inserted at the cursor as a selection, indented to match. If it needs a variable that the operation doesn't declare yet, Posting tells you which one to add.

Generated selections include every field of a type - both the fields which can be selected on their own, and objects expanded into selection sets of their own, up to four levels deep. So a field like this comes out whole:

```graphql
getStuff {
  object1 {
    field1
    field2
    listOfObjects {
      items {
        itemField1
      }
    }
  }
}
```

Deprecated fields are left out, and a type is never expanded inside itself. Required arguments become variables wherever they appear, including on nested fields - so a paginated list keeps its `first:` argument, and the variable is declared for you. Types with nothing left to select get `__typename`, which is also what you get for a union - use `... on SomeType` to select from it.

Very wide types are capped at 150 fields per selection, so picking a field can't paste thousands of lines - Posting tells you when it has left fields out.

### Variables inside GraphQL queries

GraphQL uses `$name` for its *own* variables, which would otherwise clash with [Posting's variables](./environments.md).

Inside the **Query** field, only the braced form - `${name}` - refers to a Posting variable. A bare `$name` is left exactly as you typed it, so queries can use GraphQL variables freely:

```graphql
query GetUser($id: ID!) {
  user(id: $id) {
    ${extra_field}
  }
}
```

The **Variables** and **Operation** fields behave like every other field in Posting - both `$name` and `${name}` are substituted there.

## Path parameters

Path parameters let you insert placeholders directly in the URL path using `:name` syntax. For example:

```
https://api.example.com/users/:id/comments/:commentId
```

When you type placeholders like this in the URL bar, Posting automatically extracts them and shows them in the Path tab. You can edit the values there, but you cannot add or remove rows manually — the rows come from the URL. Values are substituted into the URL path when the request is sent.

### Useful shortcuts

- With the cursor over a `:name` token in the URL bar, press ++alt+down++ to jump to that row in the Path tab.
- With a row highlighted in the Path tab, press ++alt+down++ to jump to the corresponding token in the URL bar.


### YAML representation

Path parameter values are saved in the request file under `path_params`:

```yaml
name: get comments
url: https://jsonplaceholder.typicode.com/posts/:postId/comments
path_params:
- name: postId
  value: '3'
```

You can also use variables from the environment in path parameter values (e.g., `$FOO`). Variables are resolved before the values are substituted into the URL.

### Escaping literal colons

If you need a literal `:name` in the path, escape it by doubling the colon. For example, `::id` renders as `:id` and is not treated as a placeholder:

```
http://example.com/users/::id/:id  →  http://example.com/users/:id/123
```

## Deleting a request

You can delete a request by moving the cursor over it in the tree, and pressing ++backspace++.

## Sharing requests

An easy way to share a request with others is to copy it as a cURL command.
Press ++ctrl+p++ and select `export: copy as curl` to copy the request as a cURL command to your clipboard.

You can also press ++ctrl+p++ and select `export: copy as YAML` to copy the request as YAML. This provides a quick way to share a request with other Posting users, e.g. via Slack.
