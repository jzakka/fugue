## 1. 클라이언트 비디오 duration 감지

- [x] 1.1 `getVideoMeta()`에 duration 필드 추가 (`apps/web/src/lib/media/video.ts`) — `<video>` 요소의 `duration` 속성 반환
- [x] 1.2 `VideoMeta` 인터페이스에 `duration: number` 필드 추가

## 2. 클라이언트 트리밍+압축 통합

- [x] 2.1 `compressVideo()`에 duration 기반 트리밍 로직 추가 — `getVideoMeta()`에서 받은 duration이 15초 초과 시 ffmpeg.exec 인자에 `-t 15`를 조건부 삽입, 15초 이하 시 기존 인자 유지
- [x] 2.2 패스스루 조건에 duration ≤ 15초 조건 추가
- [x] 2.3 최대 duration 상수 `MAX_VIDEO_DURATION_SECONDS = 15` 정의
- [x] 2.4 트리밍 시 진행 상태 메시지 변경 ("비디오 트리밍 및 압축 중...")

## 3. 클라이언트 UI 트리밍 안내

- [x] 3.1 `PinCreateForm.tsx`에서 트리밍 안내 표시 — 원본 길이와 트리밍 대상 길이 안내
- [x] 3.2 `compressVideo()` 반환 타입을 `{ file: File, originalDuration?: number, trimmedDuration?: number }` 형태로 변경하고, `validateAndOptimize()`에서 이를 수용하여 `OptimizeResult` 인터페이스에 `originalDuration`, `trimmedDuration` 필드를 매핑. `PinCreateForm.tsx`에서 트리밍 안내에 활용

## 4. 서버 사이드 duration 검증

- [x] 4.0 `handler.go`에 `maxVideoDurationSeconds = 15` 상수 정의 (클라이언트 `MAX_VIDEO_DURATION_SECONDS`와 동일 값, 향후 조정 가능)
- [x] 4.1 `apps/api/internal/pin/handler.go`의 Create 핸들러에서 비디오 파일을 임시 저장 후 ffprobe로 duration 추출하는 로직 추가
- [x] 4.2 duration 15초 초과 시 400 Bad Request 반환 ("비디오는 최대 15초까지 업로드 가능합니다")
- [x] 4.3 ffprobe 실패 시 graceful degradation (파일 크기 제한만 적용, duration 검증 건너뜀)
- [x] 4.4 비디오가 아닌 파일(image/audio)은 duration 검증 건너뛰기

## 5. 인프라

- [x] 5.1 API 서버용 Dockerfile 생성 시 ffmpeg 패키지 포함 (현재 `apps/api/Dockerfile` 미존재, 신규 생성 필요)
- [x] 5.2 로컬 개발 환경에 ffprobe 설치 확인 (macOS: `brew install ffmpeg`) 및 README/CLAUDE.md에 의존성 안내 추가

## 6. 테스트

- [x] 6.1 `compressVideo()` 단위 테스트 — 15초 초과 비디오 트리밍 동작 검증
- [x] 6.2 서버 duration 검증 테스트 — 15초 초과 거부 (400), 15초 이하 통과
- [x] 6.3 패스스루 조건 테스트 — duration > 15초일 때 패스스루 비활성화 확인
- [x] 6.4 기존 `pin/handler_test.go` 핀 생성 테스트가 duration 검증 추가 후에도 정상 통과하는지 확인
