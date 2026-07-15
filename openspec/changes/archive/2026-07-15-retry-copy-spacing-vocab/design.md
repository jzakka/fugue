# Design: retry-copy-spacing-vocab

## Context

login/page.tsx는 OAuth 콜백 에러 코드를 `ERROR_MESSAGES` 상수 맵으로 사용자-대면 문구에 매핑하고, 미등록 코드는 fallback 문자열로 처리한다. 이 중 재시도 권고를 포함하는 3개 문구가 "다시 시도해 주세요"(보조용언 띄어쓰기)로 작성되어 있어, 코드베이스의 확립 표기(붙여쓰기 "~해주세요" 6곳, 특히 AddToBoardButton:200의 동일 문구 "다시 시도해주세요")와 갈린다.

## Goals / Non-Goals

- **Goal**: 재시도 권고 문구 표기를 붙여쓰기로 통일하여 동일 문구의 2표기 갈림 해소
- **Non-Goal**: 에러 메시지 의미/구조 변경, 에러 코드 체계 변경, 다른 표면 카피 변경, 띄어쓰기 규범 강제(양 표기 모두 규범상 유효 — majority 정렬일 뿐)

## Decisions

### Decision 1: 문자열 리터럴 3건만 치환

`apps/web/src/app/login/page.tsx`:

```tsx
// L7
invalid_state: "세션이 만료되었습니다. 다시 시도해주세요",
// L8
exchange_failed: "인증에 실패했습니다. 다시 시도해주세요",
// L29
? ERROR_MESSAGES[errorCode] || "로그인에 실패했습니다. 다시 시도해주세요"
```

- 대안(기각): AddToBoardButton 쪽을 띄어쓰기로 변경 — majority(6:3)가 붙여쓰기이고 검증 프롬프트 5곳 전부 붙여쓰기이므로 login 쪽이 outlier. 기각.
- 대안(기각): 공용 카피 상수 모듈 추출 — 3건 문자열 정합에 과설계(effort 초과). 기각.

### Decision 2: 렌더 구조 무변경

`role="alert" aria-live="polite" className="mt-4 text-center text-sm text-error"` 문단 구조는 그대로 유지. 문자열 값만 바뀐다.

## Risks / Trade-offs

- **Risk**: 없음 수준 — 문자열 리터럴 3건, 로직/구조/스타일 무변경. 스냅샷 테스트가 있다면 갱신 필요하나 vitest에 해당 문구 스냅샷 없음(구현 단계에서 grep 확인).
