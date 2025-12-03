# Data Model: Multi-Host Ingress Support

**Feature**: 003-multi-host-ingress  
**Date**: 2025-12-03

## Entity Overview

This feature does not introduce new entities. It changes the **cardinality** of the relationship between Ingress and PangolinResource from 1:1 to 1:N.

## Relationship Changes

### Before (MVP)

```text
┌─────────────────┐         ┌─────────────────────┐
│     Ingress     │   1:1   │   PangolinResource  │
│  (N hosts)      │────────▶│   (first host only) │
└─────────────────┘         └─────────────────────┘
```

### After (Multi-Host)

```text
┌─────────────────┐         ┌─────────────────────┐
│     Ingress     │   1:N   │   PangolinResource  │
│  (N hosts)      │────────▶│   (one per host)    │
└─────────────────┘         └─────────────────────┘
                                    │
                                    │ 1:M
                                    ▼
                            ┌─────────────────────┐
                            │      Target         │
                            │  (one per path)     │
                            └─────────────────────┘
```

## Data Structures

### HostPathGroup (Internal Processing)

Intermediate structure to group paths by host during reconciliation.

| Field | Type | Description |
|-------|------|-------------|
| Host | string | Fully qualified domain name |
| Paths | []HTTPIngressPath | All paths from all rules for this host |

### PangolinResource Name Generation

No schema change. Names already incorporate host for uniqueness:

```text
Format: pic-<namespace>-<ingress>-<hash>
Hash Input: <namespace>/<ingress>/<host>
```

Example for Ingress `myapp` in `prod` with hosts `a.example.com` and `b.example.com`:
- `pic-prod-myapp-a1b2c3d4` (for `a.example.com`)
- `pic-prod-myapp-e5f6g7h8` (for `b.example.com`)

### Labels (Existing, No Change)

PangolinResources already carry these labels for tracking:

| Label | Value | Purpose |
|-------|-------|---------|
| `pic.ingress.k8s.io/uid` | Ingress UID | Owner identification for cleanup |
| `pic.ingress.k8s.io/name` | Ingress name | Human-readable reference |
| `pic.ingress.k8s.io/namespace` | Ingress namespace | Scoping |

## State Transitions

### PangolinResource Lifecycle (per host)

```text
┌──────────┐     Host added      ┌──────────┐
│  (none)  │ ──────────────────▶ │ Created  │
└──────────┘                     └──────────┘
                                      │
                          Host modified│
                                      ▼
                                 ┌──────────┐
                                 │ Updated  │
                                 └──────────┘
                                      │
                          Host removed│
                                      ▼
                                 ┌──────────┐
                                 │ Deleted  │
                                 └──────────┘
```

### Reconciliation Flow

```text
1. Ingress Event Received
2. Group all paths by host (map[string][]Path)
3. For each unique host:
   a. Generate deterministic name
   b. Build PangolinResource spec with all paths as targets
   c. Create or Update PangolinResource
   d. Track name in expectedSet
4. List all PangolinResources owned by this Ingress (by UID label)
5. Delete any not in expectedSet (orphan cleanup)
6. Record success event
```

## Validation Rules

| Rule | Enforcement |
|------|-------------|
| Host must not be empty | Skip with warning event |
| Host must be valid FQDN | Existing SplitHost validation |
| Duplicate hosts → merge paths | Map-based aggregation |
| Path must have backend | Existing validation |

## Backward Compatibility

- Single-host Ingresses continue to work identically
- PangolinResource name generation is unchanged (host already in hash)
- No CRD schema changes required
- No changes to pangolin-operator interaction
