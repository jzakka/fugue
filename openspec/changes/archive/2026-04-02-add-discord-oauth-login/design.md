## Context

Fugue 백엔드(`apps/api`)에 Discord OAuth provider 코드가 이미 구현되어 있다. `main.go`에서 `DISCORD_CLIENT_ID`와 `DISCORD_CLIENT_SECRET` 환경변수가 설정되면 자동으로 provider가 등록된다. 프론트엔드 `LoginButtons.tsx`는 `/api/auth/providers` API 응답을 기반으로 버튼을 동적 렌더링한다.

현재 상태: `.env`에 credential이 추가되었으므로, 서버 재시작 시 Discord 로그인이 활성화된다.

## Goals / Non-Goals

**Goals:**
- Discord OAuth 로그인 플로우가 정상 동작하는지 검증
- Discord Developer Portal redirect URI 설정 가이드 제공
- 로그인 → 콜백 → JWT 발급 → 프론트엔드 리다이렉트 E2E 동작 확인

**Non-Goals:**
- 백엔드 코드 변경 (이미 구현 완료)
- Twitter OAuth 추가 (별도 change로 진행)
- 계정 관리 UI (provider 연결/해제)

## Decisions

**1. 코드 변경 없이 설정만으로 활성화**
- 이유: `main.go:68-74`에서 env var 존재 여부로 conditional 등록하는 로직이 이미 있음
- 대안: provider를 필수로 변경 → 불필요 (다른 환경에서 Discord 없이도 동작해야 함)

**2. redirect URI는 클라이언트 도메인(localhost:3000) 사용**
- 이유: Next.js `rewrites`가 `/api/*`를 Go 서버로 프록시하므로, 브라우저 입장에서 단일 도메인
- 대안: 서버 도메인(localhost:8080) 직접 사용 → CORS/쿠키 도메인 불일치 문제 발생

## Risks / Trade-offs

- [Discord API rate limit] → 개발 단계에서는 문제없음. 프로덕션에서는 모니터링 필요
- [Discord 서비스 장애 시 로그인 불가] → Google 로그인이 fallback으로 존재
