# TLS Backend Overlay

> The TLS overlay will be reworked with [BackendTLSPolicy](https://gateway-api.sigs.k8s.io/api-types/backendtlspolicy/) once Gateway API v1.4 is supported.

This overlay enables end-to-end TLS between the OpenShift ingress Envoy and the `maas-api` Service without relying on Istio sidecars or ServiceEntries. It patches the MaaS API Deployment/Service to expose both HTTP and HTTPS ports, mounts a secret named `maas-api-backend-tls`, and adds a DestinationRule so that the ingress gateway originates HTTPS directly to `maas-api.maas-api.svc.cluster.local`.

## Dual-Port Architecture

This implementation temporarily uses a dual-port strategy to support both external TLS security and internal metadata access:

**External Traffic Flow (TLS):**

```
Client HTTPS:443 → Gateway (TLS Termination) → HTTPRoute:8443 → Pod HTTPS:8443
```

**Temporary Internal Metadata Flow non-TLS until resolved (HTTP):**

```
Authorino → Service:8080 → Pod HTTP:8080 → /v1/tiers/lookup
```

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

- `Deployment/maas-api` — adds HTTPS env vars, probes, volume mounts, and exposes container port `8443`
- `Service/maas-api` — publishes both `http` (`8080`) and `https` (`8443`) ports; routes now point to `https`
- `HTTPRoute/maas-api-route` — backend references point to port `8443`
- `DestinationRule/maas-api-backend-tls` (in `openshift-ingress`) — configures Envoy to originate TLS with `mode: SIMPLE` and skips verification for the self-signed cert

