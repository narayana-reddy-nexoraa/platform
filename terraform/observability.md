# Observability Implementation Overview

This document provides a detailed understanding of the observability components implemented in our AWS infrastructure using Terraform. The setup focuses on monitoring, logging, and metrics collection for both ECS and EKS workloads.

## Table of Contents
1. [Amazon Managed Prometheus (AMP)](#amazon-managed-prometheus-amp)
2. [Amazon Managed Grafana (AMG)](#amazon-managed-grafana-amg)
3. [CloudWatch Logging](#cloudwatch-logging)
4. [IAM Roles and Permissions](#iam-roles-and-permissions)
5. [Integration Points](#integration-points)
6. [Future Enhancements](#future-enhancements)

## Amazon Managed Prometheus (AMP)

### Overview
Amazon Managed Prometheus (AMP) is a fully managed, Prometheus-compatible monitoring service that provides secure, scalable, and highly available metrics collection and querying capabilities. In our setup, we've implemented a basic AMP workspace with remote write capabilities for EKS-based applications.

### Components Implemented

#### 1. AMP Workspace
- **Resource**: `aws_prometheus_workspace.main`
- **Alias**: `${var.project_name}-${var.env}`
- **Purpose**: Central metrics storage and querying endpoint
- **Tags**: Includes project, environment, and management metadata

#### 2. Remote Write IAM Role (IRSA)
- **Resource**: `aws_iam_role.amp_remote_write`
- **Purpose**: Allows EKS pods to authenticate and write metrics to AMP
- **Service Account**: `monitoring/amp-remote-write` (to be created in EKS cluster)
- **Permissions**:
  - `aps:RemoteWrite`: Write metrics to the workspace
  - `aps:GetSeries`: Query metric series
  - `aps:GetLabels`: Retrieve label information
  - `aps:GetMetricMetadata`: Access metric metadata

#### 3. OIDC Trust Relationship
- **OIDC Provider**: Leverages EKS cluster's OIDC provider for secure authentication
- **Conditions**:
  - Service account must be `system:serviceaccount:monitoring:amp-remote-write`
  - Audience must be `sts.amazonaws.com`

### Outputs
- `amp_workspace_id`: Workspace identifier (ws-...)
- `amp_workspace_arn`: Full ARN for the workspace
- `amp_prometheus_query_endpoint`: Prometheus-compatible query API endpoint
- `amp_remote_write_url`: Remote write endpoint URL
- `amp_remote_write_irsa_role_arn`: IAM role ARN for service account annotation

### Usage
To integrate with EKS applications:
1. Create a service account in the `monitoring` namespace named `amp-remote-write`
2. Annotate it with: `eks.amazonaws.com/role-arn: <amp_remote_write_irsa_role_arn>`
3. Configure Prometheus or ADOT collector to use the remote write URL

## Amazon Managed Grafana (AMG)

### Overview
[Amazon Managed Grafana](https://aws.amazon.com/grafana/) is the managed Grafana control plane used here to **visualize** metrics. Terraform provisions a **workspace** with AWS data sources enabled (including **Amazon Managed Service for Prometheus**). Wiring that workspace to **your** AMP instance is completed once in the Grafana UI (or via Grafana’s API); Terraform creates the capability, not every dashboard or data-source row.

### Components Implemented

#### 1. Grafana workspace (`amg.tf`)
- **Resource**: `aws_grafana_workspace.main`
- **Name**: `${var.project_name}-${var.env}-amg`
- **Account access**: `CURRENT_ACCOUNT` (this AWS account only)
- **Permission type**: `SERVICE_MANAGED` — AWS creates and attaches the IAM policies needed for the selected **data sources** (for example Prometheus/AMP and CloudWatch), as described in [AMG permissions](https://docs.aws.amazon.com/grafana/latest/userguide/AMG-manage-permissions.html).
- **Data sources** (default, overridable via `var.amg_data_sources`): `PROMETHEUS`, `CLOUDWATCH`
- **Authentication** (default): `AWS_SSO` via `var.amg_authentication_providers` — users sign in with **IAM Identity Center** (formerly AWS SSO). **IAM Identity Center must be enabled** in the account before `terraform apply`, or creation can fail. If you do not use Identity Center, set `amg_authentication_providers = ["SAML"]` and complete SAML setup after create.

#### 2. Optional variables (`amg_variables.tf`)
- `amg_authentication_providers` — default `["AWS_SSO"]`; may include `SAML` as well per provider rules.
- `amg_data_sources` — default `["PROMETHEUS", "CLOUDWATCH"]`.
- `amg_grafana_version` — default `10.4` (adjust when upgrading).

### Outputs
- `amg_workspace_id` — Workspace ID (`g-...`), used when selecting AMP in Grafana.
- `amg_workspace_arn` — Workspace ARN.
- `amg_workspace_endpoint` — Full **HTTPS URL** to open the Grafana UI.
- `amg_grafana_version` — Running Grafana version.

### Linking AMG to AMP (after apply)

1. Ensure **ADOT** (or another agent) is **remote writing** metrics to the AMP workspace (`amp_remote_write_url` / IRSA).
2. Open **`amg_workspace_endpoint`** in a browser and sign in (Identity Center user assigned to this Grafana workspace, or SAML per your config).
3. In Grafana: **Connections** (or **AWS** data source setup) → add **Amazon Managed Service for Prometheus** / Prometheus — choose **region** = `var.region` and the workspace whose ID matches **`amp_workspace_id`** output.  
   See also: [Add AMP using AWS data source configuration](https://docs.aws.amazon.com/grafana/latest/userguide/AMP-adding-AWS-config.html).
4. **Assign Grafana roles**: In the **Amazon Managed Grafana** console, use **Authentication** / **AWS SSO** assignments to grant users or groups **Admin** or **Editor** on this workspace (Terraform does not assign SSO users by default).

### What Terraform does *not* do
- Does not create **dashboards**, **folders**, or **alert rules** in Grafana.
- Does not register **IAM Identity Center** or **SAML IdP** (account-level or IdP-specific work).
- Does not create `aws_grafana_role_association` (optional resource if you want to map SSO groups to Grafana roles in code); you can add that later with known SSO group IDs.

## CloudWatch Logging

### Overview
CloudWatch Logs provides centralized logging for our ECS tasks, enabling real-time monitoring, alerting, and analysis of application logs.

### Log Groups Implemented

#### 1. API Service Logs
- **Log Group**: `/ecs/narayana-api`
- **Retention**: 14 days
- **Purpose**: Collect logs from API ECS tasks
- **Tags**: Project and environment specific

#### 2. Worker Service Logs
- **Log Group**: `/ecs/narayana-worker`
- **Retention**: 14 days
- **Purpose**: Collect logs from background worker ECS tasks
- **Tags**: Project and environment specific

#### 3. Migration Task Logs
- **Log Group**: `/ecs/narayana-migrate`
- **Retention**: 14 days
- **Purpose**: Collect logs from database migration ECS tasks
- **Tags**: Project and environment specific

### Integration
These log groups are automatically used by ECS task definitions that specify `awslogs` log driver in their container definitions. The logs are streamed to CloudWatch in real-time, allowing for:
- Real-time log tailing
- Log filtering and searching
- Metric creation from log patterns
- Integration with CloudWatch Alarms

## IAM Roles and Permissions

### AMP Remote Write Role
- **Name**: `${var.project_name}-${var.env}-amp-remote-write`
- **Assume Role Policy**: Web identity federation with EKS OIDC
- **Attached Policy**: Custom policy allowing remote write operations to the specific AMP workspace
- **Scope**: Limited to the monitoring service account in EKS

### Security Considerations
- Least privilege principle: Only necessary permissions for remote write operations
- Scoped to specific workspace ARN to prevent cross-workspace access
- OIDC-based authentication eliminates need for long-term credentials

## Integration Points

### ECS Integration
- Log groups are referenced in ECS task definitions (not shown in Terraform but integrated at deployment)
- CloudWatch logs provide operational visibility for containerized applications

### EKS Integration
- AMP workspace ready for metrics collection from Kubernetes workloads
- IRSA setup enables secure communication between pods and AWS services
- Foundation for deploying Prometheus exporters, ADOT collector, or custom monitoring agents

### Cross-Service Monitoring
- Metrics from EKS can be correlated with logs from ECS
- Unified observability across container orchestration platforms

### AMG and AMP
- **AMP** stores time-series metrics (ingestion via ADOT remote write).
- **AMG** queries AMP for dashboards; with **CLOUDWATCH** enabled, the same workspace can also query CloudWatch metrics/logs where useful.

## Future Enhancements

### Potential Additions
1. **CloudWatch Alarms**: Automated alerting based on metrics and log patterns
2. **X-Ray Integration**: Distributed tracing for microservices
3. **Container Insights**: Enhanced monitoring for ECS and EKS clusters
4. **Log metric filters**: Convert log entries into CloudWatch metrics
5. **Custom dashboards**: Pre-built Grafana dashboards as code (e.g. JSON + GitOps)

### Implementation Considerations
- **Cost Optimization**: Monitor usage and adjust retention periods as needed
- **Security**: Implement proper IAM policies and network isolation
- **Scalability**: Design for multi-region or multi-account setups if required
- **Alerting**: Define SLIs/SLOs and implement appropriate alerting mechanisms

This observability foundation provides comprehensive monitoring capabilities for both ECS and EKS workloads, with secure authentication and scalable metrics storage.