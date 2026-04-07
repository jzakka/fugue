## Why

모든 서브 라우트(/pins/{id}, /search, /creators/{id}, /boards/{id} 등)가 비로그인 상태에서 로그인 페이지로 강제 리다이렉트된다. 핀 상세, 검색 결과, 크리에이터 프로필, 보드는 공개 페이지로 인증 없이 접근 가능해야 한다.

## What Changes

- `apps/web/src/proxy.ts`의 `PUBLIC_PATH_PREFIXES`에 공개 경로 prefix 추가
- `/pin/new` 페이지에 자체 인증 가드 추가 (defense-in-depth)

## Capabilities

### New Capabilities
_(없음)_

### Modified Capabilities
- `auth`: 공개 경로는 인증 없이 접근 가능해야 한다는 요구사항 추가

## Impact

- Frontend: `proxy.ts` 화이트리스트 수정, `/pin/new/page.tsx` 인증 가드 추가
- 공개 경로: /, /pins/*, /search, /creators/*, /boards/*, /login
- 인증 필요 경로: /mypage, /pin/new (자체 가드 + 미들웨어 이중 보호)
