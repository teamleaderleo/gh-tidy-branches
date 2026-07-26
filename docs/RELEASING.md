# Releasing Tidy Branches

Tidy Branches is published as a precompiled GitHub CLI extension. The repository name must remain `gh-tidy-branches`, because GitHub CLI derives the command name from the repository name.

For the reusable explanation of GitHub CLI extension packaging, discovery, testing layers, and the release-candidate failures encountered here, see [GitHub CLI Extension Release Playbook](GITHUB_CLI_EXTENSION_RELEASE_PLAYBOOK.md).

## Release automation

Pushing a tag matching `v*` starts `.github/workflows/release.yml`.

The workflow:

1. checks out the tagged commit
2. runs `script/build-release.sh` to produce the supported platform binaries
3. embeds the tag in every binary's version output
4. creates or updates the matching GitHub Release through `cli/gh-extension-precompile`
5. uploads the compiled assets
6. generates build provenance attestations
7. makes the first release-candidate series visible to GitHub CLI's binary-extension discovery
8. installs the published extension and verifies its exact version

GitHub CLI currently decides whether a repository is a binary extension by inspecting the latest non-prerelease release before it honours `--pin`. A repository containing only GitHub prereleases therefore cannot install its first pinned release candidate. Until `v0.1.0` exists, the workflow keeps the `v0.1.0-rc.*` tag and candidate wording but marks the GitHub Release itself as latest and non-prerelease for installer compatibility. Stable releases need no workaround.

The release build script is exercised in ordinary pull-request CI with a representative platform and a synthetic version. The test verifies the embedded version and the intended `gh-tidy-branches-<os>-<arch>` filename. The normal cross-build matrix separately covers every supported target.

## Release history

- `v0.1.0-rc.1` failed before creating a GitHub Release or uploading assets because the release action received an invalid combined build-options value.
- `v0.1.0-rc.2` published valid binaries, but installation was never completed. We initially blamed its unprefixed filenames; later inspection of GitHub CLI showed that it selects assets by platform suffix. The repository's prerelease-only metadata was the relevant discovery problem. `rc.2` was not retested after finding that workaround.
- `v0.1.0-rc.3` published conventionally named assets. Installation succeeded after the same release was marked latest and non-prerelease, without changing the tag or binaries.

Do not move, reuse, or silently replace a tag after changing tagged source or binary contents.

## Prerelease checklist

Before tagging:

- `main` is green
- the repository is named `teamleaderleo/gh-tidy-branches`
- the repository has the `gh-extension` topic
- the release commit has no known P0 safety regression
- direct and locally installed command-surface checks pass
- the release-packaging smoke test verifies the embedded version and exact intended asset name
- Linux, macOS, and Windows cross-builds pass
- the matching release-notes file accurately describes the release and known limitations

## Publish a release candidate

From an up-to-date local clone:

```console
git switch main
git pull --ff-only
git tag -a VERSION -m "Tidy Branches VERSION"
git push origin VERSION
```

The tag starts the release workflow. Do not create a second release manually while that workflow is running.

After the workflow publishes and validates the assets, apply the reviewed notes without restoring GitHub's prerelease flag during the first RC series:

```console
gh release edit VERSION \
  --repo teamleaderleo/gh-tidy-branches \
  --notes-file docs/RELEASE_NOTES_VERSION.md \
  --prerelease=false \
  --latest
```

Use the actual release-notes filename; the command above shows the intended metadata behaviour rather than a shell-ready variable expansion.

## Validate the published extension

Inspect the asset names. The current supported set is:

```text
gh-tidy-branches-darwin-amd64
gh-tidy-branches-darwin-arm64
gh-tidy-branches-linux-amd64
gh-tidy-branches-linux-arm64
gh-tidy-branches-windows-amd64.exe
```

Then use a clean GitHub CLI extension installation:

```console
gh extension remove tidy-branches 2>/dev/null || true
gh extension install teamleaderleo/gh-tidy-branches --pin VERSION
gh tidy-branches --version
gh tidy-branches --help
gh tidy-branches doctor
```

Run a non-mutating repository scan:

```console
gh tidy-branches --preview -R teamleaderleo/smolrunner
```

Also validate configured and explicitly repeated repository selection:

```console
gh tidy-branches config add teamleaderleo/smolrunner
gh tidy-branches config add teamleaderleo/glossless
gh tidy-branches --all --preview

gh tidy-branches --preview \
  -R teamleaderleo/smolrunner \
  -R teamleaderleo/glossless
```

## Failure handling

When a release workflow or published installation fails:

1. do not reuse or move the affected tag after changing code or binaries
2. identify the exact failed layer
3. fix the problem on `main`
4. add a deterministic regression check at the closest practical layer
5. create a new tag only when source or binary contents must change
6. correct release metadata in place only when the tag and assets remain unchanged
7. record the failure and correction in the release history

Never silently replace a release binary under an existing tag.

## Stable release

Promote to `v0.1.0` only after:

- installation succeeds on an Apple Silicon Mac
- installation succeeds on at least one Linux system
- the dry-run candidate set is reviewed against known repositories
- a controlled deletion and undo test succeeds
- the stable JSON schema and selected-candidate apply interface are documented and tested
- failure exit codes behave as documented
- known limitations are documented in the release notes
