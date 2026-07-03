# Fugue

크로스미디어 창작물 큐레이션 플랫폼. Pinterest가 이미지만이라면, Fugue는 음악, 일러스트, 영상, 글, 코드를 분야를 넘나들며 한곳에서 발견한다.

마스코트: 헤드셋 끼고 붓 든 복어 (Fugue ≈ Fugu)

## 기술 스택

| 계층 | 기술 |
|------|------|
| Frontend | Next.js 16 (App Router), TypeScript |
| Backend | Go + Chi router |
| ORM | sqlc |
| DB | PostgreSQL 16 |
| Cache | Redis |
| Auth | OAuth 2.0 (Google, Discord) |
| Infra | Terraform, EKS, ArgoCD, Helm |

## MVP 기능

- **핀**: 외부 창작물 URL → OG 자동 프리뷰 → 분야/태그 선택 → 큐레이션
- **보드**: 핀을 주제별로 묶는 컬렉션
- **추천 피드**: 태그 기반 취향 학습 → 개인화된 작품 추천
- **연관 작품**: 작품 상세에서 유사 작품 자동 표시
- **소셜 로그인**: Google, Discord OAuth (구현 완료)

## 구현 현황

- [x] OAuth 소셜 로그인 (Google, Discord)
- [x] 유저 프로필 (닉네임, 아바타)
- [x] 작품 피드 (분야/태그 필터, 페이지네이션)
- [x] Bot CLI (Pioneer/Harvester 크롤러)
- [x] 핀 생성/삭제 + OG fetch
- [x] 보드 CRUD
- [x] 추천 피드
- [ ] 암묵적 취향 학습 (행동 기록은 구현, Kinesis 적재 파이프라인 미구현)
- [x] 연관 작품

## 문서

- [PRD](docs/ko/PRD.md)
- [MVP 기능 스펙](docs/mvp-features.md)
- [API 엔드포인트](docs/api-endpoints.md)
- [ERD](docs/erd.md)
- [아키텍처](docs/architecture.md)
- [기술 스택 상세](docs/tech-stack.md)

## 로컬 실행

### 1. 환경 설정

```bash
cd apps/api
cp .env.example .env
```

필수 환경변수:
- `DATABASE_URL`: PostgreSQL 연결 URL
- `REDIS_URL`: Redis 연결 URL
- `JWT_SECRET`: JWT 토큰 서명 키
- `OAUTH_CALLBACK_BASE_URL`: OAuth 콜백 베이스 URL (미설정 시 서버 기동 실패)
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`: Google OAuth
- `DISCORD_CLIENT_ID`, `DISCORD_CLIENT_SECRET`: Discord OAuth

### 2. 실행

```bash
docker-compose up -d          # PostgreSQL + Redis
cd apps/api && go run cmd/server/main.go
cd apps/web && npm run dev
```

### 3. Bot CLI (Pioneer/Harvester)

Fuguebot은 Pioneer(탐색)와 Harvester(수확) 크롤러를 제공합니다.

#### 사용법

```bash
# Make를 통한 실행 (리포지토리 루트에서)
make pioneer SITE=unsplash      # Pioneer 크롤러 실행
make harvester                  # Harvester 워커 실행 (전 사이트 URL을 우선순위 순으로 소비)

# 직접 실행 (apps/api 디렉토리에서)
go run ./cmd/bot pioneer unsplash
HARVESTER_MODE=real go run ./cmd/bot harvester

# 도움말
go run ./cmd/bot --help
go run ./cmd/bot pioneer --help
```

#### 지원 사이트

- `unsplash`: unsplash.com
- `fma`: freemusicarchive.org
- `pixiv`: pixiv.net

도메인 전체를 입력해도 동작합니다 (예: `unsplash.com`).

#### 환경변수

Pioneer/Harvester 실행 시:
- Storage 및 DB 설정 필요 (`.env` 참조)


## 개발 도구

### Bot Graph Visualization

Pioneer가 발견한 노드 그래프를 시각화:

```bash
make show-map
```

생성된 `graph.html`을 브라우저에서 열면 인터랙티브 그래프를 볼 수 있습니다.

**옵션**:
- PNG/SVG export: `make show-map` 후 `-format=png` 또는 `-format=svg` 사용 (Graphviz 필요)
- 특정 사이트만: `-filter-site=<domain>`
- 출력 경로 지정: `-output=<path>`

```bash
# 예시
cd apps/api
go run cmd/bot-visualize/main.go -format=png -output=graph.png
```

**Graphviz 설치** (PNG/SVG export용, 선택사항):
```bash
brew install graphviz
```
