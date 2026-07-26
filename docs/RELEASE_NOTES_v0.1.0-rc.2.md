# Tidy Branches v0.1.0-rc.2

Tidy Branches is a GitHub CLI extension for cleaning up remote branches after their pull requests merge. It uses GitHub pull request records and current remote refs to identify a narrow set of branches, shows the complete candidate list, and rechecks every branch immediately before deletion.

This candidate replaces `v0.1.0-rc.1`, whose release workflow failed before creating a GitHub Release or uploading any binaries. The application code in `rc.1` passed its normal test suite; `rc.2` fixes and tests the release packaging path.

## Install

```console
gh extension install teamleaderleo/gh-tidy-branches --pin v0.1.0-rc.2
```

Verify the installation and preview one repository:

```console
gh tidy-branches --version
gh tidy-branches doctor
gh tidy-branches --preview -R OWNER/REPOSITORY
```

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
- publishes precompiled Linux, macOS, and Windows binaries with provenance attestations

## Packaging change since rc.1

The release now uses a dedicated build script that produces the exact `dist/<os>-<arch>` filenames expected by the GitHub CLI precompile action and embeds the release tag in every binary. Ordinary pull-request CI exercises that packaging script and verifies the embedded version before another tag can be published.

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

Before tagging, the exact release code passed formatting checks, race-enabled tests, `go vet`, direct CLI smoke tests, a real `gh extension install .` test, representative Linux, macOS, and Windows cross-builds, and a release-packaging smoke test.

The separate on-demand workflow that creates a real branch, opens and merges a pull request, deletes the branch, restores it, and cleans up remains an RC validation task. Published installation should also be checked on real macOS and Linux systems before `v0.1.0`.

## Known limitations

- Undo is best-effort, not a native transactional undelete.
- The versioned JSON output remains experimental during the release-candidate series. Its stable schema and selected-candidate apply command will be completed before the VS Code client depends on it.
- Interactive subset selection is not included yet. The current prompt applies the complete eligible set; use `--preview`, JSON output, or separate repository runs when reviewing candidates.
- ETag caching and closed-unmerged branch review are intentionally deferred.

## Useful commands

```console
# Preview one repository
gh tidy-branches --preview -R OWNER/REPOSITORY

# Preview several repositories
gh tidy-branches --preview -R OWNER/ONE -R OWNER/TWO

# Preview configured repositories
gh tidy-branches --all --preview

# Delete every currently eligible branch after live revalidation
gh tidy-branches --yes -R OWNER/REPOSITORY

# Restore the most recent successfully deleted set
gh tidy-branches undo
```

`GH_PROMPT_DISABLED=1` prevents interactive deletion prompts. `NO_COLOR=1` disables colour, and `GH_FORCE_TTY=1` forces terminal-style presentation in the same spirit as GitHub CLI.
