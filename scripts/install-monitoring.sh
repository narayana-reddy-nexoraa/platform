#!/usr/bin/env bash
# -----------------------------------------------------------------------------
# install-monitoring.sh — Self-hosted Prometheus + Grafana on EKS (Helm)
#
# Uses kube-prometheus-stack. Scrapes Nexoraa apps via ServiceMonitors in k8s/.
# Full guide: docs/observability.md
#
# Prerequisites: kubectl configured for the target cluster, helm 3+
#
# Usage:
#   ./scripts/install-monitoring.sh [dev|staging|prod]
#   GRAFANA_ADMIN_PASSWORD='secret' ./scripts/install-monitoring.sh prod
# -----------------------------------------------------------------------------
set -euo pipefail

ENV="${1:-dev}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
HELM_DIR="${REPO_ROOT}/k8s/helm/kube-prometheus-stack"
RELEASE_NAME="kube-prometheus-stack"
NAMESPACE="monitoring"
# Pin chart version for reproducible installs (https://github.com/prometheus-community/helm-charts/releases)
CHART_VERSION="${KPS_CHART_VERSION:-67.2.0}"

case "${ENV}" in
  dev|staging|prod) ;;
  *)
    echo "Usage: $0 [dev|staging|prod]"
    exit 1
    ;;
esac

VALUES_FILES=(
  -f "${HELM_DIR}/values.yaml"
  -f "${HELM_DIR}/values-${ENV}.yaml"
)

echo "=== Nexoraa monitoring (kube-prometheus-stack) ==="
echo "Environment: ${ENV}"
echo "Chart version: ${CHART_VERSION}"
echo ""

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
helm repo update prometheus-community

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo ">>> Creating Grafana dashboards ConfigMap from deploy/grafana/dashboards/*.json ..."
DASHBOARD_DIR="${REPO_ROOT}/deploy/grafana/dashboards"
CM_ARGS=()
shopt -s nullglob
for dashboard in "${DASHBOARD_DIR}"/*.json; do
  CM_ARGS+=(--from-file="${dashboard}")
done
shopt -u nullglob
if [ "${#CM_ARGS[@]}" -eq 0 ]; then
  echo "ERROR: no dashboard JSON files found in ${DASHBOARD_DIR}"
  exit 1
fi
kubectl -n "${NAMESPACE}" create configmap nexoraa-grafana-dashboards \
  "${CM_ARGS[@]}" \
  --dry-run=client -o yaml | kubectl apply -f -

GRAFANA_PASSWORD="${GRAFANA_ADMIN_PASSWORD:-changeme}"
if [ "${ENV}" = "dev" ] && [ "${GRAFANA_PASSWORD}" = "changeme" ]; then
  echo "Note: using default Grafana admin password 'changeme' for dev. Set GRAFANA_ADMIN_PASSWORD for other envs."
fi

echo ">>> Installing / upgrading ${RELEASE_NAME}..."
helm upgrade --install "${RELEASE_NAME}" prometheus-community/kube-prometheus-stack \
  --namespace "${NAMESPACE}" \
  --version "${CHART_VERSION}" \
  "${VALUES_FILES[@]}" \
  --set "grafana.adminPassword=${GRAFANA_PASSWORD}" \
  --wait --timeout 600s

echo ""
echo ">>> Waiting for core pods..."
kubectl wait --for=condition=available --timeout=300s \
  deployment/"${RELEASE_NAME}-grafana" -n "${NAMESPACE}" 2>/dev/null || true
kubectl wait --for=condition=available --timeout=300s \
  statefulset/prometheus-"${RELEASE_NAME}-kube-prometheus-prometheus" -n "${NAMESPACE}" 2>/dev/null || true

echo ""
echo "============================================"
echo "  Monitoring stack ready"
echo "============================================"
echo ""
echo "Grafana (port-forward):"
echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME}-grafana 3001:80"
echo "  open http://localhost:3001  (user: admin)"
echo ""
echo "Prometheus (port-forward):"
echo "  kubectl port-forward -n ${NAMESPACE} svc/${RELEASE_NAME}-kube-prometheus-prometheus 9091:9090"
echo ""
echo "Next: apply app ServiceMonitors with your overlay, e.g.:"
echo "  kubectl kustomize --load-restrictor=LoadRestrictionsNone k8s/overlays/${ENV}/ | kubectl apply -f -"
echo ""
