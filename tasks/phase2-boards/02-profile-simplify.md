# 프로필 간소화

**상태**: [ ] 미착수
**우선순위**: P1
**분류**: 변경 (기존 코드 수정)
**의존**: 01-migration-creators-simplify

## 변경 내용

프로필을 포트폴리오에서 단순 계정으로 변경. 닉네임 + 아바타 + 핀/보드 목록만.

### Backend

- [ ] `UpdateCreator` sqlc 쿼리 간소화 (nickname + avatar_url만)
- [ ] `creator/handler.go` UpdateMe: roles/bio/contacts 검증 로직 제거
- [ ] `creator/dto.go` PublicDTO/PrivateDTO: roles/bio/contacts/email/work_count 제거
- [ ] 프로필 응답에 보드 목록 포함 (또는 별도 API)

### Frontend

- [ ] ProfileHeader: 역할 태그, 자기소개, SNS 연락처 UI 제거
- [ ] ProfileEditForm: 닉네임 + 아바타만 수정 가능하도록 간소화
- [ ] MyPageClient: 보드 목록 표시 추가
- [ ] WorksGrid: 핀 삭제 버튼 추가 (본인 프로필에서만)

## 영향 범위

- `apps/api/internal/creator/handler.go`
- `apps/api/internal/creator/dto.go`
- `apps/api/db/queries/creators.sql`
- `apps/web/src/components/profile/ProfileHeader.tsx`
- `apps/web/src/components/profile/ProfileEditForm.tsx`
- `apps/web/src/components/profile/MyPageClient.tsx`
- `apps/web/src/components/profile/WorksGrid.tsx`
