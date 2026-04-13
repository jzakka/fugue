## ADDED Requirements

### Requirement: 사이트 메타데이터를 저장한다
시스템은 도메인, 루트 URL, 크롤 상태를 포함한 사이트 정의를 저장해야 한다.

#### Scenario: 새 사이트 등록
- **WHEN** domain과 root_url로 새 사이트를 추가할 때
- **THEN** 시스템은 대기 상태로 사이트를 등록한다

#### Scenario: Pioneer 완료 추적
- **WHEN** Pioneer가 전체 크롤을 완료할 때
- **THEN** 시스템은 사이트 상태를 완료로 갱신하고 완료 시각을 기록한다

#### Scenario: 사이트 삭제 시 그래프도 삭제됨
- **WHEN** 사이트를 삭제할 때
- **THEN** 해당 사이트의 모든 노드, 엣지, 스크립트가 자동으로 삭제된다

---

### Requirement: 그래프 노드와 엣지를 관리한다
시스템은 크롤링한 페이지를 노드로, 링크를 엣지로 저장하며 중복을 방지해야 한다.

#### Scenario: 새 URL 추가
- **WHEN** 크롤 중 URL을 발견했을 때
- **THEN** 시스템은 해당 사이트에 해당 URL이 없는 경우에만 노드를 생성한다

#### Scenario: 중복 URL 거부
- **WHEN** 이미 존재하는 URL을 다시 추가하려 할 때
- **THEN** 시스템은 중복을 거부하고 기존 노드를 유지한다

#### Scenario: 링크 관계 기록
- **WHEN** 크롤러가 페이지 A에서 페이지 B로의 링크를 발견할 때
- **THEN** 시스템은 A에서 B로의 엣지를 생성한다

#### Scenario: 중복 엣지 방지
- **WHEN** 같은 링크를 여러 번 발견했을 때
- **THEN** 시스템은 하나의 엣지만 유지한다

#### Scenario: 사이트의 모든 listing 페이지 조회
- **WHEN** 특정 사이트의 listing 타입 노드들을 조회할 때
- **THEN** 시스템은 해당하는 모든 노드를 빠르게 반환한다

---

### Requirement: 노드 방문 통계를 추적한다
시스템은 각 노드의 방문 횟수, 성공/실패 횟수, 마지막 방문 시각을 기록해야 한다.

#### Scenario: 성공적인 방문 후 통계 갱신
- **WHEN** Harvester가 노드를 방문하여 성공적으로 아이템을 추출했을 때
- **THEN** 시스템은 방문 횟수, 성공 횟수를 증가시키고 마지막 방문 시각을 갱신한다

#### Scenario: 실패한 방문 추적
- **WHEN** Harvester가 노드를 방문했으나 스크립트 실행에 실패했을 때
- **THEN** 시스템은 방문 횟수, 실패 횟수를 증가시키고 마지막 방문 시각을 갱신한다

---

### Requirement: BFS로 사이트를 탐색한다 (Pioneer)
시스템은 너비 우선 탐색으로 사이트 링크를 순회하며 설정 가능한 최대 깊이를 준수해야 한다(SHALL). BFS 탐색 로직은 페이지 조회 방법과 독립적으로 테스트 가능해야 한다(SHALL).

#### Scenario: 최대 깊이 제한 준수
- **WHEN** Pioneer가 최대 깊이 5로 루트에서 시작할 때
- **THEN** 깊이 6의 노드는 방문하지 않는다

#### Scenario: 부모 관계 추적
- **WHEN** Pioneer가 페이지 A에서 URL B를 발견할 때
- **THEN** 노드 B는 A를 부모로 하고 A보다 깊이가 1 증가한 상태로 생성된다

#### Scenario: Fetcher 인터페이스를 통한 페이지 조회
- **WHEN** BFS 탐색 중 페이지를 조회해야 할 때
- **THEN** Fetcher 인터페이스의 Fetch 메서드를 호출하여 페이지를 가져온다

#### Scenario: 테스트 시 FileFetcher 사용
- **WHEN** 단위 테스트에서 BFS 로직을 검증할 때
- **THEN** FileFetcher를 주입하여 파일 시스템 기반 fixture로 테스트한다

#### Scenario: 프로덕션 시 HTTPFetcher 사용
- **WHEN** 실제 크롤링을 수행할 때
- **THEN** HTTPFetcher를 주입하여 HTTP 요청으로 페이지를 가져온다

---

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

---

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

---

### Requirement: 파일 확장자를 제외한다
시스템은 미디어/문서 확장자로 끝나는 URL을 건너뛰어야 한다.

#### Scenario: 이미지 파일 제외
- **WHEN** URL이 .jpg, .png, .gif, .webp, .svg로 끝날 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

#### Scenario: 정적 자산 제외
- **WHEN** URL이 .css, .js, .json, .xml로 끝날 때
- **THEN** 해당 URL은 그래프에 추가되지 않는다

---

### Requirement: 페이지 타입별로 우선순위를 적용한다
시스템은 높은 우선순위의 노드 타입을 낮은 우선순위보다 먼저 처리해야 한다.

#### Scenario: listing 페이지를 먼저 처리 (Pioneer)
- **WHEN** BFS 큐에 listing 페이지와 detail 페이지가 모두 있을 때
- **THEN** listing 페이지를 detail 페이지보다 먼저 방문한다

#### Scenario: 레벨 내 우선순위 정렬 (Harvester)
- **WHEN** 깊이 1에 listing 노드 2개와 detail 노드 10개가 있을 때
- **THEN** listing 노드 2개를 먼저 처리한 후 detail 노드 10개를 처리한다

#### Scenario: 같은 타입은 발견 순서 유지
- **WHEN** 깊이 2에 detail 노드 5개가 있을 때
- **THEN** 엣지 발견 순서대로 5개 노드를 처리한다

---

### Requirement: 파싱 스크립트를 관리한다
시스템은 사이트와 페이지 타입당 하나의 파싱 스크립트를 유지하고 AI로 생성/검증해야 한다.

#### Scenario: 새 타입용 스크립트 생성
- **WHEN** AI가 특정 (사이트, 타입) 조합에 대한 스크립트를 생성할 때
- **THEN** 시스템은 해당 조합으로 스크립트를 저장한다

#### Scenario: 재생성 시 기존 스크립트 대체
- **WHEN** Pioneer가 이미 스크립트가 존재하는 (사이트, 타입)에 대해 스크립트를 재생성할 때
- **THEN** 시스템은 기존 스크립트를 새 스크립트로 대체한다

#### Scenario: AI 메타데이터 기록
- **WHEN** AI를 통해 스크립트를 생성할 때
- **THEN** 사용한 AI 모델명과 생성 비용을 함께 저장한다

#### Scenario: 스크립트 검증 통과 (유효한 스크립트 재사용)
- **WHEN** 스크립트가 최소 10개의 유효한 아이템을 추출할 때
- **THEN** 검증이 통과하고 스크립트를 재사용하며 AI 호출 없이 재사용 카운터를 증가시킨다

#### Scenario: 스크립트 검증 실패
- **WHEN** 스크립트가 10개 미만의 유효한 아이템을 추출할 때
- **THEN** 검증이 실패하고 AI를 통해 새 스크립트를 생성한다

#### Scenario: 필수 필드 검증
- **WHEN** 추출된 아이템들을 검증할 때
- **THEN** 각 아이템이 media_url과 source_url 필드를 모두 가지고 있는지 확인한다
- **AND** 두 필드 중 하나라도 비어있으면 해당 아이템은 유효하지 않은 것으로 간주한다

#### Scenario: 선택적 필드는 검증하지 않음
- **WHEN** 아이템에 title 필드가 비어있을 때
- **THEN** media_url과 source_url이 존재하면 여전히 유효한 아이템으로 간주한다

#### Scenario: 검증 결과 기록
- **WHEN** 검증이 완료될 때
- **THEN** 성공 또는 실패 카운터를 증가시키고 마지막 검증 시각을 갱신한다

#### Scenario: 실행 메트릭 갱신
- **WHEN** 스크립트가 실행되고 완료될 때
- **THEN** 실행 시간과 추출 아이템 수의 이동 평균을 갱신한다

#### Scenario: 검증 후 스크립트 연결
- **WHEN** Pioneer가 검증 후 노드에 스크립트를 할당할 때
- **THEN** 노드가 해당 스크립트를 참조한다

---

### Requirement: Harvester가 그래프를 BFS로 순회한다
시스템은 BFS 알고리즘을 사용하여 엣지를 따라 그래프를 레벨별로 순회하며, 방문한 노드를 추적해야 한다.

#### Scenario: 루트에서 BFS 시작
- **WHEN** Harvester가 시작될 때
- **THEN** 사이트의 루트 URL 노드를 큐에 추가하고 BFS 순회를 시작한다

#### Scenario: 엣지 기반 순회
- **WHEN** 노드를 처리할 때
- **THEN** 시스템은 해당 노드에서 연결된 모든 링크를 조회하여 자식 노드 목록을 가져온다

#### Scenario: 레벨별 순회
- **WHEN** 루트 노드(깊이 0)가 3개의 자식 노드를 가질 때
- **THEN** 루트 노드를 먼저 처리한 후 3개의 자식 노드(깊이 1)를 순서대로 처리한다

#### Scenario: 방문한 노드는 재방문하지 않음
- **WHEN** 노드 A가 이미 방문되었고 다른 경로에서 A를 다시 발견할 때
- **THEN** 노드 A를 큐에 추가하지 않고 건너뛴다

#### Scenario: 순환 그래프에서 재방문 차단
- **WHEN** 노드 A → B → C → A 순환 구조에서 A를 다시 발견할 때
- **THEN** A는 이미 방문되었으므로 큐에 추가하지 않는다

#### Scenario: 서로 다른 경로에서의 중복 발견
- **WHEN** 노드 A에서 C로, 노드 B에서도 C로 가는 링크가 있을 때
- **THEN** C는 처음 발견 시에만 큐에 추가되고 두 번째 발견 시에는 건너뛴다

#### Scenario: 리프 노드 처리
- **WHEN** 노드에 연결된 자식 노드가 없을 때
- **THEN** 자식 노드 추가 없이 다음 노드로 진행한다

#### Scenario: 루트 노드 부재 시 오류
- **WHEN** 루트 URL에 해당하는 노드가 없을 때
- **THEN** 오류를 반환하고 Harvester 실행을 중단한다

---

### Requirement: Harvester가 스크립트를 실행하여 아이템을 추출한다
시스템은 HTML을 가져와서 연결된 스크립트를 로드하고 실행하여 아이템을 추출해야 한다.

#### Scenario: 성공적인 실행
- **WHEN** 노드에 스크립트가 연결되어 있고 실행이 성공할 때
- **THEN** 추출된 아이템들을 파이프라인으로 전달한다

#### Scenario: 스크립트 누락
- **WHEN** 노드에 스크립트가 연결되어 있지 않을 때
- **THEN** 해당 노드를 건너뛰고 실패 노드 수를 증가시킨다

#### Scenario: 스크립트 실행 오류
- **WHEN** 스크립트 실행이 오류를 반환할 때
- **THEN** 오류를 로깅하고 실패 노드 수와 해당 노드의 실패 횟수를 증가시킨다

#### Scenario: 파이프라인 처리
- **WHEN** 스크립트가 30개의 아이템을 추출했을 때
- **THEN** 각 아이템은 중복 체크, 미디어 다운로드, 태그 매칭, 핀 생성 단계를 거친다

---

### Requirement: Pioneer 실행 생명주기를 추적한다
시스템은 각 Pioneer 실행의 시작 시각, 완료 시각, 상태, 통계를 기록해야 한다.

#### Scenario: 실행 시작 기록
- **WHEN** Pioneer가 실행을 시작할 때
- **THEN** 실행 중 상태와 시작 시각으로 실행 레코드를 생성한다

#### Scenario: 실행 완료 기록
- **WHEN** Pioneer가 성공적으로 완료될 때
- **THEN** 상태를 완료로 갱신하고 완료 시각을 기록한다

#### Scenario: 실행 실패 기록
- **WHEN** Pioneer가 치명적 오류를 만났을 때
- **THEN** 상태를 실패로 갱신하고 오류 메시지를 기록한다

#### Scenario: 탐색 카운터 증가
- **WHEN** Pioneer가 실행 중 노드를 발견하거나 스크립트를 생성/재사용할 때
- **THEN** 해당하는 카운터를 증가시킨다

#### Scenario: AI 비용 누적
- **WHEN** Pioneer가 AI API를 호출할 때마다
- **THEN** 호출 횟수와 비용을 누적하여 기록한다

---

### Requirement: Harvester 실행 생명주기를 추적한다
시스템은 각 Harvester 실행의 시작, 완료, 상태, 통계를 기록해야 한다.

#### Scenario: Harvester 시작 기록
- **WHEN** Harvester가 시작될 때
- **THEN** 실행 중 상태와 시작 시각으로 실행 레코드를 생성한다

#### Scenario: 추출 메트릭 기록
- **WHEN** Harvester 실행이 완료될 때
- **THEN** 방문 노드 수, 성공/실패 수, 추출 아이템 수, 중복 제거 수, 생성 핀 수를 기록한다

#### Scenario: 실행 이력 조회
- **WHEN** 특정 사이트의 최근 30일 실행 이력을 조회할 때
- **THEN** 모든 실행의 타임스탬프, 통계, 오류를 반환한다

---

### Requirement: CLI root command provides usage guidance
The root command (executed without subcommands) SHALL display help information including available subcommands and basic usage.

#### Scenario: User runs fuguebot without arguments
- **WHEN** user executes the bot binary without any subcommands or arguments
- **THEN** system displays help text listing available subcommands (pioneer, harvester) and usage examples

---

### Requirement: Site name resolution supports both short names and domains
The system SHALL accept both short source names (e.g., "unsplash", "fma") and full domain names (e.g., "unsplash.com") as site identifiers, resolving short names to domains via a registry.

#### Scenario: User provides short source name
- **WHEN** user executes a command with a short source name (e.g., "unsplash")
- **THEN** system resolves it to the corresponding domain (e.g., "unsplash.com") using the source registry

#### Scenario: User provides full domain name
- **WHEN** user executes a command with a full domain name (e.g., "unsplash.com")
- **THEN** system uses the domain directly without resolution

#### Scenario: User provides unknown source name
- **WHEN** user executes a command with a source name not in the registry and not a valid domain format
- **THEN** system displays an error message listing available source names and exits with error code

---

### Requirement: Pioneer subcommand executes site exploration
The pioneer subcommand SHALL initialize and run the Pioneer crawler for a specified site.

#### Scenario: User runs pioneer with valid site name
- **WHEN** user executes `pioneer <site>` with a site name (short or domain) that resolves to a domain existing in the database
- **THEN** system resolves the site name to domain, initializes database and storage connections, creates a Pioneer instance with AI client and necessary repositories, and executes the pioneer crawl for that site

#### Scenario: User runs pioneer with non-existent site
- **WHEN** user executes `pioneer <site>` with a site name that resolves to a domain not found in the database
- **THEN** system displays an error message indicating the site domain was not found in the database and exits with non-zero code

#### Scenario: User runs pioneer without site argument
- **WHEN** user executes `pioneer` without providing a site name
- **THEN** system displays usage help for the pioneer command and exits with error code

---

### Requirement: Harvester subcommand executes content extraction
The harvester subcommand SHALL initialize and run the Harvester crawler for a specified site.

#### Scenario: User runs harvester with valid site name
- **WHEN** user executes `harvester <site>` with a site name (short or domain) that resolves to a domain existing in the database
- **THEN** system resolves the site name to domain, initializes database and storage connections, creates a Harvester instance with script executor and pipeline, and executes the harvester crawl for that site

#### Scenario: User runs harvester with non-existent site
- **WHEN** user executes `harvester <site>` with a site name that resolves to a domain not found in the database
- **THEN** system displays an error message indicating the site domain was not found in the database and exits with non-zero code

#### Scenario: User runs harvester without site argument
- **WHEN** user executes `harvester` without providing a site name
- **THEN** system displays usage help for the harvester command and exits with error code

---

### Requirement: Makefile provides convenient shortcuts
The Makefile SHALL provide `pioneer` and `harvester` targets that invoke the corresponding CLI commands.

#### Scenario: User runs make pioneer with SITE variable
- **WHEN** user executes `make pioneer SITE=<site>`
- **THEN** system runs `go run cmd/bot/main.go pioneer <site>`

#### Scenario: User runs make pioneer without SITE variable
- **WHEN** user executes `make pioneer` without providing SITE variable
- **THEN** system displays usage message indicating SITE variable is required and exits with error

#### Scenario: User runs make harvester with SITE variable
- **WHEN** user executes `make harvester SITE=<site>`
- **THEN** system runs `go run cmd/bot/main.go harvester <site>`

#### Scenario: User runs make harvester without SITE variable
- **WHEN** user executes `make harvester` without providing SITE variable
- **THEN** system displays usage message indicating SITE variable is required and exits with error

---

### Requirement: Infrastructure initialization is shared across commands
Both pioneer and harvester commands SHALL initialize database and storage connections using environment variables with sensible defaults.

#### Scenario: Commands use environment variables for configuration
- **WHEN** either pioneer or harvester command is executed
- **THEN** system reads DATABASE_URL, S3_ENDPOINT, S3_REGION, S3_BUCKET, S3_ACCESS_KEY, S3_SECRET_KEY, and S3_PUBLIC_URL from environment variables or uses documented default values

#### Scenario: Database connection fails
- **WHEN** either pioneer or harvester command is executed and database connection fails
- **THEN** system logs a clear error message and exits with non-zero code before attempting to run the crawler

#### Scenario: Storage connection fails
- **WHEN** either pioneer or harvester command is executed and storage connection fails
- **THEN** system logs a clear error message and exits with non-zero code before attempting to run the crawler

---

### Requirement: Link 타입은 URL과 DOM 구조 정보를 포함해야 한다

Link 타입은 URL 문자열과 DOM 조상 경로 정보를 함께 보유해야 한다(SHALL). Selector 구조체는 HTML 요소의 태그명, ID, Class 속성을 표현해야 한다(SHALL). 이 타입들은 crawler 패키지 내에 정의되어야 한다(MUST).

#### Scenario: Selector 구조체 정의

- **WHEN** Selector가 정의될 때
- **THEN** HTML 요소의 태그명, ID, Class 속성을 각각 표현하는 세 개의 공개 필드를 가져야 한다(MUST)

#### Scenario: Link 구조체 정의

- **WHEN** Link가 정의될 때
- **THEN** URL을 나타내는 필드와 DOM 조상 경로를 나타내는 Selector 목록 필드를 가져야 한다(MUST)
- **THEN** DOM 조상 경로는 루트에서 `<a>` 태그까지의 순서여야 한다(SHALL)

#### Scenario: Link 타입의 제로값 유효성

- **WHEN** Link의 제로값이 생성될 때
- **THEN** URL은 빈 문자열이고 조상 경로는 비어 있어야 한다(MUST)
- **THEN** 패닉 없이 정상 동작해야 한다(SHALL)

---

### Requirement: LinkFilter는 링크 목록을 받아 필터링된 목록을 반환하는 단일 연산을 정의해야 한다

LinkFilter 인터페이스는 링크 목록을 받아 필터링된 목록을 반환하는 단일 메서드를 정의해야 한다(MUST). 이 인터페이스는 bot 패키지 내에 정의되어야 한다(MUST).

#### Scenario: LinkFilter 인터페이스의 연산 계약

- **WHEN** LinkFilter 인터페이스가 정의될 때
- **THEN** Link 목록을 입력받아 필터링된 Link 목록을 반환하는 메서드 하나만 가져야 한다(MUST)
- **THEN** crawler 패키지의 Link 타입을 사용해야 한다(SHALL)

#### Scenario: 빈 슬라이스 입력 계약

- **WHEN** 빈 링크 목록이 필터에 전달될 때
- **THEN** 구현체는 빈 목록 또는 nil을 반환해야 한다(SHALL) -- 패닉이 발생해서는 안 된다(MUST NOT)

---

### Requirement: FilterChain은 여러 필터를 순차 적용하는 복합 구조를 제공해야 한다

FilterChain은 내부에 LinkFilter 목록을 보유해야 한다(MUST). 가변 인자로 필터들을 받는 생성자와 링크 목록에 필터를 순차 적용하는 메서드를 제공해야 한다(MUST). 순차 적용 시 각 필터의 출력이 다음 필터의 입력이 되어야 한다(SHALL).

#### Scenario: FilterChain 생성

- **WHEN** 여러 필터를 인자로 FilterChain이 생성될 때
- **THEN** 전달된 필터들이 전달 순서대로 내부에 저장되어야 한다(MUST)

#### Scenario: FilterChain 순차 적용

- **WHEN** FilterChain에 링크 목록이 전달될 때
- **THEN** 첫 번째 필터의 결과가 두 번째 필터의 입력으로 전달되어야 한다(MUST)
- **THEN** 마지막 필터의 출력이 최종 결과로 반환되어야 한다(MUST)

#### Scenario: 빈 FilterChain

- **WHEN** 필터 없이 생성된 체인에 링크 목록이 전달될 때
- **THEN** 입력 링크 목록이 그대로 반환되어야 한다(MUST)

#### Scenario: nil 입력 처리

- **WHEN** nil이 링크 목록으로 전달될 때
- **THEN** 패닉 없이 nil 또는 빈 목록을 반환해야 한다(MUST)

#### Scenario: nil 필터 원소 처리

- **WHEN** FilterChain에 nil 필터 원소가 포함될 때
- **THEN** nil 필터를 건너뛰고 나머지 필터를 정상 적용해야 한다(SHALL)
