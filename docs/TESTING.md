# Testing

Tidy Branches uses several test layers because destructive branch cleanup needs deterministic coverage, proof of the installed command, release-packaging checks, and validation against the real GitHub API.

## 1. Deterministic tests on every pull request

`make verify` runs:

- formatting checks
- race-enabled Go tests
- `go vet`
- a production-style build
- direct CLI command-surface smoke tests
- terminal presentation tests with and without ANSI colour

The API and scanner tests cover pagination, fork pull requests, reused branch names, protected branches, branches used by open pull requests, branches that advanced after merge, transient read failures, rate limits, deletion revalidation, undo receipts, and safe restoration.

## 2. Installed-extension test on every pull request

The `local extension install` job builds the repository-root executable, installs it through `gh extension install .`, and invokes `gh tidy-branches` through GitHub CLI. This catches local packaging and stale-binary problems that unit tests cannot see.

## 3. Release-packaging test on every pull request

The verify job runs `script/build-release.sh` for one representative platform and verifies all three release-facing properties:

- the asset is named `gh-tidy-branches-<os>-<arch>` as required by GitHub CLI
- no unprefixed alternative asset is produced
- the supplied release version is embedded in the binary

The cross-build matrix separately compiles every supported release target:

- Linux amd64
- Linux arm64
- macOS amd64
- macOS arm64
- Windows amd64

This layer exists because successful application cross-builds do not by themselves prove that the release action creates assets that `gh extension install` can recognise.

## 4. Live branch lifecycle test on demand

`.github/workflows/live-integration.yml` is manually dispatched against a dedicated fixture repository. It performs the complete remote lifecycle:

1. create a real branch and commit
2. open and merge a real pull request
3. prove `--preview --json` identifies the unchanged merged branch
4. run `--yes --json` and verify the remote ref is deleted
5. run `undo --yes --json`
6. verify the branch is recreated at the exact original SHA
7. remove the restored branch and fixture file

The workflow is intentionally not part of ordinary pull-request CI. It creates real pull requests and remote refs, so it needs an isolated repository and a narrowly scoped credential.

### One-time fixture setup

Create a repository used only for this workflow, for example:

```console
gh repo create teamleaderleo/gh-tidy-branches-fixtures --public --add-readme --clone=false
```

Keep **Automatically delete head branches** disabled in the fixture repository. The workflow needs the merged branch to remain present long enough for Tidy Branches to detect it.

Create a fine-grained personal access token restricted to the fixture repository with:

- Contents: read and write
- Pull requests: read and write

Store it in the main repository as the Actions secret `TIDY_BRANCHES_LIVE_TOKEN`.

Then open **Actions → live branch lifecycle → Run workflow**, confirm the fixture repository, and run it.

The workflow cleans up the branch and fixture file even when a later assertion fails. The merged fixture pull request remains in the fixture repository as an audit trail.

## Terminal output

Human-readable output uses colour only when writing to a terminal. Redirected output and tests remain plain text. Set `NO_COLOR=1` or `CLICOLOR=0` to disable colour. `TERM=dumb` also forces plain output, while `CLICOLOR_FORCE=1` is useful for presentation tests.

JSON output never contains terminal styling or progress animation.
