#!/usr/bin/env bash
# Generates adot-config.env for a given overlay from terraform output.
# Run from repo root after: terraform -chdir=terraform apply
#
# Usage: ./scripts/configure-adot.sh <dev|staging|prod>
set -euo pipefail

ENV="${1:-}"
if [[ -z "$ENV" ]]; then
  echo "Usage: $0 <dev|staging|prod>" >&2
  exit 1
fi

OVERLAY="k8s/overlays/${ENV}/adot"
if [[ ! -d "$OVERLAY" ]]; then
  echo "Overlay directory not found: $OVERLAY" >&2
  exit 1
fi

echo "Reading terraform outputs..."
AMP_URL=$(terraform -chdir=terraform output -raw amp_remote_write_url)
IRSA_ARN=$(terraform -chdir=terraform output -raw amp_remote_write_irsa_role_arn)
REGION=$(terraform -chdir=terraform output -raw region 2>/dev/null || echo "us-east-1")

cat > "${OVERLAY}/adot-config.env" <<EOF
AWS_REGION=${REGION}
AMP_REMOTE_WRITE_URL=${AMP_URL}
IRSA_ROLE_ARN=${IRSA_ARN}
EOF

echo "Written: ${OVERLAY}/adot-config.env"
echo "  AWS_REGION=${REGION}"
echo "  AMP_REMOTE_WRITE_URL=${AMP_URL}"
echo "  IRSA_ROLE_ARN=${IRSA_ARN}"
