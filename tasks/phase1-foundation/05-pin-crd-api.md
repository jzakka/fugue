# 핀 CRD API

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규
**의존**: 01-migration, 04-og-fetch

## 엔드포인트

```
POST   /api/works          [auth]  핀 생성
GET    /api/works/:id              핀 상세 (+ creator JOIN)
DELETE /api/works/:id       [auth]  핀 삭제 (본인만)
```

## sqlc 변경

- [ ] `DeleteWork` 어노테이션: `:exec` → `:execrows` (RowsAffected로 404 판별)
- [ ] `GetWorkWithCreator` 쿼리 신규 추가 (works JOIN creators)
- [ ] sqlc generate

## WorksQuerier 인터페이스 확장

현재 인터페이스에 List/Count만 있음. 추가 필요:
- [ ] `CreateWork`
- [ ] `DeleteWork` (반환 타입 int64로 변경)
- [ ] `GetWorkWithCreator`

## handler 구현

- [ ] `Create` handler
  - JWT에서 creator_id 추출
  - 입력 검증: url 필수, field 필수, tags 1~5개 (각 30자 이내)
  - `CreateWork` sqlc 호출
  - 201 Created 반환
- [ ] `Delete` handler
  - JWT에서 creator_id 추출
  - UUID 파싱
  - `DeleteWork` 호출 → rowsAffected == 0이면 404
  - 204 No Content 반환
- [ ] `GetByID` handler
  - UUID 파싱
  - `GetWorkWithCreator` 호출
  - creator 정보 포함 응답

## 라우트 등록 (main.go)

- [ ] POST /api/works (auth.JWTMiddleware)
- [ ] DELETE /api/works/:id (auth.JWTMiddleware)
- [ ] GET /api/works/:id (public)

## 테스트

- [ ] Create — 정상
- [ ] Create — 검증 실패 (빈 field, 태그 초과)
- [ ] Create — 미인증
- [ ] Delete — 정상 (본인 핀)
- [ ] Delete — 타인 핀 → 404
- [ ] Delete — 존재하지 않는 ID → 404
- [ ] GetByID — 정상
- [ ] GetByID — 없는 ID → 404

## 영향 범위

- `apps/api/db/queries/works.sql`
- `apps/api/internal/works/handler.go`
- `apps/api/internal/works/handler_test.go`
- `apps/api/internal/works/dto.go`
- `apps/api/cmd/server/main.go`
