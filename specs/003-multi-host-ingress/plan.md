# Implementation Plan: Multi-Host Ingress Support

**Branch**: `003-multi-host-ingress` | **Date**: 2025-12-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-multi-host-ingress/spec.md`

## Summary

Extend the Pangolin Ingress Controller to process all hosts/rules in an Ingress resource, creating one PangolinResource per unique host. Currently only the first host is processed. The implementation will:

1. Iterate all rules in an Ingress, grouping paths by host
2. Generate one PangolinResource per unique host with all its paths as targets
3. Clean up PangolinResources when hosts are removed from the Ingress
4. Maintain owner references for garbage collection

## Technical Context

**Language/Version**: Go 1.24.0  
**Primary Dependencies**: controller-runtime v0.17.0, k8s.io/api v0.29.0  
**Storage**: N/A (Kubernetes CRDs via controller-runtime)  
**Testing**: go test with testify, envtest for integration  
**Target Platform**: Linux container (Kubernetes controller)  
**Project Type**: Single Kubernetes controller  
**Performance Goals**: Handle 10+ hosts per Ingress without latency increase  
**Constraints**: Reconcile loop must remain idempotent; owner references for GC  
**Scale/Scope**: Single controller, ~500 LOC change estimated

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| **I. Controller-Runtime First** | ✅ PASS | Uses Reconciler interface, `Owns()` for owner refs, existing patterns |
| **II. CRD Interoperability** | ✅ PASS | Creates PangolinResource CRs only, no API calls, proper owner refs |
| **III. Test-First Development** | ✅ PASS | Plan includes unit + integration tests before implementation |
| **IV. Observability** | ✅ PASS | Events for multi-host processing, removal of MVP warning |
| **V. Minimal RBAC** | ✅ PASS | No new permissions required (already has PangolinResource write) |

**Gate Result**: PASS - Proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/003-multi-host-ingress/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/
├── controller/
│   └── ingress_controller.go    # Main changes: multi-host loop, cleanup logic
├── util/
│   └── naming.go                # Already supports host in name generation
└── pangolincrd/
    └── types.go                 # No changes needed

tests/
├── integration/
│   ├── reconciler_test.go       # Add multi-host scenarios
│   └── lifecycle_test.go        # Add multi-host cleanup tests
└── unit/
    └── naming_test.go           # Verify naming uniqueness per host
```

**Structure Decision**: Existing single-controller structure. Changes concentrated in `ingress_controller.go` with test coverage in `tests/integration/`.

## Complexity Tracking

> No constitution violations - table not required.

## Post-Design Constitution Re-Check

*Re-evaluated after Phase 1 design artifacts completed.*

| Principle | Status | Post-Design Evidence |
|-----------|--------|---------------------|
| **I. Controller-Runtime First** | ✅ PASS | Design uses `Owns()` pattern, List with labels, standard reconcile loop |
| **II. CRD Interoperability** | ✅ PASS | Only creates/updates/deletes PangolinResource CRs, no API calls |
| **III. Test-First Development** | ✅ PASS | quickstart.md defines 6 validation scenarios, tasks will include tests |
| **IV. Observability** | ✅ PASS | EmptyHost warning event defined, MultipleHosts warning removed |
| **V. Minimal RBAC** | ✅ PASS | No new permissions - existing PangolinResource list/create/update/delete sufficient |

**Post-Design Gate Result**: PASS - Ready for `/speckit.tasks`

## Phase 1 Artifacts

| Artifact | Status | Path |
|----------|--------|------|
| research.md | ✅ Complete | [research.md](./research.md) |
| data-model.md | ✅ Complete | [data-model.md](./data-model.md) |
| quickstart.md | ✅ Complete | [quickstart.md](./quickstart.md) |
| Agent context | ✅ Updated | `.windsurf/rules/specify-rules.md` |
