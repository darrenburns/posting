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
