# Tasks: JASM - Kubernetes Secret Synchronization Service

**Input**: Design documents from `/specs/001-k8s-secret-sync/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), data-model.md, research.md, quickstart.md

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `cmd/`, `internal/`, `config/`, `tests/` at repository root
- All paths relative to `/Users/tiago/Projects/codnod/jasm/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 Initialize Go module with `go mod init github.com/codnod/jasm`
- [X] T002 Create project directory structure per plan.md
- [X] T003 [P] Add controller-runtime v0.17+ dependency with `go get sigs.k8s.io/controller-runtime@v0.17.0`
- [X] T004 [P] Add AWS SDK for Go v2 dependencies with `go get github.com/aws/aws-sdk-go-v2/config@latest` and `go get github.com/aws/aws-sdk-go-v2/service/secretsmanager@latest`
- [X] T005 [P] Add logging dependencies with `go get github.com/go-logr/logr@latest`, `go get github.com/go-logr/zapr@latest`, and `go get go.uber.org/zap@latest`
- [X] T006 [P] Create Makefile with constitution checks (check-aws-profile, check-k8s-context) per plan.md
- [X] T007 [P] Create .gitignore for Go project (bin/, coverage.out, *.log)
- [X] T008 [P] Create Dockerfile with multi-stage build per plan.md
- [X] T009 Run `go mod tidy` to clean up dependencies

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T010 [P] Define SecretProvider interface in internal/provider/provider.go
- [X] T011 [P] Create MockProvider implementation in internal/provider/mock.go for testing
- [X] T012 [P] Define SecretSyncRequest struct in internal/annotation/parser.go
- [X] T013 [P] Define PodAnnotation struct in internal/annotation/parser.go
- [X] T014 Implement annotation parser with YAML validation in internal/annotation/parser.go
- [X] T015 Create unit tests for annotation parser in internal/annotation/parser_test.go
- [X] T016 [P] Create Kubernetes event recorder helpers in internal/events/recorder.go
- [X] T017 [P] Create basic controller reconciler skeleton in internal/controller/podsecret_controller.go
- [X] T018 [P] Create RBAC ClusterRole manifest in config/rbac/role.yaml (permissions: watch pods, create/update secrets)
- [X] T019 [P] Create RBAC ClusterRoleBinding manifest in config/rbac/role_binding.yaml
- [X] T020 [P] Create ServiceAccount manifest in config/rbac/service_account.yaml

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Deploy Application with External Secrets (Priority: P1) 🎯 MVP

**Goal**: Enable automatic secret synchronization from AWS Secrets Manager to Kubernetes secrets when pods are created with annotations

**Independent Test**: Deploy a pod with annotation pointing to AWS secret `/prod/codnod/config`, verify Kubernetes secret `app-credentials` is created automatically in the same namespace with all key-value pairs from AWS (DB_HOST, DB_USER, DB_PASSWORD)

### Implementation for User Story 1

- [X] T021 [P] [US1] Implement AWSSecretsManagerProvider.FetchSecret() in internal/provider/aws.go using AWS SDK v2
- [X] T022 [P] [US1] Implement AWSSecretsManagerProvider.Name() method in internal/provider/aws.go
- [X] T023 [P] [US1] Add JSON parsing logic for AWS secret response in internal/provider/aws.go
- [X] T024 [US1] Create unit tests for AWS provider in internal/provider/aws_test.go with table-driven tests
- [X] T025 [US1] Implement provider factory/registry pattern in internal/provider/provider.go
- [X] T026 [US1] Implement controller reconcile loop for pod Add events in internal/controller/podsecret_controller.go
- [X] T027 [US1] Add annotation extraction logic in reconcile function in internal/controller/podsecret_controller.go
- [X] T028 [US1] Add provider lookup and secret fetch logic in reconcile function in internal/controller/podsecret_controller.go
- [X] T029 [US1] Implement Kubernetes secret creation logic in reconcile function in internal/controller/podsecret_controller.go
- [X] T030 [US1] Add labels to created secrets (app.kubernetes.io/managed-by: caronte) in internal/controller/podsecret_controller.go
- [X] T031 [US1] Add annotations to created secrets (source-path, synced-at) in internal/controller/podsecret_controller.go
- [X] T032 [US1] Implement namespace validation (ensure secret created in same namespace as pod) in internal/controller/podsecret_controller.go
- [X] T033 [US1] Add Kubernetes event emission on success in internal/controller/podsecret_controller.go
- [X] T034 [US1] Implement manager setup and controller registration in cmd/controller/main.go
- [X] T035 [US1] Add health check endpoints (liveness, readiness) in cmd/controller/main.go
- [X] T036 [US1] Add structured logging with logr/Zap in cmd/controller/main.go
- [X] T037 [US1] Create controller deployment manifest in config/manager/deployment.yaml
- [X] T038 [US1] Create sample test pod manifest in config/samples/test-pod.yaml

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently - MVP complete!

---

## Phase 4: User Story 3 - Handle Secret Fetch Failures Gracefully (Priority: P1)

**Goal**: Provide clear error messages and proper handling when secrets cannot be fetched from external sources

**Independent Test**: Configure pod with annotation pointing to non-existent secret path, verify system logs meaningful errors and emits Kubernetes Warning event on the pod

### Implementation for User Story 3

- [ ] T039 [P] [US3] Add error handling for AWS ResourceNotFoundException in internal/provider/aws.go
- [ ] T040 [P] [US3] Add error handling for AWS AccessDeniedException in internal/provider/aws.go
- [ ] T041 [P] [US3] Implement exponential backoff retry logic for transient AWS errors in internal/provider/aws.go
- [ ] T042 [US3] Add comprehensive error logging in internal/controller/podsecret_controller.go
- [ ] T043 [US3] Emit Warning events on pod for fetch failures in internal/controller/podsecret_controller.go
- [ ] T044 [US3] Emit Warning events for annotation parse errors in internal/controller/podsecret_controller.go
- [ ] T045 [US3] Emit Warning events for unsupported providers in internal/controller/podsecret_controller.go
- [ ] T046 [US3] Add panic recovery wrapper in reconcile loop in internal/controller/podsecret_controller.go
- [ ] T047 [US3] Implement error result return for controller-runtime retry in internal/controller/podsecret_controller.go
- [ ] T048 [US3] Add unit tests for error scenarios in internal/controller/podsecret_controller_test.go

**Checkpoint**: User Stories 1 AND 3 are both complete and independently functional

---

## Phase 5: User Story 5 - Namespace Isolation (Priority: P1)

**Goal**: Ensure secrets are created only in the same namespace as requesting pod with no cross-namespace access

**Independent Test**: Deploy pods in namespace-a and namespace-b both requesting same secret path, verify each namespace has own secret instance

### Implementation for User Story 5

- [ ] T049 [US5] Add namespace validation check in reconcile function in internal/controller/podsecret_controller.go
- [ ] T050 [US5] Ensure secret namespace always matches pod namespace in internal/controller/podsecret_controller.go
- [ ] T051 [US5] Add unit tests for namespace isolation in internal/controller/podsecret_controller_test.go
- [ ] T052 [US5] Create E2E test script for multi-namespace scenario in tests/e2e/test_namespace_isolation.sh

**Checkpoint**: All P1 user stories (1, 3, 5) are complete - core functionality ready

---

## Phase 6: User Story 2 - Update Secrets When Source Changes (Priority: P2)

**Goal**: Automatically update Kubernetes secrets when external source changes and new pods are deployed

**Independent Test**: Update AWS secret value, delete and recreate pod with same annotations, verify Kubernetes secret reflects new value

### Implementation for User Story 2

- [ ] T053 [US2] Implement secret existence check in reconcile loop in internal/controller/podsecret_controller.go
- [ ] T054 [US2] Add secret update logic when existing secret differs from external source in internal/controller/podsecret_controller.go
- [ ] T055 [US2] Update synced-at annotation timestamp on secret updates in internal/controller/podsecret_controller.go
- [ ] T056 [US2] Emit Normal event on pod for secret updates in internal/controller/podsecret_controller.go
- [ ] T057 [US2] Add unit tests for update scenarios in internal/controller/podsecret_controller_test.go
- [ ] T058 [US2] Create E2E test for secret update flow in tests/e2e/test_secret_update.sh

**Checkpoint**: User Stories 1, 2, 3, 5 all independently functional

---

## Phase 7: User Story 4 - Support Multiple Secret Providers (Priority: P3)

**Goal**: Enable fetching secrets from multiple providers (Vault, Azure) using same annotation pattern

**Independent Test**: Deploy pods with annotations for different providers (AWS, Vault, Azure), verify each creates appropriate Kubernetes secret

### Implementation for User Story 4

- [ ] T059 [P] [US4] Create VaultProvider stub implementation in internal/provider/vault.go
- [ ] T060 [P] [US4] Create AzureKeyVaultProvider stub implementation in internal/provider/azure.go
- [ ] T061 [US4] Update provider registry to support multiple providers in internal/provider/provider.go
- [ ] T062 [US4] Add provider selection logic based on annotation in internal/controller/podsecret_controller.go
- [ ] T063 [US4] Add unit tests for multi-provider scenarios in internal/controller/podsecret_controller_test.go
- [ ] T064 [US4] Update sample manifests to show different providers in config/samples/

**Checkpoint**: All user stories (P1, P2, P3) are independently functional

---

## Phase 8: Testing & Validation

**Purpose**: Comprehensive testing across all user stories

- [ ] T065 [P] Create envtest suite setup in tests/integration/suite_test.go
- [ ] T066 [P] Create integration test for US1 (basic sync) in tests/integration/controller_test.go
- [ ] T067 [P] Create integration test for US3 (error handling) in tests/integration/controller_test.go
- [ ] T068 [P] Create integration test for US5 (namespace isolation) in tests/integration/controller_test.go
- [ ] T069 Create E2E test setup script in tests/e2e/setup.sh with AWS_PROFILE and minikube checks
- [ ] T070 Create E2E test for AWS sync (US1) in tests/e2e/test_aws_sync.sh
- [ ] T071 Create E2E test teardown script in tests/e2e/teardown.sh
- [ ] T072 Run all unit tests with `make test`
- [ ] T073 Run integration tests with `make test-integration`
- [ ] T074 Run E2E tests with `make test-e2e`

---

## Phase 9: Documentation & Polish

**Purpose**: Production readiness and documentation

- [ ] T075 [P] Create comprehensive README.md with quickstart, architecture, usage examples
- [ ] T076 [P] Add godoc comments to all exported types and functions
- [ ] T077 [P] Create CONTRIBUTING.md with development workflow
- [ ] T078 [P] Create example manifests for common use cases in config/samples/
- [ ] T079 Run `go fmt ./...` to format all code
- [ ] T080 Run `go vet ./...` to check for issues
- [ ] T081 Add Prometheus metrics for sync operations (success, failure, latency)
- [ ] T082 Update deployment manifest with resource limits (100MB memory, 0.1 CPU)
- [ ] T083 Add security context to deployment (non-root, read-only filesystem where possible)
- [ ] T084 Test full deployment flow per quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational completion
- **User Story 3 (Phase 4)**: Depends on US1 (builds on top of basic sync)
- **User Story 5 (Phase 5)**: Depends on US1 (validates existing sync behavior)
- **User Story 2 (Phase 6)**: Depends on US1 (extends basic sync with updates)
- **User Story 4 (Phase 7)**: Depends on US1 (extends provider architecture)
- **Testing (Phase 8)**: Depends on all user stories being complete
- **Documentation (Phase 9)**: Can start after US1, complete after all stories

### User Story Dependencies

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational) ← BLOCKING for all user stories
    ↓
    ├─→ US1 (P1) - Basic Sync ← MVP, foundation for all other stories
    │       ↓
    │       ├─→ US3 (P1) - Error Handling (extends US1)
    │       │
    │       ├─→ US5 (P1) - Namespace Isolation (validates US1)
    │       │
    │       ├─→ US2 (P2) - Secret Updates (extends US1)
    │       │
    │       └─→ US4 (P3) - Multi-Provider (extends US1)
    │
    └─→ Testing & Documentation (after all stories)
```

### Within Each User Story

- Foundation tasks (provider, parser) before controller logic
- Controller logic before event emission
- Core implementation before error handling
- Story complete before moving to next priority

### Parallel Opportunities

**Setup Phase**:
- T003, T004, T005 (dependency downloads) can run in parallel
- T006, T007, T008 (file creation) can run in parallel

**Foundational Phase**:
- T010, T011, T012, T013, T016, T017, T018, T019, T020 can all run in parallel (different files)
- T014, T015 depend on T012, T013

**User Story 1**:
- T021, T022, T023 (AWS provider methods) can run in parallel
- T025 can run in parallel with T021-T024
- T037, T038 (manifests) can run in parallel with controller implementation

**Testing Phase**:
- T065, T066, T067, T068 (integration tests) can run in parallel
- T069, T070, T071 (E2E tests) sequential

**Documentation Phase**:
- T075, T076, T077, T078 can all run in parallel

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup → **Foundation created**
2. Complete Phase 2: Foundational → **CRITICAL - all stories blocked until this completes**
3. Complete Phase 3: User Story 1 → **MVP achieved!**
4. **STOP and VALIDATE**: Test US1 independently with quickstart.md
5. Deploy to minikube and verify with real AWS secret
6. If successful, proceed to Phase 4 (US3)

**MVP Deliverable**: Working Kubernetes controller that syncs secrets from AWS Secrets Manager to Kubernetes based on pod annotations.

### Incremental Delivery

1. **Week 1**: Setup + Foundational + US1 → Deploy MVP
2. **Week 2**: US3 (Error Handling) + US5 (Namespace Isolation) → Core features complete
3. **Week 3**: US2 (Updates) → Production-ready
4. **Week 4**: US4 (Multi-Provider) + Testing + Documentation → Full feature set

Each phase adds value without breaking previous functionality.

### Parallel Team Strategy

With multiple developers:

1. **All**: Complete Setup + Foundational together (T001-T020)
2. **Once Foundational done**:
   - Developer A: US1 (T021-T038)
   - Developer B: US3 (T039-T048, wait for US1 checkpoint)
   - Developer C: US5 (T049-T052, wait for US1 checkpoint)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Stop at any checkpoint to validate story independently
- Constitution checks (AWS_PROFILE=codnod, minikube context) enforced in Makefile
- All file operations remain within `/Users/tiago/Projects/codnod/jasm`
- Avoid logging secret values - only metadata (paths, names, results)
- Use table-driven tests for all unit tests
- Follow Go idioms (godoc comments, error handling, panic recovery)

---

## Quick Reference

### Constitution-Compliant Commands

```bash
# Before AWS operations
make check-aws-profile

# Before Kubernetes operations
make check-k8s-context

# Full cycle
make test && make build && make deploy
```

### Validation After Each Phase

```bash
# After Setup
go mod verify && go build ./cmd/controller

# After Foundational
go test ./internal/... -v

# After US1 (MVP)
make run  # Test locally
make deploy && kubectl logs -l app=caronte -f

# After US3
# Create pod with invalid secret path, verify error events

# After US5
# Create pods in multiple namespaces, verify isolation

# After US2
# Update AWS secret, recreate pod, verify sync

# Final validation
make test-e2e
```

---

**Total Tasks**: 84
**Parallelizable Tasks**: 31 marked with [P]
**User Stories**: 5 (US1-P1, US2-P2, US3-P1, US4-P3, US5-P1)
**Estimated MVP Scope**: Phases 1-3 (T001-T038) = 38 tasks
