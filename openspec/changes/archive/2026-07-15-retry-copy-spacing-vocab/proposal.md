# Proposal: retry-copy-spacing-vocab

## Why

사용자-대면 "~해주세요" 권고 카피 전수 9곳 중 6곳(PinCreateForm:256/260, BoardActions:26, ProfileEditForm:27, search/page.tsx:76, AddToBoardButton:200)이 보조용언 붙여쓰기("선택해주세요"/"입력해주세요"/"시도해주세요")로 균일한데, login/page.tsx의 에러 카피 3건(:7 invalid_state, :8 exchange_failed, :29 fallback)만 띄어쓰기("다시 시도해 주세요")로 이탈한다. 특히 동일한 재시도 권고 문구가 AddToBoardButton("다시 시도해주세요")과 login 페이지("다시 시도해 주세요")에서 두 표기로 갈린다. 국어 규범상 두 표기 모두 유효하므로 규범 문제가 아니라 표면 간 표기 정합 문제이며, majority(6:3)와 동일-문구 직접 충돌 기준으로 붙여쓰기가 canonical이다.

## What Changes

- `apps/web/src/app/login/page.tsx`의 에러 카피 3건에서 "다시 시도해 주세요" → "다시 시도해주세요"로 표기 정합화 (문자열 리터럴만 변경, 렌더 구조·로직 무변경)

## Capabilities

### Modified Capabilities

- `auth`: 로그인 페이지 에러 메시지의 재시도 권고 문구 표기가 코드베이스 확립 어휘(붙여쓰기)를 따른다

## Impact

- **코드**: `apps/web/src/app/login/page.tsx` 3개 문자열 리터럴 (ERROR_MESSAGES.invalid_state, ERROR_MESSAGES.exchange_failed, fallback 메시지)
- **비영향**: 렌더 구조(role=alert, aria-live, text-error), 에러 코드 키, 로그인 동작, AddToBoardButton 등 타 표면 카피
