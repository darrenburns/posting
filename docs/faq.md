# Frequently Asked Questions

## Using Posting

### How do I edit headers or query parameters?

Edit them in place: move to the row with the arrow keys (or click it) and type. Type into the empty
row at the bottom of a table to add a row, press ++ctrl+x++ to delete one, and ++ctrl+space++ to
enable or disable one. See [Requests](./guide/requests.md#headers).

### Can I use Posting 3 and Posting 2 on the same collection?

Yes. Both versions use the same request files, config file and `.env` files.
See [Coming from Posting 2](./guide/migrating.md).

### Does Posting 3 run my Posting 2 scripts?

Not yet. Requests with scripts still open and send, and the scripts are kept when you save,
but they aren't run. See [Scripting](./guide/scripting.md).

### Why does a key combination do nothing?

Your terminal, multiplexer (such as tmux) or operating system may be using it, or may not be able to
send it. ++ctrl+h++ in particular is sent as ++backspace++ by many terminals. You can choose different
keys in your [keymap](./guide/keymap.md), and most things are also in the
[command palette](./guide/command_palette.md) (++ctrl+p++).

### Why don't icons show up?

Posting only draws icons when it knows a [Nerd Font](https://www.nerdfonts.com/) is available.
If you use one, set `nerd_fonts: true` in your [configuration](./guide/configuration.md).

### Where does Posting keep its files?

Run `posting locate config`, `posting locate collection` or `posting locate themes` to find the
config file, the default collection and the themes directory. History and remembered environments
are kept in `~/.local/share/posting/` (or `$XDG_DATA_HOME/posting/`).

## Contributing

### How do I suggest a feature?

You can suggest a feature by opening a Discussion on the [GitHub repository](https://github.com/darrenburns/posting/discussions) under the "Ideas" category.

### How do I report a bug?

You can report a bug by opening an Issue on the [GitHub repository](https://github.com/darrenburns/posting/issues).

### How do I contribute code to Posting?

You can contribute code to Posting by opening a Pull Request on the [GitHub repository](https://github.com/darrenburns/posting/pulls).

However, reporting bugs and suggesting features is also a great way to contribute!

## General

### How was Posting built?

Posting 3 is written in [Go](https://go.dev/), using [Terma](https://github.com/darrenburns/terma),
a library for building terminal user interfaces. Posting 2 was built in Python with
[Textual](https://textual.textualize.io/).

### Who is the original creator of Posting?

Posting was originally created by [Darren Burns](https://github.com/darrenburns), an open-source developer from Scotland, UK.
