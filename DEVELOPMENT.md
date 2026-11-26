# Development

Isodev is written in Go. [mise](https://mise.jdx.dev) installs the toolchain (Go, golangci-lint, GoReleaser, Hugo and lychee) and defines the common tasks:

```sh
mise install
mise run build                      # Build bin/isodev
mise run test                       # go test ./...
mise run lint                       # golangci-lint, configured in .golangci.yml
mise run check                      # go vet ./... and goreleaser check
mise run generate                   # Regenerate the command and configuration references
mise run docs:serve                 # Serve the documentation site at http://localhost:1313/ with live reload
mise run docs:build                 # Build the documentation site into docs/public/ and check its links
mise run snapshot                   # Build release archives for all platforms into dist/
ISODEV_DOCKER_TEST=1 go test ./...  # Include Docker integration tests
ISODEV_EXAMPLES_TEST=1 go test ./internal/config/  # Build every example image
```

## Code layout

| Package | Responsibility |
| --- | --- |
| `internal/cli` | Cobra commands; each command pairs its action with a `--dry-run` preview. `help.go` holds the help texts |
| `internal/config` | Loading and validating `config.yaml` |
| `internal/container` | Creating, starting, running commands in and removing workspace containers |
| `internal/workspace` | `up` and `down` for the CLI and integrations: `git_worktree.mode` plus the container |
| `internal/integration/gitworktree` | Detecting linked worktrees, and replacing and restoring their links |
| `internal/integration/paseo`, `…/zed` | Paseo hooks and providers; Zed tasks, settings and the ACP bridge |
| `internal/paths`, `shell`, `fsutil` | Home and state directories, running commands, file helpers |
| `internal/testutil` | Isolates tests in a temporary home directory |

## Documentation

The [documentation site](https://jakubdyszkiewicz.github.io/isodev/) is built with [Hugo](https://gohugo.io) and the [Hextra](https://imfing.github.io/hextra/) theme from [`docs/`](docs/), and published to GitHub Pages on every push to `main`.

Two references are generated, so they can't drift from the code:

- The [command reference](https://jakubdyszkiewicz.github.io/isodev/docs/commands/) comes from the command definitions and help texts in [`internal/cli`](internal/cli/).
- The [configuration reference](https://jakubdyszkiewicz.github.io/isodev/docs/configuration/) and JSON Schema come from the field comments in [`internal/config/config.go`](internal/config/config.go).

The guides embed their complete configurations from [`examples/guides`](examples/guides/), and host services from [`examples/services`](examples/services/), so the files on the site are the files in the repository.

After changing either reference, run `mise run generate`. Tests fail when the generated files are out of date, when a command or flag has no description, or when a YAML example in the README, on the documentation site or in `examples/` doesn't load. Example images are only built with `ISODEV_EXAMPLES_TEST=1`, because that downloads a lot; run it after changing a Dockerfile.

## Releases

Releases are built with [GoReleaser](https://goreleaser.com) from [`.goreleaser.yaml`](.goreleaser.yaml). GitHub Actions tests every push and pull request and builds snapshot archives for Linux and macOS on amd64 and arm64, available as workflow artifacts. Pushing a `v*` tag, for example `v1.0.0`, publishes the archives, checksums and generated release notes to GitHub Releases.
