# MaaS Platform Architecture

## Overview

The MaaS Platform is designed as a cloud-native, Kubernetes-based solution that provides policy-based access control, rate limiting, and tier-based subscriptions for AI model serving. The architecture follows microservices principles and leverages OpenShift/Kubernetes native components for scalability and reliability.

## 🏗️ High-Level Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        WebUI[Web UI]
        API[API Clients]
        CLI[CLI Tools]
    end
    
    subgraph "Gateway Layer"
        Gateway[Gateway API]
        Auth[Authentication]
        RateLimit[Rate Limiting]
        Policy[Policy Engine]
    end
    
    subgraph "Service Layer"
        MaaSAPI[MaaS API]
        ModelService[Model Service]
        TokenService[Token Service]
        TierService[Tier Service]
    end
    
    subgraph "Model Layer"
        KServe[KServe]
        Model1[Model 1]
        Model2[Model 2]
        ModelN[Model N]
    end
    
    subgraph "Data Layer"
        ConfigMap[ConfigMaps]
        Secret[Secrets]
        PVC[Persistent Volumes]
    end
    
    subgraph "Observability"
        Prometheus[Prometheus]
        Grafana[Grafana]
        Logs[Log Aggregation]
    end
    
    WebUI --> Gateway
    API --> Gateway
    CLI --> Gateway
    
    Gateway --> Auth
    Gateway --> RateLimit
    Gateway --> Policy
    
    Gateway --> MaaSAPI
    MaaSAPI --> ModelService
    MaaSAPI --> TokenService
    MaaSAPI --> TierService
    
    ModelService --> KServe
    KServe --> Model1
    KServe --> Model2
    KServe --> ModelN
    
    MaaSAPI --> ConfigMap
    MaaSAPI --> Secret
    KServe --> PVC
    
    MaaSAPI --> Prometheus
    Gateway --> Prometheus
    KServe --> Prometheus
    Prometheus --> Grafana
    Gateway --> Logs
    MaaSAPI --> Logs
```

## 🔄 Request Flow

### 1. Authentication Flow

```mermaid
sequenceDiagram
    participant Client
    participant Gateway
    participant Auth
    participant MaaSAPI
    participant Model
    
    Client->>Gateway: Request with Token
    Gateway->>Auth: Validate Token
    Auth->>MaaSAPI: Check Token Validity
    MaaSAPI-->>Auth: Token Status + Tier Info
    Auth-->>Gateway: Authentication Result
    Gateway->>Model: Forward Request (if valid)
    Model-->>Gateway: Response
    Gateway-->>Client: Response
```

### 2. Model Inference Flow

```mermaid
sequenceDiagram
    participant Client
    participant Gateway
    participant MaaSAPI
    participant KServe
    participant Model
    
    Client->>Gateway: POST /v1/models/{model}/infer
    Gateway->>MaaSAPI: Validate Request
    MaaSAPI-->>Gateway: Tier + Rate Limit Check
    Gateway->>KServe: Forward to Model
    KServe->>Model: Process Inference
    Model-->>KServe: Inference Result
    KServe-->>Gateway: Response
    Gateway-->>Client: Response
```

## Core Components

### Gateway Layer

The gateway layer handles all incoming requests and implements security policies:

- **Gateway API**: Routes requests to appropriate services
- **Kuadrant**: Policy Attachment Point for authentication and authorization
- **Authorino**: Authentication and authorization service
- **Limitador**: Token and Request Rate limiting service

### Management Layer

The management layer contains the core business logic:

- **MaaS API**: Central service for token and tier management

### Model Layer

The model layer provides AI model serving capabilities:

- **KServe**: Model serving platform
- **Model Instances**: Individual AI models (LLMs, etc.)
- **Scaling**: Automatic scaling based on demand

## Flows

### 1. Token Request Flow

<TBD>

### 2. Model Inference Flow

<TBD>
