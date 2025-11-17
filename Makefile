# Makefile for Caronte

# Project variables
PROJECT_NAME := caronte
DOCKER_IMAGE := codnod/caronte
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
	@echo "Caronte Makefile Commands:"
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
