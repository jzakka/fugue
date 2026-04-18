## Context

Pioneer의 링크 필터 체인은 archive change `2026-04-13-pioneer-link-filter-impl`(이하 "archive impl")에서 `DomainFilter → ExtensionFilter → PathPatternFilter → CanonicalDedupFilter` 구조로 확립되었다. archive impl은 같은 루트 도메인 내 BFS 크롤을 전제로 했으나, Fugue는 음악·일러스트·영상·글을 분야를 넘나들며 수집하는 크로스미디어 플랫폼이다. 즉 archive impl의 DomainFilter는 제품 방향과 구조적으로 충돌한다.

또한 archive impl의 `canonicalURL()`은 트래킹 파라미터·www·trailing slash·fragment만 다루고, scheme/host 대소문자나 default port, query 파라미터 순서는 정규화하지 않는다. 외부 사이트를 크롤할 때 필요한 robots.txt 존중 로직도 없다.

본 change는 archive impl의 해당 요구사항을 **덮어쓰는 MODIFIED 방식**으로 정책 계층을 보강한다. 실제 Go 코드 수정은 본 change의 tasks.md가 정의하는 범위에 포함된다.

## Goals / Non-Goals

**Goals:**
- DomainFilter를 "루트 도메인 고정"에서 "Allow/Deny 키워드 + 교차 사이트 기본 허용"으로 재정의한다.
- canonicalURL에 scheme 소문자·default port 제거·query 정렬 3가지 규칙을 추가한다.
- RobotsFilter를 신규 도입하여 robots.txt Disallow를 존중하고 Crawl-delay를 `scheduler-host-token-bucket`에 전달한다.
- 필터 체인의 고정 순서를 `Domain → Extension → PathPattern → Robots → Dedup`으로 스펙에 못박는다.
- Pioneer Run 루프가 `ParseLinks → FilterLinks → Enqueue` 경로만 허용하도록 강제한다.
- Redirect chain의 **최종 URL만** 필터/canonicalization 대상으로 삼는다.

**Non-Goals:**
- archive impl의 ExtensionFilter / PathPatternFilter / semanticPriorityModifier 자체 정의 변경. 본 change는 DomainFilter·canonicalURL·필터 순서·Pioneer 플로우·RobotsFilter만 건드린다.
- `scheduler-host-token-bucket`의 bucket 알고리즘 자체. 본 change는 `SetHostRate(host, 1/delay, 1)` 호출로 Crawl-delay를 주입하는 지점까지만 책임진다.
- 분산 환경에서의 robots.txt 캐시 공유. 프로세스별 인메모리 캐시를 전제로 한다.
- robots.txt의 Sitemap 지시어 활용. Disallow와 Crawl-delay만 다룬다.

## Decisions

### D1. archive impl 요구사항의 "un-archive by MODIFIED" 처리

**결정**: archive 디렉터리는 물리적으로 옮기지 않고 기록으로 남긴다. 본 change의 spec delta(`specs/bot/spec.md`)에 `## MODIFIED Requirements`로 archive impl이 추가했던 DomainFilter / canonicalURL / 필터 체인 요구사항을 override한다. 본 change가 archive로 이동하는 시점에 최종 spec baseline에는 본 change의 정의가 남는다.

**사유**: archive impl은 과거 구현 기록이다. 요구사항 자체는 살아 있으므로, baseline spec을 덮어쓰는 표준적인 OpenSpec MODIFIED 흐름을 따른다. 별도의 "un-archive" 절차(디렉터리 이동)는 OpenSpec 워크플로우에서 지원되지 않아 피한다.

### D2. DomainFilter 재정의: Allow/Deny 키워드 + 교차 사이트 기본 허용

**결정**: `DomainFilter`의 필드를 아래와 같이 교체한다.
- `RootDomain string` **제거**.
- `AllowKeywords []string` **추가**.
- `DenyKeywords []string` **추가**.

매칭 로직:
1. 링크 호스트를 lowercased하고 `www.` 접두어를 제거한다.
2. 정규화된 호스트가 DenyKeywords 중 하나라도 substring으로 포함하면 **제거**.
3. AllowKeywords가 비어 있으면 Deny에 걸리지 않은 모든 링크 **통과**(기본 허용).
4. AllowKeywords가 비어 있지 않으면, Allow 중 하나라도 substring으로 포함해야 **통과**.

**대안과 사유**:
- (A) same-site-only 유지 → Fugue의 크로스미디어 비전과 모순. 기각.
- (B) 무조건 교차 허용 → 광고/스팸 네트워크 차단 수단 부재. 기각.
- (C) **Allow/Deny 키워드 + 기본 허용** → substring 매칭으로 단순하고 운영 중 설정 한 줄로 범위 조절 가능. **채택**.

**국가별 TLD**: Allow/Deny는 substring 매칭 그대로 유지한다. `.co.kr`, `.co.jp` 같은 국가별 TLD를 특별 취급하지 않는다(Open Question 종결). 운영 중 필요시 키워드 리스트에 명시적으로 추가한다.

### D3. canonicalURL 확장 3규칙

**결정**: archive impl의 규칙에 다음 세 가지를 추가한다.
- **scheme 소문자화**: `HTTPS://Example.com` → `https://example.com` (scheme 부분만 소문자, 경로 case는 보존).
- **default port 제거**: `http`의 `:80`, `https`의 `:443`만 제거. non-default 포트(예: `:8080`)는 보존.
- **query 파라미터 이름순 오름차순 정렬**: 트래킹 파라미터 제거 후 남은 파라미터를 key 오름차순으로 재인코딩. 예: `?b=2&a=1` → `?a=1&b=2`.

Go에서는 `url.Values.Encode()`가 이미 key 사전순 정렬을 수행하므로, 트래킹 파라미터 제거 후 재인코딩 경로로 자연스럽게 만족한다.

**확장 후 최종 규칙 순서**:
1. URL 파싱
2. scheme을 lowercase
3. host를 lowercase + `www.` 제거 + default port 제거
4. fragment 제거
5. query에서 트래킹 파라미터 제거 후 key-sorted encode
6. 루트가 아닌 경로의 trailing slash 제거

**사유**: 같은 리소스가 파라미터 순서·호스트 대소문자·default port 유무로 서로 다른 해시로 빠져 중복 노드가 생기는 원인을 제거한다. 이 정규화는 링크 필터의 Dedup 계층과 Pioneer frontier의 `url_hash` 키 정의에 공통으로 사용된다.

**주의**: 본 change의 canonicalURL 확장은 "link filter의 canonical"에 한정한다. 그래프 노드 패턴화(숫자 segment → `{id}`)는 별개 레이어이며 기존 bot spec의 "그래프 노드와 엣지를 관리한다" 요구사항은 영향받지 않는다.

### D4. RobotsFilter 신규: lazy fetch + TTL 24h + fail-open

**결정**: RobotsFilter는 `LinkFilter` 인터페이스를 구현한다. 호스트별 인메모리 맵을 소유하며 각 호스트에 대해 다음 구조를 캐시한다.

```
host → {
  rules: []DisallowRule,   // User-agent 블록에서 파싱된 Disallow 경로 목록
  crawlDelay: *float64,    // 파싱된 Crawl-delay(초). nil이면 없음
  fetchedAt: time.Time,    // 캐시 기록 시각
  failOpen: bool,          // fetch 실패/응답 이상 시 true
}
```

**Fetch 절차**:
1. 호스트가 캐시에 없거나 `fetchedAt + 24h < now()`이면 `https://<host>/robots.txt`에 HTTP GET을 건다.
2. 응답 상태 분기:
   - **200**: body를 파싱한다. User-agent 블록을 순회하여 `FugueBot`이 있으면 그 블록을 사용, 없으면 `*` 블록을 사용(둘 다 없으면 빈 rules).
   - **404**: "규칙 없음"으로 해석. rules는 빈 배열로 캐시.
   - **5xx / 네트워크 오류 / 타임아웃**: `failOpen=true`로 캐시.
3. 캐시 기록 후 TTL 24시간.

**Filter 절차**: 각 링크의 호스트를 뽑아 캐시 조회/fetch → `failOpen`이면 통과 → rules와 링크 경로를 매칭해 Disallow면 제거, 아니면 통과.

**User-agent 블록 처리**:
- robots.txt의 `User-agent: FugueBot` 섹션이 존재하면 그 섹션의 Disallow/Crawl-delay만 사용.
- 없으면 `User-agent: *` 섹션을 fallback으로 사용.
- 두 섹션을 동시에 머지하지 않는다(RFC 9309 권고).

**캐싱 TTL 정책**:
- 24시간 고정. 사이트 정책 변경 반영 지연과 재조회 비용의 균형.
- fail-open 상태 역시 같은 TTL을 적용해 연속 재시도 폭주를 막는다.
- 향후 정책 변경 가능(env 노출은 안 함).

**사유**:
- fail-open: 외부 사이트 일시 장애가 Pioneer 전체를 멈추게 하면 안 된다.
- 영구 캐시는 사이트 정책 변경을 반영 못한다. 24h가 업계 관행.

### D5. Crawl-delay → `scheduler-host-token-bucket.SetHostRate`

**결정**: RobotsFilter가 robots.txt에서 `Crawl-delay: N`(초)를 파싱하면, 파싱 직후 `scheduler.SetHostRate(host, 1/N, 1)`을 호출한다. burst는 `1`로 고정한다.

**사유**:
- `scheduler-host-token-bucket` 스펙(`openspec/DECISIONS.md` §5)이 이미 `SetHostRate(host, rate, burst)` 시그니처를 제공한다.
- 1 req / N sec = rate `1/N` req/sec로 변환. 예: `Crawl-delay: 5` → `SetHostRate(host, 0.2, 1)`.
- Crawl-delay 미지정 호스트는 호출하지 않고 scheduler의 기본 rate(1 req/sec, burst 5)를 유지한다.
- 파싱 불가능한 Crawl-delay 값(문자열 등)은 무시하여 fallback.

### D6. 필터 순서: Domain → Extension → PathPattern → Robots → Dedup

**결정**: Pioneer 기본 FilterChain은 이 순서로 고정한다.

**비용 모델 근거**:
| 필터 | 비용 (cache hit 가정) | 위치 근거 |
|------|-----------------------|-----------|
| Domain | O(n_keywords) in-memory substring | 가장 싸고 선택률 높음. 맨 앞 |
| Extension | O(1) regex/suffix | in-memory, 호스트와 무관 |
| PathPattern | O(n_patterns) regex | in-memory |
| Robots | cache hit: O(n_rules) in-memory / miss: HTTP GET | cache miss 시만 네트워크. 앞의 3개로 이미 줄어든 후보에 대해서만 평가 |
| Dedup | DB/shared map 조회 | 가장 비쌈. 가장 뒤 |

**Robots filter ↔ host-token-bucket 순서**: 두 제어는 단계가 다르다.
- **Enqueue 단계**(본 change): 링크를 frontier에 넣기 전에 RobotsFilter가 Disallow를 거른다. 동시에 Crawl-delay를 `SetHostRate`로 scheduler에 주입한다.
- **Claim 단계**(scheduler-claim-api, DECISIONS §3): frontier에서 URL을 꺼낼 때 `HostRateLimiter.Allow(host)`가 token bucket을 확인한다.

즉 Robots는 **의미적 차단**(해당 URL을 크롤하면 안 됨), host bucket은 **속도 제어**(해당 호스트에 너무 자주 가면 안 됨)로 책임이 분리된다.

### D7. Redirect chain: 최종 URL만 체크

**결정**: `pioneer.go`의 `fetchHTML`가 반환하는 `finalURL`에 대해서만 필터 체인/canonicalization을 적용한다. 중간 redirect URL(예: `301 Location: ...` 체인 중간 URL)은 검사 대상에서 제외한다.

**사유**:
- 이미 `pioneer.go`는 `finalURL`을 노드 해시 계산(`finalHash := hashURL(finalURL)`)과 링크 추출(`ExtractLinksWithSelectors(html, finalURL)`)의 기준으로 사용한다.
- 필터도 같은 기준을 쓰는 것이 일관적이고, 중간 redirect의 Disallow 판정은 실익이 낮다(최종 URL이 허용되면 접근 가능한 리소스).
- redirect chain 전체 검증은 구현 복잡도가 크고 사이트별 로그인/리디렉션 패턴에 false positive를 일으킨다.

### D8. semanticPriorityModifier: archive impl 정의 유지

**결정**: archive impl의 `semanticPriorityModifier`(footer/aside=-50, nav/header=-20, else=0)는 **정의 그대로 유지**한다. 본 change는 이 함수의 계산 규칙을 바꾸지 않는다. 단, Pioneer 루프가 반환값을 실제 우선순위 점수에 가산하는지 여부는 본 change의 Pioneer 플로우 정책 범위에서 재확인된다.

### D9. archive impl의 다른 requirement 유지 여부

다음 archive impl 요구사항은 **변경 없이 유지**한다.
- ExtensionFilter
- PathPatternFilter
- CanonicalDedupFilter (내부적으로는 확장된 canonicalURL을 호출)
- semanticPriorityModifier (위 D8)

다음만 **MODIFIED로 대체**한다.
- DomainFilter (D2)
- canonicalURL (D3)
- 필터 체인 요구사항 (D6: 순서에 RobotsFilter를 삽입)

다음은 **ADDED**.
- Pioneer Run 루프 플로우(ParseLinks → FilterLinks → Enqueue)
- RobotsFilter 본체
- RobotsFilter의 lazy fetch + 24h 캐시
- RobotsFilter의 fail-open 정책
- RobotsFilter의 Crawl-delay surface (`SetHostRate` 호출 포함)

## Risks / Trade-offs

- **[교차 사이트 크롤 예산 폭발]** → scheduler-host-token-bucket과 pioneer-worker-budget이 이미 예산 제어를 담당. 운영 중 Deny 리스트 확장으로 대응.
- **[robots.txt fail-open으로 인한 정중함 위반]** → fetch 실패 호스트는 TTL 24h 동안 재조회하지 않아 폭주 방지. 운영 로그로 모니터링 가능.
- **[Crawl-delay 비현실적 큰 값(예: 300초)]** → 본 change는 scheduler `SetHostRate`에 그대로 전달. 상한 clamp는 scheduler 측에서 결정(본 change 범위 외).
- **[canonicalURL 확장이 기존 visited 맵 키와 충돌]** → archive impl은 `visited` 맵 키를 `hashURL(l.URL)`(원본), `seen` 맵 키를 `hashURL(canonicalURL(l.URL))`(정규화)로 분리 유지 중. 본 change의 canonical 확장은 `seen` 쪽만 영향. 두 맵의 역할 분리는 유지.
- **[프로세스별 인메모리 robots 캐시]** → 다중 Pioneer 인스턴스에서 같은 호스트 중복 조회 가능. 24h TTL로 유의미한 부담 아님. 분산 캐시는 후속 과제.
- **[Redirect chain 중간 URL 미검증]** → Disallow 경로로 리디렉션 당하는 드문 케이스에 false negative 가능. 최종 URL이 허용된다면 실익 낮아 수용.
- **[RobotsFilter와 `SetHostRate` 중복 호출]** → 같은 호스트 Crawl-delay는 TTL 내에 최초 1회만 호출되도록 구현(캐시 hit 시 skip).

## Migration Plan

1. 본 정책 change(`pioneer-link-filter-policy`)를 머지한다. tasks.md의 작업을 본 change 내에서 수행한다.
2. `link_filter.go`를 다음 순서로 수정한다.
   - `DomainFilter.RootDomain` 필드 제거 → `AllowKeywords`, `DenyKeywords` 추가.
   - `canonicalURL()` 확장 3규칙 구현.
   - `RobotsFilter` 타입 및 캐시 로직 추가.
   - 테스트 업데이트.
3. `pioneer.go` 초기화 경로에서 FilterChain을 `Domain → Extension → PathPattern → Robots → Dedup`로 구성. `DomainFilter{AllowKeywords: nil, DenyKeywords: defaultDeny}` 기본값.
4. `scheduler-host-token-bucket`의 `SetHostRate` 호출 지점을 RobotsFilter fetch 완료 hook에 연결.
5. 롤백: DomainFilter 변경이 breaking이므로, 롤백 시 `RootDomain` 복구와 호출부의 Allow/Deny 제거가 필요. 커밋을 필터 단위로 분리 유지.
6. 후속 아카이브: 본 change가 archive로 이동되면, baseline spec(`openspec/specs/bot/spec.md`)에는 본 change의 정의가 남는다. archive impl 디렉터리(`openspec/changes/archive/2026-04-13-pioneer-link-filter-impl/`)는 히스토리로 그대로 보존.

## Open Questions (closed)

- **Allow/Deny 키워드 매칭 방식** → substring 매칭으로 확정. 국가별 TLD 특별 처리 없음.
- **RobotsFilter HTTP 클라이언트** → Pioneer의 공유 fetcher(`apps/api/internal/bot/fetchHTMLShared`와 동일 경로의 클라이언트) 재사용. 전용 타임아웃(5s) 적용.
- **Crawl-delay 미지정 시 처리** → `SetHostRate` 호출하지 않음. scheduler의 기본 rate(1 req/sec, burst 5)가 적용.
