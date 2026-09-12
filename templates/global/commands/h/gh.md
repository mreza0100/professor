---
name: h:gh
description: Host-local `gh` CLI, installed and authenticated on this machine — use for any GitHub operation (PRs, issues, releases, repo API), the /pfm:release publish included.
---

# GH — GitHub CLI Bridge

Host fact, not project prose — a machine-scope command, shared across every project on this host, not a per-repo copy. Assumes the `gh` CLI is installed and authenticated; if it isn't, say so and stop rather than guessing at credentials.
