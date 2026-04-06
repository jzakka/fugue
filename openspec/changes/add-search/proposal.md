## Why

메인 화면에서 콘텐츠를 발견할 수 있는 유일한 방법이 피드 스크롤과 분야 필터뿐이다. 핀이 쌓일수록 원하는 콘텐츠를 다시 찾기 어렵고, 새로운 크리에이터나 보드를 탐색할 수단이 없다. 큐레이션 플랫폼의 핵심인 "검색을 통한 발견"이 빠져 있다.

## What Changes

- 통합 검색 API 추가 (`GET /api/search`): 핀 제목+태그, 크리에이터 닉네임, 보드 이름을 한 번에 검색
- PostgreSQL pg_trgm 확장 + GIN 인덱스로 유사도 기반 검색 및 랭킹
- 한글 2자 이하 검색어는 ILIKE fallback 처리
- NavBar의 disabled 검색바를 활성화하여 자동완성 드롭다운 제공
- 검색 결과 페이지 신규: 카테고리 탭(전체/핀/크리에이터/보드) + 태그 필터 칩
- 최근 검색어 히스토리 (localStorage, 프론트엔드 전용)
- 검색 결과 URL 공유 가능 (Deep Link via searchParams)
- 보드 검색은 공개 보드(`is_public = true`)만 대상 (보안)

## Capabilities

### New Capabilities
- `search`: 통합 검색 API, pg_trgm 인덱스, 자동완성, 검색 결과 페이지, 필터 칩, 최근 검색어

### Modified Capabilities
- `feed`: 검색 결과가 피드와 별도 경로(/search)로 존재하므로 피드 자체 요구사항 변경 없음. 수정 없음.

## Impact

- **Backend**: `apps/api/internal/search/` 핸들러 신규, `apps/api/db/queries/search.sql` 신규, `apps/api/db/migrations/` pg_trgm + GIN 인덱스 migration 신규
- **Frontend**: `apps/web/src/app/search/page.tsx` 신규, `NavBar.tsx` 검색바 활성화, `apps/web/src/lib/api.ts` fetchSearch 추가
- **DB**: `CREATE EXTENSION IF NOT EXISTS pg_trgm`, pins.title / creators.nickname / boards.name에 GIN 인덱스
- **API**: `GET /api/search?q=&type=all|pins|creators|boards&tags=&limit=&offset=` 신규 엔드포인트
