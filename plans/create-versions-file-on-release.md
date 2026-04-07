# Plan: Create .versions File on Release

## Context

The release workflow builds and publishes kilo-docker binaries and Docker images. Currently, version variables are extracted inline in the "Build host binaries" step. We should create a dedicated step to extract versions as workflow outputs (single source of truth) and use them throughout the workflow.

## Changes

1. **Add "Get versions" step** after semantic-release step to extract and set versions as outputs:
   ```yaml
   - name: Get versions
     id: versions
     if: ${{ steps.release.outputs.new_release_published == 'true' }}
     run: |
       echo "kilo_docker_version=${{ steps.release.outputs.new_release_version }}" >> $GITHUB_OUTPUT
       echo "kilo_version=$(grep -oP 'ARG KILO_VERSION=\K[0-9.]+' Dockerfile)" >> $GITHUB_OUTPUT
   ```

2. **Update "Build host binaries" step** to use `${{ steps.versions.outputs.* }}` instead of inline extraction

3. **Add "Create .versions file" step** before uploading binaries:
   ```yaml
   - name: Create .versions file
     run: |
       echo "KILO_DOCKER_VERSION=v${{ steps.versions.outputs.kilo_docker_version }}" > .versions
       echo "KILO_VERSION=${{ steps.versions.outputs.kilo_version }}" >> .versions
   ```

4. **Update "Upload binaries to release" step** to include `.versions`

## Files Modified

| File | Change |
|------|--------|
| `.github/workflows/release.yml` | Add versions step, create .versions, upload as artifact |

## Verification

1. Run `go vet ./...` to check for syntax errors
2. Review the workflow syntax
