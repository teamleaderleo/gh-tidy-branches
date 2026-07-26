# Tidy Branches v0.1.0-rc.1

The first release candidate of Tidy Branches is a precompiled GitHub CLI extension for safely cleaning up remote branches whose pull requests have already merged.

## Highlights

- scans one repository, repeated `-R/--repo` repositories, or configured repository groups
- discovers branches and pull requests in paginated bulk requests rather than one API call per branch
- deletes only same-repository branches merged into the current default branch
- requires the current remote ref to equal the exact pull request head SHA recorded at merge time
- excludes default, protected, fork, advanced, and open-pull-request head or base branches
- revalidates repository state, open pull requests, protection, and the exact ref immediately before deletion
- provides a complete deletion preview, terminal-aware colour, JSON output, diagnostics, and bounded read retries
- writes an exact-SHA undo receipt and can recreate deleted branches when the name is still free and GitHub retains the commit
- ships precompiled Linux, macOS, and Windows binaries with provenance attestations

## Install

```console
gh extension install teamleaderleo/gh-tidy-branches --pin v0.1.0-rc.1
```

Then verify:

```console
gh tidy-branches --version
gh tidy-branches doctor
gh tidy-branches --preview -R OWNER/REPOSITORY
```

## Safety boundary

This release candidate does not delete:

- closed but unmerged pull request branches
- arbitrary stale branches
- branches with no merged pull request evidence
- local branches or worktrees
- tags
- protected branches
- branches that advanced after their pull request merged
- branches used by an open pull request

GitHub's delete-ref API does not provide an atomic expected-SHA compare-and-delete operation. Tidy Branches narrows that race by re-reading the branch immediately before each serial deletion, but it cannot eliminate the final network race completely.

## Known limitations

- Undo is best-effort rather than a native transactional undelete.
- The JSON output is versioned but remains experimental during the release-candidate series. Its stable schema and selected-candidate apply command will be completed before the VS Code client depends on it.
- Interactive subset selection is not included yet; the current prompt applies the complete eligible set. Use `--preview`, JSON output, or separate repository runs when reviewing candidates.
- ETag caching and broader closed-unmerged branch review are intentionally deferred.
- The release candidate still needs published-install validation on real macOS and Linux systems before `v0.1.0`.

## Useful commands

```console
# Preview one repository
gh tidy-branches --preview -R OWNER/REPOSITORY

# Preview several repositories
gh tidy-branches --preview -R OWNER/ONE -R OWNER/TWO

# Preview configured repositories
gh tidy-branches --all --preview

# Delete every currently eligible branch after revalidation
gh tidy-branches --yes -R OWNER/REPOSITORY

# Restore the most recent successfully deleted set
gh tidy-branches undo
```

`GH_PROMPT_DISABLED=1` prevents interactive deletion prompts. `NO_COLOR=1` disables colour, and `GH_FORCE_TTY=1` forces terminal-style presentation in the same spirit as GitHub CLI.
