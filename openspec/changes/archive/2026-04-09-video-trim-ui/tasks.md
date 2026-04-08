## 1. 클라이언트 FFmpeg.wasm 의존성 제거

- [x] 1.1 `apps/web/src/lib/media/ffmpeg-loader.ts` 삭제
- [x] 1.2 `apps/web/src/lib/media/video.ts`에서 ffmpeg 관련 임포트/로직 제거, 단순 파일 반환으로 변경
- [x] 1.3 `apps/web/src/lib/media/index.ts`에서 `serverSideTrim`, `CompressVideoResult` 관련 로직 정리
- [x] 1.4 `apps/web/public/ffmpeg/` 디렉토리 삭제
- [x] 1.5 `apps/web/next.config.ts`에서 COEP/COOP 헤더 제거
- [x] 1.6 `apps/web/src/components/nav/NavBar.tsx`에서 `<a>` 태그를 `<Link>`로 복원
- [x] 1.7 `@ffmpeg/ffmpeg`, `@ffmpeg/core`, `@ffmpeg/util` 패키지 제거 (`npm uninstall`)

## 2. 비디오 트리밍 모달 컴포넌트

- [x] 2.1 `apps/web/src/components/pin/VideoTrimModal.tsx` 생성
- [x] 2.2 모달 내 `<video>` 요소로 비디오 프리뷰 표시
- [x] 2.3 듀얼 핸들 레인지 슬라이더 구현 (시작/종료 시간 선택)
- [x] 2.4 슬라이더 드래그 시 비디오 `currentTime` 연동 (프레임 프리뷰)
- [x] 2.5 선택 구간을 최대 15초로 제한하는 로직
- [x] 2.6 현재 선택된 구간 길이 표시 (예: "12.3초 / 15초")
- [x] 2.7 확인/취소 버튼 + 콜백 처리

## 3. PinCreateForm 통합

- [x] 3.1 비디오 파일 선택 시 duration 감지 (`<video>` loadedmetadata)
- [x] 3.2 15초 초과 비디오일 때 VideoTrimModal 표시
- [x] 3.3 15초 이하 비디오일 때 모달 건너뛰기
- [x] 3.4 모달 확인 시 trim_start/trim_end를 FormData에 포함
- [x] 3.5 모달 취소 시 파일 선택 초기화
- [x] 3.6 기존 클라이언트 사이드 최적화 로직 제거 (비디오 경로)

## 4. 서버 사이드 트리밍

- [x] 4.1 `pin/handler.go` Create 핸들러에서 `trim_start`, `trim_end` FormValue 파싱
- [x] 4.2 trim 값이 있으면 ffmpeg로 해당 구간 트리밍 — `-c copy` 모드: `ffmpeg -ss <start> -i input -t <duration> -c copy -movflags +faststart output.mp4` (input-level seek, 빠름), 재인코딩 모드: `ffmpeg -i input -ss <start> -t <duration> -c:v libx264 ...` (output-level seek, 정확)
- [x] 4.3 트리밍 후 크기가 100MB 초과하거나 duration이 요청과 2초 이상 차이나면 재인코딩 fallback (`-c:v libx264 -crf 23 -preset fast`)
- [x] 4.3.1 재인코딩 후에도 서버 크기 제한을 초과하면 에러 반환
- [x] 4.3.2 트리밍 및 재인코딩 과정에서 생성된 모든 임시 파일의 defer 정리 보장
- [x] 4.4 trim 값 유효성 검증 (start >= 0, start < end, end - start <= 15, end <= 원본 duration)
- [x] 4.5 trim 값 없이 15초 초과 비디오 업로드 시 400 거부 (서버 방어선)
- [x] 4.6 기존 자동 트리밍 로직 제거 (duration > 15s 자동 트림)

## 5. 테스트

- [x] 5.1 VideoTrimModal 컴포넌트 테스트 (15초 제한, 콜백)
- [x] 5.2 서버 트리밍 핸들러 테스트 (trim 파라미터 유효성, ffmpeg 호출)
- [x] 5.3 기존 핀 생성 테스트에서 자동 트리밍 관련 테스트를 trim_start/trim_end 기반으로 업데이트하고, 나머지 테스트가 정상 통과하는지 확인
