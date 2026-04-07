## Why

핀 상세 페이지에서 연관 핀이 전혀 표시되지 않는다. 연관 핀 API가 태그 기반으로만 매칭하는데, 핀에 태그가 없으면 early return으로 빈 배열을 반환한다. 또한 태그가 있어도 매칭되는 핀이 없으면 결과가 비어 있다. 프론트엔드는 API 에러를 조용히 무시하여 사용자에게 피드백 없이 연관 작품 영역이 사라진다.

## What Changes

- 연관 핀 조회에 다단계 fallback 전략 도입: 태그 매칭 -> 같은 미디어 타입 -> 최신 핀
- 태그가 0개인 핀에서도 연관 핀을 반환하도록 개선
- 프론트엔드에서 연관 핀 API 에러를 로깅하도록 개선 (silent fail 제거)

## Capabilities

### New Capabilities

없음

### Modified Capabilities

- `feed`: "연관 작품을 제공한다" 요구사항에 fallback 시나리오 추가 — 태그가 없거나 태그 매칭 결과가 부족할 때의 행위 정의

## Impact

- 연관 핀 SQL 쿼리 수정 또는 추가 (fallback 쿼리)
- 연관 핀 API 핸들러 로직 수정 (다단계 fallback)
- sqlc 코드 재생성
- 프론트엔드 핀 상세 페이지 에러 처리 개선
