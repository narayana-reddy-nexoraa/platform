# -----------------------------------------------------------------------------
# Amazon Managed Prometheus (AMP) — basic workspace + EKS IRSA for remote_write
# Create ServiceAccount in-cluster: monitoring/amp-remote-write with annotation
#   eks.amazonaws.com/role-arn = <output amp_remote_write_irsa_role_arn>
# -----------------------------------------------------------------------------

resource "aws_prometheus_workspace" "main" {
  alias = "${var.project_name}-${var.env}"

  tags = {
    Name        = "${var.project_name}-${var.env}-amp"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}

data "aws_iam_policy_document" "amp_remote_write_assume" {
  statement {
    actions = ["sts:AssumeRoleWithWebIdentity"]
    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.eks.arn]
    }
    condition {
      test     = "StringEquals"
      variable = "${replace(aws_eks_cluster.main.identity[0].oidc[0].issuer, "https://", "")}:sub"
      values   = ["system:serviceaccount:monitoring:amp-remote-write"]
    }
    condition {
      test     = "StringEquals"
      variable = "${replace(aws_eks_cluster.main.identity[0].oidc[0].issuer, "https://", "")}:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "amp_remote_write" {
  name               = "${var.project_name}-${var.env}-amp-remote-write"
  assume_role_policy = data.aws_iam_policy_document.amp_remote_write_assume.json

  tags = {
    Name        = "${var.project_name}-${var.env}-amp-remote-write"
    Project     = var.project_name
    Environment = var.env
    ManagedBy   = "terraform"
  }
}

data "aws_iam_policy_document" "amp_remote_write" {
  statement {
    sid    = "RemoteWriteToWorkspace"
    effect = "Allow"
    actions = [
      "aps:RemoteWrite",
      "aps:GetSeries",
      "aps:GetLabels",
      "aps:GetMetricMetadata",
    ]
    resources = [aws_prometheus_workspace.main.arn]
  }
}

resource "aws_iam_role_policy" "amp_remote_write" {
  name   = "${var.project_name}-${var.env}-amp-remote-write"
  role   = aws_iam_role.amp_remote_write.id
  policy = data.aws_iam_policy_document.amp_remote_write.json
}
