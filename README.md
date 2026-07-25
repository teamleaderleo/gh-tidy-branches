# Tidy Branches

Tidy Branches finds remote GitHub branches whose pull requests have already merged, proves that each branch still points to the exact merged pull request head, then lets you delete the eligible set.

The command is:

```console
gh tidy-branches
```

## Why it exists

GitHub can automatically delete branches after future pull request merges, but that setting does not clean an existing backlog. Ordinary Git branch cleanup also misses squash and rebase merges, and it usually works only on a local clone.

Tidy Branches uses GitHub pull request records and current remote refs. It can scan one repository or a configured group of repositories from any directory.

## Current status

The repository has the required `gh-` prefix and the initial Go implementation is under active development. It already includes:

- bulk paginated branch and pull request reads
- bounded cross-repository concurrency
- in-memory eligibility joins
- exact merged-head SHA checks
- open pull request head and base protection
- serial deletion with immediate ref revalidation
- text and JSON output
- repository configuration commands
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

Until then, build and run the development binary directly.

## Build

Requirements:

- Go 1.23 or newer
- GitHub CLI authenticated with `gh auth login`

```console
go test ./...
go build -o gh-tidy-branches ./cmd/gh-tidy-branches
```

## Run during development

```console
./gh-tidy-branches --dry-run teamleaderleo/smolrunner
./gh-tidy-branches --dry-run teamleaderleo/glossless
```

Inside a Git repository, omit the repository argument:

```console
./gh-tidy-branches --dry-run
```

Outside a Git repository, the command reads configured repositories:

```console
./gh-tidy-branches config add teamleaderleo/smolrunner
./gh-tidy-branches config add teamleaderleo/glossless
./gh-tidy-branches --all --dry-run
```

## Commands

```text
gh tidy-branches [flags] [owner/repo ...]
gh tidy-branches config add owner/repo
gh tidy-branches config remove owner/repo
gh tidy-branches config list
gh tidy-branches doctor
```

Useful flags:

```text
--dry-run             print candidates without deleting
--yes                 delete all eligible candidates without prompting
--all                 scan configured repositories
--jobs N              scan at most N repositories concurrently, default 2
--json                emit machine-readable output
--delete-delay 1s     pause between successful delete requests
```

Without `--yes` or `--dry-run`, the command displays the complete candidate set and asks once before deletion.

## Eligibility rules

A branch is eligible only when all of these are true:

1. Its pull request merged directly into the repository default branch.
2. The pull request head belonged to the same repository.
3. The remote branch still exists.
4. The branch tip still equals the exact pull request head SHA recorded at merge time.
5. The branch is not the default branch.
6. No open pull request currently uses the branch as a head or base.
7. GitHub does not report the branch as protected.

Immediately before deletion, Tidy Branches refreshes open pull requests and re-reads the branch ref. Deletion requests remain serial.

## Documentation

- [Product brief](docs/PRODUCT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Roadmap](docs/ROADMAP.md)
- [Security model](docs/SECURITY.md)

## License

MIT
