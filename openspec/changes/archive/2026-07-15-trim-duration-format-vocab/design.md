# Design: trim-duration-format-vocab

## Context

비디오 트리밍 플로우: 사용자가 `/pin/new`에서 15초 초과 비디오를 선택 → `VideoTrimModal`이 열려 구간을 선택 → `onConfirm(start, end)` → `PinCreateForm`이 `trimStart`/`trimEnd` state에 저장하고 파일 정보 행의 요약 칩에 표시.

- 모달 판독 어휘 (canonical): 위치 `fmt()` = `m:ss.s` (예 `0:05.0`), 길이 `N.N초`.

```tsx
// VideoTrimModal.tsx:15-18
function fmt(s: number): string {
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}:${sec.toFixed(1).padStart(4, "0")}`;
}
```

- 요약 칩 (outlier): `PinCreateForm.tsx:416-420`

```tsx
{trimStart != null && trimEnd != null && (
  <span className="text-text-dim font-mono">
    {trimStart.toFixed(1)}s ~ {trimEnd.toFixed(1)}s ({(trimEnd - trimStart).toFixed(1)}초)
  </span>
)}
```

## Goals / Non-Goals

**Goals**
- 요약 칩의 위치 표기를 모달과 동일한 `m:ss.s` 포맷으로 정합화.
- 한 줄 내 단위 혼용(`s` + `초`) 제거 — 위치는 무단위 `m:ss.s`, 길이는 `초`.

**Non-Goals**
- VideoTrimModal 변경 (canonical 표면 무변경).
- 공유 duration 포맷 모듈 신설 (cycle 3695: 확립 채널은 파일-로컬 추출).
- 요약 칩의 클래스/토큰/레이아웃 변경 (`text-text-dim font-mono` 유지).
- 서버 제출 값(`trim_start`/`trim_end`) 변경 없음 — 표시 전용.

## Decisions

### Decision 1: fmt 헬퍼를 PinCreateForm에 파일-로컬로 복제

VideoTrimModal:15-18과 동일한 4줄 함수를 `PinCreateForm.tsx` 모듈 레벨(기존 `formatSize` 헬퍼 인접)에 추가한다.

```tsx
function formatTime(s: number): string {
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}:${sec.toFixed(1).padStart(4, "0")}`;
}
```

이름은 `fmt` 대신 `formatTime`으로 한다 — PinCreateForm에는 이미 `formatSize`가 있어 `format*` 네이밍이 파일 내 확립 어휘다(VideoTrimModal의 `fmt`는 그 파일 유일 헬퍼라 축약형이 허용된 맥락).

**대안 기각**: (a) `VideoTrimModal`에서 `fmt` export — 모달 컴포넌트 파일이 유틸 export 소스가 되는 채널은 코드베이스에 0건, canonical 표면 수정 리스크. (b) `lib/` 공유 유틸 신설 — cycle 3695 판정대로 공유 모듈 채널 0건, 시각-무영향 리팩터링은 루프 범위 밖.

### Decision 2: 표기 문자열은 `{formatTime(trimStart)} ~ {formatTime(trimEnd)} ({(trimEnd - trimStart).toFixed(1)}초)`

- 위치 2개는 `m:ss.s`, 구분자 ` ~ ` 유지(기존 요약 칩 구분자), 길이 `(N.N초)` 유지(모달 :230 어휘와 일치).
- 결과 예: `0:05.0 ~ 0:12.3 (7.3초)` — 모달 판독 행이 보여준 값과 동일 표기.

## Risks / Trade-offs

- **리스크 최소**: 표시 전용 문자열 변경 1곳. 제출 payload·검증·모달 동작 불변.
- **Trade-off (수용)**: fmt 로직 4줄이 두 파일에 중복된다. cycle 3695 기록대로 코드베이스의 확립 채널이 파일-로컬 추출이므로 수용하고, 공유화는 루프 범위 밖으로 남긴다.

## Rollback

커밋 revert 1회. 표시 전용이라 데이터/마이그레이션 없음.
