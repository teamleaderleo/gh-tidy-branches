# Discovery and Product Notes

_Last reviewed: 2026-07-26_

This document records how GitHub CLI extensions are published and discovered, where Tidy Branches fits among nearby tools, and which additions appear worth building next.

It is a decision aid, not a promise to implement every idea below.

## What publishing a GitHub CLI extension actually means

There is no separate GitHub CLI extension store submission, approval queue, or publisher dashboard.

The public GitHub repository is the product page. Publishing consists of:

1. keeping the extension in a public repository whose name begins with `gh-`
2. publishing executable release assets for supported platforms
3. adding the repository topic `gh-extension`
4. writing a useful repository description and README
5. linking people to the repository or its install command

The official catalogue is the [`gh-extension` topic page](https://github.com/topics/gh-extension).

Users can also discover extensions from the terminal:

```console
gh extension browse
gh extension search
gh extension search branch
gh extension search tidy
gh extension search --web
```

`gh extension search` is repository search specialised for the `gh-extension` topic. With no query, it sorts by stars. Search results display the repository owner/name and repository description. This makes the repository description, topics, README opening, release status, and stars the practical listing metadata.

Useful official references:

- [GitHub CLI extension manual](https://cli.github.com/manual/gh_extension)
- [`gh extension search`](https://cli.github.com/manual/gh_extension_search)
- [`gh extension browse`](https://cli.github.com/manual/gh_extension_browse)
- [Using GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/using-github-cli-extensions)
- [Creating GitHub CLI extensions](https://docs.github.com/en/github-cli/github-cli/creating-github-cli-extensions)

## Where users can look at Tidy Branches

The main public surfaces are:

- repository page: `https://github.com/teamleaderleo/gh-tidy-branches`
- releases: `https://github.com/teamleaderleo/gh-tidy-branches/releases`
- extension catalogue: `https://github.com/topics/gh-extension`
- terminal search: `gh extension search tidy` or `gh extension search branch`
- terminal browser: `gh extension browse`

The install command is the closest equivalent to an app-store install button:

```console
gh extension install teamleaderleo/gh-tidy-branches
```

For the current release candidate, users may pin the exact version:

```console
gh extension install teamleaderleo/gh-tidy-branches --pin v0.1.0-rc.3
```

## Separate publication path for a future editor client

A future VS Code client would be a separate product with a separate publication channel.

- VS Code users discover extensions in the Visual Studio Marketplace and the editor's Extensions view.
- Extension authors package and publish with `@vscode/vsce`.
- The Marketplace listing has its own name, publisher identifier, README, icon, install count, ratings, categories, tags, and changelog.
- A `.vsix` package can also be shared directly for testing.

The CLI should remain independently installable. The editor client should depend on the CLI rather than replace it.

## Nearby GitHub CLI extensions

### `seachicken/gh-poi`

`gh-poi` safely cleans merged **local** branches. It is pull-request-aware, offers quick and deep scan modes, supports dry-run, and lets users lock branch names against deletion. Its README includes a logo and animated demonstration.

The overlap is the safety-oriented merged-branch cleanup story. The boundary is important: `gh-poi` cleans a cloned repository's local branches, while Tidy Branches cleans remote GitHub refs and can operate across repositories without local clones.

### `davidraviv/gh-clean-branches`

`gh-clean-branches` removes **local** branches whose upstream is missing, with optional worktree cleanup and a force mode. It is primarily a local Git/upstream hygiene tool.

Tidy Branches should not absorb this job. Local worktree and branch state introduces a separate safety model and would weaken the product's clear remote-GitHub boundary.

### `mislav/gh-branch`

`gh-branch` is an `fzf`-based local branch switcher with pull-request context and deletion capabilities. Its core value is interactive navigation rather than remote backlog cleanup.

The lesson for Tidy Branches is that a fast selector can be useful, but selection does not require turning the project into a full-screen terminal application.

## Product position

Do not lead with:

> Batch-delete remote branches.

That sounds broad, destructive, and interchangeable with a shell script.

Lead with something closer to:

> Review and safely retire remote branches whose pull requests already merged, using GitHub's recorded head SHA as evidence.

The mechanism is deletion. The product value is confidence, evidence, and repeatability.

### Primary user

The clearest initial user is a maintainer who:

- owns or maintains several GitHub repositories
- has an old backlog of merged pull-request branches
- uses squash or rebase merges, making local ancestry checks incomplete
- does not have every repository cloned locally
- wants to inspect evidence before changing remote refs
- wants a conservative undo path

This is not a daily tool for every developer. It is a periodic maintenance tool for people responsible for repository hygiene.

### Why it is different

Tidy Branches currently combines several properties that nearby tools generally do not:

- remote GitHub cleanup rather than local branch cleanup
- multi-repository operation
- no clone required
- merged pull-request evidence
- exact current-ref versus merged-PR-head SHA matching
- default, protection, fork, advanced-head, and open-PR exclusions
- immediate live revalidation before serial deletion
- a complete preview and exact-SHA undo receipt

The narrowness is a strength. Avoid broadening the default eligibility rule merely to increase the candidate count.

## Highest-priority publication work

### 1. Finish repository metadata

The repository should have:

- description: `Safely find and delete remote branches whose pull requests have already merged.`
- topic: `gh-extension`
- supporting topics: `github-cli`, `git`, `branch-cleanup`, `developer-tools`

Verify with:

```console
gh repo view teamleaderleo/gh-tidy-branches \
  --json description,repositoryTopics,url \
  --jq '{description, topics: [.repositoryTopics[].name], url}'
```

Apply with:

```console
gh repo edit teamleaderleo/gh-tidy-branches \
  --description "Safely find and delete remote branches whose pull requests have already merged." \
  --add-topic gh-extension \
  --add-topic github-cli \
  --add-topic git \
  --add-topic branch-cleanup \
  --add-topic developer-tools
```

After GitHub indexes the topic, verify discovery:

```console
gh extension search tidy
gh extension search branch --limit 100
gh search repos --topic gh-extension tidy-branches
```

### 2. Improve the repository's first screen

The README already explains the safety model thoroughly. Its next improvement should be visual and comparative rather than longer prose.

Recommended additions:

- one short terminal recording showing preview, evidence, confirmation, deletion, and undo
- release, CI, licence, and provenance badges without creating a badge wall
- a compact "Why not GitHub auto-delete?" explanation: auto-delete handles future merges; Tidy Branches handles an existing backlog and provides evidence
- a compact "Why not `git branch --merged`?" explanation: squash/rebase merges, remote-only repositories, reused branch names, and current-ref verification
- a clear current-status label while the JSON interface remains experimental

### 3. Make trust visible

GitHub explicitly warns that third-party CLI extensions are not verified or endorsed. Tidy Branches should therefore make its trust evidence easy to inspect:

- link the security model near installation
- link release provenance and verification instructions
- make the non-goals visible
- keep the exact candidate rule near the install command
- document permissions used and explain that authentication comes from the user's existing `gh` session

### 4. Do not collect telemetry merely to measure adoption

The CLI has no need for behavioural telemetry. Useful public signals already include:

- stars and forks
- release asset download counts as a rough, non-unique proxy
- issues and discussions from external users
- references from dotfiles, blog posts, or curated extension lists
- contributions and compatibility reports

The absence of perfect usage analytics is preferable to adding a network call unrelated to branch cleanup.

## Recommended additions to the existing CLI

### P0: close the release-candidate loop

Before new product features:

- run the live create → merge → detect → delete → undo workflow against the fixture repository
- validate installation on Linux as well as Apple Silicon macOS
- update the roadmap's completed publication checks
- finish repository metadata and verify extension search discovery
- decide whether the first stable release requires the selected-candidate interface or whether that belongs immediately after `v0.1.0`

The live mutation test is more valuable than another display feature right now.

### P1: stable machine interface and selected apply

Issue #6 remains the most important capability addition.

A stable versioned JSON scan plus exact selected-candidate apply enables:

- shell and `jq` workflows
- coding agents
- a small terminal selector
- a future VS Code client
- reusable fixtures for compatibility testing

The selected apply input must include repository, branch, and expected SHA. Every candidate must pass the same live revalidation used by the normal apply path.

### P1: explain one branch

Add an explicit evidence/explanation command after the stable data model exists, for example:

```console
gh tidy-branches explain OWNER/REPO BRANCH
gh tidy-branches explain OWNER/REPO BRANCH --json
```

It should answer:

- whether the branch is eligible
- which merged pull request supplied the evidence
- the recorded PR head SHA and current ref SHA
- which exclusion or stale condition prevented eligibility
- whether an open PR uses the branch as head or base
- whether branch protection or the default branch blocks deletion

This would improve support, user trust, and editor integration without changing eligibility.

### P1: clearer skip summaries

The scanner should retain detailed reasons for important exclusions and provide a concise summary, such as:

```text
18 branches
  3 eligible
  6 no merged PR into current default
  4 advanced after merge
  2 used by open PRs
  2 protected
  1 fork PR
```

Avoid printing a wall of per-branch diagnostics by default. Detailed evidence can live behind `--verbose`, `explain`, or JSON.

### P1: interactive selection in the terminal

After selected apply exists, add a modest selector rather than a large TUI.

A good first implementation is a checkbox or fuzzy picker that:

- starts from the exact scanned candidate records
- shows repository, branch, PR, merge date, and abbreviated SHA
- submits only selected records to the selected apply path
- cancels without mutation
- remains optional and never affects JSON or redirected output

This is useful on its own and validates the interaction model before an editor client is built.

### P1: automatic-deletion setting helper

The tool solves an old backlog. GitHub's automatic head-branch deletion setting prevents a new backlog.

A separate explicit command could complete that lifecycle:

```console
gh tidy-branches settings show OWNER/REPO
gh tidy-branches settings enable-auto-delete OWNER/REPO
```

It must never silently change repository settings during cleanup. The command should explain the difference between the setting and Tidy Branches before asking for confirmation.

### P2: receipt history and audit export

The existing undo receipt is valuable. Later additions could include:

```console
gh tidy-branches receipts list
gh tidy-branches receipts show RECEIPT_ID
gh tidy-branches undo --receipt RECEIPT_ID
gh tidy-branches receipts export --format json
```

This is more aligned with the product's trust story than adding broad stale-branch heuristics.

### P2: saved repository groups

The current configured repository list may eventually benefit from named groups:

```console
gh tidy-branches group add work owner/one owner/two
gh tidy-branches group add personal owner/three
gh tidy-branches --group work --preview
```

Do not add this until real repeated multi-repository usage shows the single list is inadequate.

### P2: conditional-request cache

An ETag cache can improve repeated scans, but only after the output and selected-apply contracts are stable. It should be inspectable, bounded, and easy to clear.

## Ideas to avoid or defer

Do not add these merely to appear more feature-complete:

- automatic deletion of arbitrary old branches
- default deletion of closed-unmerged pull-request branches
- local branch or worktree cleanup
- tag cleanup
- cron-like background deletion
- silent repository-setting changes
- direct GitHub deletion logic in an editor client
- a second safety engine in TypeScript
- a full custom TUI before a simple selector is tested
- analytics or telemetry unrelated to the requested GitHub operation

## VS Code client recommendation

The editor client remains plausible, but it should not be the next implementation milestone.

Dependency order:

1. stable JSON schema
2. exact selected-candidate apply
3. terminal selector or command-line dogfooding of selected apply
4. small VS Code prototype
5. richer tree view only after the prototype proves useful

### Smallest useful prototype

Start with one command:

```text
Tidy Branches: Scan Current Repository
```

The extension would:

1. locate `gh`
2. verify that `gh tidy-branches` is installed and compatible
3. run a JSON scan for the workspace repository
4. present candidates in VS Code Quick Pick with PR, date, and SHA evidence
5. submit the selected exact candidate records to the CLI apply command
6. show deleted, skipped, and failed outcomes
7. offer links to the associated pull requests and the undo command

Do not start with an activity-bar icon, custom webview, or large repository tree. Quick Pick is enough to test whether the editor adds meaningful review value.

### Go/no-go signals

Continue toward a full editor client when at least several of these are true:

- external users ask for selective application
- users report that terminal review is the main adoption barrier
- users regularly scan several repositories from an editor workspace
- selected apply is stable and well tested
- the Quick Pick prototype is used repeatedly during dogfooding
- maintaining a separate Marketplace package does not slow the CLI's safety work

Stop or pause when the client is merely a more expensive way to run the same one-repository terminal command.

## Practical next sequence

1. Finish issue #1 metadata and verify catalogue search.
2. Run the live fixture workflow and close the RC validation checklist.
3. Add a short demonstration recording to the README.
4. Complete issue #6.
5. Add `explain` and clearer skip summaries against the stable evidence model.
6. Build the smallest terminal selector.
7. Reassess the VS Code prototype with real usage evidence.

The next feature should strengthen evidence, selective control, or trust. It should not widen the deletion rule simply to make the tool look busier.
