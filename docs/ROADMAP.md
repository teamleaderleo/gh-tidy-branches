# Roadmap

## P0: publishable foundation

- [x] rename the repository to `gh-tidy-branches`
- [ ] add the `gh-extension` repository topic
- [x] merge the initial Go scanner
- [x] verify representative Linux, macOS, and Windows builds in CI
- [x] test source installation through `gh extension install .`
- [x] add rate-limit-aware retries for safe read requests
- [x] add recorded HTTP fixtures and an N+1 request regression ceiling
- [x] add detailed previews, scan timing, and request diagnostics
- [x] add terminal-aware colour with plain redirected and JSON output
- [x] follow native `gh` repository, prompt, and forced-TTY conventions
- [x] add a conservative exact-SHA undo receipt and restore command
- [x] add an on-demand live create → merge → delete → undo workflow for a dedicated fixture repository
- [x] draft first release-candidate notes and known limitations
- [ ] publish a prerelease with cross-compiled assets
- [ ] test installation through `gh extension install teamleaderleo/gh-tidy-branches`

## P1: first stable release

- interactive candidate selection
- stable JSON schema and schema version
- selected-candidate apply with exact-SHA revalidation
- clearer skipped-reason reporting
- release notes and compatibility table
- broader end-to-end API fixtures
- terminal progress that remains quiet in JSON mode
- explicit command for enabling GitHub's automatic head-branch deletion setting

## P2: repeated-use performance

- ETag conditional-request cache
- cache inspection and clearing commands
- adaptive pagination diagnostics
- configurable repository groups
- shell completion

## P3: broader cleanup review

- opt-in review of closed-unmerged branches
- age and last-commit evidence
- branch allowlists and deny rules
- protected-branch and ruleset explanations
- audit log export

These branches should remain review-only until a separate safety model is proven.

## P4: editor clients

- VS Code extension using the JSON interface
- repository and candidate tree view
- per-branch evidence panel
- selected apply flow
- Cursor-compatible command palette integration
- editor-native preview and undo actions backed by the CLI

## Explicit non-goals for the first release

- local branch deletion
- Git worktree cleanup
- tag cleanup
- force deletion of protected branches
- arbitrary stale-branch deletion
- automatic deletion of closed-unmerged work
- background scheduled deletion
