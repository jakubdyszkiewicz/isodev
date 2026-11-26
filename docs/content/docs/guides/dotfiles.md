---
title: Shell and dotfiles
weight: 10
---

Build your dotfiles into the image, so containers have your shell, its configuration and your command-line tools. This guide uses the author's dotfiles, [dotfishy](https://github.com/jakubdyszkiewicz/dotfishy), which set up Fish and tools such as fzf and bat with [mise](https://mise.jdx.dev).

## Set up

Use the files in [`examples/guides/dotfiles`](https://github.com/jakubdyszkiewicz/isodev/tree/main/examples/guides/dotfiles):

{{< example-file path="guides/dotfiles/config.yaml" lang="yaml" name="~/.config/isodev/config.yaml" >}}

{{< example-file path="guides/dotfiles/image/Dockerfile" lang="dockerfile" name="~/.config/isodev/image/Dockerfile" >}}

To use your own dotfiles, replace the repository URL and branch in the `ADD` step, and the `RUN` step with however your dotfiles are installed. A public repository works as it is. For a private one, build the image with access to it, or copy the dotfiles into the build context instead.

`isodev enter` starts Fish when the image has it, otherwise Bash or `sh`.

## Use it

```sh
isodev up --recreate
isodev enter
```

When you push to your dotfiles repository, the next build of the image picks up the change; existing containers keep the old image until you run `isodev up --recreate`.
