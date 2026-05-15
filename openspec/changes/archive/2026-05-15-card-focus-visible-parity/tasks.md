## 1. Pattern A — Link 단일 요소 (hover 3효과 동일 위치)

- [x] 1.1 `apps/web/src/components/feed/PinCard.tsx:146` Link className 끝에 `focus-visible:-translate-y-0.5 focus-visible:shadow-[0_8px_32px_rgba(0,0,0,0.3)] focus-visible:border-accent focus-visible:outline-none` 추가.
- [x] 1.2 `apps/web/src/app/search/SearchClient.tsx:313` 크리에이터 카드 Link className에 동일 4토큰 추가.
- [x] 1.3 `apps/web/src/app/search/SearchClient.tsx:351` 보드 카드 Link className에 동일 4토큰 추가.

## 2. Pattern B — Link / BoardCover 분리

- [x] 2.1 `apps/web/src/components/board/BoardGrid.tsx:22` Link className 끝에 `focus-visible:-translate-y-0.5 focus-visible:outline-none` 추가.
- [x] 2.2 `apps/web/src/components/profile/MyPageClient.tsx:129` Link className 끝에 동일 2토큰 추가.
- [x] 2.3 `apps/web/src/components/board/BoardCover.tsx:6` 빈 분기 div className에 `group-focus-visible:border-accent group-focus-visible:shadow-[0_8px_32px_rgba(0,0,0,0.3)]` 추가.
- [x] 2.4 `apps/web/src/components/board/BoardCover.tsx:30` 이미지 분기 div className에 동일 2토큰 추가.

## 3. 검증

- [x] 3.1 grep `focus-visible:` 결과 7곳(Pattern A 4토큰×3 + Pattern B 2토큰×2 + group-focus-visible 2토큰×2) 추가 확인.
- [x] 3.2 `apps/web/` 밖 변경 없음을 git diff로 확인. 변경 파일 = PinCard.tsx + SearchClient.tsx + BoardGrid.tsx + MyPageClient.tsx + BoardCover.tsx (5개).
- [x] 3.3 hover className 영역 손대지 않았는지 diff 재확인.

## 4. 사후 기록

- [x] 4.1 `.fugue/decision-log.md`에 "카드 5종 focus-visible: hover 미러링" 항목 1~3줄 추가.
- [x] 4.2 `.fugue/backlog-design.yaml`에서 `design-20260515-card-focus-visible-parity` 항목 status를 `done`으로 변경 + note 추가.
- [x] 4.3 change 디렉토리를 `openspec/changes/archive/2026-05-15-card-focus-visible-parity/`로 이동.
