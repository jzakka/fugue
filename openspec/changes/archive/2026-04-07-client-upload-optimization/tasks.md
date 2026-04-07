## 1. 프로젝트 셋업

- [x] 1.1 의존성 추가: `browser-image-compression`, `@ffmpeg/ffmpeg`, `@ffmpeg/util`, `coi-serviceworker`
- [x] 1.2 미디어 유틸리티 디렉토리 생성: `apps/web/src/lib/media/`

## 2. 파일 검증 모듈

- [x] 2.1 `validation.ts` 구현: MIME 타입 허용 목록 검증 함수
- [x] 2.2 `validation.ts` 구현: 미디어 타입별 원본 크기 제한 검증 (이미지 20MB, 오디오 100MB, 비디오 500MB)
- [x] 2.3 `validation.ts` 구현: 최적화 후 서버 크기 제한 검증 (이미지 10MB, 오디오 50MB, 비디오 100MB — 서버와 동일한 비교 연산자 `>` limit 사용)
- [x] 2.4 `validation.ts` 단위 테스트: MIME 타입 검증, 크기 제한 검증의 경계값 테스트

## 3. 이미지 압축 모듈

- [x] 3.1 `image.ts` 구현: `browser-image-compression`으로 리사이즈/압축 (maxWidthOrHeight 2000px, maxSizeMB 5, initialQuality 0.85)
- [x] 3.2 `image.ts` 구현: GIF 파일은 압축 스킵 (원본 반환)
- [x] 3.3 `image.ts` 구현: 이미 작은 이미지 (2000px 이하 + 5MB 이하) 패스스루

## 4. 오디오 정규화 모듈

- [x] 4.1 `audio.ts` 구현: MP3/OGG 파일 패스스루 (서버 크기 제한 50MB 초과 시 즉시 에러)
- [x] 4.2 `audio.ts` 구현: WAV/FLAC 파일을 ffmpeg.wasm으로 OGG Vorbis 44.1kHz/16bit 변환
- [x] 4.3 `audio.ts` 구현: ffmpeg.wasm 출력 Blob에 `audio/ogg` MIME 타입 명시적 설정
- [x] 4.4 `audio.ts` 구현: 변환 결과 50MB 초과 시 에러 처리
- [x] 4.5 `audio.ts` 구현: ffmpeg.wasm 미지원 브라우저에서는 원본 패스스루 (폴백)
- [x] 4.6 서버 OGG MIME 감지 실측: Go `http.DetectContentType`이 OGG에 대해 실제로 반환하는 값 확인 (`application/octet-stream` 예상 → 폴백 로직으로 `audio/ogg` 사용). 예상과 다를 경우 백엔드 대응

## 5. 비디오 압축 모듈

- [x] 5.0 `video.ts` 구현: 비디오 메타데이터 추출 (HTMLVideoElement로 해상도, ffmpeg probe로 코덱 판별)
- [x] 5.1 `ffmpeg-loader.ts` 구현: ffmpeg.wasm 공유 lazy 로드 싱글턴 (audio.ts와 video.ts에서 import하여 사용)
- [x] 5.2 `video.ts` 구현: H.264 1080p 재인코딩 (scale 필터 + force_divisible_by=2 + CRF 23 + AAC 128k)
- [x] 5.3 `video.ts` 구현: 패스스루 조건 (H.264 코덱 + MP4 컨테이너 + 1080p 이하 + 100MB 이하)
- [x] 5.4 `video.ts` 구현: H.264 MP4이지만 100MB 초과 시 CRF 23으로 재인코딩
- [x] 5.5 `video.ts` 구현: WebM(VP8/VP9)은 항상 H.264 MP4로 변환
- [x] 5.6 `video.ts` 구현: ffmpeg progress 콜백으로 진행률 반환
- [x] 5.7 `video.ts` 구현: 변환 결과 100MB 초과 시 에러 처리
- [x] 5.8 `video.ts` 구현: ffmpeg.wasm 미지원 브라우저에서는 원본 패스스루 (폴백)

## 6. 통합 파이프라인

- [x] 6.1 `index.ts` 구현: `validateAndOptimize(file, onProgress)` 통합 함수
- [x] 6.2 진행률 콜백 타입 정의: `{ stage: string, progress: number }`
- [x] 6.3 최적화 후 서버 크기 제한 재검증 로직

## 7. PinCreateForm 통합

- [x] 7.1 `handleFileChange`에 검증 + 최적화 파이프라인 연결
- [x] 7.2 최적화 진행 상태 UI: 프로그레스 바 + 상태 텍스트 + 원본/최적화 크기 비교
- [x] 7.3 최적화 중 등록 버튼 비활성화
- [x] 7.4 최적화 에러 시 에러 메시지 표시 및 파일 초기화
- [x] 7.5 최적화 후 미리보기 URL 재생성 (기존 ObjectURL revoke + 새 파일로 미리보기 생성)
- [x] 7.6 파일 업로드 영역 안내 텍스트 갱신 (자동 최적화 안내 포함)
- [x] 7.7 파일 교체/최적화 시 이전 ObjectURL이 올바르게 revoke되는지 확인

## 8. Next.js 및 브라우저 설정

- [x] 8.1 Next.js rewrite proxy의 body size 제한 조정 (`node_modules/next/dist/docs/` 참조, 불가 시 직접 API 호출 방식으로 전환)
- [x] 8.2 coi-serviceworker 설정: `/pin/new` 페이지에서만 Service Worker로 COOP/COEP 헤더 주입 (scope를 `/pin/new`로 한정)
- [x] 8.3 coi-serviceworker 적용 후 `/pin/new` 페이지에서 OG 이미지 프리뷰 로드 검증
- [x] 8.4 `/pin/:id` (핀 상세) 등 다른 `/pin/*` 경로에서 COOP/COEP 영향 없음 검증
- [x] 8.5 다른 페이지 (로그인/OAuth, 메인, 프로필 등)에서 기존 기능 영향 없음 검증
