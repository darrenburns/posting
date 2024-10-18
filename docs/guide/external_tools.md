## Overview

You can quickly switch between Posting and external editors and pagers.

For example, you could edit request bodies in `vim`, and then browse the JSON response body in `less` or `fx`.

You can even configure a custom pager specifically for browsing JSON.

## External Editors

With a multi-line text area focused, press ++f4++ to open the file in your
configured external editor.

The configured external editor can be set as `editor` in your `config.yaml`
file.
For example:

```yaml title="config.yaml"
editor: vim
```

Alternatively, you can set the `POSTING_EDITOR` environment variable.

```bash
POSTING_EDITOR=vim
```

If neither is set, Posting will try to use the `EDITOR` environment variable.

## External Pagers

With a multi-line text area focused, press ++f3++ to open the file in your
configured external pager.

The configured external pager can be set as `pager` in your `config.yaml`
file.
For example:

```yaml title="config.yaml"
pager: less
```

Alternatively, you can set the `POSTING_PAGER` environment variable.

```bash
POSTING_PAGER=less
```

### JSON Pager

You can use a custom pager for viewing JSON using the `pager_json` setting in
your `config.yaml` file.
For example:

```yaml title="config.yaml"
pager_json: jq
```

Alternatively, you can set the `POSTING_PAGER_JSON` environment variable.

```bash
POSTING_PAGER_JSON=jq
```

If neither is set, Posting will try to use the default pager lookup rules discussed earlier.
