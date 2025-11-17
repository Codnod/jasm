# Feature Specification: JASM - Kubernetes Secret Synchronization Service

**Feature Branch**: `001-k8s-secret-sync`
**Created**: 2025-10-24
**Status**: Draft
**Input**: User description: "I want a very lightweight service that will run on kubernetes to help me to map secrets from vaults and cloud services to kubernetes secrets. It needs to listen to annotations on deployments/pods, then use it to fetch the secret from the source and create it on the same namespace as the pod."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy Application with External Secrets (Priority: P1)

As a platform engineer, I need to deploy applications that automatically receive secrets from AWS Secrets Manager without manually creating Kubernetes secrets, so that my applications can securely access credentials without exposing them in deployment manifests.

**Why this priority**: This is the core value proposition - enabling applications to access external secrets seamlessly. Without this, the service has no purpose.

**Independent Test**: Can be fully tested by deploying a pod with annotations pointing to an AWS secret, verifying the Kubernetes secret is created automatically in the same namespace with correct key-value pairs from the source.

**Acceptance Scenarios**:

1. **Given** a pod definition with annotations specifying AWS Secrets Manager as source and secret path, **When** the pod is created in a namespace, **Then** a Kubernetes secret is automatically created in the same namespace containing all keys from the AWS secret
2. **Given** an AWS secret at path `/prod/codnod/config` containing keys `DB_HOST`, `DB_USER`, and `DB_PASSWORD`, **When** a pod references this secret via annotations, **Then** the created Kubernetes secret contains all three keys with their corresponding values
3. **Given** multiple pods in different namespaces referencing the same AWS secret path, **When** these pods are created, **Then** each namespace receives its own copy of the Kubernetes secret

---

### User Story 2 - Update Secrets When Source Changes (Priority: P2)

As a platform engineer, when I update a secret value in AWS Secrets Manager, I need the corresponding Kubernetes secret to be updated automatically when new pods are deployed, so that applications always receive the latest credential values without manual intervention.

**Why this priority**: Ensures secret freshness and reduces operational overhead, but the basic sync capability (P1) must work first.

**Independent Test**: Update an AWS secret value, delete and recreate a pod with the same annotations, verify the Kubernetes secret reflects the new value.

**Acceptance Scenarios**:

1. **Given** an existing Kubernetes secret created from an AWS source, **When** the AWS secret value is updated and a new pod with the same annotations is created, **Then** the Kubernetes secret is updated with the new values
2. **Given** a Kubernetes secret that was manually modified, **When** a new pod references the same AWS secret, **Then** the Kubernetes secret is reconciled to match the AWS source values

---

### User Story 3 - Handle Secret Fetch Failures Gracefully (Priority: P1)

As a platform engineer, when a secret cannot be fetched from the external source, I need clear error messages and proper handling so that I can quickly diagnose and fix configuration issues.

**Why this priority**: Critical for operational reliability - silent failures would make the system unusable in production.

**Independent Test**: Configure a pod with annotations pointing to a non-existent secret path, verify the system logs meaningful errors and the pod creation event shows why the secret sync failed.

**Acceptance Scenarios**:

1. **Given** a pod with annotations referencing a non-existent AWS secret path, **When** the pod is created, **Then** the system logs a clear error message indicating the secret was not found and emits a Kubernetes event on the pod
2. **Given** insufficient AWS permissions to access a secret, **When** a pod tries to sync the secret, **Then** the system logs the permission error with the required IAM permissions
3. **Given** AWS API is temporarily unavailable, **When** a pod tries to sync a secret, **Then** the system retries with exponential backoff and logs retry attempts

---

### User Story 4 - Support Multiple Secret Providers (Priority: P3)

As a platform engineer working in a multi-cloud environment, I need to fetch secrets from different providers (AWS Secrets Manager, HashiCorp Vault, Azure Key Vault) using the same annotation pattern, so that I can use a unified approach across different infrastructure.

**Why this priority**: Important for flexibility but can be added incrementally after AWS support is proven.

**Independent Test**: Deploy pods with annotations for different providers (AWS, Vault, Azure), verify each creates the appropriate Kubernetes secret from its respective source.

**Acceptance Scenarios**:

1. **Given** a pod with annotations specifying Vault as the provider, **When** the pod is created, **Then** the secret is fetched from Vault and created as a Kubernetes secret
2. **Given** pods in the same cluster using different providers, **When** they are all created, **Then** each successfully fetches secrets from its respective provider

---

### User Story 5 - Namespace Isolation (Priority: P1)

As a platform engineer managing multi-tenant clusters, I need secrets to be created only in the same namespace as the requesting pod and inaccessible to other namespaces, so that tenant isolation is maintained.

**Why this priority**: Security fundamental - violating namespace isolation would be a critical security flaw.

**Independent Test**: Deploy pods in two different namespaces referencing the same secret path, verify each namespace has its own secret instance and pods cannot access secrets from other namespaces.

**Acceptance Scenarios**:

1. **Given** pods in namespace-a and namespace-b both requesting the same AWS secret, **When** both pods are created, **Then** each namespace contains its own Kubernetes secret and neither can access the other's secret
2. **Given** a pod requesting a secret, **When** the Kubernetes secret is created, **Then** it is created exclusively in the pod's namespace with no cross-namespace references

---

### Edge Cases

- What happens when a pod has malformed annotations (invalid JSON, missing required fields)?
- What happens when the service itself loses permissions to create Kubernetes secrets?
- What happens when two pods in the same namespace request the same secret simultaneously?
- What happens when a secret value exceeds Kubernetes secret size limits (1MB)?
- What happens when AWS returns partial data or corrupted JSON?
- What happens when the annotation specifies a Kubernetes secret name that already exists but wasn't created by the service?
- What happens during service restart - are in-flight secret syncs retried?
- What happens when pods are deleted - should the created secrets be garbage collected?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST monitor pod creation events across all namespaces in the cluster
- **FR-002**: System MUST parse custom annotations from pod specifications to extract secret synchronization instructions
- **FR-003**: System MUST support annotations that specify: provider type (e.g., aws-secretsmanager), secret path in the provider, and destination Kubernetes secret name
- **FR-004**: System MUST fetch secrets from AWS Secrets Manager using the AWS SDK with in-cluster authentication (IAM roles)
- **FR-005**: System MUST create or update Kubernetes secrets in the same namespace as the requesting pod
- **FR-006**: System MUST preserve all key-value pairs from the source secret in the created Kubernetes secret
- **FR-007**: System MUST handle JSON-formatted secrets from AWS Secrets Manager and map each JSON key to a Kubernetes secret data key
- **FR-008**: System MUST operate event-driven (triggered by pod creation) without continuous polling of external secret providers
- **FR-009**: System MUST log all secret synchronization operations including successes, failures, and reasons
- **FR-010**: System MUST emit Kubernetes events on pods when secret synchronization succeeds or fails
- **FR-011**: System MUST use in-cluster Kubernetes authentication for API access
- **FR-012**: System MUST validate annotation format before attempting to fetch secrets
- **FR-013**: System MUST implement graceful error handling with retries for transient failures
- **FR-014**: System MUST support adding new secret providers through a plugin architecture without modifying core logic
- **FR-015**: System MUST prevent cross-namespace secret access or creation
- **FR-016**: System MUST include health check endpoints for Kubernetes liveness and readiness probes
- **FR-017**: System MUST implement panic recovery to prevent controller crashes
- **FR-018**: System MUST support deployment in lightweight environments with minimal resource consumption

### Annotation Format

The system MUST support the following annotation structure (specific format is implementation detail, but must include these components):

- Provider identifier (e.g., aws-secretsmanager, vault, azure-keyvault)
- Secret path/identifier in the source system
- Destination Kubernetes secret name

Example annotation concept (actual format defined during implementation):
```
jasm.codnod.io/secret-sync: |
  provider: aws-secretsmanager
  path: /prod/codnod/config
  secretName: app-credentials
```

### Key Entities

- **Secret Sync Request**: Represents a request to synchronize a secret, containing provider type, source path, destination secret name, and target namespace
- **Secret Provider**: An abstraction for external secret sources (AWS Secrets Manager, Vault, Azure Key Vault) that can fetch secrets given a path
- **Kubernetes Secret**: The target secret object created in the cluster, containing key-value pairs synchronized from the external source
- **Pod Event**: The triggering event when a pod is created, containing annotations that specify secret sync requirements

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Secrets are synchronized from AWS Secrets Manager to Kubernetes within 5 seconds of pod creation
- **SC-002**: System successfully handles 100 concurrent pod creations requesting different secrets without delays
- **SC-003**: Zero cross-namespace secret leaks - all secrets remain isolated to their target namespace
- **SC-004**: Service consumes less than 100MB memory and 0.1 CPU cores during normal operation
- **SC-005**: 99% of secret synchronization requests succeed when the external provider is available
- **SC-006**: All synchronization failures produce actionable error messages in logs and Kubernetes events
- **SC-007**: Service recovers automatically from crashes within 30 seconds without manual intervention
- **SC-008**: Documentation enables a new user to deploy and test the service in under 30 minutes
- **SC-009**: Adding a new secret provider requires changes to only provider-specific plugin code, not core controller logic

## Assumptions

- The Kubernetes cluster has network connectivity to external secret providers (AWS Secrets Manager, etc.)
- The service runs with appropriate RBAC permissions to watch pods and create/update secrets
- For AWS Secrets Manager, IAM roles for service accounts (IRSA) or similar mechanisms provide authentication
- External secrets are formatted as JSON key-value pairs
- The service is deployed as a Kubernetes Deployment or DaemonSet (deployment model TBD during planning)
- Minikube or similar local cluster is available for testing
- Users have basic Kubernetes knowledge (can apply manifests, view logs, describe pods)

## Dependencies

- Kubernetes cluster (version 1.20+) with API access
- AWS account with Secrets Manager containing test secrets
- AWS credentials/permissions configured for local testing (AWS profile)
- Minikube or equivalent for local development and testing

## Out of Scope

- Automatic secret rotation triggers from external providers (only syncs on pod creation)
- Secret versioning or rollback capabilities
- Encrypting secrets beyond Kubernetes' built-in encryption at rest
- Webhook-based secret injection into pods (this creates secrets, pods must reference them)
- Synchronizing secrets from Kubernetes to external providers (one-way sync only: external → Kubernetes)
- Secret lifecycle management (deletion, expiration, automatic cleanup)
- Multi-cluster secret synchronization

## Security Considerations

- The service requires elevated Kubernetes permissions (ability to create secrets in any namespace) - this must be carefully controlled via RBAC
- All communication with external secret providers must use encrypted channels (HTTPS/TLS)
- The service must not log secret values, only metadata (paths, names, operation results)
- Namespace isolation is critical - bugs that allow cross-namespace access would be security vulnerabilities
- The service should validate annotation inputs to prevent injection attacks or malicious configurations

## Performance Expectations

- Startup time under 10 seconds
- Secret synchronization latency under 5 seconds per request
- Support for clusters with 1000+ pods across 100+ namespaces
- Memory footprint under 100MB
- CPU usage under 0.1 cores during steady state
- Ability to handle burst of 50 pod creations simultaneously
