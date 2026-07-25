# Product brief

## Product

**Tidy Branches** is a GitHub-aware remote branch cleanup tool.

Canonical repository after rename:

```text
teamleaderleo/gh-tidy-branches
```

Canonical command:

```text
gh tidy-branches
```

## Problem

Repositories with frequent agent-created work accumulate large numbers of remote branches. GitHub's automatic deletion setting helps only with future merges. Local Git cleanup tools do not reliably understand squash merges, rebase merges, cross-repository pull requests, reused branch names, or a fleet of repositories that may not be cloned locally.

The cleanup task is simple in intent and surprisingly easy to make unsafe.

## First-release promise

Tidy Branches identifies a narrow, defensible set:

> Same-repository branches whose pull requests merged directly into the default branch, whose current remote tips still equal the exact pull request head SHAs recorded at merge time, and which are not involved in any open pull request.

The user reviews the full set before mutation unless `--yes` is supplied.

## Users

- developers with many short-lived feature branches
- maintainers with agent-heavy repositories
- people operating several personal repositories
- coding agents that need JSON output and deterministic safety rules
- future editor integrations that need a reusable scan engine

## Product principles

### Evidence before deletion

A merged label is insufficient. Eligibility requires current remote ref evidence and the recorded pull request head SHA.

### Bulk reads before parallel noise

The scanner fetches branch and pull request pages in bulk, then joins locally. Bounded repository concurrency improves latency without turning branch count into branch-count network calls.

### Narrow defaults

Closed-unmerged branches, arbitrary age-based stale branches, local branches, tags, worktrees, and protected long-lived branches are excluded from automatic deletion in the first release.

### CLI first

The reusable product is the scan and safety engine. A VS Code extension can later present the same JSON model.

### Useful from anywhere

Inside a repository, the current repository is selected. Elsewhere, configured repositories are selected. Explicit `owner/repo` arguments always work.

## Success measures

- a repository with hundreds of branches scans with a small number of paginated requests
- squash and rebase merge histories are handled correctly
- reused branches that advanced after merge are preserved
- open pull request head and base branches are preserved
- repeated scans are deterministic
- JSON output is stable enough for editor and agent clients
- mutation remains opt-in and auditable
