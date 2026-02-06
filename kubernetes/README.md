# DBT Reposerver Kubernetes Deployment

Kustomize manifests for deploying the DBT reposerver to Kubernetes.

## Quick Start

```bash
# Preview what will be deployed
kubectl kustomize kubernetes/base

# Deploy the base configuration
kubectl apply -k kubernetes/base

# Or create your own overlay (recommended)
cp -r kubernetes/overlays/example kubernetes/overlays/myenv
# Edit kubernetes/overlays/myenv/kustomization.yaml and reposerver.json
kubectl apply -k kubernetes/overlays/myenv
```

## Structure

```
kubernetes/
├── base/                    # Base resources
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── statefulset.yaml
│   ├── service.yaml
│   └── configmap.yaml
└── overlays/
    └── example/             # Example overlay
        ├── kustomization.yaml
        └── reposerver.json
```

## Setting the Version

Update the image tag in your overlay's `kustomization.yaml`:

```yaml
images:
  - name: ghcr.io/nikogura/dbt-reposerver
    newTag: "3.7.5"
```

Or use kustomize CLI:

```bash
cd kubernetes/overlays/myenv
kustomize edit set image ghcr.io/nikogura/dbt-reposerver:3.7.5
```

## Configuration

### Basic Auth (htpasswd)

```json
{
  "address": "0.0.0.0",
  "port": 9999,
  "serverRoot": "/var/dbt",
  "authGets": false,
  "authTypePut": "basic-htpasswd",
  "authOptsPut": {
    "idpFile": "/etc/dbt/htpasswd"
  }
}
```

### OIDC Auth

```json
{
  "address": "0.0.0.0",
  "port": 9999,
  "serverRoot": "/var/dbt",
  "authGets": true,
  "authTypeGet": "oidc",
  "authOptsGet": {
    "oidc": {
      "issuerUrl": "https://dex.example.com",
      "audiences": ["dbt-server"],
      "usernameClaimKey": "email"
    }
  },
  "authTypePut": "oidc",
  "authOptsPut": {
    "oidc": {
      "issuerUrl": "https://dex.example.com",
      "audiences": ["dbt-server"],
      "usernameClaimKey": "email",
      "allowedGroups": ["dbt-admins"]
    }
  }
}
```

## Exposing the Service

The base deployment creates a ClusterIP service. Add an Ingress or HTTPProxy in your overlay:

### Ingress Example

```yaml
# Add to your overlay's kustomization.yaml resources
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dbt-reposerver
spec:
  rules:
    - host: dbt.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: dbt-reposerver
                port:
                  name: http
  tls:
    - hosts:
        - dbt.example.com
      secretName: dbt-tls
```

## Persistent Storage

The StatefulSet uses a PersistentVolumeClaim for data storage. Adjust the size in your overlay:

```yaml
patches:
  - patch: |-
      - op: replace
        path: /spec/volumeClaimTemplates/0/spec/resources/requests/storage
        value: 100Gi
    target:
      kind: StatefulSet
      name: dbt-reposerver
```

## Health Checks

The deployment includes liveness and readiness probes that check the HTTP endpoint on port 9999.
