## Context

`openspec/specs/pin/spec.md`의 Requirement `미디어 파일을 업로드한다`는 5개 Scenario에 걸쳐 **클라이언트 검증 + 서버 검증**의 이중 방어 체계를 만든다.

- Scenario "허용되지 않은 파일 형식": 클라이언트 검증 (즉시 거절)
- Scenario "최적화 후에도 서버 크기 제한 초과": 클라이언트 측 거절(서버 전송 차단)
- Scenario "서버에서 크기 초과가 감지된 경우": 서버 측 거절 ("이중 검증 체계의 서버 방어선")

세 번째 Scenario의 SHALL("서버 방어선")은 클라이언트 검증을 우회한 요청(직접 cURL/스크립트로 임의 본문 전송)이 서버 자원에 영향을 주기 전에 거절되어야 한다는 의미다. 현재 구현은 `internal/storage/storage.go`의 `Upload`에서 `size > maxBytes[mt]`로 거절하지만, 그 시점이 multipart 파서가 본문을 모두 디스크에 스풀한 **이후**이므로 디스크 소진을 방어하지 못한다. Go `net/http` 표준 문서가 명시하듯 `Request.ParseMultipartForm(maxMemory)`의 인자는 본문 상한이 아니라 메모리/디스크 스풀 임계값일 뿐이다.

본 change는 multipart 파서 진입 **이전**에 본문 상한을 enforce해 그 SHALL을 실제 방어선으로 만든다. Go 표준 라이브러리가 그 목적으로 제공하는 surface는 `http.MaxBytesReader`다.

## Goals / Non-Goals

### Goals

- `pin.Create`가 multipart 본문 파싱을 시작하기 전에 본문 상한을 강제한다. 상한 초과는 디스크에 어떤 바이트도 스풀되기 전에 거절된다.
- spec Scenario "서버에서 크기 초과가 감지된 경우"의 SHALL("파일 크기 초과 오류를 반환한다")을 본문 상한 위반에서도 만족시킨다. 응답 문구는 storage 측 크기 초과 응답과 동일하게 유지한다.
- 본문 상한 surface는 `pin` 패키지 안에 한정한다. 다른 라우트(`/api/og/fetch`, `/api/auth/*` 등)의 본문 처리는 변경하지 않는다.

### Non-Goals

- `internal/storage/storage.go`의 미디어 타입별 상한(image 10MB / audio 50MB / video 100MB) 변경. 이는 spec text가 "서버 크기 제한"이라고만 부르는 값으로 본 change 범위 밖.
- `internal/storage/storage.go`의 `io.ReadAll(body) + append` 메모리 패턴 개선. 별도 백로그(`backlog-system.yaml`의 "보조 시나리오 (concurrent upload memory pressure)") 후보로 둔다.
- 모든 라우트에 본문 상한을 부착하는 글로벌 미들웨어 도입. 다른 라우트(OG fetch·auth·feed 등)의 본문 크기 정책은 본 change 범위 밖이며 별도 결정으로 분리.
- 스트리밍 업로드(`io.ReadAll` 제거 + multipart part Reader 직접 PutObject) 리팩터. 변경 폭이 크며 본 결함의 최소 해결 범위 밖.
- `r.ParseMultipartForm`의 메모리 임계값 인자(500 MiB) 변경. 본 change는 그 인자를 그대로 두고 `MaxBytesReader` 한도를 동일 값으로 일치시킨다.

## Decisions

### Decision 1: `http.MaxBytesReader`를 `pin.Create` 진입부에 부착 (옵션 A 채택)

세 가지 옵션을 검토했다.

- **(A) `pin.Create` 진입부 한 줄 — `r.Body = http.MaxBytesReader(w, r.Body, requestBodyCap)`** ← 채택.
  - 장점: 본 결함의 영향 라우트(`POST /api/pins`) 하나에 surface가 정확히 한정된다. 변경 폭이 최소(1줄 추가 + MaxBytesError 분기 1개)이고, 다른 라우트의 본문 정책에 영향이 없다.
  - 단점: 미래에 다른 multipart 라우트가 추가되면 같은 패턴을 다시 부착해야 한다. 그러나 본 change 시점에 multipart 라우트는 `POST /api/pins` 하나뿐이다 (`grep -rn "ParseMultipartForm" apps/api/internal/` → 1건).

- (B) chi 라우터 글로벌 미들웨어로 모든 라우트에 본문 상한을 부착.
  - 장점: 일관된 정책.
  - 단점: `/api/og/fetch`·`/api/auth/*`·`/api/feed` 등 작은 본문 라우트에는 더 작은 상한이 적절하다(예: OG fetch는 URL 파라미터 수십 KB). 단일 상한을 글로벌로 두면 라우트별 의도가 사라지고, 일부 라우트에는 과한 surface가 된다. 본 결함의 최소 해결 범위 밖.

- (C) chi 라우터 라우트 그룹에 본문 상한 미들웨어를 라우트별로 부착.
  - 장점: 라우트별 의도 보존 + 핸들러 본문 영향 없음.
  - 단점: 본 change 시점에 multipart 라우트는 하나뿐이므로 미들웨어 일반화의 효용이 낮다. 핸들러 본문 1줄과 미들웨어 1개를 비교하면 전자가 변경 폭이 작고 호출 의도가 명료하다.

(A)는 본 결함의 영향 라우트에 정확히 surface를 부착하고, 향후 multipart 라우트가 늘어나면 일반화를 별도 change로 분리할 수 있다.

### Decision 2: 본문 상한 값을 500 MiB로 둔다

`apps/api/internal/pin/handler.go:67` 주석 "Parse multipart: max 500MB (video originals before server-side trim)"이 표현하는 개발자 mental model을 그대로 강제한다.

- 정상 경로: 비디오 원본은 트리밍 전이라 100 MiB(저장 상한)를 초과할 수 있다. 비디오 + 썸네일 + 폼 필드 합산이 500 MiB를 넘는 정상 케이스는 관측되지 않는다.
- 공격 경로: 50 GB 본문 같은 명백한 abuse는 500 MiB 한 줄에서 차단된다.
- 한도 자체는 spec text로 표현하지 않는다(spec은 surface 행위 계약만 명시). 한도 값은 `internal/pin` 패키지의 `const requestBodyCap int64 = 500 << 20`으로 둔다. 한도 조정이 필요해지면 코드 상수로 조정하고 spec text는 변경하지 않아도 된다.

### Decision 3: MaxBytesError 응답 코드·문구

`http.MaxBytesReader`가 상한을 넘으면 후속 `Read` 호출이 `*http.MaxBytesError`를 반환한다(Go 1.18+). `ParseMultipartForm`이 그 에러를 internal하게 전파하므로 `Create`는 `errors.As(err, &maxBytesErr)`로 분기 가능하다.

세 가지 옵션을 검토했다.

- **(A) 400 + "파일 크기가 제한을 초과했습니다"** ← 채택.
  - storage.go의 기존 크기 초과 응답(handler.go:248-251)과 동일한 코드·문구. spec Scenario "서버에서 크기 초과가 감지된 경우"의 THEN("파일 크기 초과 오류를 반환한다")이 한 가지 형태로 통일된다.
- (B) 413 Payload Too Large + 별도 문구.
  - HTTP semantic은 더 정확하지만, 기존 storage 거절 경로가 400을 쓰고 있어 이중 거절 경로의 응답 코드가 갈라진다. 클라이언트가 두 경로를 구분해야 할 필요가 spec에 없다.
- (C) ParseMultipartForm 일반 실패와 똑같이 400 + "잘못된 요청 형식입니다".
  - 본문 상한 초과는 본문 형식 오류가 아니므로 잘못된 분류. 사용자가 "왜 거절됐는지" 파악하기 어렵다.

(A)는 기존 거절 경로와 응답 형태가 일관되며 클라이언트의 에러 분기 로직을 단순하게 유지한다.

### Decision 4: 상수의 위치 및 선언 형태

`requestBodyCap`은 `internal/pin/handler.go`의 기존 상수들(`maxVideoDurationSeconds`, `maxBytes`) 옆에 둔다.

- 같은 도메인 상수 묶음 안에 두면 핸들러 본문을 읽는 사람이 자연스럽게 발견.
- 다른 패키지(`internal/storage`)에 두면 storage가 HTTP body cap을 알아야 해 추상화가 흐려진다. HTTP body cap은 HTTP 핸들러의 책임.

선언 형태는 `const`가 아닌 `var`로 둔다. 이는 의도적 trade-off다.

- `const`였다면 단위 테스트가 500 MiB 본문을 실제로 만들어야 cap 경로를 검증할 수 있다. 500 MiB 메모리 할당은 CI에서 OOM 위험이 있고 매 테스트마다 시간이 길어진다.
- `var`로 두면 테스트가 `t.Cleanup`으로 임시 cap을 KB 단위로 낮춰 cap 경로를 결정적으로 검증할 수 있다. godoc에 production 코드가 mutate하지 않는다는 invariant를 명시하고, 테스트만 이 변수를 변경하도록 컨벤션을 둔다.
- production 코드의 mutate 가능성은 패키지 외부에 노출되지 않는다(`requestBodyCap`은 소문자, unexported). 패키지 내부에서만 접근 가능하며 패키지 내부에서 변경하는 곳은 테스트 cleanup 한 곳뿐이다.

## Risks / Trade-offs

- **R1**: 정상 경로에서 합법적인 본문이 500 MiB를 넘어 거절되는 경우.
  - 완화: 본 change 시점에 합법적 본문은 비디오 원본(트리밍 전) + 썸네일 + 폼 필드로 구성되며, 클라이언트(`apps/web/`)는 비디오 선택 시 미리 트리밍 모달을 거치므로 트리밍 전 본문이 500 MiB를 넘는 경우는 관측되지 않는다. 회귀 발생 시 단상수 1줄을 늘리면 즉시 복귀.
- **R2**: `http.MaxBytesReader`는 한도 초과 시 응답 코드를 400/413 중 어느 것으로 자동 설정할지가 Go 버전에 따라 다르다. 본 change는 핸들러가 `*http.MaxBytesError`를 명시적으로 감지해 응답을 직접 작성하므로 자동 동작 차이의 영향이 없다.
- **R3**: ParseMultipartForm 내부에서 MaxBytesReader가 트리거되면 `*MaxBytesError`가 다른 에러로 wrap될 가능성이 있다.
  - 완화: `errors.As(err, &maxBytesErr)`는 wrap 체인을 따라가므로 안전. 별도 fallback으로 `errors.Is(err, http.ErrBodyReadAfterClose)`나 단순 strings.Contains는 사용하지 않는다.
- **R4**: 본 change는 multipart 본문 처리만 다루며 `internal/storage/storage.go`의 `io.ReadAll(body) + append` 메모리 패턴은 그대로 둔다. 즉 정상 100 MiB 비디오가 처리될 때 메모리에 ~100 MiB가 올라가는 기존 동작은 변하지 않는다. 동시 업로드 메모리 압박은 별도 결함으로 남는다. 본 change의 invariant는 "다스크 스풀 이전 본문 상한"에 한정.

## Alternatives Considered

- 본문 상한을 `Content-Length` 헤더 검사로 enforce. `MaxBytesReader`는 헤더 신뢰가 아닌 실제 읽기 바이트로 동작하므로 chunked transfer encoding과 잘못된 헤더에 모두 안전. 헤더 검사는 우회 가능해 채택하지 않음.
- multipart 파서를 직접 작성해 part 단위 스트리밍 거절. 변경 폭이 매우 크며 본 결함의 최소 해결 범위 밖. `MaxBytesReader` + 기존 `ParseMultipartForm` 조합으로 충분.
- ParseMultipartForm의 `maxMemory` 인자를 0으로 낮춰 메모리 스풀을 완전 차단. 메모리 임계값 인자는 본문 상한이 아니라 디스크 스풀 임계값이므로 본 결함을 해결하지 못한다. (`io.ReadAll`이 결국 메모리에 올리는 점은 별도 결함.)

## Migration Plan

1. `internal/pin/handler.go`에 `requestBodyCap` 상수 추가(`500 << 20`).
2. `Create` 진입부에서 `auth.CreatorIDFromContext` 검증 직후, `ParseMultipartForm` 직전에 `r.Body = http.MaxBytesReader(w, r.Body, requestBodyCap)` 한 줄 추가.
3. `ParseMultipartForm` 에러 분기를 두 갈래로 확장: `errors.As(err, &maxBytesErr)`이면 storage 크기 초과 응답과 동일한 400 + "파일 크기가 제한을 초과했습니다"; 아니면 기존 400 + "잘못된 요청 형식입니다".
4. 단위 테스트 추가:
   - (a) 본문이 상한 초과 시 400 + 한국어 문구 응답.
   - (b) 한도 이내의 valid multipart 본문은 cap 분기에 걸리지 않고 정상 진행(테스트는 cap 검사만 검증해도 충분, 풀 핸들러 통합 테스트는 별도 surface refactor 필요).
   - (c) malformed multipart 본문은 기존 400 + "잘못된 요청 형식입니다"를 유지.
5. `openspec validate --specs --strict`로 pin capability의 신규 Scenario가 main spec과 정합한지 검증.
6. archive 시점에 새 Scenario를 main spec의 기존 Requirement `미디어 파일을 업로드한다` 안에 머지.

## Open Questions

없음. surface 부착 위치, 한도 값, 응답 코드·문구, 상수 위치 모두 본 design에서 확정.
