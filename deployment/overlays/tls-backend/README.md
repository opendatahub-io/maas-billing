# TLS Backend Overlay

> The TLS overlay will be reworked with [BackendTLSPolicy](https://gateway-api.sigs.k8s.io/api-types/backendtlspolicy/) once Gateway API v1.4 is supported.

This overlay enables end-to-end TLS between the OpenShift ingress Envoy and the `maas-api` Service without relying on Istio sidecars or ServiceEntries. It patches the MaaS API Deployment/Service to use HTTPS-only communication, mounts a secret named `maas-api-backend-tls`, and configures both the ingress gateway and Authorino to communicate with the backend over TLS.

## TLS-Only Architecture

This implementation uses HTTPS-only communication with the maas-api backend:

**External Traffic Flow (TLS):**

```
Client HTTPS:443 → Gateway (TLS Termination) → HTTPRoute:8443 → Pod HTTPS:8443
```

**Internal Metadata Flow (TLS):**

```
Authorino → Service:8443 → Pod HTTPS:8443 → /v1/tiers/lookup
```

## Authorino TLS Configuration

The overlay configures Authorino to trust the maas-api backend certificate for secure metadata lookups:

- Creates a CA ConfigMap (`maas-api-ca-cert`) containing the self-signed certificate
- Mounts the CA bundle in the Authorino deployment at `/etc/ssl/certs/maas-api-ca.pem`
- Sets `SSL_CERT_FILE` and `REQUESTS_CA_BUNDLE` environment variables to use the mounted CA
- Patches the AuthPolicy to use HTTPS URL for tier metadata lookups

## Generate or Rotate TLS

The `deploy-openshift.sh` script will perform all of these steps.

Use the helper script to mint a self-signed certificate that covers `maas-api.maas-api.svc` and `maas-api.maas-api.svc.cluster.local`, then insert the secret that the Deployment consumes:

```bash
./deployment/scripts/create-maas-api-cert.sh
```

- Secrets are stored as `maas-api-backend-tls` in the `maas-api` namespace
- Certificates are written under `deployment/overlays/tls-backend/certs/`
- Pass `--force` to regenerate the keypair or set `NAMESPACE`/`SECRET_NAME` to override defaults

## Apply the Overlay

```bash
kustomize build deployment/overlays/tls-backend | envsubst '$CLUSTER_DOMAIN' | kubectl apply -f -
kubectl rollout restart deployment/maas-api -n maas-api
kubectl rollout status deployment/maas-api -n maas-api --timeout=180s
```

The overlay patches:

- `Deployment/maas-api` — adds HTTPS-only configuration with TLS env vars, HTTPS probes, volume mounts, and disables HTTP listener
- `Service/maas-api` — configures HTTPS port `8443` as primary service port
- `HTTPRoute/maas-api-route` — backend references point to port `8443`
- `AuthPolicy/gateway-auth-policy` — updates metadata lookup URL to use HTTPS
- `DestinationRule/maas-api-backend-tls` (in `openshift-ingress`) — configures Envoy to originate TLS with `mode: SIMPLE` and skips verification for the self-signed cert
- `Deployment/authorino` (in `kuadrant-system`) — mounts CA certificate and configures TLS trust for backend communication
- `ConfigMap/maas-api-ca-cert` (in `kuadrant-system`) — contains the CA certificate for backend TLS validation

