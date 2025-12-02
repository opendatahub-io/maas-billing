# MaaS Installation Overview

Currently, ODH's _Models-as-a-Service_ is provided as a standalone capability that is
compatible with the Open Data Hub project (ODH), and with Red Hat OpenShift AI (RHOAI). To
install MaaS:

* Install the [Open Data Hub project](odh-setup.md),
  or install [Red Hat OpenShift AI](rhoai-setup.md).
* [Install MaaS using Kustomize manifests](maas-setup.md).

MaaS inherits the platform requirement for a Red Hat OpenShift cluster version 4.19.9 or
later, which is the version that has formal support for Gateway API. For earlier OpenShift
versions, there are alternatives (e.g. see a [guide here](https://github.com/opendatahub-io/kserve/tree/release-v0.15/docs/samples/llmisvc/ocp-4-18-setup)),
but we provide no support for such setups.

## Requirements for Open Data Hub project

MaaS requires Open Data Hub version 3.0 or later, with the Model Serving component
enabled (KServe) and properly configured for deploying models with `LLMInferenceService`
resources.

A specific requirement for MaaS is to set up ODH's Model Serving with Kuadrant v1.3+, even
though ODH can work with earlier Kuadrant versions.

## Requirements for Red Hat OpenShift AI

MaaS requires Red Hat OpenShift AI (RHOAI) version 3.0 or later, with the Model Serving
component enabled (KServe) and properly configured for deploying models with
`LLMInferenceService` resources.

A specific requirement for MaaS is to set up RHOAI Model Serving with Red Hat Connectivity
Link (RHCL) v1.2+, even though RHOAI can work with earlier RHCL versions.
