## 1. API 수정

- [x] 1.1 `apps/api/internal/pin/handler.go`에서 태그 최소 1개 검증 제거 (빈 배열 허용, 최대 10개 제한 유지)
- [x] 1.2 태그 배열이 비어있을 때 태그 연결 단계를 스킵하도록 분기 처리

## 2. Frontend 수정

- [x] 2.1 `PinCreateForm.tsx`에서 `selectedTagIds.size === 0` 에러 검증 제거
- [x] 2.2 태그 섹션의 필수 표시(*) 제거, "선택사항" 안내로 변경

## 3. 스펙 반영

- [x] 3.1 `openspec/specs/pin/spec.md`에 변경된 태그 요구사항 반영 (아카이브 시 자동 적용)
