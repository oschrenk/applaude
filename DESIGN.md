# Design

## Motivation

We don't want to just do `--dangerously-skip-permissions` and approves *everything* — `rm -rf /`, `curl … | sh` included.

But approving every Bash command by hand is tedious but. Claude ofers default `allow` and `deny` lists, but they have litte (or overreaching) expressive power.

We need something that can leverage existing primitives but can also deal with more complex commands.

Core invariant:
- **auto-allow may only approve a command whose side-effects are fully captured by the prefix(es) it matched.**

The problems can be categorized into:

- **Compound commands** — `git status && ls | grep x`: parts allowed, whole isn't.
- **Wrapper transparency** — `nohup rm -rf /` hides the inner `rm`.
- **Multi-action runners** — `mvn test deploy`: `deploy` rides in on `mvn test:*`.
- **Redirection** — `echo x > ~/.ssh/authorized_keys` writes anywhere.
- **Process substitution** — `cmd >(evil)`: same write hazard.
- **Write/exec flags** — `sort -o FILE`, `rg --pre CMD`: read-only tool, isn't (best-effort).

## Problem

### Compound commands

`git status && ls | grep x` — every part is allow-listed, but the whole string isn't, so nothing matches and you're prompted anyway.

### Wrapper transparency

`nohup rm -rf /` is emitted as one command, so we can't just allow `nohup *` as the dangerous inner `rm -rf /` is never checked on its own.

There are many commands like this.

### Multi-action runners

`Bash(mvn test:*)` would also match `mvn test deploy`and would silently authorize the `deploy` command.

Other examples in this problem category:
- `make`, `mvn`, `npm run`, `gradle`, `just`, `bazel`and more

### Redirection

`echo pwned > ~/.ssh/authorized_keys`

Redirection (`>`, `>>`, `2>`, `&>`, `<<`, `<<<`, `>(…)`) turns any read-only command into an arbitrary-file writer. `Bash(...)` entry can't express "no redirection."

### Process substitution

`cmd >(evil)` — write process substitution is the same write hazard as redirection.

### Write/exec flags

`sort -o FILE` writes; `sort --compress-program=CMD`, `rg --pre CMD`, `git diff --ext-diff` exec.

"Read-only" is a property of a command *and its flags*, not of the command name.

## Solution Space

### Compound commands — split and check

Split `a && b | c` into sub-commands, match each against `permissions.allow` / `permissions.deny`, approve only if all parts pass.

### Wrapper transparency

Allowing a wrapper like `nohup` shouldn't approve what it runs. It should just tell the parser to strip `nohup`, recurse, and check the inner command. For example `nohup rm -rf /` would become `rm -rf /`, which still has to be in the allow list on its own.

We can leverage the wildcark marker (or it's absence to apply some meaning):
- `Bash(nohup)` — **transparent**: strip `nohup`, recurse; inner must pass alone.
- `Bash(nohup:*)` — **blanket**: trust `nohup` + any args, no unwrapping.

Transparency requires **both**: a known wrapper token **and** the user opting in by omitting `:*`.

That way the user can leverage the normal `settings.json` to control the behaviour

An example of simple "name-only" wrapping commands are
- `nohup`, `setsid`, `time`, `command`

"Name-only" wrapper should nest for free eg. `nohup timeout 5 bash -c '…'`;
- each layer needs has own entry.
- a recursion cap (eg 5) is needed, so that `env env env …` isn't an issue

But there are more complicated commmands
- `stdbuf -oL` `nice -n 10` — name + dash-options.
- `timeout 5s` — options + **one** duration.
- `flock /tmp/x` — options + **one** path.
- `env VAR=val` — options + `VAR=val` assignments.

For those no clear path is known and further exploration is needed. and if so how

#### Never transparent

Some commands are wrapper-shaped but change semantics — require an explicit `:*` blanket if wanted:

- **`sudo` / `doas` / `su -c`** — escalation deserves an explicit decision.
- **`xargs cmd`** — inner runs with attacker-controlled stdin args.
- **remote/container/VM** — `ssh host cmd`, `docker run/exec`, `kubectl exec`,
  `find -exec` — inner runs in a *different* context; the local allow list is meaningless.
- **`env LD_PRELOAD=… / DYLD_* / PATH= / IFS=`** — inject code past a benign inner
  command. (`StripEnvVars` strips *any* `VAR=` blindly today — a live gap. Fix: a
  denylist of dangerous assignment names.)

### Multi-action runners

Further exploration is needed

### Redirection

Any redirection should be `FallThrough` and should prompt the human

### Process substitution

`$(…)` inner commands should be checked.

### Write/exec flags

The dangerous flag can sit at any position in the argv, so prefix matching
can't see it: `Bash(find -exec)` never matches `find . -type f -exec …` because
`-exec` isn't at the prefix.

**Solution: token-membership deny.** Match a deny entry against the command's
argv tokens, not as a prefix — `argv[0]` must equal the entry's first token,
and every remaining entry token must appear somewhere in the argv (unordered).
The user expresses danger with ordinary `settings.json` deny entries
(`Bash(find -exec)`); no per-tool table. Allow stays prefix-only. Token
membership is a strict superset of prefix, so it only ever widens a deny —
sending a simple command to the human prompt or blocking a compound, never
auto-approving. Anchoring `argv[0]` as a whole token avoids substring hits
(`Bash(rm)` ≠ `echo remove`). Residual over-match — a positional value that
literally equals a flag (`find . -name -exec`) — falls through to a human
rather than chasing per-tool getopt grammar. This is the same problem as the
`find -exec` entry under [Never transparent](#never-transparent).

Examples where an allow-listed "read-only" tool writes or execs via a flag:

Writes a file:

- `sort -o FILE` / `--output FILE`
- `sed -i` (in-place edit)
- `find … -fprint FILE` / `-fprintf FILE …`
- `awk '{ print > "FILE" }'`
- `curl -o FILE` / `wget -O FILE`

It is unclear yet if the user should whitelist these commands themselves and if so how

Executes a command:

- `find … -exec CMD {} \;` / `-execdir`
- `sort --compress-program=CMD`
- `rg --pre CMD` / `--search-zip` (spawns a decompressor)
- `git diff --ext-diff` (external diff from repo config) / `git -c core.pager=CMD`
- `awk 'BEGIN{system("CMD")}'` / GNU `sed`'s `e` command
- `tar -I CMD` / `--use-compress-program=CMD` / `--to-command=CMD`
- `ssh -o ProxyCommand=CMD` / `rsync -e CMD`

## Implementation notes

1. **Preserve the wildcard bit.** `ExtractPrefix` returns a string today; make it
   return whether the entry had `:*`. Prefixes become `{prefix string; exact bool}`,
   rippling through `MatchesPrefix` / `IsAllowed` / `hook.go` + tests.
2. **Branch on the bit.** exact non-wrapper → equality; exact registry wrapper →
   strip-and-recurse; wildcard → today's prefix logic.

Ideally the three strip-and-recurse mechanisms (`shellCRe`, `StripEnvVars`, the
wrapper registry) collapse into one "prefix transform" table applied at a single
point in the walk, differing only in their stripper.
