# Feature Specification: Multi-Host Ingress Support

**Feature Branch**: `003-multi-host-ingress`  
**Created**: 2025-12-03  
**Status**: Draft  
**Input**: User description: "Implement multi-host ingress support - handle Ingress resources with multiple rules/hosts instead of only processing the first host"

## Overview

Currently, the Pangolin Ingress Controller (PIC) only processes the first host/rule in an Ingress resource, emitting a warning event for any additional hosts. This feature removes that limitation, allowing a single Ingress to expose multiple hosts, each creating its own PangolinResource.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Multiple Hosts in Single Ingress (Priority: P1)

As a platform operator, I want to define multiple hosts in a single Ingress resource so that I can manage related services together and reduce the number of Ingress objects in my cluster.

**Why this priority**: This is the core feature request. Many real-world applications serve multiple domains (e.g., `api.example.com` and `admin.example.com`) from the same deployment, and users expect standard Kubernetes Ingress behavior.

**Independent Test**: Deploy an Ingress with two hosts pointing to the same backend service. Verify both hosts are accessible through Pangolin and route to the correct backend.

**Acceptance Scenarios**:

1. **Given** an Ingress with two rules for `app.example.com` and `api.example.com`, **When** the Ingress is created, **Then** two separate PangolinResource objects are created, one for each host.
2. **Given** an existing multi-host Ingress, **When** a third host is added, **Then** a third PangolinResource is created without affecting the existing two.
3. **Given** an existing multi-host Ingress, **When** one host is removed, **Then** only the corresponding PangolinResource is deleted; others remain intact.

---

### User Story 2 - Per-Host Path Routing (Priority: P1)

As a platform operator, I want each host in a multi-host Ingress to have its own set of path rules so that I can configure different routing for different domains.

**Why this priority**: Path-based routing per host is essential for real-world use cases where different domains have different backend structures.

**Independent Test**: Deploy an Ingress where `app.example.com` has paths `/` and `/api`, while `admin.example.com` has only `/`. Verify each host's PangolinResource has the correct targets.

**Acceptance Scenarios**:

1. **Given** an Ingress where host A has 3 paths and host B has 1 path, **When** processed, **Then** host A's PangolinResource has 3 targets and host B's has 1 target.
2. **Given** an existing multi-host Ingress, **When** a path is added to one host, **Then** only that host's PangolinResource is updated.

---

### User Story 3 - Consistent Naming Across Hosts (Priority: P2)

As a platform operator, I want PangolinResource names to be deterministic and unique per host so that I can predict resource names and avoid conflicts.

**Why this priority**: Deterministic naming enables automation, GitOps workflows, and prevents accidental resource collisions.

**Independent Test**: Create the same multi-host Ingress twice (delete and recreate). Verify the generated PangolinResource names are identical both times.

**Acceptance Scenarios**:

1. **Given** an Ingress `myapp` in namespace `prod` with hosts `a.example.com` and `b.example.com`, **When** created, **Then** PangolinResource names follow a deterministic pattern including namespace, ingress name, and host.
2. **Given** two Ingresses in different namespaces with the same host, **When** created, **Then** their PangolinResource names are different (no collision).

---

### User Story 4 - Owner Reference Cleanup (Priority: P2)

As a platform operator, I want all PangolinResources created from a multi-host Ingress to be automatically deleted when the Ingress is deleted so that I don't have orphaned resources.

**Why this priority**: Proper garbage collection is essential for cluster hygiene and preventing resource leaks.

**Independent Test**: Create a multi-host Ingress, verify PangolinResources exist, delete the Ingress, verify all PangolinResources are garbage collected.

**Acceptance Scenarios**:

1. **Given** an Ingress with 3 hosts and 3 corresponding PangolinResources, **When** the Ingress is deleted, **Then** all 3 PangolinResources are automatically deleted via owner references.

---

### Edge Cases

- **Empty host in one rule**: If one rule has an empty host while others have valid hosts, only the valid hosts should create PangolinResources; the empty host rule should be skipped with a warning event.
- **Duplicate hosts in same Ingress**: If the same host appears in multiple rules, only one PangolinResource should be created, combining all paths from all rules for that host.
- **Host conflicts across Ingresses**: If two different Ingresses define the same host, each creates its own PangolinResource (Pangolin API will handle the conflict at resource creation time).
- **Maximum hosts**: System should handle Ingresses with many hosts (10+) without performance degradation.
- **Annotation inheritance**: Per-host annotations (if supported) should override global Ingress annotations.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST process all rules in an Ingress, not just the first one.
- **FR-002**: System MUST create one PangolinResource per unique host in the Ingress.
- **FR-003**: System MUST include all paths from a host's rule(s) as targets in the corresponding PangolinResource.
- **FR-004**: System MUST set owner references on all created PangolinResources pointing to the parent Ingress.
- **FR-005**: System MUST delete PangolinResources when their corresponding host is removed from the Ingress.
- **FR-006**: System MUST generate deterministic, unique names for PangolinResources based on namespace, Ingress name, and host.
- **FR-007**: System MUST skip rules with empty hosts and emit a warning event.
- **FR-008**: System MUST deduplicate hosts if the same host appears in multiple rules, merging their paths.
- **FR-009**: System MUST NOT emit the "MultipleHosts" warning event after this feature is implemented.
- **FR-010**: System MUST support existing annotations (tunnel, domain, subdomain, SSO, block-access) applied to all hosts in the Ingress.

### Key Entities

- **Ingress**: Kubernetes native resource defining routing rules. Contains multiple rules, each with a host and HTTP paths.
- **PangolinResource**: Custom resource representing a Pangolin-exposed service. One created per host in the Ingress.
- **Target**: Backend configuration within a PangolinResource. Multiple targets per PangolinResource (one per path).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An Ingress with N hosts results in exactly N PangolinResources being created.
- **SC-002**: Deleting an Ingress with N hosts results in all N PangolinResources being garbage collected within 30 seconds.
- **SC-003**: Modifying one host in a multi-host Ingress only triggers reconciliation of that host's PangolinResource.
- **SC-004**: No "MultipleHosts" warning events are emitted for Ingresses with multiple hosts.
- **SC-005**: System handles Ingresses with 10+ hosts without errors or significant latency increase.
- **SC-006**: All existing single-host Ingresses continue to work without modification (backward compatibility).

## Assumptions

- The Pangolin API supports creating multiple resources with different subdomains on the same domain.
- The existing `util.GenerateName` function can be extended or a new function created to include host in the name generation.
- Owner references in Kubernetes support multiple resources owned by the same parent (standard behavior).
- The tunnel reference applies to all hosts in the Ingress (no per-host tunnel override in this version).
