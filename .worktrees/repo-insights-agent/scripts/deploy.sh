#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Narayana Platform — AWS Deployment Script
#
# Orchestrates: terraform apply → docker push → migrate → deploy services
# Usage: bash scripts/deploy.sh
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TF_DIR="$PROJECT_ROOT/terraform"

# --- Helpers ---
info()  { printf '\033[0;34m[INFO]\033[0m  %s\n' "$1"; }
ok()    { printf '\033[0;32m[OK]\033[0m    %s\n' "$1"; }
err()   { printf '\033[0;31m[ERROR]\033[0m %s\n' "$1" >&2; }

# --- Pre-flight checks ---
info "Running pre-flight checks..."

for cmd in aws docker terraform jq; do
  if ! command -v "$cmd" &>/dev/null; then
    err "Required command not found: $cmd"
    exit 1
  fi
done

if ! aws sts get-caller-identity &>/dev/null; then
  err "AWS credentials not configured. Run 'aws configure' or set AWS_PROFILE."
  exit 1
fi

CALLER=$(aws sts get-caller-identity --query 'Arn' --output text)
ok "AWS identity: $CALLER"

# =============================================================================
# Step 1: Terraform Apply
# =============================================================================
info "Step 1/6: Running terraform apply..."
cd "$TF_DIR"
terraform init -upgrade
terraform apply

# =============================================================================
# Step 2: Read Terraform Outputs
# =============================================================================
info "Step 2/6: Reading terraform outputs..."
ECR_URL=$(terraform output -raw ecr_repository_url)
CLUSTER_NAME=$(terraform output -raw ecs_cluster_name)
MIGRATE_TASK_ARN=$(terraform output -raw migrate_task_definition_arn)
PRIVATE_SUBNETS=$(terraform output -json private_subnet_ids | jq -r 'join(",")')
ECS_SG=$(terraform output -raw ecs_security_group_id)
ALB_DNS=$(terraform output -raw alb_dns_name)
REGION=$(terraform output -raw region)

ECR_REGISTRY="${ECR_URL%%/*}"

ok "ECR URL:  $ECR_URL"
ok "Cluster:  $CLUSTER_NAME"
ok "ALB DNS:  $ALB_DNS"
ok "Region:   $REGION"

# =============================================================================
# Step 3: Docker Build and Push
# =============================================================================
info "Step 3/6: Building and pushing Docker image..."
cd "$PROJECT_ROOT"

# Authenticate Docker to ECR
aws ecr get-login-password --region "$REGION" \
  | docker login --username AWS --password-stdin "$ECR_REGISTRY"

# Tag with git SHA for traceability
GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
IMAGE_TAG="${GIT_SHA}"

docker build --platform linux/amd64 -t "${ECR_URL}:${IMAGE_TAG}" -t "${ECR_URL}:latest" .
docker push "${ECR_URL}:${IMAGE_TAG}"
docker push "${ECR_URL}:latest"

ok "Pushed image: ${ECR_URL}:${IMAGE_TAG}"

# =============================================================================
# Step 4: Run Migrations
# =============================================================================
info "Step 4/6: Running database migrations via ECS task..."

# Build subnet JSON array for network configuration
SUBNET_ARRAY=$(echo "$PRIVATE_SUBNETS" | tr ',' '\n' | jq -R . | jq -s .)

TASK_ARN=$(aws ecs run-task \
  --region "$REGION" \
  --cluster "$CLUSTER_NAME" \
  --task-definition "$MIGRATE_TASK_ARN" \
  --launch-type FARGATE \
  --network-configuration "{
    \"awsvpcConfiguration\": {
      \"subnets\": $SUBNET_ARRAY,
      \"securityGroups\": [\"$ECS_SG\"],
      \"assignPublicIp\": \"DISABLED\"
    }
  }" \
  --query 'tasks[0].taskArn' \
  --output text)

if [ "$TASK_ARN" = "None" ] || [ -z "$TASK_ARN" ]; then
  err "Failed to start migration task"
  exit 1
fi

info "Migration task started: $TASK_ARN"
info "Waiting for migration task to complete..."

aws ecs wait tasks-stopped \
  --region "$REGION" \
  --cluster "$CLUSTER_NAME" \
  --tasks "$TASK_ARN"

# Check exit code
EXIT_CODE=$(aws ecs describe-tasks \
  --region "$REGION" \
  --cluster "$CLUSTER_NAME" \
  --tasks "$TASK_ARN" \
  --query 'tasks[0].containers[0].exitCode' \
  --output text)

if [ "$EXIT_CODE" != "0" ]; then
  err "Migration task failed with exit code: $EXIT_CODE"
  err "Check CloudWatch logs: /ecs/narayana-migrate"

  STOPPED_REASON=$(aws ecs describe-tasks \
    --region "$REGION" \
    --cluster "$CLUSTER_NAME" \
    --tasks "$TASK_ARN" \
    --query 'tasks[0].stoppedReason' \
    --output text)
  err "Stopped reason: $STOPPED_REASON"
  exit 1
fi

ok "Migrations completed successfully"

# =============================================================================
# Step 5: Force New Deployment
# =============================================================================
info "Step 5/6: Deploying new image to API and Worker services..."

# Service names: narayana-dev-cluster → narayana-dev-api, narayana-dev-worker
SERVICE_PREFIX="${CLUSTER_NAME%-cluster}"

aws ecs update-service \
  --region "$REGION" \
  --cluster "$CLUSTER_NAME" \
  --service "${SERVICE_PREFIX}-api" \
  --force-new-deployment \
  --query 'service.serviceName' \
  --output text

aws ecs update-service \
  --region "$REGION" \
  --cluster "$CLUSTER_NAME" \
  --service "${SERVICE_PREFIX}-worker" \
  --force-new-deployment \
  --query 'service.serviceName' \
  --output text

info "Waiting for services to stabilize (this may take 2-5 minutes)..."

aws ecs wait services-stable \
  --region "$REGION" \
  --cluster "$CLUSTER_NAME" \
  --services "${SERVICE_PREFIX}-api" "${SERVICE_PREFIX}-worker"

ok "Services are stable"

# =============================================================================
# Step 6: Health Check
# =============================================================================
info "Step 6/6: Verifying deployment..."

HEALTH_URL="http://${ALB_DNS}/health/ready"
info "Checking: $HEALTH_URL"

for i in $(seq 1 5); do
  HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "$HEALTH_URL" || true)
  if [ "$HTTP_CODE" = "200" ]; then
    ok "Health check passed (HTTP 200)"
    break
  fi
  if [ "$i" = "5" ]; then
    err "Health check failed after 5 attempts (last HTTP code: $HTTP_CODE)"
    err "URL: $HEALTH_URL"
    exit 1
  fi
  info "Attempt $i/5: HTTP $HTTP_CODE, retrying in 10s..."
  sleep 10
done

echo ""
ok "============================================"
ok "  Deployment complete!"
ok "  ALB URL: http://${ALB_DNS}"
ok "  Image:   ${ECR_URL}:${IMAGE_TAG}"
ok "============================================"
