# fix-harvester-og-image-cache-cleanup

## Why

Harvester가 Pin의 대표 이미지(og_image)를 자사 저장소에 캐시할 때, 캐시 객체 업로드가 Pin upsert보다 먼저 일어나고 이후 어떤 정리 경로도 없다. 그 결과 두 가지 방식으로 저장소에 불필요한 객체가 누적된다:

1. **upsert 실패 시 고아 객체** — 새 캐시 객체를 업로드한 뒤 Pin upsert가 실패하면, 어떤 Pin도 참조하지 않는 새 객체가 저장소에 남는다.
2. **재수집 시 이전 객체 누적** — 같은 후보 URL을 재캐시하면 항상 새 키로 저장된다(기존 스펙의 no-overwrite 계약). Pin row의 og_image는 새 값으로 덮어써지지만, 이전에 참조하던 캐시 객체는 삭제되지 않아 재수집마다 미참조 객체가 하나씩 쌓인다.

현재 유일한 방어선은 90일 age-based TTL 만료뿐이며, 재수집 빈도가 높은 사이트에서는 TTL 도래 전까지 미참조 객체가 선형으로 누적된다. 기존 스펙(`이미지 캐시 실패는 단일 fallback 경로로 처리된다`)이 "후속 GC change"에 위임한 것은 업로드 도중 실패로 남는 **부분 객체**의 정리로, 본 change는 그와 인접하지만 다른 문제 — **완결 저장된 후 미참조가 된 객체** — 를 다룬다. 부분 객체 정리는 여전히 storage lifecycle에 위임된 채로 남는다. (Linear: NAV-1253)

## What Changes

- Pin upsert가 실패하면, 해당 처리에서 새로 업로드한 캐시 객체를 보상 삭제(compensating delete)한다.
- Pin upsert가 성공했고 그 결과 대표 이미지 참조가 **다른 값으로 교체**되면(새 캐시 객체로 교체 또는 원본 URL fallback으로 교체), 이전에 참조하던 자사 저장소 캐시 객체를 삭제한다.
- 정리(삭제)는 best-effort다: 삭제 실패는 로그로 관찰 가능해야 하며 Pin 처리의 성공/실패에 영향을 주지 않는다. TTL 만료가 정리 실패의 최종 방어선으로 유지된다.
- 삭제 대상은 이미지 캐시 네임스페이스의 객체로 한정한다. 자사 저장소를 가리키지 않는 이전 참조(원본 URL fallback)와 캐시 네임스페이스 밖의 자사 객체(사용자 업로드 미디어 등)는 어떤 경우에도 삭제하지 않는다.
- 캐시 업로드가 실제로 이미지 캐시 네임스페이스 키로 저장되도록 저장 경로를 바로잡는다 — 현재 구현은 캐시용으로 구성한 키가 저장 계층에서 무시되어 캐시 객체가 사용자 미디어 네임스페이스에 섞여 저장되고 있으며(기존 네임스페이스 분리 요구의 미집행), 이 상태로는 네임스페이스 한정 삭제와 TTL lifecycle이 캐시 객체에 적용되지 않는다.
- 기존 캐시 키 no-overwrite 계약과 실패 fallback 계약은 변경하지 않는다. TTL 계약은 값·만료 메커니즘은 그대로 두되, "TTL 미경과 객체 제거 금지"가 연령 기반 만료 메커니즘에 한정됨을 명확히 하여 미참조 정리 삭제와의 문언 충돌을 해소한다.

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `bot`: 이미지 캐시 정리 요구가 추가된다 — (a) Pin upsert 실패 시 그 시도에서 새로 저장된 캐시 객체는 잔존하지 않도록 정리되고, (b) 재수집으로 대표 이미지 참조가 교체되면 이전 자사 캐시 객체가 정리된다. 정리는 Pin 처리 결과를 막지 않는 비차단(best-effort) 동작이다. 기존 요구 `이미지 캐시 실패는 단일 fallback 경로로 처리된다`가 외부에 위임한 정리 책임은 업로드 도중 실패로 남는 **부분 객체**에 한정된 것이므로, 완결 업로드 후 미참조가 된 객체의 정리는 신규 요구로 추가한다. 또한 TTL 요구를 MODIFIED로 개정해 "TTL 미경과 객체 제거 금지"가 연령 기반 만료 메커니즘에 한정됨을 명확히 한다(미참조 정리 삭제와의 문언 충돌 해소). 캐시 키 no-overwrite와 실패 fallback 요구는 변경하지 않는다.

## Impact

- **코드**: `apps/api/internal/bot/harvest_pipeline.go` (ProcessDocument에 정리 경로 추가), `apps/api/internal/bot/storage.go` (bot Storage 추상화에 삭제 능력 추가, StorageAdapter/MockStorage 갱신), `apps/api/internal/storage/storage.go` (URL→key 역변환 헬퍼와 호출자 키를 존중하는 업로드 경로 추가), `apps/api/db/queries/pins.sql` (upsert가 교체 이전의 대표 이미지 참조를 함께 반환하도록 확장) + sqlc 재생성.
- **의존성**: object storage 삭제 API는 스택 하위 change(fix-pin-create-orphan-media, PR #4118)가 추가한 `storage.Client.Delete`를 사용한다. 본 change는 그 브랜치 위에 스택된다.
- **운영**: 재수집이 잦은 사이트의 이미지 캐시 저장 용량 증가가 멈춘다. 삭제 실패 시에도 기존 TTL lifecycle이 그대로 동작하므로 운영 위험은 없다.
- **비영향**: Pin 생성/업데이트의 성공 조건, API 응답, 캐시 키 구성, TTL 값은 변경 없음.
