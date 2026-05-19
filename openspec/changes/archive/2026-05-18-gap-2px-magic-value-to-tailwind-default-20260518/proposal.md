## Why

`apps/web/src/components/board/BoardCover.tsx:30` (보드 cover 2x2 미니어처 그리드 셀 간 gap)과 `apps/web/src/components/feed/PinCard.tsx:32` (오디오 카드 waveform 12개 바 사이 gap) 두 곳에서 `gap-[2px]` 매직값을 직접 사용 중. DESIGN.md L54-65 Spacing 섹션은 `Base unit: 4px` + Scale `2xs: 2px / xs: 4px / sm: 8px / md: 16px / lg: 24px / xl: 32px / 2xl: 48px / 3xl: 64px`를 명시하며, 2px는 L58 '2xs' 단계 직접 매핑. Tailwind v4 기본 spacing scale의 `0.5` 단계가 0.5 × 0.25rem = 0.125rem = 2px로 동일 값이므로 매직 리터럴을 표준 토큰으로 회수해 SSoT 정합성을 회복할 수 있다.

## What Changes

- `apps/web/src/components/board/BoardCover.tsx:30` className `gap-[2px]` → `gap-0.5` 1단어 교체
- `apps/web/src/components/feed/PinCard.tsx:32` className `gap-[2px]` → `gap-0.5` 1단어 교체

## Capabilities

### New Capabilities

- `design-tokens`: DESIGN.md SSoT의 행위 계약을 코드에 일관 반영하는 영역. 본 변경은 spacing 매직 리터럴을 표준 토큰으로 회수하는 요구사항을 추가한다.

### Modified Capabilities

(없음)

## Impact

- 영향 파일: `apps/web/src/components/board/BoardCover.tsx` + `apps/web/src/components/feed/PinCard.tsx` 2개 파일 각 1단어 교체.
- 사용자 영향: 시각·동작 변화 없음 (동일 2px 간격). 보드 cover 미니어처 그리드와 오디오 카드 waveform 바 사이 간격이 동일하게 렌더링됨. SSoT 측면에서 매직 리터럴 회수로 DESIGN.md L58 '2xs: 2px' spacing 카테고리 의도가 코드에 명시화.
- 외부 영향 없음: `apps/api/` 및 `apps/web/` 외부 파일 미변경. Tailwind 설정·globals.css 미변경(Tailwind v4 기본 spacing scale을 그대로 사용).
- anti-pattern L15(Tailwind 기본 클래스 의미 덮어쓰기) 미해당: `gap-0.5`는 Tailwind v4 기본 spacing scale의 표준 토큰이며 의미 덮어쓰기 없음.
- 롤백 절차: git revert로 단일 커밋 되돌리기.
