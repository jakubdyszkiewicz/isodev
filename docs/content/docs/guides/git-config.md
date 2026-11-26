---
title: Git config
weight: 5
---

Use your host Git config in the container: your identity, settings and global ignore file.

## Set up

Use the files in [`examples/guides/git-config`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/git-config):

{{< example-file path="guides/git-config/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/git-config/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/git-config/hooks/git.sh" lang="sh" name="~/.config/isodev/hooks/git.sh" >}}

Your `~/.gitconfig` must exist on the host. The container includes it instead of copying it, so changes on the host apply right away, and the container can still add its own settings.

To commit as someone else, for example a bot account, uncomment the identity in `hooks/git.sh`. For a GitHub App's bot identity, see [GitHub authentication](../github).

### Global ignore file

The container uses your host's global ignore file, so files you never commit, such as Paseo's `paseo.json`, stay out of commits in the container too. Mount the file your host Git uses: `git config --global core.excludesFile` prints it, and Git uses `~/.config/git/ignore` when that's unset. `hooks/git.sh` points `core.excludesFile` at the mounted copy, because the host's path may not exist in the container.

The file is mounted on its own, so if your editor saves it by replacing the file, containers see the change after `isodev up --recreate`. If you don't use a global ignore file, remove its mount and its line in `hooks/git.sh`.

## Use it

```sh
isodev up --recreate
isodev exec git config user.name
```

## Trade-offs

- **Host settings must work in the container.** Settings that point to host paths or programs, such as a credential helper, may not resolve there.
