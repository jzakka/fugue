# fix-pin-mime-content-type-spoof

## Problem

`openspec/specs/pin/spec.md` L245-247 Scenario `MIME 타입 위조 방지`:

> WHEN Content-Type과 실제 파일 내용이 일치하지 않으면
> THEN 유효성 검사 오류가 반환된다

`apps/api/internal/storage/storage.go:120-139` `Upload`는 첫 512바이트로 `http.DetectContentType`만 호출하고, 호출자가 전달한 declared `contentType`과 sniff된 detected MIME의 불일치를 비교/거부하는 분기가 없다. 결과적으로 위 SHALL이 production에서 enforce되지 않는다.

핵심 우회 시나리오 (`pin/spec.md` L201-203 `구간 정보 없이 15초 초과 비디오 업로드` 거부 SHALL까지 함께 침묵 우회):

1. 인증된 유저가 16초짜리 WebM을 multipart로 업로드, media 파트 헤더에 `Content-Type: image/png` 위조 표기.
2. `apps/api/internal/pin/handler.go:148` `if strings.HasPrefix(contentType, "video/")` false → 비디오 처리 분기를 통째로 건너뜀(probe·trim·15초 거부 가드 미실행).
3. `storage.go:127` `http.DetectContentType` → `video/webm` sniff. allowlist 통과, `media_type=video`로 저장.
4. 결과 201 + 16초 비디오 핀이 트림 없이 저장됨. `MIME 타입 위조 방지`와 `구간 정보 없이 15초 초과 비디오 업로드` 두 SHALL 동시 침묵 우회.

대칭 케이스(WAV-as-image/jpeg)도 동일하게 가능 — `pin/spec.md` L99 `WAV/FLAC에서 압축 포맷으로 변환` SHALL 우회.

## Solution

`storage.Upload` 내부에서 declared `contentType`(호출자 인자)과 sniff된 detected를 비교한다. 둘 다 비어 있지 않고 정규화 후 불일치하면 `unsupported file type: content type mismatch (...)` 에러 반환. handler 측은 storage 에러 메시지에 이미 `"unsupported file type"` 분기(`pin/handler.go:272-275`)가 있어 400 + "지원하지 않는 파일 형식입니다"로 응답된다.

declared가 빈 문자열인 경우(curl raw multipart에서 Content-Type 헤더 미표기)는 비교 자체 미실행 — `pin/spec.md` L246 "일치하지 않으면"이라는 표현은 "표기 없음"과 분리된다.

정규화 매핑(false positive 회피):
- `image/jpg` ↔ `image/jpeg`
- `image/pjpeg` ↔ `image/jpeg`
- `audio/x-wav` ↔ `audio/wav`
- `audio/wave` ↔ `audio/wav`
- `audio/mp3` ↔ `audio/mpeg`
- `audio/x-flac` ↔ `audio/flac`

그 외에는 단순 lowercase 후 strict equality.

## Why

같은 핸들러 파일의 다른 SHALL(`구간 정보 없이 15초 초과 비디오 업로드` 거부, `WAV/FLAC에서 압축 포맷으로 변환`)이 declared Content-Type을 신뢰해 분기하는 코드 경로로 침묵 우회되는 결함의 공통 뿌리는 storage 레이어에 declared vs sniff 비교가 없다는 점이다. handler에서 분기마다 보호를 덧대는 것보다 storage 레이어 단일 지점에서 한 번 거부하는 게 SSoT.

## Scope

- `apps/api/internal/storage/storage.go` `Upload` 내부에 normalization + mismatch 거부 분기 1개 추가.
- `apps/api/internal/storage/storage_test.go` 신규 — 정상 케이스(declared == sniff) / 위조 케이스(WebM-as-PNG · PNG-as-MP4 · WAV-as-JPEG) / declared 빈 문자열 skip / normalize 동의어 일치 5~6개 케이스.
- `openspec/specs/pin/spec.md` 신규 Requirement: `MIME 타입 위조 방지는 storage 레이어에서 declared와 sniff의 불일치 거부로 enforce된다` (기존 Scenario는 변경 없음, wiring 계약).

## Out of scope

- handler 측 추가 분기(예: pin.Create에서 storage 호출 전 자체 sniff). 단일 지점 보호로 충분.
- 다른 미디어 도메인의 위조 방지(예: thumbnail 분기는 이미 같은 storage.Upload 경유라 자동 보호됨, 별도 변경 없음).
- spec L246 "일치하지 않으면"의 declared 빈 문자열 해석은 본 change에서 "비교 자체 미실행"으로 채택(strict 해석은 별개 결정).

## Rollback

`apps/api/internal/storage/storage.go`의 분기 한 블록 제거로 즉시 되돌릴 수 있다. 응답 contract는 동일(400 + "지원하지 않는 파일 형식입니다") — 정상 케이스에 영향 없음.

## QA plan

처리 모드 §7 실 환경 QA — `apps/api` 띄운 후 curl `--form 'media=@file;type=<mime>'`로 declared MIME 위조 강제:

1. 정상 회귀: PNG/MP3/MP4 정상 Content-Type 업로드 → 201.
2. WebM-as-PNG 위조 → 400 + "지원하지 않는 파일 형식입니다" (no-trim 16초 비디오 우회 차단).
3. PNG-as-video/mp4 위조 → 400 (현재는 우연히 probe fail-closed로 400 반환되지만, mismatch 분기에서 더 빨리 거부됨).
4. WAV-as-image/jpeg 위조 → 400 (audio 변환 SHALL 우회 차단).
5. DB 확인: `pins` 테이블에 위조 시도로 인한 잘못된 `media_type` 행이 신규 생성되지 않음.
