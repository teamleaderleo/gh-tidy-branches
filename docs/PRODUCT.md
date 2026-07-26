# Product brief

## Product

**Tidy Branches** is a GitHub CLI extension for safely cleaning up remote branches after their pull requests merge.

Canonical repository:

```text
teamleaderleo/gh-tidy-branches
```

Canonical command:

```text
gh tidy-branches
```

## Problem

Repositories accumulate short-lived remote branches faster than people clean them up. GitHub's automatic deletion setting helps with future merges, but it does not remove an existing backlog. Local Git cleanup commands also have incomplete information about squash merges, rebase merges, forks, reused branch names, and repositories that are not cloned locally.

The intent is simple—remove finished branch refs—but the evidence required to do that safely is not.

## First-release promise

Tidy Branches identifies a deliberately narrow candidate set:

> Same-repository branches whose pull requests merged directly into the current default branch, whose remote tips still equal the exact pull request head SHAs recorded at merge time, and which are not protected or involved in an open pull request.

The user sees the complete candidate set before mutation unless `--yes` is supplied. Every candidate is revalidated immediately before deletion.

## Users

- developers with many short-lived feature branches
- maintainers cleaning up an existing branch backlog
- people operating several personal or team repositories
- coding agents that need deterministic safety rules and machine-readable output
- future editor integrations that need one reusable scan and apply engine

## Product principles

### Evidence before deletion

A merged pull request is necessary but not sufficient. Eligibility also requires the current remote ref to match the exact pull request head SHA recorded at merge time.

### Narrow defaults

Closed-unmerged branches, arbitrary age-based stale branches, local branches, tags, worktrees, forks, protected branches, and branches involved in open pull requests are excluded from automatic deletion.

### Preview before mutation

Human-readable mode shows the repository, branch, pull request, merge date, and exact SHA before confirmation. JSON mode exposes the same scan result without presentation styling.

### Revalidate at the point of action

Scan results do not authorise deletion by themselves. Tidy Branches refreshes repository state, open pull requests, protection, and the exact branch ref immediately before each deletion.

### Bulk reads, bounded concurrency

The scanner fetches branch and pull request pages in bulk, joins the data locally, and limits cross-repository concurrency. Branch count should not become branch-count API traffic.

### CLI first

The reusable product is the scan, evidence, revalidation, and mutation engine. Terminal and editor clients should consume that engine rather than reimplement eligibility.

### Useful from anywhere

Inside a repository, the current repository is selected automatically. Elsewhere, users can pass repositories explicitly or scan a configured list.

## Success measures

- repositories with hundreds of branches scan with a small number of paginated requests
- squash and rebase merge histories are handled correctly
- reused branches that advanced after merge are preserved
- open pull request head and base branches are preserved
- repeated scans against the same state produce the same candidate set
- mutation remains explicit, reviewable, and auditable
- the stable JSON and selected-candidate apply interface can support a thin VS Code client
