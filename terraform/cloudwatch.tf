# -----------------------------------------------------------------------------
# CloudWatch Log Groups for ECS tasks
# -----------------------------------------------------------------------------
resource "aws_cloudwatch_log_group" "api" {
  name              = "/ecs/narayana-api"
  retention_in_days = 14

  tags = {
    Name        = "${var.project_name}-${var.env}-api-logs"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}

resource "aws_cloudwatch_log_group" "worker" {
  name              = "/ecs/narayana-worker"
  retention_in_days = 14

  tags = {
    Name        = "${var.project_name}-${var.env}-worker-logs"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}
