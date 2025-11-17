#!/bin/bash
set -e

# Caronte k3s Deployment Script
# This script deploys Caronte to a k3s cluster with proper AWS credentials

echo "==> Deploying Caronte to k3s"

# Check AWS profile
if [ "$AWS_PROFILE" != "codnod" ]; then
    echo "ERROR: AWS_PROFILE must be 'codnod'"
    exit 1
fi

# Check k3s context
CURRENT_CONTEXT=$(kubectl config current-context)
if [ "$CURRENT_CONTEXT" != "k3s" ]; then
    echo "ERROR: Current kubectl context is '$CURRENT_CONTEXT', must be 'k3s'"
    exit 1
fi

# Create namespace
echo "==> Creating caronte namespace"
kubectl create namespace caronte --dry-run=client -o yaml | kubectl apply -f -

# Create AWS credentials secret
echo "==> Creating AWS credentials secret from AWS CLI profile"
AWS_ACCESS_KEY=$(aws configure get aws_access_key_id --profile codnod)
AWS_SECRET_KEY=$(aws configure get aws_secret_access_key --profile codnod)
AWS_REGION=${AWS_REGION:-eu-west-1}

kubectl create secret generic aws-credentials \
  --from-literal=AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY" \
  --from-literal=AWS_SECRET_ACCESS_KEY="$AWS_SECRET_KEY" \
  --from-literal=AWS_REGION="$AWS_REGION" \
  --namespace=caronte \
  --dry-run=client -o yaml | kubectl apply -f -

# Load Docker image to k3s
echo "==> Loading Docker image to k3s"
docker save caronte:latest | sudo k3s ctr images import -

# Apply RBAC
echo "==> Applying RBAC manifests"
kubectl apply -f config/rbac/service_account.yaml -n caronte
kubectl apply -f config/rbac/role.yaml
kubectl apply -f config/rbac/role_binding.yaml

# Update role binding to use caronte namespace
kubectl patch clusterrolebinding caronte \
  --type='json' \
  -p='[{"op": "replace", "path": "/subjects/0/namespace", "value": "caronte"}]'

# Deploy Caronte controller
echo "==> Deploying Caronte controller"
cat config/manager/deployment.yaml | sed 's/namespace: default/namespace: caronte/g' | kubectl apply -f -

# Wait for deployment
echo "==> Waiting for Caronte to be ready"
kubectl wait --for=condition=available --timeout=60s deployment/caronte -n caronte

echo "==> Caronte deployed successfully!"
echo ""
echo "Check status with:"
echo "  kubectl get pods -n caronte"
echo "  kubectl logs -n caronte -l app=caronte"
