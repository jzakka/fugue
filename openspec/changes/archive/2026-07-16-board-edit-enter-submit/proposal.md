# Proposal: board-edit-enter-submit

## Why

제출형 텍스트 입력 표면 전수 6곳 중 5곳은 Enter 키로 제출이 가능한데, 보드 상세의 보드 편집 폼(BoardActions)만 Enter가 무반응이다.

- native `<form onSubmit>` + `type="submit"` 2곳: PinCreateForm:319/:660 · ProfileEditForm:49/:124 — Enter 암시적 제출
- `onKeyDown` Enter 핸들러 3곳: MyPageClient:124(보드 생성) · AddToBoardButton:372(보드 생성) · SearchBar:166(검색)
- **BoardActions:74(이름)/:85(설명)만 비-form div 저작 + onKeyDown 부재로 Enter 무반응**

특히 MyPageClient 보드 생성 폼과 input/button className idiom·엔티티(보드명)가 동일한데 Enter 거동만 갈리고, ProfileEditForm이 2-input 편집 폼인데 native form으로 Enter 제출이 가능해 "다중 입력 편집 폼이라 Enter 미지원"이라는 역할 가설은 성립하지 않는다. 지배 관례 5/6 정렬 사안.

## What Changes

- `apps/web/src/app/boards/[id]/BoardActions.tsx`의 편집 모드 이름 입력(:74)과 설명 입력(:85)에 인접 보드 생성 폼(MyPageClient:124·AddToBoardButton:372)과 동일한 idiom의 `onKeyDown` Enter 핸들러를 추가하여 Enter → `handleSave()` 실행.
- 시각 변경 없음. `saving` 중 중복 제출은 `handleSave` 진입 시점의 기존 상태 게이팅과 동일하게 동작(버튼과 동일 경로 호출).

## Capabilities

### Modified Capabilities

- `board`: 보드 편집 폼의 키보드 제출 어포던스 요구사항 추가.

## Impact

- 코드: `apps/web/src/app/boards/[id]/BoardActions.tsx` 1파일 (onKeyDown 2개 추가).
- 사용자: 키보드 사용자가 보드 이름/설명 입력에서 Enter로 저장 가능 — 인접 생성 폼·프로필 편집 폼과 거동 일치.
- 롤백: 커밋 revert 로 즉시 원복 가능(로컬 변경, 공개 API·토큰 비접촉).
