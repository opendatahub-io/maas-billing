## Overview

```shell
├── base          <1>
├── infra         <2>
│   ├── kuadrant
│   ├── odh       <*>
│   └── openshift-gateway-api
├── models        <3>
├── overlays      <4>
│   ├── dev
│   ├── odh
│   └── secret
├── policies      <5>
└── rbac          <6>
```

**<1> base** - Core MaaS API deployment manifests (service, deployment) with common labels and RBAC

**<2> infra** - Infrastructure dependencies for Gateway API, Kuadrant, and OpenDataHub integration
  * `<*>` - ODH minimal deployment to support models (`LLMInferenceService` machinery)

**<3> models** - Model simulation resources for testing and development environments

**<4> overlays** - Environment-specific configurations:
- `dev` - Development overlay with debug mode and local infrastructure
- `odh` - OpenDataHub operator overlay for core MaaS API component deployment (no policies/infra)
- `secret` - Secret provider-based deployment configuration

**<5> policies** - Kuadrant policies for authentication, rate limiting, and token management

**<6> rbac** - Role-based access control manifests (ServiceAccount, ClusterRole, ClusterRoleBinding)