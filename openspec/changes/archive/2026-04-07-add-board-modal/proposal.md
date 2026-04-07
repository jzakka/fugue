## Why

핀 상세 페이지의 "보드에 추가" 버튼이 항상 로그인 페이지로 리다이렉트하는 `<Link>`로 구현되어 있다. 로그인 후 핀 상세로 돌아오지만 보드 선택 UI가 없어 아무 동작도 일어나지 않는다. `addPinToBoard()` API 함수가 정의되어 있지만 어디에서도 호출되지 않으며, 백엔드(`POST /api/boards/{id}/pins`)와 DB 쿼리는 정상 구현되어 있다.

보드에 핀을 추가하는 것은 큐레이션 플랫폼의 핵심 행위이며, board 도메인 스펙에도 이미 정의된 요구사항이다.

## What Changes

- 핀 상세 페이지의 "보드에 추가" 버튼을 인증 상태에 따라 분기 처리
  - 비로그인: 로그인 페이지로 리다이렉트 후 복귀
  - 로그인: 보드 선택 모달 표시
- 보드 선택 모달 컴포넌트 생성: 내 보드 목록 표시, 보드 선택 시 핀 추가 API 호출, 모달 내 새 보드 생성 지원
- 성공/실패 피드백 제공
- 보드 추가 성공 시 행동 기록(board_add interaction) 호출

## Capabilities

### New Capabilities
_(없음 — 기존 board 도메인의 "보드에 핀을 추가한다" 요구사항에 대한 프론트엔드 구현)_

### Modified Capabilities
_(없음 — board 도메인과 interaction 도메인의 기존 요구사항을 프론트엔드에서 구현하는 것이므로 스펙 변경 불필요)_

## Impact

- **Frontend**: `apps/web/src/app/pins/[id]/page.tsx` 수정 (보드 추가 버튼 분기), 보드 선택 모달 클라이언트 컴포넌트 신규, `apps/web/src/lib/auth.ts`의 인증 상태를 핀 상세 페이지에서 활용
- **Backend**: 변경 없음 (API 이미 구현됨)
- **DB**: 변경 없음
