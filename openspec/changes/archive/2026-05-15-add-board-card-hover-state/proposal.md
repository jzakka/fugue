## Why

`DESIGN.md` L86은 카드 hover state를 명시한다: `Hover state: translateY(-2px) + box-shadow 확대 + accent border. 150ms ease.` (Card System 섹션). 동 섹션은 카드의 시각 단위를 `rounded-[10px]` overflow-hidden 컨테이너로 정의(L74-77).

`PinCard.tsx:142`는 이 스펙을 충실히 구현(`hover:-translate-y-0.5 hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)] hover:border-accent`, `border border-transparent` 베이스로 hover 시 레이아웃 시프트 없음).

그러나 보드 카드는 두 곳에서 동일하게 누락되어 있다:

- `apps/web/src/components/board/BoardGrid.tsx:22-39` — `<Link className="group">` 안에 `BoardCover` + 라벨. hover시 라벨 텍스트만 accent로 바뀌고 카드(BoardCover) 자체는 위로 뜨지 않음/그림자 없음/accent border 없음.
- `apps/web/src/components/profile/MyPageClient.tsx:130-147` — 동일 패턴, 동일 누락.

`BoardCover.tsx`는 BoardGrid/MyPageClient에서만 사용(grep 확인)하며 두 사용처 모두 `<Link className="group">` 컨텍스트 안에 둔다. 따라서 `BoardCover` 외곽 div에 `group-hover:` 트리거를 부여하면 두 사용처에 동시 적용된다.

## What Changes

1. `apps/web/src/components/board/BoardCover.tsx`의 두 분기(빈 상태·이미지 채워진 상태) 외곽 div에 다음 클래스 추가:
   - `border border-transparent group-hover:border-accent` — accent border (베이스는 투명, 레이아웃 시프트 없음).
   - `group-hover:shadow-[0_8px_32px_rgba(0,0,0,0.3)]` — box-shadow 확대 (PinCard와 동일 값).
   - `transition-all duration-200` — 트랜지션 (PinCard와 동일).
2. `apps/web/src/components/board/BoardGrid.tsx:25`의 Link className `"group"` → `"group block transition-transform duration-200 hover:-translate-y-0.5"`.
3. `apps/web/src/components/profile/MyPageClient.tsx:133`의 Link className `"group"` → `"group block transition-transform duration-200 hover:-translate-y-0.5"`.

설계 결정: translateY는 Link 전체(BoardCover + 라벨)에 적용해 그룹이 한 단위로 들리도록 한다. shadow/border는 시각 카드인 BoardCover에만 적용해 라벨 영역까지 그림자/테두리가 번지지 않게 한다. duration은 PinCard와 일치하는 200ms (DESIGN.md L86 150ms ↔ L92 200ms 자체 모순이며 기존 카드 hover와 동기화 우선).

## Capabilities

### New Capabilities
없음.

### Modified Capabilities
없음.

## Impact

- 영향 코드: BoardCover.tsx / BoardGrid.tsx / MyPageClient.tsx 3 파일.
- 사용자 영향: 보드 카드에 마우스를 올리면 (a) 카드가 -2px 위로 뜨고 (b) 카드 주변에 그림자가 확장되며 (c) accent 1px 테두리가 그려진다. 기존 라벨 텍스트 accent 전환 효과는 그대로 유지.
- 레이아웃 시프트: 없음(베이스 `border-transparent`로 1px 공간 사전 확보).
- 성능: GPU 가속 transform/shadow/opacity transition. 영향 없음.
- 의존성·인프라·DB 마이그레이션 없음.

## Rollback

- 3 파일의 변경 라인을 git revert하면 즉시 이전 상태로 복귀.
