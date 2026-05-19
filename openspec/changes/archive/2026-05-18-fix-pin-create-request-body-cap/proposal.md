## Why

`openspec/specs/pin/spec.md`의 Requirement `미디어 파일을 업로드한다` Scenario "서버에서 크기 초과가 감지된 경우"(L26-28)는 다음을 명시한다.

- **WHEN** 클라이언트 검증을 통과했지만 서버에서 크기 제한 초과가 감지되면
- **THEN** 서버가 파일 크기 초과 오류를 반환한다 (이중 검증 체계의 서버 방어선)

같은 Requirement의 Scenario "최적화 후에도 서버 크기 제한 초과"(L22-24)도 "서버 전송이 차단된다"로 표현되어 있으며, 같은 spec L26-28의 Scenario는 그 차단을 서버 측에서 **방어선**으로 보장한다.

그러나 `apps/api/internal/pin/handler.go`의 `Create`는 `r.ParseMultipartForm(500 << 20)` 한 줄만으로 multipart 본문을 받는다(L67-69, 주석 "Parse multipart: max 500MB"). Go `net/http` 표준 문서는 `Request.ParseMultipartForm(maxMemory)`의 `maxMemory` 인자가 **본문 상한이 아니라** 메모리/디스크 스풀 임계값이라고 명시한다("There is no limit on the total size of the request body or the number of files. To enforce limits on those, you should wrap the request body using `http.MaxBytesReader` before calling ParseMultipartForm"). 핸들러는 `http.MaxBytesReader`를 사용하지 않으며, `cmd/server/main.go`의 router 미들웨어 체인에도 본문 상한 미들웨어가 없다 (`grep -rn "MaxBytesReader" apps/api/` → 0건).

결과적으로 multipart 본문이 디스크에 스풀된 **후에야** `internal/storage/storage.go`의 `Upload`가 `size > maxBytes[mt]` 검사로 거절한다(L141-144). 거절 시점이 디스크 소비 이후이므로, 서버 방어선이 디스크 소진 DoS를 차단하지 못한다. 인증된 단일 유저가 `30/분/유저`(`docs/architecture.md` "### Rate Limit") 안에서 분당 30번까지 임의 크기 본문을 디스크에 스풀할 수 있고, 디스크가 소진되면 다른 핸들러·다른 서비스에 영향이 전파된다.

본 change는 `pin.Create` 진입 직후 `r.Body`를 `http.MaxBytesReader`로 래핑해 multipart 파서가 디스크에 본문을 스풀하기 전에 본문 상한을 enforce한다. 한도는 기존 주석이 가정한 500 MiB(`500 << 20`)를 유지한다. 본 change는 `internal/storage/storage.go`의 미디어 타입별 상한(image 10MB / audio 50MB / video 100MB)이나 클라이언트 검증, 트리밍/재인코딩 로직은 변경하지 않는다.

## What Changes

- `pin.Create` 핸들러는 `r.ParseMultipartForm`을 호출하기 전에 `r.Body`를 본문 상한으로 래핑해야 한다. 래핑 한도를 초과한 본문은 디스크에 스풀되기 전에 거절되어야 한다.
- 본문 상한 초과 시 응답은 `Scenario "서버에서 크기 초과가 감지된 경우"`의 SHALL("파일 크기 초과 오류를 반환한다")을 만족해야 한다. 응답 메시지는 storage 측 크기 초과 응답과 동일한 한국어 문구를 사용한다.
- 본문 상한 한도(500 MiB)는 `internal/pin` 패키지 내부 변수로 둔다(단위 테스트가 cap 경로를 결정적으로 검증할 수 있도록 형태는 `var`이며, production 코드가 mutate하지 않는다는 invariant를 godoc에 명시한다). spec text는 한도 자체를 명시하지 않고 "서버가 본문을 디스크에 스풀하기 전에 상한으로 거절한다"는 surface 행위 계약만 명시한다.
- 한도 범위 안에서의 정상 multipart 본문(트리밍 전 비디오, 썸네일, 폼 필드 합산)은 기존 동작 그대로 처리되어야 한다.
- 본문 상한과 무관한 multipart 오류(예: malformed boundary)는 기존대로 400 "잘못된 요청 형식입니다"로 응답한다.

## Capabilities

### Modified Capabilities

- `pin`: 기존 Requirement `미디어 파일을 업로드한다`에 1개 Scenario를 ADDED로 추가한다 — 서버 방어선이 디스크 스풀 이전에 발동함을 가시화한다.

### New Capabilities

<!-- 없음. pin capability는 이미 존재한다. -->

## Impact

- 영향 코드: `apps/api/internal/pin/handler.go`(상수 1개 추가, `Create` 진입부 1줄 추가, MaxBytesError 분기 1개 추가), 신규 단위 테스트 2-3건. `internal/storage/storage.go`, `cmd/server/main.go` 등 다른 파일은 수정하지 않는다.
- 운영 지표: 정상 본문(500 MiB 이하)에 대한 응답 본문·헤더·상태 코드는 변경되지 않는다. 500 MiB를 초과하는 본문에 대해서만 응답 시점이 "디스크 스풀 완료 후" → "본문 읽기 도중"으로 빨라지며, 응답 코드는 storage 거절과 동일한 400 + "파일 크기가 제한을 초과했습니다"를 유지한다.
- 의존성·인프라·DB 마이그레이션 없음. Redis·S3 서버 측 변경 없음.
- 롤백: 추가된 `http.MaxBytesReader` 한 줄을 제거하면 즉시 직전 동작으로 복귀(이 경우 서버 방어선이 디스크 스풀 이후로 다시 늦춰지지만 정상 본문 경로의 가용성에는 영향 없음). 변경 전후로 라우트 외부 컨트랙트가 동일하다.
