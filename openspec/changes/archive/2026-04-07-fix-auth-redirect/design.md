## Root Cause

`apps/web/src/proxy.ts`가 Next.js 미들웨어로 컴파일되어 모든 라우트에 인증 체크를 적용한다.

```typescript
// proxy.ts line 4-5
const PUBLIC_PATH_PREFIXES = ["/login", "/health", "/api"];
const PUBLIC_EXACT_PATHS = ["/"];
```

이 화이트리스트에 `/pins`, `/search`, `/creators`, `/boards`가 누락되어, 비로그인 유저가 이 경로에 접근하면 `fugue_access` 쿠키 부재로 `/login`으로 리다이렉트된다.

## Fix Strategy

**Option A (최소 변경, 선택):** `PUBLIC_PATH_PREFIXES`에 누락된 공개 경로 추가.

```typescript
const PUBLIC_PATH_PREFIXES = ["/login", "/health", "/api", "/pins", "/search", "/creators", "/boards"];
```

주의: `/pins`(복수)와 `/pin`(단수, `/pin/new`)는 prefix가 다르므로 `/pins`를 추가해도 `/pin/new`는 보호 상태 유지.

**Option B (구조 전환, 기각):** 보호 필요 경로만 나열하는 denylist 방식으로 전환. 향후 새 공개 페이지 추가 시 화이트리스트 갱신을 잊는 문제를 방지하지만, 구조 변경이 크고 기존 동작 검증 범위가 넓어짐. MVP에서는 A가 적절.

## Defense-in-Depth

`/pin/new/page.tsx`에는 자체 인증 가드가 없다 (미들웨어에 전적으로 의존). `/mypage/page.tsx`는 이미 `getAuthUser()` + `redirect("/login")` 자체 가드가 있다. `/pin/new`에도 동일한 자체 가드를 추가하여 미들웨어 설정이 변경되더라도 보호가 유지되도록 한다.
