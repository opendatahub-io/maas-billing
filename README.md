# MaaS Platform - Models as a Service with Policy Management

Our goal is to create a comprehensive platform for **Models as a Service** with real-time policy management.

> [!IMPORTANT]
> This project is a work in progress and is not yet ready for production.

## 📦 Technology Stack

- **Kuadrant/Authorino/Limitador**: API gateway and policy engine
- **Gateway API**: Traffic routing and management (OpenShift native implementation)
- **OpenShift Service Mesh**: Automatically provisioned when Gateway API is enabled (includes Istio)
- **React**: Frontend framework
- **Go**: Backend frameworks

## 🚀 Features

- **🎯 Policy Management**: Drag-and-drop interface for creating and managing authentication and rate-limiting policies
- **📊 Real-time Metrics**: Live dashboard showing policy enforcement decisions with filtering and analytics
- **🧪 Request Simulation**: Test policies before deployment with comprehensive simulation tools
- **🔐 Authentication**: API key-based auth with team-based access control
- **⚡ Rate Limiting**: Configurable request quotas with time-based restrictions
- **📈 Observability**: Prometheus metrics and real-time monitoring
- **🌐 Domain Routing**: Model-specific subdomains for clean API organization

## 🏗️ Architecture

### Backend Components
- **API Gateway**: OpenShift Gateway API implementation with Envoy proxy and Kuadrant integration
- **Policy Engine**: Real-time policy enforcement through Kuadrant (Authorino + Limitador)
- **Model Serving**: KServe-based AI model deployment with vLLM runtime
- **Model Discovery**: Automatic model listing model resources
- **Key Manager**: API key management and authentication
- **Metrics Collection**: Live data from Kuadrant components

### Frontend Components  
- **Policy Manager**: Create, edit, and manage policies with intuitive drag-and-drop interface
- **Live Metrics Dashboard**: Real-time view of policy enforcement with filtering capabilities
- **Request Simulator**: Test policies against simulated traffic patterns

## 📋 Prerequisites

- **Kubernetes cluster** (1.25+) with kubectl access
- **Node.js** (18+) and npm
- **Docker** (for local development)

## 🚀 Quick Start

### Deploy Infrastructure

See the comprehensive [Deployment Guide](deployment/README.md) for detailed instructions.

Quick deployment for OpenShift:
```bash
export CLUSTER_DOMAIN="apps.your-openshift-cluster.com"
kustomize build deployment/overlays/openshift | envsubst | kubectl apply -f -
```

Quick deployment for Kubernetes:
```bash
export CLUSTER_DOMAIN="your-kubernetes-domain.com"
kustomize build deployment/overlays/kubernetes | envsubst | kubectl apply -f -
```

### Start Development Environment

After deploying the infrastructure:

#### Option A: One-Command Start (Recommended)
```bash
# From the repository root
./start-dev.sh
```

This will:
- Check prerequisites (infrastructure deployment)
- Start backend API server on http://localhost:3001
- Start frontend UI on http://localhost:3000
- Provide monitoring and logging

#### Option B: Manual Start
```bash
# Terminal 1: Start Backend
./start-backend.sh

# Terminal 2: Start Frontend  
./start-frontend.sh
```

## Access the Platform

- **Frontend UI**: http://localhost:3000
- **Backend API**: http://localhost:3001
- **API Health**: http://localhost:3001/health
- **Live Metrics**: http://localhost:3001/api/v1/metrics/live-requests

## 🖥️ Using the Platform

### Policy Manager
1. Navigate to **Policy Manager** in the sidebar
2. Click **Create Policy** to open the policy builder
3. Use drag-and-drop to add teams and models
4. Configure rate limits and time restrictions
5. Save to apply policies to Kuadrant

### Live Metrics Dashboard
1. Go to **Live Metrics** to see real-time enforcement
2. Filter by decision type (Accept/Reject) or policy type
3. View detailed policy enforcement reasons
4. Monitor request patterns and policy effectiveness

### Request Simulator
1. Access **Request Simulator** to test policies
2. Select team, model, and configure request parameters
3. Run simulations to see how policies would handle traffic
4. Validate policy configurations before deployment

## 🔧 Development

### Project Structure

| Directory | Description | Documentation |
|-----------|-------------|---------------|
| `apps/frontend/` | React frontend with Material-UI | [Frontend Guide](apps/frontend/README.md) |
| `apps/backend/` | Node.js/Express API server | [Backend Guide](apps/backend/README.md) |
| `maas-api/` | Go API for key management | [MaaS API Guide](maas-api/README.md) |
| `deployment/` | Kubernetes/OpenShift deployments | [Deployment Guide](deployment/README.md) |
| `scripts/` | Automation and utility scripts | - |

### Available Scripts

From the repository root:
- `./start-dev.sh` - Start full development environment
- `./stop-dev.sh` - Stop all development services
- `./start-backend.sh` - Start backend only
- `./start-frontend.sh` - Start frontend only
- `./scripts/test-gateway.sh` - Test gateway endpoints

### Backend API Endpoints

The backend provides these key endpoints:
- `GET /api/v1/models` - List available models
- `GET /api/v1/policies` - Retrieve current policies
- `POST /api/v1/policies` - Create/update policies
- `GET /api/v1/metrics/live-requests` - Live metrics stream
- `POST /api/v1/simulator/run` - Run policy simulation

### Frontend Components

Key React components:
- `PolicyBuilder` - Drag-and-drop policy editor
- `MetricsDashboard` - Real-time metrics visualization
- `RequestSimulator` - Policy testing interface
- `TokenManagement` - API key management

## 🧪 Testing

### Test Infrastructure
```bash
# Use the test script
./scripts/test-gateway.sh

# Or manually test endpoints
curl http://localhost:3001/health
```

### Run Frontend Tests
```bash
cd apps/frontend
npm test
```

### Run Backend Tests
```bash
cd apps/backend
npm test
```

## 📚 Documentation

- [Deployment Guide](deployment/README.md) - Complete deployment instructions
- [Platform-Specific Overlays](deployment/overlays/README.md) - OpenShift vs Kubernetes
- [MaaS API Documentation](maas-api/README.md) - Go API for key management
- [OAuth Setup Guide](OAUTH_SETUP.md) - Configure OAuth authentication

## 🤝 Contributing

We welcome contributions! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## 📝 License

This project is licensed under the Apache 2.0 License.

## 🙏 Acknowledgments

Built with:
- [Kuadrant](https://kuadrant.io/) for API management
- [KServe](https://kserve.github.io/) for model serving
- [OpenShift](https://www.openshift.com/) with Service Mesh for infrastructure
- [React](https://react.dev/) and [Material-UI](https://mui.com/)

## 📞 Support

For questions or issues:
- Open an issue on GitHub
- Check the [deployment guide](deployment/README.md) for troubleshooting
- Review the [samples](deployment/samples/models/) for examples
