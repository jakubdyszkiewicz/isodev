---
title: Alternatives
weight: 10
---

Isodev overlaps with two kinds of tools: **sandboxes**, which limit what an agent can do, and **Dev Containers**, which give a project a reproducible environment. They make different trade-offs.

{{< cards >}}
  {{< card link="sandboxes" title="Sandboxes" subtitle="MicroVMs, such as Docker Sandboxes." >}}
  {{< card link="dev-containers" title="Dev Containers" subtitle="The Dev Container specification and the tools that implement it." >}}
{{< /cards >}}

| | Isodev | Docker Sandboxes | Dev Containers |
| --- | --- | --- | --- |
| Boundary | Container | microVM | Container |
| Configured | Once, for all projects | Per sandbox, with kits | Per repository |
| Environment | Your image and hooks | Kits | Image, Dockerfile and Features |
| Network | One per container | Through a proxy on the host | Docker's default network, unless the project sets one |
| Network egress control | No | Yes | No |
| Git worktrees | Main repository mounted, or worktree replaced with an independent repository | Private clone with `--clone` | Main repository mounted, experimental |

Products in this space change quickly. These pages were last checked against each product's documentation in November 2026.
