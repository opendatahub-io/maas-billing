# MaaS Platform Architecture

## Overview

The MaaS Platform is designed as a cloud-native, Kubernetes-based solution that provides policy-based access control, rate limiting, and tier-based subscriptions for AI model serving. The architecture follows microservices principles and leverages OpenShift/Kubernetes native components for scalability and reliability.

## Architecture

### 🏗️ High-Level Architecture

The MaaS Platform is an end-to-end solution that leverages Red Hat Connectivity Link (Kuadrant) Application Connectivity Policies and Red Hat OpenShift AI's Model Serving capabilities to provide a fully managed, scalable, and secure self-service platform for AI model serving.

```mermaid
graph LR
    subgraph "User Layer"
        User[Users]
        AdminUI[Admin/User UI]
    end
    
    subgraph "Token Management"
        MaaSAPI[MaaS API<br/>Token Retrieval]
    end
    
    subgraph "Gateway & Auth"
        GatewayAPI[Gateway API]
        RHCL[RHCL Components<br/>Kuadrant/Authrino/Limitador<br/>Auth & Rate Limiting]
    end
    
    subgraph "Model Serving"
        RHOAI[RHOAI<br/>LLM Models]
    end
    
    User -->|1. Request Token| AdminUI
    User -->|1a. Direct Token Request| MaaSAPI
    AdminUI -->|2. Retrieve Token| MaaSAPI
    User -->|3. Inference Request<br/>with Token| GatewayAPI
    GatewayAPI -->|4. Auth & Rate Limit| RHCL
    RHCL -->|5. Forward to Model| RHOAI
    
    style MaaSAPI fill:#e1f5fe
    style GatewayAPI fill:#f3e5f5
    style RHCL fill:#fff3e0
    style RHOAI fill:#e8f5e8
```

### Architecture Details

The MaaS Platform architecture is designed to be modular and scalable. It is composed of the following components:

- **MaaS API**: The central component for token generation and management.
- **Gateway API**: The entry point for all inference requests.
- **Kuandrant (Red Hat Connectivity Link)**: The policy engine for authentication and authorization.
- **Open Data Hub (Red Hat OpenShift AI)**: The model serving platform.

### Detailed Component Architecture

### MaaS API Component Details

The MaaS API provides a self-service platform for users to request tokens for their inference requests. By leveraging Kubernetes native objects like ConfigMaps and ServiceAccounts, it offers model owners a simple way to configure access to their models based on a familiar group-based access control model.

```mermaid
graph TB
    subgraph "External Access"
        User[Users]
        AdminUI[Admin/User UI]
    end
    
    subgraph "MaaS API Service"
        API[**MaaS API**<br/>Go + Gin Framework]
        TierMapping[**Tier Mapping Logic**]
        TokenGen[**Service Account Token Generation**]
    end
    
    subgraph "Configuration"
        ConfigMap[**ConfigMap**<br/>tier-to-group-mapping]
        K8sGroups[**Kubernetes Groups**<br/>tier-free-users<br/>tier-premium-users<br/>tier-enterprise-users]
    end
    
    subgraph "free namespace"
        FreeSA1[**ServiceAccount**<br/>freeuser1-sa]
        FreeSA2[**ServiceAccount**<br/>freeuser2-sa]
    end
    
    subgraph "premium namespace"
        PremiumSA1[**ServiceAccount**<br/>prem-user1-sa]
    end
    
    subgraph "enterprise namespace"
        EnterpriseSA1[**ServiceAccount**<br/>ent-user1-sa]
    end
    
    User -->|Direct API Call| API
    AdminUI -->|Token Request| API
    
    API --> TierMapping
    API --> TokenGen
    
    TierMapping --> ConfigMap
    ConfigMap -->|Maps Groups to Tiers| K8sGroups
    TokenGen --> FreeSA1
    TokenGen --> FreeSA2
    TokenGen --> PremiumSA1
    TokenGen --> EnterpriseSA1
    
    K8sGroups -->|Group Membership| TierMapping
    
    style API fill:#e1f5fe
    style ConfigMap fill:#ffeb3b
    style K8sGroups fill:#ffeb3b
    style FreeSA1 fill:#ffeb3b
    style FreeSA2 fill:#ffeb3b
    style PremiumSA1 fill:#ffeb3b
    style EnterpriseSA1 fill:#ffeb3b
```

**Key Features:**

- **Tier-to-Group Mapping**: Uses ConfigMap in the same namespace as MaaS API to map Kubernetes groups to tiers
- **Three Configurable Default Tiers**: Out of the box, the MaaS Platform comes with three default tiers: free, premium, and enterprise. These tiers are configurable and can be extended to support more tiers as needed.
- **Service Account Tokens**: Generates tokens for the appropriate tier's service account based on user's group membership
- **Future Enhancements**: Planned improvements for more sophisticated token management and the ability to integrate with external identity providers.

### Inference Service Component Details

Once a user has obtained their token through the MaaS API, they can use it to make inference requests to the Gateway API. RHCL's Application Connectivity Policies then validate the token and enforce access control and rate limiting policies:

```mermaid
graph TB
    subgraph "Client Layer"
        Client[Client Applications<br/>with Service Account Token]
    end
    
    subgraph "Gateway Layer"
        GatewayAPI[**maas-default-gateway**<br/>maas.CLUSTER_DOMAIN]
        Envoy[**Envoy Proxy**]
    end
    
    subgraph "RHCL Policy Engine"
        Kuadrant[**Kuadrant**<br/>Policy Attachment]
        Authorino[**Authorino**<br/>Authentication Service]
        Limitador[**Limitador**<br/>Rate Limiting Service]
    end
    
    subgraph "Policy Components"
        AuthPolicy[**AuthPolicy**<br/>gateway-auth-policy]
        RateLimitPolicy[**RateLimitPolicy**<br/>gateway-rate-limits]
        TokenRateLimitPolicy[**TokenRateLimitPolicy**<br/>gateway-token-rate-limits]
    end
    
    subgraph "Model Access Control"
        RBAC[**Kubernetes RBAC**<br/>Service Account Permissions]
        LLMInferenceService[**LLMInferenceService**<br/>Model Access Control]
    end
    
    subgraph "Model Serving"
        RHOAI[**RHOAI Platform**]
        Models[**LLM Models**<br/>Qwen, Granite, Llama]
    end
    
    subgraph "Observability"
        Prometheus[**Prometheus**<br/>Metrics Collection]
        Dashboards[**Observability Stack**<br/>Grafana/Perses Dashboards]
    end
    
    Client -->|Inference Request + Service Account Token| GatewayAPI
    GatewayAPI --> Envoy
    
    Envoy --> Kuadrant
    Kuadrant --> Authorino
    Kuadrant --> Limitador
    
    Authorino --> AuthPolicy
    Limitador --> RateLimitPolicy
    Limitador --> TokenRateLimitPolicy
    
    Envoy -->|Check Model Access| RBAC
    RBAC --> LLMInferenceService
    LLMInferenceService -->|POST Permission Check| RHOAI
    RHOAI --> Models
    
    Limitador -->|Usage Metrics| Prometheus
    Prometheus --> Dashboards
    
    style GatewayAPI fill:#f3e5f5
    style Kuadrant fill:#fff3e0
    style Authorino fill:#fff3e0
    style Limitador fill:#fff3e0
    style AuthPolicy fill:#ffeb3b
    style RateLimitPolicy fill:#ffeb3b
    style TokenRateLimitPolicy fill:#ffeb3b
    style RBAC fill:#ffeb3b
    style LLMInferenceService fill:#ffeb3b
    style RHOAI fill:#e8f5e8
```

**Policy Engine Flow:**

1. **User Request**: A user makes an inference request to the Gateway API with a valid token.
2. **Service Account Authentication**: Authorino validates service account tokens using gateway-auth-policy
3. **Rate Limiting**: Limitador enforces usage quotas per tier/user using gateway-rate-limits and gateway-token-rate-limits
4. **Model Access Control**: RBAC checks if service account has POST access to the specific LLMInferenceService
5. **Request Forwarding**: Only requests with proper model access are forwarded to RHOAI
6. **Metrics Collection**: Limitador sends usage data to Prometheus for observability dashboards

## 🔄 Component Flows

### 1. Token Retrieval Flow (MaaS API)

The MaaS API handles token generation and management for different user tiers:

```mermaid
sequenceDiagram
    participant User
    participant AdminUI[Admin/User UI]
    participant MaaSAPI[MaaS API]
    participant TokenDB[(Token Database)]
    participant TierDB[(Tier Database)]
    
    User->>AdminUI: Request Token
    AdminUI->>MaaSAPI: POST /tokens
    MaaSAPI->>TierDB: Check User Tier
    TierDB-->>MaaSAPI: Tier Limits & Permissions
    MaaSAPI->>TokenDB: Generate Token
    TokenDB-->>MaaSAPI: Token + Metadata
    MaaSAPI-->>AdminUI: Token Response
    AdminUI-->>User: Token for Inference
    
    Note over MaaSAPI: Token includes:<br/>- User ID<br/>- Tier Level<br/>- Rate Limits<br/>- Model Access
```

### 2. Gateway & Authentication Flow

The Gateway API and RHCL components handle authentication and rate limiting:

```mermaid
sequenceDiagram
    participant Client
    participant GatewayAPI[Gateway API]
    participant Kuadrant[Kuadrant]
    participant Authrino[Authrino]
    participant Limitador[Limitador]
    participant MaaSAPI[MaaS API]
    
    Client->>GatewayAPI: Inference Request + Token
    GatewayAPI->>Kuadrant: Apply Auth Policy
    Kuadrant->>Authrino: Validate Token
    Authrino->>MaaSAPI: Check Token Validity
    MaaSAPI-->>Authrino: Token Status + Tier Info
    Authrino-->>Kuadrant: Auth Result
    Kuadrant->>Limitador: Check Rate Limits
    Limitador-->>Kuadrant: Rate Limit Status
    Kuadrant-->>GatewayAPI: Policy Decision
    GatewayAPI-->>Client: Forward to Model or Reject
```

### 3. Model Inference Flow

The inference flow routes validated requests to RHOAI models:

```mermaid
sequenceDiagram
    participant Client
    participant GatewayAPI[Gateway API]
    participant RHCL[RHCL Components]
    participant RHOAI[RHOAI Platform]
    participant Model[LLM Model]
    
    Client->>GatewayAPI: POST /v1/models/{model}/infer
    GatewayAPI->>RHCL: Validate Request & Token
    RHCL-->>GatewayAPI: Validation Success
    GatewayAPI->>RHOAI: Forward Inference Request
    RHOAI->>Model: Process Inference
    Model-->>RHOAI: Inference Result
    RHOAI-->>GatewayAPI: Response
    GatewayAPI-->>Client: Model Response
    
    Note over RHCL: Updates metrics:<br/>- Token usage<br/>- Request count<br/>- Tier consumption
```

## Core Components

### MaaS API (Token Management)

The MaaS API is the central component for token generation and management:

- **Token Generation**: Creates secure tokens with embedded metadata
- **Tier Management**: Enforces subscription tier limits and permissions
- **User Authentication**: Validates user credentials and permissions
- **Rate Limit Configuration**: Sets token-specific rate limits based on tier

### Gateway API & RHCL Components

The gateway layer provides policy-based request handling:

- **Gateway API**: Entry point for all inference requests
- **Kuadrant**: Policy attachment point for authentication and authorization
- **Authrino**: Authentication and authorization service that validates tokens
- **Limitador**: Rate limiting service that enforces usage quotas

### RHOAI (Model Serving)

Red Hat OpenShift AI provides the model serving infrastructure:

- **Model Hosting**: Runs LLM models (Qwen, Granite, Llama, etc.)
- **Scaling**: Automatic scaling based on demand
- **Resource Management**: GPU allocation and management
- **Model Lifecycle**: Model deployment, updates, and retirement

## Architecture Benefits

### Security
- **Token-based Authentication**: Secure, stateless authentication
- **Policy Enforcement**: Centralized security policies via Kuadrant
- **Rate Limiting**: Prevents abuse and ensures fair resource usage

### Scalability
- **Microservices Architecture**: Independent scaling of components
- **Kubernetes Native**: Leverages OpenShift/Kubernetes scaling capabilities
- **Tier-based Resource Allocation**: Different service levels for different user tiers

### Observability
- **Comprehensive Metrics**: Token usage, request rates, and tier consumption
- **Centralized Logging**: All components log to centralized systems
- **Monitoring**: Real-time monitoring of system health and performance
