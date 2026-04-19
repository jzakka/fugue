## Why

현재 `bot` capability는 primary 이미지 캐시 객체의 TTL/만료를 **명시적으로 capability 외부**라고 선언한다. 그 결과 캐시된 이미지 객체는 object storage에 무기한 누적되며, fallback이 발생해 부분 객체가 남거나 Pin이 삭제되어 고아가 된 객체도 정리되지 않는다. 스토리지 비용과 운영 부담이 시간에 비례해 증가하므로, 제품 내 계약으로 만료 정책을 정의해 비용 상한을 보장해야 한다.

## What Changes

- **BREAKING (capability 계약 — 기존 Requirement REMOVED + 신규 Requirement ADDED)**: `bot` capability의 "이미지 캐시 객체의 TTL/만료는 본 capability 외부다" Requirement를 **REMOVED**하고, 연령 기반 TTL을 본 capability가 정의한다는 신규 Requirement를 **ADDED**한다. 외부 소비자 관점에서 "만료는 capability 외부"라는 부정 계약이 사라지므로 breaking이며, delta 스펙은 `MODIFIED Requirements`가 아니라 `REMOVED Requirements` + `ADDED Requirements` 두 섹션으로 표현된다.
- 외부 관찰 가능한 만료 행위를 Requirement로 추가한다:
  - 캐시 객체는 자신의 작성/최종 쓰기 시점 이후 TTL이 경과하면 제거 대상이 된다(SHALL).
  - 만료로 인한 객체 제거는 Pin 생성의 성공/실패에 영향을 주지 않는다(SHALL NOT block Pin creation).
  - TTL 값은 설정 가능해야 한다(SHALL).
  - 만료된 객체 참조를 Pin이 보유하고 있을 때의 조회 결과(예: 404)는 본 capability의 성공 경로로 간주되지 않으며, 소비자는 참조가 해소되지 않을 수 있음을 허용해야 한다.
- 동일 후보 URL을 다른 시점에 재캐시하면 별도 객체로 저장된다는 기존 Requirement는 유지되며, 만료는 per-object 기준으로 평가된다.
- TTL의 구체 값, 만료 스윕 메커니즘(bucket lifecycle rule vs. 자체 GC 잡), 고아 객체 판별 알고리즘 등 **내부 구현 세부**는 design 문서에서 확정한다.

## Capabilities

### New Capabilities

(없음 — 기존 capability의 기존 Requirement를 교체한다)

### Modified Capabilities

- `bot`: "이미지 캐시 객체의 TTL/만료는 본 capability 외부다" Requirement를 **REMOVED**하고, "캐시된 primary 이미지 객체는 설정 가능한 TTL 후 만료 대상이 된다"는 Requirement를 **ADDED**한다(외부 계약 기준 breaking). 본 변경 범위는 **primary 이미지 캐시 네임스페이스에 한정**되며 본문 미디어(item의 media 본체) 저장 네임스페이스의 lifecycle은 본 change에서 정의하지 않는다.

## Impact

- **Object storage**: primary 이미지 캐시 네임스페이스에 연령 기반 만료 정책이 적용된다. 구체 메커니즘은 design 문서에서 결정.
- **Harvester 코드**: 캐시 업로드 경로 자체의 계약은 변하지 않는다(키 형식, fallback 경로 유지). 만료는 업로드 이후의 별도 메커니즘이 담당하므로 hot path 동작에는 영향 없음.
- **Pin 소비자(웹/API)**: Pin의 대표 이미지 참조가 이미 만료된 object 참조일 수 있으므로, 이미지 참조가 해소되지 않는 경우를 UX에서 허용해야 한다(외부 URL일 때도 이미 존재하는 실패 모드이므로 신규 코드는 불필요할 가능성이 높음).
- **DB 스키마**: 변경 없음.
- **운영**: 새로 도입되는 TTL 설정 값은 환경 변수 `HARVESTER_IMAGE_CACHE_TTL_DAYS` (기본 90일)로 노출된다 — 구체 선택 근거와 Terraform 연계는 design 문서 참조.
