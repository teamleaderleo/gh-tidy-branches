# Tidy Branches

Tidy Branches is a GitHub CLI extension for cleaning up remote branches after their pull requests merge. It verifies that each branch still points to the exact commit recorded by the merged pull request, shows you the complete candidate list, and rechecks every branch immediately before deletion.

```console
gh tidy-branches
```

## Why it exists

GitHub can automatically delete branches after future pull request merges, but it does not clean an existing backlog. Local Git cleanup commands also struggle with squash merges, rebase merges, reused branch names, forks, and repositories that are not cloned on your machine.

Tidy Branches uses GitHub's pull request records and current remote refs, so it can clean one repository or several repositories from any directory.

## Install the release candidate

```console
gh extension install teamleaderleo/gh-tidy-branches --pin v0.1.0-rc.3
```

Check the installed version and your GitHub access:

```console
gh tidy-branches --version
gh tidy-branches doctor
```

Preview a repository without changing anything:

```console
gh tidy-branches --preview -R OWNER/REPOSITORY
```

## The safe default

Tidy Branches only proposes a branch when all of these statements are true:

1. Its pull request merged directly into the repository's current default branch.
2. The pull request came from a branch in the same repository, not a fork.
3. The remote branch still exists.
4. The branch still points to the exact pull request head SHA recorded at merge time.
5. The branch is not the default branch or a protected branch.
6. No open pull request currently uses the branch as a head or base.

The repository, open pull requests, branch protection, and exact branch ref are checked again immediately before deletion.

Tidy Branches does **not** automatically delete closed-unmerged branches, arbitrary stale branches, local branches, worktrees, or tags.

## Preview, apply, and undo

Without `--yes` or `--preview`, Tidy Branches prints the full eligible set and asks once before deleting anything.

```text
Deletion preview
Every branch below matches the exact head SHA of a merged pull request.

! owner/example
  42 branches · 2 open PRs · 3 eligible
  DELETE  feat/finished-work   PR #123   merged 2026-07-20   abc123def4
```

Delete the complete eligible set after live revalidation:

```console
gh tidy-branches --yes -R OWNER/REPOSITORY
```

After a successful deletion, Tidy Branches writes an atomic local receipt containing the previous branch names and exact SHAs. Restore the most recently deleted set with:

```console
gh tidy-branches undo
```

Undo is deliberately conservative. It recreates a branch only when the name is still available, never overwrites a branch that now points somewhere else, and depends on GitHub still retaining the recorded commit.

## Select repositories

Use positional repository names or repeat GitHub CLI's familiar `-R/--repo` flag:

```console
gh tidy-branches --preview owner/one owner/two
gh tidy-branches --preview -R owner/one -R owner/two
```

Inside a Git repository, omit the repository argument to scan the current repository:

```console
gh tidy-branches --preview
```

Configure repositories for repeated multi-repository scans:

```console
gh tidy-branches config add owner/one
gh tidy-branches config add owner/two
gh tidy-branches --all --preview
```

`--all` cannot be combined with explicit repositories. The command rejects that ambiguous combination instead of silently choosing one source.

## Commands

```text
gh tidy-branches [flags] [owner/repo ...]
gh tidy-branches undo [--yes] [--json]
gh tidy-branches config add owner/repo
gh tidy-branches config remove owner/repo
gh tidy-branches config list
gh tidy-branches doctor [owner/repo ...]
```

Useful flags:

```text
-n, --preview         print candidates without deleting
    --dry-run         alias for --preview
-y, --yes             delete every eligible candidate without prompting
    --all             scan configured repositories
-R, --repo REPO       scan an explicit repository; repeat for more
    --jobs N          scan at most N repositories concurrently, default 2
    --json            emit machine-readable output
    --delete-delay 1s pause between delete requests
```

## Terminal and automation behaviour

Colour is enabled only when output is connected to a terminal. Redirected output stays plain text. Set `NO_COLOR=1` or `CLICOLOR=0` to disable colour, and use `GH_FORCE_TTY=1` to force terminal-style presentation.

`GH_PROMPT_DISABLED=1` disables deletion prompts. In that environment, use `--preview` for a non-mutating scan or `--yes` for an explicit apply.

JSON output never contains colours, spinners, or presentation-only text. The release-candidate JSON shape is versioned but still experimental; editor clients should wait for the selected-candidate apply interface before treating it as stable.

## What deletion removes

Deleting a Git branch removes the named remote ref. It does not delete the pull request, its discussion and reviews, the merged result on the default branch, local branches, tags, issues, or releases.

With squash and rebase merges, the original pre-merge commit topology is not represented identically on the default branch. That is why Tidy Branches requires exact-SHA evidence, always previews the candidate set, and records an undo receipt.

## Development

Requirements:

- Go 1.23 or newer
- GitHub CLI authenticated with `gh auth login`

Run the complete local verification suite:

```console
make verify
```

Install or refresh a development build:

```console
make install-dev
```

`make install-dev` rebuilds the repository-root executable before reinstalling the extension, then verifies the installed version and command surface. This prevents a stale local binary from surviving a source update.

The test suite includes deterministic scanner and API fixtures, an installed-extension test, terminal presentation tests, cross-platform builds, a release-packaging smoke test that enforces GitHub CLI's asset naming convention, and an on-demand live GitHub workflow that creates, merges, deletes, restores, and cleans up a real test branch.

## Documentation

- [Product brief](docs/PRODUCT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Roadmap](docs/ROADMAP.md)
- [Security model](docs/SECURITY.md)
- [Testing](docs/TESTING.md)
- [Release runbook](docs/RELEASING.md)
- [Release candidate notes](docs/RELEASE_NOTES_v0.1.0-rc.3.md)

## License

MIT
