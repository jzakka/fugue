## 1. Discord Developer Portal 설정

- [x] 1.1 Discord Developer Portal에서 OAuth2 redirect URI 등록: `http://localhost:3000/api/auth/discord/callback`
- [x] 1.2 OAuth2 scopes에 `identify`와 `email`이 포함되어 있는지 확인

## 2. 로컬 환경 검증

- [x] 2.1 Go 서버 재시작 후 `GET /api/auth/providers` 응답에 `"discord"`가 포함되는지 확인
- [x] 2.2 프론트엔드 로그인 페이지에서 Discord 버튼이 렌더링되는지 확인

## 3. E2E 로그인 플로우 검증

- [x] 3.1 Discord 로그인 버튼 클릭 → Discord 인증 페이지로 리다이렉트 확인
- [x] 3.2 Discord에서 권한 허용 → 콜백 처리 → JWT 쿠키 설정 → 프론트엔드 리다이렉트 확인
- [x] 3.3 로그인 후 `GET /api/auth/me`에서 Discord 프로필 정보(nickname, avatar_url) 반환 확인

## 4. 계정 병합 검증

- [x] 4.1 기존 Google 계정과 동일 이메일의 Discord 로그인 시 같은 creator에 연결되는지 확인
- [x] 4.2 이메일 없는 Discord 계정 로그인 시 새 creator가 생성되는지 확인 (스킵 — 별도 계정 필요)
