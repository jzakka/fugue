## Context

`scheduler-frontier-table` change로 `bot_frontier` 테이블이 도입되어 Pioneer/Harvester가 동일 frontier에서 URL을 claim하게 된다. 그러나 frontier 자체는 우선순위 정렬만 제공하고, 동일 호스트로의 요청률을 제한하지 않는다. URLScheduler dequeue 흐름에서 host별 politeness를 적용해야 외부 사이트에 대한 매너 있는 크롤링이 가능하다.

본 change는 **호스트별 token bucket**을 인메모리로 유지하고 claim 시점에 token 가용성을 검사하는 동작을 정의한다. 토큰 버킷은 사용자 결정에 따라 채택되었다(다른 후보로 leaky bucket, fixed window 등은 검토 단계에서 제외).

스케줄러 프로세스는 복수로 운영될 수 있으며, 본 design은 **프로세스 간 조율 없는** 단순 모델을 채택한다. 외부 사이트 입장에서 본 실효 rate는 (프로세스 수 × 호스트 rate)가 되며, 운영자가 두 값을 조합해 튜닝하는 책임을 진다.

## Goals / Non-Goals

**Goals:**
- 호스트별 token bucket의 행위 계약(허용-여부 질의 동작, 호스트 rate/burst 설정 동작)을 정의한다.
- 기본 rate/burst와 호스트별 override(특히 robots.txt Crawl-delay) 진입점을 정의한다.
- 운영 롤백 수단으로서 token bucket 검사 비활성화 설정의 행위 계약을 정의한다.

**Non-Goals:**
- robots.txt 자체의 fetch/파싱 — `pioneer-link-filter-policy` 범위.
- URLScheduler Go interface 시그니처 정의 — `scheduler-claim-api` 범위.
- claim 시점의 후보 iteration(상위 N개 후보에 대한 host 허용-여부 검사 패턴)과 모든 후보 blocked 시 sleep/backoff 동작 — `scheduler-claim-api` 범위.
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

**선택**: `map[string]*rate.Limiter` + `sync.RWMutex`. 호스트 키는 `scheduler-frontier-table`의 `host` 컬럼 값(Pioneer 측 정규화 결과: 호스트명만, 포트 제외, 대소문자 원본 유지, `www.` prefix 유지)을 그대로 사용한다. 정규화 책임은 Pioneer(`pioneer-link-filter-policy`)에 있으며, scheduler는 저장된 값을 그대로 키로 취급한다.

**대안**:
- (A) `sync.Map`
- (B) `map` + `RWMutex` (채택)
- (C) 호스트별 sharded map

**근거**:
- 호스트 수는 보통 수천 단위 이내로 예상되어 단일 RWMutex로 충분.
- 신규 호스트 등록 시 write lock, 일반 조회는 read lock으로 분리하여 일반 경로에서 락 경합 최소화.
- `sync.Map`은 Limiter 포인터 lazy 생성 패턴에서 race 처리가 번거로움.
- 메모리 회수: 일정 시간 미사용 호스트의 Limiter를 GC하는 정책은 본 change 범위 외(메모 수준으로 Open Question에 기록).

### Decision 3: 호출 패턴(참고용, 본 change 범위 외)

**참고**: claim 흐름에서 본 change가 정의한 허용-여부 질의 동작이 어떻게 호출되는지(예: 상위 N개 후보를 score 순으로 가져와 각 후보의 host에 허용-여부 질의를 호출하고 첫 통과 후보를 claim, 통과 후보가 없으면 짧은 sleep)는 `scheduler-claim-api` change의 Claim 프로토콜에서 정의된다. 본 design은 호출 패턴을 결정하지 않으며, 본 capability는 두 메서드의 행위 계약만 제공한다.

### Decision 4: 모든 후보 blocked 시 동작(참고용, 본 change 범위 외)

**참고**: 모든 후보 host가 token 부족일 때의 sleep/backoff/Reserve 모델 선택은 `scheduler-claim-api`의 책임이다. 본 capability는 sleep 자체를 정의하지 않으며 관련 설정 키도 정의하지 않는다. 본 design에서는 의사결정 기록을 두지 않고 claim-api 쪽에서 다룬다.

### Decision 5: 기본 rate/burst와 설정 surface, Go 시그니처

**선택**:
- 기본 rate: **1 req/sec per host**
- 기본 burst: **5**
- 설정 키: `scheduler.host_default_rate_per_sec`, `scheduler.host_default_burst`
- Go 시그니처(receiver 포함):
  - `func (l *HostRateLimiter) SetHostRate(host string, rate float64, burst int)` — 지정 호스트의 rate/burst를 즉시 반영(Limiter 재생성). 신규 호스트면 새 Limiter 생성.
  - `func (l *HostRateLimiter) Allow(host string) bool` — 호스트 bucket의 token이 있으면 true와 함께 token 1개 소비, 없으면 false. 호스트 bucket이 없으면 기본 rate/burst로 lazy 생성 후 판정.
- 호스트별 override는 `SetHostRate`로 일원화(robots.txt Crawl-delay, 운영자 수동 조정 모두 동일 진입점).

**근거**:
- 1 req/sec은 일반적인 크롤러 politeness 디폴트와 일치.
- burst 5는 같은 호스트 내 작은 페이지 그룹을 짧게 처리하면서도 평균 1 req/sec를 유지하는 합리적 값.
- `Allow`/`SetHostRate` 두 메서드만 외부로 드러내어 claim-api 등 소비자와의 결합도를 최소화.

### Decision 5a: rate/burst 유효성 정책

**선택**: `SetHostRate`에 `rate <= 0` 또는 `burst <= 0`이 들어오면 해당 호스트 bucket을 **현재 운영자 설정 기본값**(`scheduler.host_default_rate_per_sec`, `scheduler.host_default_burst`)으로 생성/대체하고 **경고 로그**(`WARN`)를 남긴다. 운영자가 별도 설정을 하지 않은 경우 공장 기본값(1 req/sec, burst 5)이 적용된다. 서비스는 중단되지 않고 호출자에게 에러를 반환하지도 않는다(void 시그니처 유지).

**근거**:
- 잘못된 robots.txt 값(`Crawl-delay: 0`, 음수 등)이나 운영자 실수로 인한 입력이 크롤링 파이프라인을 중단시키지 않도록 안전 기본값을 적용.
- 호출부(Pioneer)가 정상/비정상 분기 로직을 가질 필요 없이 단순하게 scheduler에 값을 위임할 수 있음.
- 운영자가 default를 더 보수적으로(예: 0.5 rps) 변경한 환경에서는 그 보수적 값이 fallback으로도 적용되는 것이 안전.
- 경고 로그는 운영 분석에서 잘못된 입력 패턴을 발견하는 용도.

**대안 기각**:
- error 반환 시그니처: Pioneer 호출부 복잡도 증가, 실패 시 fallback 책임을 Pioneer가 중복 구현해야 함.
- panic: 크롤링 파이프라인 전체 중단 위험.

### Decision 5b: claim-api와의 통합

**선택**: 본 change는 `Allow(host) bool`과 `SetHostRate(host, rate, burst)` 두 메서드만 제공한다. claim 시점의 호출(후보 row의 host에 대해 `Allow(host)`를 검사하는 로직)은 `scheduler-claim-api` change의 Claim 프로토콜(`SELECT FOR UPDATE SKIP LOCKED` → 각 row의 host에 `Allow` → 첫 true row claim)에서 정의된다. 즉 claim 호출 타이밍은 claim-api의 spec requirement이며, 본 change는 **메서드의 행위 계약(behavior contract)**만 정의한다.

**근거**:
- 역할 분리: 본 change = rate limiter 동작/계약, claim-api = dequeue 프로토콜.
- Allow 호출이 claim 트랜잭션 안에서 어떻게 엮이는지는 claim-api 책임이어야, 양쪽 change가 독립적으로 진화 가능.

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
- **유효성 대체 시 운영자가 변경한 default와 공장 기본값의 혼동 가능** → 운영자가 default를 변경한 환경에서 잘못된 입력이 들어오면 변경된 default가 적용된다. 로그에는 어떤 값으로 대체되었는지 명시 필요.
- **claim 흐름 관련 risk(모든 후보 blocked 시 처리량 손실, busy-wait sleep의 CPU/DB 부담, try/skip의 우선순위 왜곡)** → 본 change의 행위 계약(`Allow`/`SetHostRate`)만으로는 발생하지 않으며, `scheduler-claim-api`의 dequeue 호출 패턴이 정의될 때 비로소 나타나는 후속 risk이다. 본 design에서는 다루지 않는다.

## Migration Plan

1. 본 change는 호스트별 token bucket의 행위 계약을 제공한다. claim 경로에서의 허용-여부 질의 호출은 `scheduler-claim-api` change에서 통합되며, robots.txt Crawl-delay 파싱 결과를 호스트 rate/burst로 surface하는 호출은 `pioneer-link-filter-policy` change에서 통합된다.
2. `golang.org/x/time/rate` 의존성을 `go.mod`에 추가.
3. 기본 rate/burst 설정값을 환경변수 또는 config 파일에 노출. 기본값(1 rps, burst 5)으로 시작.
4. Pioneer가 robots.txt 파싱 결과를 가지고 `SetHostRate`를 호출하는 통합은 `pioneer-link-filter-policy` change와 짧게 묶어 진행.
5. 롤백: scheduler 모듈에서 token bucket 검사를 비활성화하는 feature flag(`scheduler.host_token_bucket_enabled`, 기본 true)를 함께 제공하여, 문제가 발생하면 false로 즉시 비활성화 가능.

## Open Questions

- **Limiter GC 정책**: 일정 시간(예: 1시간) 미사용 호스트의 Limiter를 정리할지. 운영 후 메모리 사용량 보고 결정.
- **`Reserve()` 모델 전환 시점**: 정확한 score 우선순위 보장이 필요해지는 시점이 오면 후속 change로 전환 검토.
- **분산 조율 필요 시점**: 프로세스 수가 N개를 초과하여 외부 사이트 차단이 빈번해지면 분산 rate limit(redis 기반 등) 도입. 본 change 범위 외.

### Closed Questions

- **호스트 키 정규화** (종결): `scheduler-frontier-table`의 `host` 컬럼 규칙(호스트명만, 포트 제외, 대소문자 원본 유지, `www.` prefix 유지)과 **동일**. 정규화 책임은 Pioneer(`pioneer-link-filter-policy`)에 있고, scheduler는 저장된 값을 그대로 키로 사용한다. 따라서 scheduler 내부에 별도 정규화 로직은 두지 않는다.
