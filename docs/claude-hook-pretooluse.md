# The `PreToolUse` hook

See official [Hooks reference](https://code.claude.com/docs/en/hooks)for full lifecycle.

## What it is

A `PreToolUse` hook is a user-defined command that Claude Code runs **before a tool call executes**, giving it a chance to allow, block, or defer that call.

It fires on every tool invocation in the agentic loop whose tool name matches the hook's `matcher` eg. `Bash`.

The hook can
- *approve* a call outright (skipping the prompt)
- *block* it
- *fall through* — stay silent and let Claude Code's normal permission flow decide

Fall-through is the safe default: staying silent never grants anything, it only
declines to decide.

## Registration

Hooks live in a `settings.json` under `hooks.PreToolUse`, each block a `matcher` plus one or more command hooks. Example:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{ "type": "command", "command": ".claude/hooks/my-hook.sh" }]
      }
    ]
  }
}
```

The matcher is a regex against the tool name (`Bash`, `Edit|Write`, `mcp__.*`, …).

## Multiple hooks

All matching hooks run **in parallel**. There is no reliable ordering.
- Identical handlers are **deduplicated** (command hooks by command string + `args`, HTTP hooks by URL).
- Each hook has a per-hook `timeout` (default 600s for command hooks).

Merge rules:
- **Decision**: most restrictive wins — `deny` > `defer` > `ask` > `allow`.
- **`updatedInput`**: last hook to finish wins — **non-deterministic**. Don't let two hooks rewrite the same field.
- **`additionalContext`**: all hooks' context is kept.

## Input (via stdin)

Claude Code sends the hook a JSON payload on `stdin`:

```json
{
  "session_id": "abc123",
  "transcript_path": "/home/user/.claude/projects/.../transcript.jsonl",
  "cwd": "/home/user/my-project",
  "permission_mode": "default",
  "hook_event_name": "PreToolUse",
  "tool_name": "Bash",
  "tool_input": {
    "command": "npm test"
  }
}
```

`tool_input` varies per tool; for `Bash` it carries `command`. A hook typically
reads just the fields it cares about and ignores the rest.

## Output: two ways to decide

A `PreToolUse` hook communicates its decision through **exit code** and, for a
positive decision, a **JSON object on stdout**.

### Exit codes

- `0` Success. With two cases based on stdout outout
  - no JSON: falls through to the normal permission flow
  - if valid JSON: see "Decision JSON"
  - if invalid JSON: goes to debug log
- `2` Blocking error; the tool call is blocked.
  - stdout is ignored and
  - **stderr** is shown to Claude as the block reason.
- other:  Non-blocking error; the call proceeds anyway, with stderr surfaced as a notice.

### Decision JSON

On exit 0, stdout carries a `hookSpecificOutput` object:

```json
{
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",
    "permissionDecisionReason": "…",
    "updatedInput": { "command": "…" },
    "additionalContext": "…"
  }
}
```

`permissionDecision` values:

- `allow` Approve call, skip user confirmation
- `deny` Block call; `permissionDecisionReason` is shown to Claude
- `ask` Escalate to the user's permission dialog (as if no hook decided)
- `defer` Decline to decide; other hooks / the permission flow decide

Optional fields:
- `permissionDecisionReason` Recommended with `deny`
- `updatedInput` Replace the tool's arguments before it runs; not used with `deny`
- `additionalContext` Inject extra context

