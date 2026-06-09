---
description: |
  Bump Go to a new minor version across the entire project.
  Use when upgrading Go (e.g. from 1.25 to 1.26).
  Handles go.mod, Makefile, Dockerfile, Tiltfile, netlify.toml,
  GitHub Actions, cloudbuild, golangci-lint, and delve.
argument-hint: "<go-minor-version> (e.g. 1.26)"
allowed-tools: Bash(go *) Bash(git *) Bash(grep *) Bash(find *) Bash(docker *) Bash(gh *) Bash(make *)
---

# Bump Go Version

Target Go minor version: **$ARGUMENTS**

Follow the instructions in [docs/book/src/developers/bump-go.md](../../../docs/book/src/developers/bump-go.md) exactly, using `$ARGUMENTS` as the target Go minor version.
