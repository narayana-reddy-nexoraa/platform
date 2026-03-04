# -----------------------------------------------------------------------------
# ECS Task Execution Role — used by ECS agent to pull images and push logs
# -----------------------------------------------------------------------------
data "aws_iam_policy_document" "ecs_tasks_assume_role" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "ecs_execution" {
  name               = "${var.project_name}-${var.env}-ecs-execution-role"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume_role.json

  tags = {
    Name        = "${var.project_name}-${var.env}-ecs-execution-role"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}

resource "aws_iam_role_policy_attachment" "ecs_execution" {
  role       = aws_iam_role.ecs_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

# -----------------------------------------------------------------------------
# ECS Task Role — assumed by the running containers
# -----------------------------------------------------------------------------
resource "aws_iam_role" "ecs_task" {
  name               = "${var.project_name}-${var.env}-ecs-task-role"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume_role.json

  tags = {
    Name        = "${var.project_name}-${var.env}-ecs-task-role"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}
