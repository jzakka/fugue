# Fugue Architecture

## 시스템 구성

```
┌─────────────┐     ┌──────────────────┐     ┌──────────────┐
│  Next.js    │────→│    Go API        │────→│  PostgreSQL  │
│  (Frontend) │     │   (Chi + sqlc)   │     │              │
│             │     │                  │────→│  Redis       │
│  /pin/new   │     │  /api/pins       │     │  (cache,     │
│  /feed      │     │  /api/boards     │     │   rate limit)│
│  /boards    │     │  /api/feed       │     │              │
│  /profile   │     │  /api/og/fetch   │     └──────────────┘
└─────────────┘     │                  │
      │             │  Event Worker ───│────→┌──────────────┐
      │ proxy       │  (channel+flush) │     │  Kinesis     │
      │ /api/*      └────────┬─────────┘     │  Firehose    │
      │ → :8080              │               └──────┬───────┘
      └──────────────────────┘                      │ Parquet
                                             ┌──────▼───────┐
                              ┌─────────┐    │     S3       │
                              │ Athena  │←───│  (events)    │
                              │ (query) │    └──────────────┘
                              └─────────┘
                                             ┌──────────────┐
                                             │ External URLs│
                                             │ (SoundCloud, │
                                             │  pixiv, etc.)│
                                             └──────────────┘
```

## 핵심 모듈

### Backend (Go)

```
apps/api/
├── cmd/server/main.go          # 엔트리포인트, 라우터 설정
├── internal/
│   ├── auth/                   # OAuth, JWT, 세션 관리
│   ├── creator/                # 프로필 (간소화: 닉네임+아바타)
│   ├── pin/                    # 핀 CRD + 연관 핀
│   ├── boards/                 # 보드 CRUD + 핀 관리
│   ├── og/                     # OG 메타데이터 fetch (SSRF 방지)
│   ├── feed/                   # 추천 피드
│   ├── event/                  # 이벤트 수집 (channel + Firehose worker)
│   ├── config/                 # 환경 설정
│   └── db/                     # sqlc 생성 코드
└── db/
    ├── migrations/             # golang-migrate
    └── queries/                # sqlc SQL 파일
```

### Frontend (Next.js)

```
apps/web/src/
├── app/
│   ├── page.tsx                # 피드 (추천 기반)
│   ├── pin/new/                # 핀 등록
│   ├── pins/[id]/              # 핀 상세 + 연관 핀
│   ├── boards/[id]/            # 보드 상세
│   ├── mypage/                 # 내 프로필 (간소화)
│   ├── creators/[id]/          # 유저 프로필
│   └── login/                  # 로그인
├── components/
│   ├── feed/                   # 피드 관련 (PinCard, FieldFilter 등)
│   ├── profile/                # 프로필 (ProfileHeader, PinsGrid 등)
│   ├── nav/                    # 네비게이션
│   └── auth/                   # 인증
└── lib/
    ├── api.ts                  # API 클라이언트
    └── auth.ts                 # JWT 유틸
```

## 데이터 흐름

### 핀 생성

```
[URL 입력] → debounce 500ms → POST /api/og/fetch
                                    │
                               [SSRF 검증]
                               [HTML fetch + OG 파싱]
                                    │
                                    ▼
                           [OG 프리뷰 응답] → 프론트 카드 렌더링
                                    │
                            유저: 분야/태그 선택
                                    │
                                    ▼
                           POST /api/pins → DB INSERT → 201
                           → event channel { type: 'pin' }
```

### 이벤트 파이프라인

```
[유저 행동]
    │
    ▼
[API Handler] ──→ [Go channel] ──→ [Event Worker]
                   비동기            버퍼 500개 or 10초
                   non-blocking      │
                                     ▼
                              [Firehose PutRecordBatch]
                                     │
                                     ▼
                              [S3] Parquet, 날짜 파티셔닝
                              s3://fugue-events/year=/month=/day=/hour=/
                                     │
                                     ▼
                              [Athena] ad-hoc 분석
```

이벤트 스키마:
- 고정 필드: event_id, user_id, pin_id, event_type, timestamp
- 확장 필드: context (JSON) — source, session_id, field 등

### 추천 피드

```
[GET /api/feed]
     │
     ▼
[Redis 캐시 확인] ──hit──→ [캐시된 추천 반환]
     │
    miss
     │
     ▼
[유저 핀의 태그 빈도 집계]
     │
     ▼
[태그 가중 점수로 후보 핀 정렬]
     │
     ▼
[이미 핀한 작품 제외]
     │
     ▼
[Redis에 캐싱 (TTL 5분)]
     │
     ▼
[추천 + 최신 혼합하여 반환]
```

### 추천 엔진 진화 로드맵

```
v1 (MVP)          v2                    v3
─────────         ─────────             ─────────
태그 빈도          피처스토어             ML 모델
매칭              도입                  학습

S3 events ────→ feature_store ────→ model training
(raw data)       (유저/핀              (collaborative
                  피처 관리)            filtering 등)
```

## 보안

### OG Fetch SSRF 방지
- Allowed schemes: http/https만
- 커스텀 DialContext로 DNS 해석 후 resolved IP 검증
- Private IP 차단: 10.x, 172.16-31.x, 192.168.x, 127.x, ::1, 169.254.x
- 리다이렉트: 최대 5 hop, 매 hop마다 IP 재검증
- 응답 크기: 최대 1MB (io.LimitReader)
- 타임아웃: 커넥션 3초, 전체 5초

### 인증/인가
- JWT (access + refresh token)
- 핀 삭제: `WHERE id = $1 AND creator_id = $2` (본인만)
- 보드 수정/삭제: 소유자 검증
- 비공개 보드: 소유자만 접근

### Rate Limit
- 핀 생성: 30/분/유저
- OG fetch: 20/분/IP

## 인프라

tech-stack.md 참조. Terraform + EKS + ArgoCD.
