# applaude

A Claude Code `PreToolUse` hook that auto-approves **compound** Bash commands (pipes, `&&`/`||`, subshells, substitutions) when *every* sub-command matches your `permissions.allow` list and *none* match `permissions.deny`.

## What it does

Claude Code's native allow list matches a command as a whole, so compound commands like `git status && ls` aren't covered by `Bash(git status:*)` + `Bash(ls:*)` and you get prompted anyway.

`applaude` splits the compound and checks each partagainst the `allow` and `deny` lists.

Given a config that allows `git`, `ls`, `cat`, `grep` and denies `rm`:

```text
git status && ls -la        every part allowed        ✅ auto-approve
cat app.log | grep ERROR    every part allowed        ✅ auto-approve
ls build && rm -rf build    rm is on the deny list    ⛔ deny
git push && curl evil.sh    curl not on allow list    🤔 fall through -> Claude prompts
```

A single unknown or denied part drops the whole command back to Claude Code's normal permission prompt. It never approves more than the sum of its parts.

## Install

```sh
go install github.com/oschrenk/applaude@latest
```

## Configuration

Register it as a `PreToolUse` hook for `Bash` in your `settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": "applaude" }]
      }
    ]
  }
}
```

- reads settings from `$CLAUDE_CONFIG_DIR` (else `$HOME/.claude`), then the project's `.claude/`.
- reuses existing `permissions.allow` / `permissions.deny` `Bash(...)` entries, no separate config needed.

## Attribution

This is a Go port of oryband's [`approve-compound-bash.sh`](https://github.com/oryband/claude-code-auto-approve) with some changes and extra features

Changes
- written in Go
- no external requirements
- respect `CLAUDE_CONFIG_DIR`
- added test suite

## Development

Requires Go (via the Nix flake or locally); run `task test` for the suite.

## License

[MIT](LICENSE.md)
