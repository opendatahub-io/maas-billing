#!/bin/bash

# Stop MaaS Platform Development Environment
echo "🛑 Stopping MaaS Platform Development Environment..."

# Read PIDs from files if they exist
if [ -f ".qos.pid" ]; then
    QOS_PID=$(cat .qos.pid)
    if kill -0 $QOS_PID 2>/dev/null; then
        echo "⚡ Stopping QoS prioritizer (PID: $QOS_PID)..."
        kill $QOS_PID
    fi
    rm .qos.pid
fi

if [ -f ".backend.pid" ]; then
    BACKEND_PID=$(cat .backend.pid)
    if kill -0 $BACKEND_PID 2>/dev/null; then
        echo "🔧 Stopping backend server (PID: $BACKEND_PID)..."
        kill $BACKEND_PID
    fi
    rm .backend.pid
fi

if [ -f ".frontend.pid" ]; then
    FRONTEND_PID=$(cat .frontend.pid)
    if kill -0 $FRONTEND_PID 2>/dev/null; then
        echo "🎨 Stopping frontend server (PID: $FRONTEND_PID)..."
        kill $FRONTEND_PID
    fi
    rm .frontend.pid
fi

# Kill any remaining processes on ports 3000, 3001, 3002, and 3005
echo "🧹 Cleaning up any remaining processes..."
pkill -f "npm start" 2>/dev/null || true
pkill -f "npm run dev" 2>/dev/null || true
lsof -ti:3000 | xargs kill -9 2>/dev/null || true
lsof -ti:3001 | xargs kill -9 2>/dev/null || true
lsof -ti:3002 | xargs kill -9 2>/dev/null || true
lsof -ti:3005 | xargs kill -9 2>/dev/null || true

echo "✅ MaaS Platform development environment stopped"