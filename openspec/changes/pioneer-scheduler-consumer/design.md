## Context

Pioneer는 지금까지 단일 프로세스 BFS 크롤러였다. 큐/visited/카운터가 전부 인메모리 상태이고, 사이트 루트에서 BFS로 하강하여 링크를 추출하고 노드/엣지를 기록한 뒤, 세션 종료 시 stale edge를 정리했다. 운영 관점에서 두 가지 근본 문제가 있다.

- **수평 확장 불가**: Pioneer 프로세스가 여러 개면 인메모리 `visited` 맵을 공유할 수 없으므로 동일 URL을 중복 fetch한다. 사이트당 quota(`MaxNodesPerSite`)도 프로세스별로 따로 세어 총량이 의미를 잃는다.
- **재시작 시 상태 소실**: 크롤 도중 프로세스가 죽으면 큐 전체가 휘발하고, 재시작 시 사이트 루트부터 다시 탐색해야 한다.

선행 change들이 이 기반을 이미 만들어뒀다.
- `scheduler-frontier-table`: 영속 `bot_frontier` 테이블과 `normalized_url` unique, Pioneer claim용 partial index(`last_fetched_at IS NULL AND fetch_error_count < 5 AND next_fetch_at <= now()`)를 정의.
- `scheduler-claim-api`: `URLScheduler` 인터페이스(`Enqueue`/`Dequeue`/`SetStatus`)를 정의. `Dequeue`는 `FOR UPDATE SKIP LOCKED`로 linearizable claim을 제공하며, 큐가 비면 busy-wait + 1초 sleep한다.

본 change는 이 frontier 위에 **Pioneer의 동작 모델**을 확정한다. Pioneer는 frontier 위에서 돌아가는 얇은 consumer가 된다. 참조 구현은 `apps/api/fuguebot_pseudo.go` Pioneer.Run(라인 33-68)이다.

## Goals / Non-Goals

**Goals:**
- Pioneer의 동작을 `scheduler.Dequeue → fetch → parse → scheduler.Enqueue(urls)` 루프로 정의.
- Pioneer가 scheduler에 fetch 성공/실패를 보고할 책임(`SetStatus`)을 명시.
- Pioneer는 producer이자 consumer임을 명시(추출한 링크를 다시 `Enqueue`).
- Pioneer 코드에 인메모리 크롤 상태(큐/visited/세션 카운터)가 존재하지 않음을 규범화.
- 링크 추출과 콘텐츠 추출의 책임 경계를 명확히: Pioneer는 링크만, Harvester는 콘텐츠.
- `bot` spec에서 인메모리 BFS를 전제하는 마지막 requirement 1건 제거.

**Non-Goals:**
- fetch 에러 backoff 정책(`scheduler-retry-backoff`에서).
- host별 속도 제한(`scheduler-host-token-bucket`에서).
- Pioneer 워커 종료 조건과 사이트/전체 예산(`pioneer-worker-budget`에서).
- 링크 필터링 정책, 도메인 경계(`pioneer-link-filter-policy`에서).
- 원본 콘텐츠 스냅샷/오브젝트 스토리지 저장(`harvester-snapshot-first-fetch`에서).
- `URLScheduler` 인터페이스 시그니처와 구현(`scheduler-claim-api`에서).
- `bot_frontier` 테이블 스키마(`scheduler-frontier-table`에서).

## Decisions

### Decision 1: Pioneer는 `URLScheduler`의 얇은 consumer로 정의한다
Pioneer는 자체 큐/BFS/visited 상태를 보유하지 않는다. 메인 루프는 다음과 같이 최소화된다.

```
for {
  url := scheduler.Dequeue("not-visited")   // busy-wait, linearizable
  content, err := fetch(url)                 // retry는 fetcher 내부 관심사
  if err != nil {
    scheduler.SetStatus(url, "error")
    continue
  }
  links := parseLinks(content)
  links = filterLinks(links)                 // pioneer-link-filter-policy에서 확장
  scheduler.Enqueue(links...)
  scheduler.SetStatus(url, "fetched")
}
```

**대안**: Pioneer에 "사이트 세션" 개념을 유지하고 scheduler는 URL 저장소로만 쓰는 설계. → 거부. 복수 워커에서 "세션" 경계가 정의되지 않으며, 이는 기존 인메모리 모델의 한계를 그대로 가져온다.

**근거**: scheduler가 이미 `FOR UPDATE SKIP LOCKED` 기반 linearizable claim을 제공하므로, Pioneer는 claim의 정확성을 재구현할 필요가 없다. "Pioneer가 가벼운 consumer"라는 모델은 수평 확장, 프로세스 재시작 복구, 테스트 용이성을 동시에 얻는다.

### Decision 2: fetch 성공/실패는 `SetStatus`로만 보고한다
Pioneer는 결과 상태(컬럼 업데이트, error count 증가, next_fetch_at 계산)를 직접 쓰지 않고 scheduler API를 통과한다. status 문자열의 구체 의미(`"fetched"`, `"error"`, `"pending"` 등)는 `scheduler-claim-api`와 `scheduler-retry-backoff`에서 정의한다. 본 change는 "보고 책임이 Pioneer에 있다"는 계약만 확정한다.

**대안**: Pioneer가 frontier row를 직접 UPDATE. → 거부. frontier 스키마 변경 시 Pioneer도 매번 변경되고, 복수 consumer(Pioneer/Harvester)가 스키마에 결합된다.

### Decision 3: Pioneer는 producer이자 consumer이다
추출한 링크를 자신이 쓰는 같은 frontier에 다시 `Enqueue`한다. 중복은 `normalized_url` unique constraint가 흡수하므로 Pioneer 쪽에 별도 dedup 로직은 두지 않는다.

**대안**: Pioneer가 링크를 파일/채널로 내보내고 별도 ingestor가 `Enqueue`. → 거부. 컴포넌트와 실패 지점이 늘어나는 반면 이익이 없다. frontier가 이미 dedup을 담당한다.

### Decision 4: Pioneer는 링크 추출만 담당한다
HTML 파싱 중 JavaScript 스크립트 실행, 미디어 다운로드, Pin 생성 등 콘텐츠 추출 작업은 Harvester의 책임이다. Pioneer의 `parseLinks`는 `<a href>` 집합만 반환한다. DOM selector 메타데이터(semantic priority 보정) 역시 frontier의 `score` 계산에 해당하는데, 우선순위 계산의 구체 규칙은 `pioneer-link-filter-policy`에서 다룬다. 본 change 범위는 "링크 추출 vs 콘텐츠 추출" 경계 규범이다.

### Decision 5: 교차 사이트 크롤을 허용한다
Pioneer는 사이트/도메인 경계를 알지 않는다. 도메인 제한이 필요하면 `pioneer-link-filter-policy`의 link filter에서 정책으로 표현한다. 본 change의 스펙은 "Pioneer가 도메인 경계를 인지해야 한다"고 명시하지 않는다.

**근거**: Fugue는 크로스미디어 큐레이션 플랫폼이며 크롤 대상 자체가 교차 사이트 그래프이다. 도메인 제한은 정책이지 Pioneer의 구조적 제약이 아니다.

### Decision 6: Pioneer는 인메모리 크롤 상태를 가져서는 안 된다
"인메모리 크롤 상태"란 URL 큐, visited/방문 집합, 사이트/세션 카운터를 말한다. 프로세스 로컬 캐시(예: HTTP keep-alive pool, DNS 캐시)는 포함하지 않는다. 이 구분은 "재시작 후 frontier 상태만으로 동작이 복구되는가"로 테스트 가능하다.

### Decision 7: 다중 워커 정확성은 scheduler가 보장한다
"정확히-한 번 claim", "동일 URL 동시 처리 방지" 같은 정합성은 scheduler의 `FOR UPDATE SKIP LOCKED` claim에서 나온다. Pioneer 코드에는 mutex/semaphore/advisory lock 같은 동시성 제어를 두지 않는다. 테스트도 Pioneer 단위가 아니라 scheduler 단위에서 한다.

## Risks / Trade-offs

- **Risk: Pioneer가 scheduler API 없이 동작 불가** → `URLScheduler`는 `scheduler-claim-api`에서 안정화되며, 테스트 환경에서는 인메모리 구현체를 같은 인터페이스로 주입한다. 본 change 적용 전에 `scheduler-claim-api`가 먼저 반영되어야 한다.
- **Risk: "인메모리 상태 금지"는 강한 제약이라 리팩터 비용이 크다** → 기존 `apps/api/internal/bot/pioneer.go`와 `priority_queue.go`/`bfs_queue.go`가 삭제·재작성된다. tasks.md에서 단계적으로 제거한다.
- **Risk: `SetStatus` 호출 누락 시 URL이 영원히 claim되지 않는다** → `scheduler-claim-api`/`scheduler-retry-backoff`에서 claim 타임아웃(예: `claimed_at` + TTL)으로 보호한다. 본 change는 "보고 책임이 Pioneer에 있다"만 명시.
- **Trade-off: 교차 사이트 크롤 허용으로 quota/속도 제어를 scheduler가 전부 떠안는다** → `scheduler-host-token-bucket`(host별 속도)과 `pioneer-worker-budget`(전체 예산)이 각각 맡는다. Pioneer는 정책을 신경 쓰지 않아 심플해진다.
- **Trade-off: stale edge 정리 로직이 당분간 없다** → `scheduler-frontier-table` Migration 노트대로, frontier의 `last_fetched_at` 갱신 기반으로 후속 change에서 재도입한다. 본 change 적용 직후에는 stale edge가 일시 누적될 수 있으나 harvest 정확성에는 영향이 없다.

## Migration Plan

1. 선행 change 확인: `scheduler-frontier-table`, `scheduler-claim-api`가 반영되어 `URLScheduler` 인터페이스와 frontier 테이블이 존재해야 한다.
2. Pioneer의 새 `Run()` 구현을 `fuguebot_pseudo.go`의 pseudo를 모델로 추가(별도 PR에서 tasks로 다룸). 기존 `pioneer.go`는 병렬로 유지하며 feature flag로 전환.
3. 새 경로가 안정화되면 `priority_queue.go`, `bfs_queue.go`, 기존 `pioneer.go`의 BFS 본문을 삭제.
4. 롤백 전략: feature flag로 구 Pioneer로 되돌릴 수 있어야 하며, frontier 테이블은 삭제하지 않는다(다음 전환에서 재활용).

## Open Questions

- Pioneer가 링크 추출 시 DOM selector 메타데이터를 frontier에 함께 적재할지, 아니면 필터 단계에서 소비하고 버릴지는 `pioneer-link-filter-policy`에서 확정한다. 본 change는 중립이다.
- `SetStatus`의 status 문자열 enum은 `scheduler-claim-api`에서 결정한다. 본 change는 `"fetched"`/`"error"`를 예시 수준으로만 언급한다.
