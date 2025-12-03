# Tasks: Multi-Host Ingress Support

**Input**: Design documents from `/specs/003-multi-host-ingress/`  
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, quickstart.md ✅

**Tests**: Included per Constitution Principle III (Test-First Development)

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: US1, US2, US3, US4 (maps to user stories from spec.md)
- Exact file paths included

## Path Conventions

```text
internal/
├── controller/
│   └── ingress_controller.go    # Main reconciliation logic
├── util/
│   └── naming.go                # Name generation (no changes needed)
└── pangolincrd/
    └── types.go                 # CRD types (no changes needed)

tests/
├── integration/
│   ├── reconciler_test.go       # Multi-host reconciliation tests
│   └── lifecycle_test.go        # Cleanup and GC tests
└── unit/
    └── naming_test.go           # Naming uniqueness tests
```

---

## Phase 1: Setup

**Purpose**: Prepare codebase for multi-host changes

- [x] T001 Create feature branch `003-multi-host-ingress` and sync with main
- [x] T002 [P] Add `HostPathGroup` internal type documentation in `internal/controller/ingress_controller.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core refactoring that MUST be complete before user stories

**⚠️ CRITICAL**: All user stories depend on these foundational changes

- [x] T003 Refactor `Reconcile` function to extract host processing into `processHosts()` helper in `internal/controller/ingress_controller.go`
- [x] T004 Add `collectHostPaths()` function to group paths by host with deduplication in `internal/controller/ingress_controller.go`
- [x] T005 [P] Remove "MultipleHosts" warning event code (FR-009) in `internal/controller/ingress_controller.go`
- [x] T006 [P] Add "EmptyHost" warning event for empty host handling (FR-007) in `internal/controller/ingress_controller.go`

**Checkpoint**: Foundation ready - multi-host loop structure in place

---

## Phase 3: User Story 1 - Multiple Hosts in Single Ingress (Priority: P1) 🎯 MVP

**Goal**: Process all hosts in an Ingress, creating one PangolinResource per unique host

**Independent Test**: Deploy Ingress with 2 hosts → verify 2 PangolinResources created

### Tests for User Story 1

> **TDD**: Write tests FIRST, ensure they FAIL before implementation

- [x] T007 [P] [US1] Add test `TestReconcile_MultipleHosts_CreatesTwoResources` in `tests/integration/reconciler_test.go`
- [x] T008 [P] [US1] Add test `TestReconcile_AddHostToExisting_CreatesThirdResource` in `tests/integration/reconciler_test.go`
- [x] T009 [P] [US1] Add test `TestReconcile_RemoveHost_DeletesOrphanResource` in `tests/integration/reconciler_test.go`

### Implementation for User Story 1

- [x] T010 [US1] Implement multi-host loop in `Reconcile()` to iterate all rules in `internal/controller/ingress_controller.go`
- [x] T011 [US1] Call `buildDesiredPangolinResource()` per host in the loop in `internal/controller/ingress_controller.go`
- [x] T012 [US1] Call `reconcilePangolinResource()` per host in the loop in `internal/controller/ingress_controller.go`
- [x] T013 [US1] Implement orphan cleanup: list by owner UID label, delete non-matching in `internal/controller/ingress_controller.go`
- [x] T014 [US1] Add success event for multi-host processing in `internal/controller/ingress_controller.go`

**Checkpoint**: US1 complete - N hosts → N PangolinResources, orphans cleaned up

---

## Phase 4: User Story 2 - Per-Host Path Routing (Priority: P1)

**Goal**: Each host gets its own set of paths as targets in its PangolinResource

**Independent Test**: Deploy Ingress where host A has 3 paths, host B has 1 path → verify target counts

### Tests for User Story 2

- [x] T015 [P] [US2] Add test `TestReconcile_PerHostPaths_CorrectTargetCounts` in `tests/integration/reconciler_test.go`
- [x] T016 [P] [US2] Add test `TestReconcile_DuplicateHostMergesPaths` (FR-008) in `tests/integration/reconciler_test.go`

### Implementation for User Story 2

- [x] T017 [US2] Update `buildDesiredPangolinResource()` to accept paths slice instead of extracting from first rule in `internal/controller/ingress_controller.go`
- [x] T018 [US2] Pass host-specific paths from `collectHostPaths()` result to `buildDesiredPangolinResource()` in `internal/controller/ingress_controller.go`
- [x] T019 [US2] Verify path merging works when same host appears in multiple rules in `internal/controller/ingress_controller.go`

**Checkpoint**: US2 complete - each host has correct targets based on its paths

---

## Phase 5: User Story 3 - Consistent Naming Across Hosts (Priority: P2)

**Goal**: PangolinResource names are deterministic and unique per host

**Independent Test**: Create same Ingress twice → verify identical PangolinResource names

### Tests for User Story 3

- [x] T020 [P] [US3] Add test `TestGenerateName_DifferentHosts_DifferentNames` in `tests/unit/naming_test.go`
- [x] T021 [P] [US3] Add test `TestGenerateName_SameInputs_SameName` (idempotency) in `tests/unit/naming_test.go`
- [x] T022 [P] [US3] Add test `TestGenerateName_DifferentNamespaces_NoCollision` in `tests/unit/naming_test.go`

### Implementation for User Story 3

- [x] T023 [US3] Verify existing `util.GenerateName()` already includes host in hash (research confirmed) in `internal/util/naming.go`
- [x] T024 [US3] Add documentation comments explaining name generation algorithm in `internal/util/naming.go`

**Checkpoint**: US3 complete - deterministic naming verified

---

## Phase 6: User Story 4 - Owner Reference Cleanup (Priority: P2)

**Goal**: All PangolinResources are garbage collected when Ingress is deleted

**Independent Test**: Delete multi-host Ingress → verify all PangolinResources deleted

### Tests for User Story 4

- [x] T025 [P] [US4] Add test `TestLifecycle_DeleteIngress_AllResourcesGarbageCollected` in `tests/integration/lifecycle_test.go`
- [x] T026 [P] [US4] Add test `TestLifecycle_OwnerReferencesSetCorrectly` in `tests/integration/lifecycle_test.go`

### Implementation for User Story 4

- [x] T027 [US4] Verify `SetControllerReference()` is called for each created PangolinResource in `internal/controller/ingress_controller.go`
- [x] T028 [US4] Verify owner UID label is set on all created PangolinResources in `internal/controller/ingress_controller.go`
- [x] T029 [US4] Add logging for owner reference setup in `internal/controller/ingress_controller.go`

**Checkpoint**: US4 complete - garbage collection verified

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and documentation

- [x] T030 [P] Update README.md with multi-host Ingress examples in `README.md`
- [x] T031 [P] Add multi-host section to architecture documentation in `docs/architecture.md`
- [ ] T032 Run quickstart.md validation scenarios (all 6 scenarios) per `specs/003-multi-host-ingress/quickstart.md`
- [ ] T033 Verify backward compatibility: single-host Ingress still works (SC-006)
- [ ] T034 Performance test: 10+ hosts Ingress (SC-005)
- [x] T035 Code review: verify Constitution compliance (controller-runtime patterns, CRD interop, observability)

---

## Dependencies & Execution Order

### Phase Dependencies

```text
Phase 1 (Setup)
    │
    ▼
Phase 2 (Foundational) ─── BLOCKS ALL USER STORIES
    │
    ├──────────────────────────────────────────┐
    │                                          │
    ▼                                          ▼
Phase 3 (US1: Multi-Host) ◀────────────▶ Phase 4 (US2: Per-Host Paths)
    │                                          │
    │              P1 Stories                  │
    │         (can run in parallel)            │
    └──────────────────┬───────────────────────┘
                       │
    ┌──────────────────┴───────────────────────┐
    │                                          │
    ▼                                          ▼
Phase 5 (US3: Naming)  ◀────────────▶  Phase 6 (US4: Cleanup)
    │                                          │
    │              P2 Stories                  │
    │         (can run in parallel)            │
    └──────────────────┬───────────────────────┘
                       │
                       ▼
               Phase 7 (Polish)
```

### User Story Dependencies

| Story | Depends On | Can Parallel With |
|-------|------------|-------------------|
| US1 (Multi-Host) | Phase 2 | US2 |
| US2 (Per-Host Paths) | Phase 2 | US1 |
| US3 (Naming) | US1, US2 | US4 |
| US4 (Cleanup) | US1 | US3 |

### Within Each User Story

1. Tests MUST be written and FAIL before implementation
2. Core logic before integration
3. Logging/events last

### Parallel Opportunities

**Phase 2 (Foundational)**:
```bash
# Can run in parallel:
T005 [P] Remove "MultipleHosts" warning
T006 [P] Add "EmptyHost" warning
```

**Phase 3 (US1 Tests)**:
```bash
# Can run in parallel:
T007 [P] TestReconcile_MultipleHosts_CreatesTwoResources
T008 [P] TestReconcile_AddHostToExisting_CreatesThirdResource
T009 [P] TestReconcile_RemoveHost_DeletesOrphanResource
```

**Phase 5 (US3 Tests)**:
```bash
# Can run in parallel:
T020 [P] TestGenerateName_DifferentHosts_DifferentNames
T021 [P] TestGenerateName_SameInputs_SameName
T022 [P] TestGenerateName_DifferentNamespaces_NoCollision
```

---

## Implementation Strategy

### MVP First (US1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test with 2-host Ingress
5. Deploy if ready - MVP delivers core value

### Incremental Delivery

| Delivery | Stories | Value |
|----------|---------|-------|
| MVP | US1 | Multi-host works |
| +1 | US1 + US2 | Path routing per host |
| +2 | US1-US4 | Full feature with GC |
| Final | All + Polish | Production ready |

---

## Notes

- Constitution requires TDD - all test tasks marked with story labels
- Existing `util.GenerateName` already handles host uniqueness (research.md confirmed)
- Owner references are standard K8s pattern - no custom cleanup needed for Ingress deletion
- Main changes concentrated in `internal/controller/ingress_controller.go`
- Backward compatibility critical - single-host must continue working
