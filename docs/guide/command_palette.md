## Overview

The *command palette* lists everything Posting can do, in one searchable place.
Press ++ctrl+p++ to open it, type a few letters of what you're looking for, and press ++enter++.

<figure class="screen">
--8<-- "palette.html"
<figcaption>The command palette, with each command's shortcut alongside it.</figcaption>
</figure>

The search is fuzzy, so `exc` finds **Export as curl** and `lay` finds the layout command.
Where a command has a keyboard shortcut, it's shown on the right, so the palette is also a handy way
to learn them.

| Key | Action |
|-----|--------|
| ++up++ / ++down++ (or ++ctrl+p++ / ++ctrl+n++) | Move through the list |
| ++enter++ | Run the command, or open its submenu |
| ++escape++ | Go back from a submenu, or close the palette |

Some things can only be done from the palette, such as switching the layout or exporting a request.

## Commands

Commands are grouped in the palette as follows. Some only appear when they apply, and a few change
their name to say what they'll do, like **Layout: side by side** and **Layout: stacked**.

### Requests and tabs

| Command | Shortcut | Description |
|---------|----------|-------------|
| Send request | ++ctrl+j++ | Send the request in the current tab |
| New request tab | ++ctrl+n++ | Open a new, empty request |
| Save request | ++ctrl+s++ | Save the request to the collection |
| Close request tab | ++alt+w++ | Close the current tab |
| Keep tab open | | Keep the [preview tab](./navigation.md#preview-tabs) open when you open another request (only shown for the preview tab) |
| Search requests… | ++ctrl+g++ | Search the collection |
| Go to open tab… | ++alt+down++ | Search your open tabs |
| Jump mode | ++ctrl+o++ | Enter [jump mode](./navigation.md#jump-mode) |

### Environment

| Command | Shortcut | Description |
|---------|----------|-------------|
| Switch environment… | | Choose an environment, or **No environment**. See [Switching environments](./environments.md#switching-environments) |
| Variables | ++ctrl+shift+v++ | Open the [variables screen](./environments.md#the-variables-screen) |
| Load environment file… | | Use a `.env` file from anywhere |

### Request

| Command | Description |
|---------|-------------|
| Duplicate request | Save a copy beside this request |
| Delete request | Remove this request's file from the collection |
| Copy response body | Copy the response body to the clipboard |
| Reload collection | Read the collection from disk again |
| Open gRPC stream | Call the method and keep sending messages until you end the stream. Only for a [gRPC request](./requests.md#open-streams). Shows as **End gRPC stream** while a stream is open |
| Refresh gRPC methods | Look for the server's methods again. Only for a [gRPC request](./requests.md#finding-methods) |
| Insert gRPC message template | Replace the message with a template for the chosen method. Only for a [gRPC request](./requests.md#messages-and-streams) |

### Import and export

| Command | Description |
|---------|-------------|
| Import curl command… | Paste a curl command to load it into this tab. See [Importing curl commands](./requests.md#importing-curl-commands) |
| Import curl from clipboard | Import the curl command on your clipboard (needs a terminal that lets apps read the clipboard) |
| Export as curl | Copy the request as a curl command. See [Exporting as curl](./requests.md#exporting-as-curl). For a gRPC request, this is **Export as grpcurl**. See [Exporting as grpcurl](./requests.md#exporting-as-grpcurl) |
| Export as YAML | Copy the request as a Posting request file |

### View

| Command | Shortcut | Description |
|---------|----------|-------------|
| Layout: side by side / Layout: stacked | | Put the response beside or below the request |
| View: hide collection / View: show collection | ++ctrl+h++ | Hide or show the sidebar |
| View: expand request | | Make the Request panel fill the screen |
| View: expand response | | Make the Response panel fill the screen |
| View: restore panels | ++alt+z++ | Show both panels again (only shown while one is expanded) |
| View: compact spacing / View: standard spacing | | Remove or restore the blank rows between parts of the screen |
| Theme… | | Choose a [theme](./themes.md), previewing each as you go |

### App

| Command | Shortcut | Description |
|---------|----------|-------------|
| Clear history | | Delete every entry in [history](./collections.md#history) for this collection |
| Keyboard shortcuts | ++f1++ | Show the keyboard shortcuts |
| Open documentation | | Open these docs in your browser |
| Quit Posting | ++ctrl+c++ | Quit |

Changes made from the **View** group and **Theme…** last until you quit. To make them permanent,
set `layout`, `spacing` and `theme` in your [configuration](./configuration.md).
