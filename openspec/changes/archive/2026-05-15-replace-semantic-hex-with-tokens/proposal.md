## Why

`DESIGN.md` L50은 semantic 색 토큰을 명시한다: `success #34C759, warning #FFB800, error #FF3B30, info #5AC8FA`. `apps/web/src/app/globals.css` L19-22(`:root`)에 토큰이 정의되어 있고 L49-52의 `@theme inline`이 `--color-success`, `--color-warning`, `--color-error`, `--color-info`로 노출해 Tailwind v4 유틸리티 클래스(`bg-success`, `text-error` 등)를 통해 사용 가능하다.

그러나 `apps/web/src/components/board/AddToBoardButton.tsx:237-238`은 토큰 대신 raw hex 리터럴을 인라인 클래스로 박았다:

```tsx
feedback.type === "success"
  ? "bg-[#34C759]/10 border border-[#34C759]/30 text-[#34C759]"
  : "bg-[#FF3B30]/10 border border-[#FF3B30]/30 text-[#FF3B30]"
```

결과: 토큰 값이 변경돼도 이 파일은 자동으로 따라가지 않는다. Light mode 또는 향후 semantic 색 보정이 들어가면 이 컴포넌트만 다른 색을 띈다.

해당 패턴은 코드베이스에서 이 2줄에만 존재(`grep #34C759|#FF3B30|#FFB800|#5AC8FA` 결과: globals.css 정의처 4건 + AddToBoardButton 2건).

본 change는 그 2줄만 토큰 클래스로 교체한다.

## What Changes

- `apps/web/src/components/board/AddToBoardButton.tsx` 의 피드백 박스 className에서:
  - `bg-[#34C759]/10 border border-[#34C759]/30 text-[#34C759]` → `bg-success/10 border border-success/30 text-success`
  - `bg-[#FF3B30]/10 border border-[#FF3B30]/30 text-[#FF3B30]` → `bg-error/10 border border-error/30 text-error`
- 다른 동작·구조·접근성 속성 변경 없음. 색 값 자체는 동일(토큰 정의가 동일 hex).

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음. 디자인 시스템은 OpenSpec capability로 등록되어 있지 않다.

## Impact

- 영향 코드: `apps/web/src/components/board/AddToBoardButton.tsx` 단일 파일, L237-238 두 줄.
- 사용자 영향: 시각적으로 변화 없음(토큰 정의가 동일 hex). 추후 디자인 토큰 보정 또는 light mode 등에서 자동 연동.
- 성능 영향 없음.
- 의존성·인프라·DB 마이그레이션 없음.

## Rollback

- 변경 라인 2줄을 git revert 또는 hex 리터럴로 되돌리면 즉시 이전 상태로 복귀. 다른 파일 변경 없음.
