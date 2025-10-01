#!/bin/bash

# OpenDataHub Installation Script
# This script installs ODH operator and creates necessary resources in the correct order

set -e

echo "========================================="
echo "🚀 OpenDataHub (ODH) Installation"
echo "========================================="
echo ""

# Step 1: Install ODH Operator
echo "1️⃣ Installing ODH Operator..."

# Check if operator is already installed
if kubectl get csv -n openshift-operators 2>/dev/null | grep -q opendatahub-operator; then
    echo "   ✅ ODH operator already installed"
else
    echo "   Creating OperatorGroup..."
    cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: opendatahub
  namespace: openshift-operators
EOF

    echo "   Creating Subscription..."
    cat <<EOF | kubectl apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: opendatahub-operator
  namespace: openshift-operators
spec:
  channel: fast  # or stable, depending on your needs
  name: opendatahub-operator
  source: community-operators
  sourceNamespace: openshift-marketplace
EOF

    echo "   Waiting for operator to be ready (this may take a few minutes)..."
    sleep 30
    
    # Wait for operator to be ready
    for i in {1..30}; do
        if kubectl get deployment -n openshift-operators | grep -q opendatahub-operator; then
            echo "   Operator deployment found, waiting for ready state..."
            kubectl wait --for=condition=Available deployment -l app.kubernetes.io/name=opendatahub-operator -n openshift-operators --timeout=300s || true
            break
        fi
        echo "   Waiting for operator deployment to appear... ($i/30)"
        sleep 10
    done
fi

# Step 2: Create ODH namespace if it doesn't exist
echo ""
echo "2️⃣ Creating opendatahub namespace..."
kubectl create namespace opendatahub 2>/dev/null || echo "   Namespace already exists"

# Step 3: Wait for CRDs to be registered
echo ""
echo "3️⃣ Waiting for ODH CRDs to be registered..."
for i in {1..30}; do
    if kubectl get crd dscinitializations.dscinitialization.opendatahub.io &>/dev/null 2>&1; then
        echo "   ✅ DSCInitialization CRD found"
        break
    fi
    echo "   Waiting for DSCInitialization CRD... ($i/30)"
    sleep 10
done

if ! kubectl get crd dscinitializations.dscinitialization.opendatahub.io &>/dev/null 2>&1; then
    echo "   ❌ DSCInitialization CRD not found after waiting"
    echo "   Please check the operator logs:"
    echo "   kubectl logs -n openshift-operators deployment/opendatahub-operator-controller-manager"
    exit 1
fi

# Step 4: Create DSCInitialization (REQUIRED before DataScienceCluster)
echo ""
echo "4️⃣ Creating DSCInitialization resource..."
cat <<EOF | kubectl apply -f -
apiVersion: dscinitialization.opendatahub.io/v1
kind: DSCInitialization
metadata:
  name: default-dsci
  namespace: opendatahub
spec:
  applicationsNamespace: opendatahub
  monitoring:
    managementState: Managed
    namespace: opendatahub
  serviceMesh:
    managementState: Managed
    auth:
      audiences:
        - "https://kubernetes.default.svc"
    controlPlane:
      name: data-science-smcp
      namespace: istio-system
  trustedCABundle:
    managementState: Managed
EOF

echo "   Waiting for DSCInitialization to be ready..."
for i in {1..30}; do
    if kubectl get dscinitializations -n opendatahub default-dsci -o jsonpath='{.status.phase}' 2>/dev/null | grep -q "Ready"; then
        echo "   ✅ DSCInitialization is ready"
        break
    fi
    echo "   Waiting for DSCInitialization to be ready... ($i/30)"
    sleep 10
done

# Step 5: Create DataScienceCluster
echo ""
echo "5️⃣ Creating DataScienceCluster..."
cat <<EOF | kubectl apply -f -
apiVersion: datasciencecluster.opendatahub.io/v1
kind: DataScienceCluster
metadata:
  name: default-dsc
  namespace: opendatahub
spec:
  components:
    # Core component for notebooks
    dashboard:
      managementState: Managed
    
    # Notebook controller
    workbenches:
      managementState: Managed
    
    # Model serving with KServe in RawDeployment mode (no Knative)
    kserve:
      managementState: Managed
      defaultDeploymentMode: RawDeployment
      nim:
        managementState: Managed  # Enable NVIDIA NIM support
      rawDeploymentServiceConfig: Headless
      serving:
        ingressGateway:
          certificate:
            type: OpenshiftDefaultIngress
        managementState: Removed  # Disable Knative serving (using RawDeployment)
        name: knative-serving
    
    # Model serving platform
    modelmeshserving:
      managementState: Removed  # Use KServe instead
    
    # Data science pipelines
    datasciencepipelines:
      managementState: Removed  # Not needed for MaaS
    
    # Ray for distributed computing
    ray:
      managementState: Removed  # Not needed for MaaS
    
    # Kueue for job queueing
    kueue:
      managementState: Removed  # Not needed for MaaS
    
    # Model registry
    modelregistry:
      managementState: Removed  # Not needed for MaaS
    
    # TrustyAI for model explainability
    trustyai:
      managementState: Removed  # Not needed for MaaS
    
    # Training operator
    trainingoperator:
      managementState: Removed  # Not needed for MaaS
EOF

echo "   Waiting for DataScienceCluster to be ready..."
for i in {1..60}; do
    PHASE=$(kubectl get datasciencecluster -n opendatahub default-dsc -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
    CONDITIONS=$(kubectl get datasciencecluster -n opendatahub default-dsc -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || echo "Unknown")
    
    if [[ "$PHASE" == "Ready" ]] || [[ "$CONDITIONS" == "True" ]]; then
        echo "   ✅ DataScienceCluster is ready"
        break
    fi
    echo "   Status: Phase=$PHASE, Ready=$CONDITIONS ($i/60)"
    sleep 10
done

# Step 6: Verify installation
echo ""
echo "========================================="
echo "📊 Verification"
echo "========================================="
echo ""

echo "DSCInitialization Status:"
kubectl get dscinitializations -n opendatahub

echo ""
echo "DataScienceCluster Status:"
kubectl get datasciencecluster -n opendatahub

echo ""
echo "KServe Components:"
kubectl get pods -n kserve 2>/dev/null || echo "KServe namespace not yet created"

echo ""
echo "Istio Components:"
kubectl get pods -n istio-system 2>/dev/null || echo "Istio namespace not yet created"

echo ""
echo "========================================="
echo "✅ ODH Installation Complete!"
echo "========================================="
echo ""
echo "Next steps:"
echo "1. Verify KServe is running: kubectl get pods -n kserve"
echo "2. Check Service Mesh: kubectl get smcp -n istio-system"
echo "3. Deploy your models using KServe InferenceService"
echo ""
echo "If you encounter issues, check the logs:"
echo "- ODH Operator: kubectl logs -n openshift-operators deployment/opendatahub-operator-controller-manager"
echo "- DSCInitialization: kubectl describe dscinitializations -n opendatahub default-dsci"
echo "- DataScienceCluster: kubectl describe datasciencecluster -n opendatahub default-dsc" 