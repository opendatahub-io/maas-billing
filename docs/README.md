# MaaS Platform Documentation

Welcome to the Model-as-a-Service (MaaS) Platform documentation. This platform provides a comprehensive solution for deploying and managing AI models with policy-based access control, rate limiting, and tier-based subscriptions.

## 📚 Documentation Overview

### 🚀 Getting Started

- **[Installation Guide](installation.md)** - Complete platform deployment instructions
- **[Getting Started](getting-started.md)** - Quick start guide after installation

## Architecture and Components

- **[Architecture](architecture.md)** - Overview of the MaaS Platform architecture
- **[Observability](observability.md)** - Overview of the MaaS Platform observability components

### ⚙️ Configuration & Management

- **[Gateway Setup](gateway-setup.md)** - Setting up authentication and rate limiting
- **[Tier Management](tier-management.md)** - Configuring subscription tiers and access control
- **[Model Access Guide](model-access.md)** - Managing model access and policies

### 🔧 Advanced Administration

- **[Observability](observability.md)** - Monitoring, metrics, and dashboards


### 👥 End Users

- **[User Guide](user-guide.md)** - How end users interact with the platform

## 🚀 Quick Start for Administrators

### 📹 New to MaaS? Watch Our Installation Video

For a visual guide to getting started, check out our [Installation Video Walkthrough](installation.md#-video-walkthrough) that covers the complete deployment process.

### Administrator Getting Started Steps

1. **Deploy the platform**: Follow the [Installation Guide](installation.md) to set up MaaS in your cluster
2. **Configure authentication**: Set up [Gateway authentication](gateway-setup.md) for your organization
3. **Configure tiers**: Set up [Tier Management](tier-management.md) for access control
4. **Test the deployment**: Follow [Getting Started](getting-started.md) to verify everything works

## 📋 Prerequisites for Administrators

- **OpenShift cluster** (4.19.9+) with kubectl/oc access
- **ODH/RHOAI** with KServe enabled
- **Cluster admin** permissions for initial setup
- **Basic Kubernetes knowledge** for troubleshooting

## 🏗️ Platform Components

- **Gateway API**: Traffic routing and management
- **Kuadrant/Authorino/Limitador**: Authentication, authorization, and rate limiting
- **KServe**: Model serving platform
- **MaaS API**: Token management and tier resolution
- **React Frontend**: Web-based management interface

## 👥 For End Users

If you're an end user looking to use AI models through the MaaS platform, your administrator should provide you with:

- **Access credentials** (tokens or OAuth setup)
- **Available models** and their capabilities
- **Usage guidelines** and rate limits
- **API endpoints** for model interaction

For detailed end-user documentation, see the [User Guide](user-guide.md) (coming soon).

## 📞 Support

For questions or issues:

- **Administrators**: Open an issue on GitHub or check the [Installation Guide](installation.md) for troubleshooting
- **End Users**: Contact your platform administrator for access and usage questions
- **General**: Review the [Samples](samples/) for examples

## 📝 License

This project is licensed under the Apache 2.0 License.