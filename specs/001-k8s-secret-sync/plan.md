# Implementation Plan: JASM - Kubernetes Secret Synchronization Service

**Branch**: `001-k8s-secret-sync` | **Date**: 2025-10-24 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-k8s-secret-sync/spec.md`

## Summary

JASM is a lightweight Kubernetes controller that synchronizes secrets from external providers (AWS Secrets Manager, HashiCorp Vault, Azure Key Vault) to Kubernetes secrets. The controller watches for pod creation events with specific annotations and automatically fetches secrets from the configured provider, creating or updating Kubernetes secrets in the same namespace as the requesting pod.

**Technical Approach**: Event-driven Kubernetes controller built with controller-runtime in Go 1.23+, using a plugin architecture for secret providers, structured logging with logr/Zap, and in-cluster authentication. Initial implementation focuses on AWS Secrets Manager with extensibility for additional providers.

## Technical Context

**Language/Version**: Go 1.23+
**Primary Dependencies**:
- controller-runtime v0.17+ (Kubernetes controller framework)
- AWS SDK for Go v2 (Secrets Manager integration)
- logr with Zap backend (structured logging)
- client-go v0.29+ (Kubernetes API interactions)

**Storage**: N/A (stateless controller, no persistence required)
**Testing**:
- Unit tests: standard Go testing package with table-driven tests
- Integration tests: envtest (Kubernetes test environment)
- E2E tests: Minikube with real AWS Secrets Manager

**Target Platform**: Linux amd64 container running on Kubernetes 1.20+
**Project Type**: Single project (Kubernetes controller/operator)
**Performance Goals**:
- Sync latency < 5 seconds
- Memory < 100MB
- CPU < 0.1 cores steady state
- Handle 100 concurrent pod creations

**Constraints**:
- Event-driven only (no polling)
- Namespace-scoped secret creation
- Lightweight resource footprint
- Support for minikube local testing

**Scale/Scope**: Support clusters with 1000+ pods across 100+ namespaces, handle 50 concurrent pod creation bursts

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: AWS Profile Isolation ✅ PASS

**Requirement**: Project MUST ONLY use AWS profile `codnod` for all AWS operations.

**Implementation**:
- All AWS SDK calls will explicitly use `AWS_PROFILE=codnod` environment variable
- Local testing scripts (Makefile, test runners) MUST validate AWS_PROFILE before execution
- Integration and E2E tests MUST verify profile is set to `codnod`
- RBAC for in-cluster deployment will use IRSA (IAM Roles for Service Accounts), profile restriction applies to local development only

**Pre-flight Validation**:
```bash
# All make targets that interact with AWS must include:
check-aws-profile:
    @if [ "$$AWS_PROFILE" != "codnod" ]; then \
        echo "ERROR: AWS_PROFILE must be 'codnod', currently: $$AWS_PROFILE"; \
        exit 1; \
    fi
```

**Status**: ✅ PASS - No violations. AWS integration is isolated to provider interface, all local testing will enforce profile check.

### Principle II: Kubernetes Context Isolation ✅ PASS

**Requirement**: Project MUST ONLY use `minikube` Kubernetes context for all cluster operations.

**Implementation**:
- All kubectl commands, deployment scripts, and E2E tests MUST target minikube context
- Makefile targets will verify `kubectl config current-context` returns `minikube`
- If minikube is not running, scripts MUST attempt `minikube start` before failing
- In-cluster deployment uses in-cluster config (no context), but development/testing strictly uses minikube

**Pre-flight Validation**:
```bash
check-k8s-context:
    @CURRENT_CONTEXT=$$(kubectl config current-context 2>/dev/null || echo "none"); \
    if [ "$$CURRENT_CONTEXT" != "minikube" ]; then \
        echo "ERROR: Kubernetes context must be 'minikube', currently: $$CURRENT_CONTEXT"; \
        echo "Attempting to start minikube..."; \
        minikube start && kubectl config use-context minikube || exit 1; \
    fi
```

**Status**: ✅ PASS - No violations. All development and testing confined to minikube. Controller uses in-cluster config when deployed.

### Principle III: Workspace Boundary Enforcement ✅ PASS

**Requirement**: All file operations MUST remain within `/Users/tiago/Projects/codnod/jasm`.

**Implementation**:
- All source code, tests, configuration, and generated files within project workspace
- No external file modifications
- Documentation references to external resources (URLs) are read-only
- Build artifacts (binaries, Docker images) created within workspace or ephemeral containers

**Validation**:
- All file paths in code use relative paths from project root
- No symbolic links to external directories
- Build and test scripts operate within workspace

**Status**: ✅ PASS - No violations. Standard Go project structure entirely self-contained.

## Project Structure

### Documentation (this feature)

```text
specs/001-k8s-secret-sync/
├── plan.md              # This file
├── research.md          # Phase 0 output (technical research)
├── data-model.md        # Phase 1 output (entities and data structures)
├── quickstart.md        # Phase 1 output (local setup and testing guide)
├── contracts/           # Phase 1 output (API contracts - N/A for this project)
└── tasks.md             # Phase 2 output (created by /speckit.tasks command)
```

### Source Code (repository root)

```text
caronte/
├── cmd/
│   └── controller/
│       └── main.go              # Entrypoint: manager setup, controller registration
│
├── internal/
│   ├── controller/
│   │   ├── podsecret_controller.go      # Reconciliation logic
│   │   └── podsecret_controller_test.go # Unit tests
│   │
│   ├── provider/
│   │   ├── provider.go          # Provider interface definition
│   │   ├── aws.go              # AWS Secrets Manager implementation
│   │   ├── aws_test.go         # AWS provider unit tests
│   │   └── mock.go             # Mock provider for testing
│   │
│   ├── annotation/
│   │   ├── parser.go           # Annotation parsing logic
│   │   └── parser_test.go      # Parser unit tests
│   │
│   └── events/
│       └── recorder.go          # Kubernetes event helpers
│
├── config/
│   ├── rbac/
│   │   ├── role.yaml           # ClusterRole for watching pods
│   │   ├── role_binding.yaml   # ClusterRoleBinding
│   │   └── service_account.yaml # ServiceAccount
│   │
│   ├── manager/
│   │   └── deployment.yaml      # Controller deployment manifest
│   │
│   └── samples/
│       └── test-pod.yaml        # Sample pod with annotations
│
├── tests/
│   ├── integration/
│   │   ├── suite_test.go       # envtest suite setup
│   │   └── controller_test.go  # Integration tests
│   │
│   └── e2e/
│       ├── setup.sh            # Minikube setup script
│       ├── test_aws_sync.sh    # E2E test for AWS provider
│       └── teardown.sh         # Cleanup script
│
├── Dockerfile                   # Multi-stage build for controller image
├── Makefile                     # Development workflows
├── go.mod                       # Go module definition
├── go.sum                       # Dependency checksums
└── README.md                    # Project documentation
```

**Structure Decision**: Single project (Option 1) selected because JASM is a standalone Kubernetes controller without frontend/backend separation or mobile components. The `internal/` directory follows Go conventions for non-exported packages, while `cmd/` contains the entrypoint.

## Complexity Tracking

**No violations** - Constitution Check passed all gates.

## Phase 0: Research (Completed)

See [research.md](./research.md) for comprehensive technical research covering:
- Kubernetes controller patterns (controller-runtime vs client-go)
- AWS SDK for Go v2 and IRSA authentication
- Plugin architecture for secret providers
- Structured logging with logr/Zap
- In-cluster authentication and RBAC
- Testing strategies (envtest, Minikube E2E)
- Project structure and development workflow

All technical decisions documented with rationale, alternatives considered, and code examples.

## Phase 1: Design & Contracts

Proceeding to generate:
1. `data-model.md` - Entity definitions and data structures
2. `quickstart.md` - Local development setup guide
3. Contracts (N/A - no REST API, controller is Kubernetes-native)
