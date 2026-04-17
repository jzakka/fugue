## Why

현재 Pioneer의 링크 필터 체인은 "같은 루트 도메인 링크만 통과"를 전제로 설계되어 있어, 크로스미디어 큐레이션이라는 Fugue의 비전에 필요한 **교차 사이트 크롤**을 구조적으로 막는다. 또한 robots.txt 존중 로직과 본격적인 URL canonicalization(scheme/host 소문자, default port 제거, query 정렬 등)이 정책 차원에서 정의되지 않아, 대외 정중함(politeness)과 중복 제거 정확도 양쪽 모두에서 구멍이 있다. 이번 변경은 Pioneer가 파싱한 링크를 Enqueue 전에 반드시 **필터 체인 → robots.txt → canonicalization**을 통과하도록 정책을 확립하고, 교차 사이트 크롤을 명시적으로 허용한다.

## What Changes

- **BREAKING**: Pioneer의 도메인 정책을 "같은 루트 도메인만 허용"에서 **교차 사이트 크롤 허용**으로 변경한다. 도메인 제약은 이제 Allow/Deny 키워드 리스트로 표현된다.
- Pioneer Run 루프에 **"ParseLinks → FilterLinks → Enqueue"** 순서를 필수 정책으로 못박는다. FilterLinks를 건너뛴 Enqueue는 금지된다.
- 필터 체인의 **고정 순서**를 정의한다: `Domain allow/deny → Extension → PathPattern → Robots.txt → CanonicalDedup`. 값이 비싼 필터(robots.txt 네트워크 조회, canonical 계산)를 뒤로 배치한다.
- **DomainFilter를 Allow/Deny 키워드 기반으로 재정의**: 루트 도메인 고정 비교 대신, 도메인 allow 리스트와 deny 리스트 키워드 매칭으로 동작한다. 리스트가 비어 있으면 모든 도메인을 통과시킨다(교차 사이트 기본 허용).
- **ExtensionFilter / PathPatternFilter를 Allow/Deny 의미로 확장**: 기존 deny 확장자/경로에 더해, 선택적 allow 리스트를 지원한다.
- **RobotsFilter(신규)**: 호스트별 robots.txt를 **lazy fetch**하여 User-agent="FugueBot" 기준으로 Disallow 경로를 차단한다. **호스트별 캐시 TTL 24시간**, **fetch 실패 시 fail-open**(허용)으로 동작한다.
- **Crawl-delay pass-through**: robots.txt의 `Crawl-delay` 값을 파싱하여 호스트별 메타데이터로 노출한다. `scheduler-host-token-bucket`이 이를 host bucket rate로 반영하도록 surface만 정의한다(실제 bucket 구현은 본 변경 범위 외).
- **Canonicalization 확장**: 기존 canonicalURL을 확장하여 (a) scheme/host 소문자화, (b) 기본 포트(http:80, https:443) 제거, (c) fragment 제거(기존), (d) trailing slash 정규화(기존), (e) query 파라미터 알파벳 정렬을 수행한다. 기존 트래킹 파라미터 제거와 www 제거는 유지된다.
- **Semantic priority modifier를 score 기여 정책으로 승격**: 기존 "Selector 기반 보정값 반환" 헬퍼의 출력이 Pioneer의 우선순위 점수 계산에 **가산/감산으로 반드시 반영**되도록 정책을 고정한다(footer/aside=-50, nav/header=-20, 본문=0).

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `bot`: Pioneer 링크 처리 파이프라인의 정책 계층을 확립한다. (1) Pioneer Run 루프가 FilterLinks를 강제로 통과시키도록 플로우 요구사항을 추가, (2) 기존 DomainFilter를 "루트 도메인 고정"에서 "Allow/Deny 키워드 + 교차 사이트 기본 허용"으로 재정의, (3) RobotsFilter 요구사항 신설(lazy fetch, TTL 24h, fail-open, Crawl-delay surface), (4) canonicalURL의 정규화 규칙을 확장(scheme/host 소문자, default port 제거, query 정렬), (5) semanticPriorityModifier의 출력이 우선순위 점수에 반영되도록 정책을 고정.

## Impact

- **코드**: `apps/api/internal/bot/link_filter.go`의 `DomainFilter` 의미 변경, 신규 `RobotsFilter` 추가, `canonicalURL()` 확장, Pioneer 크롤 루프에서 FilterLinks 호출 강제 및 semantic modifier의 score 반영 지점 확인 필요.
- **정책 변경(BREAKING)**: 기존 "같은 루트 도메인만 크롤" 동작에 의존하는 호출부(Pioneer 초기화 코드, 테스트)는 Allow/Deny 리스트를 명시해야 한다.
- **네트워크**: RobotsFilter가 호스트별로 robots.txt에 HTTP GET을 추가로 수행한다(첫 접근 시 1회, 이후 24시간 캐시). scheduler-host-token-bucket의 fetcher 인프라를 통해 나가며 별도 bucket을 소비한다.
- **의존성**: `scheduler-host-token-bucket` 변경이 Crawl-delay를 bucket rate로 소비하는 쪽을 정의한다. 본 변경은 surface(호스트별 crawl-delay 값 노출)까지만 책임진다.
- **DB/API**: 스키마 변경 없음. 외부 API 변경 없음.
- **참조 구현**: `apps/api/internal/bot/link_filter.go`, `apps/api/fuguebot_pseudo.go`의 `Pioneer.FilterLinks`.
- **구현 변경 아카이브 경로**: `openspec/changes/archive/2026-04-13-pioneer-link-filter-impl/` (본 정책 변경의 후속 구현 아카이브는 향후 해당 경로에 추가).
