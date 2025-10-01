# Environment File Generator

## Quick Setup Script

The `scripts/create-my-env.sh` script automatically generates environment files for both backend and frontend by extracting information from your current OpenShift cluster.

### Prerequisites

1. OpenShift CLI (`oc`) must be installed
2. You must be logged into your OpenShift cluster

### Usage

```bash
# Make sure you're logged into your cluster
oc login --web --server=https://api.your-cluster.example.com:6443

# Run the script from the project root
./create-my-env.sh
```

### What it does

The script will:

1. ✅ Verify you're logged into OpenShift
2. 🔍 Extract cluster domain and service URLs
3. 🔑 Find Key Manager route in `platform-services` namespace
4. 🗝️ Attempt to extract admin key from secrets
5. 📝 Generate `apps/backend/.env` with all required variables
6. 📝 Generate `apps/frontend/.env.local` with React environment variables
7. 📋 Provide summary and next steps

### Generated Files

- **`apps/backend/.env`** - Backend configuration with cluster URLs and admin keys
- **`apps/frontend/.env.local`** - Frontend React app configuration

### Manual Steps (if needed)

If the admin key extraction fails, you can manually add it:

```bash
# Extract admin key manually
oc get secret key-manager-admin -n platform-services -o jsonpath='{.data.admin-key}' | base64 -d

# Then update ADMIN_KEY in apps/backend/.env
```

### After Running

1. Review the generated `.env` files
2. Start the development servers:
   ```bash
   # Terminal 1 - Backend
   cd apps/backend && npm run dev
   
   # Terminal 2 - Frontend  
   cd apps/frontend && npm start
   ```
3. Access the application at http://localhost:3000

This eliminates the need to manually configure environment variables and significantly speeds up the development setup process.