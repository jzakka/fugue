## 1. 백엔드: 인기 태그 API

- [x] 1.1 `db/queries/tags.sql`에 `GetPopularTags` 쿼리 추가 (pin_tags JOIN tags + GROUP BY + ORDER BY count DESC + LIMIT)
- [x] 1.2 `sqlc generate` 실행하여 Go 코드 생성
- [x] 1.3 `tag/handler.go`에 `PopularTags` 핸들러 메서드 구현 (`GET /api/tags/popular?limit=`)
- [x] 1.4 `cmd/server/main.go`에 `r.Get("/api/tags/popular", tagHandler.PopularTags)` 라우트 등록 (기존 `/api/tags` 바로 위에 배치)
- [x] 1.5 `go build ./...` 로 빌드 확인

## 2. 프론트엔드: API 클라이언트

- [x] 2.1 `lib/api.ts`에 `PopularTag` 응답 인터페이스 추가
- [x] 2.2 `lib/api.ts`에 `fetchPopularTags` 함수 추가 (serverSide 옵션 지원)

## 3. 프론트엔드: TagFilter 컴포넌트

- [x] 3.1 `components/feed/TagFilter.tsx` 생성 — 인기 태그 칩 목록, 다중 선택 토글, 초기화 버튼
- [x] 3.2 URL 쿼리 파라미터 `?tags=slug1,slug2`로 상태 관리 (useSearchParams + router.push)
- [x] 3.3 DESIGN.md 스타일 준수: Geist Mono 폰트, accent 색상, rounded-full 칩

## 4. 프론트엔드: 메인 페이지 연동

- [x] 4.1 `app/page.tsx`에서 `fetchPopularTags`를 SSR로 호출 (getInitialPins와 Promise.all 병렬)
- [x] 4.2 `app/page.tsx`에서 searchParams 타입에 `tags?: string` 추가, `searchParams.tags`를 파싱하여 slug→id 변환 후 `getInitialPins`에 전달 (인기 태그에 없는 slug는 무시)
- [x] 4.3 `app/page.tsx`에 `<TagFilter>` 컴포넌트 배치 (FieldFilter 아래)
- [x] 4.4 `FeedContainer`에 태그 변경 감지 + 리로드 로직 추가, 인기 태그 데이터 전달
- [x] 4.5 `FeedContainer`의 무한 스크롤에서 현재 태그 필터 유지, 페이지 폴백에도 태그 반영
- [x] 4.6 `FieldFilter` 수정: 미디어 타입 변경 시 기존 태그 필터 상태를 보존

## 5. 검증

- [x] 5.1 로컬 환경에서 인기 태그 API 응답 확인 (`curl /api/tags/popular`)
- [x] 5.2 브라우저에서 태그 칩 선택/해제 시 피드 필터링 동작 확인
- [x] 5.3 미디어 타입 + 태그 조합 필터 동작 확인
- [x] 5.4 태그 선택 상태에서 URL 공유 시 동일 필터 적용 확인
- [x] 5.5 무한 스크롤 중 태그 변경 시 첫 페이지부터 리로드 확인
