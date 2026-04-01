## Why

Fugue는 Google OAuth만 활성화된 상태. Discord는 코드가 이미 구현되어 있지만 credential이 없어서 비활성화 상태였다. 이제 Discord OAuth 앱이 등록되었으므로 로그인 옵션으로 활성화하고, 프론트엔드 로그인 UI에 Discord 버튼이 정상 노출되도록 한다.

## What Changes

- `.env`에 Discord OAuth credential 추가 (완료)
- Discord Developer Portal에 redirect URI 등록 (`http://localhost:3000/api/auth/discord/callback`)
- 프론트엔드 로그인 페이지에서 Discord 버튼 정상 노출 검증
- E2E 로그인 플로우 동작 검증 (로그인 → 콜백 → JWT 발급 → 리다이렉트)

## Capabilities

### New Capabilities
- `discord-oauth`: Discord OAuth 로그인 플로우 활성화 및 검증

### Modified Capabilities

(없음 — 백엔드 코드는 이미 Discord를 지원하며 requirement 변경 없음)

## Impact

- **Config**: `apps/api/.env`에 `DISCORD_CLIENT_ID`, `DISCORD_CLIENT_SECRET` 추가
- **External**: Discord Developer Portal에 redirect URI 등록 필요
- **Frontend**: `LoginButtons.tsx`가 `/api/auth/providers` 응답에 `discord`가 포함되면 자동으로 버튼 렌더링
- **Backend**: 코드 변경 없음 — `main.go`에서 env var가 있으면 자동으로 provider 등록
