## 1. EmptyState 공통 컴포넌트 일반화

- [x] 1.1 `apps/web/src/components/feed/EmptyState.tsx`를 props 기반으로 재작성: `{ message: string; description?: string; children?: ReactNode }`. 마스코트 `🐡` 5xl + py-16 + message(text-sm/text-text-muted) + description(text-xs/text-text-dim, optional) + children(mt-4, optional). useRouter 제거.

## 2. FeedContainer 갱신

- [x] 2.1 `apps/web/src/components/feed/FeedContainer.tsx:163`의 `<EmptyState />`를 useRouter를 본문에서 호출해 액션 버튼을 children으로 넘기는 형태로 변경: `<EmptyState message="이 분야의 작품이 아직 없어요"><button onClick={...} className="text-accent text-sm hover:underline cursor-pointer">전체 보기</button></EmptyState>`.

## 3. SearchClient 빈 상태 교체

- [x] 3.1 `apps/web/src/app/search/SearchClient.tsx:407-415`의 인라인 빈 상태 마크업을 `<EmptyState message={\`"${query}"에 대한 검색 결과가 없습니다\`} description="다른 키워드로 검색해보세요" />`로 교체. import 추가.

## 4. PinsGrid 빈 상태 교체

- [x] 4.1 `apps/web/src/components/profile/PinsGrid.tsx:121-124`의 인라인 빈 상태 마크업을 `<EmptyState message="아직 등록된 작품이 없습니다" />`로 교체. import 추가. 기존 `text-lg` 위계는 공통 컴포넌트의 sm 위계로 정렬.

## 5. MyPageClient 보드 빈 상태 교체

- [x] 5.1 `apps/web/src/components/profile/MyPageClient.tsx:123-126`의 인라인 빈 상태 마크업을 `<EmptyState message="아직 생성된 보드가 없습니다" />`로 교체. import 추가.

## 6. AddToBoardButton 빈 상태 교체

- [x] 6.1 `apps/web/src/components/board/AddToBoardButton.tsx:251-254`의 인라인 빈 상태 마크업을 `<EmptyState message="아직 생성된 보드가 없습니다" />`로 교체. import 추가.

## 7. 보드 상세 빈 상태 교체

- [x] 7.1 `apps/web/src/app/boards/[id]/page.tsx:94-119`의 grid SVG 빈 상태 블록을 `<EmptyState message="아직 보드에 추가된 작품이 없습니다" description="피드에서 마음에 드는 작품을 추가해보세요" />`로 교체. import 추가. server component에서 client EmptyState 호출 — children 없이 props만 넘기므로 안전.

## 8. 검증

- [x] 8.1 `grep -rn "🐡" apps/web/src/`는 EmptyState.tsx 1건만 반환 (인라인 사용 제거).
- [x] 8.2 `grep -rn "<EmptyState" apps/web/src/` 결과 6건 (호출처 6곳 모두 사용).
- [x] 8.3 SearchClient.tsx / PinsGrid.tsx / MyPageClient.tsx / AddToBoardButton.tsx / boards/[id]/page.tsx 어디에도 `useRouter` 또는 grid `rect` SVG 빈 상태 블록 잔존 없음(빈 상태 영역 한정).
- [x] 8.4 EmptyState.tsx에 useRouter import 없음.
- [x] 8.5 FeedContainer.test.tsx의 "이 분야의 작품이 아직 없어요" assertion 텍스트 유지.

## 9. 사후 기록

- [x] 9.1 `.fugue/decision-log.md`에 항목 1~3줄 추가.
- [x] 9.2 `.fugue/backlog-design.yaml`에서 `design-20260515-empty-state-mascot-inconsistent` status를 `done`으로 변경.
