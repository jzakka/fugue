# Architecture: Fugue — Creation-Based Collaborative Matching Platform

**Date**: 2026-03-29
**Product Stage**: New (Toy Project)
**Mascot**: A pufferfish (fugu) wearing headphones and holding a paintbrush — a wordplay on Fugue/Fugu

---

## 1. Architecture Overview

### System Architecture Diagram

```
┌──────────────────────────────────────────────────────────────────────┐
│                            Client                                    │
│                                                                      │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐              │
│  │   Browser    │    │   Mobile    │    │   PWA       │              │
│  │   (React)    │    │   (Web)     │    │             │              │
│  └──────┬───────┘    └──────┬──────┘    └──────┬──────┘              │
│         └──────────────────┼──────────────────┘                      │
│                            │                                         │
└────────────────────────────┼─────────────────────────────────────────┘
                             │ HTTPS
                             v
┌──────────────────────────────────────────────────────────────────────┐
│                         AWS Cloud                                    │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────┐        │
│  │                    VPC (10.0.0.0/16)                      │        │
│  │                                                           │        │
│  │  ┌─────────────────────────────────────────────────────┐  │        │
│  │  │              Public Subnet (10.0.1.0/24)            │  │        │
│  │  │                                                     │  │        │
│  │  │  ┌──────────┐    ┌──────────────────────────────┐   │  │        │
│  │  │  │   ALB    │    │   NAT Gateway                │   │  │        │
│  │  │  │          │    │                              │   │  │        │
│  │  │  └────┬─────┘    └──────────────────────────────┘   │  │        │
│  │  │       │                                             │  │        │
│  │  └───────┼─────────────────────────────────────────────┘  │        │
│  │          │                                                │        │
│  │  ┌───────┼─────────────────────────────────────────────┐  │        │
│  │  │       v        Private Subnet (10.0.2.0/24)         │  │        │
│  │  │                                                     │  │        │
│  │  │  ┌──────────────────────────────────────────────┐   │  │        │
│  │  │  │           EKS Cluster (Fugue)                │   │  │        │
│  │  │  │                                              │   │  │        │
│  │  │  │  ┌─────────────┐    ┌─────────────┐         │   │  │        │
│  │  │  │  │  fugue-api  │    │  fugue-web  │         │   │  │        │
│  │  │  │  │  (NestJS)   │    │  (Next.js)  │         │   │  │        │
│  │  │  │  │  Pod x2     │    │  Pod x2     │         │   │  │        │
│  │  │  │  └──────┬──────┘    └─────────────┘         │   │  │        │
│  │  │  │         │                                    │   │  │        │
│  │  │  └─────────┼────────────────────────────────────┘   │  │        │
│  │  │            │                                        │  │        │
│  │  │  ┌─────────v────────────────────────────────────┐   │  │        │
│  │  │  │         Data Layer                           │   │  │        │
│  │  │  │                                              │   │  │        │
│  │  │  │  ┌─────────────┐    ┌─────────────┐         │   │  │        │
│  │  │  │  │ PostgreSQL  │    │    Redis     │         │   │  │        │
│  │  │  │  │  (RDS)      │    │ (ElastiCache)│        │   │  │        │
│  │  │  │  └─────────────┘    └─────────────┘         │   │  │        │
│  │  │  │                                              │   │  │        │
│  │  │  └──────────────────────────────────────────────┘   │  │        │
│  │  │                                                     │  │        │
│  │  └─────────────────────────────────────────────────────┘  │        │
│  │                                                           │        │
│  └───────────────────────────────────────────────────────────┘        │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────┐        │
│  │                  Managed Services                         │        │
│  │                                                           │        │
│  │  ┌─────────────┐  ┌──────────┐  ┌─────────────────────┐  │        │
│  │  │    ECR      │  │   S3     │  │   CloudFront        │  │        │
│  │  │ (Container  │  │ (Static  │  │   (CDN)             │  │        │
│  │  │  Registry)  │  │  Assets) │  │                     │  │        │
│  │  └─────────────┘  └──────────┘  └─────────────────────┘  │        │
│  │                                                           │        │
│  │  ┌─────────────┐  ┌──────────┐  ┌─────────────────────┐  │        │
│  │  │   Route 53  │  │   ACM    │  │   CloudWatch        │  │        │
│  │  │   (DNS)     │  │ (TLS)    │  │   (Monitoring)      │  │        │
│  │  └─────────────┘  └──────────┘  └─────────────────────┘  │        │
│  │                                                           │        │
│  └───────────────────────────────────────────────────────────┘        │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### Architecture Principles

| Principle | Description |
|-----------|-------------|
| External link-based | No file storage. Works are referenced via external links (SoundCloud, pixiv, YouTube, etc.) |
| Stateless API | All state is stored in PostgreSQL/Redis. API pods are stateless and horizontally scalable |
| Monorepo | Frontend and backend in a single repository for easy management |
| Infrastructure as Code | All infrastructure managed via Terraform |
| Container-based | All services run as containers on EKS |

## 2. Tech Stack

### Frontend

| Category | Technology | Reason |
|----------|-----------|--------|
| Framework | Next.js 14 (App Router) | SSR/SSG support, React ecosystem |
| Language | TypeScript | Type safety |
| Styling | Tailwind CSS | Rapid prototyping, utility-first |
| State Management | Zustand | Lightweight, simple API |
| Data Fetching | TanStack Query (React Query) | Caching, optimistic updates |
| UI Components | Radix UI + custom | Accessible, unstyled primitives |
| OG Embed | react-player, custom OG card | SoundCloud/YouTube embed, OG metadata preview |

### Backend

| Category | Technology | Reason |
|----------|-----------|--------|
| Framework | NestJS | Modular architecture, TypeScript native |
| Language | TypeScript | Shared types with frontend |
| ORM | Prisma | Type-safe DB access, migration management |
| Authentication | Passport.js (OAuth 2.0) | Google, Twitter OAuth |
| Session | JWT (access + refresh token) | Stateless authentication |
| Validation | class-validator + class-transformer | DTO validation |
| API Docs | Swagger (OpenAPI) | Auto-generated API documentation |
| OG Fetcher | open-graph-scraper | Fetch OG metadata from external links |

### Database

| Category | Technology | Reason |
|----------|-----------|--------|
| Primary DB | PostgreSQL 15 (RDS) | Relational data, full-text search |
| Cache | Redis 7 (ElastiCache) | Session cache, OG metadata cache |
| Migrations | Prisma Migrate | Schema versioning |

### Infrastructure

| Category | Technology | Reason |
|----------|-----------|--------|
| Cloud | AWS | Broad service offering, free tier available |
| Container Orchestration | EKS (Kubernetes) | Container management, auto-scaling |
| Container Registry | ECR | AWS-native, EKS integration |
| IaC | Terraform | Declarative infrastructure, state management |
| CI/CD | GitHub Actions | GitHub-native, free for public repos |
| DNS | Route 53 | AWS-native DNS management |
| TLS | ACM (AWS Certificate Manager) | Free TLS certificates |
| CDN | CloudFront | Static asset distribution, HTTPS termination |
| Monitoring | CloudWatch | AWS-native logging and metrics |
| Static Assets | S3 | Frontend static build output hosting |

## 3. Data Model

### ER Diagram

```
┌─────────────────────┐       ┌─────────────────────┐
│       User          │       │       Work          │
├─────────────────────┤       ├─────────────────────┤
│ id          (PK)    │──┐    │ id          (PK)    │
│ email               │  │    │ userId      (FK)    │──┐
│ displayName         │  │    │ title               │  │
│ bio                 │  │    │ url                 │  │
│ avatarUrl           │  │    │ platform            │  │
│ oauthProvider       │  │    │ ogTitle             │  │
│ oauthProviderId     │  │    │ ogDescription       │  │
│ createdAt           │  │    │ ogImage             │  │
│ updatedAt           │  │    │ field               │  │
│                     │  │    │ license             │  │
└─────────────────────┘  │    │ createdAt           │  │
                         │    │ updatedAt           │  │
┌─────────────────────┐  │    └─────────────────────┘  │
│    UserRole         │  │                             │
├─────────────────────┤  │    ┌─────────────────────┐  │
│ id          (PK)    │  │    │     WorkTag         │  │
│ userId      (FK)    │──┘    ├─────────────────────┤  │
│ role                │       │ id          (PK)    │  │
│                     │       │ workId      (FK)    │──┘
└─────────────────────┘       │ tag                 │
                              │                     │
┌─────────────────────┐       └─────────────────────┘
│   UserSns           │
├─────────────────────┤       ┌─────────────────────┐
│ id          (PK)    │       │  Recommendation     │
│ userId      (FK)    │──┐    ├─────────────────────┤
│ platform            │  │    │ id          (PK)    │
│ url                 │  │    │ sourceWorkId (FK)   │
│                     │  │    │ targetWorkId (FK)   │
└─────────────────────┘  │    │ projectType         │
                         │    │ matchingTags        │
                         │    │ score               │
                         │    │ clickedAt           │
                         │    │ createdAt           │
                         │    └─────────────────────┘
                         │
                         │    ┌─────────────────────┐
                         │    │  ProfileView        │
                         │    ├─────────────────────┤
                         │    │ id          (PK)    │
                         └───>│ viewerId    (FK)    │
                              │ viewedId    (FK)    │
                              │ source              │
                              │ createdAt           │
                              └─────────────────────┘
```

### Table Specifications

#### User

| Column | Type | Constraint | Description |
|--------|------|-----------|-------------|
| id | UUID | PK, auto-generated | User ID |
| email | VARCHAR(255) | UNIQUE, NOT NULL | Email (from OAuth) |
| displayName | VARCHAR(20) | NOT NULL | Display name |
| bio | VARCHAR(100) | NULLABLE | One-line bio |
| avatarUrl | TEXT | NULLABLE | Profile image URL (from OAuth) |
| oauthProvider | VARCHAR(20) | NOT NULL | OAuth provider (google, twitter) |
| oauthProviderId | VARCHAR(255) | NOT NULL | OAuth provider user ID |
| createdAt | TIMESTAMP | NOT NULL, DEFAULT NOW | Creation date |
| updatedAt | TIMESTAMP | NOT NULL, DEFAULT NOW | Last updated date |

#### Work

| Column | Type | Constraint | Description |
|--------|------|-----------|-------------|
| id | UUID | PK, auto-generated | Work ID |
| userId | UUID | FK -> User.id, NOT NULL | Poster's user ID |
| title | VARCHAR(100) | NOT NULL | Work title (from OG or manual input) |
| url | TEXT | NOT NULL | External link URL |
| platform | VARCHAR(20) | NOT NULL | Detected platform (soundcloud, pixiv, youtube, twitter, other) |
| ogTitle | VARCHAR(255) | NULLABLE | OG meta title |
| ogDescription | TEXT | NULLABLE | OG meta description |
| ogImage | TEXT | NULLABLE | OG meta image URL |
| field | VARCHAR(20) | NOT NULL | Field tag (music, illustration, video, 3d, sound, vocals, writing) |
| license | VARCHAR(50) | NOT NULL, DEFAULT 'credit' | License type |
| createdAt | TIMESTAMP | NOT NULL, DEFAULT NOW | Creation date |
| updatedAt | TIMESTAMP | NOT NULL, DEFAULT NOW | Last updated date |

#### WorkTag

| Column | Type | Constraint | Description |
|--------|------|-----------|-------------|
| id | UUID | PK, auto-generated | Tag ID |
| workId | UUID | FK -> Work.id, NOT NULL | Work ID |
| tag | VARCHAR(30) | NOT NULL | Style/mood tag |

#### UserRole

| Column | Type | Constraint | Description |
|--------|------|-----------|-------------|
| id | UUID | PK, auto-generated | Role ID |
| userId | UUID | FK -> User.id, NOT NULL | User ID |
| role | VARCHAR(30) | NOT NULL | Role tag (composer, illustrator, video_editor, 3d_artist, sound_designer, vocalist, writer) |

#### UserSns

| Column | Type | Constraint | Description |
|--------|------|-----------|-------------|
| id | UUID | PK, auto-generated | SNS ID |
| userId | UUID | FK -> User.id, NOT NULL | User ID |
| platform | VARCHAR(20) | NOT NULL | SNS platform (twitter, discord, other) |
| url | TEXT | NOT NULL | SNS URL or handle |

#### Recommendation

| Column | Type | Constraint | Description |
|--------|------|-----------|-------------|
| id | UUID | PK, auto-generated | Recommendation ID |
| sourceWorkId | UUID | FK -> Work.id, NOT NULL | Source work (my work) |
| targetWorkId | UUID | FK -> Work.id, NOT NULL | Recommended work |
| projectType | VARCHAR(30) | NOT NULL | Project type (mv, game, album_art, animation, song, vtuber) |
| matchingTags | TEXT[] | NOT NULL | List of matching tags |
| score | FLOAT | NOT NULL | Recommendation score |
| clickedAt | TIMESTAMP | NULLABLE | Timestamp when clicked |
| createdAt | TIMESTAMP | NOT NULL, DEFAULT NOW | Creation date |

#### ProfileView

| Column | Type | Constraint | Description |
|--------|------|-----------|-------------|
| id | UUID | PK, auto-generated | View ID |
| viewerId | UUID | FK -> User.id, NULLABLE | Viewer's user ID (null if anonymous) |
| viewedId | UUID | FK -> User.id, NOT NULL | Viewed user's ID |
| source | VARCHAR(30) | NOT NULL | View source (recommendation, search, direct) |
| createdAt | TIMESTAMP | NOT NULL, DEFAULT NOW | View timestamp |

### Indexes

| Table | Index | Columns | Purpose |
|-------|-------|---------|---------|
| Work | idx_work_field | field | Filter by field |
| Work | idx_work_user | userId | List works by user |
| WorkTag | idx_worktag_tag | tag | Tag-based search |
| WorkTag | idx_worktag_work | workId | Tags for a work |
| UserRole | idx_userrole_user | userId | Roles for a user |
| UserSns | idx_usersns_user | userId | SNS for a user |
| Recommendation | idx_rec_source | sourceWorkId | Recommendations for a work |
| ProfileView | idx_pv_viewed | viewedId, createdAt | Profile view analytics |

## 4. API Design

### API Endpoints

#### Authentication

| Method | Path | Description |
|--------|------|-------------|
| GET | /auth/google | Google OAuth redirect |
| GET | /auth/google/callback | Google OAuth callback |
| GET | /auth/twitter | Twitter OAuth redirect |
| GET | /auth/twitter/callback | Twitter OAuth callback |
| POST | /auth/refresh | Refresh access token |
| POST | /auth/logout | Logout (invalidate refresh token) |

#### Users

| Method | Path | Description |
|--------|------|-------------|
| GET | /users/me | Get my profile |
| PATCH | /users/me | Update my profile |
| DELETE | /users/me | Delete my account |
| GET | /users/:id | Get user profile |
| GET | /users/:id/works | Get user's works |

#### Works

| Method | Path | Description |
|--------|------|-------------|
| POST | /works | Post a new work |
| GET | /works | List/search works (with filters) |
| GET | /works/:id | Get work details |
| PATCH | /works/:id | Update work |
| DELETE | /works/:id | Delete work |
| POST | /works/og-preview | Fetch OG metadata preview |

#### Recommendations

| Method | Path | Description |
|--------|------|-------------|
| POST | /recommendations | Generate recommendations for a work + project type |
| GET | /recommendations/:id | Get recommendation details |
| POST | /recommendations/:id/click | Record recommendation click |

#### Profile Views

| Method | Path | Description |
|--------|------|-------------|
| POST | /profile-views | Record profile view |
| GET | /users/:id/analytics | Get profile view analytics (own profile only) |

### API Example: Generate Recommendations

**Request**

```http
POST /recommendations
Content-Type: application/json
Authorization: Bearer <access_token>

{
  "workId": "550e8400-e29b-41d4-a716-446655440000",
  "projectType": "mv"
}
```

**Response**

```json
{
  "projectType": "mv",
  "sourceWork": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "title": "Midnight Rain",
    "field": "music",
    "tags": ["electronic", "night", "emotional"]
  },
  "requiredFields": ["illustration", "video"],
  "recommendations": {
    "illustration": [
      {
        "id": "660e8400-e29b-41d4-a716-446655440001",
        "title": "Neon City",
        "url": "https://pixiv.net/artworks/12345",
        "ogImage": "https://...",
        "field": "illustration",
        "tags": ["night", "electronic", "urban"],
        "matchingTags": ["night", "electronic"],
        "score": 0.67,
        "creator": {
          "id": "770e8400-e29b-41d4-a716-446655440002",
          "displayName": "ArtistA",
          "roles": ["illustrator"],
          "avatarUrl": "https://..."
        }
      }
    ],
    "video": [
      {
        "id": "880e8400-e29b-41d4-a716-446655440003",
        "title": "Lo-fi Visual Loop",
        "url": "https://youtube.com/watch?v=xxx",
        "ogImage": "https://...",
        "field": "video",
        "tags": ["night", "emotional", "lo-fi"],
        "matchingTags": ["night", "emotional"],
        "score": 0.67,
        "creator": {
          "id": "990e8400-e29b-41d4-a716-446655440004",
          "displayName": "VideoCreatorB",
          "roles": ["video_editor"],
          "avatarUrl": "https://..."
        }
      }
    ]
  }
}
```

## 5. EKS Cluster Configuration

### Cluster Specification

| Item | Spec |
|------|------|
| Cluster name | fugue-cluster |
| Kubernetes version | 1.29 |
| Region | ap-northeast-2 (Seoul) |
| Node group | Managed node group |
| Instance type | t3.medium (2 vCPU, 4 GiB) |
| Node count | min 2, max 4 (auto-scaling) |
| OS | Amazon Linux 2023 |

### Namespace Design

| Namespace | Purpose |
|-----------|---------|
| fugue | Application workloads (API, Web) |
| ingress-nginx | Ingress controller |
| monitoring | CloudWatch agent, metrics |

### Deployment Configuration

#### fugue-api

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fugue-api
  namespace: fugue
spec:
  replicas: 2
  selector:
    matchLabels:
      app: fugue-api
  template:
    metadata:
      labels:
        app: fugue-api
    spec:
      containers:
        - name: fugue-api
          image: <account-id>.dkr.ecr.ap-northeast-2.amazonaws.com/fugue-api:latest
          ports:
            - containerPort: 3000
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: fugue-secrets
                  key: database-url
            - name: REDIS_URL
              valueFrom:
                secretKeyRef:
                  name: fugue-secrets
                  key: redis-url
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: fugue-secrets
                  key: jwt-secret
          livenessProbe:
            httpGet:
              path: /health
              port: 3000
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 5
```

#### fugue-web

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fugue-web
  namespace: fugue
spec:
  replicas: 2
  selector:
    matchLabels:
      app: fugue-web
  template:
    metadata:
      labels:
        app: fugue-web
    spec:
      containers:
        - name: fugue-web
          image: <account-id>.dkr.ecr.ap-northeast-2.amazonaws.com/fugue-web:latest
          ports:
            - containerPort: 3001
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: 400m
              memory: 512Mi
          env:
            - name: NEXT_PUBLIC_API_URL
              value: "https://api.fugue.app"
          livenessProbe:
            httpGet:
              path: /
              port: 3001
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /
              port: 3001
            initialDelaySeconds: 5
            periodSeconds: 5
```

### Service Configuration

```yaml
apiVersion: v1
kind: Service
metadata:
  name: fugue-api
  namespace: fugue
spec:
  selector:
    app: fugue-api
  ports:
    - port: 80
      targetPort: 3000
  type: ClusterIP
---
apiVersion: v1
kind: Service
metadata:
  name: fugue-web
  namespace: fugue
spec:
  selector:
    app: fugue-web
  ports:
    - port: 80
      targetPort: 3001
  type: ClusterIP
```

### Ingress Configuration

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: fugue-ingress
  namespace: fugue
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - fugue.app
        - api.fugue.app
      secretName: fugue-tls
  rules:
    - host: fugue.app
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: fugue-web
                port:
                  number: 80
    - host: api.fugue.app
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: fugue-api
                port:
                  number: 80
```

## 6. Network Design

### VPC Configuration

| Item | Spec |
|------|------|
| VPC CIDR | 10.0.0.0/16 |
| Public Subnet A | 10.0.1.0/24 (ap-northeast-2a) |
| Public Subnet B | 10.0.3.0/24 (ap-northeast-2b) |
| Private Subnet A | 10.0.2.0/24 (ap-northeast-2a) |
| Private Subnet B | 10.0.4.0/24 (ap-northeast-2b) |
| NAT Gateway | 1 (in Public Subnet A, for cost savings) |
| Internet Gateway | 1 |

### Network Diagram

```
┌──────────────────────────────────────────────────────────┐
│                    VPC (10.0.0.0/16)                     │
│                                                          │
│  ┌────────────────────┐    ┌────────────────────┐        │
│  │  Public Subnet A   │    │  Public Subnet B   │        │
│  │  10.0.1.0/24       │    │  10.0.3.0/24       │        │
│  │  (ap-northeast-2a) │    │  (ap-northeast-2b) │        │
│  │                     │    │                     │        │
│  │  ┌─────┐ ┌──────┐  │    │  ┌─────┐           │        │
│  │  │ ALB │ │ NAT  │  │    │  │ ALB │           │        │
│  │  │     │ │ GW   │  │    │  │     │           │        │
│  │  └──┬──┘ └──┬───┘  │    │  └──┬──┘           │        │
│  └─────┼───────┼───────┘    └─────┼──────────────┘        │
│        │       │                  │                        │
│  ┌─────┼───────┼──────┐    ┌─────┼──────────────┐        │
│  │     v       │      │    │     v               │        │
│  │  Private Subnet A  │    │  Private Subnet B   │        │
│  │  10.0.2.0/24       │    │  10.0.4.0/24        │        │
│  │  (ap-northeast-2a) │    │  (ap-northeast-2b)  │        │
│  │                     │    │                     │        │
│  │  ┌──────────────┐  │    │  ┌──────────────┐   │        │
│  │  │  EKS Nodes   │  │    │  │  EKS Nodes   │   │        │
│  │  │  (fugue-api)  │  │    │  │  (fugue-web)  │   │        │
│  │  └──────────────┘  │    │  └──────────────┘   │        │
│  │                     │    │                     │        │
│  │  ┌──────────────┐  │    │  ┌──────────────┐   │        │
│  │  │  RDS         │  │    │  │  RDS         │   │        │
│  │  │ (PostgreSQL) │  │    │  │  (Standby)   │   │        │
│  │  └──────────────┘  │    │  └──────────────┘   │        │
│  │                     │    │                     │        │
│  │  ┌──────────────┐  │    │                     │        │
│  │  │ ElastiCache  │  │    │                     │        │
│  │  │   (Redis)    │  │    │                     │        │
│  │  └──────────────┘  │    │                     │        │
│  └─────────────────────┘    └─────────────────────┘        │
│                                                          │
└──────────────────────────────────────────────────────────┘
         │
         │  Internet Gateway
         v
    ┌──────────┐
    │ Internet │
    └──────────┘
```

### Security Groups

| Security Group | Inbound | Outbound | Attached To |
|----------------|---------|----------|-------------|
| sg-alb | 80/443 from 0.0.0.0/0 | All to VPC | ALB |
| sg-eks-node | All from sg-alb, All from sg-eks-node | All | EKS Nodes |
| sg-rds | 5432 from sg-eks-node | — | RDS |
| sg-redis | 6379 from sg-eks-node | — | ElastiCache |

### RDS Configuration

| Item | Spec |
|------|------|
| Engine | PostgreSQL 15 |
| Instance class | db.t3.micro (MVP) |
| Storage | 20 GiB gp3 |
| Multi-AZ | No (MVP, cost savings) |
| Backup retention | 7 days |
| DB name | fugue |

### ElastiCache Configuration

| Item | Spec |
|------|------|
| Engine | Redis 7 |
| Node type | cache.t3.micro (MVP) |
| Nodes | 1 (no replication in MVP) |
| Encryption | In-transit (TLS) |

## 7. CI/CD Pipeline

### Pipeline Diagram

```
┌──────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  GitHub  │    │   GitHub     │    │     ECR      │    │     EKS      │
│   Push   │───>│   Actions    │───>│  Push Image  │───>│   Deploy     │
│          │    │              │    │              │    │              │
│  main    │    │  - Lint      │    │  fugue-api   │    │  kubectl     │
│  branch  │    │  - Test      │    │  fugue-web   │    │  apply       │
│          │    │  - Build     │    │              │    │              │
└──────────┘    │  - Docker    │    └──────────────┘    └──────────────┘
                └──────────────┘
```

### GitHub Actions Workflow

```yaml
# .github/workflows/deploy.yml
name: Deploy Fugue

on:
  push:
    branches: [main]

env:
  AWS_REGION: ap-northeast-2
  ECR_REGISTRY: <account-id>.dkr.ecr.ap-northeast-2.amazonaws.com
  EKS_CLUSTER: fugue-cluster

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'pnpm'
      - run: pnpm install --frozen-lockfile
      - run: pnpm lint
      - run: pnpm test

  build-and-deploy-api:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Login to ECR
        uses: aws-actions/amazon-ecr-login@v2

      - name: Build and push API image
        run: |
          docker build -t $ECR_REGISTRY/fugue-api:${{ github.sha }} -f apps/api/Dockerfile .
          docker push $ECR_REGISTRY/fugue-api:${{ github.sha }}

      - name: Deploy to EKS
        run: |
          aws eks update-kubeconfig --name $EKS_CLUSTER --region $AWS_REGION
          kubectl set image deployment/fugue-api fugue-api=$ECR_REGISTRY/fugue-api:${{ github.sha }} -n fugue

  build-and-deploy-web:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: ${{ env.AWS_REGION }}

      - name: Login to ECR
        uses: aws-actions/amazon-ecr-login@v2

      - name: Build and push Web image
        run: |
          docker build -t $ECR_REGISTRY/fugue-web:${{ github.sha }} -f apps/web/Dockerfile .
          docker push $ECR_REGISTRY/fugue-web:${{ github.sha }}

      - name: Deploy to EKS
        run: |
          aws eks update-kubeconfig --name $EKS_CLUSTER --region $AWS_REGION
          kubectl set image deployment/fugue-web fugue-web=$ECR_REGISTRY/fugue-web:${{ github.sha }} -n fugue
```

### Branch Strategy

| Branch | Purpose | Deploy Target |
|--------|---------|---------------|
| main | Production | EKS (auto-deploy) |
| develop | Development integration | — (local only in MVP) |
| feature/* | Feature development | — |
| hotfix/* | Emergency fixes | — |

## 8. Monorepo Structure

```
fugue/
├── .github/
│   └── workflows/
│       └── deploy.yml              # CI/CD pipeline
├── apps/
│   ├── api/                        # Backend (NestJS)
│   │   ├── src/
│   │   │   ├── auth/               # Authentication module
│   │   │   │   ├── auth.controller.ts
│   │   │   │   ├── auth.service.ts
│   │   │   │   ├── auth.module.ts
│   │   │   │   ├── strategies/     # OAuth strategies
│   │   │   │   │   ├── google.strategy.ts
│   │   │   │   │   └── twitter.strategy.ts
│   │   │   │   └── guards/
│   │   │   │       └── jwt-auth.guard.ts
│   │   │   ├── users/              # User module
│   │   │   │   ├── users.controller.ts
│   │   │   │   ├── users.service.ts
│   │   │   │   ├── users.module.ts
│   │   │   │   └── dto/
│   │   │   │       ├── create-user.dto.ts
│   │   │   │       └── update-user.dto.ts
│   │   │   ├── works/              # Work module
│   │   │   │   ├── works.controller.ts
│   │   │   │   ├── works.service.ts
│   │   │   │   ├── works.module.ts
│   │   │   │   ├── og-fetcher.service.ts
│   │   │   │   └── dto/
│   │   │   │       ├── create-work.dto.ts
│   │   │   │       └── update-work.dto.ts
│   │   │   ├── recommendations/    # Recommendation module
│   │   │   │   ├── recommendations.controller.ts
│   │   │   │   ├── recommendations.service.ts
│   │   │   │   ├── recommendations.module.ts
│   │   │   │   └── templates/      # Project type templates
│   │   │   │       └── project-types.ts
│   │   │   ├── analytics/          # Analytics module
│   │   │   │   ├── analytics.controller.ts
│   │   │   │   ├── analytics.service.ts
│   │   │   │   └── analytics.module.ts
│   │   │   ├── prisma/             # Prisma module
│   │   │   │   ├── prisma.service.ts
│   │   │   │   └── prisma.module.ts
│   │   │   ├── app.module.ts
│   │   │   └── main.ts
│   │   ├── prisma/
│   │   │   ├── schema.prisma       # Database schema
│   │   │   └── migrations/         # Migration files
│   │   ├── test/
│   │   │   └── app.e2e-spec.ts
│   │   ├── Dockerfile
│   │   ├── tsconfig.json
│   │   └── package.json
│   └── web/                        # Frontend (Next.js)
│       ├── src/
│       │   ├── app/                # App Router pages
│       │   │   ├── layout.tsx
│       │   │   ├── page.tsx        # Home (work feed)
│       │   │   ├── auth/
│       │   │   │   └── callback/
│       │   │   │       └── page.tsx
│       │   │   ├── works/
│       │   │   │   ├── page.tsx    # Work list
│       │   │   │   ├── new/
│       │   │   │   │   └── page.tsx  # Post work
│       │   │   │   └── [id]/
│       │   │   │       └── page.tsx  # Work detail
│       │   │   ├── creators/
│       │   │   │   ├── page.tsx    # Creator list
│       │   │   │   └── [id]/
│       │   │   │       └── page.tsx  # Creator profile
│       │   │   ├── recommend/
│       │   │   │   └── page.tsx    # Recommendation flow
│       │   │   └── settings/
│       │   │       └── page.tsx    # Account settings
│       │   ├── components/
│       │   │   ├── ui/             # Base UI components
│       │   │   │   ├── Button.tsx
│       │   │   │   ├── Card.tsx
│       │   │   │   ├── Tag.tsx
│       │   │   │   └── Modal.tsx
│       │   │   ├── work/           # Work-related components
│       │   │   │   ├── WorkCard.tsx
│       │   │   │   ├── WorkGrid.tsx
│       │   │   │   ├── OgPreview.tsx
│       │   │   │   └── WorkForm.tsx
│       │   │   ├── creator/        # Creator-related components
│       │   │   │   ├── CreatorCard.tsx
│       │   │   │   └── CreatorProfile.tsx
│       │   │   ├── recommendation/ # Recommendation components
│       │   │   │   ├── ProjectTypeSelector.tsx
│       │   │   │   └── RecommendationList.tsx
│       │   │   └── layout/         # Layout components
│       │   │       ├── Header.tsx
│       │   │       ├── Footer.tsx
│       │   │       └── Sidebar.tsx
│       │   ├── hooks/              # Custom hooks
│       │   │   ├── useAuth.ts
│       │   │   ├── useWorks.ts
│       │   │   └── useRecommendations.ts
│       │   ├── lib/                # Utility libraries
│       │   │   ├── api.ts          # API client
│       │   │   ├── auth.ts         # Auth utilities
│       │   │   └── constants.ts    # Constants (tags, project types)
│       │   └── stores/             # Zustand stores
│       │       ├── authStore.ts
│       │       └── filterStore.ts
│       ├── public/
│       │   └── images/
│       ├── Dockerfile
│       ├── next.config.js
│       ├── tailwind.config.js
│       ├── tsconfig.json
│       └── package.json
├── packages/
│   └── shared/                     # Shared types/utilities
│       ├── src/
│       │   ├── types/              # Shared TypeScript types
│       │   │   ├── user.ts
│       │   │   ├── work.ts
│       │   │   └── recommendation.ts
│       │   └── constants/          # Shared constants
│       │       ├── fields.ts
│       │       ├── tags.ts
│       │       └── projectTypes.ts
│       ├── tsconfig.json
│       └── package.json
├── infra/                          # Terraform infrastructure
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   ├── vpc.tf                      # VPC, subnets, NAT GW
│   ├── eks.tf                      # EKS cluster, node group
│   ├── rds.tf                      # PostgreSQL RDS
│   ├── elasticache.tf              # Redis ElastiCache
│   ├── ecr.tf                      # Container registry
│   ├── s3.tf                       # Static assets bucket
│   ├── cloudfront.tf               # CDN
│   ├── route53.tf                  # DNS
│   └── acm.tf                      # TLS certificates
├── k8s/                            # Kubernetes manifests
│   ├── namespace.yml
│   ├── api-deployment.yml
│   ├── api-service.yml
│   ├── web-deployment.yml
│   ├── web-service.yml
│   ├── ingress.yml
│   └── secrets.yml
├── pnpm-workspace.yaml
├── package.json
├── turbo.json                      # Turborepo config
└── tsconfig.base.json
```

## 9. Terraform Configuration

### Main Configuration

```hcl
# infra/main.tf
terraform {
  required_version = ">= 1.7.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket = "fugue-terraform-state"
    key    = "prod/terraform.tfstate"
    region = "ap-northeast-2"
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "Fugue"
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}
```

### Variables

```hcl
# infra/variables.tf
variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "ap-northeast-2"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "prod"
}

variable "project_name" {
  description = "Project name"
  type        = string
  default     = "fugue"
}

variable "db_username" {
  description = "Database username"
  type        = string
  sensitive   = true
}

variable "db_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}
```

### VPC

```hcl
# infra/vpc.tf
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "${var.project_name}-vpc"
  cidr = "10.0.0.0/16"

  azs             = ["${var.aws_region}a", "${var.aws_region}b"]
  public_subnets  = ["10.0.1.0/24", "10.0.3.0/24"]
  private_subnets = ["10.0.2.0/24", "10.0.4.0/24"]

  enable_nat_gateway   = true
  single_nat_gateway   = true  # Cost savings for MVP
  enable_dns_hostnames = true
  enable_dns_support   = true

  public_subnet_tags = {
    "kubernetes.io/role/elb" = 1
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = 1
  }
}
```

### EKS

```hcl
# infra/eks.tf
module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.0"

  cluster_name    = "${var.project_name}-cluster"
  cluster_version = "1.29"

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  cluster_endpoint_public_access = true

  eks_managed_node_groups = {
    default = {
      instance_types = ["t3.medium"]
      min_size       = 2
      max_size       = 4
      desired_size   = 2

      labels = {
        Environment = var.environment
      }
    }
  }
}
```

### RDS

```hcl
# infra/rds.tf
module "rds" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 6.0"

  identifier = "${var.project_name}-db"

  engine               = "postgres"
  engine_version       = "15"
  family               = "postgres15"
  major_engine_version = "15"
  instance_class       = "db.t3.micro"

  allocated_storage = 20
  storage_type      = "gp3"

  db_name  = var.project_name
  username = var.db_username
  password = var.db_password
  port     = 5432

  multi_az               = false  # Cost savings for MVP
  db_subnet_group_name   = module.vpc.database_subnet_group_name
  vpc_security_group_ids = [aws_security_group.rds.id]

  backup_retention_period = 7
  skip_final_snapshot     = true  # MVP only

  parameters = [
    {
      name  = "client_encoding"
      value = "UTF8"
    }
  ]
}

resource "aws_security_group" "rds" {
  name_prefix = "${var.project_name}-rds-"
  vpc_id      = module.vpc.vpc_id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }
}
```

### ElastiCache

```hcl
# infra/elasticache.tf
resource "aws_elasticache_replication_group" "redis" {
  replication_group_id = "${var.project_name}-redis"
  description          = "Fugue Redis cache"

  engine               = "redis"
  engine_version       = "7.0"
  node_type            = "cache.t3.micro"
  num_cache_clusters   = 1  # No replication in MVP

  subnet_group_name  = aws_elasticache_subnet_group.redis.name
  security_group_ids = [aws_security_group.redis.id]

  transit_encryption_enabled = true
  at_rest_encryption_enabled = true

  parameter_group_name = "default.redis7"
}

resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.project_name}-redis-subnet"
  subnet_ids = module.vpc.private_subnets
}

resource "aws_security_group" "redis" {
  name_prefix = "${var.project_name}-redis-"
  vpc_id      = module.vpc.vpc_id

  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [module.eks.node_security_group_id]
  }
}
```

### ECR

```hcl
# infra/ecr.tf
resource "aws_ecr_repository" "api" {
  name                 = "fugue-api"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_repository" "web" {
  name                 = "fugue-web"
  image_tag_mutability = "MUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }
}

resource "aws_ecr_lifecycle_policy" "api" {
  repository = aws_ecr_repository.api.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 10 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 10
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}

resource "aws_ecr_lifecycle_policy" "web" {
  repository = aws_ecr_repository.web.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep last 10 images"
        selection = {
          tagStatus   = "any"
          countType   = "imageCountMoreThan"
          countNumber = 10
        }
        action = {
          type = "expire"
        }
      }
    ]
  })
}
```

## 10. Cost Estimate (Monthly)

### MVP Estimated Cost

| Service | Spec | Estimated Monthly Cost (USD) |
|---------|------|------------------------------|
| EKS Cluster | Control plane | $73 |
| EC2 (EKS Nodes) | t3.medium x2 | $61 |
| RDS PostgreSQL | db.t3.micro | $13 |
| ElastiCache Redis | cache.t3.micro | $12 |
| NAT Gateway | 1 instance + data transfer | $32 |
| ALB | 1 instance | $16 |
| ECR | Image storage | $1 |
| Route 53 | 1 hosted zone | $1 |
| S3 + CloudFront | Static assets | $1 |
| **Total** | | **~$210/month** |

### Cost Optimization Strategies

| Strategy | Description | Savings |
|----------|-------------|---------|
| Single NAT Gateway | Use 1 NAT GW instead of per-AZ | ~$32/month |
| RDS Single-AZ | No Multi-AZ for MVP | ~$13/month |
| No Redis Replication | Single Redis node for MVP | ~$12/month |
| Spot Instances | Consider for EKS nodes (non-critical) | Up to 60% on EC2 |
| Reserved Instances | Consider after MVP validation | Up to 30% savings |
| Scale Down Off-hours | Reduce nodes during low-traffic periods | Variable |

### Free Tier Utilization

| Service | Free Tier | Notes |
|---------|-----------|-------|
| EC2 | 750 hrs/month t2.micro (12 months) | Not applicable — t3.medium needed |
| RDS | 750 hrs/month db.t3.micro (12 months) | Applicable for first year |
| ElastiCache | 750 hrs/month cache.t3.micro (12 months) | Applicable for first year |
| S3 | 5 GB storage | Sufficient for static assets |
| CloudFront | 1 TB transfer/month (12 months) | Sufficient for MVP |
| ECR | 500 MB storage | Sufficient for MVP |

### First-Year Adjusted Cost (with Free Tier)

| Service | Adjusted Monthly Cost (USD) |
|---------|------------------------------|
| EKS Cluster | $73 |
| EC2 (EKS Nodes) | $61 |
| RDS PostgreSQL | $0 (free tier) |
| ElastiCache Redis | $0 (free tier) |
| NAT Gateway | $32 |
| ALB | $16 |
| ECR/S3/CloudFront/Route 53 | ~$3 |
| **Total (first year)** | **~$185/month** |
