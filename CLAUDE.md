# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

This is a **Models as a Service (MaaS)** platform with real-time policy management built on Kubernetes, Kuadrant, Istio, Gateway API, and KServe. The platform features a modern React-based GUI for policy creation, live metrics monitoring, and request simulation.

### Core Components

1. **Frontend (`apps/frontend/`)**: React-based UI with Material-UI, drag-and-drop policy builder, real-time metrics dashboard, and request simulator
2. **Backend (`apps/backend/`)**: Node.js/Express API server with Kuadrant integration for policy management and metrics collection  
3. **QoS Prioritizer (`apps/qos-prioritizer/`)**: Quality of Service prioritization service for model request handling
4. **MaaS API (`maas-api/`)**: Go-based service for API key management and team-based access control
5. **Deployment (`deployment/`)**: Kubernetes manifests and scripts for infrastructure deployment

### Technology Stack

- **Frontend**: React 18, TypeScript, Material-UI, React Router, Socket.io, Recharts for charts, @dnd-kit for drag-and-drop
- **Backend**: Node.js, Express, TypeScript, Socket.io, Winston logging, Axios
- **QoS Service**: Node.js, Express, TypeScript, p-queue for request prioritization
- **MaaS API**: Go 1.24+, Kubernetes client-go for k8s integration
- **Infrastructure**: Kubernetes, Istio, KServe, Kuadrant (Authorino + Limitador), Prometheus

## Development Commands

### Quick Start
```bash
# Start all services (recommended)
./start-dev.sh

# Stop all services  
./stop-dev.sh
```

### Individual Services

**Frontend:**
```bash
cd apps/frontend
npm install
npm start        # Starts on http://localhost:3000
npm run build    # Production build
npm test         # Run tests
```

**Backend:**
```bash
cd apps/backend
npm install
npm run dev      # Development with hot reload
npm run build    # TypeScript compilation
npm start        # Production mode
npm test         # Run tests
npm run lint     # ESLint
npm run typecheck # TypeScript checking
```

**QoS Prioritizer:**
```bash
cd apps/qos-prioritizer  
npm install
npm run dev      # Development mode
npm run build    # TypeScript compilation
npm start        # Production mode
npm test         # Run tests
npm run lint     # ESLint
npm run typecheck # TypeScript checking
```

**MaaS API:**
```bash
cd maas-api
make deps        # Download Go dependencies
make build       # Build binary (includes deps, fmt, and test)
make test        # Run Go tests with coverage
make lint        # Run formatting and vet checks (fmt-check + vet)
make fmt         # Format Go code using gofmt
make run         # Run locally with debug flags
make build-image REPO=your-repo TAG=your-tag  # Build container
```

## Infrastructure Deployment

### Prerequisites
- Kubernetes cluster (1.25+) with kubectl access
- OpenShift CLI (`oc`) for cluster authentication
- Node.js 18+ and npm
- Docker (for local development)

### Quick Environment Setup
```bash
# Auto-generate environment files from cluster (recommended)
./create-my-env.sh

# This extracts cluster info and creates:
# - apps/backend/.env (backend configuration)
# - apps/frontend/.env.local (frontend React config)
```

### Core Infrastructure
```bash
cd deployment/infrastructure
kubectl apply -k .
```

### Example Deployments
```bash
cd deployment/examples
kubectl apply -k basic-deployment/     # Basic model deployment
kubectl apply -k gpu-deployment/       # GPU-enabled deployment  
kubectl apply -k simulator-deployment/ # Simulator for testing
```

## Key Development Patterns

### API Structure
- Backend serves REST APIs at `/api/v1/` (policies, metrics, tokens, simulator)
- Real-time metrics via Socket.io connections
- Policy CRUD operations with Kuadrant integration via kubectl commands
- Authentication via API keys managed by MaaS API service
- QoS service integration for request prioritization (port 3005)

### Frontend Architecture
- Component-based with hooks for API integration
- Real-time updates via custom Socket.io integration
- Policy builder uses @dnd-kit for drag-and-drop functionality
- Material-UI theming and responsive design
- Proxy to backend configured in package.json

### Policy Management
- AuthPolicy and RateLimitPolicy types supported via KuadrantService
- Team-based access control with configurable rate limits
- Real-time policy enforcement monitoring
- Policy simulation and testing capabilities
- kubectl integration for Kubernetes policy management

### Service Communication
- Frontend (port 3000) → Backend (configured via FRONTEND_URL)
- Backend → QoS Service (configured via QOS_SERVICE_URL)
- Backend → MaaS API (Go service for token management)
- Backend → Kubernetes via kubectl/oc commands for policy management
- Real-time updates via Socket.io between all services

### Environment Configuration
**Critical**: All URLs, ports, and cluster-specific values must be configured via environment variables. No hardcoded fallbacks are allowed.

**Required Backend Environment Variables:**
- `PORT` - Backend server port (default: 3001)
- `FRONTEND_URL` - Frontend origin for CORS (default: http://localhost:3000)
- `QOS_SERVICE_URL` - QoS prioritizer service URL (default: http://localhost:3005)
- `CLUSTER_DOMAIN` - OpenShift cluster domain
- `OAUTH_URL` - OAuth service URL for authentication
- `CONSOLE_URL` - OpenShift console URL for login redirects
- `CLUSTER_API_URL` - Kubernetes API server URL
- `KEY_MANAGER_BASE_URL` - MaaS API service URL for token management
- `ADMIN_KEY` - Admin API key for MaaS API authentication
- `TIER_GROUP_CONFIGMAP_NAME` - ConfigMap containing tier-to-group mappings
- `TIER_GROUP_CONFIGMAP_NAMESPACE` - Namespace of the tier-group ConfigMap

### Kubernetes Integration
- Gateway API for traffic routing
- Kuadrant for policy enforcement (Authorino + Limitador)  
- KServe for AI model serving with vLLM runtime
- Prometheus for metrics collection and monitoring
- Direct kubectl integration in KuadrantService for policy CRUD operations

## Important File Locations

- `apps/frontend/src/components/` - React components (PolicyManager, MetricsDashboard, QoSMonitor, PolicyBuilder, RequestSimulator, AuthCallback)
- `apps/frontend/src/hooks/` - Custom React hooks for API integration
- `apps/frontend/src/services/api.ts` - API client for backend communication
- `apps/backend/src/routes/` - Express API route handlers (policies, metrics, tokens, simulator)
- `apps/backend/src/services/` - Integration services (KuadrantService, MetricsService, ModelService, MaasApiService)
- `apps/backend/src/app.ts` - Main Express application with Socket.io setup
- `apps/qos-prioritizer/src/` - QoS service implementation with p-queue
- `maas-api/internal/` - Go service internal packages for API key management
- `maas-api/cmd/` - Go service main entry point
- `deployment/infrastructure/` - Base Kubernetes infrastructure
- `deployment/examples/` - Example deployments and configurations
- `start-dev.sh` / `stop-dev.sh` - Development environment management scripts
- `create-my-env.sh` - Auto-generate environment files from cluster

## Monitoring and Troubleshooting

### Service URLs (when running locally)
- Frontend: http://localhost:3000
- Backend API: http://localhost:3001 (default port)
- QoS Service: http://localhost:3005 (default port)
- Health Check: http://localhost:3001/health

### Log Files
```bash
tail -f backend.log        # Backend API logs
tail -f frontend.log       # Frontend build logs  
tail -f qos-prioritizer.log # QoS service logs
```

### Common Port Issues
```bash
# Kill processes on development ports (adjust ports based on your environment)
lsof -ti:3000 | xargs kill -9  # Frontend (typically always 3000)
lsof -ti:3001 | xargs kill -9  # Backend (default port)
lsof -ti:3005 | xargs kill -9  # QoS Service (default port)
pkill -f "maas-backend"         # Kill backend by process name
pkill -f "qos-prioritizer"      # Kill QoS service by process name

# Or use the cleanup script
./stop-dev.sh                   # Stop all development services
```

### Kubernetes Debugging
```bash
kubectl get pods -n kuadrant-system    # Check Kuadrant status
kubectl get pods -n kserve-system      # Check KServe status
kubectl logs -f deployment/maas-api -n platform-services  # MaaS API logs
kubectl get authpolicies -A             # List authentication policies
kubectl get ratelimitpolicies -A        # List rate limit policies
kubectl get limitador -A                # List Limitador configurations
```