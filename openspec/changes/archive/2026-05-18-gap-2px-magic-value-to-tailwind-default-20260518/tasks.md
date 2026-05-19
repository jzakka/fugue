## 1. 매직 리터럴 회수

- [x] 1.1 `apps/web/src/components/board/BoardCover.tsx:30` className에서 `gap-[2px]` → `gap-0.5` 1단어 교체
- [x] 1.2 `apps/web/src/components/feed/PinCard.tsx:32` className에서 `gap-[2px]` → `gap-0.5` 1단어 교체

## 2. 검증

- [x] 2.1 `grep -rnE 'gap-\[2px\]' apps/web/src` 결과가 0건임을 확인
- [x] 2.2 보드 cover 미니어처 그리드 및 오디오 카드 waveform 바 사이 간격이 동일 2px로 렌더링됨을 시각 비교
