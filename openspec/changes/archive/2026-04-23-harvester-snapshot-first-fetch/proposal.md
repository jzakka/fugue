## Why

Pioneer가 그래프 탐색 단계에서 이미 원본 HTML을 가져와 ObjectStorage에 스냅샷으로 저장한다(과거 change `pioneer-snapshot-storage`에서 쓰기 경로 확정, 현재는 `bot` capability의 스냅샷 쓰기 Requirement로 존재). 그럼에도 현재 Harvester는 노드 처리 시 원본 URL로 다시 HTTP 요청을 보내는 구조라, 동일 사이트에 대해 중복 네트워크 트래픽과 robots/rate-limit 부담을 발생시키고, Pioneer 시점과 Harvester 시점 간 콘텐츠 불일치(원본 변경/삭제) 위험을 키운다. Harvester가 Pioneer가 남긴 스냅샷을 우선 재사용하도록 fetch 경로를 명세화하여, 외부 트래픽을 줄이고 결정론적 파싱을 보장한다.

## What Changes

- Harvester의 fetch 단계를 "ObjectStorage 우선 → HTTP fallback"의 합성 동작으로 명세화한다. 참조 의사코드: `apps/api/fuguebot_pseudo.go` 라인 97–112.
- 스냅샷 hit 시 외부 네트워크 호출 없이 ObjectStorage 본문만으로 파싱을 수행한다.
- 해시 함수·스냅샷 키 포맷은 `bot` capability의 스냅샷 쓰기 경로가 이미 확정한 공용 심볼(`apps/api/internal/bot/snapshot` 패키지의 `SnapshotKey`, `HashNormalizedURL`, `SnapshotKeyPattern`)을 그대로 import해 재사용한다. Harvester가 재구현하지 않는다.
- 스냅샷 키의 시간 세그먼트는 **Harvester 실행 시각 기준의 현재 UTC 날짜**로 결정한다. 동일 UTC 일자 내에 Pioneer가 쓴 스냅샷이 hit되며, 그 외는 "사용 불가"로 간주되어 HTTP 폴백 경로로 수렴한다(설계상 자기 복구 가능). 자세한 결정 근거와 대안은 design.md Decision 5a 참조.
- ObjectStorage 실패 유형(키 없음 / 네트워크 / 권한 / 내부 에러, 4종)은 모두 **단일 "사용 불가"** 로 취급해 HTTP fallback으로 동일하게 라우팅한다. TTL 만료는 lifecycle 삭제에 의해 "키 없음"으로 수렴하므로 독립 관측 범주가 아니다. 실패 종류 구분은 로그/메트릭(운영 분석용)에서만 수행하며 fetch 동작에는 영향을 주지 않는다.
- 저장 포맷 변환(ObjectStorage 경로의 압축 해제 등)은 fetch 경계 안에서 완결되며, 호출자(Harvester 파이프라인)에는 원본 HTML 바이트열만 노출된다.
- 스냅샷 사용 불가 시 HTTP fetch로 폴백한다. 폴백 결과의 재저장 책임은 본 변경의 범위가 아니며 `bot` capability의 스냅샷 쓰기 경로에 위임한다.
- ObjectStorage와 HTTP 둘 다 실패하면 해당 노드 처리는 실패로 분류되어 Harvester 워커 실행 통계(in-memory)의 fetch 실패 카운터가 증가하며, 단일 노드 실패는 다른 노드 처리를 중단시키지 않는다. `harvester_frontier.harvest_error_count` DB 컬럼과는 별개 집계다.
- 범위 외(별도 change에서 다룸):
  - ObjectStorage 쓰기 경로 / 스냅샷 키·TTL(365일)·압축 저장 정책 → `bot` capability 스냅샷 쓰기 Requirement(구 `pioneer-snapshot-storage` change)
  - Fetcher 단의 retry/backoff 정책 → `scheduler-retry-backoff`
  - Consumer 경계 입력 형식은 sibling change `harvester-snapshot-first-fetchconsumer`가 `(ctx, url)` 단일 형태로 확정하며(snapshot_key 비전달), 본 change는 그 경계에 의존한다(유지가 아닌 신규 계약)

## Capabilities

### Modified Capabilities
- `bot`: Harvester의 fetch 동작을 "ObjectStorage 우선 → HTTP fallback" 합성 의미론으로 구체화하고, hit/miss/에러/이중 실패 시나리오와 실행 통계 실패 집계 연동을 추가한다. 기존 "Harvester가 실제 HTML을 가져온다" 요구사항을 보강하는 형태로, 단일 HTTP fetch 가정을 합성 fetch 기반으로 대체한다. 기존 Scenario 2건("Harvester HTML 가져오기", "Pioneer와 Harvester의 fetch 로직 공유")은 본 MODIFIED 블록의 Scenario 집합으로 대체·완화된다(자세한 대응 관계는 `specs/bot/spec.md` 참조).

## Impact

- 영향 코드: `apps/api/internal/bot/` 하위 Harvester 실행 경로 및 Fetcher 추상화. 참고 의사코드: `apps/api/fuguebot_pseudo.go` 라인 97–112의 `CompositeFetcher`, `ObjectStorageFetcher`, `HTTPFetcher`, `Harvester.Run`.
- 의존: `bot` capability의 스냅샷 쓰기 경로(스냅샷 키 규약·TTL 365일·압축 저장·해시 함수·쓰기 책임). 본 변경은 읽기 측 계약만 사용하며 쓰기 경로를 가정하지 않는다. 키 빌더/해시 함수는 `apps/api/internal/bot/snapshot` 패키지의 공용 구현을 import해 재사용한다(재구현 금지).
- 운영 지표: Harvester 워커 실행 통계(in-memory)의 fetch 실패 카운터가 ObjectStorage/HTTP 이중 실패 케이스를 포함하도록 의미가 확장된다. fetch 출처(스냅샷/HTTP) 및 ObjectStorage 실패 종류는 로그/메트릭으로만 구분된다(행위 아님).
- 외부 트래픽: 정상 경로(스냅샷 hit)에서 Harvester의 원본 사이트 호출이 사라지므로 robots/rate-limit 부담과 외부 4xx/5xx 노출이 감소한다. 단, Harvester가 Pioneer 쓰기와 다른 UTC 일자에 실행되는 경우 전체가 HTTP 폴백으로 수렴한다(디자인 Decision 5a 트레이드오프 참조).
- 결정론: Pioneer 시점의 HTML이 보존되어 같은 UTC 일자 내 노드별 파싱 결과의 재현성이 향상된다.
