# Observability

This document is the single reference for **metrics, dashboards, and related logs** in the Nexoraa platform: how the stack is wired, how to run it locally and on EKS, and **where configuration and secrets come from**.

---

## How it works (high level)

| Surface | Stack | Purpose |
|--------|--------|--------|
| **Local dev** | Docker Compose | Prometheus + Grafana scrape **api** and **worker** from the compose network. Good for iterating on app metrics and dashboards. |
| **EKS** | [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) (Helm) | Prometheus Operator runs **Prometheus**, **Grafana**, **node-exporter**, **kube-state-metrics**, and default Kubernetes alert rules. Nexoraa apps are scraped via **ServiceMonitor** CRDs. |

**Design choice:** metrics and dashboards are **self-hosted on the cluster** (Prometheus + Grafana), not Amazon Managed Prometheus / Managed Grafana.

**Logs (ECS path):** if you use the Terraform-defined ECS tasks, application **stdout/stderr** go to **CloudWatch Logs** (see [CloudWatch log groups](#cloudwatch-log-groups-ecs)). That is separate from Prometheus metrics.

---

## Repository layout (what matters)

| Path | Role |
|------|------|
| `docker-compose.yml` | Local Prometheus, Grafana, and app services. |
| `deploy/prometheus.yml` | Local Prometheus **scrape_config** (static targets: `api:8080`, `worker:9090`). |
| `deploy/grafana/datasources/prometheus.yml` | Local Grafana datasource (URL `http://prometheus:9090`). |
| `deploy/grafana/dashboards/*.json` | Dashboard definitions (used locally **and** packaged for EKS — see below). |
| `deploy/grafana/dashboards/dashboard.yml` | Local Grafana **provisioning** (file provider for dashboards). |
| `deploy/alerting_rules.yml` | Prometheus **alerting** rules for the local Docker Prometheus container. |
| `k8s/base/monitoring/prometheusrule.yaml` | Kubernetes `PrometheusRule` for the same execution-engine alerts on EKS. |
| `k8s/helm/kube-prometheus-stack/values.yaml` | Shared Helm values (Prometheus scrape selectors, Grafana sidecar mounts, components on/off). |
| `k8s/helm/kube-prometheus-stack/values-{dev,staging,prod}.yaml` | Per-environment overrides (retention, persistence). |
| `scripts/install-monitoring.sh` | Creates the dashboards ConfigMap and runs `helm upgrade --install` for kube-prometheus-stack. |
| `scripts/setup-after-terraform-apply.sh` | Post-Terraform flow: `kubectl` context from Terraform outputs → namespaces → `install-monitoring.sh` → optional Kustomize overlay. |
| `k8s/base/api/servicemonitor.yaml` | Tells Prometheus Operator to scrape the API `/metrics` on port `http`. |
| `k8s/base/temporal-worker/servicemonitor.yaml` | Scrapes Temporal worker `/metrics` on port `metrics`. |

---

## EKS: install and upgrade

### Recommended (after Terraform created the cluster)

From the **repo root** (Terraform state must expose `eks_cluster_name` and `region`):

```bash
./scripts/setup-after-terraform-apply.sh dev
```

- Sets `kubectl` via `aws eks update-kubeconfig` using outputs from `terraform/` (override with `TERRAFORM_DIR` if your root differs).
- Applies `k8s/base/namespace.yaml`.
- Runs `./scripts/install-monitoring.sh <env>`.
- Applies `kubectl kustomize --load-restrictor=LoadRestrictionsNone k8s/overlays/<env>/ | kubectl apply -f -` (apps + ServiceMonitors + PrometheusRules), unless you pass **`--skip-kustomize`** (e.g. when Argo CD applies workloads).

Staging / production Grafana password (shell env — **do not commit**):

```bash
export GRAFANA_ADMIN_PASSWORD='your-strong-secret'
./scripts/setup-after-terraform-apply.sh prod
```

### Monitoring-only (kubectl already points at the cluster)

```bash
./scripts/install-monitoring.sh staging
```

Helm merges **`values.yaml`** then **`values-<env>.yaml`**. Chart version is pinned in the script; override with:

```bash
export KPS_CHART_VERSION=67.2.0
./scripts/install-monitoring.sh prod
```

### What gets installed (namespace `monitoring`)

| Component | Typical role |
|-----------|----------------|
| Prometheus | TSDB + scraping (cluster + apps). |
| Grafana | Dashboards; chart defaults **plus** Nexoraa JSON from ConfigMap (folder **Nexoraa**). |
| node-exporter | Node CPU, memory, disk, etc. |
| kube-state-metrics | Kubernetes object metrics (pods, nodes, …). |

Prometheus is configured so **ServiceMonitors / PodMonitors / PrometheusRules** are discovered across namespaces and without requiring the Helm release label (`serviceMonitorNamespaceSelector: {}`, `ruleNamespaceSelector: {}`, `serviceMonitorSelectorNilUsesHelmValues: false`, etc. in `values.yaml`).

### Nexoraa dashboards on EKS

1. `install-monitoring.sh` builds ConfigMap **`nexoraa-grafana-dashboards`** in `monitoring` from every `*.json` file under `deploy/grafana/dashboards/` (execution-engine dashboards, **Cluster observability — Overview**, **EKS node health**, **Kubernetes pod counts**, etc.).
2. Helm values mount that ConfigMap at `/var/lib/grafana/dashboards/nexoraa` and register a **dashboard provider** named `nexoraa` (folder **Nexoraa** in Grafana).

| Dashboard file | Grafana title | Primary use |
|----------------|---------------|-------------|
| `cluster-observability-overview.json` | Cluster observability — Overview | Self-hosted Prometheus/Grafana health, scrape performance, target `up`, TSDB pressure, monitoring pods (replaces AMP ingestion health on EKS). |
| `eks-node-health.json` | EKS — Node health | Per-node CPU, memory, disk, readiness. |
| `k8s-pod-count.json` | Kubernetes — Pod counts | Pod phases and counts by namespace. |
| `overview.json`, `performance.json`, `event-pipeline.json` | Execution engine | Application metrics. |

After editing JSON locally, **re-run** `install-monitoring.sh` (or recreate the ConfigMap the same way) so the cluster picks up changes.

### Scraping Nexoraa workloads on EKS

ServiceMonitors live under `k8s/base/` and are placed into the target namespace by each overlay (`nexoraa-dev` for dev, `nexoraa-system` for staging/prod). The overlay must be applied so Services expose the ports Prometheus scrapes:

| Workload | Path | Port name |
|----------|------|-----------|
| API | `/metrics` | `http` (8080) |
| Temporal worker | `/metrics` | `metrics` (9090) |

```bash
kubectl kustomize --load-restrictor=LoadRestrictionsNone k8s/overlays/dev/ | kubectl apply -f -
```

### Access (port-forward)

```bash
# Grafana (admin user; password from GRAFANA_ADMIN_PASSWORD / Helm values)
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3001:80

# Prometheus UI
kubectl port-forward -n monitoring svc/kube-prometheus-stack-kube-prometheus-prometheus 9091:9090
```

### Uninstall

```bash
helm uninstall kube-prometheus-stack -n monitoring
kubectl delete configmap nexoraa-grafana-dashboards -n monitoring --ignore-not-found
```

---

## Environment-specific Helm values (where numbers come from)

| Concern | Source |
|---------|--------|
| Shared defaults | `k8s/helm/kube-prometheus-stack/values.yaml` — scrape intervals, Grafana admin **placeholder** (`changeme`), Nexoraa dashboard mount, node-exporter / kube-state-metrics on, default recording/alert rules from the chart. |
| Dev | `values-dev.yaml` — shorter retention, **no** Prometheus/Grafana PVCs. |
| Staging | `values-staging.yaml` — PVCs (e.g. 20Gi TSDB, 5Gi Grafana), Alertmanager disabled. |
| Prod | `values-prod.yaml` — larger retention/storage, higher Prometheus resources, Grafana PVC 10Gi, Alertmanager disabled. Optional **existingSecret** for Grafana admin is commented for you to enable. |

At install time, **`install-monitoring.sh`** passes `--set grafana.adminPassword=...` from the environment (see next section), overriding the placeholder in values for that release.

---

## Environment variables and secrets (where values are taken from)

### Shell environment (EKS / Helm scripts)

Scripts **do not** auto-load a `.env` file. Export variables in your shell, use your CI secret store, or `source` a **local** file that is gitignored.

| Variable | Used by | Meaning |
|----------|---------|--------|
| `GRAFANA_ADMIN_PASSWORD` | `install-monitoring.sh`, `setup-after-terraform-apply.sh` | Grafana `admin` password. Defaults to **`changeme`** if unset (script warns in dev). **Set explicitly for staging/prod.** |
| `KPS_CHART_VERSION` | `install-monitoring.sh` | kube-prometheus-stack Helm chart version (default pinned in script). |
| `TERRAFORM_DIR` | `setup-after-terraform-apply.sh` | Terraform root; default `<repo>/terraform`. |
| `DB_PASSWORD` | `bootstrap-cluster.sh` | RDS / Temporal bootstrap when using that script — not used by Grafana install. |

### AWS and Terraform

| What | Where |
|------|--------|
| AWS credentials | `AWS_PROFILE`, `aws configure`, or CI role — **not** in repo files. |
| Cluster / region for `kubectl` | Terraform outputs `eks_cluster_name`, `read` by `setup-after-terraform-apply.sh`. |
| Terraform workspace / env | Your usual `-var="env=..."` or `*.tfvars`; should match the **first argument** to the setup script (`dev` / `staging` / `prod`) so the correct Helm values and Kustomize overlay are used. |

### Local Docker Compose

Compose **automatically reads** a file named `.env` in the same directory as `docker-compose.yml` for [variable substitution](https://docs.docker.com/compose/environment-variables/set-environment-variables/).

- **Template (committed):** `.env.example` — safe example keys.
- **Secrets (gitignored):** `.env` — copy from `.env.example` and set e.g. `GF_SECURITY_ADMIN_PASSWORD` for Grafana.

Relevant keys include `GF_SECURITY_ADMIN_PASSWORD` (see `.env.example`). Other compose services may still use **hardcoded** dev credentials in `docker-compose.yml`; tighten those separately if needed.

---

## Local development (Docker Compose)

```bash
make docker-up
```

- **Prometheus:** `http://localhost:19091` (config: `deploy/prometheus.yml`).
- **Grafana:** `http://localhost:13001` (provisioning: `deploy/grafana/`).

Docker Compose maps app and observability ports to **18xxx / 19xxx / 13xxx** on the host so they do not collide with `8080`–`8082` or with `kubectl port-forward` defaults (`3001`, `9091`). See the port comment block at the top of `docker-compose.yml`.

Default Grafana admin password is **`admin`** unless overridden via `.env` using `GF_SECURITY_ADMIN_PASSWORD` (see the `grafana` service in `docker-compose.yml`).

---

## CloudWatch log groups (ECS)

Defined in Terraform `terraform/cloudwatch.tf`:

| Log group | Retention |
|-----------|-----------|
| `/ecs/narayana-api` | 14 days |
| `/ecs/narayana-worker` | 14 days |
| `/ecs/narayana-migrate` | 14 days |

ECS task definitions in `terraform/ecs.tf` reference these groups for `awslogs`.

---

## Grafana datasource UIDs (dashboards)

JSON dashboards use Prometheus datasource **`uid: "prometheus"`**. The kube-prometheus-stack Grafana chart creates that by default. Local provisioning (`deploy/grafana/datasources/prometheus.yml`) sets the same **`uid: prometheus`** so dashboard JSON works in Docker Compose without edits.

---

## Alert rules

`deploy/alerting_rules.yml` contains Prometheus **alert** definitions for the execution engine (failure rate, worker up, latency, outbox backlog). Local Docker Compose mounts this file into Prometheus through `deploy/prometheus.yml`.

On EKS, the same alert set is applied as `k8s/base/monitoring/prometheusrule.yaml` by the dev, staging, and prod Kustomize overlays. Alertmanager is disabled, so no mail/pager routing is sent. Rule violations still appear in Prometheus and through Grafana panels that query the `ALERTS` metric.

---

## Quick health checklist

1. **EKS:** `kubectl get pods -n monitoring` — Prometheus, Grafana, operator, node-exporter, kube-state-metrics healthy.  
2. **Targets:** Prometheus UI → **Status → Targets** — `nexoraa-api` and `nexoraa-temporal-worker` (or equivalent job names) **UP** after overlay apply.  
3. **Grafana:** Folder **Nexoraa** lists bundled dashboards; open **Cluster observability — Overview** for Prometheus scrape/target health; Explore → Prometheus runs a simple query (e.g. `up`).  
4. **Local:** `up{job="api"}` and `up{job="worker"}` in Prometheus after `make docker-up`.

If something fails, confirm **namespace** and **Service** port names match the ServiceMonitor (`nexoraa-system`, ports `http` / `metrics`).
