## Context

`scheduler-frontier-table`에서 `fetch_error_count`, `next_fetch_at`, `harvest_error_count`, `next_harvest_at` 컬럼과 `… < 5 AND next_…_at <= now()` 조건을 포함하는 partial index가 도입됐다. 하지만 실패 시 이 컬럼을 어떻게 갱신할지는 아직 정의되지 않았다.

현재 `apps/api/fuguebot_pseudo.go`의 Pioneer/Harvester는 단일 프로세스 내에서 `for retry := 0; retry < 3; retry++` 루프로 즉시 재시도만 수행하고, 실패가 영속화되지 않는다. 워커가 수평 확장되면 실패 row가 즉시 다시 claim되어 같은 원인(예: 5xx, DNS 실패)으로 무한히 재fetch될 위험이 있다.

제약:
- 컬럼/인덱스/claim 쿼리는 이미 `scheduler-frontier-table`과 `scheduler-claim-api`가 정의한 형태에 맞춰야 한다.
- 수식은 순수 산술이어야 하며, DB에서 계산해도 되고 애플리케이션에서 계산한 뒤 UPDATE로 반영해도 된다.
- Pioneer와 Harvester가 동일 공식을 공유해야 한다(운영 일관성).

## Goals / Non-Goals

**Goals:**
- fetch/harvest 실패가 영속 backoff(`next_fetch_at`, `next_harvest_at`)로 표현되어, 어떤 워커가 claim을 시도하더라도 동일한 backoff를 관측한다.
- exponential 성장으로 장애 호스트가 frontier를 점유하는 빈도를 자연스럽게 줄인다.
- ±10% jitter로 동시에 실패한 row들이 같은 시점에 다시 깨어나 thundering herd를 만들지 않는다.
- `error_count >= 5`를 dead 컷오프로 삼아 partial index에서 자동 제외되도록 한다.
- 성공 시 카운터를 0으로 리셋해 회복된 호스트에 페널티가 누적되지 않게 한다.

**Non-Goals:**
- dead row의 삭제/아카이브/재시도 관리 (`cleanup`은 별도 change에서 다룬다).
- host 단위 페이싱(host token bucket은 `scheduler-host-token-bucket`에서 별도로).
- 에러 종류별 차등 backoff(4xx vs 5xx vs DNS). 현재는 모든 에러를 동일하게 취급.
- claim 인터페이스 자체 변경 — 본 change는 실패 보고 경로(SetStatus / RecordError 류)에만 개입.
- Pioneer/Harvester가 in-process로 몇 번 재시도하는지 — backoff는 scheduler 경계에서 관리하며, in-process 재시도는 스케줄러 관점에서 단일 시도로 본다.

## Decisions

### Backoff 공식
```
delay = 30s * 2^error_count
jitter = uniform(-0.1, +0.1) * delay
next_*_at = now() + delay + jitter
```

- `error_count`는 **이번 실패를 반영하기 전**의 값을 사용한다. 즉 첫 실패(기록 전 0 → 기록 후 1) 시 delay = 30s, 두 번째 실패(1 → 2) 시 60s, … 네 번째 실패(3 → 4) 시 240s, 다섯 번째 실패(4 → 5) 시 480s로 backoff되고 이후 claim되지 않는다(dead).
- 상한 없는 `2^n`이지만 cap이 5라 max delay는 `30s * 2^4 = 480s(8분)`이며 폭주하지 않는다.
- jitter ±10%를 선택한 이유: 같은 사이트가 동시 fetch되더라도 서로 다른 시점에 재시도되도록 분산시키되, 사용자가 기대하는 backoff의 order of magnitude는 유지.
- **대안 고려**:
  - Fixed interval — 운영이 단순하지만 장애 지속 시 재시도 폭주를 막지 못함.
  - Jitterless exponential — 장애 상황에서 herding 발생.
  - Decorrelated jitter (AWS Architecture Blog) — 더 정교하지만 순수 `2^n` 기반에 비해 이해/감사가 어려움. 요구사항은 간단한 공식으로 충분.

### 성공 시 reset
- fetch 성공은 `fetch_error_count = 0`으로 리셋한다. `last_fetched_at`은 `scheduler-frontier-table` 요구사항에 따라 비-NULL이 되므로 해당 row는 어차피 Pioneer partial index에서 제외된다. 그러나 후속 재검증(예: 콘텐츠 만료 재fetch 정책이 추가되는 경우) 시 0에서 시작할 수 있도록 reset을 명시한다.
- harvest 성공은 `harvest_error_count = 0`으로 리셋한다. `pin_id`가 채워지므로 Harvester partial index에서 제외된다.

### Dead 정책
- `fetch_error_count >= 5` 또는 `harvest_error_count >= 5`인 row는 각자의 partial index 조건(`< 5`)에서 벗어나 claim되지 않는다 — 추가 애플리케이션 로직 없이 테이블 스키마만으로 dead가 성립한다.
- cleanup(삭제/보관)은 본 change 범위 밖. frontier에 잔류하지만 다시 fetch/harvest되지 않는 상태.

### 적용 지점
- URLScheduler의 에러 보고 경로에서 공식을 적용한다. 현 pseudo 코드의 `SetStatus(key, "...")` 호출이 fetch/harvest 결과를 알리는 hook이므로, scheduler 구현은 다음 의미를 가진 메서드를 제공해야 한다(이름은 구현 세부이며 명세는 "SetStatus 또는 동등한 에러 보고 경로"로 느슨하게 표현):
  - fetch 실패 보고 → `fetch_error_count += 1`, `next_fetch_at` 공식 적용.
  - fetch 성공 보고 → `last_fetched_at = now()`, `fetch_error_count = 0`.
  - harvest 실패 보고 → `harvest_error_count += 1`, `next_harvest_at` 공식 적용.
  - harvest 성공 보고 → `pin_id` 세팅, `harvest_error_count = 0`.
- 공식 계산은 애플리케이션 측에서 수행하고 단일 UPDATE로 row에 반영하는 방식을 권장(테스트/관찰 용이). DB 측 계산(`next_fetch_at = now() + ('30 seconds'::interval * pow(2, fetch_error_count)) * (1 + (random() - 0.5) * 0.2)`)도 허용.

## Risks / Trade-offs

- [장애 호스트의 5회 실패 후 영구 dead 처리] → 수동 또는 후속 cleanup change에서 재활성화 메커니즘을 제공한다. 본 change에서는 운영자가 SQL로 `fetch_error_count = 0, next_fetch_at = now()`로 리셋할 수 있음을 전제.
- [짧은 base(30s)와 낮은 cap(5)] → 일시적 네트워크 오류가 5번 연속 발생하면 dead로 빠질 수 있음. 단일 요청의 in-process 재시도(현 pseudo의 3회 루프)는 scheduler 관점에서는 한 번의 "시도"이므로 실제 순간 장애는 완충된다.
- [jitter 구현 편차] → 명세는 "±10% 균일 분포"만 요구하고, PRNG 소스는 구현 세부로 남긴다. 테스트에서는 시드 주입 또는 경계값 검증으로 확인.
- [성공 시 reset이 silent update 비용 증가] → 모든 성공 경로가 이미 row를 UPDATE하므로 같은 UPDATE에 컬럼 하나 더 포함하는 추가 비용만 발생.
