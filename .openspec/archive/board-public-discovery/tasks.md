# Tasks: 타인의 보드 공개 탐색 경로

## Backend

- [x] **TASK-1**: 보드 상세 API 핀 목록 포함
  - `GET /api/boards/{id}` 핸들러에서 `ListBoardPins` 쿼리 호출
  - 응답을 `{ board, pins, has_more }` 형태로 변경
  - limit/offset 쿼리 파라미터 지원
  - 각 핀에 creator 정보 및 태그 포함

- [x] **TASK-2**: 핀 소속 보드 조회 쿼리 추가
  - `ListPublicBoardsByPin` sqlc 쿼리 작성 (boards.sql)
  - sqlc 코드 생성 실행

- [x] **TASK-3**: 핀 소속 보드 API 엔드포인트 추가
  - `GET /api/pins/{id}/boards` 핸들러 구현
  - 라우터에 등록
  - 공개 보드만 반환, 최대 10개

## Frontend

- [x] **TASK-4**: BoardCover 공통 컴포넌트 추출
  - MyPageClient에서 BoardCover를 `components/board/BoardCover.tsx`로 추출
  - MyPageClient에서 import 경로 변경

- [x] **TASK-5**: 크리에이터 프로필 보드 섹션 추가
  - `/creators/{id}/page.tsx`에서 `fetchBoards` 호출
  - 보드 그리드를 핀 그리드 위에 배치
  - 읽기 전용 (보드 생성 버튼 없음)

- [x] **TASK-6**: 핀 상세 소속 보드 링크 추가
  - `fetchPinBoards` API 클라이언트 함수 추가 (lib/api.ts)
  - `/pins/{id}/page.tsx`에 보드 링크 칩 섹션 추가

## 검증

- [ ] **TASK-7**: 보드 상세 페이지 정상 동작 확인
  - TASK-1 완료 후 `/boards/{id}` 페이지가 보드 메타 + 핀 목록을 정상 렌더링하는지 확인
  - 기존 프론트엔드 코드가 `{ board, pins, has_more }` 응답을 기대하므로 API 변경만으로 정상 동작해야 함

## 의존성

```
TASK-2 → TASK-3 (쿼리 먼저, 핸들러 다음)
TASK-1 (독립적, 기존 쿼리 활용)
TASK-4 → TASK-5 (공통 컴포넌트 먼저)
TASK-3 → TASK-6 (API 먼저, 프론트 다음)
TASK-1 → TASK-7 (API 변경 후 검증)
```
