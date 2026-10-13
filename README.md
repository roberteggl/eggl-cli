# eggl-cli

[![Release](https://img.shields.io/github/v/release/roberteggl/eggl-cli?style=flat-square)](https://github.com/roberteggl/eggl-cli/releases)
[![CI](https://github.com/roberteggl/eggl-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/roberteggl/eggl-cli/actions/workflows/ci.yml)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=roberteggl_eggl-cli&metric=coverage)](https://sonarcloud.io/summary/new_code?id=roberteggl_eggl-cli)
[![Quality gate status](https://sonarcloud.io/api/project_badges/measure?project=roberteggl_eggl-cli&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=roberteggl_eggl-cli)
[![Go](https://img.shields.io/github/go-mod/go-version/roberteggl/eggl-cli?style=flat-square)](https://go.dev/)
[![License](https://img.shields.io/github/license/roberteggl/eggl-cli?style=flat-square)](LICENSE)

eggl-cli is a general-purpose helper built with Go and [Cobra](https://github.com/spf13/cobra).

## Install

**Homebrew:**

```bash
brew tap roberteggl/tap
brew install eggl-cli
```

Shell completions for bash, zsh, and fish are installed automatically.

**Without Homebrew:** download the archive for your OS/arch from [GitHub Releases](https://github.com/roberteggl/eggl-cli/releases), extract `eggl` (or `eggl.exe` on Windows), and put it on your `PATH`. Or, with Go installed:

```bash
go install github.com/roberteggl/eggl-cli@latest
```

From source:

```bash
make install
```

Or build a local binary:

```bash
make build
./bin/eggl --help
```

## Usage

```bash
eggl --help
eggl version
eggl version --short
eggl doctor
eggl doctor --verbose
eggl env init
eggl env show
eggl env toggle
eggl pf list
eggl pf longhorn
eggl cd list
eggl cd homelab
eggl completion zsh
```

### Environment profiles (`eggl env`)

Switch kubectl context and Tailscale account together from `~/.config/eggl/config.yaml`:

```yaml
profiles:
  alpha:
    kube_context: context-a
    tailscale_account: b3e1
  beta:
    kube_context: context-b
    tailscale_account: a7f2
```

`tailscale_account` can be an account id, tailnet name, or email from `tailscale switch --list`. `toggle` requires exactly two profiles.

### Port-forwards (`eggl pf`)

Named kubectl port-forwards in the same config file (uses the active kubectl context):

```yaml
port_forwards:
  longhorn:
    namespace: longhorn-system
    resource: svc/longhorn-frontend
    ports: ["8080:80"]
```

`ports` is optional (defaults to `8080:80`). Run `eggl pf list` or `eggl pf longhorn` (blocks until Ctrl+C).

### Quick cd (`eggl cd`)

Named directories in the same config file. The command prints the resolved path for use in your shell:

```yaml
directories:
  homelab: ~/projects/homelab
  work: /Users/me/code/work
```

```bash
cd "$(eggl cd homelab)"
```

Optional shell wrapper:

```bash
egglcd() { cd "$(eggl cd "$1")" || return; }
```

## Shell completion

If you installed via Homebrew, completions are set up for you. Otherwise:

**Zsh:**

```bash
eggl completion zsh > "${fpath[1]}/_eggl"
```

**Bash:**

```bash
eggl completion bash > /etc/bash_completion.d/eggl
# or for the current session:
source <(eggl completion bash)
```

**Fish:**

```bash
eggl completion fish > ~/.config/fish/completions/eggl.fish
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, project layout, and release instructions.

Built by [Robert Eggl](https://eggl.dev).
