## 0. 선행 조건 / 아카이브 정리

- [ ] 0.1 archive change `2026-04-13-pioneer-link-filter-impl`의 DomainFilter / canonicalURL / 필터 체인 요구사항이 현재 baseline(`openspec/specs/bot/spec.md`)에 존재하는지 확인 (MODIFIED 대상이 실재해야 함)
- [ ] 0.2 archive 디렉터리는 **물리적으로 옮기지 않는다**는 점 재확인: 본 change의 spec delta가 baseline을 override하는 방식으로 un-archive 효과를 낸다
- [ ] 0.3 `go build ./apps/api/internal/bot/...` 현재 상태에서 성공 확인

## 1. DomainFilter 재정의 (Allow/Deny 키워드)

archive impl tasks §2에서 정의된 `DomainFilter{RootDomain}`를 본 작업이 대체한다.

- [ ] 1.1 `apps/api/internal/bot/link_filter.go`의 `DomainFilter` 구조체에서 `RootDomain string` 필드 제거
- [ ] 1.2 `DomainFilter`에 `AllowKeywords []string`, `DenyKeywords []string` 필드 추가
- [ ] 1.3 `Filter` 메서드 로직 재작성: 호스트를 lowercase + `www.` 제거 → DenyKeywords substring 매칭 시 제외 → AllowKeywords 비어 있으면 통과 → 아니면 AllowKeywords 중 하나라도 substring으로 포함해야 통과
- [ ] 1.4 Pioneer 초기화 경로(`apps/api/internal/bot/pioneer.go` 등)에서 DomainFilter 생성 지점을 찾아 Allow/Deny 리스트 주입 형태로 호출부 업데이트
- [ ] 1.5 기본 Deny 리스트는 빈 값으로 시작(추후 운영 중 확장). 기본 Allow는 빈 값(= 교차 사이트 기본 허용)
- [ ] 1.6 테스트 업데이트: archive impl의 `TestDomainFilter`를 Allow/Deny 시나리오로 교체 (기본 허용, Deny 매칭, Allow 화이트리스트, Deny 우선, www 무시, 대소문자 무시 6케이스)

## 2. canonicalURL 확장

archive impl tasks §1.2 및 §6.2에서 정의된 `canonicalURL()`을 본 작업이 확장한다.

- [ ] 2.1 `canonicalURL()` 함수 초입에 scheme을 lowercase로 변환하는 로직 추가
- [ ] 2.2 host에서 `www.` 제거 후, scheme이 `http`일 때 `:80` 포트, `https`일 때 `:443` 포트를 Host 문자열에서 제거
- [ ] 2.3 트래킹 파라미터 제거 후 `url.Values.Encode()`로 재인코딩하여 query 파라미터가 key 오름차순으로 정렬되는지 확인 (Go 표준 동작)
- [ ] 2.4 회귀 테스트 확인: 기존 www 제거 / utm 제거 / trailing slash 제거 / fragment 제거 시나리오가 여전히 통과
- [ ] 2.5 신규 테스트 추가:
  - scheme 대문자 입력(`HTTPS://Example.COM/Page`) → scheme/host만 lowercase, 경로 case 보존
  - `http://example.com:80/path` → `:80` 제거
  - `https://example.com:443/path` → `:443` 제거
  - `http://example.com:8080/path` → `:8080` 보존
  - `?b=2&a=1&c=3` → `?a=1&b=2&c=3`
  - 대표 케이스: `http://Example.com:80/path/?b=2&a=1#frag` → `http://example.com/path?a=1&b=2`
  - 루트 `/` 보존

## 3. RobotsFilter 신규 구현

- [ ] 3.1 `apps/api/internal/bot/robots_filter.go`(또는 `link_filter.go` 내) 파일에 `RobotsFilter` 구조체 선언 및 `LinkFilter` 인터페이스 구현
- [ ] 3.2 호스트별 캐시 구조 정의:
  ```
  type robotsCacheEntry struct {
      rules      []disallowRule
      crawlDelay *float64
      fetchedAt  time.Time
      failOpen   bool
  }
  ```
- [ ] 3.3 `sync.RWMutex`로 보호되는 `map[string]robotsCacheEntry` 캐시를 RobotsFilter에 포함
- [ ] 3.4 캐시 TTL 24시간 만료 체크 로직 (`time.Since(entry.fetchedAt) > 24*time.Hour`이면 재조회)
- [ ] 3.5 robots.txt fetch 유틸: `https://<host>/robots.txt` HTTP GET (Pioneer 공유 fetcher 사용, 타임아웃 5초)
- [ ] 3.6 상태 코드 분기: 200 → 파싱, 404 → 빈 rules로 캐시, 5xx/network/timeout → `failOpen=true`로 캐시
- [ ] 3.7 robots.txt 파서 구현: User-agent 블록 분리, `FugueBot` 블록 우선 / `*` fallback, `Disallow:` 경로 수집, `Crawl-delay:` 초 단위 파싱 (파싱 불가 값은 무시)
- [ ] 3.8 Filter 메서드: 각 링크의 host를 뽑아 캐시 조회/fetch → `failOpen` 또는 빈 rules면 통과 → rules와 링크 경로 prefix 매칭으로 Disallow 판정 → 매칭 시 제거
- [ ] 3.9 Crawl-delay 연동: fetch 완료 후 파싱된 `Crawl-delay`(초)가 있으면 `scheduler.SetHostRate(host, 1/delay, 1)` 호출 (캐시 hit 시 재호출하지 않음)
- [ ] 3.10 scheduler 인스턴스 주입 경로: RobotsFilter 생성자가 `HostRateSetter` 인터페이스를 받아 약한 결합 유지
- [ ] 3.11 테스트:
  - Disallow 매칭 시 링크 제거
  - rules 없을 때 통과
  - `FugueBot` 블록 우선 / `*` fallback
  - 404 응답 → 모두 통과
  - 5xx/timeout → fail-open(모두 통과)
  - TTL 만료 후 재fetch 발생
  - `Crawl-delay: 5` → `SetHostRate(host, 0.2, 1)` 호출 검증 (mock scheduler)
  - 파싱 불가능한 Crawl-delay는 `SetHostRate` 미호출

## 4. FilterChain 순서 고정 (Domain → Extension → PathPattern → Robots → Dedup)

- [ ] 4.1 Pioneer 초기화 코드에서 `NewFilterChain(...)` 호출 위치를 찾아 필터 인자 순서를 `&DomainFilter{...}, &ExtensionFilter{}, &PathPatternFilter{}, robotsFilter, dedupFilter`로 변경
- [ ] 4.2 기존 `DomainFilter → ExtensionFilter → PathPatternFilter → CanonicalDedupFilter`(archive impl) 구성을 RobotsFilter가 Dedup 앞에 삽입된 형태로 교체
- [ ] 4.3 순서 보호용 통합 테스트: 체인 구성 검증 또는 각 필터가 호출된 횟수·순서를 mock으로 확인

## 5. Pioneer Run 루프 플로우 강제 (ParseLinks → FilterLinks → Enqueue)

- [ ] 5.1 `pioneer.go` Run 루프에서 `ExtractLinksWithSelectors` 결과가 반드시 `FilterChain.Apply`를 거쳐 `URLPriorityQueue.Enqueue`되는 코드 경로를 확인
- [ ] 5.2 필터를 우회하여 Enqueue하는 경로 존재 여부를 `grep`으로 검색하여 제거 또는 필터 경유로 리팩터
- [ ] 5.3 Redirect chain 처리: 필터 입력 URL이 `fetchHTML`의 `finalURL`인지 재확인 (기존 pioneer.go 경로 유지)
- [ ] 5.4 통합 테스트: 가짜 HTML → ParseLinks → FilterChain(모두 차단) → `Enqueue` 호출 0회
- [ ] 5.5 통합 테스트: redirect가 있는 fetch → 최종 URL만 필터/canonicalization에 투입

## 6. archive impl tasks와의 관계

archive impl에서 정의된 다음 tasks는 **본 change가 대체**한다.
- archive §2 (DomainFilter 구조체 및 Filter 메서드) → 본 change §1
- archive §1.2 / §6.2 (canonicalURL 정규화 규칙) → 본 change §2

archive impl에서 정의된 다음 tasks는 **변경 없이 유지**된다.
- archive §3 (ExtensionFilter)
- archive §4 (PathPatternFilter)
- archive §5 (CanonicalDedupFilter 내부 로직)
- archive §1.3–1.4 (VisitedLink, semanticPriorityModifier)
- archive §6.3–6.7 (위 항목들의 기존 테스트)

## 7. 검증

- [ ] 7.1 `go test ./apps/api/internal/bot/...` 전체 통과
- [ ] 7.2 `openspec validate pioneer-link-filter-policy --strict` 통과
- [ ] 7.3 baseline spec(`openspec/specs/bot/spec.md`)의 DomainFilter / canonicalURL / 필터 체인 요구사항이 본 change의 MODIFIED 정의로 대체되었는지 `openspec show pioneer-link-filter-policy`로 확인

## 8. 문서

- [ ] 8.1 `AGENTS.md` 또는 bot 관련 문서에 교차 사이트 크롤 정책, Allow/Deny 설정, robots.txt 정책(24h 캐시, fail-open, Crawl-delay 연동) 기재
