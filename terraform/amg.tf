# -----------------------------------------------------------------------------
# Amazon Managed Grafana (AMG)
# Pairs with AMP (amp.tf): enable PROMETHEUS data source, then add this workspace’s
# AMP in the Grafana UI (Connections → AWS data sources → Prometheus) using the
# same region and terraform output amp_workspace_id.
# Prerequisites: IAM Identity Center if using AWS_SSO (default). See observability.md.
# -----------------------------------------------------------------------------

resource "aws_grafana_workspace" "main" {
  name                     = "${var.project_name}-${var.env}-amg"
  description              = "Nexoraa platform dashboards (${var.project_name}, ${var.env})"
  account_access_type      = "CURRENT_ACCOUNT"
  authentication_providers = var.amg_authentication_providers
  permission_type          = "SERVICE_MANAGED"
  data_sources             = var.amg_data_sources
  grafana_version          = var.amg_grafana_version

  tags = {
    Name        = "${var.project_name}-${var.env}-amg"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}
