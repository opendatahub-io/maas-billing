# 🚀 MaaS Platform Demo Guide
*Models as a Service with Real-time Policy Enforcement & Live Metrics Dashboard*

## 🎯 Demo Overview

This demonstration showcases a **complete Models-as-a-Service platform** that provides:

- **🎨 Intuitive Drag-and-Drop Policy Management** - Visual policy creation without YAML
- **📊 Real-time Metrics Dashboard** - Live monitoring of policy enforcement decisions  
- **🧪 Interactive Request Simulator** - Test policies before deployment with authentication failure testing
- **⚡ Cloud-native Policy Enforcement** - Powered by Kuadrant (Authorino + Limitador) on Kubernetes
- **🔐 Multi-tier Authentication** - API key-based auth with tiered access control
- **📈 Prometheus Integration** - Real metrics from actual policy enforcement

---

## 🏗️ What This Demo Shows

### 1. **Real-time Policy Enforcement Dashboard**
![Live Metrics](https://img.shields.io/badge/Status-Live%20Metrics-success)

**Location**: `http://localhost:3000/` (Live Metrics tab)

**Features**:
- ✅ **Live request tracking** from actual Envoy access logs
- ✅ **Policy decision breakdown** (Accept/Reject with reasons)
- ✅ **Authentication failure monitoring** 
- ✅ **Rate limiting enforcement tracking**
- ✅ **Real-time counters** updating every 2-5 seconds
- ✅ **Filtering and search** by team, model, decision type
- ✅ **Detailed request inspection** with policy decision trails

**Data Sources**:
- **Kuadrant Prometheus metrics** (Limitador + Authorino)
- **Envoy access logs** via kubectl integration
- **Real policy enforcement data** (not mock data)

### 2. **Drag-and-Drop Request Simulator**
![Request Simulator](https://img.shields.io/badge/Feature-Drag%20%26%20Drop-blue)

**Location**: `http://localhost:3000/simulator`

**Features**:
- 🎨 **Visual drag-and-drop interface** - No more dropdown menus!
- 🔧 **Tier selection with icons**:
  - 🟢 **Free Tier** - Basic access with limits
  - 🟣 **Premium Tier** - Enhanced access (changed from orange to purple)
  - 🔴 **No Auth** - Empty credentials for testing auth failures
- 🧪 **Real API testing** - Makes actual requests to Kuadrant-protected endpoints
- 📊 **Detailed result tracking** - See policy decisions with icons and colors
- ⚙️ **Configurable parameters** - Request count, tokens, custom queries

**How to Use**:
1. **Drag models** from left panel to "Model" drop zone
2. **Drag billing tiers** from left panel to "Billing Tier" drop zone
3. **Configure** request count, max tokens, and query text
4. **Run simulation** to see real policy enforcement in action

### 3. **Policy Management Interface** 
![Policy Builder](https://img.shields.io/badge/Feature-Visual%20Policy%20Builder-orange)

**Location**: `http://localhost:3000/policies`

**Features**:
- 🎯 **Visual policy creation** with drag-and-drop
- 👥 **Team-based access control**
- 🤖 **Model-specific policies** 
- ⏰ **Time-based restrictions**
- 🚦 **Rate limiting configuration**
- 📋 **Policy templates** and validation

---

## 🎮 Demo Scenarios

### Scenario 1: Authentication Failure Testing

**Steps**:
1. Go to **Request Simulator** (`/simulator`)
2. Drag **"No Auth (Test Failure)"** tier to the drop zone
3. Drag **"vLLM Simulator"** model to the drop zone
4. Enter query: `"Test authentication failure"`
5. Click **"Run Simulation"**

**Expected Results**:
- ❌ **401 Unauthorized** response
- 🔴 **Auth failure** shown in results table with red error icon
- 📊 **Live metrics dashboard** shows rejected request
- 📈 **Real-time counter** increments for "Requests Rejected"

### Scenario 2: Rate Limiting in Action

**Steps**:
1. Use **Free Tier** (🟢) credentials
2. Set **Request Count** to `10` 
3. Run simulation
4. Watch requests succeed then get rate-limited

**Expected Results**:
- ✅ **First 5 requests**: HTTP 200 (success)
- ⚠️ **Requests 6-10**: HTTP 429 (rate limited)  
- 📊 **Live dashboard** shows policy enforcement decisions
- 🎯 **Rate limiting policy** shows as "deny" in policy decisions

### Scenario 3: Premium Tier Access

**Steps**:
1. Use **Premium Tier** (🟣) credentials
2. Set **Request Count** to `25`
3. Run simulation

**Expected Results**:
- ✅ **First 20 requests**: HTTP 200 (success)
- ⚠️ **Requests 21-25**: HTTP 429 (rate limited)
- 💜 **Premium tier** shown with purple color and icons
- 📈 **Higher rate limits** compared to free tier

### Scenario 4: Real-time Monitoring

**Steps**:
1. Open **Live Metrics Dashboard** (`/`)
2. Run simulations from another tab
3. Watch **real-time updates**

**Expected Results**:
- 🔄 **Counters update** every 2-5 seconds
- 📊 **Request details** appear in table immediately
- 🎯 **Policy decisions** shown with detailed reasoning
- 🔍 **Filtering works** by decision type, policy type, source

---

## 🏁 Quick Demo Setup

### Prerequisites
- ✅ **Kubernetes cluster** with Kuadrant deployed
- ✅ **kubectl** configured and connected
- ✅ **Node.js 18+** for frontend/backend

### 1. Deploy Kuadrant Infrastructure

```bash
# Clone repository
git clone https://github.com/redhat-et/maas-billing.git
cd maas-billing/deployment/kuadrant

# Quick setup with simulator (no GPU required)
./install.sh --simulator

# Set up local domains
./setup-local-domains.sh
```

### 2. Start the MaaS Platform

```bash
# From repository root
./start-dev.sh
```

**This will**:
- 🚀 Start backend API on `http://localhost:3002`
- 🎨 Start frontend UI on `http://localhost:3000`
- 📊 Set up port forwarding for Kuadrant components
- 📈 Begin collecting real metrics

### 3. Access the Demo

- **🌐 Main Dashboard**: http://localhost:3000
- **🧪 Request Simulator**: http://localhost:3000/simulator  
- **📋 Policy Manager**: http://localhost:3000/policies

---

## 🔧 Technical Architecture

### Data Flow
```
[Frontend React App] ←→ [Backend Node.js API] ←→ [Kuadrant Components]
                                                      ↓
[Request Simulator] → [Kuadrant Gateway] → [Model Serving (KServe)]
                           ↓
[Policy Enforcement: Authorino + Limitador] → [Prometheus Metrics]
                           ↓
[Live Metrics Dashboard] ← [Real-time Data Collection]
```

### Components

**Frontend** (React + Material-UI):
- `MetricsDashboard.tsx` - Real-time policy enforcement monitoring
- `RequestSimulator.tsx` - Drag-and-drop simulation interface  
- `PolicyBuilder.tsx` - Visual policy creation
- `useApi.ts` - Real-time data hooks with auto-refresh

**Backend** (Node.js + Express):
- `metricsService.ts` - Kuadrant metrics integration
- `routes/metrics.ts` - Live dashboard API with debug logging
- `routes/simulator.ts` - Request simulation proxy

**Infrastructure** (Kubernetes + Kuadrant):
- **Istio Gateway** - Traffic management and routing
- **Authorino** - Authentication and authorization
- **Limitador** - Rate limiting with Redis backend  
- **KServe** - Model serving platform
- **Prometheus** - Metrics collection and storage

### Key Features Implemented

1. **Real-time Metrics Collection**:
   - Direct integration with Kuadrant Prometheus endpoints
   - Envoy access log parsing via kubectl
   - Live data refresh every 2-5 seconds
   - Caching and deduplication for performance

2. **Drag-and-Drop Interface**:
   - HTML5 drag-and-drop API integration
   - Visual drop zones with hover effects
   - Consistent icon and color theming
   - Real-time form validation

3. **Policy Enforcement Testing**:
   - Three-tier authentication system (Free/Premium/None)
   - Configurable rate limiting (5/20 requests per 2min)
   - Authentication failure simulation
   - Real API endpoint testing

4. **Visual Design System**:
   - 🟢 **Free Tier**: Green with success styling
   - 🟣 **Premium Tier**: Purple with secondary styling  
   - 🔴 **No Auth**: Red with error styling
   - Consistent icons and color coding throughout

---

## 📊 Metrics & Monitoring

### Available Metrics

**Dashboard Statistics**:
- **Total Requests** - All requests processed
- **Requests Approved** - Successful policy enforcement
- **Requests Rejected** - Blocked by policies
- **Success Rate** - Percentage approved

**Policy Breakdown**:
- **Authentication Failures** - 401/403 responses
- **Rate Limiting** - 429 responses  
- **Policy Decisions** - Accept/Deny with reasons

**Real-time Request Details**:
- Request timestamp and source
- Team/tier identification
- Model and endpoint details
- Policy decision trail
- Response time and token usage

### Data Sources

1. **Kuadrant Prometheus**:
   - `limitador-limitador:8080/metrics` - Rate limiting data
   - `authorino-controller-metrics:8080/metrics` - Auth data

2. **Envoy Access Logs**:
   - Real request/response data via kubectl
   - Policy decision inference from response codes
   - Request timing and routing information

3. **Live API Testing**:
   - Request simulator generates real traffic
   - Policy enforcement results tracked
   - Authentication and rate limiting validation

---

## 🚀 Deployment Options

### Development (Local)
```bash
# Start local development environment
./start-dev.sh

# Access at:
# Frontend: http://localhost:3000
# Backend:  http://localhost:3002
```

### Kubernetes Cluster
```bash
# Deploy Kuadrant infrastructure
cd deployment/kuadrant
./install.sh --simulator  # or --qwen3 for GPU

# Deploy MaaS platform
./start-dev.sh
```

### Production Considerations

**Scaling**:
- Frontend: Static build deployment
- Backend: Horizontal pod autoscaling
- Kuadrant: Multi-replica Limitador/Authorino

**Security**:
- TLS/HTTPS for all endpoints
- API key rotation and management
- Network policies and RBAC

**Monitoring**:
- Prometheus/Grafana deployment
- Alert rules for policy violations
- Log aggregation and analysis

---

## 🎯 Demo Key Points

### For Technical Audiences
- **Cloud-native architecture** with Kubernetes and Istio
- **Real-time policy enforcement** without application changes
- **Prometheus metrics integration** for observability
- **Gateway API standards** for traffic management

### For Business Audiences  
- **Visual policy management** reduces operational complexity
- **Real-time monitoring** enables proactive issue resolution
- **Multi-tier access** supports different customer segments
- **Request simulation** reduces production incidents

### For Product Audiences
- **Intuitive drag-and-drop interface** improves user experience
- **Real-time feedback** enables rapid policy iteration
- **Visual design system** ensures consistent user interaction
- **Comprehensive testing** reduces deployment risks

---

## 🔗 Resources

- **GitHub Repository**: https://github.com/redhat-et/maas-billing
- **Kuadrant Documentation**: https://kuadrant.io/
- **KServe Documentation**: https://kserve.github.io/website/
- **Demo Video**: [Coming Soon]

---

## 📝 Quick Reference

### Important URLs
- **Dashboard**: http://localhost:3000
- **Simulator**: http://localhost:3000/simulator
- **Policies**: http://localhost:3000/policies
- **Backend API**: http://localhost:3002
- **Health Check**: http://localhost:3002/health

### API Keys for Testing
| Tier | API Key | Rate Limit |
|------|---------|------------|
| Free | `freeuser1_key` | 5 req/2min |
| Premium | `premiumuser1_key` | 20 req/2min |
| None | `""` (empty) | Auth failure |

### Common Commands
```bash
# Start demo
./start-dev.sh

# Stop demo  
./stop-dev.sh

# Check Kuadrant status
kubectl get pods -n kuadrant-system

# View live metrics
curl http://localhost:3002/api/v1/metrics/dashboard

# Test authentication failure
curl -H "Authorization: APIKEY " \
     http://simulator.maas.local:8000/v1/chat/completions
```

---

*This demo showcases the future of cloud-native API management with visual policy creation, real-time enforcement monitoring, and comprehensive testing capabilities.*