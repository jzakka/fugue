## 1. DB Migration

- [x] 1.1 pg_trgm extension + GIN 인덱스 migration 생성 (`CREATE EXTENSION IF NOT EXISTS pg_trgm`, pins.title / creators.nickname / boards.name에 GIN trgm 인덱스) + down 파일
- [x] 1.2 migration 적용 및 검증

## 2. Backend — 검색 쿼리

- [x] 2.1 `apps/api/db/queries/search.sql` 작성: 핀 검색 (title similarity + pin_tags JOIN으로 태그명 매칭 부스트, ILIKE fallback)
- [x] 2.2 크리에이터 검색 쿼리 (nickname similarity + ILIKE fallback)
- [x] 2.3 보드 검색 쿼리 (name similarity + ILIKE fallback, `is_public = true` 필수)
- [x] 2.4 top_tags 집계 서브쿼리 (검색 결과 상위 100건의 pin_tags JOIN + GROUP BY + LIMIT 10)
- [x] 2.5 sqlc generate 실행

## 3. Backend — 검색 핸들러

- [x] 3.1 `apps/api/internal/search/handler.go` 생성: `GET /api/search` 핸들러 (q, type, tag_ids, limit, offset 파싱 + 빈 검색어/공백 검증)
- [x] 3.2 한글 2자 이하 분기 로직 (`len([]rune(q)) > 2` → similarity, 이하 → ILIKE)
- [x] 3.3 tag_ids 파라미터 파싱 (comma-separated UUID, 최대 5개 검증)
- [x] 3.4 type별 응답 구조 분기 (all → 3카테고리, pins/creators/boards → 단일)
- [x] 3.5 `cmd/server/main.go`에 라우터 등록

## 4. Backend — 테스트

- [x] 4.1 검색 핸들러 단위 테스트 (빈 검색어, type별 응답, 태그 필터 상한)
- [x] 4.2 보드 검색 보안 테스트 (비공개 보드 미노출 검증)

## 5. Frontend — NavBar 자동완성

UI 구현 시 `DESIGN.md`의 디자인 토큰(색상, 타이포그래피, 간격)을 따른다.

- [x] 5.1 NavBar 내 검색 input을 `SearchBar` Client Component로 분리 (NavBar는 Server Component 유지)
- [x] 5.2 자동완성 드롭다운 컴포넌트 (debounce 300ms, 3자 이상에서만 검색 API 호출)
- [x] 5.3 드롭다운 렌더링 (섹션 헤더: 핀/크리에이터/보드)
- [x] 5.4 최근 검색어 표시 (localStorage, 최대 5개, 삭제 버튼)
- [x] 5.5 Enter 키 → `/search?q=` 페이지 이동

## 6. Frontend — 검색 결과 페이지

UI 구현 시 `DESIGN.md`의 디자인 토큰을 따른다.

- [x] 6.1 `apps/web/src/app/search/page.tsx` 생성 (searchParams로 q, type, tag_ids 수신)
- [x] 6.2 카테고리 탭 UI (전체/핀/크리에이터/보드)
- [x] 6.3 태그 필터 칩 UI (top_tags 기반, 복수 선택, 클릭시 AND 필터 추가)
- [x] 6.4 검색 결과 목록 (핀: PinCard 재활용, 크리에이터/보드: 간단 카드)
- [x] 6.5 페이지네이션 (더보기 또는 infinite scroll)
- [x] 6.6 `apps/web/src/lib/api.ts`에 fetchSearch 함수 추가
