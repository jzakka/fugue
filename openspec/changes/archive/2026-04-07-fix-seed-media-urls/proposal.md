## Why

시드 데이터의 media_url이 가짜 경로(예: `image/seed-night-garden.jpg`)로 설정되어 이미지 카드에 실제 이미지가 로드되지 않고 alt text만 표시된다. 개발 환경에서 시각적 QA가 불가능하고, 새 개발자 온보딩 시 제품이 깨져 보인다.

## What Changes

- 시드 데이터의 이미지 media_url을 실제 접근 가능한 URL로 변경 (Unsplash placeholder 등)
- 오디오/비디오 핀은 media_url 유지 (재생 불가해도 카드 UI는 정상)

## Capabilities

### New Capabilities

_(없음)_

### Modified Capabilities

_(없음 -- 개발 환경 시드 데이터 개선)_

## Impact

- DB: seed.sql의 media_url 값 변경
- 프론트엔드 코드 변경 없음
