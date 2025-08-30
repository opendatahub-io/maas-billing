#!/bin/bash

# MaaS Platform Development Environment
echo "🚀 Starting MaaS Platform Development Environment..."

# Function to check if a port is in use
check_port() {
    if lsof -i :$1 >/dev/null 2>&1; then
        echo "⚠️  Port $1 is already in use"
        return 1
    fi
    return 0
}

# Check prerequisites
echo "🔍 Checking prerequisites..."

# Check if connected to OpenShift cluster
if ! oc whoami >/dev/null 2>&1; then
    echo "❌ Not connected to OpenShift cluster. Please login first:"
    echo "   oc login <cluster-url>"
    exit 1
fi

echo "✅ Connected to OpenShift cluster as: $(oc whoami)"

# Check ports
if ! check_port 3000; then
    echo "   Please stop the process using port 3000 or use a different port"
fi

if ! check_port 3001; then
    echo "   Please stop the process using port 3001 or use a different port"
fi

# Start backend in background
echo "🔧 Starting backend server..."
cd apps/backend
if [ ! -d "node_modules" ]; then
    echo "📦 Installing backend dependencies..."
    npm install --silent
fi
npm run dev > ../../backend.log 2>&1 &
BACKEND_PID=$!
cd ../..

# Wait a moment for backend to start
sleep 3

# Start frontend in background
echo "🎨 Starting frontend server..."
cd apps/frontend
if [ ! -d "node_modules" ]; then
    echo "📦 Installing frontend dependencies..."
    npm install --silent
fi
npm start > ../../frontend.log 2>&1 &
FRONTEND_PID=$!
cd ../..

# Wait for servers to start
echo "⏳ Waiting for servers to start..."
sleep 5

# Check if servers are running
if kill -0 $BACKEND_PID 2>/dev/null; then
    echo "✅ Backend server started (PID: $BACKEND_PID)"
    echo "   Backend API: http://localhost:3001"
    echo "   API Health: http://localhost:3001/health"
else
    echo "❌ Backend server failed to start"
fi

if kill -0 $FRONTEND_PID 2>/dev/null; then
    echo "✅ Frontend server started (PID: $FRONTEND_PID)"
    echo "   Frontend UI: http://localhost:3000"
else
    echo "❌ Frontend server failed to start"
fi

echo ""
echo "📊 MaaS Platform is ready!"
echo "   🌐 Frontend: http://localhost:3000"
echo "   🔧 Backend API: http://localhost:3001"
echo "   📈 Metrics: http://localhost:3001/api/v1/metrics/live-requests"
echo ""
echo "📁 Logs:"
echo "   Backend: tail -f backend.log"
echo "   Frontend: tail -f frontend.log"
echo ""
echo "🛑 To stop the servers:"
echo "   kill $BACKEND_PID $FRONTEND_PID"
echo ""

# Save PIDs to file for easy cleanup
echo "$BACKEND_PID" > .backend.pid
echo "$FRONTEND_PID" > .frontend.pid

echo "Press Ctrl+C to stop monitoring..."

# Monitor the processes
while true; do
    if ! kill -0 $BACKEND_PID 2>/dev/null; then
        echo "❌ Backend server stopped unexpectedly"
        break
    fi
    if ! kill -0 $FRONTEND_PID 2>/dev/null; then
        echo "❌ Frontend server stopped unexpectedly"
        break
    fi
    sleep 5
done