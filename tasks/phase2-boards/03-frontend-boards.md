# 프론트엔드: 보드 페이지

**상태**: [ ] 미착수
**우선순위**: P0
**분류**: 신규
**의존**: 01-board-crud-api

## 페이지

- `/boards/:id` — 보드 상세 (핀 목록)
- 프로필 페이지에 보드 탭/목록 추가

## 기능

- [ ] 보드 생성 모달 (이름 + 설명 + 공개여부)
- [ ] 보드 상세 페이지 (핀 그리드)
- [ ] 보드 수정/삭제 (소유자만)
- [ ] 핀 등록 시 보드 선택 UI
- [ ] 작품 상세에서 "보드에 추가" 버튼
- [ ] 프로필에 보드 목록 표시

## API 클라이언트

- [ ] `createBoard`, `getBoard`, `updateBoard`, `deleteBoard`
- [ ] `addPinToBoard`, `removePinFromBoard`
- [ ] `listBoards(creatorId)`

## 영향 범위

- `apps/web/src/app/boards/[id]/page.tsx` (신규)
- `apps/web/src/components/board/` (신규)
- `apps/web/src/lib/api.ts`
