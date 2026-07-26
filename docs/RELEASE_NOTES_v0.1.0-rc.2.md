# Tidy Branches v0.1.0-rc.2

> **Historical candidate — not installable through GitHub CLI.** This release published valid binaries, but their asset names omitted the required `gh-tidy-branches-` prefix. `gh extension install` therefore could not recognise them. Use `v0.1.0-rc.3` or newer.

Tidy Branches is a GitHub CLI extension for cleaning up remote branches after their pull requests merge. It uses GitHub pull request records and current remote refs to identify a narrow set of branches, shows the complete candidate list, and rechecks every branch immediately before deletion.

This candidate replaced `v0.1.0-rc.1`, whose release workflow failed before creating a GitHub Release or uploading any binaries. The application code in `rc.1` passed its normal test suite; `rc.2` fixed the release build invocation but did not yet use GitHub CLI's required release-asset prefix.

## Known packaging defect

The release assets were named `darwin-arm64`, `linux-amd64`, and similar. GitHub CLI requires precompiled extension assets to include the repository name, for example `gh-tidy-branches-darwin-arm64`. The binaries can be downloaded manually, but this candidate is not supported for installation through `gh extension install`.

## What this release does

- scans the current repository, explicit repositories, or a configured repository list
- supports repeatable `-R/--repo` flags for familiar GitHub CLI-style selection
- finds branches through paginated bulk API requests rather than one request per branch
- proposes only same-repository branches whose pull requests merged into the current default branch
- requires the current branch ref to match the exact pull request head SHA recorded at merge time
- excludes default, protected, fork, advanced, and open-pull-request head or base branches
- refreshes repository state, open pull requests, branch protection, and the exact ref immediately before deletion
- provides a complete preview, terminal-aware colour, JSON output, diagnostics, and bounded retries for safe reads
- writes an exact-SHA undo receipt and can recreate deleted branches when the name is still free and GitHub retains the commit

## Safety boundary

This release candidate does **not** automatically delete:

- closed but unmerged pull request branches
- arbitrary stale branches
- branches with no merged pull request evidence
- branches that advanced after their pull request merged
- branches used by an open pull request
- protected or default branches
- branches from forks
- local branches or worktrees
- tags

GitHub's delete-ref API does not offer an atomic “delete only if the branch still equals this SHA” operation. Tidy Branches narrows that race by re-reading each branch immediately before its serial deletion, but it cannot eliminate the final network race completely.

## What branch deletion preserves

Deleting an eligible branch removes the named remote ref. It does not delete the pull request, its discussion and reviews, the merged result on the default branch, local branches, tags, issues, or releases.

With squash and rebase merges, the original pre-merge commit topology is not represented identically on the default branch. The preview and exact-SHA undo receipt provide additional evidence and recovery for that reason.

## Validation status

Before tagging, the exact release code passed formatting checks, race-enabled tests, `go vet`, direct CLI smoke tests, a real `gh extension install .` test, representative Linux, macOS, and Windows cross-builds, and a release-build smoke test. Published installation then exposed the missing asset-name prefix.

The separate on-demand workflow that creates a real branch, opens and merges a pull request, deletes the branch, restores it, and cleans up remains an RC validation task.

## Known limitations

- Undo is best-effort, not a native transactional undelete.
- The versioned JSON output remains experimental during the release-candidate series.
- Interactive subset selection is not included yet.
- ETag caching and closed-unmerged branch review are intentionally deferred.
