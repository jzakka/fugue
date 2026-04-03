# OG Fetch 서비스

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규

## 엔드포인트

```
POST /api/og/fetch  (public, rate limited: 20/min/IP)
body: { "url": "https://soundcloud.com/..." }
response: { "title", "description", "image", "site_name", "url", "detected_field" }
```

## 구현 범위

### 핵심

- [ ] `internal/og/` 패키지 생성
- [ ] HTML fetch + OG 메타태그 파싱 (og:title, og:description, og:image, og:site_name)
- [ ] Fallback 순서: og:* → twitter:* → `<title>` + `<meta name="description">`
- [ ] 도메인 기반 분야 자동감지 (soundcloud→음악, pixiv→미술, youtube→영상편집, github→프로그래밍)
- [ ] 실패 시 partial response (에러 메시지 + URL만 반환)

### 보안 (SSRF 방지)

- [ ] Allowed schemes: http/https만
- [ ] 커스텀 DialContext로 DNS 해석 후 resolved IP 검증
- [ ] Private IP 차단: 10.x, 172.16-31.x, 192.168.x, 127.x, ::1, 169.254.x (클라우드 메타데이터)
- [ ] 리다이렉트: 최대 5 hop, 매 hop마다 IP 재검증
- [ ] 응답 크기: 최대 1MB (io.LimitReader)
- [ ] 타임아웃: 커넥션 3초, 전체 5초

### Rate Limit

- [ ] Redis 기반 IP당 20/min (기존 auth rate limiter 패턴 재사용)

### 테스트

- [ ] 정상 URL → OG 파싱 성공
- [ ] SSRF 차단 (private IP)
- [ ] 타임아웃
- [ ] 도메인→분야 매핑
- [ ] OG 태그 없는 URL → fallback

## 알려진 제한

- pixiv: 비로그인 시 OG 제한적
- X/Twitter: 인증 필요한 콘텐츠 있음
- SPA 사이트: JS 렌더링 필요 시 OG 없을 수 있음 → 수동 입력 폴백

## 영향 범위

- `apps/api/internal/og/` (신규)
- `apps/api/cmd/server/main.go` (라우트 등록)
