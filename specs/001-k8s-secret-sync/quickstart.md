# Quickstart Guide: JASM Local Development

**Feature**: 001-k8s-secret-sync
**Date**: 2025-10-24
**Target Audience**: Developers setting up local environment for JASM development

## Prerequisites

Before starting, ensure you have the following installed:

- **Go 1.23+**: [Download](https://go.dev/dl/)
- **Docker**: For building container images
- **Minikube**: For local Kubernetes cluster ([Install Guide](https://minikube.sigs.k8s.io/docs/start/))
- **kubectl**: Kubernetes CLI tool ([Install Guide](https://kubernetes.io/docs/tasks/tools/))
- **AWS CLI v2**: For creating test secrets ([Install Guide](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html))
- **make**: Build automation (pre-installed on macOS/Linux)

## Environment Setup

### 1. Verify AWS Profile

JASM requires the `codnod` AWS profile for all operations per the project constitution.

```bash
# Verify profile exists
aws configure list --profile codnod

# If profile doesn't exist, create it
aws configure --profile codnod
# Enter your AWS credentials when prompted

# Set as default for this session
export AWS_PROFILE=codnod

# Verify it's set
echo $AWS_PROFILE  # Should output: codnod
```

Add to your shell profile (`~/.zshrc`, `~/.bashrc`) to make permanent:
```bash
export AWS_PROFILE=codnod
```

### 2. Start Minikube

Per the project constitution, all Kubernetes operations must target the `minikube` context.

```bash
# Start minikube (if not already running)
minikube start

# Verify context is set to minikube
kubectl config current-context
# Should output: minikube

# Verify cluster is running
kubectl cluster-info
```

**Troubleshooting**:
- If minikube fails to start: `minikube delete && minikube start`
- If context is wrong: `kubectl config use-context minikube`

### 3. Create Test Secret in AWS Secrets Manager

Create a test secret that JASM will sync to Kubernetes.

```bash
# Create the secret (JSON format required)
aws secretsmanager create-secret \
    --name /prod/codnod/config \
    --secret-string '{"DB_HOST":"localhost","DB_USER":"testuser","DB_PASSWORD":"testpass123"}' \
    --description "Test secret for JASM quickstart" \
    --profile codnod

# Verify it was created
aws secretsmanager get-secret-value \
    --secret-id /prod/codnod/config \
    --profile codnod
```

Expected output:
```json
{
    "SecretString": "{\"DB_HOST\":\"localhost\",\"DB_USER\":\"testuser\",\"DB_PASSWORD\":\"testpass123\"}",
    "VersionId": "...",
    "ARN": "arn:aws:secretsmanager:...:secret:/prod/codnod/config-..."
}
```

**Note**: This creates a real secret in AWS. Remember to delete it after testing (see Cleanup section).

## Project Setup

### 1. Initialize Go Module

```bash
# Navigate to project root
cd /Users/tiago/Projects/codnod/jasm

# Initialize Go module
go mod init github.com/codnod/jasm

# Add initial dependencies
go get sigs.k8s.io/controller-runtime@v0.17.0
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/service/secretsmanager@latest
go get github.com/go-logr/logr@latest
go get github.com/go-logr/zapr@latest
go get go.uber.org/zap@latest

# Tidy dependencies
go mod tidy
```

### 2. Create Project Structure

```bash
# Create directory structure
mkdir -p cmd/controller
mkdir -p internal/controller
mkdir -p internal/provider
mkdir -p internal/annotation
mkdir -p internal/events
mkdir -p config/rbac
mkdir -p config/manager
mkdir -p config/samples
mkdir -p tests/integration
mkdir -p tests/e2e

# Verify structure
tree -L 2 .
```

### 3. Create Makefile

Create `Makefile` in project root:

```makefile
# Makefile for JASM

# Project variables
PROJECT_NAME := caronte
DOCKER_IMAGE := codnod/jasm
VERSION := v0.1.0

# Go variables
GOBIN := $(shell go env GOPATH)/bin
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

# Constitution checks
.PHONY: check-aws-profile
check-aws-profile:
	@if [ "$$AWS_PROFILE" != "codnod" ]; then \
		echo "ERROR: AWS_PROFILE must be 'codnod', currently: $$AWS_PROFILE"; \
		exit 1; \
	fi
	@echo "✓ AWS_PROFILE is codnod"

.PHONY: check-k8s-context
check-k8s-context:
	@CURRENT_CONTEXT=$$(kubectl config current-context 2>/dev/null || echo "none"); \
	if [ "$$CURRENT_CONTEXT" != "minikube" ]; then \
		echo "ERROR: Kubernetes context must be 'minikube', currently: $$CURRENT_CONTEXT"; \
		echo "Attempting to start minikube..."; \
		minikube start && kubectl config use-context minikube || exit 1; \
	fi
	@echo "✓ Kubernetes context is minikube"

# Development
.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out ./...

.PHONY: test-integration
test-integration: check-k8s-context
	go test -v -tags=integration ./tests/integration/...

.PHONY: test-e2e
test-e2e: check-aws-profile check-k8s-context
	./tests/e2e/setup.sh
	./tests/e2e/test_aws_sync.sh
	./tests/e2e/teardown.sh

# Build
.PHONY: build
build:
	go build -o bin/controller ./cmd/controller

.PHONY: docker-build
docker-build: check-k8s-context
	eval $$(minikube docker-env); \
	docker build -t $(DOCKER_IMAGE):$(VERSION) .

# Deployment
.PHONY: deploy
deploy: check-k8s-context docker-build
	kubectl apply -f config/rbac/
	kubectl apply -f config/manager/

.PHONY: undeploy
undeploy: check-k8s-context
	kubectl delete -f config/manager/ --ignore-not-found
	kubectl delete -f config/rbac/ --ignore-not-found

# Run locally (for development)
.PHONY: run
run: check-aws-profile check-k8s-context
	go run ./cmd/controller/main.go

# Cleanup
.PHONY: clean
clean:
	rm -rf bin/
	rm -f coverage.out

.PHONY: help
help:
	@echo "JASM Makefile Commands:"
	@echo "  make fmt              - Format Go code"
	@echo "  make vet              - Run Go vet"
	@echo "  make test             - Run unit tests"
	@echo "  make test-integration - Run integration tests (requires minikube)"
	@echo "  make test-e2e         - Run E2E tests (requires minikube + AWS)"
	@echo "  make build            - Build controller binary"
	@echo "  make docker-build     - Build Docker image in minikube"
	@echo "  make deploy           - Deploy to minikube"
	@echo "  make undeploy         - Remove from minikube"
	@echo "  make run              - Run controller locally"
	@echo "  make clean            - Clean build artifacts"
```

## Development Workflow

### 1. Run Unit Tests

```bash
# Run all unit tests
make test

# Run tests for specific package
go test -v ./internal/annotation/...

# Run with coverage
make test
go tool cover -html=coverage.out
```

### 2. Run Controller Locally

Run the controller outside the cluster, connecting to minikube:

```bash
# Ensure environment is correct
make check-aws-profile check-k8s-context

# Run controller
make run
```

The controller will:
- Connect to minikube cluster using your local kubeconfig
- Watch for pod creation events
- Use AWS profile `codnod` to fetch secrets

**In another terminal**, create a test pod:

```bash
# Create test pod with annotation
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-app
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /prod/codnod/config
      secretName: app-credentials
spec:
  containers:
  - name: busybox
    image: busybox:latest
    command: ["sleep", "3600"]
EOF

# Watch for secret creation
kubectl get secrets --watch

# Verify secret was created
kubectl get secret app-credentials -o yaml

# Check pod events
kubectl describe pod test-app
```

Expected secret:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-credentials
  labels:
    app.kubernetes.io/managed-by: caronte
    jasm.codnod.io/provider: aws-secretsmanager
  annotations:
    jasm.codnod.io/source-path: /prod/codnod/config
    jasm.codnod.io/synced-at: "2025-10-24T..."
type: Opaque
data:
  DB_HOST: bG9jYWxob3N0      # base64("localhost")
  DB_USER: dGVzdHVzZXI=      # base64("testuser")
  DB_PASSWORD: dGVzdHBhc3MxMjM=  # base64("testpass123")
```

### 3. Deploy to Minikube

Build and deploy the controller as a pod in minikube:

```bash
# Build Docker image in minikube's Docker daemon
make docker-build

# Deploy RBAC and controller
make deploy

# Watch controller logs
kubectl logs -l app=caronte -f

# Create test pod (same as above)
kubectl apply -f config/samples/test-pod.yaml
```

### 4. Test Updates

Test that secrets are updated when source changes:

```bash
# Update AWS secret
aws secretsmanager update-secret \
    --secret-id /prod/codnod/config \
    --secret-string '{"DB_HOST":"updated-host","DB_USER":"newuser","DB_PASSWORD":"newpass456"}' \
    --profile codnod

# Delete and recreate pod to trigger sync
kubectl delete pod test-app
kubectl apply -f config/samples/test-pod.yaml

# Verify secret updated
kubectl get secret app-credentials -o jsonpath='{.data.DB_HOST}' | base64 -d
# Should output: updated-host
```

## Troubleshooting

### Controller Not Starting

**Symptom**: `make run` fails with connection errors

**Solutions**:
```bash
# Verify minikube is running
minikube status

# Verify kubectl context
kubectl config current-context  # Must be "minikube"

# Check kubeconfig
kubectl cluster-info
```

### AWS Secret Not Found

**Symptom**: Controller logs show "ResourceNotFoundException"

**Solutions**:
```bash
# Verify AWS profile
echo $AWS_PROFILE  # Must be "codnod"

# Verify secret exists
aws secretsmanager describe-secret \
    --secret-id /prod/codnod/config \
    --profile codnod

# Check AWS region matches
aws configure get region --profile codnod
```

### Secret Not Created in Kubernetes

**Symptom**: Pod created but Kubernetes secret doesn't appear

**Solutions**:
```bash
# Check controller logs
kubectl logs -l app=caronte

# Check pod events
kubectl describe pod test-app

# Verify annotation format
kubectl get pod test-app -o jsonpath='{.metadata.annotations}'

# Ensure RBAC is correct
kubectl get clusterrole caronte -o yaml
kubectl get clusterrolebinding caronte -o yaml
```

### Permission Errors

**Symptom**: "AccessDeniedException" from AWS

**Solutions**:
```bash
# Verify AWS credentials
aws sts get-caller-identity --profile codnod

# Test secret access manually
aws secretsmanager get-secret-value \
    --secret-id /prod/codnod/config \
    --profile codnod

# Ensure IAM policy includes secretsmanager:GetSecretValue
```

## Cleanup

### Remove Test Resources

```bash
# Delete Kubernetes resources
kubectl delete pod test-app
kubectl delete secret app-credentials

# Undeploy controller
make undeploy

# Delete AWS secret (IMPORTANT: Avoid costs)
aws secretsmanager delete-secret \
    --secret-id /prod/codnod/config \
    --force-delete-without-recovery \
    --profile codnod

# Stop minikube (optional)
minikube stop
```

## Next Steps

After completing this quickstart:

1. **Review Code Examples**: See `research.md` for detailed code examples
2. **Read Data Model**: Understand entities in `data-model.md`
3. **Implement Core**: Start with provider interface and annotation parser
4. **Add Tests**: Write unit tests following TDD approach
5. **Build Controller**: Implement reconciliation loop
6. **Production Hardening**: Add logging, metrics, and error handling

## Quick Reference

### Constitution Compliance Checks

```bash
# Before any AWS operation
make check-aws-profile

# Before any kubectl operation
make check-k8s-context

# Both checks
make check-aws-profile check-k8s-context
```

### Common Commands

```bash
# Full development cycle
make test && make build && make run

# Deploy and watch logs
make deploy && kubectl logs -l app=caronte -f

# E2E test
make test-e2e

# Clean and rebuild
make clean && make build
```

### Sample Test Pod YAML

Save to `config/samples/test-pod.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: hello-world
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /prod/codnod/config
      secretName: app-credentials
spec:
  containers:
  - name: app
    image: busybox:latest
    command: ["sh", "-c"]
    args:
    - |
      echo "DB_HOST: $(cat /secrets/DB_HOST)"
      echo "DB_USER: $(cat /secrets/DB_USER)"
      sleep 3600
    volumeMounts:
    - name: secrets
      mountPath: /secrets
  volumes:
  - name: secrets
    secret:
      secretName: app-credentials
```

---

**End of Quickstart Guide**

You now have a complete local development environment for JASM. Follow this guide whenever setting up a new development machine or onboarding a new team member.
