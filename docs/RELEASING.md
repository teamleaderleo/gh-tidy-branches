# Releasing Tidy Branches

Tidy Branches is published as a precompiled GitHub CLI extension. The repository name must remain `gh-tidy-branches`, because GitHub CLI derives the command name from the repository name.

## Release automation

Pushing a tag matching `v*` starts `.github/workflows/release.yml`.

The workflow:

1. checks out the tagged commit
2. builds supported extension binaries with `cli/gh-extension-precompile`
3. embeds the tag in the binary version output
4. creates or updates the matching GitHub Release
5. uploads the compiled assets
6. generates build provenance attestations

Tags containing a hyphen, such as `v0.1.0-rc.1`, are published as prereleases.

## Prerelease checklist

Before tagging:

- `main` is green
- the repository is named `teamleaderleo/gh-tidy-branches`
- the repository has the `gh-extension` topic
- the release commit has no known P0 safety regression
- `gh tidy-branches --help` works in CI
- release version injection is covered by CI
- representative Linux, macOS, and Windows cross-builds pass
- `docs/RELEASE_NOTES_v0.1.0-rc.1.md` accurately describes the release and known limitations

## Publish the first release candidate

From an up-to-date local clone:

```console
git switch main
git pull --ff-only
git tag -a v0.1.0-rc.1 -m "Tidy Branches v0.1.0-rc.1"
git push origin v0.1.0-rc.1
```

The tag starts the release workflow. Do not create a second release manually while that workflow is running.

After the precompile workflow publishes the assets, set the release body from the reviewed notes:

```console
gh release edit v0.1.0-rc.1 \
  --repo teamleaderleo/gh-tidy-branches \
  --notes-file docs/RELEASE_NOTES_v0.1.0-rc.1.md \
  --prerelease
```

## Validate the published extension

Use a clean GitHub CLI extension installation:

```console
gh extension remove tidy-branches 2>/dev/null || true
gh extension install teamleaderleo/gh-tidy-branches --pin v0.1.0-rc.1
gh tidy-branches --version
gh tidy-branches --help
gh tidy-branches doctor
```

Expected version output:

```text
v0.1.0-rc.1
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

Confirm that the release contains binaries for the expected operating systems and architectures, including at minimum:

- Linux amd64
- Linux arm64
- macOS amd64
- macOS arm64
- Windows amd64

Confirm the provenance attestations were generated successfully.

## Failure handling

When the release workflow fails:

1. do not reuse a published tag after changing code
2. fix the problem on `main`
3. create a new prerelease tag, such as `v0.1.0-rc.2`
4. record the failed candidate and reason in issue #2

If a release is published with an unsafe defect, remove or clearly mark the release and publish a corrected candidate. Never silently replace a release binary under an existing tag.

## Stable release

Promote to `v0.1.0` only after:

- installation succeeds on an Apple Silicon Mac
- installation succeeds on at least one Linux system
- the dry-run candidate set is reviewed against known repositories
- a controlled deletion test succeeds
- the stable JSON schema and selected-candidate apply interface are documented and tested
- failure exit codes behave as documented
- known limitations are documented in the release notes
