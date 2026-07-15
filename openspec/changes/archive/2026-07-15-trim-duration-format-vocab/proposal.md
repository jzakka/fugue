# Proposal: trim-duration-format-vocab

## Why

비디오 핀 생성 플로우에서 duration 판독(readout) 표기가 같은 값에 대해 표면 간 갈린다.

- **VideoTrimModal** (확립 어휘, 3+3 사이트 균일): 시각 위치는 `fmt()` `m:ss.s` 포맷(`VideoTrimModal.tsx:229` fmt(start) · `:231` fmt(end) · `:288` fmt(videoDuration)), 구간 길이는 한글 `초` 단위(`:230` `{clip.toFixed(1)}초 / {MAX_CLIP}초` · `:211` 카피 `최대 {MAX_CLIP}초`).
- **PinCreateForm 트림 요약 칩** (유일 outlier, `PinCreateForm.tsx:418`): 모달 `onConfirm(start, end)`에서 그대로 넘어온 **동일 값**을 `{trimStart.toFixed(1)}s ~ {trimEnd.toFixed(1)}s ({(trimEnd - trimStart).toFixed(1)}초)`로 렌더.

사용자가 모달에서 `0:05.0 ~ 0:12.3`으로 확인한 구간이 폼 요약에서는 `5.0s ~ 12.3s (7.3초)`로 재표기된다. (a) 위치 포맷이 모달 확립 채널(`m:ss.s`)에서 이탈하고, (b) 한 줄 안에서 위치 단위 라틴 `s`와 길이 단위 한글 `초`가 혼용된다.

DESIGN.md는 duration 포맷 자체를 규정하지 않으나, L20(Data 수치는 Geist Mono — 두 표면 모두 `font-mono` 준수)·L34(2xs = timestamps, duration role) 하의 동일 아키타입 내 무근거 표기 분기이며, cycle 3689(loadmore 버튼 채움 어휘)와 동형의 majority 정렬 축이다 — 모달 어휘가 canonical(위치 `m:ss.s` 3사이트 vs raw-s 1표면, 길이 `초` 3사이트 vs 혼용 1표면).

백로그 항목: `design-20260715-trim-duration-format-vocab` (impact 2 · confidence 3 · effort 1 · risk 1 → score 6.0)

## What Changes

`PinCreateForm.tsx:418` 트림 요약 칩의 duration 표기를 VideoTrimModal 확립 어휘로 정합화한다.

- 위치 표기: `{trimStart.toFixed(1)}s ~ {trimEnd.toFixed(1)}s` → `{fmt(trimStart)} ~ {fmt(trimEnd)}` (`m:ss.s` 포맷, 예 `0:05.0 ~ 0:12.3`)
- 길이 표기: `({…}초)` 유지 (이미 모달 어휘와 일치)
- `fmt` 헬퍼는 PinCreateForm에 파일-로컬로 추가한다(4줄, VideoTrimModal:15-18과 동일 로직). 공유 모듈 추출은 하지 않는다 — cycle 3695 판정: 코드베이스의 확립 재사용 채널은 파일-로컬 추출이며 공유 모듈 채널은 0건, 공유 모듈 신설은 시각-무영향 리팩터링이라 루프 범위 밖.

**변경하지 않는 것**: VideoTrimModal의 판독 행(canonical 표면), 파일 크기 표기(`formatSize` KB/MB — 별개 수량 종류), mediaType 칩, 요약 칩의 클래스(`text-text-dim font-mono`).

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities

- `pin`: "트리밍 확인 시 구간 정보를 폼에 전달한다" Requirement에 시나리오 추가 — 폼의 트림 요약이 트리밍 모달과 동일한 시간 표기로 구간을 표시한다.

## Impact

- 코드: `apps/web/src/app/pin/new/PinCreateForm.tsx` 1파일 (fmt 헬퍼 추가 + 1줄 표기 변경)
- 사용자 영향: 비디오 핀 생성 시 트림 요약 칩의 텍스트 표기만 변경. 레이아웃·색·토큰·동작 불변.
- 롤백: 커밋 revert로 즉시 롤백 가능 (표시 전용 문자열 변경).
- API/데이터 모델: 영향 없음 (trim_start/trim_end 제출 값은 숫자 그대로, 표시만 변경).
