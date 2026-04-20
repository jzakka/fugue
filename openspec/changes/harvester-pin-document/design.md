## Context

Harvester는 본래(`apps/api/fuguebot_pseudo.go`) `fetch → ParseDocument → Index` 라는 단순한 한 줄짜리 의도를 가진 컴포넌트다. 그러나 직전 정본(`openspec/changes/archive/2026-04-15-perfect-harvester/`) 이후 실제 구현(`apps/api/internal/bot/harvester.go`, `harvest_pipeline.go`, `goja_executor.go`)은 다음 두 가지 결정을 따라 갈라졌다.

- `ParseDocument` ≈ DB에 저장된 사이트별 JS 스크립트를 `goja`로 실행해서 RawItem(`title`, `mediaURL`, `mediaType`, `sourceURL`)을 추출한다.
- `Index` ≈ RawItem 단위로 미디어를 다운로드해 S3에 올리고 Pin을 한 건 만든다.

이 결정은 두 가지 부작용을 만든다.

1. **Pin이 무엇인가가 흐려진다.** Pin은 검색·추천·중복 제거가 의존하는 정본 문서지만, 현재 정의는 "스크립트가 추출한 미디어 한 건"이다. 같은 페이지에서 N개의 미디어가 추출되면 N개의 Pin이 만들어지고, 페이지에 대한 정본(canonical title, body, og)은 어디에도 저장되지 않는다.
2. **default 경로가 site-specific이다.** 일반 웹 페이지는 OG/Twitter Card/JSON-LD/`<article>`만으로도 Pin 문서로 변환 가능하지만, 현재는 사이트별 JS가 없으면 Pin이 0개다. 미등록 도메인은 그래프 노드로만 남는다.

본 변경은 Harvester의 primary contract를 "**HTML을 한 페이지에 한 개의 Pin 문서로 변환**"으로 재정의한다. 기존 JS 스크립트 경로는 제거하지 않고, `PerSiteAdapter` 추상의 한 가지 구현(ScriptAdapter)으로 강등한다.

본 change는 다음 인접 change들과 협업한다(범위 외):
- `harvester-scheduler-consumer`: Harvester를 사이트 단위 BFS에서 frontier consumer 루프로 재정의. 본 change의 `ParseDocument`/`Index`를 호출하는 쪽이며, 본 change의 upsert 결과 `pin_id`를 scheduler의 `SetStatus(key, "harvested", pinIDs)`에 전달해 `harvester_frontier_pins` 조인 테이블에 기록하는 책임을 갖는다.
- `harvester-snapshot-first-fetch`: HTML 본문을 어디서 가져올지(snapshot vs live). 본 change는 HTML을 문자열로 제공받는 consumer 쪽 인터페이스(`HTMLFetcher.Get(url) (html, fetchURL, err)` 형태)를 가정하며, fetch 구현 자체는 정의하지 않는다.
- `harvester-image-cache`: 추출된 `media_candidates`의 캐싱 정책.
- `harvester-worker-budget`: 워커 종료 조건.

## Goals / Non-Goals

**Goals:**
- Harvester의 정본 경로(`ParseDocument` → `Index`)를 사이트 무관하게 정의하고 명세화한다.
- 일반 HTML(OG/Twitter/JSON-LD/`<article>`/title)만으로도 Pin이 생성되도록 한다.
- 같은 원본 페이지에 대해 봇은 항상 정확히 한 개의 Pin만 가진다(canonical 기준).
- listing 페이지/빈 페이지/저품질 페이지를 Pin으로 만들지 않고 사유를 분류한다.
- 사이트별 JS 스크립트가 default가 아니라 per-site override임을 명시한다(기존 ScriptExecutor 자체의 동작 계약은 유지).
- 검색 SSOT로서 Pin이 가져야 할 최소 필드(`title`, `body_text`, `canonical_url`, `thumbnail_url`, `media_candidates`, `lang`, `author`, `published_at`)를 정의한다.

**Non-Goals:**
- frontier 큐/스케줄러 자체의 정의(별도 `scheduler-*` change).
- Harvester 워커의 라이프사이클·종료 조건(별도 `harvester-worker-budget`).
- HTML/이미지 캐시 정책(별도 `harvester-image-cache`, `harvester-snapshot-first-fetch`).
- 한 페이지에서 추출된 N개 `media_candidates`를 N개의 별도 Pin으로 분리하는 것 (현재는 정본 1 Pin + `og_data.media_candidates` 보관까지만 정의).
- 본문 텍스트의 의미 분석/태깅/임베딩(별도 추후 change).
- 비-HTML 콘텐츠(JSON API, RSS 등)의 직접 처리.

## Decisions

### Decision 1: Pin = 페이지 단위 정본 문서 (canonical-URL 멱등 upsert)

**Chosen**: 봇이 만드는 Pin은 "원본 페이지에 대한 단일 정본 문서"로 정의한다. `pins(url)`에 `WHERE creator_id = <BotCreatorID>` partial unique index를 추가해, 같은 canonical URL에 대해 봇 Pin이 정확히 한 개만 존재하도록 강제한다. 같은 URL을 다시 harvest하면 update 된다(upsert).

**Alternatives considered**:
- *추출된 미디어 한 건 = Pin 한 건* (현행): 검색 정본이 분산되고, 같은 페이지가 N개 Pin으로 쪼개진다. → 검색·중복 제거가 어려워짐. 기각.
- *전역 unique on `pins(url)`*: ERD 설계 결정("URL 유니크 제약 없음, 큐레이션이므로 여러 사람이 같은 작품을 핀 가능")과 충돌. 기각.
- *별도 `bot_pins` 테이블*: 검색 인덱스를 두 테이블로 분기시켜야 함. SSOT 단일성 깨짐. 기각.

**Rationale**: ERD의 user-pin 정책을 유지하면서 봇 Pin만 멱등하게 만들 수 있다. partial unique index는 PostgreSQL이 직접 지원하므로 애플리케이션 race를 피할 수 있다.

### Decision 2: Generic HTML→Pin extractor를 default 경로로

**Chosen**: 어떤 HTML이든 다음 fallback 체인으로 Pin 후보를 만든다.

```
title:
  og:title → twitter:title → <h1> in <article> → <title> tag
body_text:
  <article> textContent → schema.org/Article.articleBody (JSON-LD) →
  최대 텍스트 밀도 블록 → og:description → meta[name=description]
canonical_url:
  <link rel=canonical> → og:url → fetch URL
  (단, 채택 후보의 호스트가 fetch URL 호스트와 다르면 해당 후보는 건너뛰고 다음 fallback — cross-domain canonical 무시가 extractor 내부에서 일어난다)
thumbnail_url:
  og:image → twitter:image → schema.org image → 본문 범위 첫 <img>
media_candidates:
  본문 범위 내 <img>/<video>/<audio>/<source> URL 수집 (절대화)
  본문 범위 정의: <article> 태그가 있으면 그 내부, 없으면 <body> 전체
lang:
  <html lang> → og:locale
author:
  schema.org Author → meta[name=author] → og:article:author
published_at:
  schema.org datePublished → article:published_time → <time datetime>
```

**Alternatives considered**:
- *오로지 site-specific 스크립트만*: 현행. 미등록 도메인 미지원. 기각.
- *Readability 외부 라이브러리에 위임*: 본문 추출은 잘 하지만 OG/JSON-LD/canonical은 별도 처리 필요. 의존 복잡도만 늘어나고 결과는 동일. 향후 본문 블록 식별 보강 시 검토 가능하나 본 change는 표준 파서(`golang.org/x/net/html`) + 명시적 fallback 체인을 채택.

**Rationale**: OG/Twitter/JSON-LD는 실제 웹 표준 채택률이 높고, 셋 모두 없는 페이지는 대체로 listing 또는 저품질 페이지여서 classifier에서 거른다.

### Decision 3: Content classifier가 Pin 생성 여부와 사유를 결정

**Chosen**: 추출 결과에 다음 3가지 사유 중 하나라도 해당되면 Pin을 만들지 않고 frontier row의 `harvested_at`만 마킹한다(반복 fetch 방지).

| 사유 | 판정 기준 |
|------|-----------|
| `listing` | 단어 수 > 0이고 `링크 수 / 단어 수 > threshold_link_density` (단일 공식, 기본 임계값은 설정값). 단어 수가 0이면 listing 판정 없이 다음 사유로 넘어간다(division-by-zero 회피) |
| `empty_body` | `body_text`가 임계 길이(설정값, 기본 200 bytes; Go `len([]byte)` 기준) 미만 |
| `no_primary_media` | `thumbnail_url`이 없고 `media_candidates`도 비어 있고 `body_text`도 임계 길이 미만 (콘텐츠 동시 부재) |

분류는 우선순위가 있다: `listing` > `empty_body` > `no_primary_media` (가장 명확한 사유부터). 하나라도 매치되면 후속은 평가하지 않는다. Classifier의 입력은 `PinDocument`뿐이며 외부 상태(node_type 등)에 의존하지 않는다 — 이는 scheduler-consumer가 노드 타입 정보 없이 frontier URL만 넘겨주는 새 흐름과 일관된다.

`low_text_link_ratio`는 본 change에서 제거한다(`listing`의 단일 공식 `링크 수 / 단어 수 > threshold_link_density`로 통합 흡수됨). `body_text` 길이 단위는 바이트(Go `len([]byte)` 기본)로 통일한다. 단, classifier는 잘리지 않은 **원본 body_text** 길이를 평가하며, `pins.description`에 저장되는 잘린 형태(500 rune)와는 별개 값이다.

**Alternatives considered**:
- *모든 페이지를 Pin으로 만들고 낮은 score 부여*: 검색 결과 품질이 떨어지고 인덱스가 비대해짐. 기각.
- *사유 없이 boolean (`pinnable: yes/no`)*: 운영/디버깅 시 "왜 Pin이 안 만들어졌는지" 추적 불가. 기각.

**Rationale**: 사유는 metric/log로 노출되어 frontier 우선순위 학습이나 site-specific override 결정의 근거가 된다.

### Decision 4: PerSiteAdapter / AdapterRegistry로 site-specific 경로를 일급화

**Chosen**: 다음 인터페이스를 도입한다.

```go
type PerSiteAdapter interface {
    Domain() string                    // 매칭할 도메인 (또는 와일드카드)
    Extract(ctx context.Context, html string, fetchURL string) (PinDocument, error)
}

type AdapterRegistry interface {
    Resolve(domain string) (PerSiteAdapter, bool)
    Register(adapter PerSiteAdapter)
}
```

Harvester는 fetch 후 다음 순서를 따른다.

```
0. html, fetchURL, err := htmlFetcher.Get(frontierURL)   // 인터페이스는 harvester-snapshot-first-fetch가 제공
1. adapter, ok := registry.Resolve(domain(fetchURL))
2. if ok:
       doc, err = adapter.Extract(ctx, html, fetchURL)
       if err != nil: fall back to generic extractor (AdapterFallback++)
   else:
       doc = genericExtractor.Extract(ctx, html, fetchURL)
   # cross-domain canonical 무시는 extractor 내부에서 일어나므로 여기서는 추가 처리하지 않는다
3. classify(doc) → if not pinnable: mark harvested_at, return
4. canonicalUpsert(doc) → pin_id
5. scheduler.SetStatus(key, "harvested", []uuid{pin_id})  // harvester_frontier_pins 조인 기록은 scheduler-consumer 소유
```

기존 `GojaExecutor`는 신규 `ScriptAdapter`로 래핑해 레지스트리에 등록한다. ScriptAdapter는 DB의 (site_id, node_type) 스크립트를 로드해 실행하고, 결과 RawItem 1건당 PinDocument 1건이 아니라 **첫 번째 RawItem을 정본 PinDocument(title, thumbnail_url, body_text, description 등 모든 메타 필드)로, 나머지 RawItem들은 `og_data.media_candidates` 배열에 추가**하는 방식으로 1 페이지 = 1 Pin 계약을 유지한다. 즉 ScriptAdapter는 N → 1 축약의 정본 선택 규칙이 "첫 RawItem"으로 고정된다.

**Alternatives considered**:
- *어댑터 매칭 시 generic extractor 결과를 무시*: 어댑터가 부분 정보만 줄 때(예: 미디어만 추출) 메타가 누락된다. → "어댑터 결과 우선, 누락 필드는 generic으로 보강" 정책으로 보완 가능하나, 본 change는 단순화를 위해 "어댑터 결과를 채택, generic은 fallback"만 정의하고 보강 정책은 향후 검토.
- *어댑터를 Go interface가 아닌 plugin/.so*: 운영 복잡도 증가. 기각.

**Rationale**: JS 스크립트 경로의 기존 계약(타임아웃·DOM 헬퍼·필드 검증)을 깨지 않으면서, 일반 페이지 처리 경로를 단순화한다.

### Decision 5: 추출 메타는 `pins.og_data` JSONB에 보관 (스키마 변경 최소화)

**Chosen**: 신규 컬럼을 추가하지 않는다. 추출된 부가 필드는 다음 키로 `og_data` JSONB에 저장한다. `body_text`는 **키에 없음** — `pins.description`에 500 rune(UTF-8 rune-safe) 잘라 저장하며 `og_data`에는 저장하지 않는다. canonical URL은 `pins.url` 컬럼에 단일 SSOT로 저장되며 `og_data`에도 중복 저장하지 않는다.

```json
{
  "lang": "ko",
  "author": "...",
  "published_at": "2026-01-01T00:00:00Z",
  "media_candidates": [
    {"type": "image", "url": "...", "width": 800, "height": 600}
  ],
  "source": "<원본 fetch URL>",
  "extractor": "generic" | "script:<site_id>" | "<adapter_name>",
  "classifier": {"pinnable": true} | {"pinnable": false, "reason": "listing" | "empty_body" | "no_primary_media"}
}
```

스키마 (behavior contract에 필요한 최소 노출):

- `og_data.classifier`: `{pinnable: boolean, reason?: "listing" | "empty_body" | "no_primary_media"}` (reason enum은 3개로 축소).
- `og_data.media_candidates[i]`: `{type: "image" | "video" | "audio", url: string, width?: number, height?: number}`.
- `og_data.source`: frontier 역참조용(cross-domain canonical 무시 정책에 따라 항상 fetch URL을 저장; 아래 cross-domain canonical 결정 참조).
- `og_data.extractor`: 추출기 식별자 문자열.
- `og_data`에 `body_text` 및 `canonical_url` 키는 **존재하지 않는다**. body_text는 `pins.description` 컬럼(500 rune), canonical URL은 `pins.url` 컬럼이 각각 단일 SSOT다.

**Alternatives considered**:
- *전용 컬럼 추가(`canonical_url VARCHAR`, `lang VARCHAR`, ...)*: 인덱싱은 좋아지지만 스키마 변경이 커지고 검색 쿼리가 본 change에서 정의되지 않은 시점에서는 과한 결정. 향후 검색 모듈 도입 시 재검토 가능.
- *별도 `pin_documents` 테이블*: SSOT가 분리됨. 기각.

**Rationale**: ERD에 이미 `og_data JSONB`가 존재하므로 마이그레이션은 partial unique index 한 개만 추가된다.

### Decision 6: Cross-domain canonical 무시

**Chosen**: HTML의 `<link rel="canonical">`(또는 `og:url`)이 fetch URL과 **다른 도메인**을 가리키면 그 canonical을 무시하고 `pins.url = fetch_url`로 fallback한다. 이때 `og_data.source = fetch_url`로 저장되어 두 값이 동일해진다. 이 판정은 **extractor 내부 fallback 체인**에서 단 한 곳에서 수행된다(Decision 2 참조). Harvester 결합 단계에서는 추가로 cross-domain 판정을 하지 않으며, extractor가 반환한 `canonical_url` 필드를 신뢰하고 그대로 `pins.url`에 upsert한다.

| 조건 | `pins.url` | `og_data.source` |
|------|------------|------------------|
| canonical 없음 | `fetch_url` | `fetch_url` |
| canonical이 fetch와 동일 호스트 | canonical | `fetch_url` |
| canonical이 fetch와 **다른 호스트** | `fetch_url` (canonical 무시) | `fetch_url` |

**Rationale**: canonical 위조로 다른 페이지의 봇 Pin을 덮어쓰는 것을 방지. Cross-domain canonical은 합법적인 경우(syndication)도 있으나 그 경우에도 원본 호스트가 별도 Pin을 가지는 것이 정본 인덱스 정책과 일관된다. 판정 위치를 extractor 하나로 집중시켜 중복 검사/결과 불일치 위험을 없앤다.

### Decision 7: Bot creator ID 처리

**Chosen**: 봇 Pin의 partial unique index는 `WHERE creator_id = <고정 UUID>`가 아닌 `WHERE creator_id IN (SELECT id FROM creators WHERE is_bot = true)` 형태가 이상적이지만, PostgreSQL partial index는 IMMUTABLE 표현식만 허용하므로 부분 index 자체는 단일 BotCreatorID 상수에 대해 정의한다. `creators` 테이블에는 현재 `is_bot` 컬럼이 없음을 확인했으며, 본 change에서는 해당 컬럼을 추가하지 않고 단일 BotCreatorID 상수 방식으로 확정한다. 다중 봇이 필요해질 때 `is_bot` 컬럼 도입 + partial index 재정의는 별도 change.

**Rationale**: 운영상 봇은 하나(혹은 소수)이며, 다중 봇이 필요해질 때 다시 partial index 정의를 갱신하는 것이 PostgreSQL의 제약을 우회하는 가장 단순한 방법이다.

**운영 제약 (env/config vs IMMUTABLE)**: BotCreatorID는 env/config로 런타임 노출되지만(tasks 9.3), partial index의 WHERE 절은 UUID 리터럴로 하드코딩된다. 즉 env 값과 마이그레이션 리터럴은 반드시 동일해야 하며, 변경 시에는 새 migration(기존 index DROP + 새 UUID로 재생성)을 동반해야 한다. 운영 문서에 이 제약을 명시한다. `UpsertBotPinByURL` 쿼리 역시 `ON CONFLICT (url) WHERE creator_id = '<UUID 리터럴>'` 형태로 partial index predicate와 정확히 일치해야 PostgreSQL이 arbiter inference에 성공한다(파라미터 바인딩 `$n`은 planner가 partial index와 매칭하지 못해 실패).

## Risks / Trade-offs

- **[generic extractor 오추출] → Mitigation**: classifier 사유를 메트릭으로 노출하고, 오추출이 빈번한 도메인은 PerSiteAdapter로 override한다. 첫 도입 시 의심 스러운 페이지는 `pinnable=false, reason=...` 로 분류되어 Pin이 생성되지 않으므로 검색 인덱스 오염 위험은 제한된다.
- **[canonical 위조 페이지가 다른 페이지의 Pin을 덮어씀] → Mitigation**: Decision 6 cross-domain canonical 무시 정책. canonical URL이 fetch URL과 다른 도메인을 가리키는 경우 fetch URL을 사용하고 `og_data.source = fetch_url`로 저장한다.
- **[partial unique index와 동시 upsert race] → Mitigation**: PostgreSQL `ON CONFLICT (url) WHERE creator_id = '<BotID UUID 리터럴>' DO UPDATE`로 처리. 인덱스가 race를 직렬화한다. predicate는 UUID 리터럴이어야 arbiter inference에 성공하며 파라미터 바인딩은 불가.
- **[ScriptAdapter가 N개 RawItem을 반환할 때 정본 1개만 채택해서 정보 손실] → Mitigation**: 채택되지 않은 RawItem들은 `og_data.media_candidates`에 type/url/width/height로 보관된다. 향후 N개 분리가 필요해지면 별도 change에서 도입.
- **[og_data JSONB 비대화] → Mitigation**: `media_candidates` 길이 상한(예: 50)을 두고 초과분은 잘라낸다. `body_text`는 og_data에 저장하지 **않는다**. 대신 `pins.description`에 500 rune(UTF-8 rune-safe) 잘라 저장한다(검색 모듈 도입 시 재정의). canonical URL 역시 `og_data`에 중복 저장하지 않고 `pins.url` 컬럼만 사용한다.
- **[dedup 스크립트의 조인 테이블 CASCADE로 인한 frontier 역참조 유실] → Mitigation**: dedup 직전에 `harvester_frontier_pins` 등 pin_id 참조 조인 row를 생존 Pin의 id로 UPDATE 재할당한 뒤 중복 Pin을 삭제한다. 운영 문서 및 tasks 1.2에 반영.
- **[ERD의 `pins.media_url NOT NULL` 제약과 충돌] → Mitigation**: generic 경로에서 `thumbnail_url`이나 첫 `media_candidates`를 `media_url`로 채운다. 둘 다 없으면 classifier의 `no_primary_media` 사유로 Pin을 만들지 않으므로 NOT NULL 위반은 발생하지 않는다.
- **[기존 RawItem 기반 통계(`PinsCreated`, `Deduped`, `Failed`)와 의미 불일치] → Mitigation**: 통계 정의를 "노드 단위"로 재정의한다. 주 카테고리(정확히 하나 증가) — PinsCreated = 신규 upsert, Deduped = 기존 봇 Pin update, Skipped = classifier 부적합, Failed = extractor/upsert 에러. 부가 카운터(주 카테고리와 독립) — AdapterFallback = 어댑터 실패로 generic 사용. AdapterFallback은 PinsCreated/Deduped/Failed와 같은 노드에서 동시에 증가할 수 있다. 통계 카테고리명은 "Skipped"로 통일하며 "Classified" 등 대체 표현은 사용하지 않는다. ScriptAdapter가 N개 RawItem을 반환하더라도 노드 1개당 주 카테고리 증가는 1이다.

## Migration Plan

본 change는 새 코드 경로 추가 + 기존 경로 강등이 핵심이며, 기존 데이터를 파괴하지 않는다.

1. **DB 마이그레이션**:
   - `apps/api/db/migrations/`에 partial unique index 추가:
     `CREATE UNIQUE INDEX pins_url_bot_unique ON pins(url) WHERE creator_id = '<BotCreatorID UUID 리터럴>';`
   - 기존 봇 Pin 중 같은 URL이 중복으로 존재하면 인덱스 생성이 실패하므로, 마이그레이션 직전에 dedup 스크립트를 돌린다. dedup 순서: (a) 그룹별 가장 최근 created_at Pin을 생존자로 지정 → (b) `harvester_frontier_pins` 등 pin_id 참조 조인 row를 생존 Pin id로 UPDATE → (c) 나머지 중복 봇 Pin 삭제. dedup 스크립트는 본 change에서 제공한다.
2. **코드 추가** (`apps/api/internal/bot/`):
   - `pin_document.go`: `PinDocument` struct + 변환 헬퍼.
   - `extractor.go`: generic HTML→PinDocument extractor.
   - `classifier.go`: 3가지 사유(`listing`, `empty_body`, `no_primary_media`) 분류 로직.
   - `adapter.go`: `PerSiteAdapter` 인터페이스 + `AdapterRegistry`.
   - `script_adapter.go`: 기존 `GojaExecutor`를 PerSiteAdapter로 래핑.
3. **Harvester 결합**:
   - `harvester.go`의 `executeNode`를 generic extractor + adapter registry 경로로 갱신. 결과는 RawItem이 아니라 PinDocument 1건.
   - `harvest_pipeline.go`는 PinDocument를 받아 canonical-URL upsert 수행. ScriptAdapter 경로의 기존 RawItem→Pin 다건 흐름은 ScriptAdapter 내부에서 PinDocument 1건으로 축약된다.
4. **Spec 업데이트**:
   - 새 `harvester` capability spec 생성.
   - 기존 `bot` spec의 ScriptExecutor 관련 requirement는 "per-site override 경로"임을 명시하는 MODIFIED로 갱신.
5. **Rollback**:
   - ScriptAdapter 등록은 per-site opt-in이라 문제 발생 시 해당 site row(`bot_scripts`)만 비워두면 generic 경로로 즉시 되돌아간다 — 별도 feature flag는 불필요.
   - DB 인덱스는 `DROP INDEX CONCURRENTLY pins_url_bot_unique`로 즉시 되돌릴 수 있다.

## Open Questions

(없음 — 이전 Open Questions는 DECISIONS.md §9 및 본 문서 Decision 3·5·6으로 모두 종결되었다.)

Resolved:
- BotCreatorID 외부화: 환경변수/설정으로 런타임 노출(tasks 9.3), 단 마이그레이션 SQL/upsert predicate에는 동일 UUID 리터럴 하드코딩(Decision 7).
- `body_text` 임계(기본 200 bytes, Go `len([]byte)` — classifier 입력), `media_candidates` 상한(기본 50), `threshold_link_density` 임계: 설정값으로 노출(tasks 9.2). 사이트별 override는 향후 사이트 설정 테이블 도입 시 재정의(범위 외).
- ScriptAdapter의 정본 선택 규칙: **첫 번째 RawItem**으로 고정(Decision 4).
- classifier reason enum: `listing`, `empty_body`, `no_primary_media` 3개로 확정(Decision 3; `low_text_link_ratio` 제거). classifier는 PinDocument만 입력으로 받으며 node_type 등 외부 상태에 의존하지 않음. 단어 수 0일 때 listing 판정 division-by-zero guard 적용.
- body_text 저장 위치: `pins.description`에 500 rune(UTF-8 rune-safe) 잘라 저장, `og_data`에는 저장하지 않음(Decision 5). classifier에는 잘리지 않은 원본 body_text가 전달된다.
- canonical URL SSOT: `pins.url` 컬럼 한 곳. `og_data.canonical_url` 키는 존재하지 않음(Decision 5).
- Cross-domain canonical 처리: 무시하고 `pins.url = fetch_url`. 판정은 extractor 내부 fallback 체인에서 단 한 곳에서 수행(Decision 2/6).
- 통계 카테고리 상호 배타성: 주 카테고리(PinsCreated/Deduped/Skipped/Failed)는 노드당 정확히 하나 증가, AdapterFallback은 주 카테고리와 독립적인 부가 카운터(동시 증가 가능).
- AdapterRegistry 초기화: 프로세스 시작 시점 1회. 런타임 DB 변경 반영은 범위 외(tasks 6.6).
- `creators.is_bot` 컬럼: 현재 스키마에 존재하지 않음을 확인. 본 change는 단일 BotCreatorID UUID 상수 방식 확정(Decision 7).
