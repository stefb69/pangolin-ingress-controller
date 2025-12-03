# Pangolin Ingress Controller (PIC)

A Kubernetes Ingress Controller that exposes services via [Pangolin](https://github.com/fosrl/pangolin) by creating `PangolinResource` CRDs.

## Overview

PIC enables a **Kubernetes-native experience** for exposing services through Pangolin. Instead of manually configuring Pangolin, you simply create a standard Kubernetes `Ingress` resource.

### Basic Example

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
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
                name: my-app
                port:
                  number: 8080
```

### With SSO Authentication

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    pangolin.ingress.k8s.io/sso: "true"
    pangolin.ingress.k8s.io/block-access: "true"
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
                name: my-app
                port:
                  number: 8080
```

### Multi-Path Routing

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    pangolin.ingress.k8s.io/sso: "false"
spec:
  ingressClassName: pangolin
  rules:
    - host: app.example.com
      http:
        paths:
          - path: /api
            pathType: Prefix
            backend:
              service:
                name: api-service
                port:
                  number: 8080
          - path: /web
            pathType: Prefix
            backend:
              service:
                name: web-service
                port:
                  number: 80
```

PIC will automatically create a `PangolinResource` with multiple targets, each with path-based routing configured.

## Prerequisites

- Kubernetes 1.28+
- `pangolin-operator` installed with CRDs
- At least one `PangolinTunnel` configured

## Installation

### Helm (recommended)

```bash
helm install pic ./charts/pangolin-ingress-controller \
  --namespace pangolin-system \
  --create-namespace
```

### With custom values

```bash
helm install pic ./charts/pangolin-ingress-controller \
  --namespace pangolin-system \
  --create-namespace \
  --set config.defaultTunnelName=my-tunnel \
  --set config.logLevel=debug
```

### Raw manifests

```bash
kubectl apply -f https://raw.githubusercontent.com/stefb69/pangolin-ingress-controller/main/deploy/install.yaml
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PIC_DEFAULT_TUNNEL_NAME` | `default` | Tunnel for `ingressClassName: pangolin` |
| `PIC_TUNNEL_CLASS_MAPPING` | - | Multi-tunnel mapping (see below) |
| `PIC_BACKEND_SCHEME` | `http` | Backend protocol |
| `PIC_RESYNC_PERIOD` | `5m` | Reconciliation interval |
| `PIC_LOG_LEVEL` | `info` | Log level |
| `PIC_WATCH_NAMESPACES` | - | Limit to specific namespaces |

### Multi-Tunnel Setup

```yaml
env:
  - name: PIC_TUNNEL_CLASS_MAPPING
    value: |
      eu=tunnel-eu
      us=tunnel-us
```

Then use `ingressClassName: pangolin-eu` to route through `tunnel-eu`.

### Annotations

| Annotation | Default | Description |
|------------|---------|-------------|
| `pangolin.ingress.k8s.io/enabled` | `true` | Enable/disable PIC processing |
| `pangolin.ingress.k8s.io/tunnel-name` | - | Override tunnel name |
| `pangolin.ingress.k8s.io/domain-name` | - | Override domain |
| `pangolin.ingress.k8s.io/subdomain` | - | Override subdomain |
| `pangolin.ingress.k8s.io/sso` | `false` | Enable SSO authentication |
| `pangolin.ingress.k8s.io/block-access` | `false` | Block access until authenticated (requires `sso: true`) |

### SSO Authentication

Pangolin supports SSO authentication to protect your services. Use the following annotations:

- **`sso: "false"`** - Service is publicly accessible (default)
- **`sso: "true"`** - SSO is enabled, users see identity but access is allowed
- **`sso: "true"` + `block-access: "true"`** - Users must authenticate before accessing

### Multi-Path Support

PIC supports multiple paths per Ingress. Each path creates a separate target in Pangolin with:

- **Path**: The URL path to match (e.g., `/api`)
- **PathMatchType**: Derived from Ingress `pathType` (`Exact` → `exact`, `Prefix` → `prefix`)
- **Priority**: Automatically calculated based on path length (longer paths = higher priority)

## Development

```bash
# Build
make build

# Test
make test

# Run locally (requires kubeconfig)
make run

# Build Docker image
make docker-build
```

## Architecture

```
Ingress ──▶ PIC ──▶ PangolinResource ──▶ pangolin-operator ──▶ Pangolin API
```

PIC only manages `PangolinResource` objects. All Pangolin API interaction is handled by `pangolin-operator`.

### Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **PIC** | Watches Ingress resources, creates/updates PangolinResource CRDs |
| **pangolin-operator** | Watches PangolinResource CRDs, calls Pangolin API to create resources/targets |
| **Pangolin API** | Manages the actual tunnel routing and SSO configuration |

## Troubleshooting

### Check PangolinResource status

```bash
kubectl get pangolinresources -A
kubectl describe pangolinresource <name> -n <namespace>
```

### View controller logs

```bash
# PIC logs
kubectl logs -n pangolin-system -l app.kubernetes.io/name=pangolin-ingress-controller

# Operator logs
kubectl logs -n pangolin-operator-system -l control-plane=controller-manager
```

### Force reconciliation

```bash
kubectl annotate ingress <name> reconcile=$(date +%s) --overwrite
```

## License

Apache 2.0
