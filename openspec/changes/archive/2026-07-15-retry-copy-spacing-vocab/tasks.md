# Tasks: retry-copy-spacing-vocab

## 1. 문자열 치환

- [x] 1.1 `apps/web/src/app/login/page.tsx` L7 `invalid_state` 문구를 "세션이 만료되었습니다. 다시 시도해주세요"로 변경
- [x] 1.2 `apps/web/src/app/login/page.tsx` L8 `exchange_failed` 문구를 "인증에 실패했습니다. 다시 시도해주세요"로 변경
- [x] 1.3 `apps/web/src/app/login/page.tsx` L29 fallback 문구를 "로그인에 실패했습니다. 다시 시도해주세요"로 변경

## 2. 검증

- [x] 2.1 "다시 시도해 주세요"(띄어쓰기) 잔존 0건 grep 확인 및 vitest 스냅샷/문구 참조 grep 확인
- [x] 2.2 tsc(typecheck) 및 vitest 통과 확인
- [x] 2.3 실 브라우저 QA — `/login?error=exchange_failed`, `/login?error=invalid_state`, `/login?error=nonexistent_code`에서 붙여쓰기 문구 렌더, `role="alert"` 구조 무변경, 콘솔 에러 0 확인
