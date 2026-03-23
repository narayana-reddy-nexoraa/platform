#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# bootstrap-cluster.sh — Provision EKS cluster and install platform components
#
# Prerequisites:
#   - AWS CLI configured with appropriate credentials
#   - terraform installed
#   - kubectl installed
#   - helm installed
#
# Usage:
#   ./scripts/bootstrap-cluster.sh [dev|staging|prod]
# -----------------------------------------------------------------------------
set -euo pipefail

ENV="${1:-dev}"
REGION="us-east-1"
PROJECT="narayana"
CLUSTER_NAME="${PROJECT}-${ENV}-eks"

echo "=== Nexoraa Platform Bootstrap ==="
echo "Environment: ${ENV}"
echo "Region: ${REGION}"
echo "Cluster: ${CLUSTER_NAME}"
echo ""

# -----------------------------------------------------------------------------
# Step 1: Provision EKS via Terraform
# -----------------------------------------------------------------------------
echo ">>> Step 1: Provisioning EKS cluster via Terraform..."
cd terraform

terraform init
terraform plan -var="env=${ENV}" -out=tfplan
echo ""
echo "Review the plan above. Apply? (yes/no)"
read -r CONFIRM
if [ "$CONFIRM" != "yes" ]; then
  echo "Aborted."
  exit 1
fi

terraform apply tfplan
cd ..

# Configure kubectl
echo ""
echo ">>> Configuring kubectl..."
aws eks update-kubeconfig --region "${REGION}" --name "${CLUSTER_NAME}"
kubectl cluster-info

# -----------------------------------------------------------------------------
# Step 2: Create namespaces
# -----------------------------------------------------------------------------
echo ""
echo ">>> Step 2: Creating namespaces..."
kubectl apply -f k8s/base/namespace.yaml

# -----------------------------------------------------------------------------
# Step 3: Install ArgoCD
# -----------------------------------------------------------------------------
echo ""
echo ">>> Step 3: Installing ArgoCD..."
kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

echo "Waiting for ArgoCD to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/argocd-server -n argocd

# Get ArgoCD admin password
echo ""
echo "ArgoCD admin password:"
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
echo ""

# Port-forward ArgoCD UI (background)
echo "ArgoCD UI: kubectl port-forward svc/argocd-server -n argocd 8443:443"

# -----------------------------------------------------------------------------
# Step 4: Install Crossplane
# -----------------------------------------------------------------------------
echo ""
echo ">>> Step 4: Installing Crossplane..."
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update
helm install crossplane crossplane-stable/crossplane \
  --namespace crossplane-system \
  --create-namespace \
  --wait

echo "Waiting for Crossplane to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/crossplane -n crossplane-system

# Install AWS provider
kubectl apply -f crossplane/provider-aws.yaml
echo "Waiting for AWS provider to be healthy..."
sleep 30
kubectl wait --for=condition=healthy --timeout=300s provider.pkg.crossplane.io/provider-aws-ec2 2>/dev/null || true

# -----------------------------------------------------------------------------
# Step 5: Install Strimzi Kafka Operator
# -----------------------------------------------------------------------------
echo ""
echo ">>> Step 5: Installing Strimzi Kafka Operator..."
kubectl create namespace kafka --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f "https://strimzi.io/install/latest?namespace=kafka" -n kafka

echo "Waiting for Strimzi operator to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/strimzi-cluster-operator -n kafka

# -----------------------------------------------------------------------------
# Step 6: Install Temporal via Helm
# -----------------------------------------------------------------------------
echo ""
echo ">>> Step 6: Installing Temporal..."
kubectl create namespace temporal --dry-run=client -o yaml | kubectl apply -f -
helm repo add temporal https://charts.temporal.io
helm repo update

# Use existing RDS as Temporal's database
RDS_ENDPOINT=$(cd terraform && terraform output -raw rds_endpoint)
helm install temporal temporal/temporal \
  --namespace temporal \
  --set server.replicaCount=1 \
  --set cassandra.enabled=false \
  --set mysql.enabled=false \
  --set postgresql.enabled=false \
  --set schema.setup.enabled=false \
  --set schema.update.enabled=false \
  --set server.config.persistence.default.driver=sql \
  --set server.config.persistence.default.sql.driver=postgres12 \
  --set "server.config.persistence.default.sql.host=${RDS_ENDPOINT%%:*}" \
  --set server.config.persistence.default.sql.port=5432 \
  --set server.config.persistence.default.sql.database=temporal \
  --set server.config.persistence.default.sql.user=narayana \
  --set server.config.persistence.default.sql.password="${DB_PASSWORD:-narayana}" \
  --wait --timeout 600s || echo "Temporal Helm install may need manual DB setup — check docs"

# -----------------------------------------------------------------------------
# Step 7: Deploy ArgoCD App-of-Apps
# -----------------------------------------------------------------------------
echo ""
echo ">>> Step 7: Deploying ArgoCD app-of-apps..."
kubectl apply -f argocd/bootstrap/app-of-apps.yaml

# -----------------------------------------------------------------------------
# Step 8: Apply Crossplane claims for this environment
# -----------------------------------------------------------------------------
echo ""
echo ">>> Step 8: Applying Crossplane claims for ${ENV}..."
kubectl apply -f "crossplane/claims/${ENV}/" 2>/dev/null || echo "No Crossplane claims for ${ENV} yet"

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
echo ""
echo "============================================"
echo "  Nexoraa Platform Bootstrap Complete!"
echo "============================================"
echo ""
echo "Cluster:     ${CLUSTER_NAME}"
echo "Region:      ${REGION}"
echo "Environment: ${ENV}"
echo ""
echo "Access points:"
echo "  kubectl:    aws eks update-kubeconfig --region ${REGION} --name ${CLUSTER_NAME}"
echo "  ArgoCD UI:  kubectl port-forward svc/argocd-server -n argocd 8443:443"
echo "  Temporal UI: kubectl port-forward svc/temporal-web -n temporal 8088:8080"
echo ""
echo "ArgoCD will now auto-sync all applications from Git."
echo "Check status: kubectl get applications -n argocd"
echo ""
echo "Next steps:"
echo "  1. Configure ArgoCD repo credentials (if private repo)"
echo "  2. Verify all ArgoCD apps are synced: kubectl get applications -n argocd"
echo "  3. Run integration tests against the cluster"
echo ""
