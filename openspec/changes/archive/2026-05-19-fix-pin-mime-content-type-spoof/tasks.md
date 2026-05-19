## 1. storage 레이어 declared vs sniff 비교 추가

- [x] 1.1 `apps/api/internal/storage/storage.go`의 `Upload` 함수 외부에 MIME alias 정규화 헬퍼 `normalizeMIME` + `mimeAliases` map을 추가한다. 매핑 6쌍 + lowercase: `image/jpg` → `image/jpeg`, `image/pjpeg` → `image/jpeg`, `audio/x-wav` → `audio/wav`, `audio/wave` → `audio/wav`, `audio/mp3` → `audio/mpeg`, `audio/x-flac` → `audio/flac`.
- [x] 1.2 `Upload` 함수에서 `http.DetectContentType` 호출 직후, declared `contentType`이 빈 문자열이 아니면 `normalizeMIME(contentType)`과 `normalizeMIME(detected)`를 비교한다. 두 값이 다르면 외부 저장소 쓰기 전에 `fmt.Errorf("storage: unsupported file type: content type mismatch (declared=%q sniffed=%q)", contentType, detected)`를 반환한다.
- [x] 1.3 기존 `octet-stream` fallback(declared로 대체) 분기와의 순서를 점검한다. declared가 `application/octet-stream`이면 비교를 skip(빈 문자열과 동치 취급)하여 spec의 "declared 빈 문자열" 분기와 일관성을 유지한다.
- [x] 1.4 신규 분기 바로 위에 "spec: pin `MIME 타입 위조 방지는 storage 레이어에서 declared와 sniff의 불일치 거부로 enforce된다`" 한 줄 주석을 남겨 회귀 시 의도가 보이도록 한다.
- [x] 1.5 (QA에서 발견) allowlist 조회 직전에 sniff된 `detected`도 `normalizeMIME`으로 canonical 변환. `http.DetectContentType`이 WAV에 대해 `audio/wave`를 반환하므로 정규화 없이는 `allowedMIME["audio/wave"]` 미스로 정상 WAV 업로드가 거부되는 회귀(QA Test 5 실패)를 막는다. 비교 측과 lookup 측을 대칭 정규화하여 일관성 확보.

## 2. storage 단위 테스트

- [x] 2.1 `apps/api/internal/storage/storage_test.go`(신규)를 생성한다. 외부 의존(S3 PutObject)이 없는 mismatch 거부 케이스는 검증 시점이 S3 호출 전이므로 mock 없이 검증한다. accept 경로는 nil s3 client에서 PutObject가 panic을 일으키는 점을 `recover`로 잡아 "mismatch 분기에서 reject되지 않고 S3까지 도달함"을 양성 신호로 검증한다.
- [x] 2.2 `TestUpload_RejectsMimeMismatch_WebMAsPNG`: WebM 매직 바이트(EBML 헤더)를 `contentType="image/png"`로 전달 → 에러 메시지에 `"unsupported file type"` + `"content type mismatch"` 포함.
- [x] 2.3 `TestUpload_RejectsMimeMismatch_PNGAsVideoMP4`: PNG 매직 바이트(`89 50 4E 47 0D 0A 1A 0A`)를 `contentType="video/mp4"`로 전달 → 같은 에러.
- [x] 2.4 `TestUpload_RejectsMimeMismatch_WAVAsImageJPEG`: WAV 매직 바이트(`RIFF....WAVE`)를 `contentType="image/jpeg"`로 전달 → 같은 에러.
- [x] 2.5 `TestUpload_RejectsMimeMismatch_SameCategoryDifferentFormat`: PNG 바이트를 `contentType="image/jpeg"`로 전달 → 같은 카테고리이지만 정확한 MIME이 다르므로 거부.
- [x] 2.6 `TestUpload_AllowsDeclaredEmpty`, `TestUpload_AllowsDeclaredOctetStream`: 비교 skip 확인.
- [x] 2.7 `TestUpload_AllowsAliasedDeclared_ImageJPG`, `_ImagePJPEG`, `_AudioMP3`, `_AudioXWav` + `TestUpload_AllowsExactMatch_PNG`: 정규화 후 일치 → 비교 분기 통과 확인.
- [x] 2.8 `TestNormalizeMIME`: 6쌍 alias + 대소문자 + 공백 trim 검증.

## 3. 검증

- [x] 3.1 `cd apps/api && go vet ./internal/storage/... ./internal/pin/...` 통과.
- [x] 3.2 `cd apps/api && go build ./...` 통과.
- [x] 3.3 `cd apps/api && go test ./internal/storage/...` 통과(신규 12개 테스트 함수).
- [x] 3.4 `cd apps/api && go test ./...` 전체 통과 (483 tests, 22 packages — 회귀 없음).
- [x] 3.5 `openspec validate 2026-05-19-fix-pin-mime-content-type-spoof --strict` 통과.

## 4. 실 환경 QA (처리 모드 §7)

- [x] 4.1 worktree와 동일 포트의 부모 worktree(`peppy-juggling-token-*`) postgres/redis/minio 인프라 재사용, `apps/api` 기동, qatest creator(`11111111-...`) JWT 발급 후 8건 curl 시나리오 실행.
- [x] 4.2 정상 회귀: 실제 PNG → 201 + `media_type=image` (Test 1), 실제 WAV → 201 + `media_type=audio` (Tests 5, 6 — `audio/wav` 및 alias `audio/x-wav`), 실제 WebM trim → 201 + `media_type=video` (Test 8).
- [x] 4.3 위조 거부: WebM-as-PNG → 400 + "지원하지 않는 파일 형식입니다" (Test 2, **핵심 bypass route 차단 확인** — handler의 video 분기를 건너뛰던 경로가 storage 레이어 mismatch로 막힘).
- [x] 4.4 위조 거부: WAV-as-JPEG → 400 + 같은 응답 (Test 4). PNG-as-MP4(Test 3)는 handler video 분기 진입 후 probe fail-closed로 400 (별 가드).
- [x] 4.5 빈 declared Content-Type(`media=@file` without `;type=`): 비교 skip하고 sniff만으로 201 통과 (Test 7).
- [x] 4.6 DB 확인: `SELECT title, media_type FROM pins WHERE creator_id=qatest AND created_at >= QA 시작 시각` 결과 4건(qa2-real-png/wav/wav-alias/png-no-type) 모두 sniff와 일치하는 media_type, 위조 시도로 생성된 잘못된 row 0건.
