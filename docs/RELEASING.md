# Releasing Tidy Branches

Tidy Branches is published as a precompiled GitHub CLI extension. The repository name must remain `gh-tidy-branches`, because GitHub CLI derives the command name from the repository name.

## Release automation

Pushing a tag matching `v*` starts `.github/workflows/release.yml`.

The workflow:

1. checks out the tagged commit
2. runs `script/build-release.sh` to produce the supported platform binaries
3. embeds the tag in every binary's version output
4. creates or updates the matching GitHub Release through `cli/gh-extension-precompile`
5. uploads the compiled assets
6. generates build provenance attestations
7. makes release-candidate tags visible to GitHub CLI's binary-extension discovery
8. installs the published extension and verifies its exact version

GitHub CLI currently decides whether a repository is a binary extension by inspecting the latest non-prerelease release before it honours `--pin`. A repository containing only GitHub prereleases therefore cannot install its first pinned release candidate. Until `v0.1.0` exists, the workflow keeps the `v0.1.0-rc.*` tag and candidate wording but marks the GitHub Release itself as latest and non-prerelease for installer compatibility. Stable releases need no workaround.

The release build script is exercised in ordinary pull-request CI with a single platform and a synthetic version. The test verifies both the embedded version and GitHub CLI's required `gh-tidy-branches-<os>-<arch>` asset naming convention. The normal cross-build matrix separately covers every supported release target.

## Prerelease checklist

Before tagging:

- `main` is green
- the repository is named `teamleaderleo/gh-tidy-branches`
- the repository has the `gh-extension` topic
- the release commit has no known P0 safety regression
- `gh tidy-branches --help` works in CI
- the release-packaging smoke test verifies the embedded version and exact installable asset name
- representative Linux, macOS, and Windows cross-builds pass
- the matching release-notes file accurately describes the release and known limitations

## Publish the current release candidate

Historical candidates:

- `v0.1.0-rc.1` failed before creating a GitHub Release or uploading assets.
- `v0.1.0-rc.2` published valid binaries with unrecognised filenames such as `darwin-arm64`, so GitHub CLI could not install it.

Do not move, reuse, or silently replace either tag. Publish the corrected candidate from an up-to-date local clone:

```console
git switch main
git pull --ff-only
git tag -a v0.1.0-rc.3 -m "Tidy Branches v0.1.0-rc.3"
git push origin v0.1.0-rc.3
```

The tag starts the release workflow. Do not create a second release manually while that workflow is running.

After the workflow publishes and validates the assets, set the release body from the reviewed notes without restoring GitHub's prerelease flag:

```console
gh release edit v0.1.0-rc.3 \
  --repo teamleaderleo/gh-tidy-branches \
  --notes-file docs/RELEASE_NOTES_v0.1.0-rc.3.md \
  --prerelease=false \
  --latest
```

## Validate the published extension

First inspect the asset names. They must be:

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
gh extension install teamleaderleo/gh-tidy-branches --pin v0.1.0-rc.3
gh tidy-branches --version
gh tidy-branches --help
gh tidy-branches doctor
```

Expected version output:

```text
v0.1.0-rc.3
```

Run a non-mutating repository scan:

```console
gh tidy-branches --preview -R teamleaderleo/smolrunner
```

Then validate configured multi-repository scanning:

```console
gh tidy-branches config add teamleaderleo/smolrunner
gh tidy-branches config add teamleaderleo/glossless
gh tidy-branches --all --preview
```

Also validate the explicit repeated-repository path:

```console
gh tidy-branches --preview \
  -R teamleaderleo/smolrunner \
  -R teamleaderleo/glossless
```

## Validate release assets

Confirm that the release contains the five prefixed binaries listed above and that provenance attestations were generated successfully.

## Failure handling

When the release workflow or published installation fails:

1. do not reuse or move the affected tag after changing code
2. identify and record the exact failure
3. fix the problem on `main`
4. add a deterministic CI test for the failed release path when practical
5. create a new prerelease tag only when the binaries or tagged source must change
6. record the affected candidate and reason in issue #2

Release metadata may be corrected in place when the tag, source, and binary assets remain unchanged. Never silently replace a release binary under an existing tag.

## Stable release

Promote to `v0.1.0` only after:

- installation succeeds on an Apple Silicon Mac
- installation succeeds on at least one Linux system
- the dry-run candidate set is reviewed against known repositories
- a controlled deletion test succeeds
- the stable JSON schema and selected-candidate apply interface are documented and tested
- failure exit codes behave as documented
- known limitations are documented in the release notes
