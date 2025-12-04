# Install Red Hat OpenShift AI

This guide covers the installation of Red Hat OpenShift AI (RHOAI), with the required
configuration to enable the Models-as-a-Service capability (MaaS).

You need a Red Hat OpenShift cluster version 4.19.9 or later. Older OpenShift versions are
not suitable.

MaaS requires RHOAI Model Serving component configured for deploying models with
`LLMInferenceService` resources. The prerequisites for this setup are Red Hat Connectivity
Link (RHCL) and the LeaderWorkerSet API (LWS).

Tools you will need:

* kubectl or oc client (this guide uses kubectl)

!!! warning
    You should choose either to install Red Hat OpenShift AI, or the Open Data Hub
    project (ODH). Follow this guide only if your cluster does not have ODH installed.

!!! note
    This guide is provided for convenience. In case of any issues or more advanced
    setups, refer to the Red Hat documentation of the installed components.


## Install LeaderWorkerSet API

Install Red Hat LeaderWorkerSet API (LWS) Operator from OpenShift's built-in OperatorHub.
This can be achieved by applying the following YAML in the cluster:

```yaml
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
```

Once the LWS operator is ready, set up the LWS API by applying the following YAML:

```yaml
apiVersion: operator.openshift.io/v1
kind: LeaderWorkerSetOperator
metadata:
  name: cluster
  namespace: openshift-lws-operator
spec:
  managementState: Managed
```

Check [Red Hat LWS documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/ai_workloads/leader-worker-set-operator)
if you need further guidance.

### Verification

Check that LWS deployments are ready:

```shell
kubectl get deployments --namespace openshift-lws-operator

NAME                     READY   UP-TO-DATE   AVAILABLE   AGE
lws-controller-manager   2/2     2            2           61s
openshift-lws-operator   1/1     1            1           4m26s
```

## Install Red Hat Connectivity Link

Initialize OpenShift's provided Gateway API implementation by creating the following
resource:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: openshift-default
spec:
  controllerName: "openshift.io/gateway-controller/v1"
```

Wait until the GatewayClass resource is accepted:

```shell
kubectl get gatewayclass openshift-default

NAME                CONTROLLER                           ACCEPTED   AGE
openshift-default   openshift.io/gateway-controller/v1   True       52s
```

Install Red Hat Connectivity Link (RHCL) Operator from OpenShift's built-in OperatorHub.
MaaS requires RHCL v1.2 or later. This can be achieved by applying the following YAML in
the cluster:

```yaml
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
```

Once the RHCL operator is ready, create a Connectivity Link instance by applying the
following YAML:

```yaml
apiVersion: kuadrant.io/v1beta1
kind: Kuadrant
metadata:
  name: kuadrant
  namespace: kuadrant-system
```

Check [RHCL documentation](https://docs.redhat.com/en/documentation/red_hat_connectivity_link)
if you need further guidance.

### Verification

Check that RHCL deployments are ready:

```shell
kubectl get deployments -n kuadrant-system

NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
authorino-operator                      1/1     1            1           80s
dns-operator-controller-manager         1/1     1            1           77s
kuadrant-console-plugin                 1/1     1            1           58s
kuadrant-operator-controller-manager    1/1     1            1           69s
limitador-operator-controller-manager   1/1     1            1           73s
authorino                               1/1     1            1           81s
limitador-limitador                     1/1     1            1           82s
```

## Install Red Hat OpenShift AI with Model Serving

First, set up the inference Gateway, required by RHOAI's Model Serving, by creating the
following resource:

```yaml
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
```

Install Red Hat OpenShift AI (RHOAI) Operator from OpenShift's built-in OperatorHub. MaaS
requires RHOAI v3.0 or later. This can be achieved by applying the following YAML in the
cluster:

```yaml
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
```

Once ready, the RHOAI Operator should automatically create a `DSCInitialization`
resource. Install the Model Serving component by creating the following
`DataScienceCluster` resource:

```yaml
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
```

Check [RHOAI documentation](https://docs.redhat.com/en/documentation/red_hat_openshift_ai_self-managed)
if you need further guidance.

### Verification

Check that RHOAI Model Serving Deployments are ready:

```shell
kubectl get deployments -n redhat-ods-applications

NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
kserve-controller-manager   1/1     1            1           73s
odh-model-controller        1/1     1            1           79s
rhods-dashboard             2/2     2            2           78s
```
