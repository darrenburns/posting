## Installation

Posting can be installed via [`pipx`](https://pipx.pypa.io/stable/) or [Rye](https://rye-up.com/guide/installation) on MacOS, Linux, and Windows:

```bash
pipx install posting
# or
rye install posting
```

On MacOS, you can also install Posting via Homebrew:

```bash
brew install darrenburns/homebrew/posting
```

Note that the Homebrew installation method requires compiling some Rust dependencies, and may take a few minutes to complete.

## A quick introduction

This introduction will show you how to create a simple POST request to the [JSONPlaceholder](https://jsonplaceholder.typicode.com/) mock API to create a new user. It focuses on an efficient keyboard-driven workflow, but you can also use the mouse if you prefer.

### Collections and requests

A *collection* is simply a directory which may contain requests saved by Posting.

If you launch Posting without specifying a collection, any requests you create will be saved in the "default" collection.
This is a directory reserved by Posting on your filesystem, and unrelated to the directory you launched Posting from.

This is fine for quick throwaway requests, but you'll probably want to create a new collection for each project you work on so that you can check it into version control.

To create a new collection, simply create a new directory and pass it into Posting:

```bash
mkdir my-collection
posting --collection my-collection
```

Now, any requests you create will be saved in the `my-collection` directory as simple YAML files with the `.posting.yaml` extension.

### Creating a request

When you launch Posting, no request is open.

You can immediately start creating a request, and then save it to disk with ++ctrl+s++.
Let's do just that.
We'll create a simple POST request to the [JSONPlaceholder](https://jsonplaceholder.typicode.com/) mock API to create a new user.

Press ++ctrl+t++ to open the request method dropdown, then press ++p++ to quickly select the `POST` method.

Press ++tab++ to move focus to the next widget, which is the URL bar. The URL bar can also be focused with ++ctrl+l++. Type `https://jsonplaceholder.typicode.com/users` into the URL bar.

### Adding a JSON body

Press ++ctrl+o++ to enter "jump mode", then press ++w++ to quickly jump to the "Body" tab.

Press ++j++ (or ++down++) to move the cursor down to the dropdown. Press ++enter++ to open it, and select the option `Raw (json, text, etc.)`.

Move down to the text area below using ++j++ (or ++down++), and type (or paste) the JSON below. 

```json
{
  "name": "John Doe",
  "username": "johndoe",
  "email": "john.doe@example.com"
}
```

### Viewing keyboard shortcuts

Now is probably a good time to note that you can see the full list of keyboard shortcuts for the focused widget by pressing ++f1++. The text area widget in particular has a lot of useful shortcuts and supports things like undo/redo.


### Sending the request

Press ++ctrl+j++ to send the request. This shortcut works globally.

### Saving the request

Finally, press ++ctrl+s++ to save the request to disk.
Fill out the form on the modal that appears, and press ++enter++ or ++ctrl+n++ to write the request to disk.
