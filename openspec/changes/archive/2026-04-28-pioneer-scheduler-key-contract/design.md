## Context

`scheduler-claim-api` capability는 `URLScheduler.SetStatus(key, ...)`/`RecordFetchError(key, ...)`/`RecordHarvestError(key, ...)`의 `key` 인자에 대해 다음 계약을 명시한다(`openspec/specs/scheduler/spec.md`의 Requirement "URLScheduler interface 시그니처가 정의된다"):

> `key`는 이전 `Dequeue`가 반환했거나 이전 `Enqueue` 호출에 전달된 URL 문자열과 동치여야 하며, 내부적으로는 해당 URL의 정규화 결과로부터 유도된 `url_hash`로 lookup된다.

본 change의 의도는 이 기존 계약을 **수정/확장하는 것이 아니라**, 정규화가 URL 형태를 변경하는 입력에 대해 계약이 깨지는 회귀를 방지할 수 있도록 명시적인 round-trip invariant 시나리오를 추가하는 것이다. spec delta는 따라서 `## ADDED Requirements`로 작성되며, 위 인용한 한 문장을 시나리오 수준으로 풀어 회귀 테스트 가능한 형태로 만든 것이다.

그러나 현재 구현 `apps/api/internal/scheduler/postgres_scheduler.go`의 `SetStatus`/`RecordFetchError`/`recordError`는 정규화 단계 없이 `hashKey(key) = sha256(key)`만 수행하여 row를 lookup한다(`url_scheduler.go`의 `hashKey`). 한편 `Enqueue`/`EnqueueHarvester`는 입력 URL을 `urlcanon.CanonicalWithHost`로 정규화한 뒤 `sha256(normalized)`로 `url_hash`를 산출한다(`postgres_scheduler.go`의 `normalizeURL`). `Dequeue`는 row의 `url`(raw) 컬럼을 반환한다(`postgres_scheduler.go`의 `tryClaim` 반환값).

정규화가 URL 형태를 변경하는 입력(예: `https://www.pixiv.net` → `https://pixiv.net`)에 대해 다음 hash가 서로 다르다:
- 저장: `sha256("https://pixiv.net")`
- SetStatus: `sha256("https://www.pixiv.net")`

결과적으로 SetStatus는 0 rows를 매치하고 row의 `last_fetched_at`/`fetch_error_count`/`next_fetch_at`이 갱신되지 않는다. lease 만료(10분) 후 동일 row가 재-claim되는 무한 루프가 발생한다.

QA(`pioneer pixiv` 90초 실행) 측정값:
- 39회 fetch 중 14회(36%)가 `WARN scheduler.set_status_fetched: unknown key (row not in frontier)` 발생
- 시드 row(`https://www.pixiv.net`, `normalized_url=https://pixiv.net`)는 `last_fetched_at=NULL` 유지

`urlcanon.Canonical`은 멱등(idempotent) 함수이므로 `Canonical(Canonical(s)) == Canonical(s)` — 따라서 lookup 단계에서 한 번 더 정규화를 적용해도 이미 정규화된 입력에 대해서는 부작용이 없다.

## Goals / Non-Goals

**Goals:**
- `SetStatus`/`RecordFetchError`/`RecordHarvestError`/`EnqueueHarvester`의 lookup이 정규화 일관성을 보장하도록 한다.
- 호출자(Pioneer/Harvester consumer)가 `Dequeue`/`Enqueue` 입력 URL을 그대로 SetStatus류 메서드에 넘기더라도 silent miss가 발생하지 않게 한다.
- 기존 통합 테스트와 행위 호환을 유지한다(behavior 변경 없음, 명시되지 않은 결함만 수정).

**Non-Goals:**
- `Dequeue` 반환값의 시그니처/의미 변경(예: raw URL → normalized URL)은 본 change에서 다루지 않는다(다른 호출 사이트의 raw URL 사용 가능성, 마이그레이션 비용).
- DomainFilter cross-domain 허용 정책 변경.
- bot_graph_nodes 생성 책임 재분배.
- 정규화 알고리즘(`urlcanon.Canonical`) 자체의 변경.

## Decisions

### Decision 1: lookup 측 정규화 (옵션 C 채택)

**선택**: `PGURLScheduler.SetStatus`/`RecordFetchError`/`RecordHarvestError`/`EnqueueHarvester`의 hash 계산 직전에 `urlcanon.Canonical(key)`을 적용한다.

**대안 비교**:

| 옵션 | 변경 위치 | 장점 | 단점 |
|---|---|---|---|
| **A**: Dequeue가 normalized_url 반환 | scheduler 내부 | 호출 사이트 무변경 | 시그니처 변경, 다른 raw URL 사용처 영향 가능 |
| **B**: 호출자(Pioneer/Harvester)가 정규화 후 호출 | bot 레이어 | scheduler 무변경 | 모든 호출 사이트 정정 필요, 새 호출 사이트 추가 시 누락 위험 영구 잔존 |
| **C**: SetStatus 등 내부에서 정규화 | scheduler 내부 | 계약 일관, 새 호출 사이트도 자동 보호, 시그니처 무변경 | scheduler 코드 작은 추가 |

**근거**:
- 기존 spec의 Requirement "URLScheduler interface 시그니처가 정의된다"가 "내부적으로는 해당 URL의 정규화 결과로부터 유도된 `url_hash`로 lookup된다" 라고 이미 명시 — 옵션 C는 spec과 impl을 정렬시키는 것이지 새로운 계약을 도입하는 것이 아니다.
- 옵션 B는 미래 호출 사이트 추가 시 동일 결함이 재발할 수 있어 방어 깊이가 얕다.
- 옵션 A는 raw URL이 필요한 다른 경로(예: 향후 fetcher로 raw URL을 그대로 넘기고 싶을 때)에 제약을 만든다.

### Decision 2: 정규화 헬퍼 일관화

**선택**: `urlcanon.Canonical`을 정규화 SSOT로 유지한다. `hashKey(normalized) []byte`(기존 `sha256` wrapper)는 **enqueue 경로 전용**(호출 직전에 이미 `normalizeURL`을 거쳐 정규화된 입력을 받는 경로)으로 한정한다. **lookup 경로**(SetStatus / RecordFetchError / RecordHarvestError, 그리고 EnqueueHarvester의 lookup 단계)는 별도 헬퍼 `hashLookupKey(rawKey) []byte`를 사용하며, 이 함수는 내부에서 `Canonical`을 호출한 뒤 `sha256`을 계산한다.

**근거**:
- enqueue 경로(`Enqueue`/`prepareEnqueueBatch`)는 이미 `normalizeURL`로 정규화한 결과를 변수로 들고 있으므로 그 변수에 `hashKey`를 적용하면 충분하다 — 이중 정규화 비용을 피한다.
- lookup 경로는 호출자가 raw URL을 넘길 수 있으므로 헬퍼 내부에서 정규화를 강제한다 — 새 호출 사이트가 추가되어도 자동 보호.
- 함수를 분리하면 호출자가 어느 경로인지 명시적으로 선택하게 되어, 정규화 누락이 코드 리뷰에서 가시화된다.

### Decision 3: 정규화 실패 정책

**선택**: `Canonical`이 빈 문자열을 반환하거나(`urlcanon.Canonical`은 파싱 실패 시 빈 문자열 반환) 입력이 빈 문자열이면, lookup은 DB 호출을 수행하지 않고 정상 반환한다. 에러도 반환하지 않는다.

**전제**: `Enqueue`/`EnqueueHarvester`는 정규화가 빈 문자열을 반환하는 입력을 거부하여 frontier에 빈 url을 가진 row를 쓰지 않는다(`prepareEnqueueBatch`/`normalizeURL`이 parse error를 반환하는 경로). 따라서 lookup 단계에서 빈 정규화 결과로 hash를 만들어 DB 조회하더라도 매칭되는 row가 없는 것이 정상이지만, 만일의 회귀(예: 마이그레이션 도구가 직접 빈 url을 INSERT) 가능성을 차단하기 위해 lookup 헬퍼가 빈 입력을 조기에 short-circuit하는 편이 안전하다.

**근거**:
- 기존 SetStatus 시맨틱은 0 rows match를 에러가 아닌 warning으로 다룬다(`warnIfUnknownKey`). short-circuit해도 외부 행위는 동일.
- 정규화 불가 입력은 상위 레이어 호출 계약 위반이며, 이 경로에서 panic이나 에러를 일으키면 전체 워커가 죽는다(spec: "한 URL이 워커를 죽이지 않아야 한다").

### Decision 4: 회귀 테스트 범위

**선택**: 신규 테스트는 정규화가 URL 형태를 변경하는 두 가지 입력에 대해 round-trip을 검증한다:
- `https://www.host/` → `https://host/` (`www.` 제거)
- `https://host/page#frag` → `https://host/page` (fragment 제거)

각 입력에 대해 Enqueue → Dequeue → SetStatus(fetched) round-trip 후 row의 `last_fetched_at`이 non-NULL이 됨을 통합 테스트로 확인한다.

추가로 `RecordFetchError` round-trip 테스트도 동일 패턴으로 추가한다.

## Risks / Trade-offs

- **Risk**: `urlcanon.Canonical`이 서로 다른 입력에 대해 동일 결과를 내는 경우(예: `?a=1&b=2`와 `?b=2&a=1`을 모두 정렬하여 동일 normalized로 만든다면), SetStatus 호출이 의도치 않은 row를 매치할 수 있다. → **Mitigation**: `Enqueue`도 동일 `Canonical`을 거쳐 `url_hash`를 만들므로 충돌은 enqueue 시점에 이미 dedup됨(`UNIQUE(url_hash)`). lookup 측 정규화가 enqueue 측 정규화와 동일 함수면 신규 충돌 없음.
- **Risk**: 기존 frontier에 누적된 row 중 `last_fetched_at=NULL`로 무한 루프 중인 row들이 다음 fetched 마크 시점에 일제히 갱신됨 — 단발성 부하. → **Mitigation**: lease 시점이 분산되어 있어 동시성 spike 없음. 정상 path로 수렴.
- **Trade-off**: 본 change는 SetStatus/RecordFetchError/RecordHarvestError의 lookup hash 산출 경로만 변경한다. 이들 메서드는 host rate limiter나 metric 라벨링과 무관(이미 fetch/harvest가 끝난 후 호출되므로)하므로 기존 host 처리 경로에 영향이 없다.
- **Trade-off**: `Canonical` 실행 비용이 SetStatus 호출당 한 번 더 발생. → **Mitigation**: 정규화는 in-memory string 연산으로 마이크로초 단위. DB round-trip 비용 대비 무시 가능.

## Migration Plan

본 change는 backward-compatible bug fix이므로 마이그레이션이 필요 없다.

배포 순서:
1. PR merge → CI 통과(기존 테스트 + 신규 round-trip 회귀 테스트)
2. 운영 환경 배포
3. Pioneer 워커 재시작 시 lease 만료된 무한 루프 row들이 다음 처리 사이클에서 정상 fetched 마크됨
4. 메트릭 관찰: `WARN scheduler.set_status_fetched: unknown key` 로그 라인 수가 0으로 수렴하는지 확인(목표: 24h 이내 0건)

롤백:
- 단순 코드 revert로 즉시 가능. DB 스키마/데이터 변경 없음.
- 롤백 시 무한 루프 결함은 다시 활성화되지만 데이터 손상은 없다.

## Open Questions

(없음 — 모든 결정이 본 design에서 확정됨)
