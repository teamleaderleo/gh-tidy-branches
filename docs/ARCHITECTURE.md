# Architecture

## Overview

Tidy Branches is a precompiled Go GitHub CLI extension with no third-party Go dependencies in its first implementation.

```text
command parsing
      |
repository selection
      |
bounded repository worker pool
      |
bulk GitHub REST reads
      |
in-memory eligibility join
      |
review or JSON output
      |
serial revalidation and deletion
```

## Request model

For each repository:

1. Fetch repository metadata and the default branch.
2. In parallel, fetch all current branches, all open pull requests, and all closed pull requests targeting the default branch.
3. Filter closed pull requests to merged, same-repository heads.
4. Keep the newest merged pull request record for each branch name.
5. Join current branches, open pull request protection, and merged records in memory.

The scan path does not perform one request per branch.

Approximate request count:

```text
1 repository request
+ ceil(branches / 100)
+ ceil(open pull requests / 100)
+ ceil(closed pull requests targeting default / 100)
```

Local evaluation is O(B + P), where B is current branches and P is fetched pull requests.

## Concurrency

Repository scans use a bounded worker pool. The default is two repositories at once.

Inside one repository, the branch, open-pull-request, and closed-pull-request page streams begin after repository metadata resolves.

Deletion is deliberately serial. Before deleting a repository's candidates, the tool refreshes open pull requests. Before each deletion, it re-reads the branch ref and compares the SHA again.

## API client

Authentication order:

1. `GH_TOKEN`
2. `GITHUB_TOKEN`
3. `GH_ENTERPRISE_TOKEN`
4. `gh auth token --hostname <host>`

Host order:

1. `GH_HOST`
2. `github.com`

Base URLs:

```text
github.com       -> https://api.github.com
other GH_HOST    -> https://<host>/api/v3
```

## Candidate model

A candidate contains:

```json
{
  "repository": "owner/repo",
  "branch": "feat/example",
  "pull_request": 123,
  "head_sha": "0123456789abcdef",
  "merged_at": "2026-07-26T12:00:00Z"
}
```

## Safety invariant

For automatic eligibility:

```text
current branch SHA == merged pull request head SHA
```

This invariant handles ordinary merge commits, squash merges, and rebase merges because it does not depend on ancestry from the default branch.

## Known race

GitHub's delete-reference endpoint does not accept an expected SHA. The tool therefore cannot make compare-and-delete atomic. It reduces the interval by re-reading the exact ref immediately before a serial delete.

A future GitHub API feature providing conditional ref deletion would close this race completely.

## Caching plan

ETag-backed conditional requests are planned after the first correct release. Cache entries should be host-aware, repository-aware, and endpoint-aware. A stale cache must never authorize deletion. Mutation always requires live revalidation.

## Editor integration

A future VS Code extension should call:

```console
gh tidy-branches --json
```

and submit selected candidates through a narrow apply command. The editor must not reimplement eligibility logic.
