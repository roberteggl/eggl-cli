# `eggl empty`

Create an empty Git commit. This is useful for retriggering a CI pipeline without changing tracked files.

```bash
eggl empty
eggl empty -m "ci: retrigger"
eggl empty --push
eggl empty -p
```

| Flag | Description |
| --- | --- |
| `-m, --message` | Commit message (default: `chore: empty commit`) |
| `-p, --push` | Push the new commit after creating it |

The command runs in the current repository and reports the commit hash. With `--push`, the configured Git remote and credentials are used.
