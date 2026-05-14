# -----------------------------------------------------------------------------
# AMP — outputs (separate file to avoid touching outputs.tf)
# -----------------------------------------------------------------------------

output "amp_workspace_id" {
  description = "Amazon Managed Prometheus workspace ID (ws-...)"
  value       = aws_prometheus_workspace.main.id
}

output "amp_workspace_arn" {
  description = "AMP workspace ARN"
  value       = aws_prometheus_workspace.main.arn
}

output "amp_prometheus_query_endpoint" {
  description = "Prometheus-compatible query API endpoint for this workspace"
  value       = aws_prometheus_workspace.main.prometheus_endpoint
}

output "amp_remote_write_url" {
  description = "Prometheus remote_write URL; use as AWS_PROMETHEUS_ENDPOINT for the ADOT collector Deployment"
  value       = "https://aps-workspaces.${var.region}.amazonaws.com/workspaces/${aws_prometheus_workspace.main.id}/api/v1/remote_write"
}

output "amp_remote_write_irsa_role_arn" {
  description = "IAM role ARN — annotate SA monitoring/amp-remote-write with eks.amazonaws.com/role-arn"
  value       = aws_iam_role.amp_remote_write.arn
}
