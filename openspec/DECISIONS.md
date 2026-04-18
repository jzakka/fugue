# Review Decisions (2026-04-17)

openspec-review 결과 확정된 설계 결정. 모든 change는 이 문서를 참조하여 proposal/design/spec/tasks를 수정한다.

## 1. 스키마 분리 (scheduler-frontier-table)

두 테이블로 완전 분리. `queue_type` 컬럼 방식이 아닌 **독립 테이블**.

### `pioneer_frontier`
| 컬럼 | 타입 | NULL | DEFAULT |
|------|------|------|---------|
| `id` | `BIGSERIAL PRIMARY KEY` | NOT NULL | — |
| `normalized_url` | `TEXT` | NOT NULL | — |
| `url` | `TEXT` | NOT NULL | — |
| `url_hash` | `BYTEA` (sha256, 32바이트) | NOT NULL | — |
| `host` | `TEXT` | NOT NULL | — |
| `depth` | `INTEGER` | NOT NULL | `0` |
| `score` | `DOUBLE PRECISION` (0.0~1.0) | NOT NULL | `0` |
| `last_fetched_at` | `TIMESTAMPTZ` | NULL | `NULL` |
| `next_fetch_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |
| `fetch_error_count` | `INTEGER` | NOT NULL | `0` |
| `last_updated_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |

- `UNIQUE(url_hash)`
- Partial index: `WHERE fetch_error_count < 5 ORDER BY score DESC, next_fetch_at ASC`

### `harvester_frontier`
| 컬럼 | 타입 | NULL | DEFAULT |
|------|------|------|---------|
| `id` | `BIGSERIAL PRIMARY KEY` | NOT NULL | — |
| `normalized_url` | `TEXT` | NOT NULL | — |
| `url` | `TEXT` | NOT NULL | — |
| `url_hash` | `BYTEA` | NOT NULL | — |
| `host` | `TEXT` | NOT NULL | — |
| `snapshot_key` | `TEXT` | NULL | `NULL` |
| `score` | `DOUBLE PRECISION` | NOT NULL | `0` |
| `harvested_at` | `TIMESTAMPTZ` | NULL | `NULL` |
| `next_harvest_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |
| `harvest_error_count` | `INTEGER` | NOT NULL | `0` |
| `last_updated_at` | `TIMESTAMPTZ` | NOT NULL | `now()` |

- `UNIQUE(url_hash)`
- Partial index: `WHERE harvested_at IS NULL AND harvest_error_count < 5 ORDER BY score DESC, next_harvest_at ASC`

### `harvester_frontier_pins` (1:N 조인)
```sql
CREATE TABLE harvester_frontier_pins (
  frontier_id BIGINT NOT NULL REFERENCES harvester_frontier(id) ON DELETE CASCADE,
  pin_id BIGINT NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
  PRIMARY KEY (frontier_id, pin_id)
);
```

### `host` 컬럼 형식
- 호스트명만 (포트 제외)
- 대소문자 **원본 유지**
- `www.` prefix **유지**

### URL 정규화 → `url_hash`
`url_hash = sha256(normalized_url)` (32바이트 BYTEA). URL 길이 무관하게 unique 제약 가능.

## 2. Queue Split / Fanout (HIGH #4)

Pioneer가 fetch + 링크 발견. 새 링크는 `pioneer_frontier`에, fetch된 원본은 `harvester_frontier`에 fanout.

```
pioneer_frontier ─ Pioneer 워커 ─┬─> pioneer_frontier (새 링크)
                                 └─> harvester_frontier (원본 + snapshot_key)

harvester_frontier ─ Harvester 워커 ─> pins + harvester_frontier_pins
```

## 3. Scheduler API (scheduler-claim-api)

### Dequeue
- `Dequeue(queueType QueueType) (url string, err error)` — queueType은 `pioneer` 또는 `harvester`
- **내부 blocking**. 빈 큐면 1초 sleep 후 재시도 (고정 간격, 상한 없음)
- Claim 후보 N: 기본 1, env `SCHEDULER_CLAIM_CANDIDATE_N`로 조절
- Claim 프로토콜:
  1. `SELECT FOR UPDATE SKIP LOCKED` 상위 N rows (partial index ORDER BY)
  2. 각 row의 host에 대해 `HostRateLimiter.Allow(host)` 호출
  3. 첫 true row를 claim: `UPDATE next_fetch_at = now() + 10min` (in-flight marker = lease_timeout)
  4. URL 반환
  5. 모두 false면 sleep 1초 후 재시도

### Lease
- `next_fetch_at` / `next_harvest_at`을 in-flight marker로 **재활용**
- Lease timeout 10분. 크래시 시 자동 회수

### SetStatus
- `SetStatus(key string, status string, pinIDs []int64) error`
- `status` enum: `"fetched"`, `"fetch_failed"`, `"harvested"`, `"harvest_failed"`
- `"harvested"` 시: `harvested_at = now()` + `pinIDs`를 `harvester_frontier_pins`에 INSERT (트랜잭션 내)
- `"fetched"` 시: Pioneer 성공 → `next_fetch_at = now() + 365 days` (재크롤)

### RecordFetchError / RecordHarvestError
- `RecordFetchError(key string, errorKind string) error`
- `errorKind` enum: `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`
- `"http_4xx"`: 즉시 dead (`fetch_error_count = 5`)
- 기타: `fetch_error_count++`, `next_fetch_at` backoff 갱신

### Consumer 호출 규약
실패 시 **SetStatus + RecordXxxError 둘 다 호출**.

## 4. Retry Backoff (scheduler-retry-backoff)

- 공식: `delay = 30s * 2^(error_count - 1) + jitter`, `jitter = uniform[-0.1*delay, +0.1*delay]`
- 계산 위치: **Go app** (DB now() 미사용)
- Dead: `error_count >= 5`
- 4xx는 RecordFetchError에서 즉시 `error_count = 5`로 설정 (공식 적용 안 함)
- spec 헤더: `## MODIFIED Requirements` (scheduler capability)

## 5. Host Token Bucket (scheduler-host-token-bucket)

- 기본: 1 req/sec, burst 5
- `rate <= 0` 또는 `burst <= 0`: 기본값 대체 + 경고 로그 (서비스 중단 없음)
- 인메모리 `map[string]*rate.Limiter` + `sync.RWMutex`
- `SetHostRate(host, rate, burst)`로 robots.txt Crawl-delay 반영 (Pioneer가 호출)
- 키: `host` 컬럼 값 그대로 (위 §1 호스트 형식)

## 6. Worker Budget (pioneer/harvester-worker-budget)

- **카운팅**: 성공한 Dequeue만 (URL 반환된 경우). idle/error는 카운트 제외
- **정책값**: 100회 빌드 상수 고정, env 노출 **금지**
- **체크 위치**: 성공 Dequeue 직후
- 임계 도달 시: 현재 작업(fetch/harvest)을 완료한 후 `exit 0`
- Pioneer 워커도 위 규칙 동일 적용 (원래 "루프 이터레이션 기준"에서 변경)

## 7. Snapshot (pioneer-snapshot-storage, harvester-snapshot-first-fetch)

- **해시**: `sha256(normalized_url)` → hex 64자
- **키 포맷**: `snapshots/<sha256_hex>/<yyyymmdd>.html.gz`
- **TTL**: 365일 (object storage lifecycle rule)
- **압축**: gzip
- **Harvester spec**: 키 포맷을 자체 기술하지 않고 "`pioneer-snapshot-storage` capability의 키 규약을 따른다"로 참조
- **동시 쓰기**: object storage last-write-wins (별도 처리 없음)

## 8. Re-Crawl 정책

- **Pioneer**: 성공 시 `next_fetch_at = now() + 365 days` 기본. 실제 재크롤 정책은 후속 change로
- **Harvester**: 재harvest 안 함. Pioneer 재크롤 시 UPSERT는 `WHERE harvested_at IS NULL`만 적용 → 이미 harvest된 URL은 no-op
- UPSERT 쿼리:
```sql
INSERT INTO harvester_frontier (...)
VALUES (...)
ON CONFLICT (url_hash) DO UPDATE
  SET snapshot_key = EXCLUDED.snapshot_key,
      next_harvest_at = now(),
      harvest_error_count = 0
  WHERE harvester_frontier.harvested_at IS NULL
```

## 9. 콘텐츠 (harvester-image-cache, harvester-pin-document)

- **cacheImage 시그니처**: `(url string, err error)`. 실패 시 원본 URL을 `url`로 반환. 에러는 로그만
- **thumbnail_url ↔ og_image**: 동일한 값을 두 컬럼에 기록 (단순화)
- **body_text**: `og_data`에 **저장하지 않음**. `pins.description`에 500자 잘라 저장
- **ScriptAdapter N→1**: 첫 RawItem을 정본. 나머지는 `og_data.media_candidates`
- **Cross-domain canonical**: canonical URL이 fetch URL과 다른 도메인이면 무시. `canonical_url = fetch_url`
- **Classifier reasons**: `listing`, `empty_body`, `no_primary_media`만 유지. `low_text_link_ratio` **제거**
- **image-cache edge case 테스트**: 기본 성공/실패 케이스만. URL 정규화 실패/hash 충돌 등은 후속

## 10. Link Filter Policy (pioneer-link-filter-policy)

- **archive 2026-04-13-pioneer-link-filter-impl을 un-archive**하여 본 change가 MODIFIED로 덮어쓰기
- **DomainFilter**: `RootDomain` 단일 필드 → `AllowKeywords []string`, `DenyKeywords []string`
- **canonicalURL 추가 규칙**:
  - scheme 소문자화
  - default port 제거 (`:80` for http, `:443` for https)
  - query 파라미터 이름순 오름차순 정렬
- **RobotsFilter**: 신규. robots.txt fetch/캐싱/파싱
- **필터 순서**: Domain → Extension → PathPattern → Robots → Dedup
- **Redirect chain**: 최종 URL만 체크 (중간 URL 무시)
- **국가별 TLD**: substring 매칭 그대로 (Open Question 종결)
- **Robots filter ↔ host bucket 순서**: Enqueue 단계에서 Robots, Claim 단계에서 host bucket

## 11. ObjectStorage 실패 처리 (harvester-snapshot-first-fetch)

- 모든 실패 유형(key 없음/만료/네트워크/권한/내부 에러)을 단일 "miss"로 취급 → HTTP fallback
- 로그 레벨에서만 에러 종류 구분 (운영 분석용)
- Task 의존 순서: tasks.md에 "5.2는 Task 2 완료 후" 등 명시

## 12. bot spec REMOVED 타이밍 (harvester-scheduler-consumer)

- 본 change에서 직접 `bot/spec.md`의 관련 requirement REMOVED 처리
- `scheduler-frontier-table`을 **prerequisite**로 tasks.md에 명시

## 13. filterLinks 단계 책임 (pioneer-scheduler-consumer)

- Pioneer consumer가 `FilterChain.Apply()` 호출 (기존 구조 유지)
- `pioneer-link-filter-policy`는 필터 "내용"(DomainFilter 정책, RobotsFilter 신규 등)만 정의
- 두 change 간 책임: policy = 필터 정의, consumer = 호출 타이밍
