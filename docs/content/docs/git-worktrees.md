---
title: Git worktrees
weight: 6
---

A linked Git worktree has a `.git` file that points into the main repository's Git directory, which Git calls the *common directory*. A container that only mounts the worktree doesn't have that directory, so Git doesn't work in it. `git_worktree.mode` chooses what Isodev does about it:

```yaml {filename="~/.config/isodev/config.yaml"}
git_worktree:
  mode: replace # or mount-common-dir, or none
```

| Mode | Git in the container | Commits reach the main repository | The container can write to |
| --- | --- | --- | --- |
| `none` | Doesn't work | Only if you commit on the host | The worktree |
| `mount-common-dir` | Works | Immediately | The worktree and the main repository's `.git` |
| `replace` | Works | When you fetch them, and on `isodev down` | The worktree |

The mode applies when a container is created. To switch modes, change it and run `isodev up --recreate`. If the worktree was replaced and the new mode isn't `replace`, Isodev restores it first, as `isodev down` does.

## none

Isodev mounts only the worktree. Use it when you don't need Git in the container, for example when you commit from the host.

## mount-common-dir

Isodev mounts the main repository's Git directory, and the workspace, at their host paths, so the worktree's `.git` file resolves in the container. Commits, branches and stashes made in the container go straight into the main repository. Because both are at their host paths, this works with ordinary worktrees; worktrees with relative paths aren't needed.

- **The container can write to the whole repository:** every branch, the config and `.git/hooks`. A hook written in the container runs on the host the next time you run Git there.
- **`git worktree prune` in the container deletes other worktrees' registrations**, because their directories don't exist in the container. Isodev sets `gc.worktreePruneExpire=never` in the container, so automatic `git gc` doesn't prune them, but an explicit `git worktree prune` still does. The setting is passed with `GIT_CONFIG_COUNT`; if `environment` sets `GIT_CONFIG_COUNT`, Isodev leaves it out.
- The workspace is mounted at its host path instead of `/workspaces/NAME`.

It's less secure than `replace`, but more convenient.

## replace

Before creating the container, Isodev replaces the worktree's `.git` file with an independent repository, so Git works in the container without access to the main repository. `isodev down`, and Paseo and Zed teardown, restore it.

### Replace

The independent repository keeps the worktree's files, branch and commit.

- The worktree must have no tracked changes; untracked files are kept. A detached HEAD, as Zed creates, is kept.
- The main checkout's cached `origin/*` refs and the branch's upstream are copied without contacting the remote. Run `git fetch origin` to refresh them.
- The main repository's `info/exclude` is copied, so locally ignored files stay ignored.
- The original worktree registration is kept, so the link can be restored. How to restore it is recorded in Isodev's state directory on the host, `~/.local/state/isodev/worktrees/`, not in the worktree, so the container can't change it.
- The main checkout gets a remote, `isodev-NAME`, pointing at the worktree. `NAME` is the worktree's name in the main repository, the directory under `.git/worktrees/`.

### Get the commits

While the worktree is replaced, fetch its commits in the main checkout at any time:

```sh
git fetch isodev-NAME
git log isodev-NAME/BRANCH
```

This works while the container is stopped, because the worktree's repository is on the host.

### Restore

Restoring fetches the worktree's branches into `isodev-NAME/BRANCH` refs in the main checkout, and its HEAD into `isodev-NAME/HEAD`, so no commit is lost when the worktree is deleted. It then restores the original link and saves the independent repository in a new directory in `.isodev-git-archive/` in the workspace.

- If the worktree is still on its original branch, or still detached, and its new commits extend the original commit, the branch is fast-forwarded to them. The worktree keeps its staged changes, so `git status` looks the same as in the container.
- Otherwise the branch stays where it was, and Isodev warns. The commits are in `isodev-NAME/BRANCH`, or `isodev-NAME/HEAD` for a detached HEAD, to merge yourself.
- The `isodev-NAME` remote is removed, but its refs stay. Delete them with `git branch -rd isodev-NAME/BRANCH` when you no longer need them.
- `.isodev-git-archive` is added to the repository's `info/exclude`, so it isn't committed.
- Restoring reads the worktree's repository only through Git's transport, like a fetch, and doesn't run Git commands in it, so configuration the container wrote there doesn't run on the host.

### By hand

The same steps are available without containers:

```sh
isodev int git-worktree replace --worktree-path PATH
isodev int git-worktree restore --worktree-path PATH
```

`replace` finds the main checkout from the worktree; pass `--source-path` to choose it. Unlike `isodev up`, it requires the worktree to be on a branch.
