## Overview

Run a coding agent such as Claude Code, Codex, Pi or Amp in one terminal pane and Posting in
another, and the agent can call your API through Posting while you watch. Each request it sends
opens and sends in your Posting window, using your active environment, and lands in your History.
The agent gets the response back to work with.

The agent does this with [`posting remote`](./remote_control.md). To make sure it knows to, give it
the `using-posting` skill. The skill tells the agent when to check for a running Posting, how to
send requests through it, and when not to, for example for polling loops or risky requests to a
shared environment.

You don't need to keep Posting open for an agent to use your saved requests. The same skill teaches
it to use [`posting run`](./running.md) without a running instance:

```bash
posting run api/health.posting.yaml --env dev --json
posting run api --env dev --json
```

The agent gets response statuses, headers and bodies to inspect, using the authentication and
variables already defined in your files. This doesn't open the UI or write to its history.
Ask for headless execution to use `run` directly, even if a Posting instance is open.

## Installing the skill

`posting remote skill` prints the skill, so you can write it wherever your agent looks for skills.
It doesn't need Posting to be running. After you upgrade Posting, run the install command again to
get the matching version of the skill.

The skill is also in the Posting repository, at
[`skills/using-posting/SKILL.md`](https://github.com/darrenburns/posting/blob/main/skills/using-posting/SKILL.md).

### Claude Code

```bash
mkdir -p ~/.claude/skills/using-posting
posting remote skill > ~/.claude/skills/using-posting/SKILL.md
```

Claude Code uses the skill when a task calls for it. You can also start it yourself with
`/using-posting`.

To let Claude Code run `posting remote` without asking each time, add this to your
`~/.claude/settings.json`, or to `.claude/settings.json` in a project:

```json
{
  "permissions": {
    "allow": ["Bash(posting remote:*)"]
  }
}
```

### Codex

```bash
mkdir -p ~/.agents/skills/using-posting
posting remote skill > ~/.agents/skills/using-posting/SKILL.md
```

Restart Codex to pick up a new skill. `/skills` lists the skills it found, and you can mention
the skill in a prompt with `$using-posting`.

### Pi

Pi reads the same folder as Codex, so the Codex command above installs it for both:

```bash
mkdir -p ~/.agents/skills/using-posting
posting remote skill > ~/.agents/skills/using-posting/SKILL.md
```

Pi picks up skills when it starts. Use `/skill:using-posting` to load the skill yourself.

### Amp

```bash
mkdir -p ~/.config/agents/skills/using-posting
posting remote skill > ~/.config/agents/skills/using-posting/SKILL.md
```

Amp also reads `~/.agents/skills`, so the Codex command above works for Amp too.

### For everyone working on a project

To share the skill with everyone who works on a repository, commit it there. Codex, Pi and Amp read
`.agents/skills`, and Claude Code reads `.claude/skills`:

```bash
mkdir -p .agents/skills/using-posting .claude/skills/using-posting
posting remote skill > .agents/skills/using-posting/SKILL.md
posting remote skill > .claude/skills/using-posting/SKILL.md
```

## Getting agents to use it

An agent loads a skill when it decides the task calls for it, which it usually will when you ask it
to call or test an API. To make Posting the default for a project, add a line to the instructions
file your agent reads (`CLAUDE.md` for Claude Code; `AGENTS.md` for Codex, Pi and Amp):

```markdown
When Posting is open, send requests to this project's API through it with `posting remote`
(see the using-posting skill), so I can see them.
```

When you want it for one task, say so: "send it through Posting", or invoke the skill by name.

## Troubleshooting

**The agent says no Posting is running, but one is.** The agent may be running commands in a
sandbox that sees a different temporary directory, where it doesn't find Posting's socket. Run
`posting remote list --json` in your own terminal, then give the agent the socket path from it,
either with `--instance` or by starting the agent with `POSTING_INSTANCE` set:

```bash
POSTING_INSTANCE=/tmp/posting-501/12345.sock codex
```

If the sandbox blocks the connection itself, approve running `posting remote` outside the sandbox
when the agent asks.

**The agent uses curl instead.** Check that the skill is installed (`/skills` in Claude Code and
Codex), add the instruction above to `CLAUDE.md` or `AGENTS.md`, or ask for Posting by name.
