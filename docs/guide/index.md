Posting is an HTTP client that lives in your terminal.
Posting 3 is a single, self-contained binary: there's no Python or other runtime to install.

## Installation

Posting 3 is in beta. Install it with Homebrew, download a release, or build it with Go.

### Homebrew

On macOS and Linux, install the beta from Posting's Homebrew tap:

```bash
brew install darrenburns/homebrew/posting@beta
```

Posting 2's Homebrew formula also installs a command called `posting`, so run
`brew uninstall posting` first if you have it.

### Downloading a release

Each [GitHub release](https://github.com/darrenburns/posting/releases) has archives for macOS,
Linux and Windows. Extract the `posting` binary and put it in a directory on your `PATH`.

### Go

If you have [Go](https://go.dev/dl/) 1.25.5 or newer installed, run:

```bash
go install github.com/darrenburns/posting/v3/cmd/posting@latest
```

This builds Posting and puts the `posting` binary in `$(go env GOPATH)/bin` (usually `~/go/bin`).
Make sure that directory is on your `PATH`, then run:

```bash
posting
```

### Building from source

To build from a clone of the repository instead:

```bash
git clone https://github.com/darrenburns/posting.git
cd posting
go build -o posting ./cmd/posting
./posting
```

### Checking your installation

```bash
$ posting version
Posting 3.0.0-beta.0
```

`posting --help` lists the options you can start Posting with.

!!! tip "Coming from Posting 2?"

    Posting 3 reads the same configuration file, collections and `.env` files as Posting 2,
    so you can point it at your existing work straight away.
    See [Coming from Posting 2](./migrating.md) for what's new and what's changed.

## A quick introduction

This introduction walks through creating a POST request to the
[JSONPlaceholder](https://jsonplaceholder.typicode.com/) mock API, sending it, and saving it.
It focuses on the keyboard, but you can click on anything in Posting too.

<figure class="screen">
--8<-- "response.html"
<figcaption>Posting 3 with a collection in the sidebar and a JSON response on screen.</figcaption>
</figure>

### Collections

A *collection* is a directory of requests. Each request is stored in its own YAML file, so a
collection is easy to read, share and keep in version control.

If you start Posting without choosing a collection, your requests are saved to the *default
collection*, a directory Posting keeps for you. That's handy for quick, throwaway requests, but
for a project you'll probably want a collection of its own:

```bash
mkdir my-collection
posting --collection my-collection
```

The name of the collection is shown at the top of the sidebar on the left.
Learn more in [Collections](./collections.md).

### Choosing the method

When Posting opens, the cursor is in the URL bar of a new, empty request.

Press ++ctrl+t++ to open the method menu. Each method has a letter you can press to choose it:
press ++p++ for `POST`.

### Entering the URL

Posting moves you back to the method selector. Press ++tab++ to move to the URL bar (or press
++ctrl+l++ from anywhere) and type:

```
https://jsonplaceholder.typicode.com/users
```

The URL is highlighted as you type, so typos are easy to spot.

!!! tip "Paste a curl command"

    You can also paste a whole `curl` command into the URL bar. Posting turns it into a request,
    filling in the method, headers and body for you. See [Importing curl commands](./requests.md#importing-curl-commands).

### Adding a JSON body

Press ++ctrl+o++ to enter *jump mode*. Labels appear over every part of the screen: type the
label of the place you want to go. Press ++w++ to jump to the request's **Body** tab.

Press ++down++ to move into the tab, which puts you on the body type selector, then press ++right++
to change the body type from **None** to **Raw**. JSON is already selected as the content type,
which is what we want, so press ++tab++ twice to move past it and into the text area. Type or paste:

```json
{
  "name": "John Doe",
  "username": "johndoe",
  "email": "john.doe@example.com"
}
```

The body is syntax highlighted, and Posting sends `Content-Type: application/json` for you
when the request goes out.

### Sending the request

Press ++ctrl+j++ to send the request. This works wherever you are in Posting.
You can also press ++alt+enter++, or press ++enter++ while the URL bar is focused.

The response appears in the **Response** panel, with its status, size and how long it took.
The JSON is formatted and highlighted for you.

### Working with the response

Press ++ctrl+o++ then ++a++ to jump to the response **Body** tab, and ++down++ to move into the
body. From here you can:

- scroll with the arrow keys, ++page-up++ and ++page-down++
- press ++y++ to copy the whole body to your clipboard
- press ++w++ to turn line wrapping on or off
- press ++f3++ to open the body in your pager, or ++f4++ to open it in your editor

The **Headers**, **Cookies** and **Trace** tabs show the rest of the response.
See [Responses](./responses.md) for more.

### Saving the request

Press ++ctrl+s++ to save the request. The first time you save, Posting asks for a name, and
where in the collection to put it:

- **Name**: `Create user`
- **File name**: leave it empty to use one made from the name (`create-user`)
- **Folder**: leave empty to save at the top of the collection, or type a path such as `users`
  to save into a folder, which is created if it doesn't exist

Press ++enter++ to save. The request now appears in the sidebar, and it's written to
`my-collection/users/create-user.posting.yaml`. From now on, ++ctrl+s++ saves your changes in place.

## Finding your way around

A few keys will get you a long way:

| Key | What it does |
|-----|--------------|
| ++ctrl+p++ | Open the [command palette](./command_palette.md), which lists everything Posting can do |
| ++ctrl+o++ | Enter [jump mode](./navigation.md#jump-mode) |
| ++f1++ | Show the keyboard shortcuts |
| ++ctrl+n++ | Open a new request tab |
| ++ctrl+g++ | Search the requests in the collection |
| ++ctrl+c++ | Quit |

The footer at the bottom of the screen always shows the most useful keys for whatever is focused.

## Making it yours

- Try a different look: press ++ctrl+p++, choose **Theme…**, and move through the list to preview
  each theme. See [Themes](./themes.md).
- Put the request and response side by side with **Layout: side by side** in the command palette,
  or remove the blank rows between sections with **View: compact spacing**.
- Make these choices permanent in your [configuration file](./configuration.md):

```yaml
theme: aurora
layout: horizontal
spacing: compact
```
