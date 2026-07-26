# Roadmap

## P0: publishable foundation

- [x] rename the repository to `gh-tidy-branches`
- [ ] add and verify the `gh-extension` repository topic and supporting discovery metadata
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
- [x] publish a release candidate with cross-compiled assets and provenance
- [x] test installation through `gh extension install teamleaderleo/gh-tidy-branches`
- [ ] run the live fixture workflow and record the controlled mutation result
- [ ] validate the published binary on Linux as well as Apple Silicon macOS
- [ ] add a short terminal demonstration to the README

## P1: first stable interface

- stable JSON schema and schema version
- selected-candidate apply with exact-SHA revalidation
- fixture-based compatibility tests for the machine interface
- interactive candidate selection backed by selected apply
- an `explain` command for one branch's evidence or exclusion reason
- clearer skipped-reason summaries
- release notes and compatibility table
- broader end-to-end API fixtures
- terminal progress that remains quiet in JSON mode
- explicit command for inspecting and enabling GitHub's automatic head-branch deletion setting

## P2: repeated use and auditability

- receipt history, targeted undo, and audit export
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

These branches should remain review-only until a separate safety model is proven.

## P4: editor clients

Build an editor client only after the stable JSON and selected-apply interfaces are complete and dogfooded from the terminal.

- prototype one VS Code command using Quick Pick
- verify `gh` and Tidy Branches installation and version compatibility
- show branch, pull request, merge date, SHA, and eligibility evidence
- apply only the exact selected candidate records through the CLI
- display deleted, skipped, and failed results
- offer pull-request and undo actions
- add a repository and candidate tree view only after the prototype proves useful
- keep compatibility with editors that support the VS Code extension API where practical

The editor must not call GitHub deletion APIs or decide eligibility independently.

## Explicit non-goals for the first release

- local branch deletion
- Git worktree cleanup
- tag cleanup
- force deletion of protected branches
- arbitrary stale-branch deletion
- automatic deletion of closed-unmerged work
- background scheduled deletion
- behavioural telemetry
