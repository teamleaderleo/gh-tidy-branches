# Tidy Branches

Tidy Branches finds remote GitHub branches whose pull requests have already merged, proves that each branch still points to the exact merged pull request head, then lets you preview and delete the eligible set.

The command is:

```console
gh tidy-branches
```

## Why it exists

GitHub can automatically delete branches after future pull request merges, but that setting does not clean an existing backlog. Ordinary Git branch cleanup also misses squash and rebase merges, and it usually works only on a local clone.

Tidy Branches uses GitHub pull request records and current remote refs. It can scan one repository or a configured group of repositories from any directory.

## Current status

The repository has the required `gh-` prefix and the initial Go implementation is under active development. It includes:

- bulk paginated branch and pull request reads
- bounded cross-repository concurrency
- in-memory eligibility joins
- exact merged-head SHA checks
- open pull request head and base protection
- serial deletion with immediate ref revalidation
- a complete deletion preview with PR, merge date, and exact SHA evidence
- bounded retries for safe read requests and rate-limit-aware backoff
- request counts, retry counts, and scan timings
- an atomic local undo receipt and `gh tidy-branches undo`
- text and JSON output
- repository configuration and expanded `doctor` diagnostics
- GitHub.com and GitHub Enterprise host support through `gh` authentication

The next release milestone is the first cross-platform prerelease and end-to-end installation test.

## Install

After the first release is published:

```console
gh extension install teamleaderleo/gh-tidy-branches
```

Then verify the installation:

```console
gh tidy-branches doctor
gh tidy-branches --help
```

Until then, install a verified development build from a local clone.

## Build and test

Requirements:

- Go 1.23 or newer
- GitHub CLI authenticated with `gh auth login`

Run the complete local verification suite:

```console
make verify
```

That runs formatting checks, race-enabled tests, vet, a production-style build, and CLI command-surface smoke tests.

Install or refresh the local GitHub CLI extension with one command:

```console
make install-dev
```

`make install-dev` always rebuilds the root extension executable before reinstalling it, then verifies the installed version and command surface. This avoids accidentally running a stale binary after pulling new source code.

Remove the development installation with:

```console
make uninstall-dev
```

## Run during development

Preview without mutation:

```console
gh tidy-branches --preview teamleaderleo/smolrunner
gh tidy-branches --preview teamleaderleo/glossless
```

Inside a Git repository, omit the repository argument:

```console
gh tidy-branches --preview
```

Outside a Git repository, the command reads configured repositories:

```console
gh tidy-branches config add teamleaderleo/smolrunner
gh tidy-branches config add teamleaderleo/glossless
gh tidy-branches --all --preview
```

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
--preview             print candidates without deleting
--dry-run             alias for --preview
--yes                 delete all eligible candidates without prompting
--all                 scan configured repositories
--jobs N              scan at most N repositories concurrently, default 2
--json                emit machine-readable output
--delete-delay 1s     pause between delete requests
```

Without `--yes` or `--preview`, the command displays the complete candidate set and asks once before deletion.

## Undo

After at least one branch is successfully deleted, Tidy Branches writes an atomic receipt in the user cache directory. Restore the last deleted set with:

```console
gh tidy-branches undo
```

Undo is deliberately conservative:

- it recreates a branch only at the exact recorded SHA
- it never overwrites a branch name that now points somewhere else
- it leaves unsuccessful entries in the receipt for later inspection
- it is best-effort because GitHub must still retain the recorded commit object

A branch that already exists at the recorded SHA is treated as already restored.

## Eligibility rules

A branch is eligible only when all of these are true:

1. Its pull request merged directly into the repository default branch.
2. The pull request head belonged to the same repository.
3. The remote branch still exists.
4. The branch tip still equals the exact pull request head SHA recorded at merge time.
5. The branch is not the default branch.
6. No open pull request currently uses the branch as a head or base.
7. GitHub does not report the branch as protected.

Immediately before deletion, Tidy Branches refreshes open pull requests and re-reads the branch ref. Deletion requests remain serial and are never automatically retried.

## Documentation

- [Product brief](docs/PRODUCT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Roadmap](docs/ROADMAP.md)
- [Security model](docs/SECURITY.md)
- [Release runbook](docs/RELEASING.md)

## License

MIT
