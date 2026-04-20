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

## Fuguebot (콘텐츠 크롤러)

API 서버와 별도 바이너리. 외부 플랫폼을 크롤링하여 피드를 자동으로 채운다.

```
apps/api/
├── cmd/bot/main.go             # fuguebot 엔트리포인트
├── internal/
│   ├── bot/
│   │   ├── engine.go           # Colly 기반 크롤 엔진
│   │   ├── source.go           # Source interface 정의
│   │   ├── downloader.go       # 미디어 다운로드 + S3 업로드
│   │   ├── dedup.go            # URL 중복 체크
│   │   └── sources/
│   │       ├── pixiv.go        # Pixiv 플러그인
│   │       └── soundcloud.go   # SoundCloud 플러그인
```

### 크롤 파이프라인

```
[Scheduler/Cron]
     │
     ▼
[Source.SeedURLs()] → 시드 URL 목록
     │
     ▼
[Colly Crawl] → robots.txt 확인 → rate limit → HTML 파싱
     │
     ▼
[Source.Extract()] → 미디어URL, 제목, 설명, 출처URL 추출
     │
     ▼
[Dedup] → URL 이미 존재? → skip
     │
     ▼
[Media Download] → 이미지/음원/비디오 다운로드 → 포맷/사이즈 검증
     │
     ▼
[S3 Upload] → 미디어 버킷에 저장
     │
     ├──→ [Auto Tagger] → OG 텍스트에서 태그 자동 추출
     │
     ▼
[Pin Create] → pins 테이블 INSERT (creator_id = fuguebot)
     │
     ▼
[Stats Logger] → S3 이벤트 파이프라인으로 크롤 통계 전송
```

### 관리 API

```
GET  /api/admin/bot/status       — 마지막 크롤 시간, 플랫폼별 수집 건수, 실패율
GET  /api/admin/bot/sources      — 등록된 소스 목록
POST /api/admin/bot/sources      — 소스 추가
DEL  /api/admin/bot/sources/:id  — 소스 제거
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

## 크롤러 frontier (Pioneer → Harvester fanout)

기존 인메모리 BFS 큐(`PriorityQueue`, `BFSQueue`)는 단일 프로세스에서만 동작하며, 재시작 시 진행 상태가 휘발된다. 복수 워커로 수평 확장 가능한 운영을 위해 URL 큐를 Postgres 기반 `pioneer_frontier` / `harvester_frontier` 두 테이블로 분리하여 영속화한다. (스키마 상세는 [erd.md](erd.md#크롤러-frontier-테이블) 참조.)

```
                        ┌─────────────────────────────┐
  새 링크 N개 enqueue    │      pioneer_frontier       │  (URL fetch 큐)
 ┌────────────────────→│ claim → HTTP fetch           │
 │                      └──────────────┬──────────────┘
 │                                     │ fetch 성공
 │                                     ▼
 │ 새로 발견된 링크         ┌─────────────────────────────┐
 │ (같은 호스트/규칙)       │       snapshot 저장 (S3)      │
 │                      └──────────────┬──────────────┘
 │                                     │ snapshot_key 확보
 │                                     ▼
 │                      ┌─────────────────────────────┐
 └──────────────────────│     harvester_frontier      │  (HTML 파싱 큐)
     (Pioneer는 자기    │ UPSERT (harvested_at IS NULL│
      자신에게도 fanout)│  가드) → claim → parse        │
                        └──────────────┬──────────────┘
                                       │ Pin N개 생성
                                       ▼
                        ┌─────────────────────────────┐
                        │   harvester_frontier_pins   │
                        │        (1:N 조인)            │
                        └─────────────────────────────┘
```

Pioneer의 한 fetch 결과는 (a) 새 링크 N개 → `pioneer_frontier` 재enqueue, (b) 원본 URL 1건 + snapshot_key → `harvester_frontier` UPSERT 로 **fanout**된다. 두 큐의 partial index(`pioneer_frontier_claimable_idx`, `harvester_frontier_claimable_idx`)가 `score DESC, next_*_at ASC` 순으로 claim 대상을 O(log n)에 꺼낸다. `next_fetch_at` / `next_harvest_at`은 claim 시 `now() + 10 minutes`로 밀어 **in-flight lease marker**를 겸한다(워커 크래시 시 10분 뒤 자동 회수).

### URLScheduler interface (scheduler-claim-api)

`apps/api/internal/scheduler/url_scheduler.go`의 `URLScheduler`는 Pioneer/Harvester 워커가 frontier 테이블과 상호작용하는 단일 경계다. 여섯 메서드를 제공한다:

- `Enqueue(queueType, urls...)` — `QueuePioneer` / `QueueHarvester` enum으로 대상 테이블을 지정. 정규화 + `url_hash` 해싱 + batch UPSERT(pioneer: `DO NOTHING`, harvester: `harvested_at IS NULL` 조건부 UPSERT, **snapshot_key 미변경**).
- `EnqueueHarvester(url, snapshotKey)` — Pioneer consumer의 fanout B 경로. 단일 URL + `snapshot_key`를 `harvester_frontier`에 UPSERT한다. `harvested_at IS NULL` 가드를 그대로 사용하므로 이미 harvest된 row는 no-op이며, 미완료 row는 `snapshot_key`/`next_harvest_at`/`harvest_error_count`가 갱신된다. `pioneer-scheduler-consumer` change에서 추가.
- `Dequeue(queueType)` — **block-on-empty**(빈 큐/host throttle에서 1초 sleep 후 재시도) · **linearizable**(`SELECT ... FOR UPDATE SKIP LOCKED` + in-flight marker UPDATE를 동일 트랜잭션에서 수행) 규약. 상위 `SCHEDULER_CLAIM_CANDIDATE_N`(기본 1)개 후보 중 `HostRateLimiter.Allow(host)`가 처음 true인 row를 claim한다.
- `SetStatus(key, status, pinIDs)` — `fetched` / `fetch_failed` / `harvested` / `harvest_failed` 네 status만 허용. `harvested` 시 `harvester_frontier_pins` 테이블에 `pinIDs`(UUID)를 동일 트랜잭션으로 INSERT.
- `RecordFetchError(key, errorKind)` / `RecordHarvestError(key, errorKind)` — `http_4xx` / `http_5xx` / `network` / `timeout` 네 enum. 4xx는 즉시 `*_error_count = 5`(dead), 나머지는 `scheduler-retry-backoff` 공식에 따라 count++ + `next_*_at` jittered backoff.

In-flight marker는 별도 컬럼 없이 `next_fetch_at = clock.Now() + 10min`(Go 시계 기준) UPDATE로 처리되며 lease 만료 시 partial index에 자연히 복귀한다. 호출부(Pioneer/Harvester worker)의 실제 교체는 `harvester-scheduler-consumer` / `pioneer-*` 후속 change에서 이루어진다.

### 로드맵
- `scheduler-frontier-table` *(완료)*: 테이블/인덱스/제약/lease 규약 확정. sqlc 모델 생성.
- `scheduler-claim-api` *(완료)*: `URLScheduler` Go 인터페이스 + `SELECT ... FOR UPDATE SKIP LOCKED` 기반 claim 쿼리 + Postgres 구현체. 호출부 마이그레이션(`priority_queue.go` / `bfs_queue.go` 제거 포함)은 후속 change 범위.
- `scheduler-retry-backoff` *(완료)*: `fetch_error_count` / `harvest_error_count`에 따른 `next_*_at` exponential backoff 공식.
- `scheduler-host-token-bucket` *(완료)*: host별 동시 요청 제어(토큰 버킷) — `host` 컬럼을 키로 사용.
- `pioneer-scheduler-consumer` *(완료)*: Pioneer Run() 진입점을 `URLScheduler.Dequeue` 기반 consumer 루프로 교체하고, fetch 성공 시 새 링크를 `Enqueue(QueuePioneer, ...)`로, 원본+snapshot_key를 `EnqueueHarvester(url, snapKey)`로 fanout한다 (feature flag `BOT_PIONEER_SCHEDULER`로 롤아웃).
- `harvester-scheduler-consumer` (예정): Harvester Run() 진입점을 `URLScheduler.Dequeue(QueueHarvester)` 기반으로 교체.

### Host Token Bucket (politeness)

`apps/api/internal/scheduler/host_rate_limiter.go`의 `HostRateLimiter`는 호스트별 token bucket을 인메모리(`map[string]*rate.Limiter` + `sync.RWMutex`)로 유지한다. 두 메서드만 외부에 노출한다.

- `Allow(host string) bool`: 토큰이 있으면 1개 소비하고 true, 없으면 false. 처음 보는 호스트는 운영자 설정 기본값으로 lazy 생성된다. 비활성화 상태(`SCHEDULER_HOST_TOKEN_BUCKET_ENABLED=false`)에서는 항상 true를 반환하며 bucket 상태를 변경하지 않는다.
- `SetHostRate(host string, rate float64, burst int)`: 호스트의 rate/burst를 즉시 교체. `rate <= 0` 또는 `burst <= 0` 입력은 운영자 설정 기본값(미설정 시 공장 기본값 1 req/sec, burst 5)으로 대체하고 WARN 로그를 남긴다. 에러 반환/패닉 없음.

설정 키:

| 키 | 환경변수 | 기본값 |
|---|---|---|
| `scheduler.host_default_rate_per_sec` | `SCHEDULER_HOST_DEFAULT_RATE_PER_SEC` | `1.0` |
| `scheduler.host_default_burst` | `SCHEDULER_HOST_DEFAULT_BURST` | `5` |
| `scheduler.host_token_bucket_enabled` | `SCHEDULER_HOST_TOKEN_BUCKET_ENABLED` | `true` |

claim 시점에 `Allow`를 호출하는 dequeue 패턴(상위 N개 후보를 가져와 host별 검사, 모든 후보 blocked 시 sleep)은 본 모듈이 아니라 `scheduler-claim-api`의 책임이다. robots.txt Crawl-delay → `SetHostRate` 호출은 `pioneer-link-filter-policy`의 책임이다.

> **운영 주의**: 토큰 상태는 프로세스별 인메모리이므로 외부 사이트가 보는 실효 rate ≈ **(scheduler 프로세스 수) × (호스트 rate)**. 동일 호스트로의 합산 요청률을 1 req/sec로 제한하고 싶다면 (a) 단일 프로세스 운영 또는 (b) 프로세스 수를 N으로 줄이고 `SCHEDULER_HOST_DEFAULT_RATE_PER_SEC = 1/N`로 설정 또는 (c) 호스트별 `SetHostRate(host, 1/N, ...)` 호출이 필요하다. 운영 중 문제 발생 시 `SCHEDULER_HOST_TOKEN_BUCKET_ENABLED=false`로 즉시 롤백 가능하다.

### 실패 보고 & 재시도 backoff

OpenSpec change `scheduler-retry-backoff`가 정의하는 정책. `URLScheduler.RecordFetchError(key, errorKind)` / `RecordHarvestError(key, errorKind)`가 실패 경로를 담당하며, 성공 경로(`fetch_error_count = 0` reset)는 `scheduler-claim-api`의 `SetStatus`가 담당한다.

**errorKind enum**: `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`. 열거 외 값은 에러 반환(row 무변경).

**4xx 즉시 dead 정책**: `errorKind == "http_4xx"`인 경우 `fetch_error_count` / `harvest_error_count`를 공식 적용 없이 즉시 5로 설정한다. `next_*_at`은 갱신하지 않는다(partial index가 이미 dead 조건을 처리). 근거: 404/410/401/403 등은 재시도해도 회복 불가.

**Non-4xx backoff 공식** (Go 애플리케이션이 `time.Now()` 기준으로 계산, DB `random()`은 사용하지 않음):

```
delay      = 30s * 2^(error_count_after - 1)          // error_count_after ∈ [1, 5]
jitter     = uniform[-0.1 * delay, +0.1 * delay]      // math/rand, 정규분포 아님
next_*_at  = T_report + delay + jitter                // T_report = 워커가 관측한 보고 시각
```

- `error_count_after = 1`부터 차례대로 30s / 60s / 120s / 240s / 480s. 최대 delay cap은 `30s * 2^4 = 480s`(8분).
- 다섯 번째 보고 UPDATE 커밋 시 `… = 5`가 되어 partial index 조건(`< 5`)에서 자동 제외 → dead. 별도 `is_dead` 컬럼 없음.
- jitter는 thundering herd 완화용. PRNG는 scheduler 구현체 생성자 레벨에서 캡슐화되어 노출 API 시그니처에 드러나지 않는다.

**dead row 운영 절차**: dead 처리된 row는 frontier 테이블에 그대로 남고 cleanup은 자동으로 수행되지 않는다. 운영자가 수동으로 재활성화하려면 아래 SQL을 psql에서 실행. 본 capability는 런타임 애플리케이션 코드가 backoff 컬럼을 직접 UPDATE하는 것을 금지하지만(`RecordFetchError` / `RecordHarvestError` / `SetStatus` 외부 경로 금지 규약), **운영자의 수동 psql 개입**은 해당 금지 조항의 명시적 예외로 허용된다(scheduler-retry-backoff spec의 "실패 보고 경로 외부에서 backoff 컬럼을 직접 수정하지 않는다" 요구사항의 예외 단서 참조).

```sql
-- (선택) 대상 row의 hex 해시 찾기: normalized_url을 아는 운영자가 UPDATE 전에
-- hex 값을 확인하려면 다음 조회를 먼저 실행한다.
SELECT encode(url_hash, 'hex') FROM pioneer_frontier   WHERE normalized_url = $1;
SELECT encode(url_hash, 'hex') FROM harvester_frontier WHERE normalized_url = $1;

-- Pioneer 측. $1은 대상 URL을 sha256 해시한 hex 문자열(운영자 psql 편의 표기).
-- 애플리케이션 런타임 쿼리는 raw BYTEA 바인딩(`WHERE url_hash = $1`, `$1 = sha256(key)`)을 사용한다.
UPDATE pioneer_frontier
SET fetch_error_count = 0,
    next_fetch_at     = now()
WHERE url_hash = decode($1, 'hex');

-- Harvester 측
UPDATE harvester_frontier
SET harvest_error_count = 0,
    next_harvest_at     = now()
WHERE url_hash = decode($1, 'hex');
```

## Snapshot Storage (Pioneer → Harvester handoff)

`pioneer-snapshot-storage` change에서 도입. Pioneer가 fetch에 성공한 raw HTML을
object storage에 보관하여, 후속 Harvester가 같은 바이트를 재요청 없이 재사용할 수
있게 한다(`harvester-snapshot-first-fetch`가 이 데이터의 첫 소비자).

### 키 규칙 (행위 계약 — Pioneer/Harvester 공유)

- 패턴: `snapshots/<sha256_hex>/<yyyymmdd>.html.gz`
  - `<sha256_hex>`: normalized URL의 sha256 hex digest, 정확히 64자 소문자
  - `<yyyymmdd>`: UTC 기준 fetch 날짜
- Go 상수/함수: `apps/api/internal/bot/snapshot.SnapshotKeyPattern`,
  `snapshot.SnapshotKey(normalizedURL, t)`. Harvester change는 같은 함수를 호출해
  키를 재구성한다.
- 동일 URL의 같은 UTC 날짜 재fetch는 같은 키에 덮어쓴다(idempotent).

### 저장 형식

- gzip 압축 (`compress/gzip` 표준 라이브러리). 객체에는 `Content-Type: text/html`,
  `Content-Encoding: gzip`이 부여된다.
- gzip trailer의 CRC-32가 손상 감지 역할을 겸하므로 별도 MD5/SHA 체크섬은 두지 않는다.
  손상이 감지되면 Harvester는 snapshot miss로 처리하고 HTTP fallback한다.

### TTL 및 lifecycle

- 365일 후 만료. 버킷의 `snapshots/` prefix에 lifecycle rule을 설정해 운영(infra
  owner action). 애플리케이션 코드는 TTL 시점을 알 필요 없다.

### 동시 쓰기 정책

- last-write-wins. object storage의 atomic PUT 동작에 위임하며, 애플리케이션 레벨의
  lock/version/조건부 헤더는 사용하지 않는다.

### 운영 토글 & 롤백

- 환경변수: `PIONEER_SNAPSHOT_ENABLED` (기본 `false`),
  `PIONEER_SNAPSHOT_BUCKET` (생략 시 `S3_BUCKET`).
- helm: `cronjob-bot.yaml`에 두 변수를 `optional: true`로 노출. ConfigMap 키는
  각각 `pioneer-snapshot-enabled`, `pioneer-snapshot-bucket`.
- **롤아웃**: 스테이징에 flag on → 24시간 모니터링(업로드 성공률, 지연, 용량 증가)
  → 운영에 점진적으로 on.
- **롤백**: ConfigMap에서 `pioneer-snapshot-enabled`를 제거하거나 `"false"`로
  설정 → 다음 CronJob 실행부터 업로드 즉시 중단. 이미 저장된 객체는 365일 TTL로
  자연 소멸하며 별도 클린업 작업은 불필요하다.

### 후속 change 의존성

- `harvester-snapshot-first-fetch`: 위 키 규칙·gzip 포맷·last-write-wins 전제를
  공유 소비. 키 함수는 같은 `snapshot` 패키지를 import하여 결정성을 유지한다.

## 인프라

tech-stack.md 참조. Terraform + EKS + ArgoCD.
