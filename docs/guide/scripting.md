## Overview

Posting 2 could run Python scripts before a request was sent and after its response arrived, to
set variables, change the request, or log information.

**Posting 3 doesn't run scripts.** It's a single binary written in Go, with no Python inside it to
run them with.

## Requests with scripts

Requests saved by Posting 2 can refer to scripts in their `scripts` section:

```yaml
scripts:
  setup: scripts/setup.py
  on_request: scripts/auth.py:sign_request
  on_response: scripts/extract_token.py
```

Posting 3 opens these requests and sends them without running the scripts. The `scripts` section
is kept when you save the request, so you can go on using the same collection with Posting 2
without losing anything.

If you rely on scripts, Posting 2 remains available and reads the same collections, so you can use
both side by side.

## Alternatives

Some common uses of scripts can be handled without them in Posting 3:

| With a script, you'd… | In Posting 3 |
|-----------------------|--------------|
| Set a variable for later requests | Set a session value on the [variables screen](./environments.md#the-variables-screen) (++ctrl+shift+v++) |
| Choose values per environment | Use [layered environment files](./environments.md#layered-environments) |
| Read secrets from somewhere else | Refer to shell variables from your environment files, e.g. `API_TOKEN="${STAGING_TOKEN}"`, or turn on [`use_host_environment`](./environments.md#using-your-shells-environment) |
| Add authentication | Use the [Auth tab](./requests.md#auth) with variables, e.g. a Bearer token of `${API_TOKEN}` |
| Keep a copy of responses | Every response is kept in [history](./collections.md#history) |

Scripting may return in a future version. See the [roadmap](../roadmap.md).
