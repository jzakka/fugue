# Architecture: Fugue

**Date**: 2026-03-29

---

## 전체 구조

```
┌──────────────────────────────────────────────────────────────────────┐
│                            AWS Cloud                                 │
│                                                                      │
│  ┌────────────────────────────────────────────────────────────────┐  │
│  │                     VPC: 10.0.0.0/16                           │  │
│  │                                                                │  │
│  │  ┌──────────────────────────────────────────────────────────┐  │  │
│  │  │  Public Subnet (AZ-a: 10.0.1.0/24, AZ-c: 10.0.2.0/24)  │  │  │
│  │  │                                                          │  │  │
│  │  │  - ALB (AWS Load Balancer Controller)                    │  │  │
│  │  │  - fck-nat (t4g.nano)                                    │  │  │
│  │  └──────────────────────────────────────────────────────────┘  │  │
│  │                                                                │  │
│  │  ┌──────────────────────────────────────────────────────────┐  │  │
│  │  │  Private Subnet - App (AZ-a: 10.0.10.0/24,              │  │  │
│  │  │                        AZ-c: 10.0.11.0/24)              │  │  │
│  │  │                                                          │  │  │
│  │  │  ┌────────────────────────────────────────────────────┐  │  │  │
│  │  │  │                  EKS Cluster                        │  │  │  │
│  │  │  │                                                    │  │  │  │
│  │  │  │  Node Group: app (Spot t3.medium × 2)              │  │  │  │
│  │  │  │  ┌──────────────────┐  ┌──────────────────┐       │  │  │  │
│  │  │  │  │ ns: dev          │  │ ns: prod         │       │  │  │  │
│  │  │  │  │  api (Go)        │  │  api (Go)        │       │  │  │  │
│  │  │  │  │  web (Next.js)   │  │  web (Next.js)   │       │  │  │  │
│  │  │  │  │  pg (CNPG)       │  │                  │       │  │  │  │
│  │  │  │  │  redis           │  │                  │       │  │  │  │
│  │  │  │  └──────────────────┘  └──────────────────┘       │  │  │  │
│  │  │  │                                                    │  │  │  │
│  │  │  │  Node Group: monitoring (Spot t3.large × 1)        │  │  │  │
│  │  │  │  ┌──────────────────────────────────────────┐     │  │  │  │
│  │  │  │  │ ns: monitoring                            │     │  │  │  │
│  │  │  │  │  Prometheus / Grafana / Loki / Tempo      │     │  │  │  │
│  │  │  │  │  Promtail (DaemonSet)                     │     │  │  │  │
│  │  │  │  │  OpenTelemetry Collector                   │     │  │  │  │
│  │  │  │  └──────────────────────────────────────────┘     │  │  │  │
│  │  │  │                                                    │  │  │  │
│  │  │  │  ┌──────────────────┐                             │  │  │  │
│  │  │  │  │ ns: argocd       │                             │  │  │  │
│  │  │  │  │  ArgoCD          │                             │  │  │  │
│  │  │  │  └──────────────────┘                             │  │  │  │
│  │  │  │                                                    │  │  │  │
│  │  │  └────────────────────────────────────────────────────┘  │  │  │
│  │  └──────────────────────────────────────────────────────────┘  │  │
│  │                                                                │  │
│  │  ┌──────────────────────────────────────────────────────────┐  │  │
│  │  │  Private Subnet - Data (AZ-a: 10.0.20.0/24,             │  │  │
│  │  │                         AZ-c: 10.0.21.0/24)             │  │  │
│  │  │                                                          │  │  │
│  │  │  - RDS PostgreSQL 16 (prod)                              │  │  │
│  │  │  - ElastiCache Redis (prod)                              │  │  │
│  │  └──────────────────────────────────────────────────────────┘  │  │
│  │                                                                │  │
│  │  Security Groups:                                              │  │
│  │  ┌────────────────┬──────────────────────────────┐            │  │
│  │  │ ALB SG         │ Inbound: 0.0.0.0 → 443      │            │  │
│  │  │ App Node SG    │ Inbound: ALB SG → 8080/3000  │            │  │
│  │  │ DB SG          │ Inbound: App Node SG → 5432  │            │  │
│  │  │ Redis SG       │ Inbound: App Node SG → 6379  │            │  │
│  │  └────────────────┴──────────────────────────────┘            │  │
│  │                                                                │  │
│  └────────────────────────────────────────────────────────────────┘  │
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │     ECR      │  │     S3       │  │  Route 53    │              │
│  │ (컨테이너    │  │ (OG 이미지   │  │  (DNS)       │              │
│  │  레지스트리)  │  │   캐시)      │  │              │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 기술 스택

### Application

| 계층 | 기술 | 이유 |
|------|------|------|
| Frontend | Next.js 15 (App Router) | SSR로 작품/프로필 페이지 OG 태그 생성. React 생태계 |
| Backend | Go + Chi (router) | K8s 네이티브. 빠른 빌드, 작은 이미지 (~20MB) |
| ORM | sqlc | SQL 직접 작성 → Go 코드 자동생성 |
| DB (prod) | RDS PostgreSQL 16 | 태그 매칭 (ARRAY + GIN 인덱스), pgvector 확장 가능 |
| DB (dev) | CloudNativePG (K8s 위) | K8s Operator 패턴 학습 |
| Cache (prod) | ElastiCache Redis | OG 메타데이터 캐시, 세션 |
| Cache (dev) | Redis (K8s 위) | dev 환경 자체 완결 |
| Auth | OAuth 2.0 (Twitter/Discord) | 타겟 유저의 주 SNS |

### Infrastructure (Terraform)

| 리소스 | 기술 | 이유 |
|--------|------|------|
| Network | VPC, 3-Tier Subnet, fck-nat | 망분리 학습. NAT 원리 이해 |
| Compute | EKS (Managed Node Group, Spot) | K8s 운영 + Spot 중단 대응 학습 |
| Registry | ECR | AWS 네이티브, EKS IAM 통합 |
| Ingress | AWS Load Balancer Controller | ALB를 K8s Ingress로 관리 |
| DNS | Route 53 | 도메인 관리 |
| TLS | ACM | 무료 인증서, ALB 연동 |
| IAM ↔ K8s | IRSA (IAM Roles for Service Accounts) | Pod 단위 AWS 권한. Access Key 불필요 |

### CI/CD

| 단계 | 도구 | 설명 |
|------|------|------|
| CI | GitHub Actions | 테스트 → Docker 빌드 → ECR 푸시 → values.yaml 업데이트 커밋 |
| CD | ArgoCD | GitOps. Git manifest 변경 감지 → K8s 배포 |
| IaC | Terraform | 전체 인프라 코드화 |
| Manifest | Helm | 환경별 values 파일로 dev/prod 분리 |

### ArgoCD Sync Policy

| 환경 | Auto Sync | Self-Heal | Prune |
|------|-----------|-----------|-------|
| dev | O | O | O |
| prod | X (수동 승인) | O | O |

### Observability (Grafana 스택 통일)

| 영역 | 도구 | 데이터 흐름 |
|------|------|------------|
| Metrics | Prometheus + Grafana | Go API `/metrics` → Prometheus 스크래핑 → Grafana |
| Logs | Loki + Promtail + Grafana | 컨테이너 stdout → Promtail (DaemonSet) → Loki → Grafana |
| Traces | Tempo + OpenTelemetry + Grafana | Go API (OTel SDK) → OTel Collector → Tempo → Grafana |
| Alerting | Grafana Alerting | 메트릭/로그 기반 알림 |

---

## EKS 클러스터 구성

### Node Groups

| Node Group | 인스턴스 | 수량 | 용도 | Spot 전략 |
|------------|----------|------|------|-----------|
| app | t3.medium (2vCPU, 4GB) | 2 | Go API, Next.js, dev 환경 DB/Redis | 다중 인스턴스 타입 폴백 (t3.medium, t3a.medium, m5.large) |
| monitoring | t3.large (2vCPU, 8GB) | 1 | Prometheus, Grafana, Loki, Tempo, ArgoCD | 다중 인스턴스 타입 폴백 |

### Namespace 구성

| Namespace | 워크로드 | 스케줄링 |
|-----------|---------|---------|
| dev | api, web, postgresql (CNPG), redis | app Node Group (Node Selector) |
| prod | api, web | app Node Group (Node Selector) |
| monitoring | Prometheus, Grafana, Loki, Tempo, Promtail, OTel Collector | monitoring Node Group (Taint & Toleration) |
| argocd | ArgoCD | monitoring Node Group |

### 환경별 DB/Cache 구성

| | dev | prod |
|---|-----|------|
| DB | CloudNativePG (K8s 내부) | RDS PostgreSQL (Data Subnet) |
| Cache | Redis Pod (K8s 내부) | ElastiCache Redis (Data Subnet) |

---

## 네트워크 설계

### VPC & Subnet

```
VPC: 10.0.0.0/16

AZ-a:
  Public:  10.0.1.0/24   (ALB, fck-nat)
  Private: 10.0.10.0/24  (EKS Nodes)
  Data:    10.0.20.0/24  (RDS, ElastiCache)

AZ-c:
  Public:  10.0.2.0/24   (ALB)
  Private: 10.0.11.0/24  (EKS Nodes)
  Data:    10.0.21.0/24  (RDS Standby)
```

### 트래픽 흐름

```
[인바운드]  사용자 → Route53 → ALB (Public) → Pod (Private App)
[내부]      Pod (Private App) → RDS/ElastiCache (Private Data)
[아웃바운드] Pod (Private App) → fck-nat (Public) → 외부 API (OG fetch, OAuth)
```

### Security Group 체이닝

```
ALB SG        ← Inbound: 0.0.0.0/0 → 443
App Node SG   ← Inbound: ALB SG → 8080, 3000
DB SG         ← Inbound: App Node SG → 5432
Redis SG      ← Inbound: App Node SG → 6379
```

---

## CI/CD 파이프라인

```
git push (main)
    │
    ▼
GitHub Actions (CI)
    ├── Stage 1: Test
    │   ├── go test ./...
    │   ├── golangci-lint
    │   └── npm test + eslint
    │
    ├── Stage 2: Build & Push
    │   ├── Docker build (multi-stage)
    │   │   Go API  → ECR: fugue-api:sha-abc123
    │   │   Next.js → ECR: fugue-web:sha-abc123
    │   └── 이미지 태그 = git commit SHA
    │
    └── Stage 3: Manifest Update
        └── Helm values의 image tag 업데이트 → Git commit & push
                │
                │ ArgoCD가 Git 변경 감지
                ▼
            ArgoCD (CD)
                │
                │ dev: 자동 Sync
                │ prod: 수동 승인 후 Sync
                ▼
              EKS 배포
```

---

## 모노레포 구조

```
fugue/
├── apps/
│   ├── api/                          # Go Backend
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── config/
│   │   │   ├── handler/
│   │   │   ├── service/
│   │   │   ├── repository/
│   │   │   ├── model/
│   │   │   └── middleware/
│   │   ├── db/
│   │   │   ├── migrations/
│   │   │   ├── queries/
│   │   │   └── sqlc.yaml
│   │   ├── Dockerfile
│   │   └── go.mod
│   │
│   └── web/                          # Next.js Frontend
│       ├── src/
│       ├── Dockerfile
│       └── package.json
│
├── helm/
│   └── fugue/
│       ├── Chart.yaml
│       ├── values.yaml               # 기본값
│       ├── values-dev.yaml
│       ├── values-prod.yaml
│       └── templates/
│           ├── api-deployment.yaml
│           ├── api-service.yaml
│           ├── web-deployment.yaml
│           ├── web-service.yaml
│           ├── ingress.yaml
│           ├── configmap.yaml
│           ├── secret.yaml
│           └── hpa.yaml
│
├── terraform/
│   ├── modules/
│   │   ├── vpc/
│   │   ├── eks/
│   │   ├── ecr/
│   │   ├── rds/
│   │   ├── elasticache/
│   │   ├── fck-nat/
│   │   └── route53/
│   └── environments/
│       └── prod/
│           ├── main.tf
│           ├── variables.tf
│           └── terraform.tfvars
│
├── .github/
│   └── workflows/
│       ├── ci-api.yml
│       ├── ci-web.yml
│       └── deploy.yml
│
├── docker-compose.yml                # 로컬 개발용
└── README.md
```

---

## 월 예상 비용

| 항목 | 비용 |
|------|------|
| EKS Control Plane | $74 |
| EC2 Spot - app (t3.medium × 2) | ~$24 |
| EC2 Spot - monitoring (t3.large × 1) | ~$25 |
| fck-nat (t4g.nano) | ~$3 |
| RDS (db.t3.micro, 단일 AZ) | ~$15 |
| ElastiCache (cache.t3.micro) | ~$12 |
| Route 53 | ~$1 |
| ECR / S3 / 데이터 전송 | ~$5 |
| **합계** | **~$159/월** |
