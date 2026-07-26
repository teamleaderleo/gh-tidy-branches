# Tidy Branches v0.1.0-rc.3

Tidy Branches is a GitHub CLI extension for cleaning up remote branches after their pull requests merge. It uses GitHub pull request records and current remote refs to identify a narrow set of branches, shows the complete candidate list, and rechecks every branch immediately before deletion.

This candidate publishes conventionally named assets, adds exact release-filename checks, and completes the first successful installation from a published tag.

The final installation blocker was not the binary contents. GitHub CLI first inspects the repository's latest non-prerelease release to decide whether it is a binary extension, before it fetches a tag supplied with `--pin`. Because the repository initially contained only prereleases, installation failed before the pinned assets were examined. Marking this same release latest and non-prerelease made the existing tag and binaries installable; no asset or tag was replaced.

## Install

```console
gh extension install teamleaderleo/gh-tidy-branches --pin v0.1.0-rc.3
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

## Packaging and installation changes

Release assets follow a clear repository-prefixed convention:

```text
gh-tidy-branches-darwin-amd64
gh-tidy-branches-darwin-arm64
gh-tidy-branches-linux-amd64
gh-tidy-branches-linux-arm64
gh-tidy-branches-windows-amd64.exe
```

Ordinary pull-request CI builds a release-shaped asset, verifies its exact intended filename, and checks the embedded version.

The release workflow now also handles the first-RC discovery workaround and installs the actual published extension. It fails unless `gh tidy-branches --version` exactly matches the release tag. This closes the gap between “assets were uploaded” and “a user can install the extension.”

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

The release workflow succeeded, the expected five assets were published, the release became visible to GitHub CLI's binary-extension discovery, and a clean Apple Silicon macOS installation reported `v0.1.0-rc.3`. A real preview against `teamleaderleo/smolrunner` completed successfully without changing any branch.

The separate on-demand workflow that creates a real branch, opens and merges a pull request, deletes the branch, restores it, and cleans up remains a validation task before stable `v0.1.0`.

## Known limitations

- Undo is best-effort, not a native transactional undelete.
- The versioned JSON output remains experimental during the release-candidate series. Its stable schema and selected-candidate apply command will be completed before the VS Code client depends on it.
- Interactive subset selection is not included yet. The current prompt applies the complete eligible set; use `--preview`, JSON output, or separate repository runs when reviewing candidates.
- ETag caching and closed-unmerged branch review are intentionally deferred.
- The GitHub Release is marked non-prerelease for compatibility with the current GitHub CLI discovery path, while the tag, title, and notes retain the release-candidate identity.

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
