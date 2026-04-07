# Design: 타인의 보드 공개 탐색 경로

## API 변경

### 1. 보드 상세 응답 확장 — `GET /api/boards/{id}`

현재 보드 메타데이터만 반환하는 응답을 핀 목록을 포함하도록 확장한다.

**요청**: `GET /api/boards/{id}?limit=20&offset=0`

**응답 구조**:
```json
{
  "board": { /* 기존 보드 메타 (id, name, description, is_public, pin_count, cover_images, ...) */ },
  "pins": [ /* Pin 객체 배열 (creator 정보 + 태그 포함) */ ],
  "has_more": true
}
```

- `limit` 기본값 20, 최대 50
- `offset` 기본값 0
- 핀 목록은 보드에 추가된 시간 역순 정렬
- 각 핀에 creator 정보와 태그를 포함 (피드의 PinCard와 동일한 데이터)
- 기존 `ListBoardPins` sqlc 쿼리를 활용

### 2. 핀 소속 보드 조회 — `GET /api/pins/{id}/boards`

새 엔드포인트. 특정 핀이 소속된 공개 보드 목록을 반환한다.

**응답 구조**:
```json
{
  "boards": [
    {
      "id": "uuid",
      "name": "보드 이름",
      "creator_id": "uuid",
      "creator_nickname": "닉네임"
    }
  ]
}
```

- 공개 보드(`is_public = true`)만 반환
- 보드에 추가된 시간 역순 정렬
- 최대 10개 (보드 소속 수가 많을 경우 상위 10개)
- 새 sqlc 쿼리 필요: `ListPublicBoardsByPin`

### 3. DB 쿼리 추가

```sql
-- name: ListPublicBoardsByPin :many
-- 핀이 소속된 공개 보드 목록 (크리에이터 닉네임 포함)
SELECT b.id, b.name, b.creator_id, c.nickname AS creator_nickname
FROM board_pins bp
JOIN boards b ON b.id = bp.board_id
JOIN creators c ON c.id = b.creator_id
WHERE bp.pin_id = $1 AND b.is_public = true
ORDER BY bp.created_at DESC
LIMIT 10;
```

## 프론트엔드 변경

### 1. 크리에이터 프로필 보드 섹션

`/creators/{id}` 페이지에 보드 그리드를 추가한다.

- 핀 그리드 위에 보드 섹션 배치
- 기존 `/mypage`의 `BoardCover` 컴포넌트를 재사용 (공통 컴포넌트로 추출)
- 보드 생성 버튼은 표시하지 않음 (읽기 전용)
- 각 보드 카드 클릭 시 `/boards/{id}`로 이동
- `fetchBoards(creatorId)` API를 호출하여 서버사이드에서 데이터 로드

### 2. 핀 상세 보드 링크

`/pins/{id}` 페이지에 소속 보드 목록을 표시한다.

- 태그 섹션과 크리에이터 섹션 사이에 배치
- 각 보드를 링크형 칩으로 표시 (보드명 + 크리에이터 닉네임)
- 클릭 시 `/boards/{id}`로 이동
- `fetchPinBoards(pinId)` API 클라이언트 함수 추가

### 3. 보드 상세 페이지 정상화

`/boards/{id}` 페이지는 이미 `{ board, pins, has_more }` 형태를 기대하고 있으므로 API 변경 후 자동으로 정상 동작한다. 추가 프론트엔드 변경 불필요.

## 컴포넌트 구조

- `BoardCover` → 공통 컴포넌트로 추출 (`components/board/BoardCover.tsx`)
- `BoardGrid` → 새 공통 컴포넌트 (`components/board/BoardGrid.tsx`) — 보드 카드 그리드 (읽기 전용)
- `PinBoardLinks` → 새 컴포넌트 (`components/pin/PinBoardLinks.tsx`) — 핀 소속 보드 칩 목록
