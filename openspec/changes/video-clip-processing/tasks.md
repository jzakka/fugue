## 1. 클라이언트 비디오 duration 감지

- [ ] 1.1 `getVideoMeta()`에 duration 필드 추가 (`apps/web/src/lib/media/video.ts`) — `<video>` 요소의 `duration` 속성 반환
- [ ] 1.2 `VideoMeta` 인터페이스에 `duration: number` 필드 추가

## 2. 클라이언트 트리밍+압축 통합

- [ ] 2.1 `compressVideo()`에 duration 기반 트리밍 로직 추가 — 15초 초과 시 FFmpeg `-t 15` 옵션 포함
- [ ] 2.2 패스스루 조건에 duration ≤ 15초 조건 추가
- [ ] 2.3 최대 duration 상수 `MAX_VIDEO_DURATION_SECONDS = 15` 정의
- [ ] 2.4 트리밍 시 진행 상태 메시지 변경 ("비디오 트리밍 및 압축 중...")

## 3. 클라이언트 UI 트리밍 안내

- [ ] 3.1 `PinCreateForm.tsx`에서 트리밍 안내 표시 — "원본 XX초 → 15초로 트리밍됩니다"
- [ ] 3.2 최적화 결과에 트리밍 전/후 길이 정보 포함

## 4. 서버 사이드 duration 검증

- [ ] 4.1 서버에 ffprobe 실행 유틸리티 함수 작성 (`apps/api/internal/storage/` 또는 `internal/media/`)
- [ ] 4.2 핀 생성 핸들러에서 비디오 업로드 시 ffprobe로 duration 검증 — 15초 초과 시 422 반환
- [ ] 4.3 ffprobe 실패 시 graceful degradation (파일 크기 제한만 적용)

## 5. 인프라

- [ ] 5.1 Dockerfile에 ffmpeg/ffprobe 패키지 추가 (`apps/api/Dockerfile`)
- [ ] 5.2 docker-compose.yml에서 api 서비스에 ffprobe 사용 가능 확인

## 6. 테스트

- [ ] 6.1 `compressVideo()` 단위 테스트 — 15초 초과 비디오 트리밍 동작 검증
- [ ] 6.2 서버 duration 검증 핸들러 테스트 — 15초 초과 거부, 15초 이하 통과
- [ ] 6.3 패스스루 조건 테스트 — duration > 15초일 때 패스스루 비활성화 확인
