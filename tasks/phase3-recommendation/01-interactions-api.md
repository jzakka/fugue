# Interactions 기록 API

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규
**의존**: Phase 1 마이그레이션

## 엔드포인트

```
POST /api/interactions  [auth]
body: { "work_id": "uuid", "type": "view" }
type: 'view' | 'pin' | 'board_add'
```

## 할 일

- [ ] `internal/interaction/` 패키지 생성
- [ ] handler: type 검증, 중복 view 방지 (같은 유저+작품+분 단위 dedup 등)
- [ ] main.go 라우트 등록
- [ ] 프론트: 작품 상세 페이지 진입 시 자동으로 view 이벤트 전송
- [ ] 프론트: 핀 생성 시 pin 이벤트 전송
- [ ] 프론트: 보드에 핀 추가 시 board_add 이벤트 전송

## 설계 메모

- view 이벤트는 프론트에서 fire-and-forget (응답 기다리지 않음)
- 실패해도 UX에 영향 없음 (추천 정확도만 약간 떨어짐)
- 이 데이터는 추후 피처스토어 → ML 학습 데이터로 활용

## 영향 범위

- `apps/api/internal/interaction/` (신규)
- `apps/api/db/queries/interactions.sql` (신규)
- `apps/api/cmd/server/main.go`
- `apps/web/src/app/works/[id]/page.tsx` (view 이벤트)
- `apps/web/src/lib/api.ts`
