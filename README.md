# JASM - Just Another Secret Manager

[![GitHub release](https://img.shields.io/github/v/release/codnod/jasm)](https://github.com/codnod/jasm/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/codnod/jasm)](https://goreportcard.com/report/github.com/codnod/jasm)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker Pulls](https://img.shields.io/docker/pulls/codnod/jasm)](https://hub.docker.com/r/codnod/jasm)

JASM (Just Another Secret Manager) is an event-driven, lightweight Kubernetes secrets controller for syncing secrets from AWS Secrets Manager to Kubernetes. Unlike polling-based external secret managers, JASM synchronizes secrets only when pods start, reducing API costs and complexity for small to medium Kubernetes clusters, k3s environments, and edge deployments.

## Why JASM?

Managing external secrets in Kubernetes doesn't have to be complicated or expensive. While solutions like **CSI Secrets Store** require complex driver installations, DaemonSets on every node, and significant resource overhead, and **External Secrets Operator** continuously polls providers (generating API costs and potential rate limits), JASM takes a simpler approach.

Designed for **small to medium Kubernetes clusters**, **k3s deployments**, and **edge environments** where simplicity and efficiency matter, JASM is **event-driven** and **resource-light**. It only fetches secrets when pods actually start—no polling, no unnecessary API calls, no heavy infrastructure. Just annotate your pods, and secrets sync automatically. Perfect for teams who want external secret management without the operational complexity or cloud costs of heavier solutions.

## Use Cases

JASM is ideal for:

- **K3s and Lightweight Kubernetes**: Perfect for Raspberry Pi clusters, edge computing, and IoT deployments
- **Cost-Conscious Cloud Deployments**: Reduce AWS API calls and costs with event-driven secret synchronization
- **Microservices with AWS Secrets Manager**: Sync database credentials, API keys, and configuration secrets to pods
- **Development and Staging Environments**: Simple secret management without complex infrastructure
- **Multi-Tenant Kubernetes**: Namespace-isolated secret synchronization for SaaS platforms
- **EKS Workloads with IRSA**: Native IAM role integration for secure secret access
- **CI/CD Pipelines**: Automated secret injection for containerized applications

## Features

- **Annotation-driven**: Simply annotate your pods to sync secrets
- **AWS Secrets Manager support**: Fetch secrets from AWS with automatic JSON parsing
- **Event-driven**: No polling - secrets are synced when pods start
- **Namespace isolation**: Secrets are created in the same namespace as the pod
- **Structured logging**: Zap-based logging with Kubernetes best practices
- **Health checks**: Built-in liveness and readiness probes
- **Multi-architecture**: Supports amd64 and arm64 platforms
- **Extensible**: Plugin architecture for adding new secret providers
- **Flexible authentication**: Supports IAM roles (IRSA/Kiam), instance profiles, and static credentials

## Example: Kubernetes Secret Sync with AWS Secrets Manager

Here's a simple example of using JASM with a Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
      annotations:
        jasm.codnod.io/secret-sync: |
          provider: aws-secretsmanager
          path: /prod/myapp/database
          secretName: db-credentials
    spec:
      containers:
      - name: app
        image: my-app:latest
        env:
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: DB_HOST
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: DB_PASSWORD
```

When the pods start, JASM automatically:
1. Fetches the secret from AWS Secrets Manager at `/prod/myapp/database`
2. Creates a Kubernetes secret named `db-credentials` in the same namespace
3. Makes it available to your application

## Quick Start: Deploy JASM to Kubernetes

### Using Pre-built Docker Images

Images are available from GitHub Container Registry:

```bash
docker pull ghcr.io/codnod/jasm:latest
```

Available tags: `latest`, `v1.0.0`, `main-sha-abc1234`
Architectures: `linux/amd64`, `linux/arm64`

### Deployment

Choose your deployment method:

#### Option 1: Kustomize (Recommended)

```bash
# Production with IRSA/Instance role
kubectl apply -k deploy/overlays/prod/

# Development with static credentials
cd deploy/overlays/dev
cp secrets.env.example secrets.env
# Edit secrets.env with your AWS credentials
kubectl apply -k .
```

#### Option 2: Raw Manifests

```bash
# Deploy base manifests
kubectl apply -f deploy/base/

# Verify
kubectl -n jasm get pods
```

For detailed deployment options and AWS authentication setup, see:
- **[Deployment Guide](deploy/README.md)** - Raw manifests and Kustomize
- **[AWS Examples](examples/aws/README.md)** - Complete AWS setup and examples

## Usage: Annotating Pods for Secret Synchronization

### Pod Annotation Format

Add the `jasm.codnod.io/secret-sync` annotation to your pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /prod/myapp/config
      secretName: app-credentials
spec:
  containers:
  - name: app
    image: my-app:latest
    volumeMounts:
    - name: secrets
      mountPath: /etc/secrets
      readOnly: true
  volumes:
  - name: secrets
    secret:
      secretName: app-credentials
      optional: true
```

### Annotation Format

The annotation value is YAML with the following fields:

- `provider`: The secret provider (currently supports `aws-secretsmanager`)
- `path`: The path to the secret in the external provider
- `secretName`: The name of the Kubernetes secret to create
- `keys` (optional): Map AWS secret keys to Kubernetes secret key names

#### Key Mapping

You can optionally map AWS secret keys to different Kubernetes secret key names using the `keys` field:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  annotations:
    jasm.codnod.io/secret-sync: |
      provider: aws-secretsmanager
      path: /prod/myapp/credentials
      secretName: app-secrets
      keys:
        database: DB_HOST
        password: DB_PASSWORD
        api_key: EXTERNAL_API_KEY
spec:
  containers:
  - name: app
    image: my-app:latest
    env:
    - name: DB_HOST
      valueFrom:
        secretKeyRef:
          name: app-secrets
          key: database
    - name: DB_PASSWORD
      valueFrom:
        secretKeyRef:
          name: app-secrets
          key: password
    - name: API_KEY
      valueFrom:
        secretKeyRef:
          name: app-secrets
          key: api_key
```

In the example above:
- The AWS secret at `/prod/myapp/credentials` contains: `{"DB_HOST": "host.example.com", "DB_PASSWORD": "secret123", "EXTERNAL_API_KEY": "key789"}`
- JASM maps these keys:
  - `DB_HOST` (AWS) → `database` (Kubernetes)
  - `DB_PASSWORD` (AWS) → `password` (Kubernetes)
  - `EXTERNAL_API_KEY` (AWS) → `api_key` (Kubernetes)

**Note**: When `keys` is provided, only specified keys are included in the Kubernetes secret. If `keys` is omitted, all keys from the AWS secret are copied as-is.

For complete examples, see the [AWS examples directory](examples/aws/).

## Architecture: How JASM Works

### Core Components

- **Controller**: Watches pods and reconciles secrets
- **Provider Interface**: Pluggable architecture for different secret backends
- **Annotation Parser**: Validates and parses pod annotations
- **Event Recorder**: Emits Kubernetes events for observability
- **AWS SDK**: Supports multiple authentication methods (IRSA, instance profiles, static credentials)

### Directory Structure

```
jasm/
├── cmd/
│   └── controller/         # Main entry point
├── internal/
│   ├── annotation/         # Annotation parsing
│   ├── controller/         # Reconciliation logic
│   ├── events/             # Event helpers
│   └── provider/           # Secret provider implementations
├── deploy/
│   ├── base/               # Base Kubernetes manifests
│   └── overlays/           # Kustomize overlays (dev, prod)
├── examples/
│   └── aws/                # AWS Secrets Manager examples
└── .github/
    └── workflows/          # CI/CD pipelines
```

## Development: Building and Running JASM

### Development Requirements

- Go 1.25+
- Docker with buildx support (for multi-arch builds)
- kubectl
- A Kubernetes cluster (minikube, k3s, etc.)

### Build

Build for current architecture:
```bash
make build
```

Build Docker image (defaults to arm64):
```bash
docker build -t jasm:latest .
```

Build for specific architecture:
```bash
docker build --build-arg TARGETARCH=amd64 -t jasm:amd64 .
```

Build multi-architecture images:
```bash
docker buildx build --platform linux/amd64,linux/arm64 -t jasm:latest .
```

### Test

```bash
make test
```

### Run locally

```bash
export AWS_PROFILE=codnod
make run
```

### Linting and formatting

```bash
make fmt
make vet
```

### CI/CD

This project uses GitHub Actions for automated builds and publishing:

- **On push to main**: Builds and publishes `latest` tag to ghcr.io
- **On version tag** (`v*`): Publishes semantic version tags
- **On PR**: Builds images for validation (no push)

Images are automatically published to `ghcr.io/codnod/jasm` with multi-architecture support (amd64/arm64).

## Configuration: AWS Authentication and Settings

### AWS Authentication Methods for Kubernetes

JASM uses the AWS SDK for Go v2, which automatically discovers credentials in this order:

1. **Environment variables**: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
2. **Web Identity Token** (IRSA): Kubernetes service account token mounted at `/var/run/secrets/eks.amazonaws.com/serviceaccount/token`
3. **EC2 Instance Metadata**: IAM role attached to EC2 instances
4. **Shared credentials file**: `~/.aws/credentials` (not available in containers)

### Environment Variables

- `AWS_ACCESS_KEY_ID`: AWS access key (optional, for static credentials)
- `AWS_SECRET_ACCESS_KEY`: AWS secret key (optional, for static credentials)
- `AWS_SESSION_TOKEN`: AWS session token (optional, for temporary credentials)
- `AWS_REGION`: AWS region (default: us-east-1, configurable)
- `AWS_ROLE_ARN`: IAM role ARN for IRSA (optional, auto-configured by EKS)
- `AWS_WEB_IDENTITY_TOKEN_FILE`: Token file path for IRSA (optional, auto-configured by EKS)

### Controller Flags

**Core flags:**
- `--metrics-bind-address`: Metrics server address (default: :8080)
- `--health-probe-bind-address`: Health probe address (default: :8081)
- `--leader-elect`: Enable leader election (default: false)

**Logging flags:**
- `--zap-log-level`: Log level - debug, info, error, panic (default: info)
- `--zap-devel`: Development mode with console output (default: true)
- `--zap-encoder`: Log encoding - console or json (default: console)
- `--zap-stacktrace-level`: When to include stacktraces (default: error)

See [deployment guide](deploy/README.md#configuring-logging) for detailed logging configuration.

## Health Checks

JASM exposes two health endpoints:

- `/healthz`: Liveness probe - returns 200 if the controller is alive
- `/readyz`: Readiness probe - returns 200 if the controller is ready to serve requests

## Observability

### Kubernetes Events

JASM emits events on pods:

- `SecretSyncSuccess`: Secret synchronized successfully
- `AnnotationInvalid`: Invalid annotation format
- `SecretFetchFailed`: Failed to fetch secret from provider
- `ProviderUnsupported`: Unknown provider

### Logs

JASM uses structured logging (Zap) with configurable log levels:

- `debug` - Verbose output, shows all reconciliation details
- `info` - Normal operations (default in production)
- `error` - Only errors and failures
- `panic` - Critical failures only

**View logs:**
```bash
kubectl logs -n jasm -l app=jasm
```

**Configure log level:**

The log level can be adjusted using deployment arguments. See the [deployment guide](deploy/README.md#configuring-logging) for detailed configuration options including:
- Log level control (`--zap-log-level`)
- Output format (console vs JSON)
- Development vs production modes
- Stacktrace configuration

## Troubleshooting

For comprehensive troubleshooting, see the [AWS examples documentation](examples/aws/README.md#troubleshooting).

Quick checks:

```bash
# Check JASM logs
kubectl logs -n jasm -l app=jasm

# Check pod events
kubectl describe pod <pod-name>

# Verify annotation
kubectl get pod <pod-name> -o yaml | grep jasm.codnod.io/secret-sync
```

## Future Enhancements

- [ ] HashiCorp Vault provider
- [ ] Azure Key Vault provider
- [ ] Google Secret Manager provider
- [ ] Secret rotation support
- [ ] Prometheus metrics export
- [ ] Helm chart for easy deployment

## Technical Details

### Technology Stack

- **Language**: Go 1.25+
- **Framework**: controller-runtime (Kubernetes)
- **Container**: Multi-arch Docker images (amd64/arm64)
- **Base Image**: distroless/static (minimal attack surface)
- **Registry**: GitHub Container Registry (ghcr.io)

### Image Details

- **Size**: ~11 MB (compressed)
- **Architectures**: linux/amd64, linux/arm64
- **Base**: gcr.io/distroless/static:nonroot (non-root user 65532)
- **Build**: Multi-stage with Go 1.25-alpine builder

## License

MIT

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `make fmt` and `make vet`
5. Submit a pull request

For bugs and feature requests, please open an issue.
