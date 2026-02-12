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

The base deployment creates a ClusterIP service. Add an Ingress or HTTPProxy in your overlay.

### Ingress Example

```yaml
# Add to your overlay's kustomization.yaml resources
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dbt-reposerver
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
    nginx.ingress.kubernetes.io/proxy-read-timeout: "600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "600"
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

The reposerver accepts binary uploads via HTTP PUT, so the proxy body size limit must
be large enough to accommodate the largest artifact you intend to publish. Adjust
`proxy-body-size` to suit your needs. The timeout annotations prevent the ingress
controller from dropping long-running uploads on slower connections.

### Web Application Firewalls (ModSecurity / OWASP CRS)

If your ingress controller runs ModSecurity with the OWASP Core Rule Set (CRS), you
**must** disable request body inspection for the reposerver. The reposerver's entire
purpose is accepting uploaded binary artifacts (compiled Go binaries, signatures,
checksums), and binary content will trigger CRS rules designed to detect SQL injection,
XSS, command injection, and other attacks in text-based request bodies. Random byte
sequences in compiled binaries match dozens of these patterns simultaneously, producing
anomaly scores far above the default threshold and causing uploads to be silently
blocked with HTTP 403 or dropped connections (HTTP 000).

This is not a matter of tuning individual rules — binary content is fundamentally
incompatible with text-oriented body inspection. Disabling individual rules only
addresses a subset of the triggers; the next binary you upload will hit different
patterns and fail again.

For nginx-ingress with ModSecurity enabled, add:

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/modsecurity-snippet: |
      SecRequestBodyAccess Off
```

This disables body content inspection while leaving all other ModSecurity protections
intact (response inspection, protocol enforcement, header checks, rate limiting, etc.).

**Why this is safe:** The reposerver authenticates every mutating request (PUT, DELETE)
via bearer token, OIDC, htpasswd, or a combination of these. It serves only as a
static file store — it does not parse, execute, or interpret uploaded content. There is
no injection surface in the request body because the server writes it directly to disk
without processing. The real security boundary is the authentication layer, not body
content filtering.

If your WAF does not support `SecRequestBodyAccess Off` (e.g., cloud WAF products),
you will need to create an exception or bypass rule for the reposerver's path or
virtual host that skips body inspection for PUT requests.

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
