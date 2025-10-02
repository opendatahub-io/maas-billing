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

# Check if Kuadrant is deployed
if ! kubectl get pods -n kuadrant-system >/dev/null 2>&1; then
    echo "❌ Kuadrant is not deployed. Please run the Kuadrant deployment first:"
    echo "   cd deployment/kuadrant && ./install.sh"
    exit 1
fi

# Check required ports
if ! check_port 3000; then
    echo "   Port 3000 (frontend) is required. Please stop the process using port 3000."
    echo "   Run: lsof -ti:3000 | xargs kill -9"
    exit 1
fi

if ! check_port 3001; then
    echo "   Port 3001 (backend) is required. Please stop the process using port 3001."
    echo "   Run: lsof -ti:3001 | xargs kill -9"
    exit 1
fi

if ! check_port 3005; then
    echo "   Port 3005 (QoS service) is required. Please stop the process using port 3005."
    echo "   Run: lsof -ti:3005 | xargs kill -9"
    exit 1
fi

# Start QoS prioritizer service in background
echo "⚡ Starting QoS prioritizer service..."
cd apps/qos-prioritizer
if [ ! -d "node_modules" ]; then
    echo "📦 Installing QoS prioritizer dependencies..."
    npm install --silent
fi
npm run dev > ../../qos-prioritizer.log 2>&1 &
QOS_PID=$!
cd ../..

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

# Backend will use port 3001 (configured in .env)
BACKEND_PORT=3001

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
if kill -0 $QOS_PID 2>/dev/null; then
    echo "✅ QoS prioritizer started (PID: $QOS_PID)"
    echo "   QoS Service: http://localhost:3005"
else
    echo "❌ QoS prioritizer failed to start"
fi

if kill -0 $BACKEND_PID 2>/dev/null; then
    echo "✅ Backend server started (PID: $BACKEND_PID)"
    echo "   Backend API: http://localhost:$BACKEND_PORT"
    echo "   API Health: http://localhost:$BACKEND_PORT/health"
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
echo "   🔧 Backend API: http://localhost:$BACKEND_PORT"
echo "   ⚡ QoS Service: http://localhost:3005"
echo "   📈 Metrics: http://localhost:$BACKEND_PORT/api/v1/metrics/live-requests"
echo ""
echo "📁 Logs:"
echo "   QoS Prioritizer: tail -f qos-prioritizer.log"
echo "   Backend: tail -f backend.log"
echo "   Frontend: tail -f frontend.log"
echo ""
echo "🛑 To stop the servers:"
echo "   kill $QOS_PID $BACKEND_PID $FRONTEND_PID"
echo ""

# Save PIDs to file for easy cleanup
echo "$QOS_PID" > .qos.pid
echo "$BACKEND_PID" > .backend.pid
echo "$FRONTEND_PID" > .frontend.pid

echo "Press Ctrl+C to stop monitoring..."

# Monitor the processes
while true; do
    if ! kill -0 $QOS_PID 2>/dev/null; then
        echo "❌ QoS prioritizer stopped unexpectedly"
        break
    fi
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