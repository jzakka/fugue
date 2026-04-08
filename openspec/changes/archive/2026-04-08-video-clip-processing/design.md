## Context

Fugue는 클라이언트 사이드에서 FFmpeg.wasm을 사용하여 비디오를 H.264/1080p로 압축한다. 현재 파이프라인은 해상도 다운스케일과 코덱 변환만 수행하며, 길이(duration) 제한이 없다. 서버는 100MB 파일 크기 제한만 적용한다.

비디오 duration 감지는 브라우저의 `<video>` 요소 `loadedmetadata` 이벤트로 이미 가능하고, FFmpeg.wasm의 `-t` 플래그로 트리밍이 가능하다. 서버 사이드에서는 Go의 ffprobe 바이너리 또는 멀티파트 수신 시 간단한 MP4 메타데이터 파싱으로 duration을 검증할 수 있다.

## Goals / Non-Goals

**Goals:**
- 비디오를 최대 15초로 자동 트리밍 (앞에서부터 15초)
- 트리밍 + 압축을 하나의 FFmpeg 패스로 처리 (이중 인코딩 방지)
- 서버에서 duration 초과 비디오를 거부하는 방어선 추가
- 사용자에게 트리밍이 발생했음을 명확히 안내

**Non-Goals:**
- 사용자가 시작/종료 지점을 선택하는 구간 선택 UI (향후 확장)
- 서버 사이드 트랜스코딩 (클라이언트 사이드 FFmpeg.wasm 유지)
- 봇(FugueBot) 다운로드 시 서버 사이드 트리밍 (봇은 이미 짧은 클립 위주 소스를 크롤링하므로 이번 변경에서 수정하지 않음)

## Decisions

### 1. 트리밍과 압축을 단일 FFmpeg 패스로 처리

FFmpeg의 `-t 15` 옵션을 기존 압축 명령에 추가하여 한 번의 인코딩으로 트리밍+압축을 동시 수행한다.

**대안**: 트리밍 후 별도 압축 → 이중 인코딩으로 품질 저하 및 처리 시간 증가. 기각.

```
ffmpeg -i input -t 15 -vf scale=... -c:v libx264 -crf 23 -preset fast -c:a aac -b:a 128k -movflags +faststart output.mp4
```

### 2. 클라이언트에서 duration 감지 후 트리밍 여부 결정

`<video>` 요소의 `duration` 속성으로 원본 길이를 감지한다. 이미 `getVideoMeta()` 내부의 `getVideoDimensions()`에서 `loadedmetadata`를 사용하고 있으므로, `getVideoDimensions()`에서 `video.duration`도 함께 반환하고, `getVideoMeta()`에서 이를 `VideoMeta`에 포함시키면 된다.

- duration ≤ 15초: 기존 압축 로직 유지 (트리밍 불필요)
- duration > 15초: `-t 15` 추가하여 트리밍+압축

### 3. 패스스루 조건에 duration 제한 추가

현재 패스스루 조건: H.264 MP4 + 1080p 이하 + 100MB 이하.
변경: H.264 MP4 + 1080p 이하 + 100MB 이하 + **15초 이하**.

15초 초과 H.264 MP4는 패스스루하지 않고 트리밍 후 재인코딩한다.

### 4. 서버 사이드 duration 검증: pin handler에서 ffprobe 사용

서버에서 업로드된 비디오의 duration을 검증하는 방어선을 추가한다. 검증 위치는 `pin/handler.go`의 Create 핸들러에서 `storage.Upload()` 호출 전에 수행한다. 이유: 현재 `storage.Upload()`는 `io.Reader`를 받아 내부에서 전체를 메모리로 읽어 S3에 업로드하는 구조이다. storage 패키지에 미디어 검증 책임을 추가하면 패키지가 비대해지므로, handler 레벨에서 처리한다. 비디오가 아닌 파일은 이 검증을 건너뛴다.

파일 접근 전략: `r.FormFile("media")`로 받은 `multipart.File`을 임시 파일로 복사한 뒤 ffprobe를 실행한다. 임시 파일은 생성 직후 `defer`로 삭제를 예약하여, 정상/에러 모든 경로에서 반드시 정리된다. 검증 통과 시 임시 파일을 `os.Open`하여 `storage.Upload()`에 body로 전달한다 (filename, contentType, size는 원본 `multipart.FileHeader`에서 가져옴). 이 방식은 multipart.File의 Seek 가능 여부에 의존하지 않으며 확실하게 동작한다.

Go에서 `ffprobe`를 exec하여 duration을 추출한다.

**대안**: MP4 moov atom 직접 파싱 → 구현 복잡도 높고 WebM 미지원. 기각.
**대안**: duration 검증 생략 → 클라이언트 우회 시 긴 비디오 저장 가능. 기각.
**대안**: `storage.Upload()` 내부에서 검증 → storage 패키지에 미디어 분석 책임이 추가되어 비대해짐. 기각.

ffprobe 명령:
```
ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 input.mp4
```

서버에 ffprobe 바이너리가 필요하다. Docker 이미지에 ffmpeg 패키지를 추가한다.

### 5. 에러 코드: 기존 패턴과 일관성 유지 (400)

기존 pin handler의 파일 크기 초과, 파일 타입 오류 등은 모두 `400 Bad Request`로 응답한다. duration 초과 검증도 동일하게 400을 사용하여 일관성을 유지한다.

### 6. 최대 길이를 15초로 설정

플랫폼 성격상 10초는 너무 짧고(음악 클립 표현 부족), 15초가 TikTok/Reels 초기 포맷과 유사하여 적절하다. 상수로 정의하여 향후 조정 가능하게 한다.

## Risks / Trade-offs

- **[Risk] FFmpeg.wasm 트리밍이 키프레임 경계와 맞지 않아 시작/끝이 깨질 수 있음** → `-t 15`는 re-encode 시 정확한 타임스탬프 커팅을 보장한다. copy 모드가 아니므로 문제없다.
- **[Risk] 서버에 ffprobe 의존성 추가** → Docker 이미지 크기 증가. Alpine 기준 ffmpeg 패키지 ~30MB. 허용 범위.
- **[Risk] 15초 초과 비디오를 무조건 앞에서 자르면 사용자가 원하는 구간이 아닐 수 있음** → Non-Goal로 명시. 향후 구간 선택 UI 확장 가능. 현재는 "처음 15초" 규칙으로 단순화.
- **[Trade-off] 패스스루 비율 감소** → 15초 초과 비디오는 재인코딩 필수. 하지만 트리밍된 15초 클립은 파일 크기가 작아 압축 시간도 짧다.
