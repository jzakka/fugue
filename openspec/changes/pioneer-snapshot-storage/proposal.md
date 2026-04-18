## Why

Pioneer가 URL을 fetch한 raw HTML은 현재 링크 추출에만 사용되고 버려진다. 곧이어 Harvester가 동일 URL을 다시 네트워크로 가져오기 때문에 같은 바이트를 두 번 받는 낭비가 발생하고, 원본 사이트에 이중 부하를 준다. 또한 Pioneer 시점과 Harvester 시점 사이에 페이지가 변경되면 두 단계가 서로 다른 스냅샷을 보게 되어 Pioneer가 판단한 노드 타입/링크와 Harvester가 실제로 파싱한 콘텐츠가 어긋날 수 있다.

raw 응답을 object storage에 스냅샷으로 남겨 두면 후속 단계(우선 Harvester, 이후 재시도/디버깅)가 동일한 바이트를 재사용할 수 있고, fetch-once 원칙을 지킬 수 있다.

## What Changes

- Pioneer는 `URLScheduler`로부터 받은 URL을 fetch하여 성공(2xx + 본문 수신)한 경우 raw 응답 바이트를 object storage에 스냅샷으로 업로드한다.
- 스냅샷은 gzip으로 압축해 저장하고, TTL 365일을 적용한다.
- 스냅샷 키는 normalized URL의 **sha256** hex digest를 사용한다: `snapshots/<sha256_hex>/<yyyymmdd>.html.gz` (hex 64자 + UTC 날짜).
- fetch가 실패하면(네트워크 오류, 4xx/5xx 등) 스냅샷을 저장하지 않는다.
- object storage write가 실패해도 Pioneer 파이프라인은 fail-open으로 계속 진행하며, 오류는 로그로만 남긴다.
- 동시 쓰기 정책: 같은 키에 대한 동시 PUT은 object storage의 기본 동작을 따르는 **last-write-wins**다. 애플리케이션 레벨의 lock/version 관리는 두지 않는다.

범위 외: Harvester가 스냅샷을 재사용하는 동작(별도 change `harvester-snapshot-first-fetch`에서 다룸), Pioneer의 BFS/스케줄 루프 구조 자체는 변경하지 않는다.

## Capabilities

### New Capabilities

_(없음)_

### Modified Capabilities

- `bot`: Pioneer의 fetch 성공 경로에 스냅샷 저장 단계를 추가한다. 스냅샷 키 규칙, TTL, 압축, fail-open 동작이 spec 수준에서 정의된다.

## Impact

- **코드 변경**: Pioneer의 fetch 성공 후크 지점에 스냅샷 저장 단계를 추가한다(현 스케치: `apps/api/fuguebot_pseudo.go`의 `SaveRawContent` 상응 지점, 실제 위치는 구현 시 Pioneer 진입점에 결정).
- **새 의존성**: object storage 클라이언트(S3 호환) 접근 경로. 기존 미디어 업로드 경로와 bucket/자격 증명을 공유 가능성이 높음.
- **설정**: 스냅샷 전용 bucket/prefix, TTL(365일) lifecycle rule, 스냅샷 on/off feature flag.
- **관측성**: 스냅샷 저장 성공/실패 카운터, 업로드 지연 로그.
- **하위 호환성**: 외부 API 변경 없음. Harvester는 아직 스냅샷을 조회하지 않으므로 기존 동작 그대로 유지.
- **후속 change 의존성**: `harvester-snapshot-first-fetch`가 본 change의 키 규칙·TTL·저장 형식에 의존한다.
