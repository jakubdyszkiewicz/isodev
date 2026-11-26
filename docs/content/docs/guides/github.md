---
title: GitHub authentication
weight: 4
---

Let `gh` and `git push` over HTTPS authenticate with a GitHub token that you choose, instead of your own credentials.

The token lives in a file on the host. A small `gh` wrapper reads it on every call, and Git uses `gh` as its credential helper, so both work in `isodev exec`, agents, hooks and `isodev enter` shells, and a replaced token takes effect immediately.

## Set up

Use the files in [`examples/guides/github`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/github). The wrapper must be executable; copying the directory keeps its mode, otherwise run `chmod +x ~/.config/isodev/bin/gh`.

{{< example-file path="guides/github/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/github/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

{{< example-file path="guides/github/bin/gh" lang="sh" name="~/.config/isodev/bin/gh" >}}

The configuration mounts the token's directory rather than the file: tools that replace the token atomically create a new file, which a single-file mount would not see.

## Provide a token

Write the token to `~/.config/isodev/github-token/token`, readable only by you:

```sh
mkdir -p ~/.config/isodev/github-token
chmod 700 ~/.config/isodev/github-token
cat > ~/.config/isodev/github-token/token   # Paste the token, then press Ctrl-D
chmod 600 ~/.config/isodev/github-token/token
```

{{< tabs >}}
  {{< tab name="Personal access token" >}}
Create a [fine-grained personal access token](https://github.com/settings/personal-access-tokens) limited to the repositories and permissions the containers need, for example **Contents** and **Pull requests** with read and write access. Commits you push and pull requests you open are attributed to you.
  {{< /tab >}}
  {{< tab name="GitHub App" >}}
[Create a GitHub App](https://docs.github.com/en/apps/creating-github-apps) with the repository permissions the containers need, install it on those repositories, and generate a private key.

Isodev doesn't create the token. You need a process on the host that exchanges the app's private key for an [installation access token](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app) and writes it to the token file. Installation tokens expire after an hour, so the process must refresh the file before then, replacing it atomically.

Pull requests and comments are attributed to the app's bot account. To attribute commits to it too, set the bot's identity in an `on_create` script, with the bot's user ID from `gh api /users/APP-SLUG%5Bbot%5D --jq .id`:

```sh
git config --global user.name 'APP-SLUG[bot]'
git config --global user.email 'USER-ID+APP-SLUG[bot]@users.noreply.github.com'
```
  {{< /tab >}}
{{< /tabs >}}

## Use it

```sh
isodev up --recreate
isodev exec gh auth status
```

## Trade-offs

- **Anything in the container can read the token,** including agents and the tools they run. Grant it only the repositories and permissions the containers need.
