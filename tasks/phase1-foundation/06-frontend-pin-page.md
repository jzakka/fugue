# 프론트엔드: 핀 등록 페이지

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규
**의존**: 04-og-fetch, 05-pin-crd-api

## 페이지

`/pin/new` (authenticated)

## 기능

- [ ] URL 입력 필드 (포커스 시 확대)
- [ ] 실시간 OG 프리뷰: URL 입력 완료 (debounce 500ms) → POST /api/og/fetch → 카드형 프리뷰
- [ ] 이전 요청 취소 (AbortController)
- [ ] 분야 자동감지: OG fetch 응답의 detected_field → 분야 자동 선택, 변경 가능
- [ ] 태그 chip 입력 (1~5개)
- [ ] 제목/설명: OG에서 가져온 값이 기본값, 수정 가능
- [ ] 제출 → POST /api/works → 성공 시 /mypage로 리다이렉트
- [ ] OG fetch 실패 시 수동 입력 폼 표시
- [ ] NavBar에 "+" 버튼 → /pin/new 이동

## API 클라이언트

- [ ] `fetchOgPreview(url)` 함수 추가 (lib/api.ts)
- [ ] `createPin(data)` 함수 추가 (lib/api.ts)

## 에러 상태

- [ ] 유효하지 않은 URL → 인라인 에러 메시지
- [ ] OG fetch 타임아웃 → "프리뷰를 가져올 수 없습니다" + 수동 입력
- [ ] 제출 실패 → 에러 토스트

## 영향 범위

- `apps/web/src/app/pin/new/page.tsx` (신규)
- `apps/web/src/lib/api.ts` (함수 추가)
- `apps/web/src/components/nav/NavBar.tsx` (+ 버튼 추가)
