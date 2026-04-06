## 1. SQL 쿼리 수정

- [ ] 1.1 `apps/api/db/queries/pins.sql`의 RelatedPins 쿼리에서 `p.tags & $2::text[]`를 `ARRAY(SELECT unnest(p.tags) INTERSECT SELECT unnest($2::text[]))` 기반 교집합 크기 계산으로 교체
- [ ] 1.2 sqlc 코드 재생성 (`sqlc generate`)

## 2. 검증

- [ ] 2.1 로컬 DB에서 연관 핀 쿼리 직접 실행하여 에러 없이 결과 반환 확인
- [ ] 2.2 `GET /api/pins/{id}/related` API 호출하여 정상 응답 확인
- [ ] 2.3 기존 테스트 통과 확인 (`go test ./internal/pin/...`)
