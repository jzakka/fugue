# 연관 작품 API

**상태**: [ ] 미착수
**우선순위**: P1
**분류**: 신규
**의존**: Phase 1 핀 CRD

## 엔드포인트

```
GET /api/works/:id/related
response: { works: [...], max 10 }
```

## 로직 (v1)

- 해당 작품의 태그와 겹치는 다른 작품을 태그 일치순 정렬
- 같은 분야 우선, 크로스 분야도 포함
- 해당 작품 자체는 제외
- 최대 10개

## 할 일

- [ ] sqlc 쿼리 작성 (works.sql에 추가)
- [ ] works handler에 `Related` 메서드 추가
- [ ] main.go 라우트 등록 (GET /api/works/:id/related)
- [ ] 프론트: 작품 상세 페이지 하단에 연관 작품 그리드
- [ ] 테스트

## 영향 범위

- `apps/api/db/queries/works.sql`
- `apps/api/internal/works/handler.go`
- `apps/api/cmd/server/main.go`
- `apps/web/src/app/works/[id]/page.tsx` (신규)
