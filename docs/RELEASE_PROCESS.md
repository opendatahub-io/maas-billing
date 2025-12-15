# Release Process

This document describes the tagging and release process for the MaaS Billing project.

## Overview

The project uses a tagging strategy that allows you to specify image tags in kustomize manifests. The `maas-api` image tags are managed through kustomization files, making it easy to update tags across all deployment configurations.

## Tag Management

### Image Tag Locations

Image tags are specified in the following files:

1. **Base Configuration**: `deployment/base/maas-api/kustomization.yaml`
   - Default tag used by base deployments
   - Currently set to: `latest`

2. **Dev Overlay**: `maas-api/deploy/overlays/dev/kustomization.yaml`
   - Tag for development deployments
   - Currently set to: `latest`

3. **ODH Overlay**: `maas-api/deploy/overlays/odh/params.env`
   - Tag for OpenDataHub operator deployments
   - Format: `quay.io/opendatahub/maas-api:<tag>`
   - Currently set to: `latest`

### Updating Tags Manually

You can update tags manually using the provided script:

```bash
./scripts/update-kustomize-tag.sh <tag>
```

Example:
```bash
./scripts/update-kustomize-tag.sh v1.0.0
```

This script will update all kustomization files with the specified tag.

## Creating a Release

### Using GitHub Actions (Recommended)

1. Go to the **Actions** tab in GitHub
2. Select **Create Release** workflow
3. Click **Run workflow**
4. Fill in the form:
   - **Tag**: The release tag (e.g., `v1.0.0`)
   - **Create GitHub release**: Optionally check to create a GitHub release (default: false)
5. Click **Run workflow**

The workflow will:
- Update all kustomization files with the new tag
- Commit the changes
- Create and push a git tag
- Optionally create a GitHub release (you can add release notes manually afterwards)

### Manual Release Process

If you prefer to create a release manually:

1. **Update the tags**:
   ```bash
   ./scripts/update-kustomize-tag.sh v1.0.0
   ```

2. **Commit the changes**:
   ```bash
   git add deployment/base/maas-api/kustomization.yaml
   git add maas-api/deploy/overlays/dev/kustomization.yaml
   git add maas-api/deploy/overlays/odh/params.env
   git commit -m "chore: update image tag to v1.0.0"
   ```

3. **Create and push the tag**:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   git push origin HEAD
   ```

4. **Create a GitHub release** (optional):
   - Go to the Releases page
   - Click "Draft a new release"
   - Select the tag you just created
   - Add release notes
   - Publish the release
   
   Alternatively, you can enable "Create GitHub release" in the workflow, which will create a basic release that you can edit afterwards to add detailed release notes.

## Tag Naming Convention

We recommend using [Semantic Versioning](https://semver.org/):
- Format: `v<major>.<minor>.<patch>`
- Examples: `v1.0.0`, `v1.2.3`, `v2.0.0-beta.1`

The GitHub Action workflow will automatically detect pre-release tags (containing `-`, `alpha`, `beta`, or `rc`) and mark them as pre-releases.

## Notes

- The `maas-api` image itself is built and pushed separately (see `.github/workflows/maas-api-release.yml`)
- This release process only updates the deployment manifests to reference the new image tag
- Deployment scripts in `deployment/scripts/` are not modified by this process (as requested)

