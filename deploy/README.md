# JASM Deployment Manifests

This directory contains Kubernetes manifests for deploying JASM using raw YAML or Kustomize.

## Directory Structure

```
deploy/
├── base/                    # Base Kubernetes manifests
│   ├── namespace.yaml       # Namespace definition
│   ├── service_account.yaml # ServiceAccount
│   ├── role.yaml           # ClusterRole
│   ├── role_binding.yaml   # ClusterRoleBinding
│   ├── deployment.yaml     # Controller deployment
│   └── kustomization.yaml  # Base kustomization
└── overlays/               # Environment-specific overlays
    ├── dev/                # Development environment
    │   ├── kustomization.yaml
    │   ├── deployment-patch.yaml
    │   └── secrets.env.example
    └── prod/               # Production environment
        ├── kustomization.yaml
        ├── deployment-patch.yaml
        └── service-account-patch.yaml
```

## Deployment Options

### Option 1: Raw Manifests

Deploy using raw Kubernetes manifests:

```bash
# Apply all base manifests
kubectl apply -f deploy/base/

# Verify deployment
kubectl -n jasm get pods
```

### Option 2: Kustomize (Recommended)

Deploy using Kustomize for environment-specific configurations:

#### Development Environment

```bash
# Create secrets file (optional, for static credentials)
cd deploy/overlays/dev
cp secrets.env.example secrets.env
# Edit secrets.env with your AWS credentials

# Deploy
kubectl apply -k deploy/overlays/dev/

# Verify
kubectl -n jasm-dev get pods
```

#### Production Environment

```bash
# Configure IRSA annotation if using EKS
# Edit deploy/overlays/prod/service-account-patch.yaml

# Deploy
kubectl apply -k deploy/overlays/prod/

# Verify
kubectl -n jasm get pods
```

## Configuration

### Base Configuration

The base manifests include:
- **Namespace**: `jasm`
- **ServiceAccount**: `jasm`
- **ClusterRole**: Permissions for pods, secrets, and events
- **Deployment**: Single replica controller with health probes

### Environment Overlays

#### Development (`overlays/dev/`)

- **Namespace**: `jasm-dev`
- **Name prefix**: `dev-`
- **Replicas**: 1
- **Resources**: Lower limits (50m CPU, 32Mi memory)
- **Image tag**: `latest`
- **AWS Auth**: Static credentials via secrets
- **Logging**: Debug level, console output

#### Production (`overlays/prod/`)

- **Namespace**: `jasm`
- **Replicas**: 2 (with leader election)
- **Resources**: Standard limits (100m CPU, 64Mi memory)
- **Image tag**: `v1.0.0` (pinned version)
- **AWS Auth**: IRSA (recommended)
- **Logging**: Info level, JSON output

## Customization

### Changing AWS Region

Edit the configMap in the overlay:

```yaml
# deploy/overlays/prod/kustomization.yaml
configMapGenerator:
  - name: jasm-config
    literals:
      - AWS_REGION=eu-west-1  # Change region
```

### Changing Image Version

Edit the image tag in the overlay:

```yaml
# deploy/overlays/prod/kustomization.yaml
images:
  - name: ghcr.io/codnod/jasm
    newTag: v1.2.0  # Pin to specific version
```

### Adding Resource Limits

Edit the deployment patch:

```yaml
# deploy/overlays/prod/deployment-patch.yaml
spec:
  template:
    spec:
      containers:
      - name: controller
        resources:
          limits:
            cpu: 500m
            memory: 256Mi
```

### Configuring IRSA (EKS)

Uncomment and configure the service account annotation:

```yaml
# deploy/overlays/prod/service-account-patch.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: jasm
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/jasm-role
```

### Configuring Logging

JASM uses structured logging with configurable levels. The overlays include pre-configured logging settings:

**Development logging** (verbose, console output):
```yaml
# deploy/overlays/dev/deployment-patch.yaml
args:
- --zap-log-level=debug        # Verbose debug output
- --zap-devel=true              # Development mode
- --zap-encoder=console         # Human-readable console format
```

**Production logging** (structured, JSON output):
```yaml
# deploy/overlays/prod/deployment-patch.yaml
args:
- --zap-log-level=info          # Normal operations only
- --zap-devel=false             # Production mode
- --zap-encoder=json            # Structured JSON for log aggregation
- --zap-stacktrace-level=error  # Stacktraces only on errors
```

#### Available Log Levels

- `debug` - Very verbose, shows all reconciliation details
- `info` - Normal operations (recommended for production)
- `error` - Only errors and failures
- `panic` - Only critical failures

#### Customizing Log Level

Create a patch or modify the deployment args:

```yaml
# Custom logging configuration
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jasm
spec:
  template:
    spec:
      containers:
      - name: controller
        args:
        - --zap-log-level=debug
        - --zap-encoder=json
        - --zap-stacktrace-level=panic
```

#### Viewing Logs

```bash
# View logs
kubectl logs -n jasm -l app=jasm

# Follow logs
kubectl logs -n jasm -l app=jasm -f

# View logs from specific pod
kubectl logs -n jasm <pod-name>

# Filter JSON logs with jq
kubectl logs -n jasm -l app=jasm | jq -r 'select(.level=="error")'
```

## Validation

Test your kustomization before applying:

```bash
# Preview manifests
kubectl kustomize deploy/overlays/prod/

# Validate
kubectl kustomize deploy/overlays/prod/ | kubectl apply --dry-run=client -f -
```

## Cleanup

```bash
# Remove specific environment
kubectl delete -k deploy/overlays/dev/

# Or remove base
kubectl delete -f deploy/base/
```

## GitOps Integration

This structure is designed for GitOps workflows (ArgoCD, Flux):

```yaml
# ArgoCD Application example
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: jasm
spec:
  source:
    repoURL: https://github.com/codnod/jasm
    targetRevision: main
    path: deploy/overlays/prod
  destination:
    server: https://kubernetes.default.svc
    namespace: jasm
```
