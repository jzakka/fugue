## 1. DB Migration

- [ ] 1.1 pg_trgm extension + GIN 인덱스 migration 생성 (`CREATE EXTENSION IF NOT EXISTS pg_trgm`, pins.title / creators.nickname / boards.name에 GIN trgm 인덱스)
- [ ] 1.2 migration 적용 및 검증

## 2. Backend — 검색 쿼리

- [ ] 2.1 `apps/api/db/queries/search.sql` 작성: 핀 검색 (title + tags similarity, 태그 정확매칭 부스트, ILIKE fallback)
- [ ] 2.2 크리에이터 검색 쿼리 (nickname similarity + ILIKE fallback)
- [ ] 2.3 보드 검색 쿼리 (name similarity + ILIKE fallback, `is_public = true` 필수)
- [ ] 2.4 top_tags 집계 서브쿼리 (상위 100건 unnest + GROUP BY + LIMIT 10)
- [ ] 2.5 sqlc generate 실행

## 3. Backend — 검색 핸들러

- [ ] 3.1 `apps/api/internal/search/handler.go` 생성: `GET /api/search` 핸들러 (q, type, tags, limit, offset 파싱)
- [ ] 3.2 한글 2자 이하 분기 로직 (`len([]rune(q)) > 2` → similarity, 이하 → ILIKE)
- [ ] 3.3 tags 파라미터 파싱 (comma-separated, 최대 5개 검증)
- [ ] 3.4 type별 응답 구조 분기 (all → 3카테고리, pins/creators/boards → 단일)
- [ ] 3.5 `cmd/server/main.go`에 라우터 등록

## 4. Backend — 테스트

- [ ] 4.1 검색 핸들러 단위 테스트 (빈 검색어, type별 응답, 태그 필터 상한)
- [ ] 4.2 보드 검색 보안 테스트 (비공개 보드 미노출 검증)

## 5. Frontend — NavBar 자동완성

- [ ] 5.1 NavBar 검색바 활성화 (disabled 제거, 상태 관리)
- [ ] 5.2 자동완성 드롭다운 컴포넌트 (debounce 300ms, `type=all&limit=5` 호출)
- [ ] 5.3 드롭다운 렌더링 (섹션 헤더: 핀 3개, 크리에이터 1개, 보드 1개)
- [ ] 5.4 최근 검색어 표시 (localStorage `fugue_recent_searches`, 최대 5개, 삭제 버튼)
- [ ] 5.5 Enter 키 → `/search?q=` 페이지 이동

## 6. Frontend — 검색 결과 페이지

- [ ] 6.1 `apps/web/src/app/search/page.tsx` 생성 (searchParams로 q, type, tags 수신)
- [ ] 6.2 카테고리 탭 UI (전체/핀/크리에이터/보드)
- [ ] 6.3 태그 필터 칩 UI (top_tags 기반, 복수 선택, 클릭시 AND 필터 추가)
- [ ] 6.4 검색 결과 목록 (핀: PinCard 재활용, 크리에이터/보드: 간단 카드)
- [ ] 6.5 페이지네이션 (더보기 또는 infinite scroll)
- [ ] 6.6 `apps/web/src/lib/api.ts`에 fetchSearch 함수 추가
