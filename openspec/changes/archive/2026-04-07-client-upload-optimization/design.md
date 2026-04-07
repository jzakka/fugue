## Context

현재 핀 생성 플로우에서 파일은 `PinCreateForm.tsx` → `createPin()` → Next.js rewrite proxy → Go API 순으로 전달된다. 클라이언트에서 어떤 검증도 없이 raw 파일이 전송되므로:

1. Next.js rewrite proxy의 body 크기 제한에 걸려 소켓 행업 발생
2. 대용량 파일이 네트워크를 타고 백엔드까지 도달한 뒤 거부됨
3. 고해상도 이미지/오디오/비디오가 원본 그대로 저장되어 스토리지 낭비

백엔드 검증(storage.go)은 이미 완성되어 있으므로, 프론트엔드에 검증 + 최적화 레이어를 추가하는 것이 이 변경의 범위다.

## Goals / Non-Goals

**Goals:**
- 파일 선택 시점에 타입/크기를 즉시 검증하여 불필요한 네트워크 요청 방지
- 이미지를 웹 표준 크기로 리사이즈 + 압축 (Pinterest 수준: 최대 2000px, 85% 품질)
- 오디오: 이미 웹 재생 가능한 압축 포맷(MP3/OGG)은 유지, 무손실(WAV/FLAC)은 압축 포맷으로 변환
- 비디오를 H.264 1080p로 재인코딩 (원본이 작으면 패스)
- 최적화 진행 상황을 사용자에게 표시

**Non-Goals:**
- 백엔드 업로드 로직 변경 (서버 검증은 그대로 유지, 단 OGG MIME 이슈 발견 시 최소한의 수정은 허용)
- 서버사이드 미디어 트랜스코딩
- CDN 이미지 리사이즈 파이프라인
- 청크 업로드/재시도 메커니즘

## Decisions

### 1. 이미지 압축: `browser-image-compression` 라이브러리 사용

Canvas API를 직접 쓸 수도 있지만, `browser-image-compression`이 EXIF orientation 처리, Web Worker 지원, maxWidthOrHeight/maxSizeMB 옵션을 내장하고 있어 안정적이다.

- **설정**: maxWidthOrHeight: 2000px, maxSizeMB: 5, initialQuality: 0.85, useWebWorker: true
- **출력 포맷**: 원본 포맷 유지 (JPEG→JPEG, PNG→PNG 등). GIF는 압축하지 않음 (애니메이션 손실)
- **GIF 크기 초과**: GIF는 압축 불가하므로, 최적화 후 서버 크기 제한(10MB) 검증에서 초과 시 에러를 표시한다.
- **대안 검토**: Canvas API 직접 사용 → EXIF rotation 처리가 번거롭고 애니메이션 GIF 불가

### 2. 오디오 정규화: 압축 포맷 패스스루 + 무손실 포맷만 변환

MP3/OGG는 이미 웹 재생 가능하고 압축되어 있으므로 변환 없이 패스스루한다. WAV/FLAC은 무압축/무손실이라 파일이 크므로, ffmpeg.wasm으로 OGG Vorbis 44.1kHz/16bit로 변환하여 크기를 줄인다.

- **패스스루 조건**: MP3/OGG 파일 → 원본 그대로 사용. 서버 크기 제한(50MB) 초과 시 클라이언트에서 즉시 에러 표시 (MP3/OGG는 압축 포맷이라 재인코딩으로 크기를 줄이기 어려움)
- **변환 대상**: WAV/FLAC → ffmpeg.wasm으로 OGG Vorbis 변환 (`-c:a libvorbis -ar 44100 -sample_fmt s16 -q:a 6`)
- **변환 후 크기 제한**: OGG 변환 결과가 50MB 초과 시 에러 (서버 제한과 동일)
- **OGG MIME 타입 주의**: ffmpeg.wasm 출력은 raw Uint8Array이므로 `new Blob([data], { type: 'audio/ogg' })`로 MIME 타입을 명시적으로 설정해야 한다. Go의 `http.DetectContentType`은 OGG magic bytes를 특별히 인식하지 않아 `application/octet-stream`을 반환할 가능성이 높다. 이 경우 `storage.go`의 폴백 로직(DetectContentType이 `application/octet-stream`이면 클라이언트 선언 Content-Type 사용)에 의해 클라이언트가 보낸 `audio/ogg`가 사용되므로 정상 동작이 기대된다. 다만 구현 시 실제 DetectContentType 반환값을 실측하여 확인한다.
- **설계 근거**: WAV→WAV 정규화는 파일 크기가 증가할 수 있어 "최적화"의 목적에 반한다. 이미 압축된 MP3/OGG를 디코딩→재인코딩하면 세대 손실이 발생하므로 패스스루가 최선이다.

### 3. 비디오 압축: `@ffmpeg/ffmpeg` (ffmpeg.wasm)

브라우저에서 비디오 트랜스코딩이 가능한 유일한 실용적 방법이다.

- **설정**: `-vf scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2` + `-c:v libx264 -crf 23 -preset fast -c:a aac -b:a 128k`
- **세로 영상 처리**: `force_original_aspect_ratio=decrease`가 1920x1080 박스 안에 원본 비율을 유지하며 축소한다. 세로 영상(1080x1920)은 1080x1080 박스에 9:16 비율로 피팅되어 608x1080이 된다. 즉 세로 영상의 높이가 1080px로 제한되며, 이는 의도된 동작이다 (1080p = 높이 1080 기준). `force_divisible_by=2`가 홀수 해상도를 방지한다.
- **패스스루 조건**: H.264 코덱 + MP4 컨테이너 + 1080p 이하 해상도 + 100MB 이하 파일 크기 → 재인코딩 없이 원본 사용. 이 4가지 조건을 모두 만족해야 패스스루. H.264 MP4이지만 100MB 초과 시 CRF 23으로 재인코딩하여 크기 축소 시도.
- **WebM 처리**: WebM(VP8/VP9)은 항상 H.264 MP4로 변환한다. WebM은 일부 구형 기기에서 재생 호환성이 낮으므로, 통일된 재생 경험을 위해 MP4로 표준화한다.
- **메타데이터 추출**: 패스스루 판별을 위해 비디오의 코덱과 해상도를 알아야 한다. `HTMLVideoElement`의 `videoWidth`/`videoHeight`로 해상도를 얻고, 코덱은 파일 확장자(MP4) + MIME 타입으로 1차 판별 후 ffmpeg.wasm의 probe 기능으로 정확한 코덱을 확인한다.
- **WASM 로드**: lazy load — 오디오(WAV/FLAC) 또는 비디오 파일 선택 시에만 로드 (~25MB)
- **진행률**: ffmpeg의 progress callback으로 실시간 표시
- **대안 검토**: MediaRecorder API → 재인코딩이 아닌 녹화 API라 용도가 다름

### 4. 최적화 파이프라인 구조

파일 선택 → 검증 → 최적화 → 미리보기 갱신 순서로 처리한다.

```
handleFileChange(file)
  → validateFile(file)          // 타입 + 크기 체크
  → optimizeMedia(file)         // 미디어 타입별 분기
    → compressImage(file)       // 이미지: browser-image-compression
    → normalizeAudio(file)      // 오디오: 패스스루 or ffmpeg.wasm → OGG
    → compressVideo(file)       // 비디오: ffmpeg.wasm → H.264
  → validateServerLimit(file)   // 최적화 후 서버 크기 제한 재검증
  → setFile(optimizedFile)      // 최적화된 파일로 교체
  → regeneratePreview()         // 기존 ObjectURL revoke + 새 미리보기 생성
```

각 최적화 함수는 `{ stage: string, progress: number }` 형태로 진행률을 콜백한다. `stage`는 UI에 표시될 상태 텍스트 ("이미지 압축 중...", "오디오 변환 중..." 등).

### 5. 유틸리티 파일 구조

```
apps/web/src/lib/media/
  ├── validation.ts      // 파일 타입/크기 검증
  ├── image.ts           // 이미지 압축
  ├── audio.ts           // 오디오 정규화
  ├── video.ts           // 비디오 압축
  ├── ffmpeg-loader.ts   // ffmpeg.wasm 공유 lazy 로드 (오디오 + 비디오에서 사용)
  └── index.ts           // 통합 파이프라인 (validateAndOptimize)
```

### 6. Next.js body size 설정

현재 프로젝트는 `next.config.ts`의 `rewrites()`로 `/api/*` 요청을 Go 백엔드로 프록시하고 있다. Next.js rewrite proxy에는 기본 body 크기 제한이 있어 대용량 파일 전송 시 소켓 행업이 발생한다. 구현 시 `node_modules/next/dist/docs/`를 참조하여 rewrite proxy의 body size 제한을 최소 110MB로 조정한다.

**주의**: `experimental.serverActions.bodySizeLimit`은 Server Actions 전용이며 rewrite에는 적용되지 않을 수 있다. 만약 rewrite proxy의 body size를 조정하는 공식 방법이 없으면, 대안으로 클라이언트에서 Go API 엔드포인트로 직접 요청하는 방식(`createPin`에서 rewrite 경유 대신 직접 `http://localhost:8080/api/pins`로 전송)을 검토한다.

### 7. COOP/COEP 헤더 전략: coi-serviceworker 패턴

ffmpeg.wasm은 `SharedArrayBuffer`를 필요로 하며, 이를 위해 COOP/COEP 헤더가 필요하다. 전체 사이트에 이 헤더를 적용하면 OAuth 플로우, OG 이미지 프리뷰, Google Fonts 등 외부 리소스 로딩이 깨질 수 있다.

- **전략**: `coi-serviceworker` 패턴을 사용하여 핀 생성 페이지(`/pin/new`)에서만 Service Worker를 통해 COOP/COEP 헤더를 주입한다. 이렇게 하면 다른 페이지에는 영향이 없다.
- **SW scope 관리**: Service Worker scope를 `/pin/new`로 한정. `/pin/:id` (핀 상세) 등 다른 `/pin/*` 경로에 영향이 없는지 검증 필요. coi-serviceworker 라이브러리의 scope 옵션으로 제어한다.
- **폴백**: Service Worker를 지원하지 않거나 SharedArrayBuffer를 사용할 수 없는 브라우저에서는 비디오/오디오(WAV/FLAC) 최적화를 건너뛰고 원본을 그대로 전송한다 (서버 크기 제한 내에서).
- **대안 검토**: 전체 사이트 헤더 적용 → OAuth, OG 프리뷰, 외부 리소스 전부 영향받아 부적절

## Risks / Trade-offs

- **[ffmpeg.wasm 번들 크기 ~25MB]** → lazy load로 오디오(WAV/FLAC) 또는 비디오 선택 시에만 로드. 이미지/MP3/OGG만 업로드하는 유저에겐 영향 없음
- **[브라우저 호환성]** → coi-serviceworker로 /pin/new에만 COOP/COEP 적용. SharedArrayBuffer 미지원 브라우저에서는 비디오/오디오 최적화를 건너뛰고 원본 전송 (폴백)
- **[GIF 미압축]** → 애니메이션 GIF는 Canvas로 압축 불가. 원본 그대로 전송하되 서버 크기 제한(10MB) 초과 시 업로드 불가하며 에러 표시
- **[비디오 변환 시간]** → 대용량 비디오의 클라이언트 트랜스코딩은 수 분 소요 가능. 진행률 UI로 사용자 불안 해소
- **[OGG MIME 감지]** → Go의 `http.DetectContentType`이 OGG를 인식 못해 `application/octet-stream`을 반환할 가능성이 높음. 이 경우 서버의 폴백 로직으로 클라이언트 선언 `audio/ogg`가 사용되어 정상 동작 기대. 구현 시 실측으로 확인

## Open Questions

- Next.js rewrite proxy의 body size 제한을 조정하는 공식 방법이 현재 버전에서 지원되는지 확인 필요
