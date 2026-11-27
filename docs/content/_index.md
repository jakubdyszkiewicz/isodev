---
title: Isodev
layout: hextra-home
---

<div style="display: flex; flex-wrap: wrap; align-items: center; gap: 3rem; width: 100%; margin-bottom: 4rem;">
<div style="flex: 1 1 420px; min-width: 0;">

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-badge link="https://github.com/jakubdyszkiewicz/isodev/releases" >}}
  <span>v0.1.0 · Open source, Apache-2.0</span>
  {{< icon name="arrow-circle-right" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-headline >}}
  One container per Git worktree for parallel work
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-10">
{{< hextra/hero-subtitle >}}
  Run development environments and coding agents side by side on the same repository, each with its own filesystem, network and processes. Your files stay on the host.
{{< /hextra/hero-subtitle >}}
</div>

<div style="display: flex; flex-wrap: wrap; gap: 0.75rem; margin-bottom: 1.5rem;">
{{< hextra/hero-button text="Get started" link="docs/getting-started" >}}
{{< hextra/hero-button text="Browse the guides" link="docs/guides" style="background: transparent; color: inherit; box-shadow: inset 0 0 0 1px rgba(127,127,127,.4);" >}}
</div>

<div style="max-width: 36rem;">

```sh
brew install jakubdyszkiewicz/tap/isodev           # macOS
mise use --global github:jakubdyszkiewicz/isodev   # Linux and macOS
```

</div>

</div>
<div style="flex: 1 1 460px; min-width: 0;">
  <img src="images/terminal.svg" alt="Two Git worktrees each get their own container with isodev up, an agent runs in one of them, and isodev ps lists the containers." style="width: 100%; border-radius: 14px; box-shadow: 0 25px 50px -12px rgba(15, 23, 42, 0.45);" />
</div>
</div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="A container per worktree"
    icon="cube"
    subtitle="Each worktree gets its own container and network. Run servers on the same ports, install anything, break things: the other worktrees don't notice."
    link="docs/how-it-works"
    style="background: radial-gradient(ellipse at 50% 80%,rgba(56,189,248,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Agents inside"
    icon="sparkles"
    subtitle="Run Claude Code, Codex and OpenCode in the container, with your logins and settings. Paseo and Zed start the containers for you."
    link="docs/integrations"
    style="background: radial-gradient(ellipse at 50% 80%,rgba(168,85,247,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Git that just works"
    icon="code"
    subtitle="Linked worktrees work in the container: mount the main repository, or swap in an independent one and get its commits back."
    link="docs/git-worktrees"
    style="background: radial-gradient(ellipse at 50% 80%,rgba(52,211,153,0.15),hsla(0,0%,100%,0));"
  >}}
  {{< hextra/feature-card
    title="Your environment, everywhere"
    icon="adjustments"
    subtitle="One config for all projects: your image, mounts and scripts. Project tools come from mise."
    link="docs/configuration"
  >}}
  {{< hextra/feature-card
    title="Batteries in the guides"
    icon="puzzle"
    subtitle="Docker-in-Docker, Tailscale, GitHub tokens, shared caches and dotfiles, each with a tested config to copy."
    link="docs/guides"
  >}}
  {{< hextra/feature-card
    title="One binary"
    icon="lightning-bolt"
    subtitle="A single Go binary with no dependencies besides Docker. Native speed on Linux, no VM per workspace."
    link="docs/alternatives"
  >}}
{{< /hextra/feature-grid >}}

<div style="width: 100%; text-align: center; margin: 5rem 0 3rem;">
  <p style="font-size: 1.5rem; font-weight: 600; margin-bottom: 0.5rem;">Works with the tools you already use</p>
  <p style="opacity: 0.65;">Claude Code · Codex · OpenCode · Paseo · Zed · Docker · Tailscale · mise · GitHub</p>
</div>
