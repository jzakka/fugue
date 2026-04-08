## Context

Fugue는 클라이언트 사이드에서 FFmpeg.wasm으로 비디오를 압축/트리밍하려 했으나, SharedArrayBuffer + COEP 요구사항으로 인해 Next.js RSC와 호환 문제가 발생했다. 서버에는 이미 ffmpeg/ffprobe가 설치되어 있다 (Dockerfile + 로컬 brew). 서버 사이드 트리밍으로 전환하면 브라우저 호환성 문제를 완전히 회피할 수 있다.

현재 핀 생성 폼은 multipart/form-data로 파일을 전송한다. 서버의 `pin/handler.go` Create 핸들러에서 파일을 임시 저장하고 ffprobe로 duration을 체크하는 인프라가 이미 갖추어져 있다.

## Goals / Non-Goals

**Goals:**
- 비디오 파일 선택 후 트리밍 구간 선택 모달 UI 제공
- 사용자가 시작/종료 시간을 선택하여 최대 15초 클립 생성
- 서버에서 ffmpeg로 선택된 구간을 트리밍하여 MP4 저장
- 클라이언트 사이드 FFmpeg.wasm 의존성 제거

**Non-Goals:**
- 클라이언트 사이드 비디오 인코딩 (FFmpeg.wasm 사용하지 않음)
- 비디오 필터/효과 적용 (트리밍만)
- 비디오 해상도/코덱 변경 UI (서버에서 자동 처리)

## Decisions

### 1. 서버 사이드 트리밍으로 전환

클라이언트는 원본 비디오 + 트리밍 구간(start, end)만 전송. 서버에서 ffmpeg로 처리.

**대안**: 클라이언트 FFmpeg.wasm → SharedArrayBuffer/COEP 문제로 Next.js와 비호환. 기각.
**대안**: MediaRecorder API → 실시간 재생 필요, 품질 손실. 기각.

### 2. 구간 선택 UI: 비디오 프리뷰 + 레인지 슬라이더

모달에 `<video>` 요소로 프리뷰를 보여주고, 듀얼 핸들 레인지 슬라이더로 시작/종료 시간을 선택한다. 슬라이더 드래그 시 비디오의 `currentTime`을 연동하여 해당 프레임을 프리뷰한다.

시작/종료 차이가 15초를 초과하면 슬라이더를 제한한다. 비디오가 15초 이하면 트리밍 모달을 건너뛰고 전체를 사용한다.

### 3. FormData에 trim_start, trim_end 추가

기존 multipart/form-data에 `trim_start`(초, float)와 `trim_end`(초, float) 필드를 추가한다. 서버에서 이 값이 있으면 ffmpeg로 트리밍, 없으면 전체 비디오를 사용한다 (단, 15초 초과 비디오는 trim 값 필수, 없으면 400 거부).

모드에 따라 ffmpeg 명령 형식이 다르다:

`-c copy` 모드 (빠름, 키프레임 경계 커팅): `ffmpeg -ss <start> -i input -t <duration> -c copy -movflags +faststart output.mp4`
재인코딩 모드 (프레임 정확): `ffmpeg -i input -ss <start> -t <duration> -c:v libx264 -crf 23 -preset fast -c:a aac -b:a 128k -movflags +faststart output.mp4`

`-c copy`에서는 `-ss`를 `-i` 앞에 배치하여 input-level seek (빠름). 재인코딩에서는 `-ss`를 `-i` 뒤에 배치하여 output-level seek (프레임 정확).

### 4. 15초 이하 비디오는 트리밍 모달 건너뛰기

`<video>` 요소의 `loadedmetadata`에서 duration을 확인하여, 15초 이하면 트리밍 모달 없이 바로 폼으로 진행한다. 사용자 경험을 불필요하게 복잡하게 만들지 않기 위함.

### 5. 클라이언트 FFmpeg.wasm 제거

COEP 관련 next.config.ts 헤더, ffmpeg-loader.ts, public/ffmpeg/ 파일, NavBar의 `<a>` 태그 (COEP용 full navigation) 등을 정리한다. 비디오 압축/트리밍을 서버에서 처리하므로 SharedArrayBuffer가 필요 없다.

### 6. 서버 트리밍 후 재인코딩이 필요한 경우

`-c copy`는 키프레임 경계에서만 정확하게 잘린다. 정확한 트리밍이 필요하면 재인코딩이 필요하지만, 서버의 네이티브 ffmpeg는 빠르므로 허용 가능하다. 사용자가 선택한 구간의 정확도를 위해, trim 결과의 duration이 요청한 것과 크게 다르면 `-c copy` 대신 재인코딩으로 fallback한다.

초기 구현은 `-c copy`로 시작한다. 트리밍 결과의 duration이 요청과 2초 이상 차이나거나(키프레임 부정확), 결과 파일이 서버 크기 제한(100MB)을 초과하면 재인코딩으로 fallback한다.

## Risks / Trade-offs

- **[Risk] 대용량 비디오 업로드 시간** → 원본 전체를 서버로 전송해야 함. 클라이언트 제한(500MB)과 서버 제한으로 방어. 향후 청크 업로드 고려 가능.
- **[Risk] `-c copy` 트리밍의 키프레임 부정확** → 시작점이 정확하지 않을 수 있음. 사용자 체감상 1-2초 오차. 정밀도가 필요하면 재인코딩 fallback 추가.
- **[Risk] 서버 부하** → ffmpeg 트리밍은 `-c copy` 기준 1초 미만. 동시 요청이 많으면 CPU 부하 증가 가능하나, 현재 규모에서는 문제 없음.
- **[Trade-off] 네트워크 비용 증가** → 100MB 원본을 서버로 보내고 15초 클립만 저장. 전송 시간은 늘지만 브라우저 호환성 문제를 완전히 해결.
