# JASM GitHub Actions CI/CD with Semantic Release

Complete setup and documentation for the automated CI/CD pipeline using GitHub Actions and semantic-release for JASM.

## Overview

The CI/CD pipeline has two main workflows:

1. **CI Workflow** - Validates pull requests and commits
2. **Release Workflow** - Automatically creates releases with semantic versioning

## CI Workflow (Pull Requests & Main Branch)

**File**: `.github/workflows/ci.yml`

Runs on:
- Every pull request to main
- Every push to main

**Jobs**:

### 1. Tests
```bash
go test -v -race -coverprofile=coverage.out ./...
```
- Runs all tests with race condition detection
- Generates coverage reports
- Uploads to Codecov for tracking

### 2. Linting
```bash
golangci-lint run
```
- Checks code quality
- Detects common mistakes
- Enforces Go idioms
- Timeout: 5 minutes

### 3. Build Verification
```bash
go build -v ./cmd/controller/main.go
go run . --help  # Verify it runs
```
- Ensures code compiles
- Verifies binary can execute
- Catches dependency issues

### 4. Docker Build Test
```bash
docker build --platform linux/amd64 -t jasm:test .
```
- Tests Dockerfile is valid
- Uses GitHub Actions cache
- Does NOT push to registry

## Release Workflow (Automatic)

**File**: `.github/workflows/release.yml`

Runs on:
- Every push to main
- Triggered manually via workflow_dispatch

**Jobs**:

### 1. Semantic Release

Uses semantic-release to automatically:

1. **Analyze commits** - Reads git history since last release
2. **Determine version** - Based on commit types:
   - `feat:` → Minor version (1.0.0 → 1.1.0)
   - `fix:` → Patch version (1.0.0 → 1.0.1)
   - `feat!:` → Major version (1.0.0 → 2.0.0)
3. **Generate changelog** - Creates CHANGELOG.md with grouped commits
4. **Create tag** - Tags commit as v1.2.0
5. **Publish release** - Creates GitHub Release with changelog
6. **Update repo** - Pushes changelog back to main

**Configuration**: `.releaserc.json`

Uses these plugins:
- `@semantic-release/commit-analyzer` - Analyzes commit types
- `@semantic-release/release-notes-generator` - Generates changelog
- `@semantic-release/changelog` - Updates CHANGELOG.md
- `@semantic-release/git` - Commits and pushes changes
- `@semantic-release/github` - Creates GitHub Releases

### 2. Build and Push Docker Images

Triggers only if semantic-release created a new version.

Builds and pushes multi-architecture Docker images:
- `linux/amd64` - Intel/AMD 64-bit
- `linux/arm64` - ARM 64-bit (Apple Silicon, Raspberry Pi, etc.)

**Tags**:
```
ghcr.io/codnod/jasm:v1.2.0    # Specific version
ghcr.io/codnod/jasm:latest    # Latest release
```

**Permissions needed**:
- `contents: write` - For creating tags and releases
- `packages: write` - For pushing to ghcr.io

## Commit Message Format

JASM uses **Conventional Commits** for automatic versioning.

**Format**:
```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Type mapping to version**:

| Type | Version | Release | Example |
|------|---------|---------|---------|
| `feat` | Minor ↑ | Yes | `feat(controller): add feature` |
| `fix` | Patch ↑ | Yes | `fix(provider): resolve bug` |
| `perf` | Patch ↑ | Yes | `perf(api): optimize calls` |
| `refactor` | None | No | `refactor: simplify code` |
| `docs` | None | No | `docs: update README` |
| `test` | None | No | `test: add unit tests` |
| `ci` | None | No | `ci: update workflow` |
| `chore` | None | No | `chore: bump dependency` |

**Breaking changes**:

Add `!` after type/scope or use footer:

```
feat!: redesign API response

Breaking change - all clients must update
```

Or:

```
feat: change database schema

BREAKING CHANGE: Old schema no longer supported
```

## Example Flow

### Step 1: Feature Development

```bash
# Create feature branch
git checkout -b feat/key-mapping

# Make changes
# ... edit files ...

# Commit with conventional format
git commit -m "feat: add secret key mapping

Allow pods to rename AWS secret keys in Kubernetes secrets"
```

### Step 2: Create Pull Request

```bash
git push origin feat/key-mapping
# Create PR on GitHub
```

**CI Workflow runs automatically:**
- ✅ Tests pass
- ✅ Linting passes
- ✅ Build succeeds
- ✅ Docker build test passes

### Step 3: Review and Merge

Reviewers approve PR, merge to main.

### Step 4: Release Workflow Runs Automatically

```
Commits analyzed:
  - feat: add secret key mapping
  - fix: resolve race condition

Decision:
  - feat found → bump minor version
  - Current: v1.0.0 → Next: v1.1.0

Actions:
  1. CHANGELOG.md updated with new entries
  2. Git tag created: v1.1.0
  3. GitHub Release published with changelog
  4. Docker images built and pushed:
     - ghcr.io/codnod/jasm:v1.1.0
     - ghcr.io/codnod/jasm:latest
```

No manual steps needed! ✨

## Troubleshooting

### Release not triggered
- Check that commits follow conventional format
- Verify at least one commit is `feat:`, `fix:`, or `perf:`
- Review semantic-release logs in workflow

### Docker build fails
- Ensure Dockerfile is valid
- Check all dependencies are available
- Verify buildx platform support

### Tests failing in CI
- All CI checks must pass before release
- Fix failing tests locally first
- Push fixes to branch before merging

### Changelog not updated
- Semantic-release must have `contents: write` permission
- Check `.releaserc.json` has changelog plugin
- Verify CHANGELOG.md exists or will be created

## Secrets Required

Only one secret is needed:

**`GITHUB_TOKEN`** - Automatically provided by GitHub Actions
- Scoped to repository only
- Permissions: contents:write, packages:write
- Automatically rotated
- No manual configuration needed

## GitHub Branch Protection Rules

Recommended settings for main branch:

1. ✅ Require pull request reviews (at least 1)
2. ✅ Require status checks to pass:
   - `test` - All tests must pass
   - `lint` - Linting must pass
   - `build` - Build must succeed
   - `docker-build` - Docker must build
3. ✅ Require branch to be up to date before merging
4. ✅ Require code reviews before merging
5. ✅ Dismiss stale pull request approvals when new commits are pushed

## Best Practices

### Commits
- One logical change per commit
- Use conventional format always
- Write clear, descriptive messages
- Reference issues: "Closes #123"

### Pull Requests
- Small, focused changes
- Clear description of why
- Link to related issues
- Request appropriate reviewers

### Releases
- Never manually manage versions
- Let semantic-release decide
- Follow conventional commits
- Use breaking change indicator for major versions

### Documentation
- Changelog auto-generated from commits
- Keep README.md current
- Document breaking changes in PR
- Update examples if API changes

## Files Overview

| File | Purpose |
|------|---------|
| `.github/workflows/ci.yml` | PR validation workflow |
| `.github/workflows/release.yml` | Automatic release workflow |
| `.releaserc.json` | Semantic-release configuration |
| `.github/CONVENTIONAL_COMMITS.md` | Commit message guide |
| `.github/CONTRIBUTING.md` | Contributing guidelines |
| `CHANGELOG.md` | Auto-generated release notes (created by semantic-release) |

## Resources

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Semantic Release](https://semantic-release.gitbook.io/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Go Testing](https://golang.org/doc/effective_go#testing)
- [golangci-lint](https://golangci-lint.run/)

## Quick Reference

**Make changes**:
```bash
git checkout -b feat/description
# ... edit files ...
git add .
git commit -m "feat: clear description of what was added"
git push origin feat/description
```

**Create PR** - Let CI run, get review, merge

**Release happens automatically** - No action needed

**Check release**:
```bash
# View GitHub Release
# Check CHANGELOG.md updated
# Check Docker image pushed
# Check Git tag created
```

That's it! Fully automated releases with proper versioning, changelog, and Docker images.
