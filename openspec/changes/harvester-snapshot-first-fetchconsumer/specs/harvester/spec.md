## ADDED Requirements

### Requirement: Consumer는 snapshot-first 진입점만 경유하여 fetch를 수행한다

Harvester consumer 루프는 fetch 단계에서 `harvester-snapshot-first-fetch` capability가 제공하는 snapshot-first 진입점만 호출해야 하며(SHALL), 저수준 `Fetcher.Fetch`를 직접 호출하지 않아야 한다(SHALL NOT). Consumer 모듈 코드는 ObjectStorage SDK(S3/MinIO 등) 구현체 타입이나 `net/http` 클라이언트 구현체를 직접 생성·참조해서는 안 된다(SHALL NOT). 인터페이스 기반 의존성 주입으로 진입점을 받는 것은 금지 대상이 아니며, 구체 구현체(클라이언트 생성부)가 consumer 모듈에 존재하지 않는 것이 경계 기준이다. 해당 구현체는 `harvester-snapshot-first-fetch` 구현 모듈 내부에만 존재한다.

#### Scenario: consumer 루프의 fetch 호출은 snapshot-first 진입점뿐이다
- **WHEN** consumer 루프 코드에서 fetch 단계를 정적으로 점검할 때
- **THEN** snapshot-first 진입점 이외의 경로(`Fetcher.Fetch` 직접 호출, raw HTTP, ObjectStorage 직접 조회 등)로 HTML을 가져오는 호출이 존재하지 않는다.

#### Scenario: consumer 모듈에 ObjectStorage/HTTP 클라이언트 의존 부재
- **WHEN** consumer 패키지의 import 그래프를 점검할 때
- **THEN** ObjectStorage(S3/MinIO) SDK 또는 `net/http` 클라이언트 인스턴스 생성부가 직접 참조되지 않으며, fetch 의존은 `harvester-snapshot-first-fetch` 모듈이 제공하는 진입점 심볼로만 연결된다.

---

### Requirement: snapshot-first 진입점의 입력은 `(ctx, url)`이며 `scheduler.Dequeue` 반환 형태와 정합한다

Snapshot-first 진입점의 입력은 `ctx`(컨텍스트)와 `url`(정규화된 URL 문자열) 두 가지여야 한다(SHALL). Consumer는 직전 `scheduler.Dequeue(QueueHarvester)`가 반환한 URL을 그대로 전달해야 하며(SHALL), 스냅샷 키를 인자로 추가 전달하거나, URL을 기반으로 스냅샷 키를 재계산해 넘겨서는 안 된다(SHALL NOT). 스냅샷 키는 진입점 내부에서 `pioneer-snapshot-storage` 공용 빌더로 계산되며(`harvester-snapshot-first-fetch` Decision 5), 이 계산은 consumer 관점에서 관측 대상이 아니다.

#### Scenario: consumer는 Dequeue URL을 그대로 전달한다
- **WHEN** consumer가 `Dequeue(QueueHarvester)`로 URL을 claim한 뒤 snapshot-first 진입점을 호출할 때
- **THEN** 전달하는 두 인자는 `ctx`와 claim 결과 URL뿐이며, 그 외 별도 DB 조회·캐시 조회로 얻은 스냅샷 키나 메타데이터를 인자로 추가하지 않는다.

#### Scenario: consumer는 스냅샷 키를 재계산하지 않는다
- **WHEN** consumer 루프 코드를 정적으로 점검할 때
- **THEN** URL로부터 스냅샷 키를 자체 계산하는 경로나, `harvester_frontier.snapshot_key`를 별도 SELECT로 조회해 진입점에 넘기는 경로가 존재하지 않는다.

---

### Requirement: snapshot-first 진입점의 반환은 3-tuple `(html, errorKind, err)` 의미론을 따른다

Snapshot-first 진입점은 세 결과(`html`, `errorKind`, `err`)를 반환해야 한다(SHALL). 실제 Go 타입(multiple return 또는 named struct)은 구현 선택이며, 행위 계약은 값 3개의 의미론만 규정한다. 성공 반환 시 `html`은 길이 1 이상의 원본 HTML 바이트열이고, `err`는 nil이어야 한다(SHALL). 성공 반환 시 `errorKind`의 구체적 값은 본 spec의 관찰 대상이 아니며, consumer는 성공 경로에서 `errorKind`를 분기 조건으로 사용해서는 안 된다(SHALL NOT). 실패 반환 시 `html`은 nil이고, `err`는 non-nil이며, `errorKind`는 본 spec의 "fetch 단 errorKind는 4종으로 한정된다" requirement의 네 값 중 하나여야 한다(SHALL).

#### Scenario: 성공 반환 형태
- **WHEN** 진입점이 HTML을 성공적으로 반환할 때
- **THEN** `html`은 길이 1 이상의 바이트열이고, `err`는 nil이다.

#### Scenario: 성공 경로에서 consumer는 errorKind를 사용하지 않는다
- **WHEN** 진입점이 성공 반환한 결과를 consumer가 처리할 때
- **THEN** consumer 코드의 후속 흐름은 `errorKind` 값을 분기 조건으로 사용하지 않으며, 성공/실패 판정은 `err`(또는 동등한 실패 표지)로만 이루어진다.

#### Scenario: 실패 반환 형태
- **WHEN** 진입점이 최종적으로 HTML을 확보하지 못할 때
- **THEN** `html`은 nil이고, `err`는 non-nil이며, `errorKind`는 "fetch 단 errorKind는 4종으로 한정된다" requirement의 네 값 중 하나다.

---

### Requirement: fetch 단 `errorKind`는 4종으로 한정된다

Snapshot-first 진입점이 실패 시 반환하는 `errorKind`는 `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"` 네 값 중 하나여야 한다(SHALL). `"parse"`, `"pin_create"`, 또는 기타 임의 문자열을 반환해서는 안 된다(SHALL NOT). Consumer는 fetch 실패 경로에서 이 값을 그대로 `scheduler.RecordHarvestError(url, errorKind)`에 전달해야 하며, HTTP 상태코드나 에러 타입을 다시 검사하여 kind를 재결정하는 로직을 포함해서는 안 된다(SHALL NOT). 본 금지는 **fetch 실패 경로에 한정**되며, 파싱/Pin 생성 실패에서 consumer가 자체 결정한 `"parse"`/`"pin_create"` kind로 `RecordHarvestError`를 호출하는 것은 본 금지의 적용 대상이 아니다. `SetStatus("harvest_failed", nil)`과 `RecordHarvestError`의 이중 호출 규약 자체는 `harvester-scheduler-consumer` capability가 정의하며 본 requirement는 그 규약을 변경하지 않는다.

#### Scenario: HTTP 4xx 응답은 errorKind = "http_4xx"
- **WHEN** snapshot miss 이후 live fetch가 HTTP 4xx로 종료될 때
- **THEN** 진입점은 `errorKind = "http_4xx"`로 실패를 반환한다.

#### Scenario: HTTP 5xx 응답은 errorKind = "http_5xx"
- **WHEN** live fetch가 HTTP 5xx로 종료될 때
- **THEN** 진입점은 `errorKind = "http_5xx"`로 실패를 반환한다.

#### Scenario: DNS/connect/TLS 실패는 errorKind = "network"
- **WHEN** live fetch가 DNS 해석, TCP connect, TLS handshake 실패로 종료될 때
- **THEN** 진입점은 `errorKind = "network"`로 실패를 반환한다.

#### Scenario: 타임아웃은 errorKind = "timeout"
- **WHEN** snapshot 경로 또는 live fetch가 ctx/자체 타임아웃으로 종료될 때
- **THEN** 진입점은 `errorKind = "timeout"`으로 실패를 반환한다.

#### Scenario: 진입점 구현은 4종 외 kind를 반환하지 않는다
- **WHEN** 진입점 구현이 실패 경로를 반환할 때
- **THEN** `errorKind`는 항상 `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"` 중 하나이며, `"parse"`, `"pin_create"`, 기타 자유 문자열이 반환되지 않는다.

#### Scenario: consumer는 fetch 실패 경로에서 errorKind를 재분류하지 않는다
- **WHEN** consumer가 진입점의 실패 반환을 받아 `RecordHarvestError`를 호출할 때
- **THEN** 전달하는 kind는 진입점이 반환한 값 그대로이며, consumer가 HTTP 상태코드/에러 타입을 다시 검사해 kind를 재결정하는 로직이 존재하지 않는다.

#### Scenario: parse/pin_create 경로는 본 재분류 금지의 적용 대상이 아니다
- **WHEN** fetch는 성공했으나 이후 `harvestPipeline.Process` 또는 Pin 생성이 실패하여 consumer가 `RecordHarvestError`를 호출할 때
- **THEN** consumer가 `"parse"` 또는 `"pin_create"` kind를 자체 결정해 전달하는 것은 본 requirement의 "errorKind 재분류 금지"와 충돌하지 않는다.

---

### Requirement: 스냅샷 내부 실패 종류는 consumer의 `errorKind`에 노출되지 않는다

ObjectStorage 조회 실패 종류(키 없음 / 만료 / 네트워크 / 권한 / 내부 에러)는 `harvester-snapshot-first-fetch`의 "단일 miss" 규약에 따라 진입점 내부에서 HTTP fallback으로 흡수되며, consumer가 받는 `errorKind`에는 반영되지 않아야 한다(SHALL). 스냅샷 경로 내부 실패가 consumer의 kind 분기에 영향을 주어서는 안 된다(SHALL NOT). 본 requirement는 `harvester-snapshot-first-fetch`가 `bot` capability에 정의한 "실패 유형은 로그로만 구분된다" Scenario를 `harvester` capability의 consumer-fetcher 경계에서 재확인하는 것이며, 새 행위를 도입하지 않는다.

#### Scenario: 스냅샷 키 부재는 consumer에 snapshot 전용 kind로 노출되지 않는다
- **WHEN** ObjectStorage에 스냅샷이 존재하지 않아 snapshot 조회가 miss로 종료될 때
- **THEN** 진입점은 `"snapshot_missing"` 같은 snapshot 전용 kind를 반환하지 않고, HTTP fallback을 수행한 뒤 그 결과(성공 또는 4종 errorKind 중 하나)를 반환한다.

#### Scenario: 스냅샷 경로 네트워크/권한/내부 에러도 동일하게 HTTP fallback
- **WHEN** ObjectStorage 조회가 네트워크/권한/내부(5xx) 에러로 실패할 때
- **THEN** 진입점은 이 실패 종류를 consumer에 전파하지 않고 HTTP fallback을 수행하며, consumer는 최종 결과(HTTP 성공 또는 4종 errorKind 중 하나의 실패)만 관측한다.
