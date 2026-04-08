## Why

현재 비디오 업로드 시 자동으로 앞에서부터 15초를 자르는 방식이지만, 사용자가 원하는 구간이 아닐 수 있다. 사용자가 직접 시작/종료 지점을 선택하여 최대 15초 클립을 만들 수 있는 트리밍 UI를 제공한다. 트리밍은 서버 사이드에서 ffmpeg로 수행하여 브라우저 호환성 문제(SharedArrayBuffer/COEP)를 회피한다.

## What Changes

- 비디오 파일 선택 후 트리밍 모달 UI 표시 (비디오 프리뷰 + 구간 선택 슬라이더)
- 사용자가 시작/종료 시간을 지정하여 최대 15초 구간 선택
- 선택된 구간 정보(start, end)를 서버에 전달
- 서버에서 ffmpeg로 해당 구간을 트리밍하여 MP4로 저장
- 클라이언트 사이드 FFmpeg.wasm 의존성 제거 (서버 사이드 처리로 전환)

## Capabilities

### Modified Capabilities
- `pin`: 비디오 업로드 파이프라인을 서버 사이드 트리밍으로 전환. 트리밍 구간 선택 모달 UI 추가. 기존 클라이언트 FFmpeg.wasm 의존성 제거. 서버 사이드 duration 검증을 트리밍 파라미터 기반으로 변경.

## Impact

- **Frontend**: `apps/web/src/app/pin/new/PinCreateForm.tsx` — 비디오 선택 시 트리밍 모달 표시
- **Frontend**: 새 컴포넌트 — 비디오 트리밍 모달 (프리뷰 + 구간 선택 슬라이더)
- **Backend**: `apps/api/internal/pin/handler.go` — trim_start/trim_end 파라미터 수신, ffmpeg 트리밍 처리
- **Backend**: 기존 클라이언트 사이드 FFmpeg.wasm 관련 코드 정리 (COEP 헤더, ffmpeg-loader 등)
