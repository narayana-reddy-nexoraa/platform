# -----------------------------------------------------------------------------
# DB Subnet Group (private subnets only)
# -----------------------------------------------------------------------------
resource "aws_db_subnet_group" "main" {
  name = "${var.project_name}-${var.env}-db-subnet-group"
  subnet_ids = [
    aws_subnet.private_1.id,
    aws_subnet.private_2.id,
  ]

  tags = {
    Name        = "${var.project_name}-${var.env}-db-subnet-group"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}

# -----------------------------------------------------------------------------
# RDS PostgreSQL 15
# -----------------------------------------------------------------------------
resource "aws_db_instance" "main" {
  identifier = "${var.project_name}-${var.env}-postgres"

  engine         = "postgres"
  engine_version = "15"
  instance_class = "db.t3.micro"

  allocated_storage = 20
  storage_type      = "gp3"

  db_name  = "execution_engine"
  username = "postgres"
  password = var.db_password

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  multi_az            = false
  publicly_accessible = false
  skip_final_snapshot = true

  tags = {
    Name        = "${var.project_name}-${var.env}-postgres"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}
