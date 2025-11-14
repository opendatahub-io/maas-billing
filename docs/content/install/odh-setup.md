# Install Open Data Hub project

This guide covers the installation of the Open Data Hub project, with the required
configuration to deploy the Model-as-a-Service capability (MaaS).

You need a Red Hat OpenShift cluster version 4.19.9 or later. Older OpenShift
versions are not suitable.

MaaS requires ODH's Model Serving component configured for deploying models with
`LLMInferenceService` resources. The prerequisites for this ODH setup are Kuadrant and the
LeaderWorkerSet API (LWS).

Tools you will need:

* kubectl or oc client (this guide uses kubectl)
* curl
* jq

!!! warning
    You should choose either to install the ODH project, or Red Hat OpenShift AI (RHOAI). 
    Follow this guide only if your cluster does not have RHOAI installed.  


## Install LeaderWorkerSet API

Install the latest version of LWS by using the _kubectl_ method from
[LWS official documentation](https://lws.sigs.k8s.io/docs/installation/#install-by-kubectl).
The following script will do so:

```shell
GH_LATEST_LWS_ENTRY_URL="https://api.github.com/repos/kubernetes-sigs/lws/releases"
LATEST_LWS_VERSION=$(curl -sSf ${GH_LATEST_LWS_ENTRY_URL} | jq -r 'sort_by(.tag_name|ltrimstr("v")|split(".")|map(tonumber)) | last | .tag_name')

kubectl apply --server-side -f https://github.com/kubernetes-sigs/lws/releases/download/${LATEST_LWS_VERSION}/manifests.yaml
```

### Verification

Check that LWS deployments are ready:

```shell
kubectl get deployments --namespace lws-system

NAME                     READY   UP-TO-DATE   AVAILABLE   AGE
lws-controller-manager   2/2     2            2           35s
```

## Install Kuadrant

[Kuadrant official documentation](https://docs.kuadrant.io/latest/install-olm/)
is used as a base to install Kuadrant's latest version (v1.3.0+ is required) using the
OLM method.

Start by creating the `kuadrant-system` namespace:

```shell
kubectl create namespace kuadrant-system
```

Create an OperatorGroup in the `kuadrant-system` namespace.

```shell
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: kuadrant-operator-group
  namespace: kuadrant-system
spec: {}
EOF
```

!!! note
    A single OperatorGroup should exist in any given namespace. Check for the
    existence of multiple OperatorGroups if Kuadrant operator is not deployed 
    successfully.

Configure Kuadrant's CatalogSource:

```shell
# Find latest Kuadrant operator version:
GH_LATEST_KUADRANT_ENTRY_URL="https://api.github.com/repos/Kuadrant/kuadrant-operator/releases/latest"
LATEST_KUADRANT_VERSION=$(curl -sSf ${GH_LATEST_KUADRANT_ENTRY_URL} | jq -r '.tag_name')

# Install the CatalogSource
kubectl apply -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: kuadrant-operator-catalog
  namespace: kuadrant-system
spec:
  displayName: Kuadrant Operators
  image: quay.io/kuadrant/kuadrant-operator-catalog:${LATEST_KUADRANT_VERSION}
  sourceType: grpc
EOF
```

Deploy the Kuadrant operator, configuring it to work with OpenShift's provided Gateway API
implementation:

```shell
kubectl apply -f - <<EOF
  apiVersion: operators.coreos.com/v1alpha1
  kind: Subscription
  metadata:
    name: kuadrant-operator
    namespace: kuadrant-system
  spec:
    channel: stable
    installPlanApproval: Automatic
    name: kuadrant-operator
    source: kuadrant-operator-catalog
    sourceNamespace: kuadrant-system
    config:
      env:
      - name: "ISTIO_GATEWAY_CONTROLLER_NAMES"
        value: "openshift.io/gateway-controller/v1"
EOF
```

### Verification

Check that Kuadrant deployments are ready:

```shell
kubectl get deployments -n kuadrant-system

NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
authorino-operator                      1/1     1            1           80s
dns-operator-controller-manager         1/1     1            1           77s
kuadrant-console-plugin                 1/1     1            1           58s
kuadrant-operator-controller-manager    1/1     1            1           69s
limitador-operator-controller-manager   1/1     1            1           73s
```

## Install Open Data Hub with Model Serving

The Open Data Hub Project (ODH) is installed via its operator, which is available in
OpenShift's preconfigured CatalogSource of community operators. Create the following
Subscription to install the latest version of ODH Operator (version 3.0 or later is 
required for MaaS):

```shell
kubectl apply -f - <<EOF
  apiVersion: operators.coreos.com/v1alpha1
  kind: Subscription
  metadata:
    name: opendatahub-operator
    namespace: openshift-operators
  spec:
    channel: fast-3
    name: opendatahub-operator
    source: community-operators
    sourceNamespace: openshift-marketplace
EOF
```

Set up the inference Gateway, required by ODH's Model Serving, by creating the 
following resources:

```shell
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: openshift-default
spec:
  controllerName: "openshift.io/gateway-controller/v1"
---
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
EOF
```

Install the ODH Model Serving component by creating two resources:
1. A `DSCInitialization` resource to initialize the ODH platform
2. A `DataScienceCluster` resource to install ODH components

```shell
kubectl apply -f - <<EOF
apiVersion: dscinitialization.opendatahub.io/v2
kind: DSCInitialization
metadata:
  name: default-dsci
spec:
  applicationsNamespace: opendatahub
  monitoring:
    managementState: Managed
    namespace: opendatahub
    metrics: {}
  trustedCABundle:
    managementState: Managed
---
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
```
