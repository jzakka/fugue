## ADDED Requirements

### Requirement: Pioneer는 ParseLinks 후 FilterLinks를 거쳐 Enqueue한다
Pioneer Run 루프는 각 페이지를 fetch한 뒤 추출된 링크 목록을 **반드시 필터 체인에 통과**시킨 후 결과만 프런티어 큐에 Enqueue해야 한다(SHALL). 필터 체인을 우회하여 Enqueue하는 경로가 있어서는 안 된다(MUST NOT). 필터 체인의 입력 URL은 `fetchHTML`이 반환하는 redirect chain의 **최종 URL**이어야 한다(SHALL).

#### Scenario: 정상 플로우
- **WHEN** Pioneer가 페이지에서 링크 목록을 추출할 때
- **THEN** 해당 목록은 필터 체인에 입력되고, 체인의 최종 출력만 큐에 Enqueue된다

#### Scenario: 필터 우회 금지
- **WHEN** 구현이 링크 목록을 필터 체인 없이 직접 Enqueue하려 할 때
- **THEN** 해당 구현은 본 스펙을 위반하며 허용되지 않는다

#### Scenario: 빈 결과 처리
- **WHEN** 필터 체인이 모든 링크를 걸러내어 빈 목록을 반환할 때
- **THEN** Pioneer는 에러 없이 다음 Dequeue로 진행한다

#### Scenario: Redirect chain의 최종 URL만 사용
- **WHEN** Pioneer가 301/302 리디렉션을 거쳐 최종 페이지에 도달할 때
- **THEN** 필터 체인과 canonicalization은 최종 URL에만 적용되고 중간 redirect URL은 검사되지 않는다

---

### Requirement: RobotsFilter는 robots.txt를 존중하여 URL을 필터링한다
RobotsFilter는 필터 체인의 구성 요소로서(SHALL), 각 링크의 호스트에 대한 robots.txt를 조회하여 `FugueBot` User-agent(없으면 `*`)의 Disallow 규칙에 매칭되는 URL을 제거해야 한다(SHALL). User-agent 블록은 `FugueBot` 우선 사용, 없을 때 `*` fallback이며 두 블록을 병합하지 않는다(SHALL).

#### Scenario: Disallow에 매칭되면 차단
- **WHEN** 링크의 경로가 해당 호스트 robots.txt의 Disallow 규칙에 매칭될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: Allow/Disallow 모두 없으면 통과
- **WHEN** robots.txt에 `FugueBot` 또는 `*`에 대한 Disallow 규칙이 없을 때
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
- **WHEN** 어떤 호스트에 대한 링크가 RobotsFilter에 처음 도달할 때
- **THEN** RobotsFilter는 `https://<host>/robots.txt`를 fetch하고 결과를 캐시한다

#### Scenario: 캐시 적중
- **WHEN** 같은 호스트에 대한 링크가 24시간 이내에 재도달할 때
- **THEN** RobotsFilter는 새로운 fetch 없이 캐시된 규칙을 사용한다

#### Scenario: TTL 만료 후 재조회
- **WHEN** 캐시 엔트리가 저장된 지 24시간을 초과한 뒤 같은 호스트의 링크가 도달할 때
- **THEN** RobotsFilter는 robots.txt를 다시 fetch하여 캐시를 갱신한다

---

### Requirement: RobotsFilter는 fetch 실패 시 fail-open한다
RobotsFilter는 robots.txt fetch가 네트워크 오류, 타임아웃, 5xx 응답 등으로 실패할 때 **모든 링크를 허용(fail-open)**해야 한다(SHALL). 404 응답은 "robots.txt 없음 = 모두 허용"으로 해석해야 한다(SHALL). 실패 상태 역시 24시간 TTL과 함께 캐시하여 연속 재시도로 인한 폭주를 방지해야 한다(SHALL).

#### Scenario: 네트워크 오류 시 fail-open
- **WHEN** 호스트의 robots.txt fetch가 타임아웃되거나 네트워크 오류로 실패할 때
- **THEN** 해당 호스트의 모든 링크는 RobotsFilter를 통과한다

#### Scenario: 404 응답은 규칙 없음으로 해석
- **WHEN** robots.txt가 404로 응답할 때
- **THEN** 해당 호스트에는 제한이 없는 것으로 간주하여 모든 링크가 통과한다

#### Scenario: 5xx 응답 시 fail-open 상태 캐시
- **WHEN** robots.txt가 5xx로 응답할 때
- **THEN** 해당 호스트는 fail-open 상태로 캐시되며, TTL 이내에는 재시도하지 않는다

---

### Requirement: RobotsFilter는 Crawl-delay를 호스트 bucket에 반영한다
RobotsFilter는 robots.txt에서 `Crawl-delay: N` (초) 지시어를 파싱해야 하며(SHALL), 파싱에 성공한 경우 `scheduler-host-token-bucket` capability가 노출하는 `SetHostRate(host, 1/N, 1)`을 호출하여 해당 호스트의 토큰 버킷 rate를 갱신해야 한다(SHALL). burst는 `1`로 고정한다(SHALL). Crawl-delay가 없거나 파싱에 실패한 경우 `SetHostRate`을 호출하지 않으며 scheduler의 기본 rate가 유지되어야 한다(SHALL).

#### Scenario: Crawl-delay 파싱 및 SetHostRate 호출
- **WHEN** robots.txt에 `Crawl-delay: 5`가 포함될 때
- **THEN** RobotsFilter는 해당 호스트에 대해 `SetHostRate(host, 0.2, 1)`을 호출하여 초당 0.2 requests로 rate를 갱신한다

#### Scenario: Crawl-delay 미지정 시 기본 rate 유지
- **WHEN** robots.txt에 Crawl-delay가 명시되지 않을 때
- **THEN** `SetHostRate`은 호출되지 않고 scheduler의 기본 rate가 유지된다

#### Scenario: 파싱 불가능한 Crawl-delay 무시
- **WHEN** Crawl-delay 값이 정수/실수로 파싱되지 않을 때
- **THEN** 해당 값은 무시되고 `SetHostRate`은 호출되지 않는다

#### Scenario: 캐시 TTL 내 중복 호출 방지
- **WHEN** 같은 호스트에 대해 24시간 캐시 TTL 이내에 다수 링크가 필터링될 때
- **THEN** `SetHostRate`은 캐시 갱신 시점(최초 fetch 또는 TTL 만료 재fetch)에만 호출된다

---

## MODIFIED Requirements

### Requirement: DomainFilter는 Allow/Deny 키워드로 도메인을 필터링하며 교차 사이트 크롤을 기본 허용한다
archive impl(`2026-04-13-pioneer-link-filter-impl`)의 "DomainFilter가 루트 도메인 링크만 통과시킨다" 요구사항을 본 요구사항이 대체한다. DomainFilter는 단일 루트 도메인 비교가 아니라 **Allow 키워드 리스트**와 **Deny 키워드 리스트**를 받아 링크 호스트에 대한 substring 매칭으로 필터링해야 한다(SHALL).

- Deny 리스트에 매칭되는 호스트는 항상 차단한다(SHALL).
- Allow 리스트가 비어 있으면 Deny에 걸리지 않은 모든 호스트를 통과시킨다(SHALL). 즉 **교차 사이트 크롤을 기본 허용**한다.
- Allow 리스트가 비어 있지 않으면, Allow 키워드 중 하나라도 매칭되는 호스트만 통과시킨다(SHALL).
- 매칭 규칙은 호스트 문자열을 lowercased + `www.` 접두어 제거 후 **대소문자 무시 substring** 비교다(SHALL).
- 국가별 TLD에 대한 특별 처리는 없으며 substring 매칭이 그대로 적용된다(SHALL).

#### Scenario: Allow 비어 있음 - 교차 사이트 기본 허용
- **WHEN** Allow=[], Deny=[] 이고 seed 호스트와 다른 외부 호스트 링크가 입력될 때
- **THEN** 해당 링크는 필터를 통과한다

#### Scenario: Deny 리스트 매칭 차단
- **WHEN** Deny 키워드가 "adnetwork"이고 호스트에 "adnetwork"를 포함하는 링크가 입력될 때
- **THEN** 해당 링크는 필터에서 제거된다

#### Scenario: Allow 리스트가 있으면 화이트리스트 모드
- **WHEN** Allow 키워드가 "music"이고 호스트가 "music.io"인 링크가 입력될 때
- **THEN** 해당 링크는 Allow 매칭으로 통과한다
- **WHEN** 같은 Allow 설정에서 호스트가 "other.net"인 링크가 입력될 때
- **THEN** 해당 링크는 Allow에 매칭되지 않아 제거된다

#### Scenario: Deny가 Allow보다 우선
- **WHEN** Allow 키워드 "example.com"과 Deny 키워드 "tracker"가 모두 설정되고, 호스트 "tracker.example.com" 링크가 입력될 때
- **THEN** Deny 매칭이 우선 적용되어 해당 링크는 제거된다

#### Scenario: www 접두어 및 대소문자 무시
- **WHEN** Allow 키워드 "example.com"이 설정되고 호스트 "WWW.Example.com" 링크가 입력될 때
- **THEN** www와 대소문자가 정규화되어 Allow 매칭으로 통과한다

---

### Requirement: canonicalURL은 URL을 RFC 3986 수준으로 정규화한다
archive impl(`2026-04-13-pioneer-link-filter-impl`)의 "canonicalPath가 URL을 정규화한다" 요구사항을 본 요구사항이 대체한다. URL 정규화는 다음을 **모두** 수행해야 한다(SHALL).

- scheme을 소문자로 변환한다(SHALL).
- host를 소문자로 변환하고 `www.` 접두어를 제거한다(SHALL).
- default port를 제거한다: `http`의 `:80`과 `https`의 `:443`(SHALL). non-default 포트는 보존한다(SHALL).
- fragment(`#...`)를 제거한다(SHALL).
- 루트가 아닌 경로의 trailing slash를 제거한다(SHALL).
- 트래킹 파라미터를 제거한다: `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `ref`, `fbclid`, `gclid`(SHALL).
- 남은 query 파라미터를 이름(key) 오름차순으로 정렬하여 재인코딩한다(SHALL).
- 경로(path)의 대소문자는 **보존**한다(SHALL).

#### Scenario: scheme과 host는 소문자, 경로는 보존
- **WHEN** `HTTPS://Example.COM/Page` 형태의 URL이 입력될 때
- **THEN** `https://example.com/Page` 로 정규화된다

#### Scenario: default port 제거
- **WHEN** scheme이 http이고 호스트에 `:80`이 포함된 URL이 입력될 때
- **THEN** `:80`이 제거된 URL이 반환된다
- **WHEN** scheme이 https이고 호스트에 `:443`이 포함된 URL이 입력될 때
- **THEN** `:443`이 제거된 URL이 반환된다

#### Scenario: non-default 포트는 보존
- **WHEN** 호스트에 `:8080` 등 default가 아닌 포트를 포함한 URL이 입력될 때
- **THEN** 해당 포트는 그대로 유지된다

#### Scenario: query 파라미터 이름순 정렬
- **WHEN** 파라미터가 `b=2&a=1&c=3` 순서로 입력될 때
- **THEN** 정규화된 URL은 `a=1&b=2&c=3` 순서가 된다

#### Scenario: 트래킹 파라미터 제거 후 정렬
- **WHEN** `utm_source=twitter&id=123&a=z` 쿼리가 입력될 때
- **THEN** `utm_source`가 제거되고 남은 파라미터가 `a=z&id=123`로 정렬된다

#### Scenario: fragment 제거
- **WHEN** URL에 `#section` 등 fragment가 포함될 때
- **THEN** fragment가 제거된 URL이 반환된다

#### Scenario: trailing slash 통일과 루트 보존
- **WHEN** 경로가 `/page/`인 URL이 입력될 때
- **THEN** trailing slash가 제거되어 `/page`가 된다
- **WHEN** 경로가 루트 `/`인 URL이 입력될 때
- **THEN** 루트 `/`는 보존된다

#### Scenario: 대표 복합 케이스
- **WHEN** `http://Example.com:80/path/?b=2&a=1#frag` 가 입력될 때
- **THEN** `http://example.com/path?a=1&b=2` 로 정규화된다

---

### Requirement: 필터 체인은 고정된 순서로 필터를 적용한다
archive impl(`2026-04-13-pioneer-link-filter-impl`)의 "필터 체인이 순서대로 필터를 적용한다" 요구사항을 본 요구사항이 대체한다. Pioneer의 기본 필터 체인은 다음 순서를 따라야 한다(SHALL): (1) Domain allow/deny, (2) Extension, (3) PathPattern, (4) Robots, (5) Dedup. 이 순서는 값이 싼 필터를 앞에, 네트워크 I/O(Robots)와 공유 맵/DB 조회(Dedup)가 있는 값비싼 필터를 뒤에 배치하기 위해 고정되어야 한다(SHALL).

- Domain / Extension / PathPattern은 인메모리 문자열 또는 regex 매칭으로 가장 저렴하다.
- Robots는 캐시 hit 시 인메모리, miss 시 HTTP GET이 발생하므로 앞 세 필터로 먼저 후보를 줄인 뒤 평가한다.
- Dedup은 공유 visited 맵 조회와 canonical 해시 계산이 필요하므로 가장 뒤에 배치된다.
- Robots 필터는 **Enqueue 단계**에서 호출되어 의미적 차단을 담당한다. **Claim 단계**의 host token bucket 체크는 scheduler-host-token-bucket capability가 담당하며 본 스펙과 별개다.

#### Scenario: 기본 체인 구성
- **WHEN** Pioneer가 필터 체인을 기본 구성으로 초기화할 때
- **THEN** 체인의 필터 순서는 Domain → Extension → PathPattern → Robots → Dedup이다

#### Scenario: 앞 단계에서 탈락한 링크는 뒤 단계에 도달하지 않는다
- **WHEN** 어떤 링크가 Extension 또는 PathPattern 필터에서 제거될 때
- **THEN** 해당 링크는 Robots나 Dedup에 도달하지 않으며, robots.txt 조회 비용을 유발하지 않는다

#### Scenario: 빈 링크 목록 처리
- **WHEN** 빈 링크 목록이 필터 체인에 입력될 때
- **THEN** 에러 없이 빈 목록이 반환되며 Robots는 robots.txt를 fetch하지 않는다

#### Scenario: Enqueue 단계와 Claim 단계의 책임 분리
- **WHEN** Pioneer가 링크를 Enqueue하고 scheduler가 해당 URL을 Claim할 때
- **THEN** Enqueue 단계에서 Robots 필터가 Disallow를 거르고, Claim 단계에서 host token bucket이 속도를 제어한다
