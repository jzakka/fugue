# 기술 스택

## Application

| 계층 | 기술 |
|------|------|
| Frontend | Next.js 16 (App Router) |
| Backend | Go + Chi router |
| ORM | sqlc |
| DB | PostgreSQL 16 |
| Cache | Redis |
| Auth | OAuth 2.0 (Google, Discord, Twitter) |

## Infrastructure

| 항목 | 기술 |
|------|------|
| IaC | Terraform |
| Container | EKS (Managed Node Group, Spot) |
| Network | VPC 3-Tier (Public / Private-App / Private-Data), fck-nat |
| Registry | ECR |
| Ingress | AWS Load Balancer Controller |
| DNS/TLS | Route 53 + ACM |
| DB (prod) | RDS PostgreSQL |
| Cache (prod) | ElastiCache Redis |
| DB (dev) | CloudNativePG (K8s) |
| Event Ingestion | Kinesis Data Firehose |
| Event Storage | S3 (Parquet, 날짜 파티셔닝) |
| Event Query | Athena |
| IAM↔K8s | IRSA |

## CI/CD

| 단계 | 기술 |
|------|------|
| CI | GitHub Actions |
| CD | ArgoCD (GitOps) |
| Manifest | Helm |
| 환경 분리 | 같은 클러스터 내 namespace (dev/prod) |

## Observability

| 영역 | 기술 |
|------|------|
| Metrics | Prometheus + Grafana |
| Logs | Loki + Promtail |
| Traces | Tempo + OpenTelemetry |
| Alerting | Grafana Alerting |
