## Why

`DESIGN.md` L73-77은 border radius 스케일을 네 단계로 한정한다: `sm: 6px (inputs, alerts)` / `md: 10px (cards)` / `lg: 16px (modals, panels)` / `full: 9999px (buttons, chips, avatars, search bar)`. 모달/패널은 `lg = 16px`로 명시되어 있고, 실제로 다른 모달 패널은 이 값을 따른다 — 예: `apps/web/src/components/board/AddToBoardButton.tsx:203` (`rounded-[16px]`).

그러나 `apps/web/src/components/pin/VideoTrimModal.tsx:121`의 트림 모달 외곽 패널만 `rounded-[12px]`(매직값)으로 모서리가 어긋난다. 비디오 핀 업로드 흐름에서 이 모달만 다른 모달보다 살짝 더 각져 보인다.

본 change는 해당 한 줄을 DESIGN.md 스펙에 맞춰 `rounded-[16px]`로 교체해 모달 radius 일관성을 회복한다.

## What Changes

- `apps/web/src/components/pin/VideoTrimModal.tsx:121`의 외곽 패널 className에서 `rounded-[12px]` → `rounded-[16px]`로 변경한다.
- 모달 내부의 video preview(L136 `rounded-[8px]`), track/handle(L156,163,167,188,197) radius는 본 change 범위 밖. 이들은 카드/UI 요소 radius 결정이 필요한 별도 후보로 남는다.

## Capabilities

### New Capabilities
없음. 디자인 시스템은 OpenSpec capability로 등록되어 있지 않다.

### Modified Capabilities
없음.

## Impact

- 영향 코드: `apps/web/src/components/pin/VideoTrimModal.tsx` 단일 파일, 1 라인.
- 사용자 영향: 비디오 핀 업로드 시 트림 모달 외곽 모서리가 4px 더 둥글어진다. 다른 모달(보드 추가, 프로필 편집 등)과 모서리가 일관된다.
- 의존성·인프라·DB 마이그레이션 없음.

## Rollback

- 해당 한 줄을 `rounded-[12px]`로 복원하거나 git revert. 다른 파일 변경 없음.
