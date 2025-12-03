# Research: Multi-Host Ingress Support

**Feature**: 003-multi-host-ingress  
**Date**: 2025-12-03

## Research Tasks

### R1: Kubernetes Owner References with Multiple Owned Resources

**Question**: Can a single Ingress own multiple PangolinResources via ownerReferences?

**Decision**: Yes - Kubernetes natively supports one owner having multiple owned resources.

**Rationale**: 
- `ownerReferences` is a list on the owned resource, not a constraint on the owner
- Each PangolinResource will have an ownerReference pointing to the same Ingress
- Garbage collection will delete all owned resources when the Ingress is deleted
- controller-runtime's `SetControllerReference` handles this correctly

**Alternatives Considered**:
- Finalizers on Ingress for manual cleanup - Rejected: more complex, owner refs are idiomatic
- Single PangolinResource with multiple hosts - Rejected: CRD design is 1 resource = 1 host

### R2: Orphan Cleanup Strategy

**Question**: How to detect and delete PangolinResources when a host is removed from an Ingress?

**Decision**: List PangolinResources by owner UID, compare to current hosts, delete orphans.

**Rationale**:
- PangolinResources have `pic.ingress.k8s.io/uid` label set to Ingress UID
- After processing all hosts, list all PangolinResources with matching UID label
- Compare to the set of generated resource names
- Delete any that don't match (orphans from removed hosts)

**Implementation Pattern**:
```go
// After creating/updating all PangolinResources for current hosts
existingList := &PangolinResourceList{}
client.List(ctx, existingList, client.MatchingLabels{"pic.ingress.k8s.io/uid": ingress.UID})

expectedNames := map[string]bool{} // populated during host processing
for _, existing := range existingList.Items {
    if !expectedNames[existing.Name] {
        client.Delete(ctx, &existing) // orphan
    }
}
```

**Alternatives Considered**:
- Store host list in annotation, diff on update - Rejected: stateful, complex
- Use finalizer to track hosts - Rejected: overkill for simple set comparison

### R3: Host Deduplication Strategy

**Question**: How to handle duplicate hosts in same Ingress (same host in multiple rules)?

**Decision**: Use map[host][]paths to aggregate paths, then generate one resource per host.

**Rationale**:
- Iterate all rules, collect paths by host into a map
- For each unique host, combine all paths from all rules
- Generate single PangolinResource with merged targets
- Natural deduplication via map key

**Implementation Pattern**:
```go
hostPaths := make(map[string][]networkingv1.HTTPIngressPath)
for _, rule := range ingress.Spec.Rules {
    if rule.Host == "" {
        continue // skip empty hosts
    }
    if rule.HTTP != nil {
        hostPaths[rule.Host] = append(hostPaths[rule.Host], rule.HTTP.Paths...)
    }
}
// hostPaths now has unique hosts with all their paths
```

**Alternatives Considered**:
- Process rules in order, first wins - Rejected: loses paths from duplicate hosts
- Error on duplicate hosts - Rejected: valid Kubernetes Ingress config

### R4: Naming Uniqueness Verification

**Question**: Does existing `util.GenerateName` already produce unique names per host?

**Decision**: Yes - the function already includes host in the hash computation.

**Evidence**:
```go
// From internal/util/naming.go
hashInput := fmt.Sprintf("%s/%s/%s", namespace, ingressName, host)
hash := sha256.Sum256([]byte(hashInput))
```

**Conclusion**: No changes needed to naming utility. Different hosts will produce different hashes.

### R5: Empty Host Handling

**Question**: How to handle rules with empty hosts mixed with valid hosts?

**Decision**: Skip empty hosts with warning event, process only valid hosts.

**Rationale**:
- Empty host typically means "match all" which doesn't translate to Pangolin's subdomain model
- Other hosts in the same Ingress should still be processed
- Emit a warning event per empty host for visibility

**Implementation Pattern**:
```go
for _, rule := range ingress.Spec.Rules {
    if rule.Host == "" {
        recorder.Event(ingress, "Warning", "EmptyHost", 
            "Skipping rule with empty host - Pangolin requires explicit host")
        continue
    }
    // process host...
}
```

## Summary

All research questions resolved. Key findings:

| Topic | Decision |
|-------|----------|
| Owner References | Native K8s support, multiple owned resources OK |
| Orphan Cleanup | List by owner UID label, delete non-matching |
| Deduplication | Map-based aggregation of paths by host |
| Naming | Existing function already handles uniqueness |
| Empty Hosts | Skip with warning, process valid hosts |

No blockers identified. Ready for Phase 1 design.
