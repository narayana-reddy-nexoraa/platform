# -----------------------------------------------------------------------------
# AMG — outputs (separate from outputs.tf)
# -----------------------------------------------------------------------------

output "amg_workspace_id" {
  description = "Amazon Managed Grafana workspace ID (g-...)"
  value       = aws_grafana_workspace.main.id
}

output "amg_workspace_arn" {
  description = "AMG workspace ARN"
  value       = aws_grafana_workspace.main.arn
}

output "amg_workspace_endpoint" {
  description = "Grafana UI URL (HTTPS) for this workspace"
  value       = "https://${aws_grafana_workspace.main.endpoint}"
}

output "amg_grafana_version" {
  description = "Grafana version running on the workspace"
  value       = aws_grafana_workspace.main.grafana_version
}
