# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture Overview

This is a **Models as a Service (MaaS)** platform with real-time policy management built on Kubernetes, Kuadrant, Istio, Gateway API, and KServe. The platform features a modern React-based GUI for policy creation, live metrics monitoring, and request simulation.

### Core Components

1. **Frontend (`apps/frontend/`)**: React-based UI with Material-UI, drag-and-drop policy builder, real-time metrics dashboard, and request simulator
2. **Backend (`apps/backend/`)**: Node.js/Express API server with Kuadrant integration for policy management and metrics collection  
3. **QoS Prioritizer (`apps/qos-prioritizer/`)**: Quality of Service prioritization service for model request handling
4. **Key Manager (`key-manager/`)**: Go-based service for API key management and team-based access control
5. **Deployment (`deployment/`)**: Kubernetes manifests and scripts for infrastructure deployment

### Technology Stack

- **Frontend**: React 18, TypeScript, Material-UI, React Router, Socket.io, Recharts
- **Backend**: Node.js, Express, TypeScript, Socket.io, Winston logging, Axios
- **QoS Service**: Node.js, Express, TypeScript, p-queue for request prioritization
- **Key Manager**: Go 1.24+, Kubernetes client-go, Gin HTTP framework
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

**Key Manager:**
```bash
cd key-manager
make build       # Build binary
make test        # Run Go tests with coverage
make lint        # Run formatting and vet checks
make run         # Run locally
make build-image REPO=your-repo TAG=your-tag  # Build container
```

## Infrastructure Deployment

### Prerequisites
- Kubernetes cluster (1.25+) with kubectl access
- Node.js 18+ and npm
- Docker (for local development)

### Core Infrastructure
```bash
cd deployment/core-infrastructure
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
- Backend serves REST APIs at `/api/v1/`
- Real-time metrics via Socket.io connections
- Policy CRUD operations with Kuadrant integration
- Authentication via API keys managed by Key Manager

### Frontend Architecture
- Component-based with hooks for API integration
- Real-time updates via custom `useLiveRequests` hook
- Policy builder uses @dnd-kit for drag-and-drop functionality
- Material-UI theming and responsive design

### Policy Management
- AuthPolicy and RateLimitPolicy types supported
- Team-based access control with configurable rate limits
- Real-time policy enforcement monitoring
- Policy simulation and testing capabilities

### Kubernetes Integration
- Gateway API for traffic routing
- Kuadrant for policy enforcement (Authorino + Limitador)  
- KServe for AI model serving with vLLM runtime
- Prometheus for metrics collection and monitoring

## Important File Locations

- `apps/frontend/src/components/` - React components (PolicyManager, MetricsDashboard, etc.)
- `apps/frontend/src/hooks/` - Custom React hooks for API integration
- `apps/backend/src/routes/` - Express API route handlers
- `apps/backend/src/services/` - Kuadrant integration services
- `key-manager/internal/` - Go service internal packages
- `deployment/core-infrastructure/kustomize-templates/` - Kubernetes resource templates

## Monitoring and Troubleshooting

### Service URLs (when running locally)
- Frontend: http://localhost:3000
- Backend API: http://localhost:3001 (or auto-detected port)
- QoS Service: http://localhost:3005
- Health Check: http://localhost:3001/health

### Log Files
```bash
tail -f backend.log        # Backend API logs
tail -f frontend.log       # Frontend build logs  
tail -f qos-prioritizer.log # QoS service logs
```

### Common Port Issues
```bash
# Kill processes on development ports
lsof -ti:3000 | xargs kill -9  # Frontend
lsof -ti:3001 | xargs kill -9  # Backend
lsof -ti:3005 | xargs kill -9  # QoS service
```

### Kubernetes Debugging
```bash
kubectl get pods -n kuadrant-system    # Check Kuadrant status
kubectl get pods -n kserve-system      # Check KServe status
kubectl logs -f deployment/key-manager -n platform-services  # Key manager logs
```