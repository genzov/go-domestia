---
name: release
description: Cut a new go-domestia release - bump the semver version in go_domestia/config.yaml, write the CHANGELOG entry, commit, and push to main to trigger the release workflow.
disable-model-invocation: true
argument-hint: "[patch|minor|major]"
allowed-tools: Bash(git *) Bash(gh *) Bash(sed *) Bash(go *) Bash(gofmt *)
---

# Release

Releases are triggered by pushing a changed `version` in `go_domestia/config.yaml` to `main` (see "Releasing" in AGENTS.md). `release.yml` refuses to run without a matching `## <version>` section in `go_domestia/CHANGELOG.md`.

Bump argument: `$ARGUMENTS` (empty, or one of `patch`, `minor`, `major`).

## 1. Preconditions

Stop and tell the user if any of these fail:

- Current branch is `main` and up to date with `origin/main` (`git fetch`, then compare).
- The working tree is clean, except for changes the user explicitly wants in this release. If there are uncommitted changes, ask whether to include them in the release commit.
- `gofmt -l .` prints nothing, and `go vet ./...` and `go test ./...` pass. (Go may not be on PATH; look in `~/sdk/go*/bin`.)

## 2. Find what changed

The last released version is the `version` in `go_domestia/config.yaml`; releases are tagged with the bare version (e.g. `1.3.1`). List commits since that tag with `git log <tag>..HEAD --oneline`, and read the diffs where a message is unclear. If there are no changes since the tag, stop: there is nothing to release.

## 3. Choose the bump (semantic versioning)

- **patch**: bug fixes, log or documentation changes, internal refactors. No change users must act on.
- **minor**: new backwards-compatible behavior, e.g. a new optional configuration option or a new supported architecture.
- **major**: breaking changes, e.g. a removed or renamed option, a changed required option, or anything that changes `unique_id` (`d_<relay>`) or device identifiers (`domestia_relay_<relay>`) and so breaks existing Home Assistant entities.

If `$ARGUMENTS` is `patch`, `minor` or `major`, use it. Otherwise propose a bump with a one-line justification and confirm it with the user (AskUserQuestion) before continuing. If an explicit argument looks too small for the changes (e.g. `patch` with a breaking change), say so and ask.

Compute the new version from the current one (`1.3.1` -> patch `1.3.2`, minor `1.4.0`, major `2.0.0`; lower components reset to 0).

## 4. Edit files

- `go_domestia/config.yaml`: set `version: "<new>"` (keep the quotes; the workflow parses it with sed).
- `go_domestia/CHANGELOG.md`: add `## <new>` directly under `# Changelog`, above the previous release. Match the existing style: bullet points written for add-on users describing behavior they will notice, not commit summaries. Wrap at about 80 columns. If a change needs user action (a major bump, a changed option), say what to do.

If a configuration option changed in this release, check it was updated everywhere listed under "Changing configuration options" in AGENTS.md, and flag anything missing.

## 5. Confirm, commit, push

Show the user the new version and the changelog section, and ask for confirmation before pushing. Pushing to `main` publishes container images and a GitHub release, which cannot be cleanly undone.

On confirmation:

1. Commit with a message like `Release <new>` (plus a short summary line of the main change), following the commit attribution rules in the session.
2. `git push origin main`.

## 6. Watch the release

Find the triggered run with `gh run list --workflow release.yml --limit 1`, then follow it with `gh run watch <id> --exit-status`. Report the result and, on success, the GitHub release URL (`gh release view <new> --json url`). On failure, show the failing step's log (`gh run view <id> --log-failed`) and do not retry or force-push without asking.

Remind the user to update the add-on in Home Assistant once images are published.
