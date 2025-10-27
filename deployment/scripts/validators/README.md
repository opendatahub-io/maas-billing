# Validators

This folder contains validation scripts for verifying that components exist and are properly configured in the MaaS (Model-as-a-Service) billing system.

## Purpose

These scripts validate the presence, configuration, and health of various components that are required for the MaaS deployment. They help ensure that all dependencies are correctly installed and functioning before proceeding with the main deployment.

## Available Scripts

- `validate-rhoai3.sh` - Validates RHOAI/OpenDataHub installation by checking ClusterServiceVersion
- `validate-cert-manager.sh` - Validates cert-manager certificate management installation

## Usage

```bash
# Validate RHOAI installation
./validate-rhoai3.sh

# Validate cert-manager installation
./validate-cert-manager.sh
```

