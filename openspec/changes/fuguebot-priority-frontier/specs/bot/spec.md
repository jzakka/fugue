## MODIFIED Requirements

### Requirement: 그래프 노드와 엣지를 관리한다
시스템은 발견한 페이지를 URL 단위 노드로, 페이지 간 발견된 링크를 엣지로 저장해야 한다(SHALL). 노드의 기본 식별자는 정규화된 URL이어야 하며(SHALL), 사이트별 템플릿 패턴 노드로 강제 머지하지 않아야 한다(SHALL). 시스템은 host/domain 정보를 스케줄링 힌트로 기록할 수 있어야 하며(SHALL), allow/deny 규칙과 `robots.txt` 정책상 허용된 링크만 사이트 경계를 넘어 저장할 수 있어야 한다(SHALL).

#### Scenario: 정규화된 URL로 노드 식별
- **WHEN** Pioneer가 같은 페이지를 가리키는 URL 변형을 여러 번 발견할 때
- **THEN** 시스템은 정규화된 URL 기준으로 하나의 노드를 유지한다

#### Scenario: 사이트 경계를 넘는 링크 기록
- **WHEN** 페이지 A에서 다른 host의 페이지 B로 향하는 링크를 발견할 때
- **THEN** 시스템은 정책상 허용된 경우 페이지 B 노드와 A→B 엣지를 저장한다

#### Scenario: 샘플 fetch URL 보존
- **WHEN** 정규화된 URL과 실제 fetch된 최종 URL이 다를 때
- **THEN** 시스템은 노드 식별자와 별도로 마지막 fetch URL을 보존한다

#### Scenario: 템플릿 패턴 강제 머지 없음
- **WHEN** `/artworks/12345`와 `/artworks/67890`를 발견할 때
- **THEN** 시스템은 두 URL을 별도 URL 노드로 저장할 수 있으며, 패턴 분석은 선택적 보조 정보로만 취급한다

### Requirement: URL 패턴으로 페이지 타입을 분류한다
시스템은 발견한 URL과 fetch 결과를 바탕으로 탐색 및 수확 우선순위에 사용할 페이지 분류 신호를 계산해야 한다(SHALL). 분류 결과는 frontier 점수 산정과 Harvester 후보 선별에 사용되어야 하며(SHALL), 탐색 대상 제외 여부와는 분리되어야 한다(SHALL). 간단한 키워드 기반 allow/deny 필터와 `robots.txt` 준수는 별도 정책으로 적용되어야 한다(SHALL).

#### Scenario: 콘텐츠 후보 URL 분류
- **WHEN** URL 경로가 `artworks`, `works`, `illust`, `photos`, `posts` 등 콘텐츠 상세를 강하게 시사할 때
- **THEN** 시스템은 해당 URL을 high-value content candidate로 분류한다

#### Scenario: 탐색 허브 URL 분류
- **WHEN** URL이 listing, ranking, discover, tag, category, gallery 성격을 나타낼 때
- **THEN** 시스템은 해당 URL을 링크 발견 가치가 높은 탐색 허브로 분류한다

#### Scenario: 필터링과 분류의 책임 분리
- **WHEN** URL이 로그인, 장바구니, 광고, 정책 페이지처럼 저가치 경로를 포함할 때
- **THEN** 시스템은 분류와 별개로 필터 정책에서 제외 여부를 결정한다

## ADDED Requirements

### Requirement: Pioneer가 priority frontier로 URL을 탐색한다
Pioneer는 고정된 BFS/DFS 순서 대신 점수와 쿼리 가능한 선별 조건을 가진 priority frontier에서 다음 URL을 선택해야 한다(SHALL). frontier는 URL 점수, depth, 다음 시도 시각, 마지막 fetch 시각, fetch 오류 여부를 관리해야 하며(SHALL), 여러 worker가 동시에 소비해도 동일 URL을 중복 선점하지 않아야 한다(SHALL).

#### Scenario: 미방문 URL 우선 선점
- **WHEN** Pioneer가 다음 작업을 가져올 때
- **THEN** `last_fetched_at IS NULL` 이고 현재 시각에 처리 가능한 항목 중 우선순위가 높은 URL을 선택한다

#### Scenario: 점수 기반 선택
- **WHEN** 두 URL이 모두 처리 가능하지만 하나는 고가치 content/listing 신호를 더 많이 가질 때
- **THEN** 시스템은 더 높은 점수의 URL을 먼저 선택한다

#### Scenario: 재시도 시각 준수
- **WHEN** fetch 실패로 backoff가 걸린 URL이 frontier에 존재할 때
- **THEN** 시스템은 `next_fetch_at` 이전에는 해당 URL을 다시 선점하지 않는다

#### Scenario: 병렬 worker 중복 선점 방지
- **WHEN** 두 Pioneer worker가 동시에 같은 frontier를 소비할 때
- **THEN** 같은 URL이 동시에 두 worker에 의해 선택되지 않는다

#### Scenario: Pioneer 선별 필드 기반 인덱스 활용
- **WHEN** Pioneer가 `last_fetched_at IS NULL` 및 `next_fetch_at <= now()` 조건으로 URL을 선별할 때
- **THEN** 시스템은 해당 조건과 점수 정렬에 맞는 인덱스를 사용하여 풀스캔 없이 후보를 선택할 수 있어야 한다

### Requirement: Pioneer가 fetch snapshot과 링크 발견 결과를 저장한다
Pioneer는 URL을 fetch한 뒤 raw HTML snapshot, fetch 메타데이터, 추출 링크, 후속 frontier 갱신 결과를 저장해야 한다(SHALL). raw HTML snapshot은 영구 보관을 전제하지 않고 장기 TTL 캐시로 저장해야 하며(SHALL), TTL이 만료되면 존재하지 않는 snapshot과 동일하게 취급해야 한다(SHALL).

#### Scenario: fetch 성공 시 snapshot 저장
- **WHEN** Pioneer가 URL fetch에 성공할 때
- **THEN** 시스템은 HTML snapshot 위치, fetch 시각, 상태 코드, content type을 기록한다

#### Scenario: 링크 추출 후 frontier 갱신
- **WHEN** Pioneer가 HTML에서 링크를 추출하고 필터를 통과시킬 때
- **THEN** 시스템은 새 링크를 frontier에 upsert하고 부모 페이지에서 자식 링크로의 관계를 저장한다

#### Scenario: snapshot 저장 실패 시 상태 보존
- **WHEN** HTML fetch는 성공했지만 snapshot 저장이 실패할 때
- **THEN** 시스템은 fetch 실패와 별도로 snapshot 저장 실패를 기록하고 URL 재처리 정책을 결정할 수 있어야 한다

#### Scenario: snapshot TTL 만료
- **WHEN** 저장된 snapshot의 TTL이 만료될 때
- **THEN** 시스템은 해당 snapshot을 없는 것으로 간주하고 이후 소비자는 재사용을 기대하지 않는다

### Requirement: allow/deny 규칙과 robots 정책을 적용한다
시스템은 링크를 frontier에 추가하거나 fetch하기 전에 간단한 키워드 기반 allow/deny 규칙을 적용해야 한다(SHALL). 시스템은 대상 호스트의 `robots.txt` disallow 정책을 준수해야 한다(SHALL).

#### Scenario: deny 키워드 링크 제외
- **WHEN** URL이 login, signup, policy, cart, checkout 같은 deny 키워드를 포함할 때
- **THEN** 시스템은 해당 링크를 frontier에 추가하지 않거나 fetch 대상에서 제외한다

#### Scenario: allow 규칙이 있는 링크 통과
- **WHEN** URL이 artworks, works, illust, gallery, tag 같은 allow 성격 경로를 포함하고 다른 차단 규칙에 걸리지 않을 때
- **THEN** 시스템은 해당 링크를 frontier 후보로 유지할 수 있다

#### Scenario: robots disallow 준수
- **WHEN** 대상 호스트의 `robots.txt`가 특정 경로를 금지할 때
- **THEN** 시스템은 해당 경로를 fetch하지 않는다

#### Scenario: robots 조회 실패 시 진행
- **WHEN** `robots.txt`를 가져오거나 파싱하는 데 실패할 때
- **THEN** 시스템은 해당 실패를 기록하되 크롤링은 계속 진행한다

### Requirement: Harvester가 frontier에서 수확 대기 URL을 선별한다
Harvester는 그래프 전체를 기계적으로 순회하지 않고, 아직 searchable Pin으로 저장되지 않은 URL 중 우선순위가 높은 항목을 선택해 처리해야 한다(SHALL). 수확 여부는 URL 분류 신호, fetch 결과 메타데이터, 이전 Pin 생성 상태를 기준으로 결정해야 한다(SHALL).

#### Scenario: 미인덱싱 콘텐츠 후보 선별
- **WHEN** frontier에 아직 인덱싱되지 않은 content candidate URL이 존재할 때
- **THEN** Harvester는 해당 URL을 수확 대상으로 선택할 수 있다

#### Scenario: 이미 인덱싱된 URL 재수확 제어
- **WHEN** URL이 이미 searchable Pin으로 저장되었을 때
- **THEN** Harvester는 해당 URL을 다시 수확하지 않는다

#### Scenario: 비콘텐츠 허브 페이지 제외
- **WHEN** URL이 링크 발견용 허브 페이지로 분류되고 문서 인덱싱 가치가 낮을 때
- **THEN** Harvester는 이를 수확 대상으로 선택하지 않거나 낮은 우선순위로 둔다

#### Scenario: Harvester 선별 필드 기반 인덱스 활용
- **WHEN** Harvester가 `pin_id IS NULL`, `last_fetched_at IS NOT NULL`, `next_harvest_at <= now()` 조건으로 수확 대기 URL을 선별할 때
- **THEN** 시스템은 해당 조건과 점수 정렬에 맞는 인덱스를 사용하여 풀스캔 없이 후보를 선택할 수 있어야 한다

### Requirement: Harvester가 snapshot 우선 fetch와 HTTP fallback을 사용한다
Harvester는 문서를 추출할 때 저장된 fetch snapshot을 우선 사용해야 한다(SHALL). snapshot이 없거나 만료되었거나 사용할 수 없을 때만 HTTP를 통해 다시 fetch해야 한다(SHALL).

#### Scenario: snapshot 재사용
- **WHEN** Harvester가 처리할 URL에 유효한 HTML snapshot이 존재할 때
- **THEN** Harvester는 HTTP 요청 없이 snapshot을 사용한다

#### Scenario: snapshot 만료 시 HTTP fallback
- **WHEN** snapshot이 없거나 TTL이 만료되었을 때
- **THEN** Harvester는 HTTP fetch를 수행하고 최신 결과로 처리한다

#### Scenario: object storage 우선 fetch
- **WHEN** fetcher가 object storage와 HTTP를 모두 지원할 때
- **THEN** 시스템은 object storage fetch를 먼저 시도하고 실패 시 HTTP를 사용한다

### Requirement: Harvester가 구조화 document와 검색 인덱스를 저장한다
Harvester는 fetch된 HTML에서 searchable Pin 생성에 필요한 구조화 데이터를 추출해야 한다(SHALL). Pin은 검색 인덱스의 document 역할을 해야 하며(SHALL), 최소한 제목, 본문 텍스트 또는 설명, canonical URL, 대표 이미지 후보를 포함해야 한다(SHALL).

#### Scenario: 구조화 document 저장
- **WHEN** Harvester가 콘텐츠 페이지를 성공적으로 처리할 때
- **THEN** 시스템은 Pin 생성에 필요한 제목, 본문 텍스트 또는 설명, canonical URL, 대표 이미지 후보를 저장한다

#### Scenario: 검색 인덱스 반영
- **WHEN** Pin 생성 또는 갱신이 완료될 때
- **THEN** 시스템은 해당 Pin을 검색 가능한 document로 취급한다

#### Scenario: Pin 테이블이 검색 SSOT
- **WHEN** 시스템이 수확 결과를 영속 저장할 때
- **THEN** 별도 문서 저장소를 강제하지 않고 Pin 테이블과 그 메타데이터를 검색 인덱스의 SSOT로 사용한다

#### Scenario: 저가치 문서 스킵
- **WHEN** 본문이 비어 있거나 콘텐츠 가치가 낮다고 판정된 페이지일 때
- **THEN** 시스템은 검색 인덱스 반영을 건너뛸 수 있다

### Requirement: Harvester가 이미지 캐시 메타데이터를 저장한다
Harvester는 검색 결과에 사용할 대표 이미지가 있을 때 캐시 가능한 미디어 정보를 저장해야 한다(SHALL). 이미지 캐시는 원본 URL과 분리된 검색용 미디어 위치를 제공할 수 있어야 한다(SHALL).

#### Scenario: 대표 이미지 캐시 성공
- **WHEN** 문서에서 대표 이미지 후보를 식별하고 캐시 저장에 성공할 때
- **THEN** 시스템은 검색 인덱스가 사용할 캐시된 이미지 위치를 저장한다

#### Scenario: 이미지 캐시 실패 시 문서 유지
- **WHEN** 이미지 캐시 저장이 실패할 때
- **THEN** 문서 인덱싱은 계속되고 이미지 필드만 비워지거나 원본 후보만 유지된다

## REMOVED Requirements

### Requirement: 크롤된 URL 집합에서 가변 segment를 자동 탐지한다
**Reason**: To-Be 모델은 템플릿 패턴 노드가 아니라 URL 단위 frontier와 fetch 상태를 중심으로 동작한다.
**Migration**: 기존 pattern merge 로직은 선택적 분석 도구로 격리하고, frontier 식별자는 normalized URL로 전환한다.

### Requirement: 패턴 분석 결과를 기반으로 노드를 머지한다
**Reason**: URL 노드를 템플릿 대표 노드로 머지하면 fetch 상태, 인덱싱 상태, canonical URL 관리가 왜곡된다.
**Migration**: 기존 merge 프로세스는 비활성화하고, 필요 시 별도 reporting/analysis 용도로만 유지한다.

### Requirement: Pioneer가 DB 기존 노드와 무관하게 BFS 큐를 관리한다
**Reason**: Pioneer는 세션별 BFS 큐가 아니라 영속 frontier 상태 저장소를 사용해야 한다.
**Migration**: `visited` 인메모리 맵 중심 로직을 frontier status claim/update 모델로 교체한다.

### Requirement: MaxNodesPerSite는 새로 생성된 노드만 집계한다
**Reason**: 사이트별 quota보다 frontier 상태와 worker budget이 우선이며, cross-site 탐색에서는 per-site node quota가 핵심 제약이 아니다.
**Migration**: site quota를 host/domain budget 또는 global worker budget으로 재정의한다.

### Requirement: 크롤 완료 후 stale edge를 삭제한다
**Reason**: priority frontier 모델에서는 부분적 탐색과 비동기 재방문이 기본이므로, 단일 세션 종료 시점의 stale edge 삭제를 보장할 수 없다.
**Migration**: edge freshness는 개별 링크의 last_seen_at 기반 정리 작업으로 대체한다.

### Requirement: JavaScript 파싱 스크립트를 실행하여 콘텐츠 항목을 추출한다
**Reason**: Harvester의 주 계약은 script execution이 아니라 searchable Pin 생성에 필요한 구조화 데이터 추출로 이동한다.
**Migration**: 기존 스크립트 실행기는 특정 사이트용 보조 extractor나 fallback adapter로 축소한다.

### Requirement: DOM 헬퍼 함수를 스크립트 런타임에 주입한다
**Reason**: script runtime은 핵심 수확 계약이 아니며, 문서 추출 파이프라인의 보조 구현 세부사항으로 내려간다.
**Migration**: DOM helper 의존 코드는 legacy extractor adapter 내부로 한정한다.

### Requirement: 스크립트 실행 결과를 콘텐츠 항목 배열로 변환한다
**Reason**: 표준 산출물이 RawItem 배열이 아니라 searchable Pin 생성에 필요한 구조화 데이터로 바뀐다.
**Migration**: RawItem 중심 파이프라인은 Pin 생성용 호환 계층에서만 유지한다.
