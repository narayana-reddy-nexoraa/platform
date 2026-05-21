#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# setup-after-terraform-apply.sh — Run AFTER Terraform has created EKS (and VPC/RDS, etc.)
#
# Does NOT run terraform apply. Use this when apply is done separately or by CI.
#
# What it does:
#   1. Reads EKS cluster name + region from Terraform outputs (terraform/ must match state)
#   2. Configures kubectl for that cluster
#   3. Applies base namespaces (k8s/base/namespace.yaml)
#   4. Installs kube-prometheus-stack (Prometheus + Grafana)
#   5. Applies the environment Kustomize overlay (apps + ServiceMonitors for scraping)
#
# Prerequisites:
#   - terraform apply already succeeded for this workspace
#   - AWS CLI credentials with EKS describe/update-kubeconfig permissions
#   - kubectl, helm 3+
#
# Usage:
#   ./scripts/setup-after-terraform-apply.sh [dev|staging|prod] [--skip-kustomize]
#   GRAFANA_ADMIN_PASSWORD='secret' ./scripts/setup-after-terraform-apply.sh prod
#
# Flags:
#   --skip-kustomize  Do not apply the Kustomize overlay (use when Argo CD will
#                     sync apps; monitoring stack is still installed.)
#
# Optional env vars:
#   TERRAFORM_DIR  — path to terraform root (default: repo/terraform)
# -----------------------------------------------------------------------------
set -euo pipefail

ENV="dev"
SKIP_KUSTOMIZE=false
for arg in "$@"; do
  case "${arg}" in
    dev|staging|prod) ENV="${arg}" ;;
    --skip-kustomize) SKIP_KUSTOMIZE=true ;;
    -h|--help)
      echo "Usage: $0 [dev|staging|prod] [--skip-kustomize]"
      exit 0
      ;;
    *)
      echo "Usage: $0 [dev|staging|prod] [--skip-kustomize]"
      exit 1
      ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
TERRAFORM_DIR="${TERRAFORM_DIR:-${REPO_ROOT}/terraform}"

case "${ENV}" in
  dev|staging|prod) ;;
  *)
    echo "Usage: $0 [dev|staging|prod] [--skip-kustomize]"
    exit 1
    ;;
esac

echo "=== Post-Terraform EKS setup (observability + app manifests) ==="
echo "Environment: ${ENV}"
echo "Terraform dir: ${TERRAFORM_DIR}"
echo ""

if ! command -v terraform >/dev/null 2>&1; then
  echo "Error: terraform not found in PATH."
  exit 1
fi

if ! command -v aws >/dev/null 2>&1; then
  echo "Error: aws CLI not found in PATH."
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "Error: kubectl not found in PATH."
  exit 1
fi

if ! command -v helm >/dev/null 2>&1; then
  echo "Error: helm not found in PATH."
  exit 1
fi

# -----------------------------------------------------------------------------
# Resolve cluster + region from Terraform (must match applied workspace)
# -----------------------------------------------------------------------------
echo ">>> Reading Terraform outputs..."
CLUSTER_NAME="$(terraform -chdir="${TERRAFORM_DIR}" output -raw eks_cluster_name)"
REGION="$(terraform -chdir="${TERRAFORM_DIR}" output -raw region)"

if [ -z "${CLUSTER_NAME}" ] || [ -z "${REGION}" ]; then
  echo "Error: empty eks_cluster_name or region from terraform output."
  echo "Run terraform apply first, or fix TERRAFORM_DIR (current: ${TERRAFORM_DIR})."
  exit 1
fi

echo "    Cluster: ${CLUSTER_NAME}"
echo "    Region:  ${REGION}"
echo ""

echo ">>> Configuring kubectl..."
aws eks update-kubeconfig --region "${REGION}" --name "${CLUSTER_NAME}"
kubectl cluster-info

echo ""
echo ">>> Verifying API access..."
kubectl get nodes

echo ""
echo ">>> Applying Kubernetes namespaces..."
kubectl apply -f "${REPO_ROOT}/k8s/base/namespace.yaml"

echo ""
echo ">>> Installing monitoring stack (Prometheus + Grafana)..."
"${SCRIPT_DIR}/install-monitoring.sh" "${ENV}"

if [ "${SKIP_KUSTOMIZE}" = true ]; then
  echo ""
  echo ">>> Skipping Kustomize overlay (--skip-kustomize). Deploy apps via Argo CD or run:"
  echo "    kubectl kustomize --load-restrictor=LoadRestrictionsNone k8s/overlays/${ENV}/ | kubectl apply -f -"
else
  echo ""
  echo ">>> Applying Kustomize overlay (apps + ServiceMonitors): k8s/overlays/${ENV}/"
  kubectl kustomize --load-restrictor=LoadRestrictionsNone "${REPO_ROOT}/k8s/overlays/${ENV}/" | kubectl apply -f -
fi

echo ""
echo "============================================"
echo "  Post-apply setup complete"
echo "============================================"
echo ""
echo "Cluster: ${CLUSTER_NAME}"
echo "Region:  ${REGION}"
echo ""
echo "Grafana:"
echo "  kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3001:80"
echo "  (user: admin — set GRAFANA_ADMIN_PASSWORD next time for non-dev)"
echo ""
echo "Prometheus:"
echo "  kubectl port-forward -n monitoring svc/kube-prometheus-stack-kube-prometheus-prometheus 9091:9090"
echo ""
echo "Observability (metrics, Grafana, runbooks):"
echo "  docs/observability.md"
echo ""
echo "For full platform (ArgoCD, Crossplane, Strimzi, Temporal, etc.) see:"
echo "  ./scripts/bootstrap-cluster.sh  (includes terraform apply)"
echo ""
