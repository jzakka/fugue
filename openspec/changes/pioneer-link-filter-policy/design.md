## Context

Pioneer는 현재 `apps/api/internal/bot/link_filter.go`의 `FilterChain`에 `DomainFilter → ExtensionFilter → PathPatternFilter → CanonicalDedupFilter` 네 필터를 순서대로 연결하여 링크를 거른다. `DomainFilter`는 `isSameDomain(url, RootDomain)`에 의존하여 구조적으로 **같은 루트 도메인만 통과**시킨다. 한편 Fugue는 크로스미디어 큐레이션 플랫폼으로, 음악 사이트 → 아트 사이트 → 블로그처럼 서로 다른 호스트를 가로지르며 자료를 수집하는 것이 본질이다. 즉, 현재 필터 정책과 제품 방향 사이에 모순이 있다.

또한 다음 정책이 공백 상태다:
- **robots.txt**: 현재 구현은 robots.txt를 조회하지 않는다. 외부 사이트를 크롤할 때 법적/윤리적 리스크를 키운다.
- **Full canonicalization**: 현재 `canonicalURL()`은 www 제거, 트래킹 파라미터 제거, trailing slash, fragment만 다룬다. scheme/host 대소문자, default port(80/443), query 정렬 같은 RFC 3986 수준의 정규화가 빠져 있어 같은 리소스가 서로 다른 해시로 튀는 경우가 남는다.
- **Crawl-delay 연계**: `scheduler-host-token-bucket` 변경이 호스트별 요청률을 관리하지만, 개별 사이트의 robots.txt `Crawl-delay` 값과 연결되어 있지 않다.
- **Semantic priority modifier의 실제 반영**: `semanticPriorityModifier()` 함수는 존재하지만, Pioneer 루프의 우선순위 점수 계산에 반영되는지 스펙 차원에서 강제되지 않는다.

이 변경은 위 공백을 정책 계층에서 메운다. 코드 수정 자체는 후속 구현 변경이 담당한다.

## Goals / Non-Goals

**Goals:**
- Pioneer의 Run 루프에서 "ParseLinks → FilterLinks → Enqueue" 순서를 스펙으로 강제한다.
- 필터 체인의 순서를 `Domain(allow/deny) → Extension → PathPattern → Robots.txt → CanonicalDedup`으로 고정한다.
- 교차 사이트 크롤을 기본 허용(allow 리스트 비어 있으면 모든 호스트 통과)으로 전환한다.
- robots.txt를 정책적으로 존중하되 가용성을 해치지 않도록 fail-open + 24h 캐시로 정의한다.
- robots.txt `Crawl-delay`를 `scheduler-host-token-bucket`이 소비 가능한 형태로 surface한다.
- canonicalization 규칙을 확장하여(scheme/host 소문자, default port 제거, query 정렬) 중복 제거 정확도를 올린다.
- semanticPriorityModifier의 출력이 실제 우선순위 점수에 반영되어야 함을 정책으로 고정한다.

**Non-Goals:**
- `scheduler-host-token-bucket` 자체의 bucket 알고리즘/구현. 본 변경은 "Crawl-delay 값이 호스트별로 노출된다"는 surface까지만 정의한다.
- Scheduler 인터페이스 변경. 본 변경은 Pioneer 내부 필터 정책에 한정된다.
- 분산 환경에서의 robots.txt 캐시 공유. 본 변경은 **프로세스별 인메모리 캐시**를 전제로 한다.
- robots.txt의 Sitemap 지시어 활용. 본 변경은 Disallow와 Crawl-delay만 다룬다.
- 실제 Go 구현(`RobotsFilter` 파일 추가, `DomainFilter` 재작성 등). 후속 구현 변경에서 다룬다.

## Decisions

### D1. 교차 사이트 크롤 허용 (기본 허용 + Allow/Deny 키워드)

**결정**: DomainFilter는 Allow/Deny 키워드 리스트를 받는다. Allow 리스트가 비어 있으면 모든 호스트를 허용(기본 허용). Deny 리스트에 매칭되면 차단.

**대안과 사유**:
- (A) same-site-only 유지 → Fugue의 크로스미디어 비전과 모순. 기각.
- (B) 무조건 교차 허용 → 명백히 피하고 싶은 도메인(스팸, 광고 네트워크)을 차단할 수단이 없음. 기각.
- (C) **Allow/Deny 키워드 + 기본 허용** → 도메인 substring 매칭으로 단순하고 설정 파일 한 줄 수정으로 범위 조절 가능. **채택**.

### D2. 필터 순서 고정

**결정**: `Domain(allow/deny) → Extension → PathPattern → Robots.txt → CanonicalDedup`.

**사유**: 비용이 싼 필터(문자열 매칭)를 앞에, 비싼 필터(네트워크 I/O, 파싱)를 뒤에 배치한다.
- Domain/Extension/PathPattern은 순수 문자열 연산: O(1) ~ O(n) 매칭.
- Robots.txt는 첫 접근 시 HTTP GET 발생. 호스트별 캐시 적중 이후는 O(1)에 가깝지만, Extension/PathPattern이 먼저 필터링해 주면 robots.txt 조회 대상이 줄어든다.
- CanonicalDedup은 정규화를 거친 URL 해시 계산 + 방문 맵 조회. 필터를 통과한 후보만 대상으로 해야 낭비가 없다.

### D3. robots.txt: lazy fetch + TTL 24h + fail-open

**결정**:
- 호스트 최초 방문 시 `https://<host>/robots.txt`를 fetch.
- 결과(파싱된 rule 집합 + Crawl-delay + fetched_at)를 호스트별 인메모리 맵에 캐시.
- TTL 24시간. 만료 시 다음 접근에 재조회.
- Fetch 실패(네트워크 오류, 5xx, 타임아웃) 시 **fail-open**(모두 허용). 캐시에 "fail-open" 상태로 TTL을 적용하여 폭주 방지.
- 404는 "robots.txt 없음 = 모두 허용"으로 해석(RFC 9309 기본 동작).

**대안과 사유**:
- fail-closed: 크롤 중단 위험. 외부 서비스의 일시 장애가 Pioneer 전체를 멈추게 함. 기각.
- 영구 캐시: 사이트 정책 변경을 반영 못함. 24h가 업계 관행.
- User-agent 매칭은 `FugueBot`을 우선, 없으면 `*` fallback.

### D4. Crawl-delay → 호스트 bucket rate로 surface

**결정**: RobotsFilter는 파싱한 `Crawl-delay`(초 단위)를 호스트별로 기록한다. 이 값은 `scheduler-host-token-bucket`이 bucket refill rate를 계산할 때 사용된다. 상한 없이 그대로 전달한다(정책은 scheduler 쪽에서 결정).

**사유**: 두 스펙 사이의 결합을 최소화한다. 본 변경은 "값을 노출한다"까지, scheduler 변경은 "값을 해석해 rate에 반영한다"까지 책임진다.

### D5. Canonicalization 확장

**결정**: 기존 canonicalURL에 다음 규칙을 추가한다.
- scheme 소문자: `HTTPS://` → `https://`
- host 소문자: `Example.COM` → `example.com`
- default port 제거: `http://x:80/` → `http://x/`, `https://x:443/` → `https://x/`
- fragment 제거: `#section` 삭제 (기존 유지)
- trailing slash 정규화: 루트가 아니면 제거 (기존 유지)
- query 파라미터 알파벳 순 정렬: `?b=2&a=1` → `?a=1&b=2`
- 트래킹 파라미터 제거 (기존 유지: utm_*, ref, fbclid, gclid)
- www 제거 (기존 유지)

**사유**: 같은 리소스가 다른 해시로 빠져 노드가 중복 생성되는 원인을 줄인다. query 정렬은 특히 중요한데, 서버가 파라미터 순서에 무관하더라도 URL 문자열은 달라지기 때문이다.

**주의**: 본 변경은 "link filter에서 쓰는 canonical" 정의의 확장이며, 그래프 노드 패턴화(숫자 segment → `{id}`)와는 별개 레이어다. 기존 `bot` spec의 "그래프 노드와 엣지를 관리한다" 요구사항은 영향받지 않는다.

### D6. Semantic priority modifier를 score에 반드시 반영

**결정**: semanticPriorityModifier의 반환값(footer/aside=-50, nav/header=-20, else=0)을 Pioneer의 우선순위 점수 계산에 **가산**한다. 이를 스펙으로 고정하여 "구현되어 있지만 호출되지 않는" 상태를 방지한다.

**사유**: 본문 링크(main/article)를 먼저 소비하여 가치가 높은 노드를 선탐험하도록 프론티어 편향을 정책화한다.

## Risks / Trade-offs

- **[교차 사이트 크롤의 예산 폭발]** → scheduler-host-token-bucket과 pioneer-worker-budget이 이미 예산 제어를 담당한다. Deny 리스트에 대형 광고/스팸 도메인 키워드를 운영 중 추가 가능.
- **[robots.txt fail-open으로 인한 정중함 위반]** → fetch 실패가 연속으로 발생하는 호스트는 캐시에 fail-open 표식과 TTL이 남아 있어 재조회 간격이 벌어진다. 운영 중 로그로 모니터링 가능.
- **[Crawl-delay 값이 비현실적으로 큰 경우(예: 300초)]** → bucket이 해석하는 쪽(scheduler 변경)에서 상한을 둬야 한다. 본 변경 범위 밖.
- **[canonical 확장이 기존 방문 맵 키와 충돌]** → 현재 방문 맵은 `hashURL(l.URL)` 원본 URL 해시를 쓰며, CanonicalDedup은 `hashURL(canonicalURL(l.URL))` 별도 맵을 쓴다. 구현 시 두 맵의 정의가 흐트러지지 않도록 주의. 후속 구현 변경에서 마이그레이션 고려.
- **[프로세스별 인메모리 robots.txt 캐시]** → 다중 Pioneer 인스턴스에서 같은 호스트에 중복 조회가 생길 수 있다. 24h TTL과 호스트 수를 감안하면 유의미한 부담은 아님. 분산 캐시는 후속 과제.
- **[semanticPriorityModifier의 단위 충돌]** → 현재 우선순위 점수의 스케일과 -50/-20 폭이 맞지 않으면 편향이 약하거나 과도할 수 있다. 점수 스케일 문서화는 `pioneer-worker-budget` 또는 후속 변경에서.

## Migration Plan

1. 본 정책 변경을 머지한 뒤, 후속 구현 변경(`2026-04-??-pioneer-link-filter-policy-impl`)에서 다음을 수행한다:
   - `DomainFilter` 재작성: `RootDomain` 필드를 `AllowKeywords []string`, `DenyKeywords []string`로 교체.
   - 기존 DomainFilter 호출부(Pioneer 초기화 코드, 테스트 픽스처)에서 Allow 리스트를 비우거나 seed 도메인 기반으로 명시 지정.
   - `RobotsFilter` 신규 파일 추가.
   - `canonicalURL()` 확장 규칙 구현.
   - Pioneer 루프에서 FilterChain에 RobotsFilter가 CanonicalDedup 직전에 삽입되었는지, semantic modifier 출력이 우선순위 계산에 가산되는지 확인.
2. 롤백: DomainFilter 변경이 breaking이므로, 롤백 시 호출부의 Allow 리스트 제거와 `RootDomain` 복구가 필요. 구현 변경을 별도 커밋으로 유지.
3. 후속 구현 아카이브는 `openspec/changes/archive/2026-04-13-pioneer-link-filter-impl/` 계열 디렉터리 규칙을 따라 날짜 접두사 + 슬러그로 저장.

## Open Questions

- Allow/Deny 키워드는 "호스트 substring 매칭"인가 "정확한 도메인/서브도메인 매칭"인가? → 본 스펙은 substring 매칭으로 제안. 구현 시 confirm 필요.
- RobotsFilter가 쓰는 HTTP 클라이언트는 Pioneer의 기존 fetcher를 재사용할지, 별도 경량 클라이언트를 둘지? → 후속 구현 결정.
- Crawl-delay가 명시되지 않은 호스트의 기본값은? → 본 변경은 "값이 없으면 surface하지 않음"으로 정의. scheduler가 기본 rate를 쓴다.
