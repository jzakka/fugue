## ADDED Requirements

### Requirement: Pioneer는 URLScheduler의 consumer이다
Pioneer는 URL frontier를 `URLScheduler` 인터페이스를 통해서만 읽어야 하며(SHALL), 자체 URL 큐/스택/BFS 자료구조를 보유해서는 안 된다(SHALL NOT). 크롤할 다음 URL은 `scheduler.Dequeue`로만 획득한다.

#### Scenario: Dequeue로만 다음 URL을 획득한다
- **WHEN** Pioneer가 다음으로 크롤할 URL을 결정할 때
- **THEN** `scheduler.Dequeue(...)`를 호출하여 URL을 얻으며, 내부 큐/스택/리스트에서 꺼내지 않는다

#### Scenario: 자체 URL 큐를 보유하지 않는다
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** Pioneer 내부에 URL을 누적하는 큐/스택/슬라이스/채널 상태가 존재하지 않는다 (scheduler가 유일한 큐이다)

#### Scenario: frontier가 비어 있으면 Dequeue가 블록된다
- **WHEN** frontier에 claim 가능한 URL이 없는 상태에서 Pioneer가 `scheduler.Dequeue`를 호출할 때
- **THEN** Pioneer는 scheduler가 URL을 반환할 때까지 대기한다 (scheduler의 busy-wait/sleep 정책을 그대로 따르며 Pioneer 측에서 별도 스핀/슬립을 구현하지 않는다)

---

### Requirement: Pioneer 메인 루프는 Dequeue → fetch → parse → Enqueue 반복이다
Pioneer의 메인 루프는 `scheduler.Dequeue`, URL fetch, 링크 추출, `scheduler.Enqueue(urls...)`의 반복으로 구성되어야 한다(SHALL). 추가 단계를 이 루프의 책임으로 포함하지 않아야 한다(SHALL NOT).

#### Scenario: 정상 경로의 루프 순서
- **WHEN** Pioneer가 한 번의 반복을 수행할 때
- **THEN** 순서대로 `scheduler.Dequeue` → URL fetch → 링크 추출 → `scheduler.Enqueue(urls...)`가 실행된다

#### Scenario: 추출한 링크를 같은 frontier로 다시 Enqueue한다
- **WHEN** 한 URL의 fetch 결과에서 n개 링크를 추출했을 때
- **THEN** Pioneer는 `scheduler.Enqueue(links...)`를 호출하여 같은 frontier에 다시 투입한다 (별도 큐/채널/파일로 내보내지 않는다)

#### Scenario: 루프는 종료 조건 없이 계속된다
- **WHEN** Pioneer 프로세스가 정상 구동 중일 때
- **THEN** 메인 루프는 `Dequeue → fetch → parse → Enqueue`를 반복하며, 종료 조건 및 예산 관리는 이 루프의 책임이 아니다 (worker 종료 정책은 별도 capability에서 정의한다)

---

### Requirement: Pioneer는 fetch 성공/실패를 scheduler에 보고한다
Pioneer는 URL fetch 결과를 `scheduler.SetStatus(url, status)`로 scheduler에 보고해야 한다(SHALL). 성공과 실패 모두 보고 대상이며(SHALL), Pioneer가 frontier 테이블의 컬럼을 직접 UPDATE해서는 안 된다(SHALL NOT).

#### Scenario: fetch 성공 시 성공 상태 보고
- **WHEN** Pioneer가 URL을 성공적으로 fetch하고 링크를 추출했을 때
- **THEN** `scheduler.SetStatus(url, <성공 상태>)`를 호출한다 (구체 상태 문자열은 scheduler 계약에서 정의한다)

#### Scenario: fetch 실패 시 실패 상태 보고
- **WHEN** Pioneer가 URL fetch에 실패했을 때
- **THEN** `scheduler.SetStatus(url, <실패 상태>)`를 호출한 뒤 다음 `Dequeue`로 진행한다

#### Scenario: frontier 컬럼 직접 UPDATE 금지
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** Pioneer가 `bot_frontier` 테이블의 `last_fetched_at`/`fetch_error_count`/`next_fetch_at` 등 컬럼을 직접 UPDATE하는 코드가 존재하지 않는다 (모든 상태 변경은 `URLScheduler` API를 경유한다)

---

### Requirement: Pioneer는 producer이자 consumer이다
Pioneer는 동일한 `URLScheduler`에 대해 consumer(`Dequeue`)와 producer(`Enqueue`) 역할을 모두 수행해야 한다(SHALL). 별도의 ingestor나 외부 큐 파이프라인을 경유해서는 안 된다(SHALL NOT).

#### Scenario: 같은 scheduler 인스턴스에 Enqueue와 Dequeue
- **WHEN** Pioneer가 링크를 추출하여 frontier에 다시 투입할 때
- **THEN** `Dequeue`로 URL을 받았던 동일한 `URLScheduler` 인스턴스의 `Enqueue`를 호출한다

#### Scenario: 중복 URL은 scheduler가 흡수한다
- **WHEN** 이미 frontier에 존재하는 URL을 포함한 링크 목록을 Pioneer가 `Enqueue`할 때
- **THEN** 중복은 frontier의 `normalized_url` unique constraint가 걸러내며, Pioneer가 사전 dedup을 수행할 필요가 없다

---

### Requirement: 콘텐츠 추출은 Pioneer의 책임이 아니다
Pioneer는 fetch된 HTML에서 **링크만** 추출해야 한다(SHALL). JavaScript 스크립트 실행, 미디어 다운로드, Pin 생성, 콘텐츠 항목 배열 생성은 Pioneer의 책임이 아니다(SHALL NOT).

#### Scenario: Pioneer는 링크 목록만 반환한다
- **WHEN** Pioneer가 HTML을 parse할 때
- **THEN** 추출 결과는 URL 목록(및 관련 메타데이터에 한함)이며, 콘텐츠 항목(title/mediaURL/mediaType 등)을 반환하지 않는다

#### Scenario: 스크립트 실행기 미호출
- **WHEN** Pioneer 실행 경로를 분석할 때
- **THEN** Pioneer는 JavaScript 파싱 스크립트 실행기나 처리 파이프라인을 호출하지 않는다 (이 책임은 Harvester에 있다)

#### Scenario: Pin 생성 미호출
- **WHEN** Pioneer가 페이지를 처리할 때
- **THEN** Pioneer는 Pin을 생성하지 않는다 (Pin 생성은 Harvester의 처리 파이프라인 책임이다)

---

### Requirement: 다중 워커 정확성은 scheduler가 보장한다
Pioneer는 복수 인스턴스로 동시 실행되는 것을 전제로 해야 하며(SHALL), 동일 URL 중복 fetch 방지와 정확히-한 번 claim 보장은 `URLScheduler` 구현체(Postgres `FOR UPDATE SKIP LOCKED`)에 위임해야 한다(SHALL). Pioneer 코드에 워커 간 동시성 제어(분산 락, advisory lock, 중앙 조정자)를 두어서는 안 된다(SHALL NOT).

#### Scenario: 복수 Pioneer가 동일 frontier를 공유
- **WHEN** N개의 Pioneer 인스턴스가 동일 scheduler에 연결되어 실행될 때
- **THEN** 모든 인스턴스는 `Dequeue`/`Enqueue`만으로 동작하며, 서로를 인지하거나 조율하는 코드가 존재하지 않는다

#### Scenario: 동일 URL 동시 fetch 금지는 scheduler의 claim으로 보장
- **WHEN** 동일 URL이 두 Pioneer에게 동시에 요구될 때
- **THEN** scheduler의 claim(`FOR UPDATE SKIP LOCKED`)이 한 쪽에만 URL을 반환하며, Pioneer는 이 동작을 재구현하지 않는다

#### Scenario: Pioneer 내부에 분산 락/mutex 부재
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** 워커 간 조율을 위한 분산 락, advisory lock, Redis lock, 전역 mutex 등이 존재하지 않는다

---

### Requirement: Pioneer는 인메모리 크롤 상태를 보유하지 않는다
Pioneer 프로세스는 인메모리에 URL 큐, visited 집합, 사이트/세션 카운터 같은 **크롤 상태**를 보유해서는 안 된다(SHALL NOT). HTTP 커넥션 풀, DNS 캐시 등 프로세스 로컬 리소스 캐시는 이 제약에서 제외된다.

#### Scenario: 재시작 후 상태가 frontier로부터 복구된다
- **WHEN** 크롤 진행 중 Pioneer 프로세스가 종료되었다가 재시작될 때
- **THEN** 새 프로세스는 즉시 `scheduler.Dequeue`를 호출하여 frontier의 현재 상태 그대로 크롤을 이어간다 (이전 세션의 큐/visited 복원이 불필요하다)

#### Scenario: visited 맵/세션 카운터 부재
- **WHEN** Pioneer 구현체를 정적 분석할 때
- **THEN** URL 방문 여부를 기록하는 인메모리 맵, 사이트별 처리 카운터, 세션 단위 상태 변수가 존재하지 않는다

#### Scenario: 사이트 경계를 기억하지 않는다
- **WHEN** Pioneer가 교차 도메인 링크를 `Enqueue`할 때
- **THEN** Pioneer는 "어느 사이트 세션에 속한다"는 인메모리 맥락 없이 단순히 scheduler에 투입하며, 사이트 분류는 frontier/필터가 담당한다

---

### Requirement: Pioneer는 교차 사이트 크롤을 허용한다
Pioneer는 사이트/도메인 경계를 인지하지 않아야 한다(SHALL NOT). 도메인 제한이 필요한 경우 link filter 정책으로 표현되어야 하며(SHALL), Pioneer의 구조적 제약으로 두지 않는다.

#### Scenario: 외부 도메인 링크 Enqueue 허용
- **WHEN** Pioneer가 현재 fetch 중인 페이지와 다른 도메인의 링크를 추출했을 때
- **THEN** 도메인 차이만을 이유로 해당 링크를 버리지 않으며, 필터 정책이 별도로 차단하지 않는 한 `scheduler.Enqueue`에 포함시킨다

#### Scenario: 도메인 제한은 필터의 책임
- **WHEN** 특정 배포에서 도메인 제한이 필요할 때
- **THEN** 제한은 link filter 정책(별도 capability)으로 구현되며 Pioneer 코드 자체는 변경되지 않는다
