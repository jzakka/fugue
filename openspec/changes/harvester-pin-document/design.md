## Context

Harvester는 본래(`apps/api/fuguebot_pseudo.go`) `fetch → ParseDocument → Index` 라는 단순한 한 줄짜리 의도를 가진 컴포넌트다. 그러나 직전 정본(`openspec/changes/archive/2026-04-15-perfect-harvester/`) 이후 실제 구현(`apps/api/internal/bot/harvester.go`, `harvest_pipeline.go`, `goja_executor.go`)은 다음 두 가지 결정을 따라 갈라졌다.

- `ParseDocument` ≈ DB에 저장된 사이트별 JS 스크립트를 `goja`로 실행해서 RawItem(`title`, `mediaURL`, `mediaType`, `sourceURL`)을 추출한다.
- `Index` ≈ RawItem 단위로 미디어를 다운로드해 S3에 올리고 Pin을 한 건 만든다.

이 결정은 두 가지 부작용을 만든다.

1. **Pin이 무엇인가가 흐려진다.** Pin은 검색·추천·중복 제거가 의존하는 정본 문서지만, 현재 정의는 "스크립트가 추출한 미디어 한 건"이다. 같은 페이지에서 N개의 미디어가 추출되면 N개의 Pin이 만들어지고, 페이지에 대한 정본(canonical title, body, og)은 어디에도 저장되지 않는다.
2. **default 경로가 site-specific이다.** 일반 웹 페이지는 OG/Twitter Card/JSON-LD/`<article>`만으로도 Pin 문서로 변환 가능하지만, 현재는 사이트별 JS가 없으면 Pin이 0개다. 미등록 도메인은 그래프 노드로만 남는다.

본 변경은 Harvester의 primary contract를 "**HTML을 한 페이지에 한 개의 Pin 문서로 변환**"으로 재정의한다. 기존 JS 스크립트 경로는 제거하지 않고, `PerSiteAdapter` 추상의 한 가지 구현(ScriptAdapter)으로 강등한다.

본 change는 다음 인접 change들과 협업한다(범위 외):
- `harvester-scheduler-consumer`: Harvester를 사이트 단위 BFS에서 frontier consumer 루프로 재정의. 본 change의 `ParseDocument`/`Index`를 호출하는 쪽.
- `harvester-snapshot-first-fetch`: HTML 본문을 어디서 가져올지(snapshot vs live).
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
thumbnail_url:
  og:image → twitter:image → schema.org image → 본문 첫 <img>
media_candidates:
  본문 내 <img>/<video>/<audio>/<source> URL 수집 (절대화)
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

**Chosen**: 추출 결과에 다음 4가지 사유 중 하나라도 해당되면 Pin을 만들지 않고 frontier row의 `harvested_at`만 마킹한다(반복 fetch 방지).

| 사유 | 판정 기준 |
|------|-----------|
| `listing` | URL 노드 타입이 `list`이거나, 같은 도메인 내 outgoing link 수가 본문 텍스트 단어 수의 일정 비율을 초과 (`low_text_link_ratio` 와 다름) |
| `empty_body` | `body_text`가 임계 길이(설정값, 기본 200 chars) 미만 |
| `low_text_link_ratio` | 본문 텍스트 길이 / outgoing 링크 수가 임계값 미만 (네비게이션 hub로 추정) |
| `no_primary_media` | `thumbnail_url`이 없고 `media_candidates`도 비어 있고 `body_text`도 임계 길이 미만 (콘텐츠 동시 부재) |

분류는 우선순위가 있다: `listing` > `empty_body` > `no_primary_media` > `low_text_link_ratio` (가장 명확한 사유부터). 하나라도 매치되면 후속은 평가하지 않는다.

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
1. adapter, ok := registry.Resolve(domain(fetchURL))
2. if ok:
       doc, err = adapter.Extract(...)
       if err != nil: fall back to generic extractor
   else:
       doc = genericExtractor.Extract(...)
3. classify(doc) → if not pinnable: mark harvested_at, return
4. canonicalUpsert(doc) → set frontier.pin_id
```

기존 `GojaExecutor`는 신규 `ScriptAdapter`로 래핑해 레지스트리에 등록한다. ScriptAdapter는 DB의 (site_id, node_type) 스크립트를 로드해 실행하고, 결과 RawItem 1건당 PinDocument 1건이 아니라 **첫 번째 RawItem을 정본 PinDocument로, 나머지를 `og_data.media_candidates`에 추가**하는 방식으로 1 페이지 = 1 Pin 계약을 유지한다.

**Alternatives considered**:
- *어댑터 매칭 시 generic extractor 결과를 무시*: 어댑터가 부분 정보만 줄 때(예: 미디어만 추출) 메타가 누락된다. → "어댑터 결과 우선, 누락 필드는 generic으로 보강" 정책으로 보완 가능하나, 본 change는 단순화를 위해 "어댑터 결과를 채택, generic은 fallback"만 정의하고 보강 정책은 향후 검토.
- *어댑터를 Go interface가 아닌 plugin/.so*: 운영 복잡도 증가. 기각.

**Rationale**: JS 스크립트 경로의 기존 계약(타임아웃·DOM 헬퍼·필드 검증)을 깨지 않으면서, 일반 페이지 처리 경로를 단순화한다.

### Decision 5: 추출 메타는 `pins.og_data` JSONB에 보관 (스키마 변경 최소화)

**Chosen**: 신규 컬럼을 추가하지 않는다. 추출된 부가 필드는 다음 키로 `og_data` JSONB에 저장한다.

```json
{
  "canonical_url": "...",
  "lang": "ko",
  "author": "...",
  "published_at": "2026-01-01T00:00:00Z",
  "media_candidates": [
    {"type": "image", "url": "...", "width": 800, "height": 600},
    ...
  ],
  "source": "<원본 fetch URL>",
  "extractor": "generic" | "script:<site_id>" | "<adapter_name>",
  "classifier": {"pinnable": true} | {"pinnable": false, "reason": "listing"}
}
```

`og_data.source`는 frontier 역참조용이다(canonical URL과 fetch URL이 다를 때 원본 URL을 보존).

**Alternatives considered**:
- *전용 컬럼 추가(`canonical_url VARCHAR`, `lang VARCHAR`, ...)*: 인덱싱은 좋아지지만 스키마 변경이 커지고 검색 쿼리가 본 change에서 정의되지 않은 시점에서는 과한 결정. 향후 검색 모듈 도입 시 재검토 가능.
- *별도 `pin_documents` 테이블*: SSOT가 분리됨. 기각.

**Rationale**: ERD에 이미 `og_data JSONB`가 존재하므로 마이그레이션은 partial unique index 한 개만 추가된다.

### Decision 6: Bot creator ID 처리

**Chosen**: 봇 Pin의 partial unique index는 `WHERE creator_id = <고정 UUID>`가 아닌 `WHERE creator_id IN (SELECT id FROM creators WHERE is_bot = true)` 형태가 이상적이지만, PostgreSQL partial index는 IMMUTABLE 표현식만 허용하므로 부분 index 자체는 단일 BotCreatorID 상수에 대해 정의한다. 봇 계정이 여러 개 있을 가능성을 위해 `creators.is_bot` 플래그(이미 있다면 재활용, 없다면 본 change에서 추가하지 않고 BotCreatorID 상수로 처리)를 운용 정책으로 둔다. 본 change는 "단일 BotCreatorID 상수"를 가정한다.

**Rationale**: 운영상 봇은 하나(혹은 소수)이며, 다중 봇이 필요해질 때 다시 partial index 정의를 갱신하는 것이 PostgreSQL의 제약을 우회하는 가장 단순한 방법이다.

## Risks / Trade-offs

- **[generic extractor 오추출] → Mitigation**: classifier 사유를 메트릭으로 노출하고, 오추출이 빈번한 도메인은 PerSiteAdapter로 override한다. 첫 도입 시 의심 스러운 페이지는 `pinnable=false, reason=...` 로 분류되어 Pin이 생성되지 않으므로 검색 인덱스 오염 위험은 제한된다.
- **[canonical 위조 페이지가 다른 페이지의 Pin을 덮어씀] → Mitigation**: canonical URL이 fetch URL과 다른 도메인을 가리키는 경우 fetch URL을 사용한다(cross-domain canonical 무시). 본 정책을 generic extractor에 명시한다.
- **[partial unique index와 동시 upsert race] → Mitigation**: PostgreSQL `ON CONFLICT (url) WHERE creator_id = <BotID> DO UPDATE`로 처리. 인덱스가 race를 직렬화한다.
- **[ScriptAdapter가 N개 RawItem을 반환할 때 정본 1개만 채택해서 정보 손실] → Mitigation**: 채택되지 않은 RawItem들은 `og_data.media_candidates`에 type/url/width/height로 보관된다. 향후 N개 분리가 필요해지면 별도 change에서 도입.
- **[og_data JSONB 비대화] → Mitigation**: `media_candidates` 길이 상한(예: 50)을 두고 초과분은 잘라낸다. 본문 텍스트(`body_text`)는 og_data에 넣지 않고 `pins.description` 또는 별도 컬럼/검색 인덱스에 넣는 것이 이상적이나, 본 change는 description (500자 제한)에 잘라 넣고 전체 본문은 og_data에 두지 않는 보수적 정책을 채택한다(검색 모듈 도입 시 재정의).
- **[ERD의 `pins.media_url NOT NULL` 제약과 충돌] → Mitigation**: generic 경로에서 `thumbnail_url`이나 첫 `media_candidates`를 `media_url`로 채운다. 둘 다 없으면 classifier의 `no_primary_media` 사유로 Pin을 만들지 않으므로 NOT NULL 위반은 발생하지 않는다.
- **[기존 RawItem 기반 통계(`PinsCreated`, `Deduped`, `Failed`)와 의미 불일치] → Mitigation**: 통계 정의를 "노드 단위"로 재정의한다. PinsCreated = 신규 upsert, Deduped = 기존 봇 Pin update, Failed = extractor/classifier/upsert 에러. ScriptAdapter가 N개 RawItem을 반환하더라도 노드 1개당 통계 1개로 집계된다.

## Migration Plan

본 change는 새 코드 경로 추가 + 기존 경로 강등이 핵심이며, 기존 데이터를 파괴하지 않는다.

1. **DB 마이그레이션**:
   - `apps/api/db/migrations/`에 partial unique index 추가:
     `CREATE UNIQUE INDEX pins_url_bot_unique ON pins(url) WHERE creator_id = '<BotCreatorID>';`
   - 기존 봇 Pin 중 같은 URL이 중복으로 존재하면 인덱스 생성이 실패하므로, 마이그레이션 직전에 dedup 스크립트를 돌린다(가장 최근 created_at만 남기고 나머지 삭제). dedup 스크립트는 본 change에서 제공한다.
2. **코드 추가** (`apps/api/internal/bot/`):
   - `pin_document.go`: `PinDocument` struct + 변환 헬퍼.
   - `extractor.go`: generic HTML→PinDocument extractor.
   - `classifier.go`: 4가지 사유 분류 로직.
   - `adapter.go`: `PerSiteAdapter` 인터페이스 + `AdapterRegistry`.
   - `script_adapter.go`: 기존 `GojaExecutor`를 PerSiteAdapter로 래핑.
3. **Harvester 결합**:
   - `harvester.go`의 `executeNode`를 generic extractor + adapter registry 경로로 갱신. 결과는 RawItem이 아니라 PinDocument 1건.
   - `harvest_pipeline.go`는 PinDocument를 받아 canonical-URL upsert 수행. ScriptAdapter 경로의 기존 RawItem→Pin 다건 흐름은 ScriptAdapter 내부에서 PinDocument 1건으로 축약된다.
4. **Spec 업데이트**:
   - 새 `harvester` capability spec 생성.
   - 기존 `bot` spec의 ScriptExecutor 관련 requirement는 "per-site override 경로"임을 명시하는 MODIFIED로 갱신.
5. **Rollback**:
   - 코드 변경은 feature flag(`HARVESTER_DEFAULT_EXTRACTOR=generic|script`) 뒤에 배포 가능. 기본값은 `generic`.
   - DB 인덱스는 `DROP INDEX CONCURRENTLY pins_url_bot_unique`로 즉시 되돌릴 수 있다.

## Open Questions

- BotCreatorID는 환경 변수/설정으로 외부화해야 하는가, 아니면 코드 상수로 둘 것인가? (운영 정책에 따라 결정; 본 change는 환경변수/설정으로 노출하고 마이그레이션은 placeholder로 두는 것을 제안.)
- `body_text` 임계 길이(기본 200), `low_text_link_ratio` 임계값, `media_candidates` 상한(기본 50) 등의 튜닝 값은 어디서 관리할 것인가? (사이트별 override 가능성 → 향후 사이트 설정 테이블 도입 시 재정의.)
- ScriptAdapter가 multi-RawItem 결과를 정본 1 PinDocument로 축약할 때, 어떤 RawItem을 "정본"으로 선택할지의 우선순위(첫 번째 vs largest media vs explicit primary 플래그). 본 change는 "첫 번째"로 고정, 추후 어댑터별 옵션 도입 가능.
