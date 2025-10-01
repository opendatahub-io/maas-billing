#!/bin/bash

# MaaS Frontend Development Server  
echo "🎨 Starting MaaS Frontend Development Server..."

# Change to frontend directory
cd apps/frontend

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "📦 Installing frontend dependencies..."
    npm install
fi

# Start the development server
echo "🌐 Starting frontend server on http://localhost:3000"
npm start