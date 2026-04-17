## Why

Pioneer가 그래프 탐색 단계에서 이미 원본 HTML을 가져와 ObjectStorage에 스냅샷으로 저장한다(`pioneer-snapshot-storage` 참조). 그럼에도 현재 Harvester는 노드 처리 시 원본 URL로 다시 HTTP 요청을 보내는 구조라, 동일 사이트에 대해 중복 네트워크 트래픽과 robots/rate-limit 부담을 발생시키고, Pioneer 시점과 Harvester 시점 간 콘텐츠 불일치(원본 변경/삭제) 위험을 키운다. Harvester가 Pioneer가 남긴 스냅샷을 우선 재사용하도록 fetch 경로를 명세화하여, 외부 트래픽을 줄이고 결정론적 파싱을 보장한다.

## What Changes

- Harvester의 fetch 단계를 `CompositeFetcher` 동작(ObjectStorage 우선 → HTTP fallback)으로 명세화한다. 참조: `apps/api/fuguebot_pseudo.go` 라인 86-97.
- 스냅샷 hit 시 외부 네트워크 호출 없이 ObjectStorage 본문만으로 파싱을 수행한다.
- 스냅샷 miss/expired/조회 실패 시 HTTP fetch로 폴백한다. 폴백 결과의 재저장 책임은 본 변경의 범위가 아니며 `pioneer-snapshot-storage` 계약에 위임한다.
- ObjectStorage와 HTTP 둘 다 실패하면 해당 노드 처리는 실패로 분류되어 `harvest_error_count`가 증가한다.
- 범위 외(별도 change에서 다룸):
  - ObjectStorage 쓰기 경로 / 스냅샷 키·TTL(365일)·gzip 압축 정책 → `pioneer-snapshot-storage`
  - Fetcher 단의 retry/backoff 정책 → `scheduler-retry-backoff`

## Capabilities

### New Capabilities
<!-- 없음. 본 변경은 기존 bot capability의 Harvester fetch 동작을 구체화한다. -->

### Modified Capabilities
- `bot`: Harvester의 fetch 동작을 `CompositeFetcher`(ObjectStorage 우선 → HTTP fallback)로 구체화하고, hit/miss/에러/이중 실패 시나리오와 실패 카운터(`harvest_error_count`) 연동을 추가한다. 기존 "Harvester가 실제 HTML을 가져온다" 요구사항을 보강하는 형태로, 단일 HTTP fetch 가정을 CompositeFetcher 기반 fetch로 대체한다.

## Impact

- 영향 코드: `apps/api/internal/bot/` 하위 Harvester 실행 경로 및 Fetcher 추상화. 참고 의사코드: `apps/api/fuguebot_pseudo.go`의 `CompositeFetcher`, `ObjectStorageFetcher`, `HTTPFetcher`, `Harvester.Run`.
- 의존: `pioneer-snapshot-storage`(스냅샷 키 규약·TTL 365일·gzip 압축·쓰기 책임). 본 변경은 읽기 측 계약만 사용하며 쓰기 경로를 가정하지 않는다.
- 운영 지표: Harvester 실행 통계의 실패 집계(`harvest_error_count`)가 ObjectStorage/HTTP 이중 실패 케이스를 포함하도록 의미가 확장된다.
- 외부 트래픽: 정상 경로에서 Harvester의 원본 사이트 호출이 사라지므로 robots/rate-limit 부담과 외부 4xx/5xx 노출이 감소한다.
- 결정론: Pioneer 시점의 HTML이 보존되어 노드별 파싱 결과의 재현성이 향상된다.
