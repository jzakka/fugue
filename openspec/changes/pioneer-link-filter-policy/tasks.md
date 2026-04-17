## 1. DomainFilter 재정의 (Allow/Deny 키워드)

- [ ] 1.1 `DomainFilter` 구조체 필드를 `RootDomain` 단일 필드에서 `AllowKeywords []string`, `DenyKeywords []string`로 교체
- [ ] 1.2 Filter 로직을 "호스트 substring 매칭(대소문자 무시, www 제거)"으로 재작성: Deny 우선 → Allow 비어 있으면 통과 → 그 외 Allow 매칭만 통과
- [ ] 1.3 기존 DomainFilter 호출부(Pioneer 초기화, 테스트 픽스처) 전수 조사 및 Allow/Deny 리스트로 마이그레이션
- [ ] 1.4 기본 설정값: Allow=[], Deny=[] (= 교차 사이트 기본 허용)을 Pioneer 초기화 기본 경로에 반영
- [ ] 1.5 단위 테스트: 기본 허용, Deny 매칭, Allow 화이트리스트, Deny 우선, www 무시 6개 시나리오 커버

## 2. canonicalURL 확장

- [ ] 2.1 scheme 소문자화 로직 추가
- [ ] 2.2 host 소문자화 확인(기존 `strings.ToLower` 경로 유지) 및 default port(http:80, https:443) 제거 로직 추가
- [ ] 2.3 query 파라미터 알파벳 순 정렬(`url.Values.Encode()`가 이미 정렬하므로 파라미터 제거 후 재인코딩 확인)
- [ ] 2.4 기존 트래킹 파라미터 제거 / www 제거 / trailing slash / fragment 제거 동작 회귀 테스트 통과 확인
- [ ] 2.5 단위 테스트 추가: scheme 대문자, default port(80/443), non-default port 보존, query 정렬, fragment, 루트 "/" 보존

## 3. RobotsFilter 신규 구현

- [ ] 3.1 `RobotsFilter` 구조체 선언 및 `LinkFilter` 인터페이스 구현 (apps/api/internal/bot/robots_filter.go 또는 link_filter.go 내)
- [ ] 3.2 호스트별 인메모리 캐시 구조 정의: `map[host]robotsCacheEntry { rules, crawlDelay, fetchedAt, failOpen bool }`
- [ ] 3.3 캐시 TTL 24h 만료 체크 및 재조회 로직
- [ ] 3.4 robots.txt fetch 유틸: `https://<host>/robots.txt`로 HTTP GET, 타임아웃 설정, 상태 코드별 분기(200: 파싱, 404: 규칙 없음, 5xx/network error: fail-open)
- [ ] 3.5 robots.txt 파서: `User-agent` 블록 구분, `FugueBot` 우선 `*` fallback, `Disallow` 경로 수집, `Crawl-delay` 초 단위 파싱(파싱 불가 시 무시)
- [ ] 3.6 Filter 본문: 각 링크 호스트별로 캐시 조회/fetch → 경로 매칭 → Disallow에 걸리면 제거, 아니면 통과
- [ ] 3.7 Crawl-delay surface API: `(host) → (delay seconds, ok)` 형태의 호스트별 조회 메서드 추가 (scheduler-host-token-bucket이 사용)
- [ ] 3.8 단위 테스트: Disallow 매칭 차단, 규칙 없음 통과, FugueBot 블록 우선, `*` fallback, 404 허용, 5xx fail-open, 타임아웃 fail-open, TTL 만료 재조회, Crawl-delay 파싱

## 4. 필터 체인 순서 고정

- [ ] 4.1 Pioneer 초기화 코드에서 `NewFilterChain(...)` 호출 시 필터 순서를 `Domain → Extension → PathPattern → Robots → CanonicalDedup`으로 고정
- [ ] 4.2 순서가 바뀌지 않도록 보호하는 통합 테스트 추가 (체인 구성 검증 또는 링크 흐름 관찰)

## 5. Pioneer 루프 정책 강제

- [ ] 5.1 Pioneer Run 루프에서 `ParseLinks` 결과가 반드시 `FilterChain.Apply`를 거쳐 Enqueue되는지 코드 경로 확인
- [ ] 5.2 우회 경로(필터 없이 Enqueue) 존재 여부를 검색하여 제거
- [ ] 5.3 통합 테스트: 가짜 HTML → ParseLinks → FilterChain(모두 차단) → Enqueue 호출 0회 확인

## 6. Semantic priority modifier 점수 반영

- [ ] 6.1 Pioneer 크롤 루프에서 각 링크의 우선순위 점수 계산 지점을 찾아 `semanticPriorityModifier(link)` 반환값을 가산
- [ ] 6.2 가산이 실제 우선순위 큐의 Enqueue 점수에 반영되는지 단위 또는 통합 테스트로 증명 (footer 링크의 상대 우선순위가 본문 링크보다 낮음)

## 7. scheduler-host-token-bucket 연동 확인

- [ ] 7.1 RobotsFilter의 Crawl-delay surface API 서명을 `scheduler-host-token-bucket` 스펙과 맞춤 확인
- [ ] 7.2 scheduler 측이 해당 API를 호출하여 rate를 조정하는지 후속 변경(또는 연동 테스트)에서 검증

## 8. 문서 / 마이그레이션

- [ ] 8.1 `AGENTS.md` 또는 bot 관련 문서에 교차 사이트 크롤 정책과 Allow/Deny 설정 방법 기재
- [ ] 8.2 구현 완료 후 본 변경을 아카이브(`openspec/changes/archive/2026-04-13-pioneer-link-filter-impl/` 규칙에 맞춘 날짜 슬러그 디렉터리)
- [ ] 8.3 bot capability 스펙(`openspec/specs/bot/spec.md`)에 ADDED/MODIFIED 요구사항이 반영되었는지 `openspec` 도구로 확인
