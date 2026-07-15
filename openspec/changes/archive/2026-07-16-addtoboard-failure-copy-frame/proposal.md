# Proposal: addtoboard-failure-copy-frame

## Why

`apps/web/` 뮤테이션(생성·수정·삭제·추가) 실패 카피의 지배 문형은 "〈명사구〉에 실패했습니다"이다 (9곳: 파일 처리 PinCreateForm.tsx:142, 핀 등록 PinCreateForm.tsx:289, 보드 수정 BoardActions.tsx:41, 보드 삭제 BoardActions.tsx:56, 보드 생성 AddToBoardButton.tsx:223 · MyPageClient.tsx:49, 프로필 업데이트 ProfileEditForm.tsx:40, 인증 login/page.tsx:8, 로그인 login/page.tsx:29). 그러나 `AddToBoardButton.tsx:200`의 보드 추가 실패만 "보드에 추가하지 못했습니다. 다시 시도해주세요"로 "〈동사〉하지 못했습니다" 문형을 사용한다 — 동일 뮤테이션 실패 역할 내 유일한 이탈이며, 심지어 같은 컴포넌트 안(:223)의 보드 생성 실패는 지배 문형을 따른다.

c3737 선례(FeedContainer 어체 정합, PR #4730)에 따라 role-identical 카피의 지배 관례 정렬은 DESIGN.md L7 "디자인 시스템 일관성" 범위다. 페치 실패("불러올 수 없습니다")와 파이프라인 오류("오류가 발생했습니다")는 c3713 역할 분업 기판정에 따라 모집단 밖이다.

## What Changes

- `AddToBoardButton.tsx:200`의 보드 추가 실패 카피를 "보드에 추가하지 못했습니다. 다시 시도해주세요" → "보드 추가에 실패했습니다. 다시 시도해주세요"로 변경한다.
- "다시 시도해주세요" 접미는 login 선례("인증에 실패했습니다. 다시 시도해주세요")와 동형이므로 보존한다.
- 409 중복 카피("이미 이 보드에 추가된 핀입니다"), 성공 카피(`"${boardName}" 보드에 추가했습니다`), 보드 생성 실패 카피("보드 생성에 실패했습니다")는 변경하지 않는다.

## Capabilities

### Modified Capabilities

- `board`: 보드 핀 추가 실패 피드백 카피가 서비스 전반의 뮤테이션 실패 카피 지배 문형("〈명사구〉에 실패했습니다")을 따른다는 요구사항 추가.

## Impact

- 코드: `apps/web/src/components/board/AddToBoardButton.tsx` 1행 (문자열 1건).
- 사용자 영향: 보드 추가 실패 시 에러 문구만 변경. 동작·레이아웃·색·타이밍 무변경.
- 롤백: 문자열 1건 원복으로 즉시 롤백 가능.
