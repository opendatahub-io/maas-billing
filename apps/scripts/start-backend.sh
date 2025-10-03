#!/bin/bash

# MaaS Backend Development Server
echo "🚀 Starting MaaS Backend Development Server..."

# Change to backend directory
cd apps/backend

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "📦 Installing backend dependencies..."
    npm install
fi

# Start the development server
echo "🔧 Starting backend server on http://localhost:3001"
npm run dev