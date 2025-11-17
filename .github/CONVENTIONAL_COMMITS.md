# Conventional Commits Guide for JASM

JASM uses [Conventional Commits](https://www.conventionalcommits.org/) to enable automatic versioning, changelog generation, and release management through semantic-release.

## Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

## Types

Each commit type maps to semantic versioning:

| Type | Semver | Release | Description |
|------|--------|---------|-------------|
| `feat` | Minor | Yes | A new feature |
| `fix` | Patch | Yes | A bug fix |
| `perf` | Patch | Yes | Performance improvement |
| `refactor` | - | No | Code refactoring without feature/fix |
| `docs` | - | No | Documentation only changes |
| `style` | - | No | Code style changes (formatting, semicolons, etc.) |
| `test` | - | No | Test additions or modifications |
| `ci` | - | No | CI/CD configuration changes |
| `chore` | - | No | Dependency updates, build tools, etc. |

## Breaking Changes

Breaking changes trigger a **major version bump** and must be indicated by adding `!` after the type/scope:

```
feat!: redesign API response structure

This breaks compatibility with versions < 2.0.0
All clients must update their parsers.
```

Or in the footer:

```
feat: redesign API response structure

BREAKING CHANGE: API responses now use camelCase instead of snake_case
```

## Examples

### New Feature
```
feat(controller): add support for secret key mapping

Add optional 'keys' field to pod annotation for mapping AWS secret keys
to custom Kubernetes secret key names. Only specified keys are included
in the resulting Kubernetes secret when mapping is provided.
```

### Bug Fix
```
fix: resolve race condition in secret reconciliation

Ensure secret reconciliation is atomic by using proper locking
mechanisms during concurrent pod creation events.
```

### Performance Improvement
```
perf(provider): optimize AWS API calls with caching

Implement in-memory cache for fetched secrets with configurable
TTL to reduce AWS API calls and improve reconciliation performance.
```

### Breaking Change
```
feat!: redesign annotation format for better extensibility

BREAKING CHANGE: Annotation structure changed from single-level
to nested format. Update all pod annotations from:

  jasm.codnod.io/secret-sync: |
    provider: aws-secretsmanager
    path: /prod/myapp/database
    secretName: db-credentials

To:

  jasm.codnod.io/secret-sync: |
    provider: aws-secretsmanager
    path: /prod/myapp/database
    secretName: db-credentials
    keys:
      host: DB_HOST
      password: DB_PASSWORD
```

### Documentation Update (No Release)
```
docs: clarify IRSA setup instructions

Add more details about OIDC provider configuration for EKS clusters
and troubleshooting steps for common IRSA issues.
```

### Code Refactoring (No Release)
```
refactor: simplify provider interface

Consolidate provider methods for better code organization.
No functional changes.
```

## Scopes (Optional)

Use scopes to indicate which component is affected:

- `controller` - Controller reconciliation logic
- `provider` - Secret provider implementations
- `annotation` - Annotation parsing logic
- `events` - Event recording and logging
- `deploy` - Kubernetes manifests and deployment
- `ci` - GitHub Actions workflows
- `docs` - Documentation

Example:
```
fix(provider): handle AWS rate limiting errors gracefully
```

## Commit Body Guidelines

The body should explain **what** and **why**, not **how**:

**Good:**
```
fix: resolve secret update race condition

When pods are created simultaneously, concurrent secret reconciliation
can result in outdated secret values. The controller now uses a
namespace-level lock to serialize secret updates.

Fixes #123
```

**Avoid:**
```
fix: added locking

We added a mutex to prevent concurrent updates.
```

## Footers

Use footers for metadata:

```
fix: update dependency version

Closes #789
Refs #123, #456
Co-authored-by: Jane Doe <jane@example.com>
```

**Common footers:**
- `Closes #123` - Automatically closes GitHub issue on release
- `Refs #123` - References related issues
- `Co-authored-by:` - Credit co-authors
- `BREAKING CHANGE:` - Indicates breaking changes

## Semantic Release Workflow

1. **Commits are analyzed** - Semantic-release reads commit history since last release
2. **Next version is determined** - Based on commit types:
   - `feat` → increment minor version
   - `fix` → increment patch version
   - `feat!` → increment major version
3. **Changelog is generated** - Commits are grouped by type
4. **Tag is created** - New version is tagged in Git
5. **Release is published** - GitHub Release is created with changelog
6. **Docker image is built** - Multi-arch Docker image is pushed to ghcr.io

## Validation

The CI/CD pipeline validates:
- Tests pass
- Linting passes
- Build succeeds
- Only then is release processing triggered

## Tips

1. **One concern per commit** - Each commit should have a single logical purpose
2. **Write imperative messages** - "add feature" not "added feature"
3. **No commit scope suffix** - Don't add `:` after scope if empty
4. **Reference issues** - Link to GitHub issues in footers
5. **Break lines at 72 characters** - For better readability in logs

## GitHub Integration

When commits are merged to main:
- CI workflow runs (tests, linting, build)
- On success, release workflow triggers automatically
- Semantic-release analyzes commits and creates release if needed
- Changelog is updated automatically
- Docker images are built and pushed
- Version tag is created in Git

No manual release process needed!
