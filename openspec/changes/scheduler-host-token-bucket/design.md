## Context

`scheduler-frontier-table` change로 `bot_frontier` 테이블이 도입되어 Pioneer/Harvester가 동일 frontier에서 URL을 claim하게 된다. 그러나 frontier 자체는 우선순위 정렬만 제공하고, 동일 호스트로의 요청률을 제한하지 않는다. URLScheduler dequeue 흐름에서 host별 politeness를 적용해야 외부 사이트에 대한 매너 있는 크롤링이 가능하다.

본 change는 **호스트별 token bucket**을 인메모리로 유지하고 claim 시점에 token 가용성을 검사하는 동작을 정의한다. 토큰 버킷은 사용자 결정에 따라 채택되었다(다른 후보로 leaky bucket, fixed window 등은 검토 단계에서 제외).

스케줄러 프로세스는 복수로 운영될 수 있으며, 본 design은 **프로세스 간 조율 없는** 단순 모델을 채택한다. 외부 사이트 입장에서 본 실효 rate는 (프로세스 수 × 호스트 rate)가 되며, 운영자가 두 값을 조합해 튜닝하는 책임을 진다.

## Goals / Non-Goals

**Goals:**
- 호스트별 token bucket으로 claim 시점 politeness를 강제한다.
- 한 호스트가 token 부족이면 다른 호스트의 후보로 즉시 fallback하여 처리량 손실을 최소화한다.
- 모든 후보가 blocked일 때의 동작(busy-wait sleep)을 명확히 정의한다.
- 기본 rate/burst와 호스트별 override(특히 robots.txt Crawl-delay) 인터페이스를 정의한다.

**Non-Goals:**
- robots.txt 자체의 fetch/파싱 — `pioneer-link-filter-policy` 범위.
- URLScheduler Go interface 시그니처 정의 — `scheduler-claim-api` 범위.
- frontier 테이블 컬럼/인덱스 — `scheduler-frontier-table` 범위.
- 프로세스 간 token 조율(분산 rate limit) — 본 change에서는 명시적으로 제외.
- 호스트별 동시 inflight 카운트 제한(semaphore 류) — token bucket과 별도 개념이며 본 change 범위 외.

## Decisions

### Decision 1: Token bucket 알고리즘과 라이브러리

**선택**: `golang.org/x/time/rate.Limiter`를 호스트별로 인스턴스화하여 사용한다.

**대안**:
- (A) 직접 구현(time.Now 기반 토큰 누적)
- (B) `golang.org/x/time/rate` (채택)
- (C) 외부 라이브러리(`uber-go/ratelimit` 등 leaky bucket)

**근거**:
- `x/time/rate`는 Go 표준 준-표준 라이브러리로 추가 의존성 부담이 작고, `Allow()`/`Reserve()`/`Wait()` API가 본 design의 "체크 후 즉시 진행 또는 다른 호스트 시도" 패턴에 적합하다.
- burst 개념을 1급으로 지원해 일시적 우선순위 spike(피드 갱신 등) 흡수에 유리.
- leaky bucket(C)은 정확한 간격 제약에 강하지만 Pioneer/Harvester처럼 우선순위 기반 dequeue에서는 burst가 자연스러운 token bucket이 더 잘 맞는다.

### Decision 2: 인메모리 자료구조와 락

**선택**: `map[string]*rate.Limiter` + `sync.RWMutex`. 호스트 키는 `bot_frontier.host` 컬럼 값을 그대로 사용.

**대안**:
- (A) `sync.Map`
- (B) `map` + `RWMutex` (채택)
- (C) 호스트별 sharded map

**근거**:
- 호스트 수는 보통 수천 단위 이내로 예상되어 단일 RWMutex로 충분.
- 신규 호스트 등록 시 write lock, 일반 조회는 read lock으로 분리하여 일반 경로에서 락 경합 최소화.
- `sync.Map`은 Limiter 포인터 lazy 생성 패턴에서 race 처리가 번거로움.
- 메모리 회수: 일정 시간 미사용 호스트의 Limiter를 GC하는 정책은 본 change 범위 외(메모 수준으로 Open Question에 기록).

### Decision 3: claim 흐름에서의 token 체크 위치

**선택**: URLScheduler가 frontier에서 후보 N개를 score 순으로 가져온 뒤, 각 후보에 대해 host bucket의 `Allow()`를 호출. 통과한 첫 후보를 claim한다. 통과 후보가 없으면 짧은 sleep 후 재시도.

**대안**:
- (A) DB 쿼리에서 host별 가용 row만 SELECT (host당 throttle을 SQL로 표현)
- (B) 후보 N개를 가져와 application에서 필터 (채택)
- (C) host bucket의 `Reserve()`로 대기 시각을 받아 가장 빨리 가능한 host 선택

**근거**:
- (A)는 host bucket 상태가 인메모리이므로 SQL로 노출하기 어렵고, 프로세스별로 상태가 달라 단일 쿼리 표현이 부자연스럽다.
- (B)는 frontier가 score 우선순위 정렬을 이미 보장하므로, 상위 N개 안에서 host 다양성이 어느 정도 확보되면 처리량 손실이 작다. N은 설정값(예: 16~64).
- (C)는 정확한 wait 계산은 가능하나 dequeue 호출자에게 wait 책임을 떠넘겨 코드 복잡도가 증가. 본 change는 단순화를 위해 `Allow()` 기반의 try/skip 모델 채택.

### Decision 4: 모든 후보 blocked 시 동작

**선택**: 후보 N개가 모두 token 부족이면 짧은 시간(기본 100ms) sleep 후 다시 frontier를 조회한다. 이 sleep은 설정 가능하며, 상한(예: 1s)을 두어 polling 간격이 무한히 늘어나지 않게 한다.

**대안**:
- (A) Exponential backoff (100ms → 200ms → … → 1s)
- (B) 고정 sleep (채택, 단순)
- (C) `Reserve()` 기반의 가장 빠른 wait 시각까지 정확히 sleep

**근거**:
- 본 change 범위에서는 단순성을 우선. Exponential backoff는 운영 데이터로 필요성이 확인되면 후속에서 도입.
- (C)는 Decision 3의 `Allow()` 모델과 충돌. 후속 개선 후보로 Open Questions에 기록.

### Decision 5: 기본 rate/burst와 설정 surface

**선택**:
- 기본 rate: **1 req/sec per host**
- 기본 burst: **5**
- 설정 키: `scheduler.host_default_rate_per_sec`, `scheduler.host_default_burst`
- 호스트별 override: 단일 호스트의 rate/burst를 외부에서 set할 수 있는 메서드를 노출(예: `SetHostRate(host, rate, burst)`).

**근거**:
- 1 req/sec은 일반적인 크롤러 politeness 디폴트와 일치.
- burst 5는 같은 호스트 내 작은 페이지 그룹을 짧게 처리하면서도 평균 1 req/sec를 유지하는 합리적 값.
- override 메서드를 둠으로써 robots.txt Crawl-delay나 운영자 수동 조정이 동일 진입점을 사용.

### Decision 6: robots.txt Crawl-delay 반영

**선택**: 본 change는 `SetHostRate(host, rate, burst)` 인터페이스만 제공한다. `pioneer-link-filter-policy`에서 robots.txt를 파싱한 결과(Crawl-delay 초)를 `rate = 1/Crawl-delay`, `burst = 1`(보수적)로 환산해 호출하는 책임은 Pioneer 측이 진다.

**대안**:
- (A) scheduler가 robots.txt를 직접 fetch하여 rate를 자동 적용
- (B) Pioneer가 파싱하고 scheduler.SetHostRate 호출 (채택)

**근거**:
- robots.txt 파싱은 Pioneer 책임 영역(`pioneer-link-filter-policy`). scheduler는 host 정책의 단일 적용지점만 제공.
- 이중 fetch/파싱 방지.

## Risks / Trade-offs

- **프로세스 간 조율 없음** → 외부 사이트가 보는 실효 rate는 프로세스 수 × 호스트 rate. → 운영자가 프로세스 수와 rate를 함께 튜닝. 보수적 디폴트(1 req/sec)와 호스트 override로 완화. 분산 조율은 후속 change(`scheduler-distributed-rate-limit` 등)에서 검토.
- **Limiter 메모리 누수 가능** → 한 번 본 호스트의 Limiter는 영구 보유. 호스트 다양성이 큰 환경에서 메모리 증가. → 본 change 범위 외이며 Open Question에 기록. 운영 모니터링 후 LRU/TTL 정리 정책 추가.
- **모든 후보 blocked 시 처리량 손실** → host 다양성이 낮은 frontier(특정 사이트 집중) 상황에서 sleep 비중 증가. → N(후보 개수)을 늘리거나, host별 quota를 frontier 쿼리에서 일부 다양화하는 후속 개선 가능.
- **busy-wait sleep의 CPU/DB 부담** → sleep 100ms 단위 polling이 다수 워커에서 발생하면 frontier 쿼리가 누적. → sleep 상한(1s)과 워커 수 운영으로 완화.
- **`Allow()` 기반 try/skip의 우선순위 왜곡** → 최상위 score 후보가 host blocked이면 차상위 후보가 먼저 처리되어 score 정렬이 일시적으로 깨짐. → 의도된 trade-off. 정확한 score 보장이 필요한 경우 후속에서 `Reserve()` 모델로 전환 검토.

## Migration Plan

1. 본 change는 `scheduler-claim-api`의 URLScheduler 구현 일부로 동작한다. 본 change 단독으로는 사용처가 없으며, 후속 change에서 dequeue 호출 경로에 host bucket 검사를 삽입한다.
2. `golang.org/x/time/rate` 의존성을 `go.mod`에 추가.
3. 기본 rate/burst 설정값을 환경변수 또는 config 파일에 노출. 기본값(1 rps, burst 5)으로 시작.
4. Pioneer가 robots.txt 파싱 결과를 가지고 `SetHostRate`를 호출하는 통합은 `pioneer-link-filter-policy` change와 짧게 묶어 진행.
5. 롤백: scheduler 모듈에서 token bucket 검사를 비활성화하는 feature flag(`scheduler.host_token_bucket_enabled`, 기본 true)를 함께 제공하여, 문제가 발생하면 false로 즉시 비활성화 가능.

## Open Questions

- **Limiter GC 정책**: 일정 시간(예: 1시간) 미사용 호스트의 Limiter를 정리할지. 운영 후 메모리 사용량 보고 결정.
- **`Reserve()` 모델 전환 시점**: 정확한 score 우선순위 보장이 필요해지는 시점이 오면 후속 change로 전환 검토.
- **호스트 키 정규화**: `host` 컬럼에 포트가 포함된 경우(`example.com:8080`)와 그렇지 않은 경우의 정책. 본 change는 frontier에 저장된 값을 그대로 키로 사용한다고 가정.
- **분산 조율 필요 시점**: 프로세스 수가 N개를 초과하여 외부 사이트 차단이 빈번해지면 분산 rate limit(redis 기반 등) 도입. 본 change 범위 외.
