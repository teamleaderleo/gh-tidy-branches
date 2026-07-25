# Agent instructions

Tidy Branches is a safety-sensitive GitHub CLI extension.

## Working rules

- Keep remote deletion opt-in.
- Preserve the exact-SHA eligibility rule.
- Preserve open pull request head and base branches.
- Keep scans bulk-oriented. Do not add a per-branch network lookup to the discovery path.
- Keep repository concurrency bounded and deletion serial.
- Add tests for every eligibility-rule change.
- Use colons and commas in prose. Avoid em dashes.
- Run `go test ./...`, `go vet ./...`, and `go build ./cmd/gh-tidy-branches` before requesting review.

## Product boundary

The first release cleans same-repository remote branches associated with pull requests merged directly into the repository default branch. Closed-unmerged branches, arbitrary stale branches, tags, local branches, and worktrees are outside the automatic deletion set.
