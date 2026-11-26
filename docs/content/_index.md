---
title: Isodev
layout: hextra-home
---

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  One container per Git worktree&nbsp;<br class="hx:sm:block hx:hidden" />for parallel work
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Run development environments and coding agents side by side on the same repository,&nbsp;<br class="hx:sm:block hx:hidden" />each with its own filesystem, network and processes.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Get started" link="docs/getting-started" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="A container per worktree"
    subtitle="Each workspace gets its own container and network, built from your image and set up by your hooks. Workspace files stay on the host."
    link="docs/commands"
  >}}
  {{< hextra/feature-card
    title="Independent Git checkouts"
    subtitle="Git works in each worktree's container, with the main repository mounted or the worktree's link replaced and restored when you're done."
    link="docs/git-worktrees"
  >}}
  {{< hextra/feature-card
    title="Paseo and Zed"
    subtitle="Worktree hooks, tasks and agent providers set up each worktree's container and run agents inside it."
    link="docs/integrations"
  >}}
{{< /hextra/feature-grid >}}
