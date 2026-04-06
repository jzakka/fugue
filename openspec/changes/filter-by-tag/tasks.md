## 1. 백엔드: 인기 태그 API

- [ ] 1.1 `db/queries/pins.sql`에 `GetPopularTags` 쿼리 추가 (unnest + GROUP BY + ORDER BY count DESC)
- [ ] 1.2 `sqlc generate` 실행하여 Go 코드 생성
- [ ] 1.3 `pin/handler.go`의 `PinQuerier` 인터페이스에 `GetPopularTags` 메서드 추가
- [ ] 1.4 `pin/handler.go`에 `PopularTags` 핸들러 메서드 구현 (`GET /api/tags/popular?limit=`)
- [ ] 1.5 `cmd/server/main.go`에 `r.Get("/api/tags/popular", pinHandler.PopularTags)` 라우트 등록
- [ ] 1.6 `go build ./...` 로 빌드 확인

## 2. 프론트엔드: API 클라이언트

- [ ] 2.1 `lib/api.ts`에 `PopularTag` 인터페이스 추가 (`{ tag: string; count: number }`)
- [ ] 2.2 `lib/api.ts`에 `fetchPopularTags` 함수 추가 (serverSide 옵션 지원)

## 3. 프론트엔드: TagFilter 컴포넌트

- [ ] 3.1 `components/feed/TagFilter.tsx` 생성 — 인기 태그 칩 목록, 다중 선택 토글, 초기화 버튼
- [ ] 3.2 URL 쿼리 파라미터 `?tags=tag1,tag2`로 상태 관리 (useSearchParams + router.push)
- [ ] 3.3 DESIGN.md 스타일 준수: Geist Mono 폰트, accent 색상, rounded-full 칩

## 4. 프론트엔드: 메인 페이지 연동

- [ ] 4.1 `app/page.tsx`에서 `fetchPopularTags`를 SSR로 호출 (getInitialPins와 Promise.all 병렬)
- [ ] 4.2 `app/page.tsx`에서 `searchParams.tags`를 파싱하여 `getInitialPins`에 전달
- [ ] 4.3 `app/page.tsx`에 `<TagFilter>` 컴포넌트 배치 (FieldFilter 아래)
- [ ] 4.4 `FeedContainer`에 `initialTags` prop 추가, 태그 변경 시 리로드 로직 구현 (field 패턴 확장)
- [ ] 4.5 `FeedContainer`의 `loadMore`에서 현재 선택된 태그를 `fetchPins`에 전달

## 5. 검증

- [ ] 5.1 로컬 환경에서 인기 태그 API 응답 확인 (`curl /api/tags/popular`)
- [ ] 5.2 브라우저에서 태그 칩 선택/해제 시 피드 필터링 동작 확인
- [ ] 5.3 분야 + 태그 조합 필터 동작 확인
- [ ] 5.4 태그 선택 상태에서 URL 공유 시 동일 필터 적용 확인
- [ ] 5.5 무한 스크롤 중 태그 변경 시 첫 페이지부터 리로드 확인
