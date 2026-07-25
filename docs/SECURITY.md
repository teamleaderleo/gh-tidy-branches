# Security model

Tidy Branches performs destructive remote mutations. The project treats eligibility and application as separate phases.

## Threats

- deleting a reused branch that advanced after merge
- deleting a branch used by an open pull request
- deleting a long-lived integration branch
- deleting a branch from a fork
- acting on stale scan output
- exposing an authentication token
- creating excessive API traffic

## Controls

### Exact merged-head comparison

The current remote branch SHA must equal the pull request head SHA recorded at merge time.

### Default-branch scope

Only pull requests merged directly into the repository default branch enter the automatic candidate set.

### Same-repository scope

Fork pull request heads are excluded.

### Open pull request protection

Branches used as an open pull request head or base are excluded during scanning and refreshed before application.

### Protected branch exclusion

Branches reported as protected are excluded from automatic eligibility. Repository rules may also reject deletion at application time.

### Live application check

The branch ref is re-read immediately before deletion.

### Serial mutation

Delete requests run one at a time with a configurable delay.

### Token handling

Tokens are read from the environment or `gh auth token`. They are sent only in the `Authorization` header and are never printed or written to configuration.

### Least mutation by default

`--dry-run` never mutates. Without `--yes`, interactive confirmation is required.

## Remaining race

The GitHub API does not currently provide conditional branch deletion by expected SHA. A branch can theoretically move after the final read and before deletion. Serial immediate revalidation narrows this interval but cannot remove it.

## Reporting security problems

Do not open a public issue containing a token, private repository name, or sensitive branch metadata. Use GitHub's private security advisory flow once it is enabled for the repository.
