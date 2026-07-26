# GitHub CLI Extension Release Playbook

This guide records the contracts, failure modes, and verification steps that mattered while publishing Tidy Branches. It is written to be reusable for a future precompiled GitHub CLI extension rather than as a project-specific victory lap.

## The most important lesson

A successful build is not a successful release. A successful GitHub Release is not an installable extension. The release is complete only after a clean machine can run the same command a user will run:

```console
gh extension install OWNER/gh-COMMAND --pin TAG
gh COMMAND --version
```

Every release workflow should test that path against the published release.

## 1. Repository and executable naming

GitHub CLI derives the command name from the repository name.

- repository: `gh-example`
- command: `gh example`
- installed executable: `gh-example`

For local development installation, `gh extension install .` links the repository directory into GitHub CLI's extension directory. It does not compile the project. The repository root therefore needs an executable named exactly like the repository.

A Go project can keep its source under `cmd/gh-example`, but the development workflow must build a root executable before reinstalling:

```console
go build -o gh-example ./cmd/gh-example
gh extension remove example 2>/dev/null || true
gh extension install .
```

### Pitfall: stale local executable

Pulling new source code does not update a previously built root executable. This can produce confusing results such as new source containing a flag while `gh example --help` still reports the old command surface.

Make the supported development command rebuild first:

```make
install-dev: build
	gh extension remove example 2>/dev/null || true
	gh extension install .
	gh example --version
```

CI should also install the extension locally and invoke it through `gh`, not only run the compiled binary directly.

## 2. Version injection

Do not let published binaries report `dev` or an unrelated commit version. Pass the release tag into the build and assert it before publishing.

For Go:

```console
go build \
  -trimpath \
  -ldflags="-s -w -X=main.version=${VERSION}" \
  -o "$OUTPUT" \
  ./cmd/gh-example
```

Then verify:

```console
test "$(./dist/gh-example-linux-amd64 --version)" = "$VERSION"
```

Use one build script for local packaging tests and the release workflow. Avoid maintaining two subtly different release commands.

## 3. Release asset names

The current GitHub CLI installer selects a binary by looking for an asset whose name ends with the current platform suffix:

```text
darwin-amd64
darwin-arm64
linux-amd64
linux-arm64
windows-amd64.exe
```

A clear, conventional naming scheme is:

```text
gh-example-darwin-amd64
gh-example-darwin-arm64
gh-example-linux-amd64
gh-example-linux-arm64
gh-example-windows-amd64.exe
```

The repository-name prefix is useful for humans, release pages, and tooling consistency. The installer itself currently matches the platform suffix, so do not confuse a naming convention with the actual discovery algorithm.

CI should assert the exact intended filename and reject unexpected alternatives:

```console
test -f dist/gh-example-linux-amd64
test ! -e dist/linux-amd64
```

## 4. The first-prerelease discovery trap

This was the least obvious failure.

The current GitHub CLI installation flow first checks the repository's **latest non-prerelease release** to decide whether the repository is a binary extension. Only after that classification succeeds does the installer fetch a release specified by `--pin`.

That means a repository containing only GitHub prereleases can fail like this:

```text
extension is not installable: no usable release artifact or script found
```

The pinned release may exist. Its assets may be valid. Their names may match the platform. The installer can still reject the repository before inspecting the pinned tag.

### Current workaround

Until the first stable release exists, keep the release-candidate identity in the tag, title, and notes, but mark the GitHub Release metadata as latest and non-prerelease:

```console
gh release edit v0.1.0-rc.1 \
  --repo OWNER/gh-example \
  --prerelease=false \
  --latest
```

Confirm the latest-release endpoint sees it:

```console
gh api repos/OWNER/gh-example/releases/latest --jq '.tag_name'
```

This is a GitHub CLI discovery workaround, not a change to the tag or binary contents. Once a stable release exists, normal prerelease metadata can be used for later candidates because the repository already has a non-prerelease binary release for classification.

## 5. Release workflow shape

A dependable release workflow should perform all of these operations in one job:

1. Check out the tagged commit.
2. Build every supported platform through the same tested script.
3. Embed the exact tag in every binary.
4. Publish assets and attestations.
5. Apply any required release-metadata compatibility step.
6. Install the published extension through `gh extension install`.
7. Assert the installed command reports the exact tag.
8. Remove the test installation.

The final verification should resemble:

```console
gh extension install OWNER/gh-example --pin "$GITHUB_REF_NAME"
test "$(gh example --version)" = "$GITHUB_REF_NAME"
gh extension remove example
```

This catches problems that cross-compilation, asset inspection, and local installation cannot catch.

## 6. What each test layer proves

### Unit and integration tests

Prove application behaviour and safety rules.

### Direct binary smoke test

Proves the program builds and exposes the expected command surface.

### Local extension installation

Proves the repository-root executable and `gh` command dispatch work together.

### Cross-build matrix

Proves the source compiles for each supported target.

### Release-packaging smoke test

Proves the release script creates the intended filenames and embeds the supplied version.

### Published installation test

Proves GitHub Release metadata, assets, GitHub CLI discovery, downloading, installation, and execution all work together.

None of the earlier layers substitutes for the last one.

## 7. Failure handling

When a release candidate fails:

1. Do not move or reuse the tag after changing tagged source or binaries.
2. Preserve the failure as part of the release history.
3. Identify the exact layer that failed.
4. Add a deterministic regression check at the closest practical layer.
5. Publish a new tag only when source or binary contents must change.
6. Correct release metadata in place only when the tag and assets remain unchanged.
7. Never silently replace binaries under an existing tag.

## 8. Tidy Branches release postmortem

### `v0.1.0-rc.1`: build invocation failed

The release action received several `go build` arguments as one quoted option while also supplying its own linker flags. The release workflow failed before creating assets.

**Fix:** replace the fragile argument string with a dedicated build script and exercise that script in pull-request CI.

### `v0.1.0-rc.2`: release published, installation still failed

The release contained valid platform binaries. We initially blamed the unprefixed asset names. Reading the current GitHub CLI installer later showed that diagnosis was incomplete: the installer accepts assets by platform suffix.

The relevant blocker was that the repository had only prereleases, so binary-extension discovery failed before the pinned release was inspected. `rc.2` was not retested after discovering the metadata workaround.

**Lesson:** inspect the consumer's actual discovery path before drawing conclusions from a nearby difference such as naming.

### `v0.1.0-rc.3`: assets were correct, metadata blocked discovery

The release workflow succeeded and published conventionally named assets, but installation still failed while the release was marked as a prerelease. Marking the same release latest and non-prerelease made the existing tag and binaries install successfully.

**Fix:** automate the compatibility metadata step and install the published extension inside the release workflow.

## 9. Reusable checklist

Before tagging:

- [ ] Repository name is `gh-COMMAND`.
- [ ] Root executable is rebuilt for local installation.
- [ ] `gh extension install .` works in CI.
- [ ] Direct and installed help/version smoke tests pass.
- [ ] Release build script is used by both CI and release automation.
- [ ] Every supported platform builds.
- [ ] Asset filenames are asserted exactly.
- [ ] Every packaged binary reports the supplied version.
- [ ] Release notes describe known limitations honestly.

After tagging:

- [ ] Release workflow succeeds.
- [ ] Expected assets and attestations exist.
- [ ] Latest-release discovery returns an installable binary release.
- [ ] Clean `gh extension install OWNER/gh-COMMAND --pin TAG` succeeds.
- [ ] Installed version equals the tag.
- [ ] A non-mutating real command succeeds.
- [ ] Release notes are applied without undoing required metadata compatibility.

## 10. Better default for the next extension

Start with the published-install test in the first release workflow. Do not wait for a failed release to add it.

The practical definition of done is not “the workflow is green.” It is:

```text
A user can install the published tag through GitHub CLI and run the command successfully.
```
