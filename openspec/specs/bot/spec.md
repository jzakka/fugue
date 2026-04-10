## ADDED Requirements

### Requirement: 사이트 메타데이터를 저장한다
시스템은 도메인, 루트 URL, 크롤 상태를 포함한 사이트 정의를 저장해야 한다.

#### Scenario: 새 사이트 등록
- **WHEN** domain과 root_url로 새 사이트를 추가할 때
- **THEN** 시스템은 대기 상태로 사이트를 등록한다

#### Scenario: Pioneer 완료 추적
- **WHEN** Pioneer가 전체 크롤을 완료할 때
- **THEN** 시스템은 사이트 상태를 완료로 갱신하고 완료 시각을 기록한다

### Requirement: URL 중복을 방지한다
시스템은 동일한 사이트 내에서 같은 URL을 중복 저장하지 않아야 한다.

#### Scenario: 새 URL 추가
- **WHEN** 크롤 중 URL을 발견했을 때
- **THEN** 시스템은 해당 사이트에 해당 URL이 없는 경우에만 노드를 생성한다

#### Scenario: 중복 URL 거부
- **WHEN** 이미 존재하는 URL을 다시 추가하려 할 때
- **THEN** 시스템은 중복을 거부하고 기존 노드를 유지한다

### Requirement: 링크 관계를 저장한다
시스템은 페이지 간 링크 관계를 방향성 있게 저장해야 한다.

#### Scenario: 링크 관계 기록
- **WHEN** 크롤러가 페이지 A에서 페이지 B로의 링크를 발견할 때
- **THEN** 시스템은 A에서 B로의 엣지를 생성한다

#### Scenario: 중복 엣지 방지
- **WHEN** 같은 링크를 여러 번 발견했을 때
- **THEN** 시스템은 하나의 엣지만 유지한다

### Requirement: 사이트와 타입별로 노드를 조회한다
시스템은 특정 사이트의 특정 페이지 타입 노드들을 효율적으로 조회할 수 있어야 한다.

#### Scenario: 사이트의 모든 listing 페이지 조회
- **WHEN** 특정 사이트의 listing 타입 노드들을 조회할 때
- **THEN** 시스템은 해당하는 모든 노드를 빠르게 반환한다

### Requirement: 노드 방문 통계를 추적한다
시스템은 각 노드의 방문 횟수, 성공/실패 횟수, 마지막 방문 시각을 기록해야 한다.

#### Scenario: 성공적인 방문 후 통계 갱신
- **WHEN** Harvester가 노드를 방문하여 성공적으로 아이템을 추출했을 때
- **THEN** 시스템은 방문 횟수, 성공 횟수를 증가시키고 마지막 방문 시각을 갱신한다

#### Scenario: 실패한 방문 추적
- **WHEN** Harvester가 노드를 방문했으나 스크립트 실행에 실패했을 때
- **THEN** 시스템은 방문 횟수, 실패 횟수를 증가시키고 마지막 방문 시각을 갱신한다

### Requirement: 사이트 삭제 시 관련 데이터를 함께 삭제한다
시스템은 사이트 삭제 시 해당 사이트의 모든 노드, 엣지, 스크립트를 자동으로 삭제해야 한다.

#### Scenario: 사이트 삭제 시 그래프도 삭제됨
- **WHEN** 사이트를 삭제할 때
- **THEN** 해당 사이트의 모든 노드, 엣지, 스크립트가 자동으로 삭제된다
## ADDED Requirements

### Requirement: BFS로 사이트를 탐색한다
시스템은 너비 우선 탐색으로 사이트 링크를 순회하며 설정 가능한 최대 깊이를 준수해야 한다.

#### Scenario: 최대 깊이 제한 준수
- **WHEN** Pioneer가 최대 깊이 5로 루트에서 시작할 때
- **THEN** 깊이 6의 노드는 방문하지 않는다

#### Scenario: 부모 관계 추적
- **WHEN** Pioneer가 페이지 A에서 URL B를 발견할 때
- **THEN** 노드 B는 A를 부모로 하고 A보다 깊이가 1 증가한 상태로 생성된다

### Requirement: URL 패턴으로 페이지 타입을 분류한다
시스템은 발견한 URL을 키워드 매칭을 통해 타입(listing, gallery, detail, category, skip)으로 분류해야 한다.

#### Scenario: listing 페이지 분류
- **WHEN** URL이 'trending', 'popular', 'hot', 'featured', 'recent' 키워드를 포함할 때
- **THEN** 노드 타입이 'listing'으로 설정된다

#### Scenario: detail 페이지 분류
- **WHEN** URL이 숫자 ID 패턴을 가지고 고우선순위 키워드가 없을 때
- **THEN** 노드 타입이 'detail'로 설정된다

#### Scenario: 불필요한 URL 제외
- **WHEN** URL이 'ad', 'popup', 'login', 'signup', 'cart'를 포함할 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

### Requirement: 엄격한 도메인 경계를 적용한다
시스템은 루트 도메인과 일치하는 링크만 따라가야 하며 서브도메인은 제외해야 한다.

#### Scenario: 동일 도메인 허용
- **WHEN** 링크가 dribbble.com → www.dribbble.com일 때
- **THEN** 정규화된 도메인이 일치하여 링크를 따라간다

#### Scenario: 서브도메인 차단
- **WHEN** 링크가 dribbble.com → ads.dribbble.com일 때
- **THEN** 서브도메인이 달라 링크를 거부한다

#### Scenario: 외부 도메인 차단
- **WHEN** 링크가 dribbble.com → twitter.com일 때
- **THEN** 도메인이 달라 링크를 거부한다

### Requirement: 파일 확장자를 제외한다
시스템은 미디어/문서 확장자로 끝나는 URL을 건너뛰어야 한다.

#### Scenario: 이미지 파일 제외
- **WHEN** URL이 .jpg, .png, .gif, .webp, .svg로 끝날 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

#### Scenario: 정적 자산 제외
- **WHEN** URL이 .css, .js, .json, .xml로 끝날 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

### Requirement: 페이지 타입별로 우선순위를 적용한다
시스템은 높은 우선순위의 노드 타입을 낮은 우선순위보다 먼저 처리해야 한다.

#### Scenario: listing 페이지를 먼저 처리
- **WHEN** BFS 큐에 listing 페이지와 detail 페이지가 모두 있을 때
- **THEN** listing 페이지를 detail 페이지보다 먼저 방문한다

### Requirement: 유효한 스크립트를 재사용한다
시스템은 기존 스크립트를 검증하여 검증이 통과하면 AI 생성을 건너뛰어야 한다.

#### Scenario: 스크립트 검증 통과
- **WHEN** 기존 스크립트가 페이지의 예상 아이템 중 70% 이상을 추출할 때
- **THEN** 스크립트를 재사용하고 AI 호출 없이 재사용 카운터를 증가시킨다

#### Scenario: 스크립트 검증 실패
- **WHEN** 기존 스크립트가 예상 아이템의 70% 미만을 추출할 때
- **THEN** AI를 통해 새 스크립트를 생성하고 기존 스크립트를 대체한다

### Requirement: AI로 파싱 스크립트를 생성한다
시스템은 페이지 HTML과 URL을 AI에 전달하여 파싱 스크립트를 생성해야 한다.

#### Scenario: 새 노드 타입용 스크립트 생성
- **WHEN** 특정 (사이트, 노드타입) 조합에 대한 스크립트가 없을 때
- **THEN** AI를 호출하여 스크립트를 생성하고 비용을 추적한다

### Requirement: Pioneer 실행 통계를 기록한다
시스템은 각 Pioneer 실행의 메트릭을 기록해야 한다.

#### Scenario: 실행 완료 기록
- **WHEN** Pioneer 실행이 완료될 때
- **THEN** 발견한 노드 수, 생성한 스크립트 수, 재사용한 스크립트 수, AI 비용을 기록한다
## ADDED Requirements

### Requirement: 매 실행마다 전체 그래프를 순회한다
시스템은 이전 방문 상태와 무관하게 그래프의 모든 노드를 방문해야 한다.

#### Scenario: 전체 순회
- **WHEN** Harvester가 시작될 때
- **THEN** 해당 사이트의 모든 노드를 방문 대상으로 큐에 추가한다

### Requirement: 타입 우선순위로 노드를 정렬한다
시스템은 중복 제거 효율성을 위해 listing/gallery 노드를 detail 노드보다 먼저 처리해야 한다.

#### Scenario: listing 페이지를 먼저 방문
- **WHEN** 그래프에 50개의 listing 노드와 400개의 detail 노드가 있을 때
- **THEN** listing 노드를 detail 노드보다 먼저 방문한다

### Requirement: 저장된 스크립트를 실행한다
시스템은 HTML을 가져와서 연결된 스크립트를 로드하고 실행해야 한다.

#### Scenario: 성공적인 실행
- **WHEN** 노드에 스크립트가 연결되어 있고 실행이 성공할 때
- **THEN** 추출된 아이템들을 파이프라인으로 전달한다

#### Scenario: 스크립트 누락
- **WHEN** 노드에 스크립트가 연결되어 있지 않을 때
- **THEN** 해당 노드를 건너뛰고 실패 노드 수를 증가시킨다

#### Scenario: 스크립트 실행 오류
- **WHEN** 스크립트 실행이 오류를 반환할 때
- **THEN** 오류를 로깅하고 실패 노드 수와 해당 노드의 실패 횟수를 증가시킨다

### Requirement: 노드 실행 통계를 갱신한다
시스템은 실행 결과에 따라 성공 또는 실패 횟수를 증가시켜야 한다.

#### Scenario: 성공 시 갱신
- **WHEN** 스크립트가 성공적으로 실행될 때
- **THEN** 노드의 성공 횟수를 증가시키고 마지막 방문 시각을 갱신한다

#### Scenario: 실패 시 갱신
- **WHEN** 스크립트 실행이 실패할 때
- **THEN** 노드의 실패 횟수를 증가시키고 마지막 방문 시각을 갱신한다

### Requirement: 기존 파이프라인으로 아이템을 처리한다
시스템은 추출된 아이템을 중복 제거, 다운로드, 태그 매칭, Pin 생성 파이프라인으로 전달해야 한다.

#### Scenario: 파이프라인 처리
- **WHEN** 스크립트가 30개의 아이템을 추출했을 때
- **THEN** 각 아이템은 중복 체크, 미디어 다운로드, 태그 매칭, 핀 생성 단계를 거친다

### Requirement: Harvester 실행 통계를 기록한다
시스템은 방문한 노드 수, 추출한 아이템 수, 생성한 핀 수를 실행별로 기록해야 한다.

#### Scenario: 실행 메트릭 기록
- **WHEN** Harvester 실행이 완료될 때
- **THEN** 방문 노드 수, 추출 아이템 수, 생성 핀 수, 중복 제거 수를 기록한다
## ADDED Requirements

### Requirement: 사이트와 페이지 타입당 하나의 스크립트를 유지한다
시스템은 각 사이트의 페이지 타입별로 하나의 파싱 스크립트만 유지해야 한다.

#### Scenario: 새 타입용 스크립트 생성
- **WHEN** AI가 특정 (사이트, 타입) 조합에 대한 스크립트를 생성할 때
- **THEN** 시스템은 해당 조합으로 스크립트를 저장한다

#### Scenario: 재생성 시 기존 스크립트 대체
- **WHEN** Pioneer가 이미 스크립트가 존재하는 (사이트, 타입)에 대해 스크립트를 재생성할 때
- **THEN** 시스템은 기존 스크립트를 새 스크립트로 대체한다

### Requirement: AI 생성 메타데이터를 추적한다
시스템은 사용한 AI 모델과 스크립트별 생성 비용을 기록해야 한다.

#### Scenario: AI 메타데이터 기록
- **WHEN** AI를 통해 스크립트를 생성할 때
- **THEN** 사용한 AI 모델명과 생성 비용을 함께 저장한다

### Requirement: 스크립트를 검증한다
시스템은 예상 아이템 수를 추정하고 스크립트를 실행하여 성공률을 계산해야 한다.

#### Scenario: 검증 통과
- **WHEN** 페이지에 약 30개의 이미지가 있고 스크립트가 25개를 추출했을 때
- **THEN** 성공률이 70% 이상이므로 검증이 통과한다

#### Scenario: 검증 실패
- **WHEN** 페이지에 약 30개의 이미지가 있고 스크립트가 15개를 추출했을 때
- **THEN** 성공률이 70% 미만이므로 검증이 실패한다

### Requirement: 검증 통계를 추적한다
시스템은 검증 성공 또는 실패 횟수를 기록해야 한다.

#### Scenario: 검증 결과 기록
- **WHEN** 검증이 완료될 때
- **THEN** 성공 또는 실패 카운터를 증가시키고 마지막 검증 시각을 갱신한다

### Requirement: 실행 성능을 추적한다
시스템은 스크립트의 평균 실행 시간과 평균 추출 아이템 수를 기록해야 한다.

#### Scenario: 실행 메트릭 갱신
- **WHEN** 스크립트가 실행되고 완료될 때
- **THEN** 실행 시간과 추출 아이템 수의 이동 평균을 갱신한다

### Requirement: 스크립트를 노드에 연결한다
시스템은 노드가 스크립트를 참조할 수 있도록 해야 한다.

#### Scenario: 검증 후 스크립트 연결
- **WHEN** Pioneer가 검증 후 노드에 스크립트를 할당할 때
- **THEN** 노드가 해당 스크립트를 참조한다
## ADDED Requirements

### Requirement: Pioneer 실행 생명주기를 추적한다
시스템은 각 Pioneer 실행의 시작 시각, 완료 시각, 상태를 기록해야 한다.

#### Scenario: 실행 시작 기록
- **WHEN** Pioneer가 실행을 시작할 때
- **THEN** 실행 중 상태와 시작 시각으로 실행 레코드를 생성한다

#### Scenario: 실행 완료 기록
- **WHEN** Pioneer가 성공적으로 완료될 때
- **THEN** 상태를 완료로 갱신하고 완료 시각을 기록한다

#### Scenario: 실행 실패 기록
- **WHEN** Pioneer가 치명적 오류를 만났을 때
- **THEN** 상태를 실패로 갱신하고 오류 메시지를 기록한다

### Requirement: Pioneer 탐색 통계를 추적한다
시스템은 발견한 노드, 갱신한 노드, 생성/재사용한 스크립트 수를 세어야 한다.

#### Scenario: 탐색 카운터 증가
- **WHEN** Pioneer가 실행 중 노드를 발견하거나 스크립트를 생성/재사용할 때
- **THEN** 해당하는 카운터를 증가시킨다

### Requirement: Pioneer AI 비용을 추적한다
시스템은 모든 AI API 호출과 비용을 실행별로 합산해야 한다.

#### Scenario: AI 비용 누적
- **WHEN** Pioneer가 AI API를 호출할 때마다
- **THEN** 호출 횟수와 비용을 누적하여 기록한다

### Requirement: Harvester 실행 생명주기를 추적한다
시스템은 각 Harvester 실행의 시작, 완료, 상태를 기록해야 한다.

#### Scenario: Harvester 시작 기록
- **WHEN** Harvester가 시작될 때
- **THEN** 실행 중 상태와 시작 시각으로 실행 레코드를 생성한다

### Requirement: Harvester 추출 통계를 추적한다
시스템은 방문한 노드(성공/실패), 추출한 아이템, 중복 제거, 생성한 핀 수를 세어야 한다.

#### Scenario: 추출 메트릭 기록
- **WHEN** Harvester 실행이 완료될 때
- **THEN** 방문 노드 수, 성공/실패 수, 추출 아이템 수, 중복 제거 수, 생성 핀 수를 기록한다

### Requirement: 이력 분석을 지원한다
시스템은 추세 분석과 디버깅을 위해 모든 실행 기록을 보존해야 한다.

#### Scenario: 실행 이력 조회
- **WHEN** 특정 사이트의 최근 30일 실행 이력을 조회할 때
- **THEN** 모든 실행의 타임스탬프, 통계, 오류를 반환한다
