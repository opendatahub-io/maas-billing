#!/bin/bash

# Hard cleanup script for MaaS Billing development environment
# This script removes environment files and restarts the development setup

echo "🧹 Starting hard cleanup of MaaS Billing environment..."

# Remove environment files
echo "📁 Removing environment files..."
rm -f apps/backend/.env
rm -f apps/frontend/.env.local

echo "✅ Environment files removed"

# Create fresh environment
echo "🔧 Creating fresh environment..."
./create-my-env.sh

echo "🛑 Stopping development services..."
./stop-dev.sh

echo "🚀 Starting development services..."
./start-dev.sh

echo "✅ Hard cleanup completed successfully!"
echo "🌐 Frontend should be available at http://localhost:3000"
echo "🔧 Backend API should be available at the auto-detected port"