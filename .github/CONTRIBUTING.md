# Contributing to JASM

Thank you for your interest in contributing to JASM! This document provides guidelines and information for contributors.

## Development Setup

### Prerequisites

- Go 1.25 or later
- Docker with buildx support
- kubectl
- Access to a Kubernetes cluster (minikube, k3s, kind, etc.)
- AWS account for testing (optional)

### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/jasm.git
   cd jasm
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Run tests:
   ```bash
   make test
   ```

## Development Workflow

### Making Changes

1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes following the coding standards below

3. Run formatting and linting:
   ```bash
   make fmt
   make vet
   ```

4. Run tests:
   ```bash
   make test
   ```

5. Commit your changes:
   ```bash
   git commit -m "Description of changes"
   ```

### Coding Standards

- Follow standard Go conventions and idioms
- Use meaningful variable and function names
- Add godoc comments for all exported functions, types, and packages
- Keep functions small and focused
- Use the standard library when possible
- Write tests for new functionality

### Testing

- Write unit tests for new code
- Ensure all tests pass before submitting PR
- Aim for high test coverage on critical paths
- Include integration tests for controller logic

### Documentation

- Update README.md if adding features or changing behavior
- Add godoc comments to all exported items
- Update examples if changing annotation format
- Document configuration options

## Pull Request Process

1. Update documentation with details of changes
2. Add tests covering your changes
3. Ensure all tests pass
4. Update CHANGELOG.md (if applicable)
5. Submit a pull request with a clear description

### PR Title Format

Use conventional commit style:
- `feat: Add new feature`
- `fix: Fix bug in reconciliation`
- `docs: Update README`
- `refactor: Improve error handling`
- `test: Add integration tests`

### PR Description

Include:
- What the PR does
- Why the change is needed
- How to test the changes
- Any breaking changes

## Code Review

- All submissions require review before merging
- Reviewers will check for:
  - Code quality and style
  - Test coverage
  - Documentation completeness
  - Security considerations

## Adding New Secret Providers

To add support for a new secret provider:

1. Create a new file in `internal/provider/` (e.g., `vault.go`)
2. Implement the `Provider` interface:
   ```go
   type Provider interface {
       FetchSecret(ctx context.Context, path string) (map[string]string, error)
   }
   ```
3. Add provider creation logic in `provider.go`
4. Add tests in the corresponding test file
5. Update documentation with usage examples

## Building and Testing Locally

### Build Binary

```bash
make build
```

### Build Docker Image

```bash
# Default (arm64)
docker build -t jasm:dev .

# Specific architecture
docker build --build-arg TARGETARCH=amd64 -t jasm:dev .

# Multi-arch
docker buildx build --platform linux/amd64,linux/arm64 -t jasm:dev .
```

### Run Locally Against Cluster

```bash
# Set AWS credentials
export AWS_PROFILE=your-profile

# Run controller
make run
```

### Deploy to Test Cluster

```bash
# Build image
docker build -t jasm:dev .

# Load into k3s/kind
docker save jasm:dev | sudo k3s ctr images import -

# Update deployment to use jasm:dev
# Deploy
kubectl apply -f config/rbac/
kubectl apply -f config/manager/
```

## GitHub Actions CI/CD

The project uses GitHub Actions for automated testing, validation, and releasing:

### CI Workflow (`.github/workflows/ci.yml`)

Runs on every pull request and push to main:

- **Tests**: `go test -v -race -coverprofile=coverage.out ./...`
- **Linting**: golangci-lint checks code quality
- **Build Verification**: Ensures binary compiles successfully
- **Docker Build Test**: Validates Dockerfile without pushing

All checks must pass before merging to main.

### Release Workflow (`.github/workflows/release.yml`)

Runs on every push to main:

- **Semantic Release**: Automatically determines next version based on commits
- **Changelog Generation**: Creates CHANGELOG.md automatically
- **Git Tag**: Creates version tag (e.g., v1.2.0)
- **GitHub Release**: Publishes release with changelog
- **Docker Build & Push**: Multi-arch images to ghcr.io on successful release

No manual intervention needed!

## Commit Message Format

JASM uses [Conventional Commits](https://www.conventionalcommits.org/) for automatic versioning.

See [CONVENTIONAL_COMMITS.md](.github/CONVENTIONAL_COMMITS.md) for detailed guidelines.

**Quick reference:**

- `feat:` - New feature (triggers minor version bump)
- `fix:` - Bug fix (triggers patch version bump)
- `feat!:` - Breaking change (triggers major version bump)
- `docs:`, `test:`, `ci:`, `chore:` - No release

Example:

```bash
git commit -m "feat(controller): add secret key mapping support

This allows pods to rename AWS secret keys when creating Kubernetes secrets.

Closes #123"
```

## Release Process

**Releases are automated!** No manual steps needed.

1. Make changes in feature branch
2. Create pull request
3. CI workflow runs tests and checks
4. On approval, merge to main
5. Release workflow automatically:
   - Analyzes commits since last release
   - Determines next version (major.minor.patch)
   - Creates CHANGELOG.md entries
   - Creates git tag
   - Publishes GitHub Release
   - Builds and pushes Docker images to ghcr.io

The next version is determined from commits:
- All `fix:` commits → patch version
- Any `feat:` commits → minor version
- Any `feat!:` or breaking changes → major version

Example flow:
```
main branch has commits:
- fix: resolve race condition
- feat: add key mapping support

Result: Version bumped to v1.2.0
Changelog auto-generated from commits
Docker image pushed as ghcr.io/codnod/jasm:v1.2.0 and :latest
```

Images are published to: `ghcr.io/codnod/jasm`

## Getting Help

- Open an issue for bugs or feature requests
- Check existing issues before opening new ones
- Use discussions for questions and ideas

## Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
