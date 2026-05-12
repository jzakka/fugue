## Why

Harvester가 미디어 다운로드/추출에 실패한 경우에도 1x1 픽셀 placeholder GIF (37바이트)를 미디어 스토리지에 업로드하고, 해당 placeholder URL을 Pin의 정본 미디어로 저장하는 결함이 2026-04-27 QA에서 노출되었다 (보고서: `.gstack/qa-reports/qa-report-localhost-3000-2026-04-27.md`). 핀 `a25429e1-305d-469c-8321-5dc8ce1a4b9f`의 `media_url`이 1x1 GIF (`b2136cc2-...gif`, 37B)로 확인되어, 사용자에게는 메인 이미지가 깨진 빈 핀이 노출된다.

기존 classifier는 `media_candidates`/`thumbnail_url`이 "빈 배열/빈 문자열"인 경우만 `no_primary_media`로 분류한다. 값이 채워져 있되 그 값이 placeholder/무효 미디어를 가리키는 경우를 식별하지 못한다. Prior learning `[fuguebot-media-crawl]` 이 정확히 이 위험("HTTP download, format validation, size limits")을 사전에 경고했으나 검증이 누락되었다.

## What Changes

- Harvester는 미디어 후보의 **유효성**을 검증해야 한다. 무효한 후보는 후보 목록에서 제외되어야 하며, 결과적으로 primary media가 부재한 PinDocument는 기존 `no_primary_media` 경로로 처리되어 Pin이 생성되지 않는다.
- 유효성은 외부 관찰 가능한 행위 기준으로 정의된다: 미디어가 선언된 타입의 디코더로 디코딩 가능하고, 의미 있는 콘텐츠 크기 임계값을 만족할 것. 구체적인 임계값과 측정 축은 design.md에 위치하며 운영 학습으로 조정 가능한 구현 파라미터다.
- 검증에서 탈락한 미디어는 미디어 스토리지에 영속되지 않아야 한다 (placeholder를 업로드하지 않는다).
- 검증 실패 사유는 디버깅/메트릭을 위해 PinDocument의 `og_data`에 관찰 가능한 형태로 기록된다.
- 본 변경 이후 **새로 생성/갱신되는 Pin**은 항상 유효한 primary media를 참조한다는 invariant를 명시화한다. 본 변경 배포 이전에 누적된 placeholder Pin들은 invariant의 예외이며, 운영 backfill (재크롤 큐 재투입)을 통해 점진 정상화된다. backfill로도 정상화되지 않는 long-tail Pin들은 메트릭으로 노출되어 후속 정리 정책의 입력이 된다.
- **BREAKING** 없음. 기존 정상 미디어를 가진 핀은 영향 받지 않는다.

## Capabilities

### New Capabilities

(없음 — 기존 harvester capability에 행위가 추가된다.)

### Modified Capabilities

- `harvester`: Content classifier의 `no_primary_media` 분류 기준에 미디어 후보의 유효성 검증을 추가한다. PinDocument 생성 시 무효 미디어 후보가 `media_candidates`/`thumbnail_url`로 채택되지 않도록 한다. 유효성 실패 사유를 `og_data`에 기록하는 행위를 추가한다.

## Impact

- **코드**: `apps/api/internal/bot/` (harvester 미디어 처리 경로), classifier 모듈, ObjectStorage 업로드 경로.
- **데이터**: 기존 placeholder media를 가진 Pin들의 식별 + 재크롤 큐 재투입 일회성 작업. Pin 데이터 모델 자체는 변경 없음.
- **API**: 변경 없음. `/api/pins/<id>` 응답 스키마 동일.
- **운영**: 무효 미디어 비율 메트릭 추가 권장 (관찰성).
- **의존성 신규 추가 없음**. 기존 미디어 디코딩 라이브러리(이미지/비디오) 활용 범위 내 처리.
