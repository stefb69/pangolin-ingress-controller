# Quickstart: Multi-Host Ingress Validation

**Feature**: 003-multi-host-ingress  
**Purpose**: Validate multi-host ingress support after implementation

## Prerequisites

- Kubernetes cluster with PIC and pangolin-operator deployed
- At least one PangolinTunnel configured and connected
- kubectl access

## Scenario 1: Basic Multi-Host Ingress

### Deploy Test Ingress

```yaml
# test-multi-host.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: multi-host-test
  namespace: default
spec:
  ingressClassName: pangolin
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: app-service
            port:
              number: 80
  - host: api.example.com
    http:
      paths:
      - path: /v1
        pathType: Prefix
        backend:
          service:
            name: api-service
            port:
              number: 8080
```

```bash
kubectl apply -f test-multi-host.yaml
```

### Verify PangolinResources Created

```bash
# Should show 2 PangolinResources
kubectl get pangolinresources -l pic.ingress.k8s.io/name=multi-host-test

# Expected output:
# NAME                          READY   AGE
# pic-default-multi-host-xxxx   True    10s
# pic-default-multi-host-yyyy   True    10s
```

### Verify Each Resource

```bash
# Check first resource (app.example.com)
kubectl get pangolinresource pic-default-multi-host-xxxx -o yaml | grep -A5 httpConfig

# Check second resource (api.example.com)
kubectl get pangolinresource pic-default-multi-host-yyyy -o yaml | grep -A5 httpConfig
```

**Expected**: Each PangolinResource has different subdomain matching its host.

## Scenario 2: Add Host to Existing Ingress

### Add Third Host

```bash
kubectl patch ingress multi-host-test --type='json' -p='[
  {"op": "add", "path": "/spec/rules/-", "value": {
    "host": "admin.example.com",
    "http": {
      "paths": [{
        "path": "/",
        "pathType": "Prefix",
        "backend": {"service": {"name": "admin-service", "port": {"number": 80}}}
      }]
    }
  }}
]'
```

### Verify Third Resource Created

```bash
# Should now show 3 PangolinResources
kubectl get pangolinresources -l pic.ingress.k8s.io/name=multi-host-test

# Expected: 3 resources
```

## Scenario 3: Remove Host from Ingress

### Remove Second Host

```bash
kubectl patch ingress multi-host-test --type='json' -p='[
  {"op": "remove", "path": "/spec/rules/1"}
]'
```

### Verify Resource Deleted

```bash
# Should now show 2 PangolinResources
kubectl get pangolinresources -l pic.ingress.k8s.io/name=multi-host-test

# Expected: 2 resources (api.example.com resource deleted)
```

## Scenario 4: Delete Ingress (Garbage Collection)

### Delete Ingress

```bash
kubectl delete ingress multi-host-test
```

### Verify All Resources Deleted

```bash
# Should show no PangolinResources
kubectl get pangolinresources -l pic.ingress.k8s.io/name=multi-host-test

# Expected: No resources found
```

## Scenario 5: Duplicate Host Merging

### Deploy Ingress with Duplicate Hosts

```yaml
# test-duplicate-host.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: duplicate-host-test
  namespace: default
spec:
  ingressClassName: pangolin
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: frontend
            port:
              number: 80
  - host: app.example.com  # Same host, different path
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: backend
            port:
              number: 8080
```

```bash
kubectl apply -f test-duplicate-host.yaml
```

### Verify Single Resource with Merged Paths

```bash
# Should show 1 PangolinResource
kubectl get pangolinresources -l pic.ingress.k8s.io/name=duplicate-host-test

# Check targets (should have 2 targets for the 2 paths)
kubectl get pangolinresource -l pic.ingress.k8s.io/name=duplicate-host-test -o jsonpath='{.items[0].spec.targets}' | jq

# Expected: 2 targets (/ and /api)
```

## Scenario 6: No MultipleHosts Warning

### Check Events

```bash
kubectl get events --field-selector reason=MultipleHosts

# Expected: No events (warning should not be emitted)
```

## Cleanup

```bash
kubectl delete ingress multi-host-test duplicate-host-test --ignore-not-found
```

## Success Criteria Checklist

| Criteria | Command | Expected |
|----------|---------|----------|
| SC-001: N hosts → N resources | `kubectl get pr -l pic...name=X \| wc -l` | Equals host count |
| SC-002: Delete → all GC'd | `kubectl get pr -l pic...name=X` after delete | No resources |
| SC-004: No MultipleHosts warning | `kubectl get events --field-selector reason=MultipleHosts` | No events |
| SC-006: Single-host still works | Deploy single-host ingress | 1 resource created |
