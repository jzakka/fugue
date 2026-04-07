## Why

Fugue는 짧은 창작물 큐레이션 플랫폼이다. 현재 비디오 업로드 시 해상도 다운스케일과 코덱 변환은 수행하지만 **길이 제한이 없어** 수 분짜리 영상도 업로드 가능하다. 이는 스토리지 비용 증가, 피드 스크롤 UX 저하, 플랫폼의 "짧은 클립 큐레이션" 정체성과 맞지 않는 문제를 야기한다. 비디오를 최대 15초 클립으로 트리밍하고, 이에 맞게 압축 파이프라인을 강화한다.

## What Changes

- 비디오 업로드 시 **최대 15초** 길이 제한 적용 (클라이언트에서 자동 트리밍)
- 15초 초과 비디오는 앞에서부터 15초까지만 자동 클리핑
- 비디오 메타데이터에서 **duration**을 감지하여 트리밍 필요 여부 판단
- 트리밍 후 압축 파이프라인 적용 (기존 H.264/1080p 압축 유지)
- 서버 사이드에서도 duration 15초 초과 비디오를 거부하는 방어선 추가
- 트리밍 시 사용자에게 진행 상태 및 원본 길이 → 트리밍 결과를 안내하는 UI 제공

## Capabilities

### New Capabilities
- `video-clip-trimming`: 비디오 길이를 최대 15초로 트리밍하는 클라이언트/서버 파이프라인

### Modified Capabilities
- `pin`: 비디오 업로드 requirement에 duration 제한 시나리오 추가, 서버 검증 강화

## Impact

- **Frontend**: `apps/web/src/lib/media/video.ts` — FFmpeg.wasm 트리밍 로직 추가, duration 감지
- **Frontend**: `apps/web/src/lib/media/validation.ts` — duration 검증 추가
- **Frontend**: `apps/web/src/app/pin/new/PinCreateForm.tsx` — 트리밍 안내 UI
- **Backend**: `apps/api/internal/storage/storage.go` 또는 pin handler — 서버 사이드 duration 검증
- **Backend**: `apps/api/internal/bot/downloader.go` — 봇 다운로드 시에도 duration 제한 적용 고려
