## Tasks

- [x] 1. `apps/api/internal/pin/handler.go`에 본문 상한 상수와 부착 로직 추가
  - [x] 1.1 파일 상단 상수 묶음(`maxVideoDurationSeconds`, `maxBytes`) 옆에 `var requestBodyCap int64 = 500 << 20` 추가(테스트가 cap 경로를 결정적으로 검증할 수 있도록 `var`; production 코드는 mutate하지 않는다는 invariant를 godoc에 명시).
  - [x] 1.2 `Create` 진입부에서 `auth.CreatorIDFromContext` 검증을 통과한 직후, `ParseMultipartForm` 호출 직전에 `r.Body = http.MaxBytesReader(w, r.Body, requestBodyCap)` 한 줄 추가.
  - [x] 1.3 기존 `ParseMultipartForm` 에러 분기를 두 갈래로 확장. `errors.As(err, &maxBytesErr)`(`http.MaxBytesError` 포인터)이면 400 + "파일 크기가 제한을 초과했습니다"; 아니면 기존 400 + "잘못된 요청 형식입니다"를 유지.
  - [x] 1.4 `errors` 패키지 import 추가 (`net/http`는 이미 import됨).

- [x] 2. `apps/api/internal/pin/handler_test.go`에 회귀 방지 테스트 추가
  - [x] 2.1 `TestCreate_RejectsBodyOverCapBeforeDiskSpool`: `requestBodyCap`보다 큰 multipart 본문을 `bytes.NewReader`로 만들고 핸들러를 직접 호출. 응답 코드 400, body가 "파일 크기가 제한을 초과했습니다"를 포함, 응답 헤더에 storage 측 거절 응답과 동일한 Content-Type을 가짐.
  - [x] 2.2 `TestCreate_PreservesGenericMultipartErrorMessage`: 본문 상한 안에서 malformed multipart(잘못된 boundary)를 전송하면 기존 400 + "잘못된 요청 형식입니다"가 유지됨.
  - [x] 2.3 (선택) `TestCreate_RequestBodyCapConstant`: 상수 값이 `500 << 20`임을 직접 단언해 향후 의도치 않은 변경을 가시화.

- [x] 3. `openspec/changes/fix-pin-create-request-body-cap/specs/pin/spec.md`에 ADDED Scenario 작성
  - [x] 3.1 기존 Requirement `미디어 파일을 업로드한다`에 1개 Scenario를 ADDED.
  - [x] 3.2 Scenario 제목: "서버가 본문을 디스크에 스풀하기 전에 본문 상한으로 거절한다".
  - [x] 3.3 본 Scenario는 한도 값(500 MiB)을 명시하지 않으며 surface 행위 계약("디스크 스풀 이전에 거절", "파일 크기 초과 오류 반환")만 명시한다.

- [x] 4. 검증·통합
  - [x] 4.1 `go build ./...` 통과.
  - [x] 4.2 `go test ./internal/pin/...` 통과(신규 테스트 포함).
  - [x] 4.3 `go test ./...` 통과(전체 회귀 없음).
  - [x] 4.4 `openspec validate --specs --strict`로 pin capability 검증. 본 change 외 사전 드리프트는 본 change 범위 밖.
  - [x] 4.5 archive: capability main spec(`openspec/specs/pin/spec.md`)의 Requirement `미디어 파일을 업로드한다`에 신규 Scenario 머지. 기존 Scenario들의 본문·순서는 변경하지 않는다.
