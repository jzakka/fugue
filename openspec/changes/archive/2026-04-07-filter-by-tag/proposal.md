## Why

메인 피드에서 사용자가 관심 태그를 선택해 원하는 작품만 볼 수 있는 방법이 없다. 현재 미디어 타입 필터만 존재하며, 태그 기반 탐색은 백엔드(`GET /api/pins?tag_ids=`)에서만 지원되고 UI가 없다. 사전 정의된 태그 시스템이 도입되었으므로, 인기 태그 기반 피드 필터링이 가능하다.

## What Changes

- 메인 피드 페이지에 **태그 필터 UI** 추가 — 인기 태그 칩을 노출하고, 사용자가 하나 이상 선택하면 해당 태그가 연결된 핀만 표시
- **인기 태그 조회 API** 추가 (`GET /api/tags/popular`) — `pin_tags` 테이블에서 가장 많이 사용된 태그를 집계하여 반환
- FeedContainer가 선택된 태그 ID를 `fetchPins`에 전달하도록 연동
- URL 쿼리 파라미터(`?tags=slug1,slug2`)로 태그 필터 상태 유지 (공유 가능)

## Capabilities

### New Capabilities
_(없음 — 기존 도메인에 요구사항 추가)_

### Modified Capabilities
- `pin`: 인기 태그 집계 조회 API 요구사항 추가
- `feed`: 피드 페이지에서 태그 필터 UI 제공 + 태그 기반 필터링 요구사항 추가

## Impact

- **백엔드**: `tags.sql`에 인기 태그 집계 쿼리 추가, sqlc 재생성, `tag` 핸들러에 엔드포인트 추가, `main.go`에 라우트 등록
- **프론트엔드**: `TagFilter` 컴포넌트 신규 생성, `page.tsx`/`FeedContainer.tsx` 수정, `api.ts`에 `fetchPopularTags` 추가
- **DB**: 추가 마이그레이션 없음 (`pin_tags` 테이블 + `idx_pin_tags_tag` 인덱스 활용)
- **기존 API**: 변경 없음 (`GET /api/pins?tag_ids=` 이미 지원)
