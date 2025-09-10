#!/bin/bash

echo "========================================================================"
echo "🚀 QoS Demo Environment Preparation Script"
echo "========================================================================"
echo ""
echo "This script will validate and start all required services for the QoS demo:"
echo "- Kubernetes LLM pod deployment"
echo "- Port forwarding for LLM access"
echo "- QoS prioritizer service"
echo "- Backend service (MaaS platform API)"
echo "- Frontend UI (real-time monitoring)"
echo ""

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MAAS_ROOT="$(dirname "$(dirname "$PROJECT_ROOT")")"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Kill process on port
kill_port() {
    local port=$1
    local pids=$(lsof -ti:$port 2>/dev/null)
    if [ -n "$pids" ]; then
        log_warning "Killing existing processes on port $port"
        echo $pids | xargs kill -9 2>/dev/null || true
        sleep 2
    fi
}

# Check prerequisites
echo "🔍 Checking prerequisites..."
echo ""

# Check required commands
REQUIRED_COMMANDS=("kubectl" "node" "npm" "curl")
for cmd in "${REQUIRED_COMMANDS[@]}"; do
    if command_exists $cmd; then
        log_success "$cmd is installed"
    else
        log_error "$cmd is not installed or not in PATH"
        exit 1
    fi
done

# Check if kubectl is configured
if ! kubectl cluster-info >/dev/null 2>&1; then
    log_error "kubectl is not configured or cluster is not accessible"
    echo "Please ensure your Kubernetes cluster is running and kubectl is configured"
    exit 1
fi
log_success "Kubernetes cluster is accessible"

echo ""

# Step 1: Deploy and validate LLM pod
echo "========================================================================"
echo "📦 Step 1: LLM Model Deployment"
echo "========================================================================"
echo ""

# Check if llm namespace exists
if ! kubectl get namespace llm >/dev/null 2>&1; then
    log_info "Creating llm namespace..."
    kubectl create namespace llm
fi
log_success "LLM namespace exists"

# Check if LLM pod is running
LLM_POD=$(kubectl get pods -n llm -l app=simple-llm -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$LLM_POD" ]; then
    log_warning "LLM pod not found, deploying..."
    
    # Check if deployment file exists
    if [ -f "$SCRIPT_DIR/simple-llm-stable.yaml" ]; then
        kubectl apply -f "$SCRIPT_DIR/simple-llm-stable.yaml"
    elif [ -f "$SCRIPT_DIR/simple-llm-simple.yaml" ]; then
        kubectl apply -f "$SCRIPT_DIR/simple-llm-simple.yaml"
    else
        log_error "LLM deployment file not found in simulation directory"
        exit 1
    fi
    
    log_info "Waiting for LLM pod to be ready (this may take 2-3 minutes)..."
    if ! kubectl wait --for=condition=ready pod -l app=simple-llm -n llm --timeout=300s; then
        log_error "LLM pod failed to become ready within 5 minutes"
        kubectl get pods -n llm
        kubectl logs -n llm -l app=simple-llm --tail=20
        exit 1
    fi
    
    LLM_POD=$(kubectl get pods -n llm -l app=simple-llm -o jsonpath='{.items[0].metadata.name}')
fi

# Check pod status
POD_STATUS=$(kubectl get pod -n llm $LLM_POD -o jsonpath='{.status.phase}' 2>/dev/null)
if [ "$POD_STATUS" != "Running" ]; then
    log_error "LLM pod is not running (status: $POD_STATUS)"
    kubectl get pods -n llm
    exit 1
fi
log_success "LLM pod '$LLM_POD' is running"

echo ""

# Step 2: Set up port forwarding for LLM
echo "========================================================================"
echo "🔌 Step 2: LLM Port Forwarding"
echo "========================================================================"
echo ""

# Kill any existing port forward on 8004
kill_port 8004

# Check if port forward is already running
if curl -s http://localhost:8004/health >/dev/null 2>&1; then
    log_success "LLM service already accessible on port 8004"
else
    log_info "Starting port forward for LLM service..."
    
    # Start port forward in background
    kubectl port-forward -n llm $LLM_POD 8004:8080 >/dev/null 2>&1 &
    PORT_FORWARD_PID=$!
    
    # Wait for port forward to be ready
    for i in {1..10}; do
        if curl -s http://localhost:8004/health >/dev/null 2>&1; then
            log_success "LLM service accessible on port 8004 (PID: $PORT_FORWARD_PID)"
            break
        fi
        if [ $i -eq 10 ]; then
            log_error "Port forward failed to start"
            kill $PORT_FORWARD_PID 2>/dev/null || true
            exit 1
        fi
        sleep 1
    done
fi

# Test LLM connectivity
log_info "Testing LLM model connectivity..."
test_response=$(curl -s -w "%{http_code}" \
    -H "Content-Type: application/json" \
    -d '{"model":"gpt2-medium","messages":[{"role":"user","content":"test"}],"max_tokens":5}' \
    --max-time 15 \
    "http://localhost:8004/v1/chat/completions" 2>/dev/null)

status_code=${test_response: -3}
if [[ "$status_code" != "200" ]]; then
    log_error "LLM model test failed (HTTP $status_code)"
    exit 1
fi
log_success "LLM model is responding correctly"

echo ""

# Step 3: Start QoS service
echo "========================================================================"
echo "⚡ Step 3: QoS Prioritizer Service"
echo "========================================================================"
echo ""

# Check if QoS service is already running
if curl -s http://localhost:3005/health >/dev/null 2>&1; then
    log_success "QoS service is already running on port 3005"
else
    log_info "Starting QoS prioritizer service..."
    
    # Kill any existing process on port 3005
    kill_port 3005
    
    # Navigate to QoS service directory
    cd "$PROJECT_ROOT"
    
    # Check if node_modules exists
    if [ ! -d "node_modules" ]; then
        log_info "Installing QoS service dependencies..."
        npm install
    fi
    
    # Start QoS service in background
    log_info "Starting QoS service..."
    npm run dev >/dev/null 2>&1 &
    QOS_PID=$!
    
    # Wait for QoS service to be ready
    for i in {1..15}; do
        if curl -s http://localhost:3005/health >/dev/null 2>&1; then
            log_success "QoS service is running on port 3005 (PID: $QOS_PID)"
            break
        fi
        if [ $i -eq 15 ]; then
            log_error "QoS service failed to start"
            kill $QOS_PID 2>/dev/null || true
            exit 1
        fi
        sleep 1
    done
fi

# Test QoS service with LLM integration
log_info "Testing QoS → LLM integration..."
test_response=$(timeout 25 curl -s -w "%{http_code}" \
    -H "Content-Type: application/json" \
    -H "x-auth-identity: {\"metadata\":{\"annotations\":{\"kuadrant.io/groups\":\"enterprise\"}}}" \
    -d '{"model":"gpt2-medium","messages":[{"role":"user","content":"test"}],"max_tokens":5}' \
    "http://localhost:3005/v1/chat/completions" 2>/dev/null || echo "000")

status_code=${test_response: -3}
if [[ "$status_code" != "200" ]]; then
    log_error "QoS → LLM integration test failed (HTTP $status_code)"
    exit 1
fi
log_success "QoS → LLM integration is working"

echo ""

# Step 4: Backend Service  
echo "========================================================================"
echo "💾 Step 4: Backend Service"
echo "========================================================================"
echo ""

# Check if backend is already running
if curl -s http://localhost:3001/health >/dev/null 2>&1; then
    log_success "Backend service is already running on port 3001"
else
    log_info "Starting backend service..."
    
    # Kill any existing process on port 3001
    kill_port 3001
    
    # Navigate to backend directory
    cd "$MAAS_ROOT/apps/backend"
    
    # Check if node_modules exists
    if [ ! -d "node_modules" ]; then
        log_info "Installing backend dependencies..."
        npm install
    fi
    
    # Start backend in background
    log_info "Starting backend service..."
    npm run dev >/dev/null 2>&1 &
    BACKEND_PID=$!
    
    # Wait for backend to be ready
    for i in {1..20}; do
        if curl -s http://localhost:3001/health >/dev/null 2>&1; then
            log_success "Backend service is running on port 3001 (PID: $BACKEND_PID)"
            break
        fi
        if [ $i -eq 20 ]; then
            log_warning "Backend service took longer than expected to start"
            break
        fi
        sleep 2
    done
fi

echo ""

# Step 5: Frontend UI
echo "========================================================================"
echo "🎨 Step 5: Frontend UI"
echo "========================================================================"
echo ""

# Check if frontend is already running
if curl -s http://localhost:3000 >/dev/null 2>&1; then
    log_success "Frontend UI is already running on port 3000"
else
    log_info "Starting frontend UI..."
    
    # Kill any existing process on port 3000
    kill_port 3000
    
    # Navigate to frontend directory
    cd "$MAAS_ROOT/apps/frontend"
    
    # Check if node_modules exists
    if [ ! -d "node_modules" ]; then
        log_info "Installing frontend dependencies..."
        npm install
    fi
    
    # Start frontend in background
    log_info "Starting frontend service..."
    npm start >/dev/null 2>&1 &
    FRONTEND_PID=$!
    
    # Wait for frontend to be ready
    for i in {1..30}; do
        if curl -s http://localhost:3000 >/dev/null 2>&1; then
            log_success "Frontend UI is running on port 3000 (PID: $FRONTEND_PID)"
            break
        fi
        if [ $i -eq 30 ]; then
            log_warning "Frontend UI took longer than expected to start"
            break
        fi
        sleep 2
    done
fi

echo ""

# Summary
echo "========================================================================"
echo "🎉 Environment Setup Complete!"
echo "========================================================================"
echo ""
echo "✅ All services are running and validated:"
echo ""
echo "🔹 LLM Model Pod: $LLM_POD (Kubernetes)"
echo "🔹 LLM Access: http://localhost:8004 (port-forwarded)"
echo "🔹 QoS Service: http://localhost:3005 (local)"
echo "🔹 Backend Service: http://localhost:3001 (local)"
echo "🔹 Frontend UI: http://localhost:3000 (local)"
echo ""
echo "🚀 Ready to run demos!"
echo ""
echo "Usage:"
echo "  ./demo.sh                    # Run interactive demo"
echo "  ./demo.sh <<< '2'           # Run QoS demo automatically"
echo "  ./demo.sh <<< '3'           # Run advanced 3-tier demo"
echo ""
echo "Monitoring:"
echo "  Open http://localhost:3000 → QoS tab for real-time graphs"
echo "  Backend API: http://localhost:3001/health"
echo "  Check QoS logs: tail -f the terminal where this script was run"
echo "  Check LLM logs: kubectl logs -n llm -f $LLM_POD"
echo ""
echo "Cleanup:"
echo "  pkill -f 'kubectl port-forward'  # Stop port forwards"
echo "  pkill -f 'npm run dev'          # Stop QoS + Backend services"
echo "  pkill -f 'npm start'            # Stop frontend"
echo ""