#!/bin/bash
#
# deploy-rhoai-stable.sh - Deploy Red Hat OpenShift AI v3 with Model-as-a-Service standalone capability
#
# DESCRIPTION:
#   This script automates the deployment of Red Hat OpenShift AI (RHOAI) v3 along with
#   its required prerequisites and the Model-as-a-Service (MaaS) capability.
#
#   The deployment includes:
#   - cert-manager
#   - Leader Worker Set (LWS)
#   - Red Hat Connectivity Link
#   - RHOAI v3 with KServe for model serving
#   - MaaS standalone capability (Developer Preview)
#
# PREREQUISITES:
#   - OpenShift cluster v4.19.9+
#   - Cluster administrator privileges
#   - kubectl CLI tool configured and connected to cluster
#   - kustomize tool available in PATH
#   - jq tool for JSON processing
#
# USAGE:
#   ./deploy-rhoai-stable.sh
#
# NOTES:
#   - The script is idempotent for most operations
#   - No arguments are expected

set -e

waitsubscriptioninstalled() {
  local ns=${1?namespace is required}; shift
  local name=${1?subscription name is required}; shift

  echo "  * Waiting for Subscription $ns/$name to start setup..."
  kubectl wait subscription --timeout=300s -n $ns $name --for=jsonpath='{.status.currentCSV}'
  local csv=$(kubectl get subscription -n $ns $name -o jsonpath='{.status.currentCSV}')
  sleep 5 # Because, sometimes, the CSV is not there immediately.

  echo "  * Waiting for Subscription setup to finish setup. CSV = $csv ..."
  kubectl wait -n $ns --for=jsonpath="{.status.phase}"=Succeeded csv $csv --timeout=600s
  if [ $? -ne 0 ]; then
    echo "    * ERROR: Timeout while waiting for Subscription to finish installation."
    exit 1
  fi
}

deploy_certmanager() {
  echo
  echo "* Installing cert-manager operator..."

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager-operator
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: cert-manager-operator
  namespace: cert-manager-operator
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-cert-manager-operator
  namespace: cert-manager-operator
spec:
  channel: stable-v1
  installPlanApproval: Automatic
  name: openshift-cert-manager-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF

  waitsubscriptioninstalled "cert-manager-operator" "openshift-cert-manager-operator"
}

deploy_lws() {
  echo
  echo "* Installing LWS operator..."

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: openshift-lws-operator
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: leader-worker-set
  namespace: openshift-lws-operator
spec:
  targetNamespaces:
  - openshift-lws-operator
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: leader-worker-set
  namespace: openshift-lws-operator
spec:
  channel: stable-v1.0
  installPlanApproval: Automatic
  name: leader-worker-set
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF

  waitsubscriptioninstalled "openshift-lws-operator" "leader-worker-set"
  echo "* Setting up LWS instance and letting it deploy asynchronously."

  cat <<EOF | kubectl apply -f -
apiVersion: operator.openshift.io/v1
kind: LeaderWorkerSetOperator
metadata:
  name: cluster
  namespace: openshift-lws-operator
spec:
  managementState: Managed
EOF
}

deploy_rhcl() {
  echo
  echo "* Initializing Gateway API provider..."

  cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: openshift-default
spec:
  controllerName: "openshift.io/gateway-controller/v1"
EOF

  echo "  * Waiting for GatewayClass openshift-default to transition to Accepted status..."
  kubectl wait --timeout=300s --for=condition=Accepted=True GatewayClass/openshift-default

  echo
  echo "* Installing RHCL operator..."

  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: kuadrant-system
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: kuadrant-operator-group
  namespace: kuadrant-system
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: kuadrant-operator
  namespace: kuadrant-system
spec:
  channel: stable
  installPlanApproval: Automatic
  name: rhcl-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF

  waitsubscriptioninstalled "kuadrant-system" "kuadrant-operator"
  echo "* Setting up RHCL instance..."

  cat <<EOF | kubectl apply -f -
apiVersion: kuadrant.io/v1beta1
kind: Kuadrant
metadata:
  name: kuadrant
  namespace: kuadrant-system
EOF
}

deploy_rhoai() {
  echo
  echo "* Installing RHOAI v3 operator..."

  cat <<EOF | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: openshift-ai-inference
  namespace: openshift-ingress
spec:
  gatewayClassName: openshift-default
  listeners:
  - name: http
    port: 80
    protocol: HTTP
    allowedRoutes:
      namespaces:
        from: All
  infrastructure:
    labels:
      serving.kserve.io/gateway: kserve-ingress-gateway
---
apiVersion: v1
kind: Namespace
metadata:
  name: redhat-ods-operator
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: rhoai3-operatorgroup
  namespace: redhat-ods-operator
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: rhoai3-operator
  namespace: redhat-ods-operator
spec:
  channel: fast-3.x
  installPlanApproval: Automatic
  name: rhods-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF

  waitsubscriptioninstalled "redhat-ods-operator" "rhoai3-operator"
  echo "* Setting up RHOAI instance and letting it deploy asynchronously."

  cat <<EOF | kubectl apply -f -
apiVersion: datasciencecluster.opendatahub.io/v2
kind: DataScienceCluster
metadata:
  name: default-dsc
spec:
  components:
    # Components required for MaaS:
    kserve:
      managementState: Managed
      rawDeploymentServiceConfig: Headed

    # Components recommended for MaaS:
    dashboard:
      managementState: Managed
EOF
}

echo "## Installing prerequisites"

deploy_certmanager
deploy_lws
deploy_rhcl
deploy_rhoai

echo
echo "## Installing Model-as-a-Service"

export CLUSTER_DOMAIN=$(kubectl get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
export AUD="$(kubectl create token default --duration=10m 2>/dev/null | cut -d. -f2 | base64 -d 2>/dev/null | jq -r '.aud[0]' 2>/dev/null)"

echo "* Cluster domain: ${CLUSTER_DOMAIN}"
echo "* Cluster audience: ${AUD}"

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: maas-api
EOF

# TODO: Use correct ref=tag
kubectl apply --server-side=true \
  -f <(kustomize build "https://github.com/opendatahub-io/maas-billing.git/deployment/overlays/openshift?ref=main" | \
       envsubst '$CLUSTER_DOMAIN')

if [[ -n "$AUD" && "$AUD" != "https://kubernetes.default.svc"  ]]; then
  echo "* Configuring audience in MaaS AuthPolicy"
  kubectl patch authpolicy maas-api-auth-policy -n maas-api --type=merge --patch-file <(echo "
spec:
  rules:
    authentication:
      openshift-identities:
        kubernetesTokenReview:
          audiences:
            - $AUD
            - maas-default-gateway-sa")
fi

# Patch maas-api Deployment with stable image
kubectl set image -n maas-api deployment/maas-api maas-api=registry.redhat.io/rhoai/odh-maas-api-rhel9:v3.0.0