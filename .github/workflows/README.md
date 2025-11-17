# GitHub Actions Workflows

This directory contains CI/CD workflows for automated building, testing, and publishing of JASM.

## Workflows

### `docker.yml` - Docker Build and Push

Builds multi-architecture Docker images and publishes to GitHub Container Registry.

#### Triggers

- **Push to main branch**: Builds and pushes `latest` tag
- **Version tags** (`v*`): Publishes semantic version tags
- **Pull requests**: Builds for validation (no push)
- **Manual dispatch**: Allows manual workflow runs

#### Jobs

**Build Job**
- Runs in parallel for `linux/amd64` and `linux/arm64` platforms
- Uses Docker Buildx for cross-platform builds
- Leverages GitHub Actions cache for layer caching
- Exports image digest for each platform
- Uploads digests as artifacts

**Merge Job**
- Downloads all platform digests
- Creates multi-architecture manifest list
- Pushes final multi-arch image to GHCR
- Inspects and validates the published image

#### Image Tags

Images are tagged based on the trigger:

- `latest` - Latest build from main branch
- `v1.2.3` - Semantic version from git tag
- `v1.2` - Major.minor version
- `v1` - Major version only
- `main-sha-abc1234` - Branch name + commit SHA
- `pr-123` - Pull request number (not pushed)

#### Registry

Images are published to:
```
ghcr.io/codnod/jasm:TAG
```

#### Permissions

The workflow requires:
- `contents: read` - Read repository contents
- `packages: write` - Publish to GitHub Packages/GHCR
- `id-token: write` - Generate OIDC tokens (future signing)

#### Secrets

Uses built-in `GITHUB_TOKEN` - no manual secret configuration needed.

## Configuration

### Repository Settings

Ensure the following settings are configured:

1. **Actions Permissions**:
   - Settings → Actions → General → Workflow permissions
   - Select "Read and write permissions"

2. **Package Visibility**:
   - After first publish, go to the package settings
   - Set visibility (public for open source)

### Running Workflows Locally

You can test workflows locally using [act](https://github.com/nektos/act):

```bash
# Install act
brew install act

# Test build workflow (dry-run)
act push --dryrun

# Run specific job
act -j build
```

Note: Multi-arch builds may not work fully in act due to platform limitations.

## Optimization Features

### Caching

- **GitHub Actions Cache**: Docker layer cache persisted across builds
- **Mode**: `max` - Caches all layers including intermediate stages
- **Cache Key**: Based on platform and Go module files

### Performance

- **Parallel Builds**: amd64 and arm64 build simultaneously
- **Native Compilation**: Uses cross-compilation instead of QEMU when possible
- **Layer Optimization**: Dockerfile structured for maximum cache hits

## Troubleshooting

### Build Failures

Check the workflow logs:
1. Go to Actions tab in GitHub
2. Click on the failed workflow run
3. Examine the job logs for errors

Common issues:
- Go module cache invalidation: Clear and retry
- GHCR authentication: Check workflow permissions
- Platform-specific errors: Test locally with same architecture

### Publishing Failures

If images fail to push to GHCR:

1. Verify repository workflow permissions
2. Check package permissions if it exists
3. Ensure GITHUB_TOKEN has correct scopes

### Cache Issues

To clear the GitHub Actions cache:

1. Go to Actions → Caches
2. Delete relevant cache entries
3. Re-run the workflow

## Future Improvements

Planned enhancements:

- [ ] Add security scanning (Trivy, Grype)
- [ ] Image signing with Sigstore/Cosign
- [ ] SBOM generation
- [ ] Vulnerability reporting
- [ ] Helm chart publishing
- [ ] Performance benchmarking
