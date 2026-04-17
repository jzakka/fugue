## ADDED Requirements

### Requirement: Pioneer는 ParseLinks 후 FilterLinks를 거쳐 Enqueue한다
Pioneer Run 루프는 각 페이지를 fetch한 뒤 `ParseLinks`로 추출한 링크 목록을 **반드시 FilterLinks(필터 체인)에 통과**시킨 후 결과만 URLPriorityQueue에 `Enqueue`해야 한다(SHALL). FilterLinks를 우회하여 Enqueue하는 경로가 있어서는 안 된다(MUST NOT).

#### Scenario: 정상 플로우
- **WHEN** Pioneer가 페이지 컨텐츠에서 `ParseLinks`로 링크 목록을 얻을 때
- **THEN** 해당 목록은 `FilterLinks`(필터 체인)에 입력되고, 체인의 최종 출력만 큐에 Enqueue된다

#### Scenario: 필터 우회 금지
- **WHEN** 구현이 `ParseLinks`의 출력을 필터 체인 없이 직접 Enqueue하려 할 때
- **THEN** 해당 구현은 본 스펙을 위반하며 허용되지 않는다

#### Scenario: 빈 결과 처리
- **WHEN** 필터 체인이 모든 링크를 걸러내어 빈 목록을 반환할 때
- **THEN** Pioneer는 에러 없이 다음 Dequeue로 진행한다

---

### Requirement: 필터 체인은 고정된 순서로 필터를 적용한다
Pioneer의 필터 체인은 다음 순서를 SHALL: (1) Domain allow/deny, (2) Extension, (3) PathPattern, (4) Robots.txt, (5) CanonicalDedup. 이 순서는 값이 싼 필터를 앞에, 네트워크/해시 계산이 있는 값비싼 필터를 뒤에 배치하기 위해 고정되어야 한다(SHALL).

#### Scenario: 정상 순서 적용
- **WHEN** 링크 목록이 필터 체인에 진입할 때
- **THEN** 링크는 Domain → Extension → PathPattern → Robots.txt → CanonicalDedup 순서로 통과한다

#### Scenario: 앞 단계에서 탈락한 링크는 뒤 단계에 도달하지 않는다
- **WHEN** 어떤 링크가 ExtensionFilter에서 제거될 때
- **THEN** 해당 링크는 RobotsFilter나 CanonicalDedup에 도달하지 않으며, robots.txt 조회 비용도 유발하지 않는다

---

### Requirement: RobotsFilter는 robots.txt를 존중하여 URL을 필터링한다
RobotsFilter는 LinkFilter 인터페이스를 구현해야 하며(SHALL), 각 링크의 호스트에 대한 robots.txt를 조회하여 `FugueBot` User-agent(없으면 `*`)의 Disallow 규칙에 매칭되는 URL을 제거해야 한다(SHALL).

#### Scenario: Disallow에 매칭되면 차단
- **WHEN** 링크의 경로가 해당 호스트 robots.txt의 Disallow 규칙에 매칭될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: Allow/Disallow 모두 없으면 통과
- **WHEN** robots.txt에 FugueBot 또는 `*`에 대한 Disallow 규칙이 없을 때
- **THEN** 해당 링크는 필터를 통과한다

#### Scenario: FugueBot 규칙이 우선, 없으면 `*` fallback
- **WHEN** robots.txt에 `User-agent: FugueBot` 블록이 존재할 때
- **THEN** 해당 블록의 규칙을 사용하고 `*` 블록은 무시된다
- **WHEN** `FugueBot` 블록이 없을 때
- **THEN** `User-agent: *` 블록의 규칙을 사용한다

---

### Requirement: RobotsFilter는 lazy fetch와 호스트별 24시간 캐시를 사용한다
RobotsFilter는 호스트별로 robots.txt를 **최초 필요 시점에만 fetch**해야 하며(SHALL), 파싱 결과를 호스트별 인메모리 맵에 캐시해야 한다(SHALL). 캐시 TTL은 **24시간**이어야 하며(SHALL), TTL 경과 후 다음 접근에 재조회해야 한다(SHALL).

#### Scenario: 최초 접근 시 fetch
- **WHEN** 호스트 `example.com`에 대한 링크가 RobotsFilter에 처음 도달할 때
- **THEN** RobotsFilter는 `https://example.com/robots.txt`를 fetch하고 결과를 캐시한다

#### Scenario: 캐시 적중
- **WHEN** 같은 호스트에 대한 두 번째 링크가 24시간 이내에 도달할 때
- **THEN** RobotsFilter는 새로운 fetch 없이 캐시된 규칙을 사용한다

#### Scenario: TTL 만료 후 재조회
- **WHEN** 캐시 엔트리가 저장된 지 24시간을 초과한 뒤 같은 호스트의 링크가 도달할 때
- **THEN** RobotsFilter는 robots.txt를 다시 fetch하여 캐시를 갱신한다

---

### Requirement: RobotsFilter는 fetch 실패 시 fail-open한다
RobotsFilter는 robots.txt fetch가 네트워크 오류, 타임아웃, 5xx 응답 등으로 실패하거나 응답이 404일 때 **모든 링크를 허용(fail-open)**해야 한다(SHALL). 실패 상태 역시 TTL과 함께 캐시하여 연속 재시도로 인한 폭주를 방지해야 한다(SHALL).

#### Scenario: 네트워크 오류 시 fail-open
- **WHEN** 호스트의 robots.txt fetch가 타임아웃되거나 네트워크 오류로 실패할 때
- **THEN** 해당 호스트의 모든 링크는 RobotsFilter를 통과한다

#### Scenario: 404 응답은 "규칙 없음"으로 해석
- **WHEN** robots.txt가 404로 응답할 때
- **THEN** 해당 호스트에는 제한이 없는 것으로 간주하여 모든 링크가 통과한다

#### Scenario: 5xx 응답 시 fail-open 상태 캐시
- **WHEN** robots.txt가 5xx로 응답할 때
- **THEN** 해당 호스트는 fail-open 상태로 캐시되며, TTL 이내에는 재시도하지 않는다

---

### Requirement: RobotsFilter는 Crawl-delay를 호스트별로 surface한다
RobotsFilter는 robots.txt에서 `Crawl-delay` 지시어를 파싱하여(SHALL), 호스트별로 초 단위 값을 외부에서 조회 가능한 형태로 노출해야 한다(SHALL). 이 값은 `scheduler-host-token-bucket`이 host bucket rate 계산에 사용한다. 본 스펙은 값의 노출까지만 정의하며 bucket 측 해석은 해당 capability가 담당한다.

#### Scenario: Crawl-delay 파싱 및 노출
- **WHEN** robots.txt에 `Crawl-delay: 5`가 포함될 때
- **THEN** RobotsFilter는 해당 호스트에 대해 `5`초를 Crawl-delay 값으로 기록하고 scheduler가 조회할 수 있게 노출한다

#### Scenario: Crawl-delay 미지정 시 값 없음
- **WHEN** robots.txt에 Crawl-delay가 명시되지 않을 때
- **THEN** 해당 호스트의 Crawl-delay는 "없음" 상태로 유지되며 scheduler는 기본 rate를 사용한다

#### Scenario: 파싱 불가능한 Crawl-delay 무시
- **WHEN** Crawl-delay 값이 정수/실수로 파싱되지 않을 때
- **THEN** 해당 값은 무시되고 "없음"과 동일하게 취급된다

---

## MODIFIED Requirements

### Requirement: DomainFilter가 Allow/Deny 키워드로 도메인을 필터링하며 교차 사이트 크롤을 기본 허용한다
DomainFilter는 LinkFilter 인터페이스를 구현하며(SHALL), **Allow 키워드 리스트**와 **Deny 키워드 리스트**를 받아 링크 호스트에 대한 substring 매칭으로 필터링해야 한다(SHALL).

- Deny 리스트에 매칭되는 호스트는 항상 차단한다(SHALL).
- Allow 리스트가 비어 있으면 Deny에 걸리지 않은 모든 호스트를 통과시킨다(SHALL). 즉 **교차 사이트 크롤을 기본 허용**한다.
- Allow 리스트가 비어 있지 않으면, Allow에 매칭되는 호스트만 통과시킨다(SHALL).
- 매칭은 호스트 문자열에 대한 **대소문자 무시 substring** 매칭이며 www 접두어는 무시된다(SHALL).

#### Scenario: 기본(Allow 비어 있음) 교차 사이트 허용
- **WHEN** Allow=[], Deny=[] 이고 "https://other.com/page" 링크가 입력될 때
- **THEN** 해당 링크는 필터를 통과한다 (seed 호스트와 다른 호스트여도 허용)

#### Scenario: Deny 리스트 매칭 차단
- **WHEN** Deny=["adnetwork", "spam"] 이고 "https://x.adnetwork.com/ad" 링크가 입력될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: Allow 리스트가 있으면 화이트리스트 모드
- **WHEN** Allow=["example.com", "music"] 이고 "https://music.io/track" 링크가 입력될 때
- **THEN** 해당 링크는 Allow 키워드 "music"에 매칭되어 통과한다
- **WHEN** 같은 Allow 설정에서 "https://other.net/page" 링크가 입력될 때
- **THEN** 해당 링크는 Allow에 매칭되지 않아 제거된다

#### Scenario: Deny가 Allow보다 우선
- **WHEN** Allow=["example.com"], Deny=["tracker"] 이고 "https://tracker.example.com/x" 링크가 입력될 때
- **THEN** 해당 링크는 Deny 매칭으로 차단된다

#### Scenario: www 접두어 무시
- **WHEN** Allow=["example.com"] 이고 "https://www.example.com/page" 링크가 입력될 때
- **THEN** www가 무시되어 Allow에 매칭되어 통과한다

---

### Requirement: canonicalURL이 URL을 정규화한다
`canonicalURL()` 함수는 URL에 대해 다음 정규화를 모두 수행해야 한다(SHALL):
- scheme을 소문자로 변환(SHALL)
- host를 소문자로 변환하고 `www.` 접두어를 제거(SHALL)
- default port 제거: `http`의 `:80`, `https`의 `:443`(SHALL)
- fragment(`#...`) 제거(SHALL)
- 루트가 아닌 경로의 trailing slash 제거(SHALL)
- query 파라미터를 이름 기준 오름차순으로 정렬(SHALL)
- 트래킹 파라미터 제거: `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `ref`, `fbclid`, `gclid`(SHALL)

#### Scenario: scheme/host 소문자
- **WHEN** "HTTPS://Example.COM/Page" URL이 입력될 때
- **THEN** "https://example.com/Page"로 정규화된다 (경로 case는 보존)

#### Scenario: default port 제거
- **WHEN** "http://example.com:80/page" URL이 입력될 때
- **THEN** "http://example.com/page"로 정규화된다
- **WHEN** "https://example.com:443/page" URL이 입력될 때
- **THEN** "https://example.com/page"로 정규화된다

#### Scenario: non-default 포트는 보존
- **WHEN** "http://example.com:8080/page" URL이 입력될 때
- **THEN** 포트 8080은 그대로 유지된다

#### Scenario: fragment 제거
- **WHEN** "https://example.com/page#section" URL이 입력될 때
- **THEN** fragment가 제거되어 "https://example.com/page"로 정규화된다

#### Scenario: trailing slash 통일
- **WHEN** "https://example.com/page/" URL이 입력될 때
- **THEN** trailing slash가 제거되어 "https://example.com/page"로 정규화된다
- **WHEN** "https://example.com/" 루트 URL이 입력될 때
- **THEN** 루트의 "/"는 보존된다

#### Scenario: query 파라미터 정렬
- **WHEN** "https://example.com/page?b=2&a=1&c=3" URL이 입력될 때
- **THEN** "https://example.com/page?a=1&b=2&c=3"로 정렬된다

#### Scenario: 트래킹 파라미터 제거
- **WHEN** "https://example.com/page?utm_source=twitter&id=123" URL이 입력될 때
- **THEN** "https://example.com/page?id=123"으로 정규화된다 (utm_source 제거, id 보존)

#### Scenario: www 접두어 제거
- **WHEN** "https://www.example.com/page" URL이 입력될 때
- **THEN** "https://example.com/page"로 정규화된다

---

### Requirement: semanticPriorityModifier가 HTML 위치 기반 우선순위 보정을 우선순위 점수에 반영시킨다
`semanticPriorityModifier()` 함수는 링크의 HTML 위치(Selector)에 따라 우선순위 보정값을 반환해야 한다(SHALL). Pioneer의 크롤 루프는 해당 반환값을 각 링크의 우선순위 점수에 **반드시 가산**해야 하며(SHALL), 구현되어 있으나 호출되지 않는 상태는 허용되지 않는다(MUST NOT).

- footer/aside 영역: -50을 반환(SHALL)
- nav/header 영역: -20을 반환(SHALL)
- main/article/기타 본문: 0을 반환(SHALL)

#### Scenario: footer/aside 링크 우선순위 감소
- **WHEN** 링크의 Selector가 footer 또는 aside 영역을 나타낼 때
- **THEN** semanticPriorityModifier는 -50을 반환한다

#### Scenario: nav/header 링크 우선순위 소폭 감소
- **WHEN** 링크의 Selector가 nav 또는 header 영역을 나타낼 때
- **THEN** semanticPriorityModifier는 -20을 반환한다

#### Scenario: 본문 링크 우선순위 유지
- **WHEN** 링크의 Selector가 main, article 또는 기타 영역을 나타낼 때
- **THEN** semanticPriorityModifier는 0을 반환한다

#### Scenario: 우선순위 점수에 실제 반영
- **WHEN** Pioneer 크롤 루프가 링크의 우선순위 점수를 계산할 때
- **THEN** semanticPriorityModifier의 반환값이 해당 링크의 최종 점수에 가산된다 (-50, -20, 또는 0)

#### Scenario: 다중 Selector 중 첫 매칭 적용
- **WHEN** 링크가 footer와 nav 두 Selector를 모두 가질 때
- **THEN** 첫 번째로 매칭되는 태그의 보정값이 반환된다 (구현에 위임된 안정 동작)

---

### Requirement: 필터 체인이 순서대로 필터를 적용한다
여러 LinkFilter를 순서대로 체이닝하여 적용할 수 있어야 하며(SHALL), 각 필터의 출력이 다음 필터의 입력이 되어야 한다(SHALL). Pioneer의 기본 체인 구성은 다음 순서로 고정되어야 한다(SHALL): `Domain(allow/deny) → Extension → PathPattern → Robots.txt → CanonicalDedup`.

#### Scenario: 기본 체인 구성
- **WHEN** Pioneer가 FilterChain을 기본 구성으로 초기화할 때
- **THEN** 체인의 필터 순서는 Domain → Extension → PathPattern → Robots.txt → CanonicalDedup이다

#### Scenario: 체이닝 순서 보장
- **WHEN** DomainFilter → ExtensionFilter → PathPatternFilter → RobotsFilter → CanonicalDedupFilter 순서로 체인이 구성될 때
- **THEN** 링크 목록이 순서대로 각 필터를 통과하며 최종 결과가 반환된다

#### Scenario: 빈 링크 목록 처리
- **WHEN** 빈 링크 목록이 필터 체인에 입력될 때
- **THEN** 에러 없이 빈 목록이 반환되며 RobotsFilter는 robots.txt를 fetch하지 않는다
