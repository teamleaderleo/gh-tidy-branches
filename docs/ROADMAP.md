# Roadmap

## P0: publishable foundation

- rename the repository to `gh-tidy-branches`
- add the `gh-extension` repository topic
- merge the initial Go scanner
- verify Linux, macOS, and Windows builds
- publish a prerelease with cross-compiled assets
- test installation through `gh extension install teamleaderleo/gh-tidy-branches`

## P1: first stable release

- interactive candidate selection
- stable JSON schema and schema version
- clearer skipped-reason reporting
- `--repo` and repeated repository selection ergonomics
- release notes and compatibility table
- end-to-end tests against recorded HTTP fixtures
- rate-limit and retry handling
- terminal progress that remains quiet in JSON mode

## P2: repeated-use performance

- ETag conditional-request cache
- cache inspection and clearing commands
- adaptive pagination diagnostics
- repository scan timing output
- configurable repository groups
- shell completion

## P3: broader cleanup review

- opt-in review of closed-unmerged branches
- age and last-commit evidence
- branch allowlists and deny rules
- protected-branch and ruleset explanations
- audit log export
- optional check for GitHub's automatic head-branch deletion setting

These branches should remain review-only until a separate safety model is proven.

## P4: editor clients

- VS Code extension using the JSON interface
- repository and candidate tree view
- per-branch evidence panel
- selected apply flow
- Cursor-compatible command palette integration

## Explicit non-goals for the first release

- local branch deletion
- Git worktree cleanup
- tag cleanup
- force deletion of protected branches
- arbitrary stale-branch deletion
- automatic deletion of closed-unmerged work
- background scheduled deletion
