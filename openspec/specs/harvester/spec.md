# harvester Specification

## Purpose
TBD - created by archiving change harvester-pin-document. Update Purpose after archive.
## Requirements
### Requirement: Pin은 원본 페이지에 대한 단일 정본 문서다
Bot이 만드는 Pin은 검색 SSOT 역할을 하며, 한 원본 페이지(canonical URL 기준)에 대해 봇 계정 소유 Pin이 정확히 한 개만 존재해야 한다(SHALL). 같은 canonical URL을 다시 harvest하면 새 Pin을 만들지 않고 기존 봇 Pin을 update해야 한다(SHALL). 일반 사용자 Pin은 본 제약의 영향을 받지 않으며, 일반 사용자는 동일 URL로 복수 Pin을 보유할 수 있다(SHALL).

#### Scenario: 새 canonical URL에 대한 첫 harvest
- **WHEN** Harvester가 어떤 canonical URL에 해당하는 봇 Pin이 DB에 없는 상태에서 그 페이지를 harvest할 때
- **THEN** 시스템은 해당 canonical URL로 봇 계정 소유 Pin을 한 개 새로 생성한다

#### Scenario: 같은 canonical URL 재harvest
- **WHEN** Harvester가 이미 봇 Pin이 존재하는 canonical URL을 다시 harvest할 때
- **THEN** 시스템은 새 Pin을 만들지 않고 기존 봇 Pin의 메타(title, description, og_image, og_data 등)를 update한다

#### Scenario: 동일 URL의 일반 사용자 Pin은 영향 없음
- **WHEN** 어떤 일반 사용자가 이미 동일 URL로 Pin을 보유한 상태에서 봇이 그 URL을 harvest할 때
- **THEN** 봇 Pin이 추가로 생성되며 기존 사용자 Pin은 변경되지 않는다

#### Scenario: 일반 사용자의 동일 URL 복수 Pin은 허용
- **WHEN** 두 명의 서로 다른 일반 사용자가 같은 URL로 Pin을 생성할 때
- **THEN** 두 Pin이 모두 생성된다 (URL 중복 제약은 봇 Pin에만 적용)

#### Scenario: 봇 Pin 멱등성을 강제하는 partial unique index
- **WHEN** `pins` 테이블 스키마를 확인할 때
- **THEN** `WHERE creator_id = <BotCreatorID>` 조건의 partial unique index가 `pins(url)`에 존재한다

---

### Requirement: Generic HTML→Pin extractor가 default 변환 경로다
시스템은 어떤 HTML 페이지에 대해서도 site-specific 스크립트 없이 Pin 문서를 생성할 수 있어야 한다(SHALL). Default 변환은 OpenGraph → Twitter Card → JSON-LD(`schema.org`) → semantic HTML(`<article>`, `<title>`) 순의 fallback 체인으로 동작해야 한다(SHALL).

#### Scenario: title 추출 fallback 체인
- **WHEN** generic extractor가 HTML에서 title을 추출할 때
- **THEN** 시스템은 `og:title`, `twitter:title`, `<article>` 내 `<h1>`, `<title>` 태그 순으로 검사하여 처음 발견된 비어있지 않은 값을 사용한다

#### Scenario: body_text 추출 fallback 체인
- **WHEN** generic extractor가 HTML에서 본문 텍스트를 추출할 때
- **THEN** 시스템은 `<article>` textContent, JSON-LD `schema.org/Article.articleBody`, 본문 내 최대 텍스트 밀도 블록, `og:description`, `meta[name=description]` 순으로 검사하여 처음 발견된 비어있지 않은 값을 사용한다

#### Scenario: canonical_url 결정
- **WHEN** generic extractor가 canonical URL을 결정할 때
- **THEN** 시스템은 `<link rel="canonical">`, `og:url`, fetch에 사용한 URL 순으로 검사하여 처음 발견된 비어있지 않은 값을 사용한다. 단, 채택 후보의 호스트가 fetch URL의 호스트와 다르면 해당 후보는 건너뛰고 다음 fallback으로 넘어간다 (cross-domain canonical 무시)

#### Scenario: thumbnail_url 추출
- **WHEN** generic extractor가 썸네일 URL을 결정할 때
- **THEN** 시스템은 `og:image`, `twitter:image`, JSON-LD `schema.org` image 필드, 본문 내 첫 `<img>` 순으로 검사하여 처음 발견된 비어있지 않은 절대 URL을 사용한다

#### Scenario: media_candidates 수집
- **WHEN** generic extractor가 본문에서 미디어 후보를 수집할 때
- **THEN** 시스템은 본문 범위 내 `<img>`, `<video>`, `<audio>`, `<source>` 태그의 URL을 절대 경로로 변환하여 type(image/video/audio)과 함께 `media_candidates` 배열에 수집한다. 본문 범위는 `<article>` 태그가 있으면 그 내부, 없으면 `<body>` 전체로 정의된다

#### Scenario: source의 srcset 다중 후보에서 첫 URL만 수집
- **WHEN** `<source>` 태그가 `src` 속성 없이 `srcset` 속성만 가지며, 그 값이 `"a.webp 1x, b.webp 2x"`처럼 디스크립터를 포함한 콤마 구분 후보 목록일 때
- **THEN** 시스템은 첫 후보의 URL 토큰(콤마 분리 후 첫 후보의 공백 앞부분)만 추출하여 절대 경로로 변환해 `media_candidates`에 수집하고, 디스크립터(`1x`/`640w` 등)와 나머지 후보는 버린다. 추출된 URL 토큰이 비어 있으면 해당 `<source>`는 후보로 수집하지 않는다

#### Scenario: lang/author/published_at 추출
- **WHEN** generic extractor가 부가 메타를 추출할 때
- **THEN** 시스템은 `<html lang>` 또는 `og:locale`로부터 lang을, `schema.org` Author/`meta[name=author]`/`og:article:author`로부터 author를, `schema.org` datePublished/`article:published_time`/`<time datetime>`로부터 published_at을 추출한다 (없으면 빈 값/null)

#### Scenario: cross-domain canonical 무시
- **WHEN** HTML의 `<link rel="canonical">`이 fetch URL과 다른 도메인을 가리킬 때
- **THEN** 시스템은 해당 canonical을 무시하고 fetch URL을 canonical_url로 사용한다

#### Scenario: 사이트별 스크립트가 없어도 Pin이 생성된다
- **WHEN** 어떤 사이트에 대해 PerSiteAdapter가 등록되어 있지 않은 상태에서 일반 HTML 페이지를 harvest할 때
- **THEN** generic extractor가 동작하여 PinDocument가 생성되고, classifier가 통과시키면 Pin이 DB에 upsert된다

---

### Requirement: PinDocument는 검색 SSOT 최소 필드를 가진다
Generic extractor 또는 PerSiteAdapter가 반환하는 `PinDocument`는 다음 최소 필드를 가져야 한다(SHALL): `title`, `body_text`, `canonical_url`, `thumbnail_url`, `media_candidates`, `lang`, `author`, `published_at`, `og_data`. 추출 실패 또는 부재 필드는 빈 값/빈 배열/null로 표현해야 하며, 객체 자체는 항상 반환되어야 한다(SHALL).

#### Scenario: 모든 필드가 추출된 경우
- **WHEN** generic extractor가 OG, JSON-LD, `<article>`을 모두 가진 페이지를 처리할 때
- **THEN** PinDocument의 모든 최소 필드가 비어있지 않은 값으로 채워진다

#### Scenario: 일부 필드만 추출된 경우
- **WHEN** generic extractor가 `og:title`만 있고 본문이 비어 있는 페이지를 처리할 때
- **THEN** PinDocument의 title은 채워지고 body_text는 빈 문자열, media_candidates는 빈 배열, published_at은 null로 반환된다

#### Scenario: PinDocument는 항상 반환된다
- **WHEN** generic extractor가 어떠한 HTML에 대해서도 호출될 때
- **THEN** 시스템은 nil이 아닌 PinDocument 객체를 반환한다 (필드는 비어 있을 수 있다)

---

### Requirement: Content classifier가 Pin 생성 가능 여부를 판정한다
시스템은 PinDocument 생성 후 Pin으로 indexing할지 여부를 판정해야 한다(SHALL). 부적합한 경우 Pin을 만들지 않고 사유를 다음 3개 enum 중 하나로 분류해야 한다(SHALL): `listing`, `empty_body`, `no_primary_media`. 사유는 우선순위(`listing` > `empty_body` > `no_primary_media`) 순으로 평가되며, 첫 매치에서 평가가 종료되어야 한다(SHALL). `body_text` 길이 단위는 바이트다(SHALL). classifier는 `PinDocument`만을 입력으로 받으며 외부 상태(node_type 등)에 의존하지 않아야 한다(SHALL).

#### Scenario: listing 페이지 분류
- **WHEN** 페이지의 단어 수가 0보다 크고 `링크 수 / 단어 수 > threshold_link_density`일 때
- **THEN** classifier는 `pinnable=false, reason=listing`을 반환한다

#### Scenario: 단어 수 0인 페이지는 listing 아님
- **WHEN** 페이지의 단어 수가 0일 때
- **THEN** classifier는 `listing` 판정을 내리지 않고 다음 우선순위 사유(`empty_body`) 평가로 진행한다 (division-by-zero 회피)

#### Scenario: empty_body 분류
- **WHEN** PinDocument의 body_text 바이트 길이가 임계값(기본 200 bytes) 미만일 때
- **THEN** classifier는 `pinnable=false, reason=empty_body`를 반환한다 (단, listing이 먼저 매치되면 listing이 우선)

#### Scenario: no_primary_media 분류
- **WHEN** PinDocument의 thumbnail_url이 비어 있고 media_candidates가 빈 배열일 때 (listing/empty_body가 먼저 매치되지 않은 경우)
- **THEN** classifier는 `pinnable=false, reason=no_primary_media`를 반환한다

#### Scenario: 정상 콘텐츠 페이지 통과
- **WHEN** PinDocument가 충분한 body_text를 가지고 thumbnail_url 또는 media_candidates가 존재하며 listing 패턴에 해당하지 않을 때
- **THEN** classifier는 `pinnable=true`를 반환한다

#### Scenario: 사유는 og_data에 보존된다
- **WHEN** classifier가 어떤 사유로든 판정을 내릴 때
- **THEN** 판정 결과가 PinDocument의 `og_data.classifier = {pinnable: boolean, reason?: "listing" | "empty_body" | "no_primary_media"}` 스키마로 저장되어 후속 디버깅/메트릭에 사용된다

#### Scenario: reason enum은 3개로 제한
- **WHEN** classifier가 `pinnable=false`를 반환하며 reason을 설정할 때
- **THEN** reason은 정확히 `listing`, `empty_body`, `no_primary_media` 중 하나여야 하며 다른 값(예: `low_text_link_ratio`)은 사용되지 않는다

---

### Requirement: Pinnable이 아닌 페이지는 Pin을 만들지 않고 harvested_at만 마킹한다
시스템은 classifier가 `pinnable=false`로 판정한 페이지에 대해 Pin을 생성하거나 update해서는 안 된다(SHALL NOT). 대신 frontier row의 `harvested_at`만 마킹해 동일 URL의 즉시 재harvest를 방지해야 한다(SHALL).

#### Scenario: listing 페이지 처리
- **WHEN** classifier가 어떤 페이지를 `listing`으로 분류할 때
- **THEN** Pin이 생성되지 않고 frontier row의 harvested_at만 현재 시각으로 갱신된다

#### Scenario: 빈 페이지 처리
- **WHEN** classifier가 어떤 페이지를 `empty_body`로 분류할 때
- **THEN** Pin이 생성되지 않고 frontier row의 harvested_at만 현재 시각으로 갱신된다

#### Scenario: 통계 분류
- **WHEN** Harvester가 노드 단위 통계를 집계할 때
- **THEN** pinnable=false로 분류된 노드는 PinsCreated/Deduped/Failed 어디에도 카운트되지 않고 별도의 `Skipped` 카운트로 분류된다 (카테고리명은 `Skipped`로 통일하며 `Classified` 등 대체 표현은 사용하지 않는다)

---

### Requirement: PerSiteAdapter / AdapterRegistry로 site-specific 변환을 등록한다
시스템은 도메인별로 generic extractor를 대체할 수 있는 어댑터를 등록할 수 있어야 한다(SHALL). 어댑터가 도메인에 매치되면 generic extractor 대신 어댑터의 결과를 채택해야 한다(SHALL). 어댑터 실행이 에러를 반환하면 generic extractor로 fallback해야 한다(SHALL).

#### Scenario: 어댑터 등록과 조회
- **WHEN** 어떤 도메인에 대한 PerSiteAdapter가 AdapterRegistry에 등록되어 있을 때
- **THEN** 같은 도메인 URL에 대해 `Resolve(domain)`이 해당 어댑터와 ok=true를 반환한다

#### Scenario: 어댑터 우선 적용
- **WHEN** Harvester가 어떤 URL을 처리할 때 그 도메인의 PerSiteAdapter가 등록되어 있을 때
- **THEN** Harvester는 generic extractor가 아닌 어댑터의 `Extract`를 호출하고 그 결과를 PinDocument로 사용한다

#### Scenario: 어댑터 실패 시 generic으로 fallback
- **WHEN** PerSiteAdapter의 `Extract`가 에러를 반환할 때
- **THEN** Harvester는 generic extractor로 fallback하여 PinDocument를 생성한다

#### Scenario: 어댑터 미등록 도메인은 generic 사용
- **WHEN** 어떤 URL의 도메인에 대해 PerSiteAdapter가 등록되어 있지 않을 때
- **THEN** Harvester는 generic extractor를 사용한다

#### Scenario: 어댑터 식별자가 og_data에 보존된다
- **WHEN** 어떤 PerSiteAdapter 또는 generic extractor가 PinDocument를 생성할 때
- **THEN** `og_data.extractor`에 사용된 추출기 식별자(`generic`, `script:<site_id>`, 또는 어댑터 이름)가 기록된다

---

### Requirement: 기존 JS 스크립트 경로는 ScriptAdapter로 PerSiteAdapter에 등록된다
시스템은 기존 `GojaExecutor` 기반 JS 스크립트 실행 경로를 `ScriptAdapter`로 래핑하여 AdapterRegistry에 등록해야 한다(SHALL). ScriptAdapter는 DB의 (site_id, node_type) 스크립트를 로드해 실행하고, 결과를 PinDocument 1건으로 축약해야 한다(SHALL).

#### Scenario: ScriptAdapter 등록
- **WHEN** Harvester 부트스트랩 시 DB에 (site_id, node_type) 스크립트가 등록된 사이트가 있을 때
- **THEN** 그 사이트의 도메인에 대한 ScriptAdapter가 AdapterRegistry에 등록된다

#### Scenario: ScriptAdapter는 RawItem N건을 PinDocument 1건으로 축약 (첫 RawItem 정본)
- **WHEN** ScriptAdapter가 스크립트를 실행하여 N개의 RawItem을 받을 때
- **THEN** **첫 번째 RawItem**이 PinDocument의 정본 메타(title, thumbnail_url, body_text, description 등 모든 메타 필드)로 채택되고, 나머지 RawItem들은 `{type, url, width?, height?}` 스키마로 `og_data.media_candidates` 배열에 추가된다

#### Scenario: ScriptAdapter 결과의 extractor 식별자
- **WHEN** ScriptAdapter가 PinDocument를 반환할 때
- **THEN** `og_data.extractor`에 `script:<site_id>` 형태의 식별자가 기록된다

#### Scenario: ScriptAdapter 실행 실패 시 generic fallback
- **WHEN** ScriptAdapter 내부의 스크립트 실행이 타임아웃, 구문 에러, 런타임 에러 중 하나로 실패할 때
- **THEN** Harvester는 generic extractor로 fallback하여 PinDocument를 생성한다

---

### Requirement: Canonical-URL 기반 멱등 upsert
시스템은 PinDocument를 DB에 indexing할 때 canonical_url을 키로 봇 Pin을 멱등하게 upsert해야 한다(SHALL). upsert는 partial unique index `pins(url) WHERE creator_id = <BotCreatorID>`를 사용한 PostgreSQL `ON CONFLICT DO UPDATE`로 구현되어야 한다(SHALL).

#### Scenario: 신규 Pin 생성 (insert)
- **WHEN** PinDocument의 canonical_url에 해당하는 봇 Pin이 없을 때
- **THEN** 시스템은 새 Pin을 insert한다

#### Scenario: 기존 봇 Pin update
- **WHEN** PinDocument의 canonical_url에 해당하는 봇 Pin이 이미 있을 때
- **THEN** 시스템은 INSERT ... ON CONFLICT DO UPDATE를 통해 기존 Pin의 title, description, og_image, og_data를 새 값으로 갱신한다

#### Scenario: 동시 upsert race
- **WHEN** 두 워커가 같은 canonical_url에 대해 동시에 upsert를 시도할 때
- **THEN** PostgreSQL의 partial unique index가 race를 직렬화하여 정확히 한 개의 Pin만 존재한다 (한 쪽은 insert, 다른 쪽은 update로 귀결)

#### Scenario: 일반 사용자 Pin과의 충돌 없음
- **WHEN** 같은 canonical_url로 일반 사용자 Pin이 이미 존재할 때 봇이 그 URL을 upsert할 때
- **THEN** partial unique index는 `creator_id = <BotCreatorID>`만 적용되므로 충돌 없이 봇 Pin이 별도로 insert된다

---

### Requirement: 원본 fetch URL은 og_data.source에 보존된다
시스템은 PinDocument를 Pin으로 변환할 때, fetch에 사용한 원본 URL을 `og_data.source`에 보존해야 한다(SHALL). `og_data.source`는 항상 fetch URL이며, cross-domain canonical이 감지된 경우 `canonical_url` 역시 fetch URL로 fallback된다(SHALL). frontier row 역참조에 사용된다(SHALL).

#### Scenario: canonical_url과 fetch URL이 같을 때
- **WHEN** PinDocument의 canonical_url과 fetch URL이 동일할 때
- **THEN** `og_data.source`에 fetch URL이 저장된다 (canonical_url과 동일한 값)

#### Scenario: same-domain canonical
- **WHEN** HTML의 canonical이 fetch URL과 동일 도메인의 다른 URL(예: query string 정규화)일 때
- **THEN** `pins.url`에는 canonical이, `og_data.source`에는 fetch URL이 저장된다

#### Scenario: cross-domain canonical은 fetch_url 기준으로 fallback
- **WHEN** HTML의 canonical이 fetch URL과 **다른 도메인**을 가리킬 때
- **THEN** 시스템은 canonical을 무시하고 `pins.url = fetch_url`로 설정하며, `og_data.source = fetch_url`로 저장한다 (두 값이 동일해진다)

#### Scenario: frontier 역참조
- **WHEN** 운영자가 어떤 Pin이 어떤 frontier URL에서 유래했는지 추적할 때
- **THEN** `og_data.source` 필드를 통해 원본 frontier URL을 알 수 있다

---

### Requirement: 추출 부가 메타는 pins.og_data JSONB에 보관한다
시스템은 PinDocument의 부가 필드(lang, author, published_at, media_candidates, source, extractor, classifier)를 `pins` 테이블의 신규 컬럼이 아닌 기존 `og_data` JSONB 컬럼에 보관해야 한다(SHALL). canonical URL은 `pins.url` 컬럼에 단일 SSOT로 저장되며 `og_data`에는 중복 저장하지 않는다(SHALL NOT). `body_text`는 `og_data`에 저장하지 **않으며**(SHALL NOT), `pins.description`에 500자(rune 기준, UTF-8 문자 500개) 이내로 잘라 저장해야 한다(SHALL). classifier는 잘리지 않은 원본 `body_text`를 입력으로 받으며, `pins.description`에 저장되는 값은 잘린 형태다(SHALL). `media_candidates`의 길이는 상한(기본 50)을 넘지 않도록 잘려야 한다(SHALL).

#### Scenario: og_data 키 구조
- **WHEN** Pin이 upsert될 때
- **THEN** `og_data`에는 다음 키가 포함된다: `lang`, `author`, `published_at`, `media_candidates`, `source`, `extractor`, `classifier`. `body_text`와 `canonical_url` 키는 존재하지 않는다 (body_text는 `pins.description`, canonical URL은 `pins.url` 컬럼이 SSOT)

#### Scenario: media_candidates 원소 스키마
- **WHEN** `og_data.media_candidates` 배열의 원소를 확인할 때
- **THEN** 각 원소는 `{type: "image" | "video" | "audio", url: string, width?: number, height?: number}` 스키마를 따른다

#### Scenario: classifier 스키마
- **WHEN** `og_data.classifier`를 확인할 때
- **THEN** 값은 `{pinnable: boolean, reason?: "listing" | "empty_body" | "no_primary_media"}` 스키마를 따르며 reason enum은 3개로 제한된다

#### Scenario: body_text는 og_data가 아니라 description에 저장된다
- **WHEN** extractor 또는 adapter가 body_text를 추출할 때
- **THEN** 시스템은 `og_data`에 body_text를 포함하지 않고, `pins.description`에 500 rune 이내로 잘라 저장한다 (UTF-8 rune-safe, multi-byte 문자를 바이트 경계에서 절단하지 않는다)

#### Scenario: classifier는 원본 body_text를 받고 description은 잘린 형태
- **WHEN** classifier가 PinDocument를 평가할 때
- **THEN** classifier는 잘리지 않은 원본 body_text(바이트 길이 기준)로 `empty_body`/`no_primary_media` 판정을 수행하며, `pins.description`에 저장되는 값은 이와 무관하게 500 rune으로 잘린 형태다

#### Scenario: same-domain canonical은 og_data에 중복 저장되지 않는다
- **WHEN** Pin이 upsert될 때
- **THEN** 채택된 canonical URL은 `pins.url` 컬럼 한 곳에만 저장되며 `og_data.canonical_url` 키는 존재하지 않는다

#### Scenario: media_candidates 상한 적용
- **WHEN** 추출된 media_candidates가 상한(기본 50)을 초과할 때
- **THEN** 앞에서부터 상한 개수까지만 보관되고 나머지는 잘린다

#### Scenario: 신규 컬럼 미추가
- **WHEN** 본 변경의 마이그레이션을 적용한 후 `pins` 테이블 스키마를 확인할 때
- **THEN** 부가 필드를 위한 새 컬럼이 추가되지 않았으며, partial unique index만 추가되어 있다

---

### Requirement: Harvester 노드 단위 통계 정의
시스템은 Harvester가 처리한 한 노드(URL)에 대해 다음 4개 주 카테고리 중 정확히 하나로 집계해야 한다(SHALL): `PinsCreated`(신규 봇 Pin insert), `Deduped`(기존 봇 Pin update), `Skipped`(classifier가 pinnable=false 판정), `Failed`(extractor/upsert 에러). `AdapterFallback`(어댑터 실패로 generic으로 fallback)은 주 카테고리와 독립적인 부가 카운터이며 주 카테고리와 동시에 증가할 수 있다(SHALL). ScriptAdapter가 RawItem을 N개 반환하더라도 노드 1개당 주 카테고리 증가는 1이어야 한다(SHALL).

#### Scenario: 신규 페이지 harvest
- **WHEN** Harvester가 새 canonical URL의 페이지를 처리하고 Pin을 새로 insert할 때
- **THEN** PinsCreated가 1 증가한다

#### Scenario: 기존 페이지 재harvest
- **WHEN** Harvester가 이미 봇 Pin이 있는 canonical URL을 다시 처리하고 Pin을 update할 때
- **THEN** Deduped가 1 증가한다

#### Scenario: pinnable=false 페이지 처리
- **WHEN** Harvester가 어떤 페이지를 처리하고 classifier가 pinnable=false로 판정할 때
- **THEN** Skipped가 1 증가한다 (PinsCreated/Deduped/Failed는 증가하지 않는다)

#### Scenario: 추출/upsert 에러
- **WHEN** Harvester가 어떤 페이지에 대해 extractor 또는 upsert에서 에러를 만날 때
- **THEN** Failed가 1 증가한다

#### Scenario: 어댑터 실패 후 generic 성공
- **WHEN** ScriptAdapter가 실패해 generic extractor로 fallback되어 Pin이 생성될 때
- **THEN** PinsCreated 또는 Deduped가 1 증가하고 별도로 AdapterFallback이 1 증가한다

#### Scenario: ScriptAdapter의 N개 RawItem
- **WHEN** ScriptAdapter가 한 노드에 대해 N개의 RawItem을 반환하여 PinDocument 1건으로 축약될 때
- **THEN** 노드 1개당 PinsCreated 또는 Deduped가 정확히 1만 증가한다 (N이 아니다)

### Requirement: Harvester는 Dequeue(QueueHarvester)로만 URL을 수급한다
Harvester 워커는 자체적인 큐/그래프 순회 구조를 보유하지 않아야 하며(SHALL NOT), 처리할 URL은 오직 `scheduler.Dequeue(scheduler.QueueHarvester)` 호출 결과로만 획득해야 한다(SHALL). 단일 `Dequeue` 호출은 `harvester_frontier`의 partial index(`WHERE harvested_at IS NULL AND harvest_error_count < 5`)를 대상으로 한 claim이며, 다른 테이블이나 인메모리 캐시를 참조하지 않는다.

#### Scenario: 메인 루프는 Dequeue(QueueHarvester)로 시작
- **WHEN** Harvester 워커가 한 iteration을 시작할 때
- **THEN** 가장 먼저 `scheduler.Dequeue(scheduler.QueueHarvester)`를 호출하여 처리 대상 URL을 획득하고, 그 외의 어떤 인메모리 큐/리스트에서도 다음 URL을 꺼내지 않는다.

#### Scenario: 다른 큐 타입을 사용하지 않음
- **WHEN** Harvester 구현체를 점검할 때
- **THEN** `Dequeue(QueuePioneer)` 호출이나 `pioneer_frontier` 직접 조회가 존재하지 않는다.

#### Scenario: 자체 큐/visited/nodeMap 자료구조 부재
- **WHEN** 신규 Harvester 구현체의 필드와 함수를 정적으로 점검할 때
- **THEN** `BFSQueue`, `visited map`, 사이트 전체 노드를 사전 적재하는 `nodeMap` 등 "다음 URL 후보"를 보관하는 인메모리 자료구조가 존재하지 않는다.

#### Scenario: 그래프 순회 로직 부재
- **WHEN** Harvester가 한 URL을 처리한 직후
- **THEN** 해당 URL의 outgoing edge를 따라가 다음 URL을 자체 결정하지 않으며, 다음 URL 결정은 다시 `scheduler.Dequeue(scheduler.QueueHarvester)` 호출로 위임된다.

---

### Requirement: 메인 루프는 snapshot-first fetch → PinDocument → Pin 생성 → SetStatus 순서를 따른다
Harvester의 단일 iteration은 다음 단계를 순서대로 수행해야 한다(SHALL):
1. `scheduler.Dequeue(scheduler.QueueHarvester)`로 처리 대상 URL을 claim한다.
2. `harvester-snapshot-first-fetch` capability가 제공하는 snapshot-first 경로로 HTML을 획득한다(snapshot_key가 있으면 snapshot 우선, miss 시 HTTP live fetch).
3. `harvester-pin-document` capability의 `harvestPipeline.Process`로 HTML을 `PinDocument`로 파싱한다.
4. `PinDocument.Pinnable`이 true이면 Pin을 생성하여 `pinIDs []uuid.UUID`를 수집한다. false이면 Pin 생성을 건너뛴다.
5. 성공 시 `scheduler.SetStatus(url, "harvested", pinIDs)`를 호출한다. `pinIDs`가 nil 또는 빈 슬라이스이면 매핑 없이 완료 표기한다.

각 단계의 실패는 다음 단계 실행을 중단해야 하며(SHALL), 실패 처리(본 spec의 별도 requirement)를 따라야 한다.

**워커 종료 조건**(work budget 소진 = 성공 Dequeue 100회 후 종료)은 본 capability의 "Harvester 워커는 100회 Dequeue 후 종료한다" requirement에 정의되어 있다. 본 루프 requirement는 단일 iteration의 **단계 순서**만 규범화한다.

#### Scenario: 정상 흐름 - Pin 1건 생성
- **WHEN** `Dequeue`가 URL `U`를 반환하고, snapshot-first fetch와 PinDocument 파싱이 성공하고 `Pinnable = true`이며 Pin 1건이 생성될 때
- **THEN** Harvester는 `scheduler.SetStatus(U, "harvested", []uuid.UUID{pinID})`를 호출한 뒤 다음 iteration을 시작한다.

#### Scenario: 정상 흐름 - Pin N건 생성
- **WHEN** `PinDocument`가 복수 Pin으로 materialize되어 `pinIDs`가 길이 N(N>=2)인 슬라이스일 때
- **THEN** Harvester는 `scheduler.SetStatus(U, "harvested", pinIDs)`를 **단일 호출**로 전달하고, scheduler 구현이 `harvested_at` UPDATE와 `harvester_frontier_pins` 일괄 INSERT를 한 트랜잭션에서 처리한다.

#### Scenario: Pinnable = false 시 Pin 생성 스킵
- **WHEN** fetch와 파싱은 성공했으나 `PinDocument.Pinnable == false`일 때
- **THEN** Harvester는 Pin을 생성하지 않고 `scheduler.SetStatus(U, "harvested", nil)`을 호출하여 해당 row를 완료 상태로 표기한다. `harvester_frontier_pins`에는 아무 row도 INSERT되지 않는다.

#### Scenario: 빈 pinIDs 슬라이스도 완료 표기로 처리
- **WHEN** `pinIDs`가 nil이거나 길이 0인 슬라이스일 때
- **THEN** `SetStatus(U, "harvested", nil)` 호출과 동일하게 처리되어 매핑 없이 `harvested_at`만 갱신된다.

#### Scenario: 루프는 상기 단계를 반복하되 budget 요건에 따라 종료한다
- **WHEN** Harvester 프로세스가 정상 구동 중일 때
- **THEN** 메인 루프는 상기 단계를 반복하며, 워커 종료 조건(work budget 소진)은 본 capability의 "Harvester 워커는 100회 Dequeue 후 종료한다" requirement에 정의되어 있다(이 루프 스펙 내에서는 단계 순서만 규범화한다).

### Requirement: 성공 상태 전이는 harvested_at UPDATE와 harvester_frontier_pins INSERT를 한 트랜잭션으로 처리한다
Harvester consumer가 호출하는 `scheduler.SetStatus(url, "harvested", pinIDs)`는 다음 작업을 단일 DB 트랜잭션에서 수행해야 한다(SHALL):
- `harvester_frontier` row의 `harvested_at`을 `now()`로 UPDATE.
- `harvester_frontier.harvest_error_count`를 0으로 리셋(scheduler SSOT: `scheduler/spec.md` `"harvested"` status 요구사항).
- `pinIDs`의 각 원소에 대해 `harvester_frontier_pins(frontier_id, pin_id)`에 INSERT.

위 작업이 분리되어 "매핑 없는 harvested row" 또는 "harvested_at이 NULL인데 매핑이 있는 row"가 생겨서는 안 된다(SHALL NOT).

#### Scenario: 성공 트랜잭션 원자성
- **WHEN** `SetStatus(U, "harvested", []uuid.UUID{p1, p2, p3})` 호출 중 어느 한 INSERT가 실패할 때
- **THEN** `harvested_at` UPDATE를 포함한 전체 트랜잭션이 롤백되어, 다음 `Dequeue`에서 동일 row가 다시 반환될 수 있다.

#### Scenario: Pin 0건 성공의 매핑 부재
- **WHEN** `SetStatus(U, "harvested", nil)`이 호출될 때
- **THEN** `harvested_at`은 갱신되고 `harvester_frontier_pins`에는 아무 row도 INSERT되지 않으며, partial index에서 해당 row가 제외된다.

#### Scenario: harvested row는 partial index에서 제외
- **WHEN** `SetStatus(U, "harvested", ...)` 호출이 성공한 직후
- **THEN** 동일 URL은 다음 `Dequeue(QueueHarvester)` 호출로 반환되지 않는다 (partial index의 `WHERE harvested_at IS NULL` 조건).

---

### Requirement: 실패 시 SetStatus + RecordHarvestError를 둘 다 호출한다
Harvester가 fetch/파싱/Pin 생성 중 어느 단계에서 실패하든, 해당 URL에 대해 다음 두 호출을 순서대로 수행해야 한다(SHALL):
1. `scheduler.SetStatus(url, "harvest_failed", nil)` — 상태 전이 표기.
2. `scheduler.RecordHarvestError(url, errorKind)` — 카운터 누적과 backoff 적용.

`errorKind`는 `scheduler-claim-api`가 허용하는 enum 4종 중 하나여야 한다(SHALL): `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"`. 열거 외 값을 전달해서는 안 된다(SHALL NOT). Harvester 내부에서 "파싱 실패", "Pin 생성 실패" 같은 범주 구분이 필요하면 로그/메트릭 라벨 수준에서 유지하되, scheduler 보고 시에는 위 4종 중 하나로 매핑해야 한다(SHALL).

#### Scenario: HTTP 4xx 응답 시 errorKind = http_4xx
- **WHEN** snapshot miss 후 live fetch가 HTTP 4xx를 반환할 때
- **THEN** `SetStatus(U, "harvest_failed", nil)`과 `RecordHarvestError(U, "http_4xx")`가 이 순서로 호출된다.

#### Scenario: HTTP 5xx 응답 시 errorKind = http_5xx
- **WHEN** fetch가 HTTP 5xx를 반환할 때
- **THEN** `RecordHarvestError(U, "http_5xx")`가 호출된다.

#### Scenario: DNS/connect/TLS 실패 시 errorKind = network
- **WHEN** fetch가 DNS 해석/TCP connect/TLS handshake 실패로 종료될 때
- **THEN** `RecordHarvestError(U, "network")`가 호출된다.

#### Scenario: 타임아웃 시 errorKind = timeout
- **WHEN** fetch 또는 스크립트 실행이 타임아웃으로 종료될 때
- **THEN** `RecordHarvestError(U, "timeout")`가 호출된다.

#### Scenario: 파싱 실패는 scheduler enum으로 매핑된다
- **WHEN** `harvestPipeline.Process`가 스크립트 구문/런타임 에러 등으로 실패할 때
- **THEN** `RecordHarvestError(U, "network")`가 호출되어 일시 실패로 backoff 재시도에 편입되고, consumer 측 로그/메트릭에는 `parse` 내부 라벨이 남아 운영 분석이 가능하다.

#### Scenario: Pin 생성 실패는 scheduler enum으로 매핑된다
- **WHEN** DB 에러 등으로 Pin INSERT가 실패할 때
- **THEN** `RecordHarvestError(U, "network")`가 호출되고, consumer 측 로그/메트릭에는 `pin_create` 내부 라벨이 남는다.

#### Scenario: scheduler 허용 외 errorKind 미전달
- **WHEN** Harvester consumer 구현체를 정적으로 점검할 때
- **THEN** `RecordHarvestError`에 전달되는 errorKind 값은 항상 `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"` 중 하나이며, 다른 문자열이 전달되는 경로가 존재하지 않는다.

#### Scenario: SetStatus와 RecordHarvestError 둘 다 호출 보장
- **WHEN** 어떤 실패 경로로든 iteration이 종료될 때
- **THEN** `SetStatus("harvest_failed", nil)`과 `RecordHarvestError`가 모두 호출된 상태로 다음 iteration으로 넘어간다. 둘 중 하나만 호출하고 종료하지 않는다.

---

### Requirement: 재harvest를 수행하지 않는다 (UPSERT guard에 의존)
Harvester는 "이미 harvest된 URL인지" 직접 확인하는 코드를 포함하지 않아야 한다(SHALL NOT). 재harvest 방지는 `scheduler-frontier-table`이 정의한 UPSERT guard(`WHERE harvester_frontier.harvested_at IS NULL`)에 의해 스키마 수준에서 달성된다고 가정해야 한다(SHALL).

#### Scenario: Pioneer 재크롤 후에도 재harvest 안 함
- **WHEN** Pioneer가 동일 URL을 재크롤하여 `harvester_frontier`에 UPSERT를 시도했을 때 (해당 row는 이미 `harvested_at IS NOT NULL`)
- **THEN** UPSERT guard로 `next_harvest_at`/`snapshot_key`가 덮어써지지 않으며, partial index의 `WHERE harvested_at IS NULL` 조건에 의해 `Dequeue`에 반환되지 않는다.

#### Scenario: Harvester 코드 내부에 harvested 체크 부재
- **WHEN** Harvester 구현체를 정적으로 점검할 때
- **THEN** "이미 harvested인지"를 SELECT하는 경로가 존재하지 않는다. 중복 방지는 partial index와 UPSERT guard로 일원화된다.

---

### Requirement: 다중 워커 정확성은 URLScheduler에 위임한다
Harvester는 동일 URL이 두 워커에 동시 dequeue되는 것을 방지하는 락/큐 정확성 로직을 자체 구현하지 않아야 한다(SHALL NOT). 해당 정확성은 `scheduler-claim-api`의 `FOR UPDATE SKIP LOCKED` 기반 claim이 보장한다고 가정해야 한다(SHALL).

#### Scenario: Harvester 자체 락 부재
- **WHEN** Harvester 구현체를 점검할 때
- **THEN** Harvester가 직접 잡는 advisory lock, 분산 락, 워커 간 조정 채널이 존재하지 않으며, 정확성 보장은 `scheduler.Dequeue`의 계약에 의존한다.

#### Scenario: 임의 워커 수에서 동시 실행 안전
- **WHEN** Harvester 워커 N개(N >= 2)가 동시에 실행될 때
- **THEN** 동일 `normalized_url`이 두 워커에 동시 dequeue되지 않고, 동일 row에 대해 최대 한 번만 Pin 생성 시도가 일어난다 (정확성 자체는 scheduler 계약이 보장).

---

### Requirement: Harvester는 사이트 경계와 무관하게 동작한다
Harvester는 `queue_type`이 harvester인 모든 URL을 host와 무관하게 처리해야 한다(SHALL). 한 워커가 한 번의 실행에서 단일 사이트만 처리한다는 가정을 가져서는 안 된다(SHALL NOT). 사이트별 상태(루트 노드, 사이트별 진행률 등)를 메인 루프에서 유지해서는 안 된다(SHALL NOT).

#### Scenario: 단일 워커가 여러 host row 처리
- **WHEN** `harvester_frontier`에 host A와 host B의 처리 대기 row가 모두 존재하고 워커가 연속해서 `Dequeue`를 호출할 때
- **THEN** 우선순위(`score DESC, next_harvest_at ASC`)에 따라 A와 B의 row가 임의 순서로 반환되며, Harvester는 host가 바뀐다는 사실에 대해 어떤 특별 처리도 하지 않는다.

#### Scenario: 사이트 루트 노드 탐색 부재
- **WHEN** Harvester 메인 루프 코드를 점검할 때
- **THEN** 처리 시작 시 사이트 루트 노드를 찾는 단계(`findRootNode` 등)가 존재하지 않는다. 처리 단위는 `Dequeue`가 반환한 단일 URL 그 자체다.

---

### Requirement: 빈 큐 polling은 Dequeue 내부 책임이다
Harvester consumer 루프는 빈 큐 처리를 위한 자체 sleep/backoff 로직을 가져서는 안 된다(SHALL NOT). `scheduler.Dequeue(QueueHarvester)`는 내부에서 polling(빈 결과 시 1초 sleep 후 재시도)을 수행하고, URL이 claim되기 전에는 return하지 않는 blocking 시그니처여야 한다(SHALL, `scheduler-claim-api`가 보장). 예외는 `ctx` 취소뿐이다.

#### Scenario: consumer 루프에 sleep 호출 부재
- **WHEN** Harvester consumer 루프 코드를 정적으로 점검할 때
- **THEN** `time.Sleep`, `time.After` 등의 자체 polling backoff 호출이 존재하지 않는다. 빈 큐 재시도는 `Dequeue` 내부에서만 발생한다.

#### Scenario: 컨텍스트 취소 시 안전 종료
- **WHEN** `Dequeue` 대기 중 `ctx`가 취소될 때
- **THEN** `Dequeue`가 에러를 반환하고, Harvester는 현재 iteration을 안전하게 종료하여 워커 루프를 빠져나간다.

---

### Requirement: 인메모리 진행 상태를 보유하지 않는다
Harvester 프로세스는 어떤 진행 상태(이미 처리한 URL 집합, 사이트별 진행률, 다음 처리 예정 URL 후보 등)도 인메모리에만 보관해서는 안 된다(SHALL NOT). 모든 공유 상태는 `harvester_frontier`/`harvester_frontier_pins` 또는 다른 영속 저장소에 보관되어야 한다(SHALL).

#### Scenario: 워커 재시작 시 진행 상태 보존
- **WHEN** Harvester 워커 프로세스가 SIGTERM/크래시로 중단되었다가 재시작될 때
- **THEN** 이전에 성공 처리된 row는 `harvested_at`이 채워진 상태로 frontier에 남아 다시 claim되지 않으며, 처리 중이던 row는 트랜잭션 롤백 + lease timeout으로 다시 claim 가능 상태로 복원된다.

#### Scenario: 사이트별 visited/nodeMap 부재
- **WHEN** 워커가 동작 중인 임의 시점에 메모리 사용량을 점검할 때
- **THEN** 사이트별 노드 사전 적재(`nodeMap`)나 사이트별 `visited` 집합 등 사이트 단위로 비례하여 증가하는 자료구조가 존재하지 않는다.

#### Scenario: 다음 처리 후보를 메모리에 누적하지 않음
- **WHEN** 워커가 한 iteration 처리를 완료한 직후
- **THEN** 다음 iteration 후보 URL은 메모리에 보관되지 않으며, 항상 다음 `Dequeue` 호출로 새로 획득한다.

### Requirement: Harvester 워커는 100회 Dequeue 후 종료한다
Harvester 워커 프로세스는 `URLScheduler.Dequeue` 호출을 통해 처리할 URL을 정확히 100회 수령한 뒤 정상 종료(exit code 0)해야 한다(SHALL). 카운터는 "URL을 실제로 반환한 Dequeue 호출"만 증가시켜야 하며, 증가 시점은 **성공 Dequeue 직후**(= 호출이 URL을 리턴한 직후)여야 한다(SHALL). 빈 결과 또는 오류를 반환한 Dequeue 호출은 카운트하지 않아야 한다(SHALL NOT). budget 값(100)은 **빌드 타임 상수**로 고정되어야 하며(SHALL), 환경변수·설정 파일·CLI 플래그 등 어떤 런타임 수단으로도 변경 가능하게 노출되어서는 안 된다(SHALL NOT). ctx 취소 등 외부 종료 신호로 워커가 100회 미만에서 종료되는 경로는 본 정책과 독립적이며, budget은 상한(상향 제한)으로 기능한다. 본 정책은 `pioneer-worker-budget`과 대칭이다.

#### Scenario: 100회 처리 후 정상 종료
- **WHEN** Harvester 워커가 시작되어 `URLScheduler.Dequeue`로부터 URL을 100회 수령하고 각 URL의 harvest 작업을 완료했을 때
- **THEN** 워커 프로세스는 exit code 0으로 종료한다.

#### Scenario: 빈 Dequeue는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue`가 처리할 URL이 없어 빈 결과를 반환할 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 짧게 대기한 뒤 다시 Dequeue를 시도한다.

#### Scenario: Dequeue 자체 오류는 카운트되지 않는다
- **WHEN** `URLScheduler.Dequeue` 호출이 (URL을 반환하지 않고) 오류를 반환할 때
- **THEN** Dequeue 카운터는 증가하지 않으며, 워커는 오류를 로깅한 뒤 다시 Dequeue를 시도한다.

#### Scenario: 카운터는 성공 Dequeue 직후 증가한다
- **WHEN** `URLScheduler.Dequeue`가 URL을 성공적으로 반환한 직후
- **THEN** Dequeue 카운터가 1 증가한 뒤에 해당 URL의 harvest 파이프라인이 시작된다.

#### Scenario: 99회까지는 종료하지 않는다
- **WHEN** Harvester 워커가 URL을 99회 수령하여 처리한 직후
- **THEN** 워커는 종료하지 않고 다음 Dequeue를 호출한다.

#### Scenario: budget은 빌드 시 상수
- **WHEN** 운영자가 환경변수·설정 파일·CLI 플래그로 budget 값을 변경하려 할 때
- **THEN** 워커 동작은 영향을 받지 않으며, 항상 100회 후 종료한다(budget은 빌드 타임에만 결정된다).

#### Scenario: ctx 취소 경로는 budget과 독립적이다
- **WHEN** 100회 도달 전에 외부 ctx 취소 또는 SIGTERM으로 워커가 종료 신호를 받았을 때
- **THEN** 워커는 budget 미소진 상태에서도 종료할 수 있으며(진행 중 fetch/pipeline은 ctx 전파로 중단될 수 있음), budget 정책은 위반되지 않는다(budget은 상향 제한이지 하한이 아니다).

---

### Requirement: 진행 중 harvest 작업이 완료된 뒤에 종료한다
100회째 Dequeue로 받은 URL의 harvest 작업이 진행 중인 동안에는 워커가 종료해서는 안 된다(SHALL NOT). 특히 100회째 URL 처리의 **최종 상태 전이**가 완료된 뒤에만 워커가 exit 0으로 종료해야 한다(SHALL). 최종 상태 전이는 다음 두 경로 중 하나를 의미한다:

- **성공 경로**: `SetStatus(harvested, pinIDs)` 호출이 성공적으로 반환(= `harvester_frontier` 갱신 및 `harvester_frontier_pins` INSERT가 단일 트랜잭션으로 커밋)된 직후.
- **실패 경로**: 기존 "Harvester 실패 시 SetStatus + RecordHarvestError를 둘 다 호출한다" requirement에 따라 `SetStatus(harvest_failed, nil)` 호출과 `RecordHarvestError(errorKind)` 호출이 **둘 다 완료된** 직후.

작업 실패가 워커 종료 코드를 바꾸지 않는다(SHALL). 실패 경로에서도 exit code는 0이다.

#### Scenario: 100회째 작업 성공 완료 후 종료
- **WHEN** Harvester 워커가 100회째 Dequeue로 URL을 받아 harvest를 시작했고, `SetStatus(harvested, pinIDs)` 호출이 성공적으로 반환되었을 때
- **THEN** 워커는 그 직후에 exit code 0으로 종료한다.

#### Scenario: 100회째 작업 진행 중에는 종료하지 않는다
- **WHEN** 100회째 Dequeue 직후 harvest 파이프라인이 외부 페이지 fetch·콘텐츠 추출·pin 생성·상태 전이 중 어느 단계든 수행 중일 때
- **THEN** 워커는 해당 단계가 완료되고 최종 상태 전이 호출이 반환될 때까지 종료하지 않는다.

#### Scenario: 100회째 작업이 실패해도 종료는 정상
- **WHEN** 100회째 URL의 fetch/parse/pin 생성이 오류로 끝나고 `SetStatus(harvest_failed, nil)` 호출과 `RecordHarvestError(errorKind)` 호출이 **둘 다 완료**된 직후
- **THEN** 워커는 exit code 0으로 종료한다(작업 실패가 워커 종료 코드를 바꾸지 않는다).

#### Scenario: 실패 경로 dual-call 도중 종료하지 않는다
- **WHEN** 100회째 URL이 실패로 끝나고 `SetStatus(harvest_failed, nil)`는 반환되었으나 `RecordHarvestError`가 아직 호출되지 않은 상태일 때
- **THEN** 워커는 `RecordHarvestError`가 반환될 때까지 종료를 지연한다(둘 중 하나만 호출하고 종료하지 않는다).

---

### Requirement: 워커 재시작은 supervisor의 책임이다
Harvester 워커 프로세스 자체는 자기 자신을 재기동하는 로직을 가져서는 안 된다(SHALL NOT). 종료 후 새 인스턴스를 띄우는 것은 외부 supervisor(systemd, Kubernetes Deployment, Docker restart policy, foreman 등)의 책임이어야 한다(SHALL). 종료 직전 워커는 `pioneer-worker-budget`과 동일한 필드(`reason=budget_exhausted`, `dequeues=100`, `component=harvester_worker`)를 포함한 **기계 파싱 가능한 key=value 포맷 로그**를 정확히 1회 남겨야 한다(SHALL). (구체 로그 문자열 예시는 tasks.md §2.1 참조.)

#### Scenario: 워커는 자식을 spawn하지 않는다
- **WHEN** Harvester 워커가 100회 처리를 마치고 종료할 때
- **THEN** 워커는 새 워커 프로세스를 fork/exec하거나 내부 루프를 재개하지 않고 단순히 종료한다.

#### Scenario: 종료 사유 로그
- **WHEN** Harvester 워커가 budget 소진으로 종료하기 직전일 때
- **THEN** Pioneer worker-budget과 동일한 필드(`reason=budget_exhausted`, `dequeues=100`, `component=harvester_worker`)를 포함한 key=value 포맷 로그 라인이 정확히 1회 출력된다.

#### Scenario: supervisor가 새 워커를 띄운다
- **WHEN** supervisor(예: docker restart policy, systemd `Restart=always`, k8s `restartPolicy: Always`)가 정상 종료(exit 0)한 Harvester 워커를 감지할 때
- **THEN** supervisor의 재시작 정책에 따라 새 워커 프로세스가 기동되며, 새 워커는 자체 카운터를 0부터 시작한다.

#### Scenario: 종료 시 상태 청산
- **WHEN** 워커가 budget 소진으로 종료할 때
- **THEN** 인메모리 Dequeue 카운터·Goja 런타임·HTTP 커넥션·임시 파일 등 세션 상태는 프로세스 종료와 함께 폐기되며 외부로 전달되지 않는다.

---

### Requirement: Dequeue 카운터는 워커 간 공유 상태가 아니다
Dequeue 카운터는 각 Harvester 워커 프로세스의 인메모리 변수로만 보관되어야 하며(SHALL), DB·Redis·frontier 등 워커 간 공유 저장소에 보관해서는 안 된다(SHALL NOT). 본 카운터는 워커 수명 관리용이며, 도메인 상태가 아니다.

#### Scenario: 복수 워커는 각자 독립 카운터를 갖는다
- **WHEN** Harvester 워커 두 인스턴스가 동시에 실행되고 있을 때
- **THEN** 한 워커가 50회를 처리해도 다른 워커의 카운터에는 영향을 주지 않는다.

#### Scenario: 카운터는 영속되지 않는다
- **WHEN** Harvester 워커가 종료된 직후
- **THEN** 카운터 값은 어디에도 저장되지 않으며, 새로 기동한 워커는 0에서 다시 시작한다.

---

### Requirement: Consumer는 snapshot-first 진입점만 경유하여 fetch를 수행한다

Harvester consumer 루프는 fetch 단계에서 `harvester-snapshot-first-fetch` capability가 제공하는 snapshot-first 진입점만 호출해야 하며(SHALL), 저수준 `Fetcher.Fetch`를 직접 호출하지 않아야 한다(SHALL NOT). Consumer 모듈 코드는 ObjectStorage SDK(S3/MinIO 등) 구현체 타입이나 `net/http` 클라이언트 구현체를 직접 생성·참조해서는 안 된다(SHALL NOT). 인터페이스 기반 의존성 주입으로 진입점을 받는 것은 금지 대상이 아니며, 구체 구현체(클라이언트 생성부)가 consumer 모듈에 존재하지 않는 것이 경계 기준이다. 해당 구현체는 `harvester-snapshot-first-fetch` 구현 모듈 내부에만 존재한다.

#### Scenario: consumer 루프의 fetch 호출은 snapshot-first 진입점뿐이다
- **WHEN** consumer 루프 코드에서 fetch 단계를 정적으로 점검할 때
- **THEN** snapshot-first 진입점 이외의 경로(`Fetcher.Fetch` 직접 호출, raw HTTP, ObjectStorage 직접 조회 등)로 HTML을 가져오는 호출이 존재하지 않는다.

#### Scenario: consumer 모듈에 ObjectStorage/HTTP 클라이언트 의존 부재
- **WHEN** consumer 패키지의 import 그래프를 점검할 때
- **THEN** ObjectStorage(S3/MinIO) SDK 또는 `net/http` 클라이언트 인스턴스 생성부가 직접 참조되지 않으며, fetch 의존은 `harvester-snapshot-first-fetch` 모듈이 제공하는 진입점 심볼로만 연결된다.

---

### Requirement: snapshot-first 진입점의 입력은 `(ctx, url)`이며 `scheduler.Dequeue` 반환 형태와 정합한다

Snapshot-first 진입점의 입력은 `ctx`(컨텍스트)와 `url`(정규화된 URL 문자열) 두 가지여야 한다(SHALL). Consumer는 직전 `scheduler.Dequeue(QueueHarvester)`가 반환한 URL을 그대로 전달해야 하며(SHALL), 스냅샷 키를 인자로 추가 전달하거나, URL을 기반으로 스냅샷 키를 재계산해 넘겨서는 안 된다(SHALL NOT). 스냅샷 키는 진입점 내부에서 `pioneer-snapshot-storage` 공용 빌더로 계산되며(`harvester-snapshot-first-fetch` Decision 5), 이 계산은 consumer 관점에서 관측 대상이 아니다.

#### Scenario: consumer는 Dequeue URL을 그대로 전달한다
- **WHEN** consumer가 `Dequeue(QueueHarvester)`로 URL을 claim한 뒤 snapshot-first 진입점을 호출할 때
- **THEN** 전달하는 두 인자는 `ctx`와 claim 결과 URL뿐이며, 그 외 별도 DB 조회·캐시 조회로 얻은 스냅샷 키나 메타데이터를 인자로 추가하지 않는다.

#### Scenario: consumer는 스냅샷 키를 재계산하지 않는다
- **WHEN** consumer 루프 코드를 정적으로 점검할 때
- **THEN** URL로부터 스냅샷 키를 자체 계산하는 경로나, `harvester_frontier.snapshot_key`를 별도 SELECT로 조회해 진입점에 넘기는 경로가 존재하지 않는다.

---

### Requirement: snapshot-first 진입점의 반환은 3-tuple `(html, errorKind, err)` 의미론을 따른다

Snapshot-first 진입점은 세 결과(`html`, `errorKind`, `err`)를 반환해야 한다(SHALL). 실제 Go 타입(multiple return 또는 named struct)은 구현 선택이며, 행위 계약은 값 3개의 의미론만 규정한다. 성공 반환 시 `html`은 길이 1 이상의 원본 HTML 바이트열이고, `err`는 nil이어야 한다(SHALL). 성공 반환 시 `errorKind`의 구체적 값은 본 spec의 관찰 대상이 아니며, consumer는 성공 경로에서 `errorKind`를 분기 조건으로 사용해서는 안 된다(SHALL NOT). 실패 반환 시 `html`은 nil이고, `err`는 non-nil이며, `errorKind`는 본 spec의 "fetch 단 errorKind는 4종으로 한정된다" requirement의 네 값 중 하나여야 한다(SHALL).

#### Scenario: 성공 반환 형태
- **WHEN** 진입점이 HTML을 성공적으로 반환할 때
- **THEN** `html`은 길이 1 이상의 바이트열이고, `err`는 nil이다.

#### Scenario: 성공 경로에서 consumer는 errorKind를 사용하지 않는다
- **WHEN** 진입점이 성공 반환한 결과를 consumer가 처리할 때
- **THEN** consumer 코드의 후속 흐름은 `errorKind` 값을 분기 조건으로 사용하지 않으며, 성공/실패 판정은 `err`(또는 동등한 실패 표지)로만 이루어진다.

#### Scenario: 실패 반환 형태
- **WHEN** 진입점이 최종적으로 HTML을 확보하지 못할 때
- **THEN** `html`은 nil이고, `err`는 non-nil이며, `errorKind`는 "fetch 단 errorKind는 4종으로 한정된다" requirement의 네 값 중 하나다.

---

### Requirement: fetch 단 `errorKind`는 4종으로 한정된다

Snapshot-first 진입점이 실패 시 반환하는 `errorKind`는 `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"` 네 값 중 하나여야 한다(SHALL). `"parse"`, `"pin_create"`, 또는 기타 임의 문자열을 반환해서는 안 된다(SHALL NOT). Consumer는 fetch 실패 경로에서 이 값을 그대로 `scheduler.RecordHarvestError(url, errorKind)`에 전달해야 하며, HTTP 상태코드나 에러 타입을 다시 검사하여 kind를 재결정하는 로직을 포함해서는 안 된다(SHALL NOT). 본 금지는 **fetch 실패 경로에 한정**되며, 파싱/Pin 생성 실패에서 consumer가 자체 결정한 `"parse"`/`"pin_create"` kind로 `RecordHarvestError`를 호출하는 것은 본 금지의 적용 대상이 아니다. `SetStatus("harvest_failed", nil)`과 `RecordHarvestError`의 이중 호출 규약 자체는 `harvester-scheduler-consumer` capability가 정의하며 본 requirement는 그 규약을 변경하지 않는다.

#### Scenario: HTTP 4xx 응답은 errorKind = "http_4xx"
- **WHEN** snapshot miss 이후 live fetch가 HTTP 4xx로 종료될 때
- **THEN** 진입점은 `errorKind = "http_4xx"`로 실패를 반환한다.

#### Scenario: HTTP 5xx 응답은 errorKind = "http_5xx"
- **WHEN** live fetch가 HTTP 5xx로 종료될 때
- **THEN** 진입점은 `errorKind = "http_5xx"`로 실패를 반환한다.

#### Scenario: DNS/connect/TLS 실패는 errorKind = "network"
- **WHEN** live fetch가 DNS 해석, TCP connect, TLS handshake 실패로 종료될 때
- **THEN** 진입점은 `errorKind = "network"`로 실패를 반환한다.

#### Scenario: 타임아웃은 errorKind = "timeout"
- **WHEN** snapshot 경로 또는 live fetch가 ctx/자체 타임아웃으로 종료될 때
- **THEN** 진입점은 `errorKind = "timeout"`으로 실패를 반환한다.

#### Scenario: 진입점 구현은 4종 외 kind를 반환하지 않는다
- **WHEN** 진입점 구현이 실패 경로를 반환할 때
- **THEN** `errorKind`는 항상 `"http_4xx"`, `"http_5xx"`, `"network"`, `"timeout"` 중 하나이며, `"parse"`, `"pin_create"`, 기타 자유 문자열이 반환되지 않는다.

#### Scenario: consumer는 fetch 실패 경로에서 errorKind를 재분류하지 않는다
- **WHEN** consumer가 진입점의 실패 반환을 받아 `RecordHarvestError`를 호출할 때
- **THEN** 전달하는 kind는 진입점이 반환한 값 그대로이며, consumer가 HTTP 상태코드/에러 타입을 다시 검사해 kind를 재결정하는 로직이 존재하지 않는다.

#### Scenario: parse/pin_create 경로는 본 재분류 금지의 적용 대상이 아니다
- **WHEN** fetch는 성공했으나 이후 `harvestPipeline.Process` 또는 Pin 생성이 실패하여 consumer가 `RecordHarvestError`를 호출할 때
- **THEN** consumer가 `"parse"` 또는 `"pin_create"` kind를 자체 결정해 전달하는 것은 본 requirement의 "errorKind 재분류 금지"와 충돌하지 않는다.

---

### Requirement: 스냅샷 내부 실패 종류는 consumer의 `errorKind`에 노출되지 않는다

ObjectStorage 조회 실패 종류(키 없음 / 만료 / 네트워크 / 권한 / 내부 에러)는 `harvester-snapshot-first-fetch`의 "단일 miss" 규약에 따라 진입점 내부에서 HTTP fallback으로 흡수되며, consumer가 받는 `errorKind`에는 반영되지 않아야 한다(SHALL). 스냅샷 경로 내부 실패가 consumer의 kind 분기에 영향을 주어서는 안 된다(SHALL NOT). 본 requirement는 `harvester-snapshot-first-fetch`가 `bot` capability에 정의한 "실패 유형은 로그로만 구분된다" Scenario를 `harvester` capability의 consumer-fetcher 경계에서 재확인하는 것이며, 새 행위를 도입하지 않는다.

#### Scenario: 스냅샷 키 부재는 consumer에 snapshot 전용 kind로 노출되지 않는다
- **WHEN** ObjectStorage에 스냅샷이 존재하지 않아 snapshot 조회가 miss로 종료될 때
- **THEN** 진입점은 snapshot 전용 kind(키 부재·만료·캐시 miss 등 어떤 명칭이든)를 반환하지 않고, HTTP fallback을 수행한 뒤 그 결과(성공 또는 4종 errorKind 중 하나)를 반환한다.

#### Scenario: 스냅샷 경로 네트워크/권한/내부 에러도 동일하게 HTTP fallback
- **WHEN** ObjectStorage 조회가 네트워크/권한/내부(5xx) 에러로 실패할 때
- **THEN** 진입점은 이 실패 종류를 consumer에 전파하지 않고 HTTP fallback을 수행하며, consumer는 최종 결과(HTTP 성공 또는 4종 errorKind 중 하나의 실패)만 관측한다.

#### Scenario: ctx 취소/deadline은 스냅샷 경로 내부 실패로 분류되지 않고 "timeout"으로 귀결된다
- **WHEN** 스냅샷 조회 또는 HTTP fallback이 ctx 취소/deadline으로 종료될 때
- **THEN** 이는 "스냅샷 경로 내부 실패 흡수"의 대상이 아니라 fetch 단 진입점이 반환하는 `errorKind = "timeout"`로 분류되며, 두 요구(`ObjectStorage 실패 흡수` vs `timeout 분류`)가 경계에서 충돌하지 않는다.

### Requirement: 미디어 후보 유효성 검증

시스템은 추출 단계에서 수집된 미디어 후보(이미지/비디오/오디오)를 PinDocument의 `media_candidates` 또는 `thumbnail_url`로 채택하기 전에 외부 관찰 가능한 유효성 기준으로 검증해야 한다(SHALL). 검증을 통과하지 못한 후보는 PinDocument에 채택되지 않아야 한다(SHALL NOT). 유효성 기준은 (a) 선언된 타입의 미디어로 디코딩 가능할 것, (b) 미디어가 의미 있는 콘텐츠 크기를 가질 것이다. 구체 임계값과 측정 축은 운영 학습으로 조정 가능한 구현 파라미터이며 본 스펙의 행위 계약 일부가 아니다.

#### Scenario: 의미 있는 콘텐츠 크기 임계값을 만족하지 못하는 이미지는 후보에서 제외된다
- **WHEN** 추출된 이미지 후보가 디코딩 결과 의미 있는 콘텐츠 크기 임계값을 만족하지 못할 때 (QA-reported regression: 1x1 픽셀 placeholder GIF)
- **THEN** 해당 후보는 PinDocument의 `media_candidates`와 `thumbnail_url` 어디에도 포함되지 않는다

#### Scenario: 디코딩 불가능한 미디어는 후보에서 제외된다
- **WHEN** 미디어 후보 바이트열이 선언된 타입(image/video/audio)의 디코더 검증에 실패할 때
- **THEN** 해당 후보는 PinDocument의 `media_candidates`와 `thumbnail_url` 어디에도 포함되지 않는다

#### Scenario: 정상 미디어는 검증을 통과해 후보로 채택된다
- **WHEN** 미디어 후보가 디코딩 가능하고 의미 있는 콘텐츠 크기 임계값을 만족할 때
- **THEN** 해당 후보는 기존 추출 행위에 따라 PinDocument의 `media_candidates` 또는 `thumbnail_url`에 채택된다

#### Scenario: 모든 후보가 무효일 때는 빈 배열로 PinDocument가 구성된다
- **WHEN** 추출된 모든 미디어 후보가 검증에 탈락할 때
- **THEN** PinDocument의 `media_candidates`는 빈 배열, `thumbnail_url`은 빈 문자열로 구성된다

---

### Requirement: 정본 키 영속 제한

시스템은 미디어 후보 검증에서 탈락한 파일을 Pin이 참조 가능한 정본 키(Pin의 `media_url`이 가리키는 ObjectStorage 객체 키)로 ObjectStorage에 업로드하지 않아야 한다(SHALL NOT). Pin의 `media_url`이 가리키는 ObjectStorage 자원은 항상 유효성 검증을 통과한 미디어여야 한다(SHALL).

#### Scenario: 검증 탈락 후보는 정본 키에 업로드되지 않는다
- **WHEN** 미디어 후보가 유효성 검증에 실패할 때
- **THEN** 해당 미디어 바이트열은 Pin이 참조하는 정본 키로 ObjectStorage에 영속되지 않는다

#### Scenario: 정본 미디어 키는 항상 유효한 미디어를 가리킨다
- **WHEN** Pin의 `media_url`이 가리키는 ObjectStorage 자원을 조회할 때
- **THEN** 해당 자원은 본 스펙의 미디어 후보 유효성 기준을 만족하는 미디어 파일이다

---

### Requirement: 검증 실패 사유의 og_data 기록

시스템은 미디어 후보가 검증에 탈락한 사실과 사유를 PinDocument의 `og_data`에 관찰 가능한 형태로 기록해야 한다(SHALL). 이 기록은 디버깅 및 운영 메트릭 집계에 사용된다. 기록은 최소한 (a) 탈락한 후보 수, (b) 사유 분류(예: 디코딩 실패, 최소 크기 미달)별 카운트를 외부에서 조회 가능해야 한다. 구체 필드명/포맷은 구현 결정이며 본 스펙의 행위 계약 일부가 아니다.

#### Scenario: 모든 후보가 탈락한 경우 og_data에 사유가 보존된다
- **WHEN** 추출된 미디어 후보가 모두 유효성 검증에 탈락할 때
- **THEN** PinDocument의 `og_data`에서 탈락 후보 수와 사유 분류별 카운트가 관찰 가능하다

#### Scenario: 일부 후보가 탈락한 경우에도 사유가 보존된다
- **WHEN** 추출된 미디어 후보 중 일부만 검증에 탈락하고 나머지가 채택될 때
- **THEN** PinDocument의 `og_data`에서 탈락한 후보 수와 사유가 관찰 가능하며, 채택된 후보들은 정상 경로로 `media_candidates`/`thumbnail_url`에 들어간다

---

### Requirement: Pin primary media invariant

시스템은 본 변경 배포 이후 새로 생성되는 Pin이 가리키는 primary media(`media_url`이 가리키는 자원)가 본 스펙의 미디어 후보 유효성 기준을 만족함을 보장해야 한다(SHALL). 유효한 primary media를 확보할 수 없는 페이지에 대해서는 Pin을 생성하지 않아야 한다(SHALL NOT). 이는 기존 classifier의 `no_primary_media` 경로와 정합한다. 본 변경 배포 이전에 누적된 Pin은 본 invariant의 예외이며, 운영 backfill로 점진 정상화된다.

#### Scenario: 모든 미디어 후보가 무효한 페이지는 Pin을 만들지 않는다
- **WHEN** 페이지에서 추출된 모든 미디어 후보가 유효성 검증에 탈락할 때
- **THEN** Pin은 생성되지 않고 페이지는 `harvested_at`만 마킹되며 classifier reason은 `no_primary_media`로 기록된다

#### Scenario: 유효 미디어가 하나 이상 있으면 Pin이 생성된다
- **WHEN** 페이지에서 추출된 미디어 후보 중 하나 이상이 유효성 검증을 통과하고 다른 pinnability 조건도 만족될 때
- **THEN** Pin이 생성되며 `media_url`은 검증을 통과한 미디어 자원을 가리킨다

### Requirement: 노드 단위 통계는 워커 lifetime 동안 관찰 가능하다

Harvester 워커 프로세스는 "Harvester 노드 단위 통계 정의" Requirement가 정의한 5개 카운터(`PinsCreated`, `Deduped`, `Skipped`, `Failed`, `AdapterFallback`) 각각의 누적 값을 워커 lifetime 동안 외부에서 읽을 수 있는 상태로 보유해야 한다(SHALL). 카운터 값은 워커 종료 시 폐기되며 워커 간 공유되지 않는다(SHALL NOT, "Dequeue 카운터는 워커 간 공유 상태가 아니다" Requirement와 동일 정책).

#### Scenario: 노드 처리 직후 카운터 변화가 관찰된다
- **WHEN** 어떤 노드 처리가 끝난 직후 카운터 값을 읽을 때
- **THEN** 해당 노드의 분류에 해당하는 주 카테고리 카운터가 처리 전 대비 1 증가한 값으로 관찰되며, `AdapterFallback`은 어댑터 실패가 발생한 경우에 한해 함께 1 증가한 값으로 관찰된다

#### Scenario: 다중 노드 처리 시 카운터가 누적된다
- **WHEN** 같은 워커가 N개의 노드를 처리한 직후 카운터 값을 읽을 때
- **THEN** 5개 카운터의 합(주 카테고리 4개의 합 + `AdapterFallback`은 별도)이 노드 수와 어댑터 fallback 발생 횟수에 정합하게 누적되어 있다 (주 카테고리 4개 합 = 처리 노드 수)

#### Scenario: 카운터는 워커 종료 시 폐기된다
- **WHEN** Harvester 워커 프로세스가 종료될 때
- **THEN** in-memory 카운터 값은 어떤 외부 저장소(DB/Redis/파일)에도 보관되지 않으며, 새 워커는 모든 카운터를 0에서 시작한다

---

### Requirement: 외부 미디어 fetch는 SSRF-safe HTTP client를 경유한다

Harvester가 외부 사이트로부터 추출된 — 즉, 호출자가 제어할 수 없는 — URL에 대해 미디어 바이트를 직접 가져와 객체 저장소에 적재할 때, 그 HTTP fetch는 SSRF-safe HTTP client를 경유해야 한다(SHALL). SSRF-safe HTTP client는 다음을 모두 enforce한다:

- 매 outbound 연결의 dial 단계에서 대상 호스트의 모든 해소된 IP가 private/reserved 범위(loopback, link-local, IPv4 RFC1918, IPv6 ULA, unspecified, carrier-grade NAT, benchmarking, documentation 등)에 속하면 연결을 거부한다(SHALL).
- HTTP redirect의 매 hop마다 대상 host를 재해소하고 같은 검사를 반복한다(SHALL).
- 명시적 connect timeout과 total timeout을 가진다(SHALL). 응답이 그 안에 완료되지 않으면 client는 에러로 종료한다(SHALL).
- 명시적 최대 redirect 횟수를 초과하면 에러로 종료한다(SHALL).
- non-`http`/`https` scheme으로의 redirect를 거부한다(SHALL).

Harvester가 외부 미디어 응답 본문을 stream으로 객체 저장소에 전달할 때는 명시적인 stream 크기 상한 가드를 갖춰야 한다(SHALL). 외부 서버가 `Content-Length`를 거짓으로 작게 응답하더라도 실제 stream된 바이트 수가 상한을 넘으면 fetch가 끊겨야 한다(SHALL).

본 Requirement는 기존 Requirement `Generic HTML→Pin extractor가 default 변환 경로다`가 추출하는 `og:image`/`twitter:image`/JSON-LD `schema.org` image 후보 URL과, 그 외 페이지에서 추출되는 임의 미디어 URL에 대한 fetch 경로를 모두 포괄한다. 기존 Scenario의 추출 / 우선순위 / 정규화 정의는 변경하지 않는다.

#### Scenario: 후보 이미지 URL이 private/reserved IP를 가리키면 거부

- **WHEN** 외부 페이지의 `og:image`가 `http://169.254.169.254/latest/meta-data/...` 같은 link-local IP URL이고 Harvester가 그 URL을 cacheImage 경로로 fetch 하려고 시도하면
- **THEN** SSRF-safe client의 dialer가 dial 직전에 IP 검사로 연결을 거부하고, 외부 저장소(S3/MinIO)에 해당 응답 바이트가 객체로 적재되지 않는다. cacheImage는 fallback 경로(원본 candidate URL 그대로 반환)로 진입한다.

#### Scenario: 미디어 URL이 사설 IP를 가리키면 거부

- **WHEN** 외부 페이지에서 추출된 `mediaURL`이 `http://10.0.0.1/...` 같은 IPv4 private 범위 IP URL이고 Harvester가 downloadAndUpload 경로로 fetch 하려고 시도하면
- **THEN** SSRF-safe client의 dialer가 연결을 거부하고, 외부 저장소에 해당 응답 바이트가 적재되지 않으며, 호출자는 fetch 에러로 인식한다.

#### Scenario: redirect 응답에서 사설 IP로의 hop을 거부

- **WHEN** 외부 서버가 200 OK 대신 302 Location: `http://192.168.1.1/...` 같은 응답을 돌려보내고 Harvester가 그 redirect를 follow 하려고 시도하면
- **THEN** SSRF-safe client의 CheckRedirect 콜백이 hop의 host를 재해소하여 사설 IP 매핑을 감지하고 redirect를 거부한다. 외부 저장소에 사설 호스트 응답 바이트가 적재되지 않는다.

#### Scenario: 공개 IP를 가리키는 정상 미디어는 통과

- **WHEN** 외부 페이지의 `og:image`가 공개 IP를 가진 정상 CDN URL(예: `https://github.githubassets.com/favicon.ico`)이고 Harvester가 cacheImage 경로로 fetch 하면
- **THEN** SSRF-safe client는 정상 응답을 받아 객체 저장소에 적재하고 storage URL을 반환한다. 이전 동작과 동일.

#### Scenario: total timeout 초과 시 fetch가 종료된다

- **WHEN** 외부 서버가 응답 본문을 매우 느린 속도(예: 1바이트/초)로 전송하여 SSRF-safe client에 설정된 total timeout 안에 응답이 끝나지 않는 경우
- **THEN** client는 timeout 에러로 종료하고 호출자는 fetch 에러로 인식하며, Harvester 워커는 다음 작업으로 진행한다. 외부 저장소에는 미완료 부분 응답이 적재되지 않는다.


---

### Requirement: title은 pins.title 컬럼 cap에 맞춰 rune-safe 사전 절단된다

시스템은 `PinDocument.Title`을 `pins.title` 컬럼에 저장하기 전에 200 rune 이내로 잘라야 한다(SHALL). 절단은 UTF-8 rune 경계에서만 수행되며 멀티바이트 문자를 바이트 경계에서 절단해서는 안 된다(SHALL NOT). 200 rune 이하 입력은 무손실로 보존된다(SHALL).

이는 `pins.title VARCHAR(200) NOT NULL` 컬럼 cap에서 발생하는 `value too long for type character varying(200)` 거부를 사전 차단해 Pin upsert SHALL이 결정적으로 충족되도록 한다. `pins.description`의 500 rune cap 사전 절단과 동일한 패턴을 title에 대해서도 enforce한다.

#### Scenario: 201 rune title 입력 시 200 rune으로 잘려 저장된다
- **WHEN** Pioneer가 가져온 페이지의 `<title>` 또는 `<h1>` 또는 `og:title`이 201 rune 이상이고 Harvester가 그 PinDocument로 Pin을 upsert하려 할 때
- **THEN** 시스템은 `pins.title`에 정확히 200 rune까지만 저장하며 PostgreSQL의 `value too long for type character varying(200)` 에러를 발생시키지 않는다

#### Scenario: 멀티바이트 title은 rune 경계에서 잘린다
- **WHEN** title이 한국어/일본어/이모지 등 멀티바이트 문자열이고 201 rune 이상일 때
- **THEN** 시스템은 200번째 rune까지만 보존하며, 201번째 rune의 일부 바이트가 잘려 들어가 깨진 문자가 발생하지 않는다

#### Scenario: 200 rune 이하 title은 무손실 보존된다
- **WHEN** title이 빈 문자열 또는 200 rune 이하일 때
- **THEN** 시스템은 입력 그대로 `pins.title`에 저장하며 길이/내용을 변경하지 않는다

#### Scenario: classifier 입력은 절단되지 않은 원본 title을 받는다
- **WHEN** classifier가 PinDocument를 평가할 때
- **THEN** classifier는 잘리지 않은 원본 title로 판정을 수행하며, `pins.title`에 저장되는 값은 이와 무관하게 200 rune으로 잘린 형태다 (description rune-cap 정책과 대칭)


---

### Requirement: ProcessDocument의 media_url 후보는 pins.media_url 컬럼 cap에 맞춰 사전 차단된다

시스템은 `PinDocument.ThumbnailURL`과 `PinDocument.MediaCandidates`의 각 `URL` 길이를 `pins.media_url` 컬럼 cap(500 rune) 기준으로 검사해야 하며(SHALL), 500 rune을 초과하는 후보는 ProcessDocument의 media_url 선택 단계에 도달하기 전에 제거해야 한다(SHALL). 길이 검사는 UTF-8 rune 단위로 수행하며 바이트 단위로 수행해서는 안 된다(SHALL NOT). 500 rune 이하 입력은 변경 없이 보존된다(SHALL).

URL은 의미 단위가 rune 경계에 정렬되지 않으므로 시스템은 절단(truncate)이 아닌 차단(skip) 형태로 cap을 enforce한다(SHALL). 차단된 후보는 picker의 fallback chain에서 다음 후보로 자연 승계되며, 모든 후보가 차단되면 classifier의 `no_primary_media` 판정이 자연 발화하여 Pin은 생성되지 않고 URL은 skipped+harvested로 처리된다(SHALL).

이는 `pins.media_url VARCHAR(500) NOT NULL` 컬럼 cap(`apps/api/db/migrations/000012_pivot_pins_media.up.sql`)에서 발생하는 PostgreSQL `value too long for type character varying(500)` 거부를 사전 차단해 Pin upsert SHALL(idempotent via ON CONFLICT DO UPDATE)이 결정적으로 충족되도록 한다. title cap 사전 절단(L781-801)과 동일한 enforce-before-write 패턴을 media_url에 대해서도 적용하되, URL semantic 보호를 위해 절단이 아닌 차단으로 형태를 달리한다.

#### Scenario: 501 rune ThumbnailURL은 ProcessDocument 도달 전에 차단된다
- **WHEN** Pioneer가 가져온 페이지의 `og:image` 또는 `twitter:image` 또는 첫 `<img src>`가 501 rune 이상의 URL이고 Harvester가 그 PinDocument를 처리할 때
- **THEN** 시스템은 `PinDocument.ThumbnailURL`을 빈 문자열로 만들어 picker가 MediaCandidates fallback으로 진행하도록 하고, PostgreSQL의 `value too long for type character varying(500)` 에러를 발생시키지 않는다

#### Scenario: 모든 후보가 500 rune을 초과하면 Pin이 생성되지 않는다
- **WHEN** `PinDocument.ThumbnailURL`과 `PinDocument.MediaCandidates`의 모든 URL이 500 rune을 초과하는 경우
- **THEN** 시스템은 모든 후보를 차단하여 classifier가 `no_primary_media` 판정을 내리도록 하고, ProcessDocument는 호출되지 않으며 URL은 skipped+harvested로 기록되어 harvest retry 비용이 누수되지 않는다

#### Scenario: 500 rune 이하 media URL은 무손실 보존된다
- **WHEN** ThumbnailURL과 MediaCandidates 후보가 모두 500 rune 이하일 때
- **THEN** 시스템은 입력 URL을 변경 없이 그대로 picker에 전달하며 `pins.media_url`에 입력 그대로 저장한다

#### Scenario: 멀티바이트 media URL은 rune 단위로 검사된다
- **WHEN** media URL이 percent-encoded 한국어/일본어/이모지 등 멀티바이트 시퀀스를 포함하여 rune 수가 500을 초과하지만 바이트 수는 더 클 때
- **THEN** 시스템은 rune 카운트 기준으로 차단 여부를 판정하며 바이트 카운트로 잘못 판정하지 않는다
