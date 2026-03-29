# Fugue

창작물 기반 협업 매칭 플랫폼. 사람이 아닌 "작품"을 매칭한다.

마스코트: 헤드셋 끼고 붓 든 복어 (Fugue ≈ Fugu)

## 프로젝트 구조

```
fugue/
├── apps/
│   ├── api/          # Go Backend
│   └── web/          # Next.js Frontend
├── helm/
│   └── fugue/
├── terraform/
├── docs/             # 기획 문서 (ko/en/zh/ja)
├── docker-compose.yml
└── CLAUDE.md
```

## 기술 스택

### Application

| 계층 | 기술 |
|------|------|
| Frontend | Next.js 15 (App Router) |
| Backend | Go + Chi router |
| ORM | sqlc |
| DB | PostgreSQL 16 |
| Cache | Redis |
| Auth | OAuth 2.0 (Twitter, Discord) |

### Infrastructure

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
| IAM↔K8s | IRSA |

### CI/CD

| 단계 | 기술 |
|------|------|
| CI | GitHub Actions |
| CD | ArgoCD (GitOps) |
| Manifest | Helm |
| 환경 분리 | 같은 클러스터 내 namespace (dev/prod) |

### Observability

| 영역 | 기술 |
|------|------|
| Metrics | Prometheus + Grafana |
| Logs | Loki + Promtail |
| Traces | Tempo + OpenTelemetry |
| Alerting | Grafana Alerting |

## MVP 기능 스펙

### 1. 작품 투고

외부 플랫폼 링크를 붙여넣으면 자동 프리뷰 생성.

- URL 붙여넣기 → OG 메타데이터 fetch (제목, 썸네일, 설명)
- 지원: SoundCloud, pixiv, YouTube, Twitter, 기타 URL
- 분야 자동감지 (도메인 기반) + 수동 선택
- 분야 태그: 음악, 일러스트, 영상, 3D, 사운드
- 스타일 태그: 최소 1개 ~ 최대 5개
- 라이선스: "크레딧 표기 시 자유 사용" 기본값

### 2. 크리에이터 프로필

- 닉네임 (필수)
- 역할 태그 복수 선택 (작곡, 일러스트, 영상편집, 3D모델링, 성우, 사운드디자인, 작사, 기타)
- 한 줄 소개
- SNS 연락처 최소 1개 (Twitter, Discord, 기타)
- 프로필 이미지 (없으면 기본 아바타)
- 투고 작품이 자동으로 포트폴리오가 됨

### 3. 추천 (작품 매칭)

핵심 기능. "뭘 만들고 싶으신가요?" → 프로젝트 유형 선택 → 내 작품 태그 기반으로 다른 분야 작품 추천.

프로젝트 유형별 필요 분야:
- MV → 일러스트, 영상
- 게임 → 일러스트, 음악, 사운드, 3D
- 앨범 아트워크 → 일러스트
- 애니메이션 → 일러스트, 음악, 성우
- 보이스드라마 → 성우, 음악, 사운드
- 기타 → 유저가 직접 선택

추천 로직 (v1):
```sql
SELECT * FROM works
WHERE field IN (프로젝트 유형의 필요 분야)
  AND tags && ARRAY[내 작품 태그들]
ORDER BY array_length(tags & ARRAY[내 작품 태그들]) DESC, created_at DESC
```

### 4. 탐색

- 작품 탐색: 분야 + 태그 필터, 최신순/태그 일치순 정렬
- 크리에이터 탐색: 역할 태그 필터

### 5. 인증

- 소셜 로그인: Twitter OAuth, Discord OAuth
- 이메일/비밀번호 (대안)

## DB 스키마

```sql
CREATE TABLE creators (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nickname    VARCHAR(50) NOT NULL,
    bio         VARCHAR(200),
    roles       TEXT[] NOT NULL,
    contacts    JSONB NOT NULL,
    avatar_url  VARCHAR(500),
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE works (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id  UUID NOT NULL REFERENCES creators(id),
    url         VARCHAR(1000) NOT NULL,
    title       VARCHAR(200) NOT NULL,
    description VARCHAR(500),
    field       VARCHAR(50) NOT NULL,
    tags        TEXT[] NOT NULL,
    og_image    VARCHAR(1000),
    og_data     JSONB,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_works_field ON works(field);
CREATE INDEX idx_works_tags ON works USING GIN(tags);
CREATE INDEX idx_works_creator ON works(creator_id);
CREATE INDEX idx_creators_roles ON creators USING GIN(roles);
```

## API 엔드포인트

```
# Auth
POST   /api/auth/twitter/callback
POST   /api/auth/discord/callback
POST   /api/auth/logout

# Creator
GET    /api/creators/:id
PUT    /api/creators/me
GET    /api/creators?roles=&page=&limit=

# Work
POST   /api/works
GET    /api/works/:id
DELETE /api/works/:id
GET    /api/works?field=&tags=&page=&limit=

# Recommendation
POST   /api/recommend
       body: { work_id, project_type }

# OG Metadata
POST   /api/og/fetch
       body: { url }
```

## 개발 가이드

### 로컬 실행

```bash
docker-compose up -d     # PostgreSQL + Redis
cd apps/api && go run cmd/server/main.go
cd apps/web && npm run dev
```

### 코드 컨벤션

- Go: 표준 프로젝트 레이아웃 (cmd/ internal/)
- Go router: Chi
- Go DB: sqlc (SQL 직접 작성 → Go 코드 자동 생성)
- Frontend: Next.js App Router, TypeScript
