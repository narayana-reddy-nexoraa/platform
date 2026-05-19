# -----------------------------------------------------------------------------
# Amazon Managed Grafana (AMG) — optional inputs
# -----------------------------------------------------------------------------

variable "amg_authentication_providers" {
  description = "How users sign in to AMG. AWS_SSO requires IAM Identity Center enabled in the account. SAML requires IdP configuration after create."
  type        = list(string)
  default     = ["AWS_SSO"]

  validation {
    condition = length(var.amg_authentication_providers) > 0 && alltrue([
      for p in var.amg_authentication_providers : contains(["AWS_SSO", "SAML"], p)
    ])
    error_message = "At least one provider required; each must be AWS_SSO or SAML."
  }
}

variable "amg_data_sources" {
  description = "AWS data sources the workspace may use. PROMETHEUS enables AMP; CLOUDWATCH enables metrics/logs in Grafana."
  type        = list(string)
  default     = ["PROMETHEUS", "CLOUDWATCH"]
}

variable "amg_grafana_version" {
  description = "Grafana major line for the workspace (e.g. 10.4). See aws_grafana_workspace docs for supported values."
  type        = string
  default     = "10.4"
}
