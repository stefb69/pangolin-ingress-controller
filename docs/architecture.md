# Pangolin Ingress Controller Architecture

## Overview

The Pangolin Ingress Controller (PIC) bridges Kubernetes Ingress resources with the Pangolin tunnel system by creating `PangolinResource` Custom Resources.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Kubernetes Cluster                              │
│                                                                             │
│  ┌─────────────┐      ┌─────────────────┐      ┌────────────────────────┐  │
│  │   Ingress   │──────▶│      PIC        │──────▶│   PangolinResource    │  │
│  │  (N hosts)  │      │  (Controller)   │      │   (N resources)       │  │
│  └─────────────┘      └─────────────────┘      └────────────────────────┘  │
│                                                          │                  │
│                                                          ▼                  │
│                                               ┌────────────────────────┐   │
│                                               │  pangolin-operator     │   │
│                                               └────────────────────────┘   │
│                                                          │                  │
└──────────────────────────────────────────────────────────│──────────────────┘
                                                           │
                                                           ▼
                                                  ┌─────────────────┐
                                                  │  Pangolin API   │
                                                  └─────────────────┘
```

## Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **PIC** | Watches Ingress resources, creates/updates/deletes PangolinResource CRDs |
| **pangolin-operator** | Watches PangolinResource CRDs, calls Pangolin API |
| **Pangolin API** | Manages tunnel routing and SSO configuration |

## Multi-Host Ingress Support

PIC supports Ingress resources with multiple hosts. Each unique host in an Ingress creates a separate `PangolinResource`.

### Data Flow

```
Ingress (3 hosts)
    │
    ├── host: app.example.com ──────▶ PangolinResource (pic-ns-ing-hash1)
    │       └── paths: /, /api
    │
    ├── host: api.example.com ──────▶ PangolinResource (pic-ns-ing-hash2)
    │       └── paths: /v1, /v2
    │
    └── host: admin.example.com ────▶ PangolinResource (pic-ns-ing-hash3)
            └── paths: /
```

### Processing Algorithm

1. **Collect hosts**: Iterate all rules, group paths by host
2. **Deduplicate**: Merge paths if same host appears in multiple rules
3. **Skip empty**: Emit warning for rules with empty hosts
4. **Build resources**: Create one `PangolinResource` per unique host
5. **Set ownership**: Each resource has owner reference to parent Ingress
6. **Cleanup orphans**: Delete resources for hosts no longer in Ingress

### Naming Convention

PangolinResource names follow the pattern:

```
pic-<namespace>-<ingress>-<hash>
```

Where `<hash>` is derived from `SHA256(namespace/ingress/host)`.

This ensures:
- **Uniqueness**: Different hosts produce different names
- **Determinism**: Same inputs always produce same name
- **Predictability**: Names can be computed for GitOps workflows

### Garbage Collection

All PangolinResources created by PIC have:

1. **Owner Reference**: Points to parent Ingress with `controller: true`
2. **Labels**: `pic.ingress.k8s.io/uid`, `pic.ingress.k8s.io/name`, `pic.ingress.k8s.io/namespace`

When an Ingress is deleted, Kubernetes automatically garbage collects all owned PangolinResources.

When a host is removed from an Ingress (but Ingress still exists), PIC explicitly deletes the orphaned PangolinResource.

## Reconciliation Loop

```go
func Reconcile(ingress) {
    // 1. Validate Ingress is managed by PIC
    if !isManaged(ingress) {
        return handleUnmanaged(ingress)
    }
    
    // 2. Resolve and validate tunnel
    tunnel := resolveTunnel(ingress)
    
    // 3. Process all hosts
    hostGroups := collectHostPaths(ingress)  // Deduplicate hosts
    
    desiredNames := {}
    for _, group := range hostGroups {
        // 4. Build and reconcile each PangolinResource
        resource := buildDesiredPangolinResource(ingress, group.Host, group.Paths)
        setControllerReference(ingress, resource)
        reconcilePangolinResource(resource)
        desiredNames[resource.Name] = true
    }
    
    // 5. Cleanup orphaned resources
    cleanupOrphanedResources(ingress, desiredNames)
}
```

## Events

| Event | Type | Reason | Description |
|-------|------|--------|-------------|
| Created | Normal | Created | PangolinResource created for host |
| Updated | Normal | Updated | PangolinResource updated |
| Deleted | Normal | Deleted | PangolinResource deleted (host removed or Ingress unmanaged) |
| Warning | Warning | EmptyHost | Rule with empty host skipped |
| Warning | Warning | NoRules | Ingress has no rules defined |
| Warning | Warning | TunnelNotFound | Referenced tunnel does not exist |
| Warning | Warning | InvalidHost | Host format is invalid |

## Configuration

See [README.md](../README.md) for configuration options including:
- Default tunnel name
- Multi-tunnel mapping
- SSO annotations
- Domain/subdomain overrides
