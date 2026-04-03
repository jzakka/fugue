# 보드 CRUD API

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규
**의존**: Phase 1 마이그레이션

## 엔드포인트

```
POST   /api/boards                      [auth]  보드 생성
GET    /api/boards/:id                          보드 조회 (공개 또는 본인)
PUT    /api/boards/:id                  [auth]  보드 수정 (소유자만)
DELETE /api/boards/:id                  [auth]  보드 삭제 (소유자만)
GET    /api/boards?creator_id=          보드 목록 (본인=전체, 타인=공개만)
POST   /api/boards/:id/pins            [auth]  보드에 핀 추가 (소유자만)
DELETE /api/boards/:id/pins/:work_id   [auth]  보드에서 핀 제거 (소유자만)
```

## 할 일

- [ ] `internal/boards/` 패키지 생성
- [ ] sqlc 쿼리 작성 (boards.sql)
- [ ] handler 구현 (CRUD + 핀 추가/제거)
- [ ] 소유자 검증 로직
- [ ] 비공개 보드 접근 제어
- [ ] main.go 라우트 등록
- [ ] 테스트

## 영향 범위

- `apps/api/internal/boards/` (신규)
- `apps/api/db/queries/boards.sql` (신규)
- `apps/api/cmd/server/main.go`
