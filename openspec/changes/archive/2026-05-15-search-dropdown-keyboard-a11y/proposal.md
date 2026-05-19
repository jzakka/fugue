## Why

`apps/web/src/components/nav/SearchBar.tsx`의 드롭다운 5종 아이템이 `<div onClick={...}>`로 마크업되어 있다:

| 위치 | 동작 | 마크업 |
|---|---|---|
| L184-231 | 최근 검색 클릭 → 검색 재실행 | `<div onClick>` |
| L254-296 | 핀 결과 클릭 → `/pins/{id}` 이동 | `<div onClick={router.push}>` |
| L306-328 | 크리에이터 결과 클릭 → `/creators/{id}` 이동 | `<div onClick={router.push}>` |
| L336-372 | 보드 결과 클릭 → `/boards/{id}` 이동 | `<div onClick={router.push}>` |
| L378-384 | 전체 결과 보기 → 검색 페이지 이동 | `<div onClick={handleSubmit}>` |

같은 파일 L212-229 삭제 X 버튼은 이미 `<button>`을 정상적으로 사용 — 즉 부분적으로만 적용된 패턴이다.

`<div onClick>`은 다음 문제를 일으킨다:

- **키보드 도달 불가**: 기본 `tabindex`가 0이 아니라 Tab으로 도달할 수 없다. 입력창에 포커스를 둔 사용자가 결과를 키보드로 활성화하지 못한다.
- **스크린리더 시맨틱 누락**: AT가 인터랙티브 요소로 인식하지 못해 "버튼/링크"로 안내하지 않는다.
- **Enter/Space 활성화 부재**: 키보드 활성화 자체가 막힌다.

루프 정체성(`prompts/loop-design.md` L7)은 "apps/web/ 안의 UI/UX, 디자인 시스템 일관성, 타이포그래피, 색/여백, 인터랙션, **접근성**, 빈 상태, 에러 표시"를 디자인 트랙 영역으로 명시한다.

## What Changes

`apps/web/src/components/nav/SearchBar.tsx`의 5종 아이템 마크업을 의미에 맞는 시맨틱 요소로 교체한다:

1. **최근 검색 (L184-231)** — 검색 재실행 동작이므로 검색어 영역을 `<button type="button">`으로. 삭제 버튼이 child로 들어가면 button-in-button 중첩이 되므로 외곽 wrapper는 `<div>` (호버 group 컨테이너)로 유지하고, 검색어 텍스트 영역만 button으로, 삭제 X 버튼은 sibling button으로 둔다.

2. **핀 / 크리에이터 / 보드 결과 (L254-296, L306-328, L336-372)** — URL 이동이므로 각각 `<Link href="/pins/{id}">`, `<Link href="/creators/{id}">`, `<Link href="/boards/{id}">`로 교체. `onClick`은 `setOpen(false)` 만 호출(드롭다운 닫기). Next.js `Link`는 prefetch도 활성화돼 부수 효과 없음.

3. **전체 결과 보기 (L378-384)** — `handleSubmit`이 `saveRecentSearch + router.push`를 함께 호출하므로 단순 `Link` href로는 불충분. `<button type="button">`로 교체하고 `w-full block`을 className에 추가해 기존 div block 레이아웃을 보존한다.

4. **import 추가** — `Link from "next/link"` 추가.

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: `apps/web/src/components/nav/SearchBar.tsx` 단일 파일.
- 사용자 영향:
  - 마우스 사용자: 동일. 클릭 동작/스타일 변경 없음.
  - 키보드 사용자: 입력창 포커스 후 Tab/Shift+Tab으로 드롭다운 결과·삭제·전체 결과 보기 도달 가능. Enter/Space로 활성화 가능. 브라우저 기본 focus 표시(`:focus-visible` outline) 살아남음.
  - 스크린리더 사용자: 결과 아이템이 "버튼" 또는 "링크"로 안내된다.
- 시각 회귀: 기존 `flex items-center gap-3 px-3 py-2 rounded-[10px] hover:bg-surface-hover` 클래스를 새 button/Link로 그대로 옮기므로 시각 변경 없음. button 기본 inline-block은 `w-full`로 block화하거나 flex 자체로 block처럼 동작하므로 레이아웃 동일.
- 인터페이스·DB·인프라 마이그레이션 없음.

## Rollback

- `git revert` 또는 5곳을 다시 `<div onClick>`으로 복원하면 즉시 이전 상태. 다른 파일 변경 없음.
