## Context

`scheduler-frontier-table`에서 `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼과 `… < 5` 조건을 포함하는 partial index가 도입됐다. `scheduler-claim-api`는 claim 프로토콜(`Dequeue`)과 성공 경로(`SetStatus`)를 정의한다. 하지만 실패 시 이 컬럼을 어떻게 갱신할지는 아직 정의되지 않았다.

현재 `apps/api/fuguebot_pseudo.go`의 Pioneer/Harvester는 단일 프로세스 내에서 `for retry := 0; retry < 3; retry++` 루프로 즉시 재시도만 수행하고, 실패가 영속화되지 않는다. 워커가 수평 확장되면 실패 row가 즉시 다시 claim되어 같은 원인(예: 5xx, DNS 실패)으로 무한히 재fetch될 위험이 있다.

제약:
- 컬럼/인덱스/claim 쿼리는 이미 `scheduler-frontier-table`과 `scheduler-claim-api`가 정의한 형태에 맞춰야 한다.
- Pioneer와 Harvester가 동일 공식을 공유해야 한다(운영 일관성).
- 성공 경로의 reset은 `SetStatus`(`scheduler-claim-api`) 책임이며 본 change는 실패 경로만 갱신한다.

## Goals / Non-Goals

**Goals:**
- fetch/harvest 실패가 영속 backoff(`next_fetch_at`, `next_harvest_at`)로 표현되어, 어떤 워커가 claim을 시도하더라도 동일한 backoff를 관측한다.
- exponential 성장으로 장애 호스트가 frontier를 점유하는 빈도를 자연스럽게 줄인다.
- ±10% uniform jitter로 동시에 실패한 row들이 같은 시점에 다시 깨어나 thundering herd를 만들지 않는다.
- `error_count >= 5`를 dead 컷오프로 삼아 partial index에서 자동 제외되도록 한다.
- **4xx는 즉시 dead**로 취급하여 회복 불가능한 URL이 5회 재시도 비용을 쓰지 않게 한다.

**Non-Goals:**
- dead row의 삭제/아카이브/재시도 관리 (`cleanup`은 별도 change에서 다룬다).
- host 단위 페이싱(host token bucket은 `scheduler-host-token-bucket`에서 별도로).
- 성공 시 `fetch_error_count = 0` reset — `scheduler-claim-api`의 `SetStatus("fetched" | "harvested:*")` 책임.
- claim 인터페이스 자체 변경 — 본 change는 실패 보고 경로(`RecordFetchError`, `RecordHarvestError`)에만 개입.
- Pioneer/Harvester가 in-process로 몇 번 재시도하는지 — backoff는 scheduler 경계에서 관리하며, in-process 재시도는 스케줄러 관점에서 단일 시도로 본다.

## Decisions

### Backoff 공식

```
delay  = 30s * 2^(error_count - 1)      // error_count는 이번 실패를 반영한 "후"의 값
jitter = uniform[-0.1 * delay, +0.1 * delay]   // uniform 분포 (정규분포 아님)
next_*_at = time.Now() + delay + jitter
```

- `error_count`는 **이번 실패를 반영한 후**의 값을 공식에 넣는다. 즉 첫 실패(0 → 1)면 `2^0 = 1 * 30s = 30s`, 두 번째(1 → 2)면 `2^1 = 2 * 30s = 60s`, ... 네 번째(3 → 4)면 `2^3 = 8 * 30s = 240s`, 다섯 번째(4 → 5)면 `2^4 = 16 * 30s = 480s`가 반영되지만 이 시점에는 이미 dead이므로 claim되지 않는다.
- **최대 delay**: `30s * 2^4 = 480s (8분)`. `error_count <= 5`가 보장되므로 `int64` nanosecond overflow는 없다. cap이 명시적으로 5이므로 `2^n` 폭주 없음.
- **jitter 분포는 uniform**. Go의 `math/rand` (또는 `crypto/rand`)로 `[-0.1, +0.1]` 균일 표집 후 delay에 곱한다. **정규분포(Normal) 아님**을 명시.
- jitter ±10%를 선택한 이유: 같은 사이트가 동시 fetch되더라도 서로 다른 시점에 재시도되도록 분산시키되, 사용자가 기대하는 backoff의 order of magnitude는 유지.
- **대안 고려**:
  - Fixed interval — 운영이 단순하지만 장애 지속 시 재시도 폭주를 막지 못함.
  - Jitterless exponential — 장애 상황에서 herding 발생.
  - Decorrelated jitter (AWS Architecture Blog) — 더 정교하지만 순수 `2^n` 기반에 비해 이해/감사가 어려움. 요구사항은 간단한 공식으로 충분.
  - 정규분포 jitter — 평균 근처 집중으로 herd 분산 효과가 약하며, 경계가 무한이라 운영 가시성이 나쁨. uniform 채택.

### 계산 위치 (Go app)

- delay / jitter / `next_*_at`은 **Go 애플리케이션에서 `time.Now()` 기준**으로 계산한다.
- DB `now()` 및 `random()`은 공식 계산에 사용하지 않는다. 단 `last_updated_at = now()` 같은 단순 타임스탬프는 DB now() 사용 가능.
- 이유: (1) 단위 테스트에서 clock을 주입하여 결정적 검증이 가능, (2) 여러 워커가 관측하는 시간 기준이 Go monotonic clock으로 통일, (3) jitter PRNG가 DB 독립적이라 관찰/디버깅 편의.

**예시 의사 SQL (RecordFetchError, non-4xx):**
```sql
-- delay_seconds, jitter_seconds는 Go가 계산한 값
UPDATE pioneer_frontier
SET fetch_error_count = fetch_error_count + 1,
    next_fetch_at     = $2,   -- time.Now().Add(delay + jitter)
    last_updated_at   = now()
WHERE url_hash = $1
```

**예시 의사 SQL (RecordFetchError, http_4xx 즉시 dead):**
```sql
UPDATE pioneer_frontier
SET fetch_error_count = 5,
    next_fetch_at     = $2,   -- time.Now() (dead라 의미 없지만 일관성 위해 채움)
    last_updated_at   = now()
WHERE url_hash = $1
```

### 에러 종류별 분기

`errorKind` enum: `"http_4xx"` | `"http_5xx"` | `"network"` | `"timeout"`.

| errorKind | 정책 |
|-----------|------|
| `http_4xx` | 즉시 dead. `fetch_error_count = 5`로 set. 공식 **미적용**. |
| `http_5xx` | 공식 적용, `fetch_error_count += 1`. |
| `network` | 공식 적용, `fetch_error_count += 1`. |
| `timeout` | 공식 적용, `fetch_error_count += 1`. |

- 4xx는 Caller(404/410) 또는 Auth(401/403) 모두 "이 URL을 계속 fetch해도 회복 불가"로 간주. 5회 재시도 비용을 절약.
- Harvester 측도 동일 분기(`harvest_error_count`, `next_harvest_at`).
- 에러 종류 분류는 호출부(fetcher)가 책임진다. scheduler는 enum만 받는다.

### Dead 정책

- `fetch_error_count >= 5` 또는 `harvest_error_count >= 5`인 row는 각자의 partial index 조건(`< 5`)에서 벗어나 claim되지 않는다 — 추가 애플리케이션 로직 없이 테이블 스키마만으로 dead가 성립한다.
- **별도 `is_dead` 컬럼은 두지 않는다**. partial index 조건이 단일 진실 원천(Single Source of Truth).
- cleanup(삭제/보관)은 본 change 범위 밖. frontier에 잔류하지만 다시 fetch/harvest되지 않는 상태.

### 책임 분리 (RecordError vs SetStatus)

- **RecordFetchError / RecordHarvestError** (본 change): 실패 경로. `error_count` 증가 또는 4xx 즉시 dead, `next_*_at` backoff 공식 적용.
- **SetStatus** (`scheduler-claim-api`): 성공 경로. `"fetched"` → `fetch_error_count = 0`, `last_fetched_at = now()`. `"harvested:<pin_id>"` → `harvest_error_count = 0`, `pin_id = ...`.
- 본 change는 성공 reset을 재정의하지 않는다. spec에서도 "성공 시 reset은 SetStatus 책임"이라고 참조만 한다.
- Consumer 호출 규약(DECISIONS §3): 실패 시 **SetStatus(...,"fetch_failed")와 RecordFetchError(...errorKind) 둘 다 호출**. SetStatus는 상태 전이만, RecordFetchError는 backoff/카운트 갱신만.

## Risks / Trade-offs

- [장애 호스트의 5회 실패 후 영구 dead 처리] → 수동 또는 후속 cleanup change에서 재활성화 메커니즘을 제공한다. 본 change에서는 운영자가 SQL로 `fetch_error_count = 0, next_fetch_at = now()`로 리셋할 수 있음을 전제.
- [짧은 base(30s)와 낮은 cap(5)] → 일시적 네트워크 오류가 5번 연속 발생하면 dead로 빠질 수 있음. 단일 요청의 in-process 재시도는 scheduler 관점에서는 한 번의 "시도"이므로 실제 순간 장애는 완충된다.
- [4xx 즉시 dead의 과잉 차단] → 일부 사이트가 일시적 401(예: 세션 만료)을 반환할 수 있지만, fetcher가 재로그인을 책임지지 않는 한 재시도해도 의미 없음. 일시적 401은 fetcher 내부에서 해결하거나 운영자 수동 reset.
- [jitter 구현 편차] → 명세는 "±10% uniform 분포"만 요구하고, PRNG 소스는 구현 세부(`math/rand` vs `crypto/rand`)로 남긴다. 테스트에서는 시드 주입 또는 경계값 검증으로 확인.
- [Go clock vs DB clock 불일치] → Go 측이 계산한 `next_fetch_at`이 DB `now()`와 수 초 틀어질 수 있음. 5초 skew는 partial index의 `next_fetch_at <= now()` 판정에 무시 가능한 수준.

## Open Questions

모두 해결됨.

- ~~에러 종류별 차등 backoff?~~ → **해결**: 4xx는 즉시 dead, 기타는 공식 적용.
- ~~공식 계산 위치(Go vs DB)?~~ → **해결**: Go 앱(`time.Now()`).
- ~~jitter 분포(uniform vs normal)?~~ → **해결**: uniform.
- ~~별도 `is_dead` 컬럼 필요?~~ → **해결**: 불필요. partial index로 충분.
- ~~성공 시 reset은 누구 책임?~~ → **해결**: `SetStatus` (`scheduler-claim-api`).
