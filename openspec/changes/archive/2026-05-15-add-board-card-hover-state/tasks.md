## 1. BoardCover에 group-hover 트리거 추가

- [x] 1.1 빈 상태 분기(L5-24)의 외곽 div className: `w-full aspect-square bg-surface-elevated rounded-[10px] flex items-center justify-center` → `w-full aspect-square bg-surface-elevated rounded-[10px] flex items-center justify-center border border-transparent group-hover:border-accent group-hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)] transition-all duration-200`.
- [x] 1.2 이미지 분기(L29-45)의 외곽 div className: `w-full aspect-square rounded-[10px] overflow-hidden grid grid-cols-2 grid-rows-2 gap-[2px]` → `w-full aspect-square rounded-[10px] overflow-hidden grid grid-cols-2 grid-rows-2 gap-[2px] border border-transparent group-hover:border-accent group-hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)] transition-all duration-200`.

## 2. BoardGrid Link className

- [x] 2.1 `apps/web/src/components/board/BoardGrid.tsx:25` className `"group"` → `"group block transition-transform duration-200 hover:-translate-y-0.5"`.

## 3. MyPageClient Link className

- [x] 3.1 `apps/web/src/components/profile/MyPageClient.tsx:133` className `"group"` → `"group block transition-transform duration-200 hover:-translate-y-0.5"`.

## 4. 검증

- [x] 4.1 BoardCover.tsx의 두 외곽 div에 `group-hover:border-accent`, `group-hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)]`, `border-transparent`, `transition-all duration-200`이 모두 존재.
- [x] 4.2 BoardGrid.tsx 및 MyPageClient.tsx에 `hover:-translate-y-0.5` 1건씩 존재.
- [x] 4.3 PinCard.tsx의 카드 hover state(L142)는 변경 없음 (grep으로 기존 클래스 유지 확인).

## 5. 사후 기록

- [x] 5.1 `.fugue/decision-log.md`에 항목 1~3줄 추가.
- [x] 5.2 `.fugue/backlog-design.yaml`에서 `design-20260515-board-card-no-hover` status를 `done`으로 변경.
