# Development

**Requirements**

- [go](https://go.dev/) — provided by the Nix flake devshell (`nix develop`), or
  `brew install go`
- [staticcheck](https://staticcheck.dev/) `go install honnef.co/go/tools/cmd/staticcheck@latest`

**Debugging the hook**

Inspect the sub-commands the hook would extract from a command:

```sh
echo 'cat foo | grep bar && echo done' | applaude parse
```

prints one sub-command per line. Add `--debug` to the hook invocation to trace
decisions on stderr.

**Commands** (via [Task](https://taskfile.dev/))

- `task run` Run
- `task build` Build project → `./applaude`
- `task test` Run tests
- `task tidy` Ensure all imports are satisfied
- `task lint` Lint (staticcheck + tidy)
- `task install` Install app in `$GOBIN/`
- `task uninstall` Remove app from `$GOBIN/`
- `task artifacts` Produce release artifacts in `./.release` (darwin/linux, arm64/amd64)
- `task tag` Push git tag from `VERSION`
- `task release` Create GitHub release from artifacts
- `task sha` Print artifact hashes
- `task clean` Remove build artifacts
- `task updates` Find dependency updates
