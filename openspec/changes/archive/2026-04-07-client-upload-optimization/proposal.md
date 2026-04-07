## Why

현재 핀 생성 시 파일이 클라이언트 검증 없이 바로 API 서버로 전송된다. Next.js 미들웨어의 기본 body 크기 제한(10MB)에 걸리면 소켓이 끊기고, 이를 넘기더라도 대용량 파일이 백엔드까지 도달한 후에야 거부되어 사용자 경험이 매우 나쁘다. 클라이언트에서 파일 검증과 압축/리사이즈를 수행하여 업로드 실패를 사전에 방지하고 전송 시간을 단축해야 한다.

## What Changes

- 클라이언트 파일 크기/타입 즉시 검증 추가 (서버 왕복 없이 에러 피드백)
- 이미지 클라이언트 압축: 최대 2000px 리사이즈 + 품질 85% 압축 (원본 포맷 유지)
- 오디오 클라이언트 정규화: 이미 웹 재생 가능한 MP3/OGG는 패스스루, WAV/FLAC(무압축/무손실)은 ffmpeg.wasm으로 OGG Vorbis 44.1kHz로 압축
- 비디오 클라이언트 압축: ffmpeg.wasm으로 H.264 1080p 재인코딩 (원본이 1080p 이하면 패스)
- 압축/변환 진행률 UI 표시 (프로그레스 바 + 상태 텍스트)

## Capabilities

### New Capabilities

(없음)

### Modified Capabilities
- `pin`: 핀 생성 플로우에 클라이언트 파일 검증, 미디어 최적화(리사이즈/압축/트랜스코딩), 진행률 표시 단계를 추가 (기존 서버 검증은 유지)

## Impact

- **프론트엔드**: `PinCreateForm.tsx` 파일 선택 핸들러에 검증/최적화 파이프라인 추가
- **의존성 추가**: `browser-image-compression` (이미지), `@ffmpeg/ffmpeg` + `@ffmpeg/util` (오디오 정규화 + 비디오 압축)
- **번들 크기**: ffmpeg.wasm WASM 바이너리 ~25MB (lazy load, 오디오(WAV/FLAC) 또는 비디오 선택 시에만 로드)
- **백엔드**: 변경 없음 (기존 서버 검증 그대로 유지, 이중 검증 체계)
- **Next.js 설정**: `next.config.ts`에 `experimental.serverActions.bodySizeLimit` 또는 middleware body size 설정 조정 검토
