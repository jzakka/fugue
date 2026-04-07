## Context

핀 상세 페이지(`apps/web/src/app/pins/[id]/page.tsx`)는 Server Component로, `getAuthUser()`를 호출하여 인증 상태를 확인할 수 있다. 현재 "보드에 추가" 버튼은 인증 상태와 무관하게 항상 `/login?redirect=...`로 이동하는 `<Link>`이다.

`addPinToBoard(boardId, pinId)` API 함수가 `apps/web/src/lib/api.ts`에 이미 존재하며, `fetchBoards(creatorId)`로 유저의 보드 목록 조회, `createBoard(data)`로 새 보드 생성이 가능하다. 백엔드 엔드포인트(`POST /api/boards/{id}/pins`)도 구현 완료 상태.

## Goals / Non-Goals

**Goals:**
- 인증된 유저가 핀 상세 페이지에서 보드 선택 모달을 통해 핀을 보드에 추가할 수 있도록 한다
- 비인증 유저는 기존과 동일하게 로그인 페이지로 이동 후 핀 상세로 복귀한다
- 모달 내에서 새 보드를 생성하고 즉시 핀을 추가할 수 있도록 한다
- 중복 추가 시 사용자 친화적인 오류 메시지를 표시한다

**Non-Goals:**
- 보드 검색/필터 (보드 수가 적은 MVP 단계에서 불필요)
- 핀 카드 컴포넌트에서의 보드 추가 (이번 변경은 핀 상세 페이지만 대상)
- 보드에서 핀 제거 UI (별도 변경으로 처리)

## Decisions

### D1: 인증 분기 — Server Component에서 인증 확인 후 Client Component에 전달

핀 상세 페이지는 Server Component이므로 `getAuthUser()`를 호출할 수 있다. 인증 유저 정보를 보드 추가 버튼 영역의 Client Component에 props로 전달한다.

- **대안 A (선택):** Server Component에서 `getAuthUser()` 호출 → user 존재 여부를 Client Component에 전달. 비인증이면 `<Link>`, 인증이면 모달 트리거 버튼 렌더링.
- **대안 B (기각):** Client Component에서 `/api/auth/me` 호출. 불필요한 추가 네트워크 요청. Server Component에서 이미 인증 확인이 가능하므로 비효율적.

### D2: 모달 구현 — 전용 Client Component

별도의 모달 라이브러리 없이 직접 구현한다. 프로젝트에 기존 모달/다이얼로그 패턴이 없으므로 간단한 오버레이+패널 구조로 작성한다. `DESIGN.md`의 디자인 토큰(surface-elevated 배경, border-radius lg: 16px, 간격)을 따른다.

### D3: 모달 내 보드 목록 — 유저 보드 전체 로드

모달 열림 시 `fetchBoards(userId)`로 유저의 전체 보드 목록을 로드한다. MVP 단계에서 보드 수가 적으므로 페이지네이션 없이 전체 로드. 보드가 없는 경우 빈 상태 메시지와 새 보드 생성 유도.

### D4: 새 보드 생성 — 모달 내 인라인 폼

MyPageClient의 보드 생성 패턴을 재활용한다. 모달 하단에 "새 보드 만들기" 버튼 → 이름 입력 → 생성 → 즉시 해당 보드에 핀 추가.

### D5: 성공/실패 피드백 — 모달 내 메시지

- 성공: 모달 내에 성공 메시지 표시 후 자동 닫힘
- 중복 추가 오류: 모달 내에 "이미 이 보드에 추가된 핀입니다" 메시지
- 기타 오류: 일반 오류 메시지

### D6: 키보드 접근성

ESC 키로 모달 닫기. 모달 외부(오버레이) 클릭으로 닫기. 모달 열릴 때 배경 스크롤 방지.

### D7: 행동 기록 — board_add interaction

`interaction` 도메인 스펙에 따라, 보드 추가 성공 시 `recordInteraction(pinId, "board_add")`를 fire-and-forget으로 호출한다. 기록 실패가 사용자 경험에 영향을 주지 않아야 한다(스펙 요구사항).

## Risks / Trade-offs

- **모달 라이브러리 미사용**: 포커스 트랩, aria 속성 등 완전한 접근성 구현이 누락될 수 있으나, MVP 단계에서는 기본적인 키보드/마우스 상호작용만 지원한다.
- **보드 전체 로드**: 보드 수가 많아지면 성능 문제 가능. 현재 MVP에서는 무시 가능하며, 필요 시 검색/페이지네이션 추가.
