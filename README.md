# Fugue

창작물 기반 협업 매칭 플랫폼. 사람이 아닌 "작품"을 매칭한다.

마스코트: 헤드셋 끼고 붓 든 복어 (Fugue ≈ Fugu)

## 기술 스택

| 계층 | 기술 |
|------|------|
| Frontend | Next.js 15 (App Router), TypeScript |
| Backend | Go + Chi router |
| ORM | sqlc |
| DB | PostgreSQL 16 |
| Cache | Redis |
| Auth | OAuth 2.0 (Twitter, Discord) |
| Infra | Terraform, EKS, ArgoCD, Helm |

## MVP 개발 일정

### Phase 1: 소셜 인증 (Week 1 전반)

> 모든 기능의 전제조건. 유저 식별 없이 투고/프로필 불가.

- [ ] Twitter OAuth 2.0 콜백 (`POST /api/auth/twitter/callback`)
- [ ] Discord OAuth 콜백 (`POST /api/auth/discord/callback`)
- [ ] JWT 발급/검증 미들웨어
- [ ] 로그아웃 (`POST /api/auth/logout`)
- [ ] creators 테이블 자동 생성 (첫 로그인 시)

### Phase 2: 크리에이터 프로필 (Week 1 후반)

> DB 스키마 확정 + sqlc 코드 생성 기반. 작품 투고의 FK 의존성 해소.

- [ ] 프로필 수정 (`PUT /api/creators/me`)
- [ ] 프로필 조회 (`GET /api/creators/:id`)
- [ ] 역할 태그 복수 선택 (작곡, 일러스트, 영상편집 등)
- [ ] 한 줄 소개, SNS 연락처, 프로필 이미지
- [ ] 프로필 페이지 프론트엔드

### Phase 3: 작품 투고 + OG Fetch (Week 2)

> 플랫폼의 콘텐츠 공급 파이프라인. OG fetch가 핵심 "와" 모먼트.

- [ ] OG 메타데이터 fetch (`POST /api/og/fetch`)
- [ ] 작품 등록 (`POST /api/works`)
- [ ] 작품 조회 (`GET /api/works/:id`)
- [ ] 작품 삭제 (`DELETE /api/works/:id`)
- [ ] 분야 자동감지 (도메인 기반) + 수동 선택
- [ ] 스타일 태그 선택 (1~5개)
- [ ] OG 태그 없는 URL fallback (유저 직접 입력)
- [ ] 투고 폼 + 프리뷰 프론트엔드

### Phase 4: 추천 - 작품 매칭 (Week 3 전반)

> Fugue의 핵심 차별점. "뭘 만들고 싶으세요?" → 태그 기반 작품 추천.

- [ ] 추천 API (`POST /api/recommend`)
- [ ] 프로젝트 유형별 필요 분야 매핑 (MV, 게임, 앨범 아트워크 등)
- [ ] 태그 교집합 기반 정렬 (GIN 인덱스 활용)
- [ ] 추천 결과 페이지 (겹치는 태그 하이라이트)

### Phase 5: 탐색 (Week 3 후반)

> 추천의 안전망. 유저가 직접 브라우징할 수 있는 필터 탐색.

- [ ] 작품 탐색 (`GET /api/works?field=&tags=&page=&limit=`)
- [ ] 크리에이터 탐색 (`GET /api/creators?roles=&page=&limit=`)
- [ ] 분야 + 태그 필터 UI
- [ ] 최신순 / 태그 일치순 정렬

## 리스크

| 리스크 | 영향 | 대응 |
|--------|------|------|
| OG fetch 엣지케이스 (JS 렌더링 필요 사이트, 비표준 메타태그) | Phase 3 일정 지연 | v1은 OG 없으면 유저 직접 입력 fallback. headless browser는 v2 |
| Twitter OAuth 앱 승인 대기 | Phase 1 블로커 | Discord부터 먼저 구현 |
| 초기 작품 수 부족 시 추천 품질 저하 | Phase 4 가치 검증 어려움 | 시드 데이터 준비, 팀 내부 테스트 투고 |

## 로컬 실행

```bash
docker-compose up -d          # PostgreSQL + Redis
cd apps/api && go run cmd/server/main.go
cd apps/web && npm run dev
```
