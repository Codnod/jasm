# Data Model: JASM - Kubernetes Secret Synchronization Service

**Feature**: 001-k8s-secret-sync
**Date**: 2025-10-24
**Purpose**: Define core entities, data structures, and their relationships

## Overview

JASM operates on a simple data model centered around secret synchronization requests extracted from pod annotations. The system is stateless and event-driven, with no persistent storage. All state is derived from Kubernetes resources (pods, secrets) and external providers (AWS Secrets Manager).

## Core Entities

### 1. SecretSyncRequest

**Description**: Represents a parsed secret synchronization request from a pod annotation.

**Purpose**: Encapsulates all information needed to fetch a secret from an external provider and create/update a Kubernetes secret.

**Attributes**:

| Field | Type | Description | Validation Rules |
|-------|------|-------------|------------------|
| `Provider` | string | Provider identifier (e.g., "aws-secretsmanager", "vault", "azure-keyvault") | Required, non-empty, must match supported provider |
| `SecretPath` | string | Path or identifier of secret in the external provider | Required, non-empty, provider-specific format |
| `SecretName` | string | Name of the Kubernetes secret to create/update | Required, valid Kubernetes resource name (DNS-1123 subdomain) |
| `Namespace` | string | Namespace where the secret will be created | Required, same as requesting pod's namespace |
| `PodName` | string | Name of the requesting pod (for logging/events) | Required |
| `PodUID` | string | UID of the requesting pod (for event correlation) | Required |

**State Transitions**: None (immutable value object created from annotations)

**Relationships**:
- Created from Pod annotations
- Used by SecretProvider to fetch external secret
- Results in creation/update of Kubernetes Secret resource

**Example**:
```go
type SecretSyncRequest struct {
    Provider   string
    SecretPath string
    SecretName string
    Namespace  string
    PodName    string
    PodUID     types.UID
}
```

---

### 2. SecretProvider (Interface)

**Description**: Abstraction for external secret sources (AWS Secrets Manager, Vault, Azure Key Vault).

**Purpose**: Provides a unified interface for fetching secrets from different providers, enabling plugin architecture.

**Operations**:

| Operation | Input | Output | Description |
|-----------|-------|--------|-------------|
| `FetchSecret` | `ctx context.Context, path string` | `map[string]string, error` | Fetches secret from provider at given path, returns key-value pairs |
| `Name` | - | `string` | Returns provider identifier (e.g., "aws-secretsmanager") |

**Implementations**:
- `AWSSecretsManagerProvider`: Fetches from AWS Secrets Manager
- `MockProvider`: In-memory provider for testing
- (Future) `VaultProvider`, `AzureKeyVaultProvider`

**Example Interface**:
```go
type SecretProvider interface {
    FetchSecret(ctx context.Context, path string) (map[string]string, error)
    Name() string
}
```

---

### 3. ExternalSecret

**Description**: Represents secret data fetched from an external provider.

**Purpose**: Temporary data structure holding key-value pairs from external source before conversion to Kubernetes Secret.

**Attributes**:

| Field | Type | Description | Constraints |
|-------|------|-------------|-------------|
| `Data` | `map[string]string` | Key-value pairs from external secret | Keys must be valid Kubernetes secret data keys |
| `SourcePath` | string | Original path in external provider | For logging/auditing |
| `Provider` | string | Provider that sourced the secret | For logging/events |

**Validation**:
- All keys must match Kubernetes secret data key format (alphanumeric, `-`, `_`, `.`)
- Total size must not exceed Kubernetes secret limit (1MB)
- Values are base64-encoded when stored in Kubernetes Secret

**Example**:
```go
type ExternalSecret struct {
    Data       map[string]string
    SourcePath string
    Provider   string
}
```

---

### 4. PodAnnotation

**Description**: Structured representation of the `jasm.codnod.io/secret-sync` annotation.

**Purpose**: Parses and validates pod annotations to extract secret synchronization configuration.

**Format** (YAML within annotation):
```yaml
jasm.codnod.io/secret-sync: |
  provider: aws-secretsmanager
  path: /prod/codnod/config
  secretName: app-credentials
```

**Parsed Structure**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provider` | string | Yes | Provider identifier (e.g., "aws-secretsmanager") |
| `path` | string | Yes | Secret path in the external provider |
| `secretName` | string | Yes | Name for the Kubernetes secret to create |

**Validation Rules**:
- Annotation must be valid YAML
- All three fields required
- `provider` must be supported (initial: "aws-secretsmanager")
- `secretName` must be valid Kubernetes name (DNS-1123 label)
- `path` format validated by provider implementation

**Error Cases**:
- Missing annotation → no action (pod is not requesting secret sync)
- Malformed YAML → log error, emit Warning event on pod
- Invalid provider → log error, emit Warning event
- Invalid secretName → log error, emit Warning event

---

## Data Flow

### 1. Pod Creation → Secret Sync

```
1. Pod created with annotation
   ↓
2. Controller receives pod Add/Update event
   ↓
3. AnnotationParser extracts SecretSyncRequest
   ↓
4. Validation (provider supported, fields valid)
   ↓
5. SecretProvider.FetchSecret(path) → ExternalSecret
   ↓
6. Convert ExternalSecret to Kubernetes Secret spec
   ↓
7. Create or Update Kubernetes Secret in pod's namespace
   ↓
8. Emit Kubernetes Event on pod (success or failure)
```

### 2. Error Handling Flow

```
Error at step 3 (parse):
  → Log warning
  → Emit Warning event on pod
  → Do not reconcile

Error at step 4 (validation):
  → Log error with details
  → Emit Warning event on pod
  → Do not reconcile

Error at step 5 (fetch):
  → Log error with provider details
  → Emit Warning event on pod
  → Return error (triggers retry)

Error at step 7 (create/update):
  → Log error with Kubernetes API details
  → Emit Warning event on pod
  → Return error (triggers retry)
```

---

## Kubernetes Resources (External to Controller)

### 1. Pod

**Monitored Resource**: The controller watches for pod Add/Update events.

**Relevant Fields**:
- `metadata.annotations["jasm.codnod.io/secret-sync"]`: Sync configuration
- `metadata.name`: Pod name (for logging/events)
- `metadata.namespace`: Namespace (determines where secret is created)
- `metadata.uid`: Pod UID (for event correlation)

**Controller Actions**:
- Watch: Monitor creation and updates
- Event Emission: Emit Normal/Warning events on pod for sync status

---

### 2. Secret

**Managed Resource**: The controller creates/updates secrets based on sync requests.

**Created Fields**:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <from annotation secretName>
  namespace: <same as pod>
  labels:
    app.kubernetes.io/managed-by: caronte
    jasm.codnod.io/provider: <provider name>
  annotations:
    jasm.codnod.io/source-path: <external secret path>
    jasm.codnod.io/synced-at: <RFC3339 timestamp>
type: Opaque
data:
  <key>: <base64-encoded value>
  ...
```

**Controller Actions**:
- Create: If secret doesn't exist
- Update: If secret exists and data differs (reconcile to external source)
- Label: Mark as managed by JASM for future ownership checks

---

## Key Relationships

```
┌─────────────────┐
│      Pod        │
│  (with annotation) │
└────────┬────────┘
         │ triggers
         ↓
┌─────────────────────┐
│ SecretSyncRequest   │ ←──── parsed from annotation
└────────┬────────────┘
         │ used by
         ↓
┌─────────────────────┐
│ SecretProvider      │ ←──── interface
│   (AWS impl)        │
└────────┬────────────┘
         │ returns
         ↓
┌─────────────────────┐
│  ExternalSecret     │
│  (key-value data)   │
└────────┬────────────┘
         │ converted to
         ↓
┌─────────────────────┐
│ Kubernetes Secret   │ ←──── created in same namespace
└─────────────────────┘
```

---

## Data Validation

### Annotation Validation

**Parser Layer**:
- YAML well-formed
- Required fields present
- Field types correct (all strings)

**Controller Layer**:
- Provider supported
- SecretName valid Kubernetes name
- Namespace matches pod namespace (enforced by controller, not annotation)

### Secret Data Validation

**Provider Layer**:
- External secret exists
- Returned data is valid JSON (AWS) or key-value format
- No null or undefined values

**Controller Layer**:
- All keys valid for Kubernetes secret (match regex: `[-._a-zA-Z0-9]+`)
- Total data size < 1MB
- Values can be base64-encoded

### Error Responses

| Validation Failure | Action | User Feedback |
|-------------------|--------|---------------|
| Annotation parse error | Skip reconcile, emit Warning event | Event message with YAML error |
| Unsupported provider | Skip reconcile, emit Warning event | Event message listing supported providers |
| Invalid secret name | Skip reconcile, emit Warning event | Event message with DNS-1123 rules |
| Secret not found in provider | Retry with backoff, emit Warning event | Event message with provider error |
| Permission denied (AWS IAM) | Retry (may be transient), emit Warning event | Event message with required permissions |
| Secret too large | Skip reconcile, emit Warning event | Event message with size limit |

---

## Immutability and State

**Stateless Controller**: JASM maintains no persistent state. All decisions are made based on:
1. Current pod annotations (desired state)
2. Current Kubernetes secrets (actual state)
3. External provider data (source of truth)

**Reconciliation**: On each pod event or periodic resync, the controller:
1. Fetches current state of external secret
2. Compares with Kubernetes secret (if exists)
3. Creates or updates Kubernetes secret to match external source

**No Caching**: External secrets are fetched on-demand, not cached. This ensures:
- Fresh data on every pod creation
- No stale secret values
- Simpler controller logic
- Lower memory footprint

---

## Provider-Specific Data Models

### AWS Secrets Manager

**Secret Path Format**: `/path/to/secret` or `secret-name`

**Expected Response Format**: JSON object
```json
{
  "DB_HOST": "db.example.com",
  "DB_USER": "admin",
  "DB_PASSWORD": "secret123"
}
```

**Error Cases**:
- Secret not found: ResourceNotFoundException
- Access denied: AccessDeniedException
- Invalid JSON: ParsingException

**Retry Strategy**:
- Transient errors (5xx, throttling): Exponential backoff, max 5 retries
- Permanent errors (404, 403): No retry, emit event

---

### Mock Provider (Testing)

**Secret Path Format**: Any string

**In-Memory Store**: `map[string]map[string]string`

**Operations**:
- `AddSecret(path, data)`: Store secret for testing
- `FetchSecret(path)`: Return stored data or error
- `Clear()`: Reset all secrets

---

## Future Extensions

### Multi-Secret Annotations (v2)

Support multiple secrets per pod:
```yaml
jasm.codnod.io/secret-sync: |
  secrets:
    - provider: aws-secretsmanager
      path: /prod/db/credentials
      secretName: db-credentials
    - provider: vault
      path: secret/data/api-keys
      secretName: api-keys
```

### Secret Versioning (v3)

Track secret versions from providers:
```yaml
annotations:
  jasm.codnod.io/version: "v123"  # AWS version ID
```

### Secret Filtering (v3)

Select specific keys from external secret:
```yaml
jasm.codnod.io/secret-sync: |
  provider: aws-secretsmanager
  path: /prod/all-secrets
  secretName: filtered-secret
  keys:
    - DB_PASSWORD
    - API_KEY
```

---

**End of Data Model Document**

This data model provides a complete view of all entities, their attributes, relationships, validation rules, and data flow for the JASM secret synchronization controller.
