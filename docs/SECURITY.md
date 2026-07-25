# Security model

Tidy Branches performs destructive remote mutations. The project treats eligibility, preview, application, and optional restoration as separate phases.

## Threats

- deleting a reused branch that advanced after merge
- deleting a branch used by an open pull request
- deleting a long-lived integration branch
- deleting a branch from a fork
- acting on stale scan output
- overwriting a branch name during restoration
- exposing an authentication token
- creating excessive API traffic
- duplicating a mutation after an ambiguous network failure

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

### Explicit preview

Text mode displays the repository, branch, pull request, merge date, and exact head SHA evidence before confirmation. `--preview` never mutates.

### Live application check

The branch ref is re-read immediately before deletion.

### Serial mutation

Delete and restore requests run one at a time with a configurable delay.

### Safe retry boundary

Only idempotent GET requests are retried automatically. Tidy Branches uses bounded exponential backoff with jitter and respects GitHub rate-limit signals. Delete and create-ref requests are never automatically retried because an interrupted response does not prove that the mutation did not occur.

### Conservative undo

After successful deletion, an atomic local receipt records the exact repository, branch name, and previous SHA. `gh tidy-branches undo` recreates a branch only when the name is absent. It never force-updates a branch that now points to a different SHA. Failed or unsafe entries remain in the receipt.

Undo is best-effort. It depends on GitHub continuing to retain the recorded commit object and is not a substitute for the live deletion checks.

### Receipt handling

The undo receipt is stored in the operating system user cache directory with mode `0600` where supported. It contains repository names, branch names, pull request numbers, SHAs, and deletion timestamps, but never authentication tokens.

### Token handling

Tokens are read from the environment or `gh auth token`. They are sent only in the `Authorization` header and are never printed or written to configuration or receipts.

### Least mutation by default

`--preview` and `--dry-run` never mutate. Without `--yes`, interactive confirmation is required for deletion and restoration.

## Remaining deletion race

The GitHub API does not currently provide conditional branch deletion by expected SHA. A branch can theoretically move after the final read and before deletion. Serial immediate revalidation narrows this interval but cannot remove it.

## Reporting security problems

Do not open a public issue containing a token, private repository name, or sensitive branch metadata. Use GitHub's private security advisory flow once it is enabled for the repository.
