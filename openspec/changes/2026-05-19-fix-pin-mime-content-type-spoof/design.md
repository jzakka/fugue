# Design — fix-pin-mime-content-type-spoof

## Context

- `openspec/specs/pin/spec.md` L238-247의 Requirement `미디어 타입을 자동 감지한다`는 두 Scenario를 묶는다:
  - `MIME 타입 기반 자동 분류` — 실제 파일 형식 검증으로 미디어 타입을 결정.
  - `MIME 타입 위조 방지` — declared Content-Type과 실제 파일 내용 불일치 시 유효성 검사 오류.
- 첫 번째는 `apps/api/internal/storage/storage.go:127` `http.DetectContentType` + `allowedMIME` 조회로 enforce되어 있다.
- 두 번째는 현재 enforce되지 않는다. storage.Upload는 sniff된 mime만 allowlist에 통과시키고 declared `contentType` 인자와의 비교를 수행하지 않는다.

## Decision

### Decision 1: 검증 지점은 storage 레이어 단일 지점

대안: handler(pin.Create)에서 storage 호출 전 자체 sniff 후 비교.

채택: storage.Upload 내부에서 declared(인자)와 sniff(이미 수행 중) 비교.

이유:
- handler 측은 분기(`if strings.HasPrefix(contentType, "video/")`)로 declared를 이미 신뢰하고 있어서 분기에 도달하기 전에 거부해야 의미가 있다. 따라서 storage가 호출 시점에 거부해야 handler의 분기들이 자연스럽게 막힌다.
- 같은 storage.Upload가 다른 호출자(예: thumbnail 분기 `pin/handler.go:302-310`)에서도 사용된다. 단일 지점 보호로 모든 호출자 자동 보호.
- storage.Upload 시그니처 미변경 — 호출자는 이미 `contentType` 인자를 전달 중. 기존 octet-stream fallback(L131-134)이 declared를 이미 의식한 분기이므로 같은 함수에서 mismatch 검증 추가가 자연스럽다.

### Decision 2: declared가 빈 문자열이면 비교 skip

대안: declared 미표기 시 strict 거부.

채택: declared가 빈 문자열이면 비교 자체 미실행, sniff 결과로만 allowlist 검증(현재 동작 유지).

이유:
- `pin/spec.md` L246 "일치하지 않으면"이라는 표현은 두 값이 존재해야 비교 결과를 만든다. 한쪽이 없으면 "일치/불일치" 자체가 성립하지 않는다.
- multipart 클라이언트 중에는 Content-Type을 표기하지 않는 경우가 있다(예: 일부 SDK, curl raw without `;type=`). strict 거부는 정상 유저 회귀.
- 정상 경로(브라우저 multipart)는 Content-Type을 자동 표기하므로 비교가 항상 가동된다.

### Decision 3: 동의 MIME 정규화 매핑 6쌍

대안 A: detected를 그대로 비교 (정규화 없음).
대안 B: 모든 IANA registry alias를 광범위 정규화.

채택: 동의가 명확한 6쌍만 정규화 + lowercase.

이유:
- `http.DetectContentType`은 IANA의 canonical 표기를 반환하지만, 클라이언트(브라우저·SDK·curl)는 OS·환경에 따라 alias를 보낼 수 있다.
- 광범위 alias 매핑은 표준 외 표기까지 받게 되어 SHALL의 표면적이 모호해진다.
- 6쌍은 실측 가능한 흔한 변형(image/jpg ↔ image/jpeg, audio/x-wav ↔ audio/wav, audio/mp3 ↔ audio/mpeg 등). allowedMIME 자체가 10종이므로 정규화 입력 공간이 작아 명시 매핑이 가독성 우수.

### Decision 4: 에러 메시지에 "unsupported file type" 접두 보존

대안: 새로운 분기 에러 메시지(예: "content type mismatch").

채택: `fmt.Errorf("storage: unsupported file type: content type mismatch (declared=%q sniffed=%q)")`로 기존 "unsupported file type" 접두를 보존.

이유:
- `apps/api/internal/pin/handler.go:272-275` `if strings.Contains(err.Error(), "unsupported file type")` 분기가 이미 400 + "지원하지 않는 파일 형식입니다"로 응답을 매핑한다.
- 메시지 접두 보존으로 handler 변경 없이 응답 매핑 그대로 작동.
- 디버그용 declared·sniffed 값은 메시지 뒤에 괄호로 첨부 — 운영 로그에서 위조 시도 식별 가능.

## Why not — 대안 분석

### Why not handler에서 자체 sniff
- 같은 검증을 두 곳(handler·storage)에 두면 SSoT 위반. 한쪽 누락 시 silent 우회 재발생.
- handler의 분기는 declared를 기준으로 routing(`HasPrefix("video/")`)하므로 분기 전 결정 시점에는 sniff 결과가 없다. handler가 미리 sniff하려면 multipart Reader를 두 번 읽어야 해서 메모리 사용량/복잡성 증가.

### Why not storage.Upload 시그니처에 strict 모드 옵션 추가
- 호출자가 1곳뿐(pin/handler.go의 Create + thumbnail 분기). 옵션 분기는 호출자 증가 시 의미 — 현재 불필요.
- strict가 default여야 SHALL을 enforce. 옵션은 default 변경으로 충분.

## Edge cases

- **multipart part Content-Type이 ‘octet-stream’**: storage.go L131-134에서 sniff로 대체. 본 검증에선 declared가 octet-stream이면 sniff 결과를 신뢰하므로 비교 분기 진입 전. 신규 분기와 충돌 없음.
- **declared가 sniff와 같은 카테고리이지만 정확한 MIME이 다름** (예: declared=image/jpeg, sniff=image/png): 둘 다 image이지만 형식이 다르므로 거부. spec text "일치하지 않으면"의 strict 해석.
- **trim된 비디오의 contentType 재설정** (`pin/handler.go:230` `contentType = "video/mp4"`): 비디오 분기 내에서 trim 후 재기록되어 storage에 전달됨. 이 경우 contentType은 declared가 아닌 서버가 결정한 값이라 sniff와 일치(re-encode 결과는 mp4). 정상 통과.
- **thumbnail 분기** (`pin/handler.go:302-310`): 같은 storage.Upload 경유, 클라이언트가 표기한 thumbnail Content-Type과 sniff 비교. 위조 시 400 + thumbnail 실패. 현재 코드는 thumbnail 실패를 best-effort로 처리(log only)하므로 본 변경이 thumbnail UX에 영향을 주지 않음.

## Test plan

### 단위 테스트 (storage_test.go 신규)
- `TestUpload_RejectsMimeMismatch_WebMAsPNG` — WebM 바이트를 `contentType="image/png"`로 전달 → "unsupported file type" 에러.
- `TestUpload_RejectsMimeMismatch_PNGAsVideoMP4` — PNG 바이트를 `contentType="video/mp4"`로 전달 → 같은 에러.
- `TestUpload_RejectsMimeMismatch_WAVAsImageJPEG` — WAV 바이트를 `contentType="image/jpeg"`로 전달 → 같은 에러.
- `TestUpload_AllowsDeclaredEmpty` — declared 빈 문자열이면 sniff만으로 통과(현재 동작 유지).
- `TestUpload_AllowsAliasedDeclared` — `contentType="image/jpg"`(alias)와 sniff=image/jpeg → 통과(정규화).
- `TestUpload_AllowsAudioMP3Alias` — `contentType="audio/mp3"`(alias)와 sniff=audio/mpeg → 통과.

S3 PutObject mock은 기존 패턴 따름. 또는 sniff/검증 시점이 S3 호출 전이라 mismatch 케이스는 mock 없이도 검증 가능.

### 실 환경 QA (처리 모드 §7)
- `docker-compose up -d`, `apps/api` 기동, 로그인 후 JWT 확보.
- 정상 회귀: PNG → 201 image, MP3 → 201 audio, 10초 MP4 → 201 video.
- 위조 거부: `curl -F "media=@video.webm;type=image/png"` → 400 + "지원하지 않는 파일 형식입니다".
- 회귀 체크: `curl -F "media=@thumb.png"` 정상 thumbnail은 201, thumbnail에 mismatch 시도해도 핀 본문은 201 유지(best-effort 보존).
- DB: `SELECT media_type, COUNT(*) FROM pins ...` 위조로 생성된 잘못된 mediaType 행 0건.

## Migration / rollback

- 마이그레이션 없음 — 코드 변경만.
- 롤백: 분기 한 블록 제거 시 즉시 직전 동작 복귀. 응답 contract 동일.
- 운영 모니터링: 신규 분기 트리거 시 로그에 declared/sniffed 표시 — 위조 시도 빈도 추적 가능.
