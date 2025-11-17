# Technical Research: JASM Kubernetes Secret Synchronization Service

**Feature**: 001-k8s-secret-sync
**Date**: 2025-10-24
**Go Version**: 1.23+
**Target Platform**: Kubernetes 1.20+

## Executive Summary

This document provides comprehensive technical research for building JASM, a lightweight Kubernetes controller in Go 1.23+ that synchronizes secrets from external providers (AWS Secrets Manager, HashiCorp Vault, Azure Key Vault) to Kubernetes secrets based on pod annotations.

---

## 1. Kubernetes Controller Patterns

### Decision: Use controller-runtime

**Recommendation**: Use `controller-runtime` (sigs.k8s.io/controller-runtime) as the foundation for building the JASM controller.

### Rationale

1. **Higher-Level Abstractions**: controller-runtime provides managed components (Manager, Cache, Controller) that handle boilerplate setup, eliminating the need to manually configure informers, work queues, and event handlers.

2. **Integrated Caching**: Built-in shared cache across controllers that automatically:
   - Reduces API server load by serving Get/List operations from in-memory cache
   - Watches resources and keeps cache synchronized
   - Handles cache invalidation automatically

3. **Declarative Reconciliation**: The reconciliation loop pattern is the idiomatic Kubernetes controller approach - you implement a `Reconcile(ctx, Request)` function and the framework handles:
   - Event-driven triggers (pod creation/updates)
   - Automatic retries with exponential backoff
   - Work queue management
   - Rate limiting

4. **RBAC Integration**: Kubebuilder markers (used with controller-runtime) can auto-generate RBAC manifests from code comments.

5. **Generic Client**: No need for code generation - works with built-in types (Pods, Secrets) and CRDs using the same API.

6. **Production-Ready**: Used by major Kubernetes projects (Cluster API, Operator SDK, Crossplane).

### Alternatives Considered

| Option | Pros | Cons | Why Not Chosen |
|--------|------|------|----------------|
| **client-go directly** | Full control, minimal abstraction | Must manually implement informers, work queues, caching, retry logic, event handlers | Too much boilerplate for this use case; reinventing controller patterns |
| **Operator SDK** | Higher-level than controller-runtime, includes scaffolding | Heavier weight, includes CRD management we don't need | JASM doesn't use CRDs - just watches pods and creates secrets |
| **Kubebuilder** | Excellent scaffolding, documentation | Same as Operator SDK - optimized for CRD-based operators | Can use kubebuilder for initial setup but controller-runtime is the actual dependency |

### Code Structure Recommendations

```go
// Project structure for controller-runtime
cmd/
└── manager/
    └── main.go              // Manager setup, controller registration

internal/
├── controller/
│   └── podsecret/
│       ├── controller.go    // Reconcile logic
│       └── controller_test.go
├── provider/
│   ├── provider.go          // Provider interface
│   ├── aws.go              // AWS Secrets Manager implementation
│   └── mock.go             // Mock for testing
└── webhook/                 // Optional admission webhook (future)

pkg/
└── api/                     // If defining CRDs (future extension)
```

**Manager Setup Pattern**:

```go
// cmd/manager/main.go
func main() {
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme: scheme,
        Metrics: metricsserver.Options{BindAddress: ":8080"},
        HealthProbeBindAddress: ":8081",
        LeaderElection: true,
        LeaderElectionID: "jasm.codnod.io",
    })

    // Register controller
    if err := podsecret.NewReconciler(mgr).SetupWithManager(mgr); err != nil {
        // handle error
    }

    // Add health checks
    mgr.AddHealthzCheck("healthz", healthz.Ping)
    mgr.AddReadyzCheck("readyz", healthz.Ping)

    // Start manager
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        // handle error
    }
}
```

---

## 2. Event-Driven Pod Watching Patterns

### Decision: Use Controller-Runtime Cache with Predicate Filtering

**Recommendation**: Implement a controller that watches Pod resources with predicate filtering to only process pods containing JASM annotations.

### Rationale

1. **Efficiency**: Predicate filtering happens at the controller level (not API server), reducing reconciliation triggers to only annotated pods.

2. **Event Handling**: controller-runtime's cache/informer architecture automatically handles:
   - Watch event distribution
   - Cache consistency
   - Reconnection on watch failures
   - Stale cache detection

3. **Namespace Scoping**: Default behavior watches all namespaces, which is required for JASM to work across the cluster.

4. **Memory Optimization**: While the cache is cluster-scoped by default, predicate filtering prevents unnecessary reconciliation work.

### Implementation Pattern

```go
// internal/controller/podsecret/controller.go
type PodSecretReconciler struct {
    client.Client
    Scheme    *runtime.Scheme
    Providers map[string]provider.SecretProvider
}

func (r *PodSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)

    // Fetch the pod
    var pod corev1.Pod
    if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
        if apierrors.IsNotFound(err) {
            return ctrl.Result{}, nil // Pod deleted, ignore
        }
        return ctrl.Result{}, err
    }

    // Parse annotation
    syncConfig, err := parseAnnotation(pod.Annotations)
    if err != nil {
        // Log error, emit event, don't requeue
        return ctrl.Result{}, nil
    }

    // Fetch secret from provider
    provider := r.Providers[syncConfig.Provider]
    secretData, err := provider.GetSecret(ctx, syncConfig.Path)
    if err != nil {
        // Handle transient vs permanent errors
        if isTransient(err) {
            return ctrl.Result{RequeueAfter: 30*time.Second}, err
        }
        // Permanent error - emit event, log, don't requeue
        return ctrl.Result{}, nil
    }

    // Create/update Kubernetes secret
    if err := r.reconcileSecret(ctx, pod.Namespace, syncConfig.SecretName, secretData); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}

func (r *PodSecretReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&corev1.Pod{}).
        WithEventFilter(predicate.Funcs{
            CreateFunc: func(e event.CreateEvent) bool {
                return hasJASMAnnotation(e.Object)
            },
            UpdateFunc: func(e event.UpdateEvent) bool {
                // Only trigger if annotation changed
                return hasJASMAnnotation(e.ObjectNew) &&
                    annotationChanged(e.ObjectOld, e.ObjectNew)
            },
            DeleteFunc: func(e event.DeleteEvent) bool {
                return false // Don't reconcile on pod deletion
            },
        }).
        Complete(r)
}

func hasJASMAnnotation(obj client.Object) bool {
    _, exists := obj.GetAnnotations()["jasm.codnod.io/secret-sync"]
    return exists
}
```

### Best Practices

1. **Predicate Filtering**: Use `WithEventFilter` to filter events before they hit the reconcile queue.

2. **Efficient Cache Reads**: Get operations are served from cache, List operations should be avoided in reconcile loop.

3. **Watch Ownership**: Optionally watch Secrets with `Owns(&corev1.Secret{})` if implementing garbage collection (future enhancement).

4. **Context Propagation**: Always pass context through the reconciliation chain for proper timeout/cancellation handling.

---

## 3. Reconciliation Loop Best Practices

### Decision: Implement Idempotent Reconciliation with Smart Error Handling

### Key Patterns

#### 3.1 Idempotency

Every reconciliation should be idempotent - running it multiple times produces the same result:

```go
func (r *PodSecretReconciler) reconcileSecret(ctx context.Context, namespace, name string, data map[string][]byte) error {
    secret := &corev1.Secret{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
    }

    // CreateOrUpdate is idempotent
    _, err := ctrl.CreateOrUpdate(ctx, r.Client, secret, func() error {
        // Set data
        secret.Data = data
        // Set owner labels for tracking
        secret.Labels = map[string]string{
            "app.kubernetes.io/managed-by": "caronte",
        }
        return nil
    })

    return err
}
```

#### 3.2 Error Handling Strategy

**Default Exponential Backoff**:
- Base delay: 5ms
- Max delay: ~1000s (16.67 minutes)
- Formula: `baseDelay * 2^<num-failures>`

**Error Handling Decision Tree**:

```go
func (r *PodSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Case 1: Permanent error (bad annotation format, non-existent provider)
    // Return: (Result{}, nil) - don't requeue

    // Case 2: Transient error (network timeout, API throttling)
    // Return: (Result{}, error) - triggers exponential backoff

    // Case 3: Controlled retry (rate limit with known retry-after)
    // Return: (Result{RequeueAfter: duration}, nil)

    // Case 4: Success
    // Return: (Result{}, nil)
}
```

**Error Classification**:

```go
func classifyError(err error) ErrorType {
    // Network timeouts, connection refused
    if isNetworkError(err) {
        return TransientError
    }

    // AWS/Provider throttling
    if isThrottlingError(err) {
        return ThrottlingError // Use RequeueAfter with provider's retry-after
    }

    // 404 not found, invalid path
    if isNotFoundError(err) {
        return PermanentError
    }

    // Permission denied
    if isPermissionError(err) {
        return PermanentError // Don't retry, needs config fix
    }

    return UnknownError // Default to transient, allow retry
}
```

#### 3.3 Rate Limiting

Controller-runtime provides two layers:

1. **Overall Rate Limiting**: Token bucket for the entire controller
2. **Per-Item Rate Limiting**: Exponential backoff per object

Default configuration is usually sufficient, but can be customized:

```go
// Custom rate limiter (advanced use case)
ctrl.NewControllerManagedBy(mgr).
    For(&corev1.Pod{}).
    WithOptions(controller.Options{
        MaxConcurrentReconciles: 3, // Limit concurrent reconciliations
        RateLimiter: workqueue.NewItemExponentialFailureRateLimiter(
            100*time.Millisecond,  // base delay
            1*time.Hour,           // max delay
        ),
    }).
    Complete(r)
```

**Recommendation for JASM**: Use default rate limiter initially. The per-item exponential backoff ensures failed pods don't impact others.

#### 3.4 Context and Timeouts

```go
func (r *PodSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Manager provides context with cancellation
    // Add timeout for external API calls

    fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    secretData, err := provider.GetSecret(fetchCtx, path)
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            // Timeout - treat as transient
            return ctrl.Result{}, err
        }
        return handleError(err)
    }

    return ctrl.Result{}, nil
}
```

### Best Practices Summary

1. **Idempotency**: Use `CreateOrUpdate`, `Patch`, or Server-Side Apply - never assume state.
2. **Error Classification**: Distinguish transient (retry) from permanent (log and move on) errors.
3. **Context Propagation**: Always use the context parameter, add timeouts for external calls.
4. **Structured Logging**: Log reconciliation steps with consistent fields (namespace, pod, provider).
5. **Events**: Emit Kubernetes events on the pod for user visibility.
6. **Metrics**: Export Prometheus metrics for reconciliation duration, success/failure counts.

---

## 4. AWS SDK for Go v2

### Decision: Use AWS SDK for Go v2 with Default Credential Chain

**Recommendation**: Use `github.com/aws/aws-sdk-go-v2` with default configuration for IRSA-based authentication.

### Rationale

1. **IRSA Support**: SDK automatically detects and uses IRSA credentials from the service account token mounted in the pod.

2. **Default Credential Chain**: No explicit authentication code needed - SDK searches:
   - Environment variables
   - Web identity token (IRSA)
   - EC2 instance metadata
   - Shared credentials file

3. **Built-in Retries**: Standard retryer handles throttling, timeouts, and transient errors.

4. **Modern API**: Go v2 uses context, generics, and idiomatic Go patterns.

### Integration with Secrets Manager

```go
// internal/provider/aws.go
package provider

import (
    "context"
    "encoding/json"

    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type AWSSecretsProvider struct {
    client *secretsmanager.Client
}

func NewAWSSecretsProvider(ctx context.Context) (*AWSSecretsProvider, error) {
    // LoadDefaultConfig automatically uses IRSA if available
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"), // Can be overridden by env var AWS_REGION
        config.WithRetryer(func() aws.Retryer {
            return retry.AddWithMaxAttempts(retry.NewStandard(), 5)
        }),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }

    return &AWSSecretsProvider{
        client: secretsmanager.NewFromConfig(cfg),
    }, nil
}

func (p *AWSSecretsProvider) GetSecret(ctx context.Context, path string) (map[string][]byte, error) {
    // Add timeout for API call
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(path),
    }

    result, err := p.client.GetSecretValue(ctx, input)
    if err != nil {
        return nil, classifyAWSError(err)
    }

    // Parse JSON secret
    var data map[string]interface{}
    if err := json.Unmarshal([]byte(*result.SecretString), &data); err != nil {
        return nil, fmt.Errorf("invalid JSON in secret: %w", err)
    }

    // Convert to map[string][]byte for Kubernetes secret
    secretData := make(map[string][]byte)
    for key, value := range data {
        secretData[key] = []byte(fmt.Sprintf("%v", value))
    }

    return secretData, nil
}

func classifyAWSError(err error) error {
    // Use AWS error types to determine if retryable
    var rnfe *types.ResourceNotFoundException
    if errors.As(err, &rnfe) {
        return &PermanentError{Cause: err}
    }

    // Throttling, timeouts are transient
    return &TransientError{Cause: err}
}
```

### IRSA Configuration Best Practices

1. **Service Account Setup**:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: caronte
  namespace: caronte-system
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT:role/caronte-role
```

2. **IAM Role Trust Policy**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT:oidc-provider/OIDC_PROVIDER"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "OIDC_PROVIDER:sub": "system:serviceaccount:caronte-system:caronte"
        }
      }
    }
  ]
}
```

3. **IAM Policy**:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": "arn:aws:secretsmanager:*:ACCOUNT:secret:*"
    }
  ]
}
```

### Error Handling and Retries

```go
// Configure custom retry logic
cfg, err := config.LoadDefaultConfig(ctx,
    config.WithRetryer(func() aws.Retryer {
        return retry.AddWithMaxAttempts(
            retry.AddWithMaxBackoffDelay(
                retry.NewStandard(),
                30*time.Second, // Max backoff delay
            ),
            5, // Max attempts
        )
    }),
)
```

**SDK Default Retryable Errors**:
- HTTP 500, 502, 503, 504
- Throttling, RequestLimitExceeded
- ProvisionedThroughputExceededException

### Alternatives Considered

| Option | Pros | Cons | Why Not Chosen |
|--------|------|------|----------------|
| **AWS SDK for Go v1** | Mature, widely used | Legacy API, pointer-heavy, less idiomatic | v2 is the current standard, better IRSA support |
| **External Secrets Operator** | Full-featured, handles many providers | Heavyweight, CRD-based, different model | JASM is event-driven (pod creation), not polling |
| **Direct AWS API calls** | No SDK dependency | Must implement signing, retries, error handling | Reinventing the wheel |

---

## 5. Plugin Architecture in Go

### Decision: Interface-Based Provider Pattern

**Recommendation**: Use implicit interface implementation with dependency injection for secret providers.

### Rationale

1. **Go Idiomatic**: Interfaces are defined at point of use, not implementation. This allows providers to be developed independently.

2. **Compile-Time Plugins**: No dynamic loading overhead - all providers compiled into binary, selected at runtime via configuration.

3. **Testability**: Easy to mock providers for testing controller logic.

4. **Extensibility**: Adding a new provider requires:
   - Implementing the interface
   - Registering in the provider factory
   - No changes to controller code

### Provider Interface Design

```go
// internal/provider/provider.go
package provider

import "context"

// SecretProvider abstracts external secret sources
type SecretProvider interface {
    // GetSecret fetches a secret from the provider and returns it as key-value pairs
    // Returns error if secret not found, permission denied, or transient failure
    GetSecret(ctx context.Context, path string) (map[string][]byte, error)

    // Name returns the provider identifier (e.g., "aws-secretsmanager")
    Name() string
}

// Factory creates providers by name
type Factory struct {
    providers map[string]SecretProvider
}

func NewFactory() *Factory {
    return &Factory{
        providers: make(map[string]SecretProvider),
    }
}

func (f *Factory) Register(provider SecretProvider) {
    f.providers[provider.Name()] = provider
}

func (f *Factory) Get(name string) (SecretProvider, error) {
    p, exists := f.providers[name]
    if !exists {
        return nil, fmt.Errorf("provider %q not found", name)
    }
    return p, nil
}
```

### Provider Implementation Pattern

```go
// internal/provider/aws.go
type AWSSecretsProvider struct {
    client *secretsmanager.Client
}

func NewAWSSecretsProvider(ctx context.Context, opts ...Option) (*AWSSecretsProvider, error) {
    // Options pattern for configuration
    cfg := defaultConfig()
    for _, opt := range opts {
        opt(cfg)
    }

    awsCfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, err
    }

    return &AWSSecretsProvider{
        client: secretsmanager.NewFromConfig(awsCfg),
    }, nil
}

func (p *AWSSecretsProvider) Name() string {
    return "aws-secretsmanager"
}

func (p *AWSSecretsProvider) GetSecret(ctx context.Context, path string) (map[string][]byte, error) {
    // Implementation from section 4
}

// internal/provider/vault.go (future)
type VaultProvider struct {
    client *vault.Client
}

func (p *VaultProvider) Name() string {
    return "vault"
}

func (p *VaultProvider) GetSecret(ctx context.Context, path string) (map[string][]byte, error) {
    // Vault-specific implementation
}

// internal/provider/mock.go (for testing)
type MockProvider struct {
    GetSecretFunc func(ctx context.Context, path string) (map[string][]byte, error)
}

func (p *MockProvider) Name() string {
    return "mock"
}

func (p *MockProvider) GetSecret(ctx context.Context, path string) (map[string][]byte, error) {
    if p.GetSecretFunc != nil {
        return p.GetSecretFunc(ctx, path)
    }
    return nil, errors.New("not implemented")
}
```

### Dependency Injection in Controller

```go
// cmd/manager/main.go
func main() {
    ctx := context.Background()

    // Create provider factory
    factory := provider.NewFactory()

    // Register providers
    awsProvider, err := provider.NewAWSSecretsProvider(ctx)
    if err != nil {
        log.Fatal(err)
    }
    factory.Register(awsProvider)

    // Future providers
    // vaultProvider, _ := provider.NewVaultProvider(ctx)
    // factory.Register(vaultProvider)

    // Create manager
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{...})

    // Create reconciler with provider factory
    reconciler := podsecret.NewReconciler(mgr.GetClient(), mgr.GetScheme(), factory)
    if err := reconciler.SetupWithManager(mgr); err != nil {
        log.Fatal(err)
    }

    mgr.Start(ctrl.SetupSignalHandler())
}

// internal/controller/podsecret/controller.go
type PodSecretReconciler struct {
    client.Client
    Scheme          *runtime.Scheme
    ProviderFactory *provider.Factory
}

func (r *PodSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Parse annotation to get provider name
    config, err := parseAnnotation(pod.Annotations)

    // Get provider from factory
    provider, err := r.ProviderFactory.Get(config.Provider)
    if err != nil {
        // Unknown provider - permanent error
        return ctrl.Result{}, nil
    }

    // Use provider
    secretData, err := provider.GetSecret(ctx, config.Path)
}
```

### Alternatives Considered

| Option | Pros | Cons | Why Not Chosen |
|--------|------|------|----------------|
| **Go plugin package** | True dynamic loading | Linux-only, versioning issues, complex builds | Over-engineered for this use case |
| **Shared libraries (.so)** | Native plugins | Platform-specific, CGO required, complex | Unnecessary complexity |
| **External processes** | Language-agnostic | IPC overhead, lifecycle management | Performance overhead |
| **Registry pattern** | Global access | Tight coupling, harder to test | Dependency injection is more explicit |

### Best Practices

1. **Interface at Point of Use**: Define the interface in the controller package or a shared package, not in provider implementations.

2. **Small Interfaces**: `SecretProvider` has only two methods - easy to implement and test.

3. **Error Wrapping**: Providers should wrap errors with context:
   ```go
   return nil, fmt.Errorf("aws: failed to get secret %q: %w", path, err)
   ```

4. **Options Pattern**: Use functional options for provider configuration:
   ```go
   provider.NewAWSSecretsProvider(ctx,
       provider.WithRegion("us-west-2"),
       provider.WithEndpoint("http://localstack:4566"),
   )
   ```

---

## 6. Structured Logging with slog

### Decision: Use logr (controller-runtime standard) with Zap Backend

**Recommendation**: Use controller-runtime's logr interface with Zap as the backend, not Go's slog directly.

### Rationale

1. **Ecosystem Standard**: controller-runtime, client-go, and all Kubernetes tooling use logr.

2. **Context Integration**: controller-runtime automatically injects loggers into context with useful fields (controller name, request).

3. **Performance**: Zap is one of the fastest structured logging libraries in Go.

4. **Kubernetes Objects**: Zap backend for logr includes special encoding for Kubernetes objects (shows name, namespace, kind).

5. **Interoperability**: logr and slog can work together if needed, but logr is the primary API.

### Why Not slog?

While Go 1.21+ includes slog as the standard logging library, the Kubernetes ecosystem has standardized on logr. Switching to slog would:
- Lose automatic context propagation from controller-runtime
- Require custom encoders for Kubernetes objects
- Break compatibility with other Kubernetes libraries
- Provide minimal benefit since Zap is already high-performance

### Implementation Pattern

```go
// cmd/manager/main.go
import (
    "go.uber.org/zap/zapcore"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
    // Configure Zap logger
    opts := zap.Options{
        Development: false, // Production mode
        Level:       zapcore.InfoLevel,
        TimeEncoder: zapcore.ISO8601TimeEncoder,
    }

    // Set global logger
    ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

    // Now all controller-runtime components use this logger
    mgr, err := ctrl.NewManager(...)
}
```

### Logging Best Practices in Controllers

```go
func (r *PodSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Get logger from context (automatically includes controller name, request)
    log := log.FromContext(ctx)

    // Structured logging with key-value pairs
    log.Info("reconciling pod",
        "namespace", req.Namespace,
        "pod", req.Name,
    )

    var pod corev1.Pod
    if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
        if apierrors.IsNotFound(err) {
            log.V(1).Info("pod not found, ignoring")
            return ctrl.Result{}, nil
        }
        log.Error(err, "failed to get pod")
        return ctrl.Result{}, err
    }

    // Log Kubernetes objects directly
    log.V(1).Info("fetched pod", "pod", pod)

    // Parse annotation
    config, err := parseAnnotation(pod.Annotations)
    if err != nil {
        log.Error(err, "invalid annotation format",
            "annotation", pod.Annotations["jasm.codnod.io/secret-sync"],
        )
        r.recordEvent(&pod, corev1.EventTypeWarning, "AnnotationError", err.Error())
        return ctrl.Result{}, nil
    }

    // Fetch secret
    provider, err := r.ProviderFactory.Get(config.Provider)
    if err != nil {
        log.Error(err, "unknown provider", "provider", config.Provider)
        return ctrl.Result{}, nil
    }

    log.Info("fetching secret from provider",
        "provider", config.Provider,
        "path", config.Path,
    )

    secretData, err := provider.GetSecret(ctx, config.Path)
    if err != nil {
        log.Error(err, "failed to fetch secret",
            "provider", config.Provider,
            "path", config.Path,
        )
        return ctrl.Result{}, err
    }

    // NEVER log secret values
    log.Info("successfully fetched secret",
        "provider", config.Provider,
        "path", config.Path,
        "keys", len(secretData), // Log count, not values
    )

    return ctrl.Result{}, nil
}
```

### Log Levels

| Level | Usage | Example |
|-------|-------|---------|
| **Error** | Errors that prevent reconciliation | `log.Error(err, "failed to fetch secret")` |
| **Info (V=0)** | Important events, default level | `log.Info("reconciling pod")` |
| **V=1** | Detailed flow, debugging | `log.V(1).Info("fetched pod", "pod", pod)` |
| **V=2** | Very verbose, API calls | `log.V(2).Info("calling AWS API")` |

### Key Naming Conventions

From controller-runtime documentation:

1. **Use lowercase, space-separated keys**: `"pod name"` not `"podName"` or `"pod_name"`
2. **Be consistent**: Use the same key names across the application
3. **Match terminology**: If message says "pod", key should be "pod", not "object"
4. **Brief but descriptive**: `"namespace"` not `"ns"`, but not `"kubernetes_namespace"`

```go
// Good
log.Info("created secret", "namespace", ns, "secret name", name)

// Bad
log.Info("created secret", "ns", ns, "secretName", name)
```

### Kubernetes Object Logging

Controller-runtime's Zap backend automatically formats Kubernetes objects:

```go
log.Info("processing pod", "pod", pod)
// Output: {"msg": "processing pod", "pod": {"name": "app", "namespace": "default", "kind": "Pod", "apiVersion": "v1"}}
```

### Security: Never Log Secret Values

```go
// WRONG - logs secret values
log.Info("fetched secret", "data", secretData)

// CORRECT - log metadata only
log.Info("fetched secret",
    "provider", config.Provider,
    "path", config.Path,
    "key_count", len(secretData),
)
```

### Alternatives Considered

| Option | Pros | Cons | Why Not Chosen |
|--------|------|------|----------------|
| **Go slog** | Standard library, modern | Not Kubernetes ecosystem standard, less integration | Would require custom wrappers for controller-runtime |
| **logrus** | Popular, feature-rich | Slower than Zap, not logr-compatible | Performance and ecosystem reasons |
| **zerolog** | Very fast, low allocation | Not logr-compatible | Integration effort not worth marginal performance gain |

---

## 7. Kubernetes Client Authentication

### Decision: Use In-Cluster Configuration with ServiceAccount

**Recommendation**: Use `ctrl.GetConfigOrDie()` from controller-runtime which automatically detects in-cluster vs out-of-cluster configuration.

### Rationale

1. **Automatic Detection**: controller-runtime handles both:
   - In-cluster: Uses service account token at `/var/run/secrets/kubernetes.io/serviceaccount/token`
   - Out-of-cluster (development): Uses kubeconfig from `~/.kube/config` or `KUBECONFIG` env var

2. **No Explicit Code**: Just call `ctrl.GetConfigOrDie()` and it works in both environments.

3. **Security**: Service account tokens are automatically rotated by Kubernetes.

### RBAC Requirements for JASM

JASM needs permissions to:
- Watch/list/get Pods (all namespaces)
- Create/update/patch Secrets (all namespaces)
- Create Events (for user feedback)

```yaml
# config/rbac/role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: caronte-controller
rules:
# Pod watching
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]

# Secret management
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list", "create", "update", "patch"]

# Event creation for user feedback
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

```yaml
# config/rbac/role_binding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: caronte-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: caronte-controller
subjects:
- kind: ServiceAccount
  name: caronte
  namespace: caronte-system
```

```yaml
# config/rbac/service_account.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: caronte
  namespace: caronte-system
  annotations:
    # For AWS IRSA
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT:role/caronte-role
```

### Deployment Configuration

```yaml
# config/manager/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: caronte-controller
  namespace: caronte-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: caronte
  template:
    metadata:
      labels:
        app: caronte
    spec:
      serviceAccountName: caronte  # Use custom service account
      containers:
      - name: manager
        image: caronte:latest
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop: ["ALL"]
          runAsNonRoot: true
          runAsUser: 65532
        resources:
          limits:
            cpu: 100m
            memory: 128Mi
          requests:
            cpu: 50m
            memory: 64Mi
```

### In-Cluster Authentication Code

```go
// cmd/manager/main.go
func main() {
    // Automatically uses in-cluster config when running in pod
    // Falls back to kubeconfig for local development
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme: scheme,
        // ... other options
    })
    if err != nil {
        log.Fatal("failed to create manager", "error", err)
    }

    // Manager client is already authenticated
    // All reconcilers inherit authentication
}
```

### Development vs Production

**Development (local)**:
```bash
# Uses ~/.kube/config automatically
go run cmd/manager/main.go
```

**Production (in-cluster)**:
```yaml
# ServiceAccount token automatically mounted at:
# /var/run/secrets/kubernetes.io/serviceaccount/token
# /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
# /var/run/secrets/kubernetes.io/serviceaccount/namespace
```

### Security Best Practices

1. **Principle of Least Privilege**: Only grant permissions actually needed.

2. **Namespace-Scoped When Possible**: If JASM only needs to work in specific namespaces, use Role instead of ClusterRole.

3. **Read-Only Where Possible**: JASM only needs read access to Pods, not write.

4. **Audit Logging**: Enable Kubernetes audit logging to track secret creation.

5. **Secret Access Control**: Note that "list" and "watch" on Secrets effectively allows reading secret values - this is necessary for JASM but should be documented as a security consideration.

### Important Security Note

From Kubernetes documentation:

> Permission to create workloads (either Pods, or workload resources that manage Pods) in a namespace implicitly grants access to many other resources in that namespace, such as Secrets, ConfigMaps, and PersistentVolumes that can be mounted in Pods.

JASM's RBAC model requires careful consideration:
- It can create Secrets in any namespace (necessary for functionality)
- It can read Pod specs (necessary to parse annotations)
- Users who can annotate Pods can trigger secret creation

**Mitigation**: Consider validating that the requesting Pod's ServiceAccount has permission to access the secret being created (future enhancement).

---

## 8. Testing Kubernetes Controllers

### Decision: Use envtest for Integration Tests, Table-Driven Tests for Unit Tests

**Recommendation**: Implement three testing layers:
1. Unit tests for provider logic and annotation parsing
2. Integration tests with envtest for controller behavior
3. E2E tests with real Kubernetes (Minikube) for end-to-end validation

### Rationale

1. **envtest Provides Real API Server**: Unlike mocks, envtest spins up a real etcd and kube-apiserver, catching issues like RBAC, webhooks, and API versioning.

2. **Fast Enough**: envtest is lightweight (no kubelet, controllers), starting in ~1-2 seconds.

3. **Standard in Ecosystem**: Kubebuilder, Operator SDK, and controller-runtime all use envtest.

4. **No Flaky Mocks**: Fake clients gradually diverge from real API behavior; envtest stays current with Kubernetes releases.

### Testing Architecture

```
tests/
├── unit/
│   ├── provider/
│   │   ├── aws_test.go          # Unit test AWS provider with localstack/mocks
│   │   └── parser_test.go       # Unit test annotation parsing
│   └── controller/
│       └── logic_test.go         # Test reconciliation logic with mock providers
├── integration/
│   └── controller/
│       └── controller_test.go    # Integration tests with envtest
└── e2e/
    └── scenarios_test.go         # End-to-end tests with Minikube
```

### envtest Setup

```go
// internal/controller/podsecret/suite_test.go
package podsecret_test

import (
    "path/filepath"
    "testing"

    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"

    corev1 "k8s.io/api/core/v1"
    "k8s.io/client-go/kubernetes/scheme"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
    logf "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
    testEnv   *envtest.Environment
    k8sClient client.Client
    ctx       context.Context
    cancel    context.CancelFunc
)

func TestControllers(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
    logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

    ctx, cancel = context.WithCancel(context.TODO())

    By("bootstrapping test environment")
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd")},
        ErrorIfCRDPathMissing: false,
    }

    cfg, err := testEnv.Start()
    Expect(err).NotTo(HaveOccurred())
    Expect(cfg).NotTo(BeNil())

    err = corev1.AddToScheme(scheme.Scheme)
    Expect(err).NotTo(HaveOccurred())

    k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
    Expect(err).NotTo(HaveOccurred())
    Expect(k8sClient).NotTo(BeNil())

    // Start controller manager
    mgr, err := ctrl.NewManager(cfg, ctrl.Options{
        Scheme: scheme.Scheme,
    })
    Expect(err).NotTo(HaveOccurred())

    // Register controller with mock provider
    mockProvider := &provider.MockProvider{
        GetSecretFunc: func(ctx context.Context, path string) (map[string][]byte, error) {
            // Default behavior - can be overridden in tests
            return map[string][]byte{
                "username": []byte("testuser"),
                "password": []byte("testpass"),
            }, nil
        },
    }

    factory := provider.NewFactory()
    factory.Register(mockProvider)

    reconciler := podsecret.NewReconciler(
        mgr.GetClient(),
        mgr.GetScheme(),
        factory,
    )
    err = reconciler.SetupWithManager(mgr)
    Expect(err).NotTo(HaveOccurred())

    go func() {
        defer GinkgoRecover()
        err = mgr.Start(ctx)
        Expect(err).NotTo(HaveOccurred())
    }()
})

var _ = AfterSuite(func() {
    cancel()
    By("tearing down the test environment")
    err := testEnv.Stop()
    Expect(err).NotTo(HaveOccurred())
})
```

### Integration Test Example

```go
// internal/controller/podsecret/controller_test.go
var _ = Describe("PodSecret Controller", func() {
    const (
        timeout  = time.Second * 10
        interval = time.Millisecond * 250
    )

    Context("When creating a Pod with JASM annotation", func() {
        It("Should create a Secret with data from provider", func() {
            ctx := context.Background()

            namespace := &corev1.Namespace{
                ObjectMeta: metav1.ObjectMeta{
                    Name: "test-ns-" + rand.String(5),
                },
            }
            Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

            pod := &corev1.Pod{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-pod",
                    Namespace: namespace.Name,
                    Annotations: map[string]string{
                        "jasm.codnod.io/secret-sync": `{"provider":"mock","path":"/test/secret","secretName":"app-creds"}`,
                    },
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{
                        {
                            Name:  "test",
                            Image: "nginx",
                        },
                    },
                },
            }

            Expect(k8sClient.Create(ctx, pod)).To(Succeed())

            // Eventually assert secret is created
            secret := &corev1.Secret{}
            Eventually(func() bool {
                err := k8sClient.Get(ctx, types.NamespacedName{
                    Name:      "app-creds",
                    Namespace: namespace.Name,
                }, secret)
                return err == nil
            }, timeout, interval).Should(BeTrue())

            // Assert secret data
            Expect(secret.Data).To(HaveKey("username"))
            Expect(secret.Data["username"]).To(Equal([]byte("testuser")))
            Expect(secret.Data).To(HaveKey("password"))
            Expect(secret.Data["password"]).To(Equal([]byte("testpass")))

            // Assert secret is labeled
            Expect(secret.Labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "caronte"))
        })

        It("Should handle invalid annotation gracefully", func() {
            ctx := context.Background()

            namespace := &corev1.Namespace{
                ObjectMeta: metav1.ObjectMeta{
                    Name: "test-ns-" + rand.String(5),
                },
            }
            Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

            pod := &corev1.Pod{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "invalid-pod",
                    Namespace: namespace.Name,
                    Annotations: map[string]string{
                        "jasm.codnod.io/secret-sync": `invalid json`,
                    },
                },
                Spec: corev1.PodSpec{
                    Containers: []corev1.Container{{Name: "test", Image: "nginx"}},
                },
            }

            Expect(k8sClient.Create(ctx, pod)).To(Succeed())

            // Assert no secret is created
            secret := &corev1.Secret{}
            Consistently(func() bool {
                err := k8sClient.Get(ctx, types.NamespacedName{
                    Name:      "app-creds",
                    Namespace: namespace.Name,
                }, secret)
                return err != nil
            }, time.Second*3, interval).Should(BeTrue())

            // Assert warning event is created
            // (Would need to query events API)
        })
    })
})
```

### Unit Test Example (Provider)

```go
// internal/provider/aws_test.go
func TestAWSSecretsProvider_GetSecret(t *testing.T) {
    tests := []struct {
        name        string
        secretPath  string
        mockResp    *secretsmanager.GetSecretValueOutput
        mockErr     error
        wantData    map[string][]byte
        wantErr     bool
        errType     string
    }{
        {
            name:       "successful fetch",
            secretPath: "/prod/db/creds",
            mockResp: &secretsmanager.GetSecretValueOutput{
                SecretString: aws.String(`{"username":"dbuser","password":"dbpass"}`),
            },
            wantData: map[string][]byte{
                "username": []byte("dbuser"),
                "password": []byte("dbpass"),
            },
            wantErr: false,
        },
        {
            name:       "secret not found",
            secretPath: "/nonexistent",
            mockErr:    &types.ResourceNotFoundException{},
            wantErr:    true,
            errType:    "permanent",
        },
        {
            name:       "invalid JSON",
            secretPath: "/prod/db/creds",
            mockResp: &secretsmanager.GetSecretValueOutput{
                SecretString: aws.String(`not json`),
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Mock AWS client
            mockClient := &mockSecretsManagerClient{
                getSecretValueFunc: func(ctx context.Context, input *secretsmanager.GetSecretValueInput, opts ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
                    return tt.mockResp, tt.mockErr
                },
            }

            provider := &AWSSecretsProvider{client: mockClient}

            gotData, err := provider.GetSecret(context.Background(), tt.secretPath)

            if tt.wantErr {
                assert.Error(t, err)
                if tt.errType != "" {
                    // Assert error type
                }
                return
            }

            assert.NoError(t, err)
            assert.Equal(t, tt.wantData, gotData)
        })
    }
}
```

### E2E Test with Minikube

```go
// tests/e2e/scenarios_test.go
func TestE2E_AWSSecretsSync(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test in short mode")
    }

    // Requires:
    // - Minikube running
    // - JASM deployed
    // - AWS credentials configured (localstack or real AWS)

    kubeconfig := os.Getenv("KUBECONFIG")
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    require.NoError(t, err)

    clientset, err := kubernetes.NewForConfig(config)
    require.NoError(t, err)

    ctx := context.Background()
    namespace := "e2e-test-" + rand.String(5)

    // Create namespace
    _, err = clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
        ObjectMeta: metav1.ObjectMeta{Name: namespace},
    }, metav1.CreateOptions{})
    require.NoError(t, err)
    defer clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})

    // Create pod with JASM annotation
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name: "test-app",
            Annotations: map[string]string{
                "jasm.codnod.io/secret-sync": `{"provider":"aws-secretsmanager","path":"/e2e/test","secretName":"app-secret"}`,
            },
        },
        Spec: corev1.PodSpec{
            Containers: []corev1.Container{{Name: "nginx", Image: "nginx:alpine"}},
        },
    }

    _, err = clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
    require.NoError(t, err)

    // Wait for secret to be created
    var secret *corev1.Secret
    require.Eventually(t, func() bool {
        secret, err = clientset.CoreV1().Secrets(namespace).Get(ctx, "app-secret", metav1.GetOptions{})
        return err == nil
    }, 30*time.Second, 1*time.Second, "Secret should be created")

    // Assert secret contents
    assert.NotEmpty(t, secret.Data)
    assert.Contains(t, secret.Data, "DB_HOST")
}
```

### Testing Best Practices

1. **Use Eventually for Asynchronous Assertions**: Controllers are async - use Gomega's `Eventually` for assertions.

2. **Avoid Sleeping**: Never use `time.Sleep` - use `Eventually` or `Consistently` instead.

3. **Isolate Tests**: Each test should create its own namespace and clean up after.

4. **Mock External Dependencies**: Use mock providers for integration tests, real providers for E2E.

5. **Test Error Cases**: Test both happy path and error scenarios (invalid annotations, provider failures, etc.).

6. **Table-Driven Tests**: Use table-driven tests for unit tests with multiple scenarios.

7. **Envtest Limitations**:
   - No namespace deletion (namespaces stay in Terminating state)
   - No built-in controllers (no deployments, replicasets, etc.)
   - Control plane only (no kubelet, no running pods)

### envtest vs Minikube

| Aspect | envtest | Minikube |
|--------|---------|----------|
| **Speed** | Fast (~1-2s startup) | Slow (~30s startup) |
| **Scope** | API server + etcd only | Full cluster |
| **Use Case** | Integration tests | E2E tests |
| **CI Friendly** | Yes | Requires nested virtualization |
| **Real Pods** | No | Yes |
| **Controllers** | No built-in | Yes |

**Recommendation**: Use envtest for CI and fast iteration, Minikube for full E2E validation before releases.

---

## 9. Project Structure Recommendations

### Recommended Directory Layout

```
caronte/
├── cmd/
│   └── manager/
│       └── main.go                 # Entry point, manager setup
│
├── internal/
│   ├── controller/
│   │   └── podsecret/
│   │       ├── controller.go       # Reconciler implementation
│   │       ├── controller_test.go  # Integration tests (envtest)
│   │       ├── suite_test.go       # Test suite setup
│   │       └── events.go           # Event recording helpers
│   │
│   ├── provider/
│   │   ├── provider.go             # Interface and factory
│   │   ├── aws.go                  # AWS Secrets Manager
│   │   ├── aws_test.go             # AWS provider unit tests
│   │   ├── mock.go                 # Mock provider for testing
│   │   └── errors.go               # Error classification
│   │
│   └── annotation/
│       ├── parser.go               # Annotation parsing logic
│       └── parser_test.go          # Parser unit tests
│
├── config/
│   ├── manager/
│   │   ├── deployment.yaml         # Controller deployment
│   │   └── kustomization.yaml
│   │
│   ├── rbac/
│   │   ├── service_account.yaml
│   │   ├── role.yaml
│   │   ├── role_binding.yaml
│   │   └── kustomization.yaml
│   │
│   └── samples/
│       └── pod_with_annotation.yaml
│
├── tests/
│   └── e2e/
│       ├── scenarios_test.go       # E2E tests with Minikube
│       └── README.md               # E2E test instructions
│
├── docs/
│   ├── architecture.md
│   ├── development.md
│   └── deployment.md
│
├── .github/
│   └── workflows/
│       ├── ci.yaml                 # Unit + integration tests
│       └── e2e.yaml                # E2E tests (manual trigger)
│
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

### Rationale for Structure

1. **cmd/**: Entry points for binaries. JASM has one binary (manager).

2. **internal/**: Private application code that cannot be imported by other projects.
   - `controller/`: Controller implementations (one per resource type)
   - `provider/`: Secret provider implementations (AWS, Vault, etc.)
   - `annotation/`: Annotation parsing logic (shared utility)

3. **config/**: Kubernetes manifests organized by concern (RBAC, deployment, samples).

4. **tests/e2e/**: End-to-end tests separate from unit/integration tests.

5. **docs/**: Documentation for developers and operators.

### Why No pkg/?

The `pkg/` directory is for code intended to be imported by external projects. JASM is a standalone controller, not a library, so everything goes in `internal/`.

If JASM later exposes a Go library (e.g., provider interfaces for third-party providers), those would go in `pkg/`.

### Makefile Targets

```makefile
# Development
.PHONY: run
run:
	go run ./cmd/manager/main.go

.PHONY: test
test:
	go test ./internal/... -coverprofile=coverage.out

.PHONY: test-integration
test-integration:
	go test ./internal/controller/... -v

.PHONY: test-e2e
test-e2e:
	go test ./tests/e2e/... -v -timeout 10m

# Build
.PHONY: build
build:
	go build -o bin/manager ./cmd/manager

.PHONY: docker-build
docker-build:
	docker build -t caronte:latest .

# Deployment
.PHONY: deploy
deploy:
	kubectl apply -k config/default

.PHONY: undeploy
undeploy:
	kubectl delete -k config/default

# Code quality
.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint:
	golangci-lint run
```

---

## 10. Summary and Recommendations

### Technology Stack

| Component | Choice | Version |
|-----------|--------|---------|
| **Language** | Go | 1.23+ |
| **Controller Framework** | controller-runtime | v0.16+ |
| **AWS SDK** | aws-sdk-go-v2 | v1.24+ |
| **Logging** | logr + Zap | v1.3+ / v1.26+ |
| **Testing Framework** | Ginkgo + Gomega | v2.13+ / v1.30+ |
| **Integration Testing** | envtest | (part of controller-runtime) |
| **Kubernetes** | client-go | v0.28+ |

### Key Dependencies (go.mod)

```go
module github.com/codnod/jasm

go 1.23

require (
    github.com/aws/aws-sdk-go-v2 v1.24.0
    github.com/aws/aws-sdk-go-v2/config v1.26.0
    github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.26.0
    github.com/onsi/ginkgo/v2 v2.13.0
    github.com/onsi/gomega v1.30.0
    go.uber.org/zap v1.26.0
    k8s.io/api v0.28.4
    k8s.io/apimachinery v0.28.4
    k8s.io/client-go v0.28.4
    sigs.k8s.io/controller-runtime v0.16.3
)
```

### Development Workflow

1. **Setup**:
   ```bash
   # Initialize module
   go mod init github.com/codnod/jasm

   # Add dependencies
   go get sigs.k8s.io/controller-runtime@v0.16.3
   go get github.com/aws/aws-sdk-go-v2@latest
   ```

2. **Local Development**:
   ```bash
   # Run controller locally (uses kubeconfig)
   make run

   # Run tests
   make test
   make test-integration
   ```

3. **Testing with Minikube**:
   ```bash
   # Start Minikube
   minikube start

   # Build and load image
   eval $(minikube docker-env)
   make docker-build

   # Deploy
   make deploy

   # Test E2E
   make test-e2e
   ```

### Implementation Phases

**Phase 1: Core Controller**
- [ ] Project scaffolding
- [ ] Provider interface and mock implementation
- [ ] Annotation parser
- [ ] Controller with reconciliation loop
- [ ] Unit tests

**Phase 2: AWS Integration**
- [ ] AWS Secrets Manager provider
- [ ] IRSA configuration
- [ ] Integration tests with envtest
- [ ] Error handling and retry logic

**Phase 3: Production Readiness**
- [ ] Structured logging throughout
- [ ] Metrics (Prometheus)
- [ ] Health checks
- [ ] RBAC manifests
- [ ] Deployment manifests
- [ ] E2E tests

**Phase 4: Additional Providers (Future)**
- [ ] HashiCorp Vault provider
- [ ] Azure Key Vault provider
- [ ] Plugin discovery/registration

### Security Checklist

- [ ] Never log secret values
- [ ] Use least-privilege RBAC
- [ ] Validate annotation inputs (prevent injection)
- [ ] Use TLS for all external connections
- [ ] Document security model (namespace isolation, RBAC requirements)
- [ ] Run as non-root user
- [ ] Drop all capabilities
- [ ] Use read-only root filesystem (where possible)

### Performance Targets (from spec)

| Metric | Target | How to Achieve |
|--------|--------|----------------|
| **Startup Time** | < 10s | Minimize initialization, use default manager settings |
| **Sync Latency** | < 5s | Event-driven (no polling), fast reconciliation |
| **Memory** | < 100MB | Efficient caching, limit cache scope if needed |
| **CPU** | < 0.1 cores | Event-driven, no continuous work |
| **Concurrency** | 100 pods | Default rate limiting handles this |

### Next Steps

1. **Initialize Project**:
   - Create directory structure
   - Initialize Go module
   - Add dependencies

2. **Implement Core**:
   - Provider interface
   - Mock provider
   - Annotation parser with tests

3. **Build Controller**:
   - Reconciler skeleton
   - Manager setup
   - Basic integration test

4. **Add AWS Provider**:
   - Implement AWS Secrets Manager client
   - Test with localstack/real AWS

5. **Production Hardening**:
   - Add logging, metrics, events
   - Create RBAC manifests
   - Write E2E tests
   - Create Dockerfile and deployment manifests

---

## References

### Documentation

- [Kubebuilder Book](https://book.kubebuilder.io/)
- [controller-runtime Documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [AWS SDK for Go v2 Developer Guide](https://aws.github.io/aws-sdk-go-v2/docs/)
- [Kubernetes RBAC Authorization](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [IRSA Documentation](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html)

### Articles and Best Practices

- [Kubernetes Controllers at Scale](https://medium.com/@timebertt/kubernetes-controllers-at-scale-clients-caches-conflicts-patches-explained-aa0f7a8b4332)
- [Testing Kubernetes Controllers with envtest](https://blog.marcnuri.com/go-testing-kubernetes-applications-envtest)
- [Error Backoff with Controller Runtime](https://stuartleeks.com/posts/error-back-off-with-controller-runtime/)
- [Provider Pattern in Go](https://medium.com/swlh/provider-model-in-go-and-why-you-should-use-it-clean-architecture-1d84cfe1b097)

### GitHub Repositories (Examples)

- [kubernetes-sigs/controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [kubernetes-sigs/kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
- [external-secrets/external-secrets](https://github.com/external-secrets/external-secrets) (reference implementation)

---

**End of Research Document**

This research provides the technical foundation for implementing JASM. All decisions are based on current (2025) best practices in the Kubernetes ecosystem and Go development.
