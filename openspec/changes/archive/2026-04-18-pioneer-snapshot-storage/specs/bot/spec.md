## ADDED Requirements

### Requirement: Pioneer는 fetch 성공 시 raw 응답을 object storage에 스냅샷 저장한다
스냅샷 저장 기능이 운영 토글로 활성화된 상태에서 Pioneer가 URL을 fetch하여 성공적으로 본문을 수신한 경우, 시스템은 해당 raw 응답 바이트를 object storage에 스냅샷으로 업로드해야 한다(SHALL). "fetch 성공"은 HTTP 2xx 응답 수신 및 본문 길이 > 0을 의미한다(SHALL). 운영 토글이 비활성화된 경우 시스템은 어떤 스냅샷도 업로드하지 않아야 한다(SHALL NOT).

#### Scenario: 2xx 본문 수신 시 스냅샷 업로드
- **GIVEN** 스냅샷 저장 기능이 활성화되어 있을 때
- **WHEN** Pioneer가 URL을 fetch하여 HTTP 200 응답과 비어 있지 않은 본문을 수신할 때
- **THEN** 해당 raw 응답 바이트를 object storage에 업로드한다

#### Scenario: 링크 추출과 별개로 저장 수행
- **GIVEN** 스냅샷 저장 기능이 활성화되어 있을 때
- **WHEN** Pioneer가 fetch 성공 후 링크를 추출할 때
- **THEN** 링크 추출과 무관하게 동일한 raw 바이트가 스냅샷으로 저장된다

#### Scenario: 스냅샷 저장 기능 비활성화 시 업로드 스킵
- **GIVEN** 스냅샷 저장 기능이 비활성화되어 있을 때
- **WHEN** Pioneer가 URL을 fetch하여 HTTP 2xx 응답과 비어 있지 않은 본문을 수신할 때
- **THEN** object storage에 스냅샷이 업로드되지 않으며, 링크 추출과 후속 큐 처리는 정상적으로 수행된다

---

### Requirement: 스냅샷은 gzip 압축으로 저장되며 TTL 365일을 따른다
시스템은 스냅샷을 gzip으로 압축하여 저장해야 한다(SHALL). 스냅샷의 보존 기간은 업로드 시점 기준 365일이며, 이 기간이 지나면 삭제되어야 한다(SHALL).

#### Scenario: 압축된 콘텐츠 업로드
- **WHEN** Pioneer가 raw 응답 바이트를 스냅샷으로 저장할 때
- **THEN** 바이트는 gzip으로 압축된 상태로 object storage에 업로드된다

#### Scenario: 365일 경과 후 만료
- **WHEN** 스냅샷이 업로드된 지 365일이 지났을 때
- **THEN** 해당 스냅샷은 object storage에서 삭제된다

---

### Requirement: 스냅샷 키는 normalized URL의 sha256 기반이다
시스템은 스냅샷 키를 normalized URL의 **sha256** digest(hex 인코딩 64자 소문자)를 기반으로 생성해야 한다(SHALL). 키 형식은 `snapshots/{sha256(normalized_url)}/{yyyymmdd}.html.gz` 이며, `{sha256(normalized_url)}`은 정확히 64자의 hex digest이고 `{yyyymmdd}`는 UTC 기준 fetch 날짜이다(SHALL). 동일한 normalized URL은 동일한 sha256 hex를 산출해야 한다(SHALL).

#### Scenario: 동일 URL은 동일 sha256
- **WHEN** Pioneer가 normalized 결과가 같은 두 URL을 각각 fetch할 때
- **THEN** 두 스냅샷 키의 sha256 hex 세그먼트가 동일하다

#### Scenario: 키 형식 준수 (64자 hex + UTC 날짜)
- **WHEN** Pioneer가 UTC 2026-04-17에 URL을 fetch하여 스냅샷을 저장할 때
- **THEN** 업로드되는 객체 키는 `snapshots/<64-char-sha256-hex>/20260417.html.gz` 형태이고, hex 세그먼트는 소문자 `[0-9a-f]`로만 구성된 정확히 64자다

#### Scenario: 같은 날 같은 URL 재fetch
- **WHEN** Pioneer가 같은 UTC 날짜에 동일한 normalized URL을 두 번 fetch하여 저장할 때
- **THEN** 두 번째 업로드는 첫 번째와 같은 키를 덮어쓴다

---

### Requirement: 동일 키에 대한 동시 쓰기는 last-write-wins이다
시스템은 동일 키에 대한 동시 PUT을 object storage의 기본 atomic PUT 동작에 위임해야 한다(SHALL). 애플리케이션 레벨의 lock, conditional write, versioning을 사용하지 않아야 한다(SHALL NOT). 마지막에 commit된 PUT이 최종 객체로 남아야 한다(SHALL).

#### Scenario: 동일 URL을 여러 Pioneer 워커가 같은 날 저장
- **WHEN** 두 개 이상의 Pioneer 워커가 동일한 normalized URL을 같은 UTC 날짜에 각각 fetch하여 스냅샷을 업로드할 때
- **THEN** 두 PUT 모두 동일 키를 대상으로 수행되며, 마지막으로 commit된 쓰기의 내용이 최종 객체로 유지된다 (last-write-wins)

#### Scenario: 동시 쓰기 시 별도 충돌 에러가 Pioneer에 전파되지 않음
- **WHEN** 동일 키에 대한 동시 PUT이 발생할 때
- **THEN** Pioneer는 lock 획득 실패나 version conflict 같은 별도 에러 경로를 거치지 않고, 각자의 PUT 결과(성공/실패)만 일반 업로드 경로로 처리한다

---

### Requirement: fetch 실패 시 스냅샷을 저장하지 않는다
시스템은 fetch가 실패한 경우 스냅샷을 업로드하지 않아야 한다(SHALL). 실패에는 HTTP 4xx/5xx 응답, 네트워크 오류, 타임아웃, 본문 길이 0이 포함된다(SHALL).

#### Scenario: HTTP 404 응답
- **WHEN** Pioneer가 fetch한 URL이 HTTP 404를 반환할 때
- **THEN** object storage에 스냅샷이 업로드되지 않는다

#### Scenario: 네트워크 타임아웃
- **WHEN** Pioneer가 fetch 중 타임아웃으로 실패할 때
- **THEN** object storage에 스냅샷이 업로드되지 않는다

#### Scenario: 본문이 비어 있는 성공 응답
- **WHEN** Pioneer가 HTTP 2xx를 수신했으나 본문 길이가 0일 때
- **THEN** object storage에 스냅샷이 업로드되지 않는다

---

### Requirement: 스냅샷 저장 실패는 fail-open으로 처리한다
시스템은 object storage 업로드가 실패하더라도 Pioneer의 크롤 진행을 중단시키지 않아야 한다(SHALL). 업로드 실패는 로그로 기록되어야 하며(SHALL), fetch된 응답으로부터 링크 추출 및 후속 큐 처리는 정상적으로 수행되어야 한다(SHALL).

#### Scenario: 업로드 실패 시 크롤 계속
- **WHEN** Pioneer가 fetch 성공 후 object storage에 업로드를 시도했으나 업로드가 실패할 때
- **THEN** Pioneer는 해당 URL의 링크 추출과 다음 URL 처리를 계속한다

#### Scenario: 업로드 실패 로그 기록
- **WHEN** object storage 업로드가 실패할 때
- **THEN** 실패 원인이 로그로 남는다

#### Scenario: 업로드 실패가 스케줄러 상태에 영향 없음
- **WHEN** object storage 업로드만 실패하고 fetch는 성공했을 때
- **THEN** URLScheduler 상 해당 URL은 fetch 성공으로 취급되어 재시도 큐에 들어가지 않는다
