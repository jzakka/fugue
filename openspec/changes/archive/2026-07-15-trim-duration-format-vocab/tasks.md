# Tasks: trim-duration-format-vocab

## 1. 구현

- [x] 1.1 `PinCreateForm.tsx` 모듈 레벨(기존 `formatSize` 인접)에 `formatTime` 헬퍼 추가 — VideoTrimModal:15-18과 동일 로직 (`m:ss.s`)
- [x] 1.2 `PinCreateForm.tsx:418` 트림 요약 표기 변경 — `{trimStart.toFixed(1)}s ~ {trimEnd.toFixed(1)}s` → `{formatTime(trimStart)} ~ {formatTime(trimEnd)}`, 길이 `({…}초)` 유지

## 2. 검증

- [x] 2.1 `cd apps/web && npm run lint && npm run typecheck && npm test -- --run` 통과
- [x] 2.2 실 브라우저 QA — dev 서버에서 `/pin/new` 비디오 업로드 → 트림 모달 선택 완료 → 요약 칩이 `m:ss.s ~ m:ss.s (N.N초)`로 렌더되고 모달 판독 값과 동일한지 확인, 콘솔 에러 0, 인접 회귀(모달 판독 행·formatSize·mediaType 칩) 0
