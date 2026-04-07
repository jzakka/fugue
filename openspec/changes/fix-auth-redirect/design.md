## Root Cause Investigation

Investigate the auth redirect root cause. Check next.config, middleware, layout.tsx for auth guards. Fix by whitelisting public routes.

### Investigation Targets
1. `apps/web/middleware.ts` - Next.js 미들웨어에서 모든 경로에 인증 체크 적용 여부
2. `apps/web/next.config.*` - redirects/rewrites 설정 확인
3. `apps/web/app/layout.tsx` - 루트 레이아웃의 인증 가드 확인
4. 인증 관련 provider/context 컴포넌트 - 전역 인증 래핑 여부

### Fix Strategy
- 공개 경로 화이트리스트 방식으로 전환
- 미들웨어에서 matcher 또는 조건 분기로 공개/비공개 경로 분리
- 인증 필요 경로만 명시적 보호 (allowlist → denylist 전환)
