## Why

현재 Pioneer의 링크 필터 체인은 archive change `2026-04-13-pioneer-link-filter-impl`(이하 "archive impl")에서 확립되었지만, 다음 세 가지 공백이 존재한다.

1. **DomainFilter의 same-root-domain 전제**: archive impl에서 `DomainFilter.RootDomain` 단일 필드를 사용해 루트 도메인 고정 매칭으로 동작한다. 이는 Fugue의 **크로스미디어 큐레이션** 비전(음악→아트→블로그를 넘나들며 수집)과 구조적으로 모순된다.
2. **canonicalURL의 부분적 정규화**: archive impl의 `canonicalURL()`은 www 제거, 트래킹 파라미터 제거, trailing slash 통일, fragment 제거만 수행한다. scheme/host 소문자, default port(80/443) 제거, query 파라미터 정렬 등 RFC 3986 수준의 정규화가 빠져 있어 같은 리소스가 서로 다른 해시로 튀는 중복 생성 위험이 있다.
3. **robots.txt 미지원**: 외부 사이트를 크롤할 때 요구되는 `Disallow` 존중과 `Crawl-delay` 반영이 정책 차원에서 정의되어 있지 않다.

본 change는 **archive impl을 un-archive하여 덮어쓰는(MODIFIED) 방식**으로 위 세 공백을 메운다. 즉 `openspec/changes/archive/2026-04-13-pioneer-link-filter-impl/`의 기존 요구사항 중 DomainFilter·canonicalURL·FilterChain 요구사항을 본 change가 대체하고, RobotsFilter·필터 순서 강제·Pioneer 루프 플로우 요구사항을 새로 추가한다.

## What Changes

- **archive 재활용(특수)**: archive change `2026-04-13-pioneer-link-filter-impl`의 DomainFilter / canonicalURL / 필터 체인 요구사항을 본 change가 **MODIFIED로 덮어쓴다**. archive 디렉터리는 기록용으로 그대로 유지하되, 실제 baseline(`openspec/specs/bot/spec.md`)에는 본 change의 delta가 반영되어 archive 내용을 override한다.
- **BREAKING**: `DomainFilter.RootDomain` 단일 필드 → `AllowKeywords []string`, `DenyKeywords []string` 두 필드로 교체한다. 교차 사이트 크롤을 기본 허용(Allow 비어 있으면 모든 호스트 통과)하고, Deny 리스트에 매칭되는 호스트만 차단한다. 두 리스트 모두에 매칭되면 Deny가 우선한다.
- **canonicalURL 확장**: 기존 규칙(www 제거, 트래킹 파라미터 제거, trailing slash 통일, fragment 제거)에 더해 다음 세 규칙을 추가한다.
  - scheme 소문자화 (`HTTPS://` → `https://`)
  - default port 제거 (`http://x:80/` → `http://x/`, `https://x:443/` → `https://x/`)
  - query 파라미터 이름순 오름차순 정렬 (`?b=2&a=1` → `?a=1&b=2`)
- **RobotsFilter(신규)**: 호스트별 robots.txt를 lazy fetch하여 `FugueBot` User-agent(없으면 `*`)의 Disallow 규칙에 매칭되는 URL을 제거한다. 호스트별 인메모리 캐시 TTL 24시간, fetch 실패(네트워크 오류, 5xx, 타임아웃) 시 fail-open으로 동작한다. 404는 "규칙 없음"으로 해석하여 모두 허용한다. `Crawl-delay` 값을 파싱하여 `scheduler-host-token-bucket`의 `SetHostRate(host, 1/delay, 1)`로 전달한다.
- **필터 체인 순서 고정**: `Domain → Extension → PathPattern → Robots → Dedup`. 값이 싼 필터(in-memory 문자열/regex)를 앞에, 네트워크 I/O가 있는 Robots를 중간에, DB 조회가 있는 Dedup을 뒤에 배치한다.
- **Pioneer 루프 플로우 강제**: `ParseLinks → FilterLinks → Enqueue` 순서를 정책으로 못박아 FilterLinks 우회 Enqueue를 금지한다.
- **Redirect chain 처리**: 필터는 `pioneer.go`의 `fetchHTML`가 반환하는 **최종 URL(finalURL)** 에 대해서만 적용된다. 중간 redirect URL은 검사하지 않는다.
- **국가별 TLD(Open Question 종결)**: Allow/Deny 매칭은 호스트 substring 매칭으로 고정한다. `.co.kr`, `.co.jp` 등 국가별 TLD에 대한 특별 처리는 추가하지 않는다.

## Capabilities

### New Capabilities
(없음)

### Modified Capabilities
- `bot`: archive impl의 DomainFilter / canonicalURL / 필터 체인 요구사항을 MODIFIED로 대체하고, RobotsFilter와 Pioneer 루프 플로우 / 필터 순서 / Crawl-delay surface 요구사항을 ADDED로 추가한다.

## Impact

- **코드**:
  - `apps/api/internal/bot/link_filter.go`: `DomainFilter.RootDomain` 제거 → `AllowKeywords`, `DenyKeywords` 필드 추가. `canonicalURL()` 확장(scheme 소문자, default port 제거, query 정렬). `RobotsFilter` 타입 신규 추가(별도 파일 `robots_filter.go` 또는 `link_filter.go` 내).
  - `apps/api/internal/bot/pioneer.go`: 기본 FilterChain 조립 시 필터 순서 `Domain → Extension → PathPattern → Robots → Dedup`로 조정. DomainFilter 생성부에서 Allow/Deny 기본값(`nil, nil`)을 주입.
  - RobotsFilter의 Crawl-delay surface를 `scheduler-host-token-bucket.SetHostRate`에 연결.
- **archive 재활용 처리 방식**:
  - archive 디렉터리(`openspec/changes/archive/2026-04-13-pioneer-link-filter-impl/`)는 **물리적으로 이동하지 않는다**. archive는 과거 구현 change의 기록이다.
  - 본 change는 `openspec/specs/bot/spec.md` 베이스라인에 기록된 archive impl의 요구사항 중 DomainFilter / canonicalURL / 필터 체인 3개를 `## MODIFIED Requirements`로 override한다.
  - 본 change가 archive 완료 후 `openspec archive`로 이동하면 `pioneer-link-filter-policy-impl`(후속 구현 change) 대신 본 change가 해당 요구사항의 최종 소유자가 된다.
- **네트워크**: RobotsFilter가 호스트별로 robots.txt HTTP GET을 추가한다(첫 접근 1회, 이후 24h 캐시).
- **DB/API**: 스키마·외부 API 변경 없음.
- **테스트**:
  - archive impl의 기존 DomainFilter 테스트는 Allow/Deny 매칭 시나리오로 대체된다.
  - canonicalURL 테스트는 기존 회귀 테스트(www, utm, trailing slash, fragment) 유지 + 신규(scheme 대문자, default port 80/443, non-default 포트 보존, query 정렬) 추가.
  - RobotsFilter 테스트 신규: Disallow 차단, 404 허용, 5xx fail-open, Crawl-delay 파싱, TTL 만료 재조회.
- **참조 구현**: `apps/api/internal/bot/link_filter.go`, `apps/api/internal/bot/pioneer.go` (Run 루프의 finalURL 취득 지점).
